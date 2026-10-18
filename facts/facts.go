// Package facts is the deadset facts document as Go values, generated from
// contract/facts.schema.json, and the rules every writer and reader applies to
// it: [Decode] and [Encode], which refuse what the schema and [Validate]
// refuse, and [Build], which puts a document in its canonical order.
package facts

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/document"
	"github.com/cplieger/deadset-spec/v8/report"
	"github.com/cplieger/deadset-spec/v8/vocab"
	"golang.org/x/mod/semver"
)

// ErrInvalid reports a document that is no facts document the facts schema
// and the rules of [Validate] admit. The error wraps the [*document.Error]
// naming the value at fault, or [document.ErrMalformed].
var ErrInvalid = errors.New("facts: not a facts document the facts schema admits")

// schemaFamily is the family of the schema reader's codes.
const schemaFamily = vocab.Prefix + "18"

var schema = sync.OnceValues(func() (*document.Schema, error) {
	return document.Compile(spec.Contract, "contract/facts.schema.json")
})

// Decode reads one facts document, refusing, wrapping [ErrInvalid], a document
// that is not strict JSON, every value the facts schema refuses and every
// value the rules of [Validate] refuse.
func Decode(data []byte) (*Document, error) {
	if err := check(data); err != nil {
		return nil, err
	}
	var read Document
	if err := json.Unmarshal(data, &read); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := read.rules(); err != nil {
		return nil, err
	}
	return &read, nil
}

// Validate refuses, as [Decode] would, a document built in memory. Beyond the
// schema it holds the document to the rules no schema keyword states: the
// canonical order and uniqueness [Build] gives, a generated_file naming a
// generated entry with a generator, a via naming another implementer record, a
// carried finding naming its record's component, a generator row of the
// document's language holding the stated version, every symbol in the
// language's scheme, a schema record's symbol equal to its schema, and an
// embedded finding minted by the writer for its own record.
func Validate(d *Document) error {
	encoded, err := marshal(d)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := check(encoded); err != nil {
		return err
	}
	return d.rules()
}

// Encode writes d with two spaces of indentation, members in the schema's
// order and one trailing line feed, refusing before writing anything a
// document [Decode] would refuse.
func Encode(w io.Writer, d *Document) error {
	var written bytes.Buffer
	encoder := json.NewEncoder(&written)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(d); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := check(written.Bytes()); err != nil {
		return err
	}
	if err := d.rules(); err != nil {
		return err
	}
	if _, err := w.Write(written.Bytes()); err != nil {
		return fmt.Errorf("facts: write: %w", err)
	}
	return nil
}

// Build puts d in the canonical order at the facts version this module writes,
// with no array nil, and refuses it as [Validate] would.
func Build(d *Document) (*Document, error) {
	d.FactsVersion = vocab.FactsVersion
	if d.Generated == nil {
		d.Generated = []Generated{}
	}
	if d.Bindings == nil {
		d.Bindings = []Binding{}
	}
	slices.SortFunc(d.Generated, func(a, b Generated) int { return strings.Compare(a.Path, b.Path) })
	for i := range d.Generated {
		slices.Sort(d.Generated[i].SchemaFiles)
	}
	for i := range d.Bindings {
		b := &d.Bindings[i]
		slices.Sort(b.Via)
		slices.Sort(b.Uses)
		slices.Sort(b.Members)
		if b.Role == RoleSchema && b.Uses == nil {
			b.Uses = []string{}
		}
		if b.Role == RoleSchema && b.Members == nil {
			b.Members = []string{}
		}
	}
	slices.SortFunc(d.Bindings, func(a, b Binding) int { return compareTuple(&a, &b) })
	if err := Validate(d); err != nil {
		return nil, err
	}
	return d, nil
}

// compareTuple is the bindings order: schema file, schema, role, symbol.
func compareTuple(a, b *Binding) int {
	return cmp.Or(strings.Compare(a.SchemaFile, b.SchemaFile), strings.Compare(a.Schema, b.Schema),
		strings.Compare(string(a.Role), string(b.Role)), strings.Compare(a.Symbol, b.Symbol))
}

func (d *Document) rules() error {
	withGenerator, err := d.generatedRules()
	if err != nil {
		return err
	}
	scheme := ""
	if row, ok := vocab.LanguageRowOf(d.Language); ok {
		scheme = row.Scheme + "://"
	}
	implementers := map[string]bool{}
	for i := range d.Bindings {
		if d.Bindings[i].Role == RoleImplementer {
			implementers[d.Bindings[i].Symbol] = true
		}
	}
	for i := range d.Bindings {
		if err := d.bindingRules(i, scheme, withGenerator, implementers); err != nil {
			return err
		}
	}
	return nil
}

// generatedRules holds the generated entries to their order and generator
// rows, and returns the paths of the entries naming a generator.
func (d *Document) generatedRules() (map[string]bool, error) {
	withGenerator := map[string]bool{}
	for i := range d.Generated {
		g := &d.Generated[i]
		at := "/generated/" + strconv.Itoa(i)
		if i > 0 && d.Generated[i-1].Path >= g.Path {
			return nil, refused(at, "order", "the entry is not after the previous one by path, or repeats its path")
		}
		if !ascending(g.SchemaFiles) {
			return nil, refused(at+"/schema_files", "order", "the schema files are not unique and ascending")
		}
		if g.Generator != nil {
			if err := d.generatorFits(at+"/generator", g.Generator); err != nil {
				return nil, err
			}
			withGenerator[g.Path] = true
		}
	}
	return withGenerator, nil
}

func (d *Document) bindingRules(i int, scheme string, withGenerator, implementers map[string]bool) error {
	b := &d.Bindings[i]
	at := "/bindings/" + strconv.Itoa(i)
	switch {
	case i > 0 && compareTuple(&d.Bindings[i-1], b) >= 0:
		return refused(at, "order", "the record is not after the previous one by schema file, schema, role and symbol, or repeats its tuple")
	case !ascending(b.Via):
		return refused(at+"/via", "order", "the via entries are not unique and ascending")
	case !ascending(b.Uses):
		return refused(at+"/uses", "order", "the uses entries are not unique and ascending")
	case !ascending(b.Members):
		return refused(at+"/members", "order", "the members entries are not unique and ascending")
	case b.GeneratedFile != "" && !withGenerator[b.GeneratedFile]:
		return refused(at+"/generated_file", "generated_file", "names no generated entry that has a generator")
	case !strings.HasPrefix(b.Symbol, scheme):
		return refused(at+"/symbol", "scheme", "the symbol is not in the scheme "+scheme+" of the document's language")
	case b.Role == RoleSchema && b.Symbol != b.Schema:
		return refused(at+"/symbol", "schema", "a schema record's symbol is not its schema")
	}
	for j, v := range b.Via {
		if v == b.Symbol || !implementers[v] {
			return refused(at+"/via/"+strconv.Itoa(j), "via", "names no other implementer record of the document")
		}
	}
	return d.embeddedRules(at, b)
}

// embeddedRules holds a record's finding to its writer's rules and every
// carried finding to the record's component.
func (d *Document) embeddedRules(at string, b *Binding) error {
	if b.Finding == nil {
		return nil
	}
	if err := d.findingRules(at+"/finding", b, b.Finding); err != nil {
		return err
	}
	for j := range b.Carried {
		if b.Carried[j].Component.ID != b.Finding.Component.ID {
			return refused(at+"/carried/"+strconv.Itoa(j), "carried", "names a component other than the record's finding")
		}
	}
	return nil
}

func (d *Document) findingRules(at string, b *Binding, f *report.Finding) error {
	path := f.Position.Path
	switch {
	case !strings.HasPrefix(f.Component.ID, d.Analyzer.Name+"/c-"):
		return refused(at+"/component/id", "finding", "the component is not minted by "+d.Analyzer.Name)
	case f.Symbol.Ref != b.Symbol:
		return refused(at+"/symbol/ref", "finding", "the finding is not about the record's symbol")
	case b.Role == RoleSchema && vocab.FamilyOf(f.Code) != schemaFamily:
		return refused(at+"/code", "finding", "a schema record's finding carries no "+schemaFamily+" code")
	case b.Role == RoleSchema && path != b.SchemaFile && !strings.HasSuffix(path, "/"+b.SchemaFile):
		return refused(at+"/position/path", "finding", "the finding is not positioned in the record's schema file")
	case b.Role == RoleImplementer && vocab.FamilyOf(f.Code) == schemaFamily:
		return refused(at+"/code", "finding", "an implementer record's finding carries a "+schemaFamily+" code")
	}
	return nil
}

func (d *Document) generatorFits(at string, g *Generator) error {
	rows := vocab.GeneratorRows()
	for i := range rows {
		row := &rows[i]
		if row.ID != g.Row {
			continue
		}
		if row.Language != d.Language {
			return refused(at+"/row", "generator", "the row is a generator of "+string(row.Language)+", not of the document's language")
		}
		if g.Version == "" || (row.VersionMin == "" && row.VersionMax == "") {
			return nil
		}
		if !semver.IsValid(g.Version) ||
			(row.VersionMin != "" && semver.Compare(g.Version, row.VersionMin) < 0) ||
			(row.VersionMax != "" && semver.Compare(g.Version, row.VersionMax) > 0) {
			return refused(at+"/version", "generator", "the version lies outside the row's interval")
		}
		return nil
	}
	return refused(at+"/row", "generator", "names no generator row")
}

func ascending(values []string) bool {
	for i := 1; i < len(values); i++ {
		if values[i-1] >= values[i] {
			return false
		}
	}
	return true
}

func check(data []byte) error {
	compiled, err := schema()
	if err != nil {
		return err
	}
	value, err := document.Decode(data)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if found := compiled.CheckValue(value); len(found) > 0 {
		return fmt.Errorf("%w: %w", ErrInvalid, &found[0])
	}
	return nil
}

func refused(pointer, keyword, reason string) error {
	return fmt.Errorf("%w: %w", ErrInvalid, &document.Error{Pointer: pointer, Keyword: keyword, Reason: reason})
}

func marshal(v any) ([]byte, error) {
	var written bytes.Buffer
	encoder := json.NewEncoder(&written)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(written.Bytes(), []byte("\n")), nil
}
