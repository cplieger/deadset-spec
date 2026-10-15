// Package suppress is the language-neutral core of contract/grammar/suppression.md:
// the records the three suppression mechanisms resolve to, the strict reader of
// the ignore file and the baseline, the binding of their entries, the ledger that
// decides what each record did, the counts a report carries, and the baseline
// writer with the fixpoint a baseline write runs. An analyzer reads its own
// inline directives and its own inventory; everything a record does once it
// exists is decided here.
package suppress

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// Mechanism is where a suppression is written, spelled as a finding carries it.
type Mechanism string

// The three documents a run reads.
const (
	Inline   Mechanism = "inline"
	Ignore   Mechanism = "ignore"
	Baseline Mechanism = "baseline"
)

// Site is where a suppression is written: a target-relative path, and the line
// and column, counting UTF-16 code units, of its first character. A site naming
// the document alone has no line.
type Site struct {
	Path   string
	Line   int
	Column int
}

// key is the site's identity, as a position renders it.
func (s Site) key() string {
	return s.Path + ":" + strconv.Itoa(s.Line) + ":" + strconv.Itoa(s.Column)
}

// Record is one suppression: one code bound to one site, with the reason it
// carries. A directive naming two codes is two records with one reason, and an
// entry naming a symbol two declarations share is one record per declaration.
type Record struct {
	Code   vocab.Code
	Symbol string
	Path   string
	Reason string

	// Bound is the key of the declaration the record bound, as [Finding.Key]
	// spells a finding's, and empty where it bound none.
	Bound     string
	Mechanism Mechanism
	Site      Site
}

// MarksLive reports whether the record keeps its declaration live before the
// sweep: a bound record does, unless its code reports a part, which says the
// part is wanted and not the declaration, or a dependency row, which is no
// declaration.
func (r *Record) MarksLive() bool {
	subject := r.Code.Subject()
	return r.Bound != "" && subject != vocab.SubjectPart && subject != vocab.SubjectDependency
}

// Refusal is one suppression the grammar refuses with a finding of its own: one
// carrying no reason, or an entry naming a symbol and no path. It binds nothing,
// and the finding it becomes reports the suppression as written.
type Refusal struct {
	Reported  vocab.Code
	Code      vocab.Code
	Symbol    string
	Path      string
	Reason    string
	Mechanism Mechanism
	Site      Site
}

// ErrMalformed is the class of every suppression the grammar refuses before any
// finding exists, which ends the run with the usage code.
var ErrMalformed = errors.New("suppress: malformed suppression")

// MalformedError is one directive, entry or document the grammar refuses before
// any finding exists: the text at fault, as written, and the form the grammar
// fixes for it.
type MalformedError struct {
	// Err is the decoder's own error, nil where the grammar refused a value the
	// decoder accepted.
	Err       error
	Text      string
	Want      string
	Mechanism Mechanism
	Site      Site
}

// Error names the mechanism, the site, the text at fault and the form expected.
func (e *MalformedError) Error() string {
	at := e.Site.Path
	if e.Site.Line > 0 {
		at = e.Site.key()
	}
	message := fmt.Sprintf("suppress: %s %s: %s", e.Mechanism, at, e.Text)
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message + ": want " + e.Want
}

// Unwrap is [ErrMalformed] and the decoder's error where it refused the text.
func (e *MalformedError) Unwrap() []error {
	if e.Err == nil {
		return []error{ErrMalformed}
	}
	return []error{ErrMalformed, e.Err}
}

// Entry is one ignore entry or baseline row as the document writes it, in the
// neutral reference shape: a member is nil where the document leaves it out.
type Entry struct {
	Code   vocab.Code
	Symbol string
	Path   *string
	Reason *string
	Site   Site
}

// Binder binds the entries of one analyzer's scheme to its inventory.
type Binder struct {
	// Lookup is the keys of every declaration the inventory holds under one
	// reference in one file, none where it holds none.
	Lookup func(symbol, path string) []string

	// Holder is the reference of the declaration holding the part a reference
	// names, and false where the reference names no part.
	Holder func(symbol string) (string, bool)

	// Valid reports whether a reference of the scheme is in the analyzer's
	// grammar; one that is not is malformed.
	Valid func(symbol string) bool

	// Scheme opens every reference of the analyzer's language, "go://" for one.
	Scheme string
}

// Bind resolves the entries of the binder's scheme to records, binding each to
// the declarations its reference and path name; an entry under a part kind
// naming a part binds the declaration holding it. An entry of another scheme
// belongs to another analyzer and resolves to nothing. An entry with no reason,
// or naming no path, is a refusal and binds nothing, and one whose reference is
// outside the grammar is a [*MalformedError].
func (b *Binder) Bind(entries []Entry, mechanism Mechanism) ([]Record, []Refusal, error) {
	var records []Record
	var refusals []Refusal
	for i := range entries {
		entry := &entries[i]
		if !ofScheme(entry.Symbol, b.Scheme) {
			continue
		}
		if !b.Valid(entry.Symbol) {
			return nil, nil, &MalformedError{Text: strconv.Quote(entry.Symbol), Want: "a stable symbol reference of " + b.Scheme, Site: entry.Site, Mechanism: mechanism}
		}
		if refused := refusalsOf(entry, mechanism); refused != nil {
			refusals = append(refusals, refused...)
			continue
		}
		records = append(records, b.records(entry, mechanism)...)
	}
	return records, refusals, nil
}

// refusalsOf is every refusal one entry carries: one without a reason and one
// naming no path each, since neither rule outranks the other.
func refusalsOf(entry *Entry, mechanism Mechanism) []Refusal {
	refusal := Refusal{Code: entry.Code, Symbol: entry.Symbol, Path: value(entry.Path), Reason: value(entry.Reason), Site: entry.Site, Mechanism: mechanism}
	var refused []Refusal
	if Blank(entry.Reason) {
		refusal.Reported = vocab.SuppressionWithoutReason
		refused = append(refused, refusal)
	}
	if refusal.Path == "" {
		refusal.Reported = vocab.UnscopedIgnoreEntry
		refused = append(refused, refusal)
	}
	return refused
}

// records is one well-formed entry's records: one per declaration it binds, or
// one bound to nothing.
func (b *Binder) records(entry *Entry, mechanism Mechanism) []Record {
	path := value(entry.Path)
	one := Record{Code: entry.Code, Symbol: entry.Symbol, Path: path, Reason: value(entry.Reason), Site: entry.Site, Mechanism: mechanism}
	bound := b.Lookup(entry.Symbol, path)
	if held, part := b.Holder(entry.Symbol); len(bound) == 0 && part && entry.Code.Subject() == vocab.SubjectPart {
		bound = b.Lookup(held, path)
	}
	if len(bound) == 0 {
		return []Record{one}
	}
	records := make([]Record, len(bound))
	for i, key := range bound {
		records[i] = one
		records[i].Bound = key
	}
	return records
}

// Blank reports whether a reason is absent, empty or whitespace only, which the
// grammar reads as no reason at all.
func Blank(reason *string) bool {
	if reason == nil {
		return true
	}
	for _, r := range *reason {
		if r != ' ' && r != '\t' && r != '\r' && r != '\n' {
			return false
		}
	}
	return true
}

// value is the string a member holds, empty where it is absent.
func value(held *string) string {
	if held == nil {
		return ""
	}
	return *held
}
