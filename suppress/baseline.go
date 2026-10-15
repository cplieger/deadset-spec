package suppress

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/cplieger/deadset-spec/v8/report"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// baselineDescription is what a written baseline says about itself.
const baselineDescription = "Recorded findings: a run fails only on a finding this document does not hold. " +
	"Every reason is the analyzer that recorded the row, never an adjudication."

// errProvenance reports a baseline write whose rows would carry no reason.
var errProvenance = errors.New("suppress: a baseline row would carry no reason")

// Recorded is one finding a baseline row records: the three values a later run
// matches the row on.
type Recorded struct {
	Code   vocab.Code
	Symbol string
	Path   string
}

// Provenance is the analyzer identity every written row carries as its reason.
type Provenance struct {
	Analyzer string
	Version  string
}

// Recordable reports whether a round records a row for a reported finding: none
// for one about a row of a document other than a dependency row, which no record
// binds, so its row would be stale on the first read back.
func Recordable(found *report.Finding) bool {
	subject, _ := vocab.SubjectOf(found.Symbol.Kind)
	return subject != vocab.SubjectFile && subject != vocab.SubjectRecord
}

// Fixpoint runs the rounds of a baseline write: the first reads no row, each
// later one reads back the rows recorded so far. A round answers the rows it
// records, in report order, and the rows it reports stale. Each round appends
// the rows not already held and not removed earlier, and removes the stale
// ones, and the write ends with the first round that records and removes
// nothing, so N distinct rows end within 2N + 1 rounds.
func Fixpoint(round func(read []Recorded) (recorded, stale []Recorded, err error)) ([]Recorded, error) {
	rows := []Recorded{}
	removed := map[Recorded]bool{}
	for {
		recorded, stale, err := round(slices.Clone(rows))
		if err != nil {
			return nil, err
		}
		gone := map[Recorded]bool{}
		for _, row := range stale {
			gone[row] = true
			removed[row] = true
		}
		kept := slices.DeleteFunc(rows, func(row Recorded) bool { return gone[row] })
		added := 0
		for _, row := range recorded {
			if !removed[row] && !slices.Contains(kept, row) {
				kept = append(kept, row)
				added++
			}
		}
		if added == 0 && len(stale) == 0 {
			return kept, nil
		}
		rows = kept
	}
}

// OtherRows is every entry of a baseline not of scheme, the scheme of the rows a
// write records, in document order, which the write keeps member for member.
func OtherRows(entries []Entry, scheme string) []Entry {
	var kept []Entry
	for i := range entries {
		if !ofScheme(entries[i].Symbol, scheme) {
			kept = append(kept, entries[i])
		}
	}
	return kept
}

// WriteBaseline writes the baseline document: one row per recorded finding, in
// the order given, each carrying the provenance as its reason, and every row of
// another scheme the document keeps, the rows ordered by their scheme compared
// bytewise and in the order given within one, so the document is the same
// whichever analyzer writes last.
func WriteBaseline(w io.Writer, recorded []Recorded, p Provenance, others []Entry) error {
	if p.Analyzer == "" || p.Version == "" {
		return fmt.Errorf("%w: the analyzer identity names %q at version %q", errProvenance, p.Analyzer, p.Version)
	}
	reason := "recorded by " + p.Analyzer + " " + p.Version
	rows := make([]wireRow, 0, len(recorded)+len(others))
	for i := range recorded {
		found := &recorded[i]
		if found.Code == "" || found.Symbol == "" || found.Path == "" {
			return fmt.Errorf("%w: a finding names code %q, symbol %q and path %q", errProvenance, found.Code, found.Symbol, found.Path)
		}
		rows = append(rows, wireRow{Code: found.Code, Symbol: found.Symbol, Path: &found.Path, Reason: &reason})
	}
	for i := range others {
		rows = append(rows, wireRow{Code: others[i].Code, Symbol: others[i].Symbol, Path: others[i].Path, Reason: others[i].Reason})
	}
	slices.SortStableFunc(rows, func(a, b wireRow) int { return cmp.Compare(schemeOf(a.Symbol), schemeOf(b.Symbol)) })

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(wireBaseline{Description: baselineDescription, Baseline: rows}); err != nil {
		return fmt.Errorf("suppress: write %s: %w", BaselineFileName, err)
	}
	return nil
}

// schemeOf is a reference's scheme name.
func schemeOf(symbol string) string {
	name, _, _ := strings.Cut(symbol, "://")
	return name
}

// wireBaseline is the written document, its members in the order written.
type wireBaseline struct {
	Description string    `json:"description"`
	Baseline    []wireRow `json:"baseline"`
}

// wireRow is one written row, its members in the order an entry writes them.
//
//nolint:govet // fieldalignment: the field order is the member order the encoding writes
type wireRow struct {
	Code   vocab.Code `json:"code"`
	Symbol string     `json:"symbol"`
	Path   *string    `json:"path,omitempty"`
	Reason *string    `json:"reason,omitempty"`
}
