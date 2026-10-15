package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"strconv"
	"sync"

	"github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/document"
)

// ErrInvalid reports a document that is no report the schemas admit, or a
// report Encode refuses to write. The error wraps the [*document.Error] naming
// the value at fault, or [document.ErrMalformed].
var ErrInvalid = errors.New("report: not a report the report schema admits")

// schema is the report schema, compiled from the Contract this module embeds.
var schema = sync.OnceValues(func() (*document.Schema, error) {
	return document.Compile(spec.Contract, "contract/report.schema.json")
})

// Decode reads one report document. It refuses, wrapping [ErrInvalid], a
// document that is not strict JSON, every value the report and finding
// schemas refuse, and a components listing that is not one entry for each
// component a finding names. It judges neither the schema version nor the
// conformance result.
func Decode(data []byte) (*Report, error) {
	if err := check(data); err != nil {
		return nil, err
	}
	var read Report
	if err := json.Unmarshal(data, &read); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := read.listsEveryComponent(); err != nil {
		return nil, err
	}
	return &read, nil
}

// Validate refuses, as [Decode] would, a report built in memory.
func Validate(r *Report) error {
	encoded, err := marshal(r)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := check(encoded); err != nil {
		return err
	}
	return r.listsEveryComponent()
}

// Encode writes r in the encoding every report is compared in: two spaces of
// indentation, every object's members in the schema's order, no character
// escaped that strict JSON does not require escaped, and one trailing line
// feed. It refuses, before writing anything, a report [Decode] would refuse.
func Encode(w io.Writer, r *Report) error {
	var written bytes.Buffer
	encoder := json.NewEncoder(&written)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(r); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalid, err)
	}
	if err := check(written.Bytes()); err != nil {
		return err
	}
	if err := r.listsEveryComponent(); err != nil {
		return err
	}
	if _, err := w.Write(written.Bytes()); err != nil {
		return fmt.Errorf("report: write: %w", err)
	}
	return nil
}

// check holds one encoded report to the report schema.
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

// listsEveryComponent holds a report that lists components to one listing per
// component a finding or an edge evaluation's finding names, and none other,
// which no schema keyword can state.
func (r *Report) listsEveryComponent() error {
	if r.Components == nil {
		return nil
	}
	listed := make(map[string]bool, len(r.Components))
	for i := range r.Components {
		id := r.Components[i].ID
		if listed[id] {
			return refused("/components/"+strconv.Itoa(i), "the entry lists component "+id+" a second time")
		}
		listed[id] = true
	}
	named := map[string]bool{}
	for at, id := range r.componentsNamed() {
		named[id] = true
		if !listed[id] {
			return refused(at, "the finding names component "+id+", which the components listing lacks")
		}
	}
	for i := range r.Components {
		if !named[r.Components[i].ID] {
			return refused("/components/"+strconv.Itoa(i), "the entry lists component "+r.Components[i].ID+", which no finding names")
		}
	}
	return nil
}

// componentsNamed yields the pointer of every finding and the component it
// names: the reported findings, then those the edge evaluations carry.
func (r *Report) componentsNamed() iter.Seq2[string, string] {
	return func(yield func(string, string) bool) {
		for i := range r.Findings {
			if !yield("/findings/"+strconv.Itoa(i), r.Findings[i].Component.ID) {
				return
			}
		}
		for i := range r.EdgeEvaluations {
			if f := r.EdgeEvaluations[i].Finding; f != nil {
				if !yield("/edge_evaluations/"+strconv.Itoa(i)+"/finding", f.Component.ID) {
					return
				}
			}
		}
	}
}

func refused(pointer, reason string) error {
	return fmt.Errorf("%w: %w", ErrInvalid, &document.Error{Pointer: pointer, Keyword: "components", Reason: reason})
}

// marshal is v's compact encoding, escaping only what strict JSON requires.
func marshal(v any) ([]byte, error) {
	var written bytes.Buffer
	encoder := json.NewEncoder(&written)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(written.Bytes(), []byte("\n")), nil
}
