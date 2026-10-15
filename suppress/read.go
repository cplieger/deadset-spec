package suppress

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// The names the grammar fixes for the two documents, both at the target root.
const (
	IgnoreFileName   = "deadset-ignore.json"
	BaselineFileName = "deadset-baseline.json"
)

// shape is what one document looks like to the reader: its file, the member
// holding its entries, and the forms a refusal names.
type shape struct {
	file, member, arrayWant, entryWant string
	mechanism                          Mechanism
}

var (
	ignoreShape = shape{
		file: IgnoreFileName, member: "ignore", mechanism: Ignore,
		arrayWant: "an object carrying the ignore array, and an optional description",
		entryWant: "an entry naming code, symbol, path and reason, each a string",
	}
	baselineShape = shape{
		file: BaselineFileName, member: "baseline", mechanism: Baseline,
		arrayWant: "an object carrying the baseline array, and an optional description",
		entryWant: "a row naming code, symbol, path and reason, each a string",
	}
)

// The forms a malformed value of an entry is told to take.
const (
	codeWant   = "DS followed by four digits"
	pathWant   = "a path relative to the target root, with the solidus as separator"
	symbolWant = "a stable symbol reference, <scheme>://<reference>"
)

// forms are the code and path patterns of contract/finding.schema.json, which
// an entry's code and path take, and the scheme a reference opens with.
var forms = sync.OnceValues(func() (*entryForms, error) {
	data, err := spec.Contract.ReadFile("contract/finding.schema.json")
	if err != nil {
		return nil, err
	}
	var schema struct {
		Properties struct {
			Code struct {
				Pattern string `json:"pattern"`
			} `json:"code"`
		} `json:"properties"`
		Defs struct {
			RelativePath struct {
				Pattern string `json:"pattern"`
			} `json:"relative_path"`
		} `json:"$defs"`
	}
	if decodeErr := json.Unmarshal(data, &schema); decodeErr != nil {
		return nil, decodeErr
	}
	code, err := regexp.Compile(schema.Properties.Code.Pattern)
	if err != nil {
		return nil, err
	}
	path, err := regexp.Compile(schema.Defs.RelativePath.Pattern)
	if err != nil {
		return nil, err
	}
	return &entryForms{code: code, path: path}, nil
})

type entryForms struct{ code, path *regexp.Regexp }

// scheme is the opening of a stable symbol reference: a scheme name and ://.
var scheme = regexp.MustCompile(`^[a-z][a-z0-9]*://[^\r\n]+$`)

// ofScheme reports whether symbol opens with the scheme, "go://" for one.
func ofScheme(symbol, own string) bool { return strings.HasPrefix(symbol, own) }

// ReadIgnore reads the ignore file's entries in document order, each at the
// brace that opens it. An absent document is nil data and holds none.
func ReadIgnore(data []byte) ([]Entry, error) { return read(data, &ignoreShape) }

// ReadBaseline reads the baseline's rows in document order, each at the brace
// that opens it. An absent document is nil data and holds none.
func ReadBaseline(data []byte) ([]Entry, error) { return read(data, &baselineShape) }

// read is one document's entries. The document is strict JSON with a closed key
// list at both levels: a member written twice at any depth, an undeclared key, a
// value of the wrong type, a missing code or symbol, and a code, symbol or path
// outside its form are each a [*MalformedError].
func read(data []byte, held *shape) ([]Entry, error) {
	if data == nil {
		return nil, nil
	}
	sites, err := entrySites(data, held)
	if err != nil {
		return nil, err
	}
	array, err := held.array(data)
	if err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(array))
	for i, raw := range array {
		at := held.site()
		if i < len(sites) {
			at = sites[i]
		}
		entry, err := held.entry(raw, at)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// array is the entries a document's array holds, unread: the document is an
// object holding the array, required and possibly empty, and a description
// the analysis does not read.
func (s *shape) array(data []byte) ([]json.RawMessage, error) {
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(data, &wire); err != nil {
		return nil, s.refuse(err, s.file, s.arrayWant, s.site())
	}
	var entries []json.RawMessage
	var description string
	for name, raw := range wire {
		var err error
		switch name {
		case s.member:
			err = json.Unmarshal(raw, &entries)
		case "description":
			err = json.Unmarshal(raw, &description)
		default:
			return nil, s.refuse(nil, "the member "+strconv.Quote(name), s.arrayWant, s.site())
		}
		if err != nil {
			return nil, s.refuse(err, s.file, s.arrayWant, s.site())
		}
	}
	if entries == nil {
		return nil, s.refuse(nil, s.file, s.arrayWant, s.site())
	}
	return entries, nil
}

// wireEntry is the closed key list of one entry; each member is a pointer, so an
// absent member is told from an empty one.
type wireEntry struct {
	Code   *string `json:"code"`
	Symbol *string `json:"symbol"`
	Path   *string `json:"path"`
	Reason *string `json:"reason"`
}

// entry decodes one entry and refuses every defect that ends the run.
func (s *shape) entry(raw json.RawMessage, at Site) (Entry, error) {
	held, err := forms()
	if err != nil {
		return Entry{}, err
	}
	var wire wireEntry
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return Entry{}, s.refuse(err, string(raw), s.entryWant, at)
	}
	switch {
	case wire.Code == nil:
		return Entry{}, s.refuse(nil, "an entry naming no code", s.entryWant, at)
	case wire.Symbol == nil:
		return Entry{}, s.refuse(nil, "an entry naming no symbol", s.entryWant, at)
	case !held.code.MatchString(*wire.Code):
		return Entry{}, s.refuse(nil, strconv.Quote(*wire.Code), codeWant, at)
	case !scheme.MatchString(*wire.Symbol):
		return Entry{}, s.refuse(nil, strconv.Quote(*wire.Symbol), symbolWant, at)
	case value(wire.Path) != "" && !held.path.MatchString(*wire.Path):
		return Entry{}, s.refuse(nil, strconv.Quote(*wire.Path), pathWant, at)
	}
	return Entry{Code: vocab.Code(*wire.Code), Symbol: *wire.Symbol, Path: wire.Path, Reason: wire.Reason, Site: at}, nil
}

func (s *shape) refuse(err error, text, want string, at Site) error {
	return &MalformedError{Err: err, Text: text, Want: want, Site: at, Mechanism: s.mechanism}
}

// site names the document itself.
func (s *shape) site() Site { return Site{Path: s.file} }

// walk reads a document as tokens, which a decode cannot: a member written
// twice is refused rather than one value chosen, and the brace opening each
// entry is recorded, which is the site every finding about the entry carries.
type walk struct {
	held    *shape
	data    []byte
	entries []Site
}

// The roles a value of the document plays.
const (
	roleDocument = iota
	roleArray
	roleEntry
	roleOther
)

// entrySites is the site of every entry the document's array holds.
func entrySites(data []byte, held *shape) ([]Site, error) {
	w := &walk{held: held, data: data}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := w.value(decoder, roleDocument); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, held.refuse(nil, "content after the document", held.arrayWant, held.site())
	}
	return w.entries, nil
}

func (w *walk) value(decoder *json.Decoder, role int) error {
	token, err := decoder.Token()
	if err != nil {
		return w.malformed(decoder, err)
	}
	switch token {
	case json.Delim('{'):
		if role == roleEntry {
			w.entries = append(w.entries, w.at(decoder.InputOffset()-1))
		}
		err = w.object(decoder, role)
	case json.Delim('['):
		inner := roleOther
		if role == roleArray {
			inner = roleEntry
		}
		for err == nil && decoder.More() {
			err = w.value(decoder, inner)
		}
	default:
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := decoder.Token(); err != nil {
		return w.malformed(decoder, err)
	}
	return nil
}

// object reads the members of the object just opened, refusing one written
// twice.
func (w *walk) object(decoder *json.Decoder, role int) error {
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return w.malformed(decoder, err)
		}
		name, _ := key.(string)
		if seen[name] {
			return w.held.refuse(nil, fmt.Sprintf("the member %s is written twice, so neither value is chosen", strconv.Quote(name)),
				w.held.arrayWant, w.at(decoder.InputOffset()))
		}
		seen[name] = true
		member := roleOther
		if role == roleDocument && name == w.held.member {
			member = roleArray
		}
		if err := w.value(decoder, member); err != nil {
			return err
		}
	}
	return nil
}

// malformed is a document the walk could not read, at the place it stopped.
func (w *walk) malformed(decoder *json.Decoder, err error) error {
	return w.held.refuse(err, w.held.file, w.held.arrayWant, w.at(decoder.InputOffset()))
}

// at is one byte offset of the document as a site: the line from one, and the
// column counting UTF-16 code units from one.
func (w *walk) at(offset int64) Site {
	if offset < 0 || offset > int64(len(w.data)) {
		return w.held.site()
	}
	before := w.data[:offset]
	start := bytes.LastIndexByte(before, '\n') + 1
	column := 1
	for line := before[start:]; len(line) > 0; {
		r, size := utf8.DecodeRune(line)
		column += len(utf16.Encode([]rune{r}))
		line = line[size:]
	}
	return Site{Path: w.held.file, Line: 1 + bytes.Count(before, []byte("\n")), Column: column}
}
