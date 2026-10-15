package corpus

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
)

// The results a fixture and an expectation end as.
const (
	Pass    = "pass"
	Gap     = "gap"
	Fail    = "fail"
	Pending = "pending"
)

// Gaps is the conformance.json a product commits: the capabilities it declines.
type Gaps struct {
	CorpusVersion string     `json:"corpus_version"`
	Product       GapProduct `json:"product"`
	Gaps          []GapRow   `json:"gaps"`
}

// GapProduct is the product a gaps document belongs to.
type GapProduct struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// GapRow is one declined capability, for one fixture and optionally one symbol.
type GapRow struct {
	Fixture    string `json:"fixture"`
	Symbol     string `json:"symbol,omitempty"`
	Capability string `json:"capability"`
	Reason     string `json:"reason"`
}

// covering returns the capability a declared gap covers for one row of one
// fixture, or "".
func covering(gaps []GapRow, fixture string, row *Row) string {
	exercised := Capabilities(row)
	for _, gap := range gaps {
		if gap.Fixture == fixture && (gap.Symbol == "" || gap.Symbol == row.Symbol) && slices.Contains(exercised, gap.Capability) {
			return gap.Capability
		}
	}
	return ""
}

// coversFixture reports whether a fixture-level declared gap names the capability.
func coversFixture(gaps []GapRow, fixture, capability string) bool {
	return slices.ContainsFunc(gaps, func(gap GapRow) bool {
		return gap.Fixture == fixture && gap.Symbol == "" && gap.Capability == capability
	})
}

// Capabilities is what one expectation exercises: its report code when that is
// not none, and every class in its retained_by.
func Capabilities(row *Row) []string {
	var out []string
	if row.Report != ReportNone && row.Report != "" {
		out = append(out, row.Report)
	}
	return append(out, row.RetainedBy...)
}

// Results is the conformance-results.json a run writes.
type Results struct { //nolint:govet // the members in the results schema's order, which Encode writes
	CorpusVersion string          `json:"corpus_version"`
	Product       Product         `json:"product"`
	Result        string          `json:"result"`
	Totals        Totals          `json:"totals"`
	Fixtures      []FixtureResult `json:"fixtures"`
}

// Product is the product that ran and the language whose renderings it answered.
type Product struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Language string `json:"language"`
}

// Totals is the fixture count by result.
type Totals struct {
	Fixtures int `json:"fixtures"`
	Pass     int `json:"pass"`
	Gap      int `json:"gap"`
	Fail     int `json:"fail"`
	Pending  int `json:"pending"`
}

// FixtureResult is one fixture's row.
type FixtureResult struct { //nolint:govet // the members in the results schema's order, which Encode writes
	Fixture      string              `json:"fixture"`
	Result       string              `json:"result"`
	Expectations []ExpectationResult `json:"expectations"`
	Unexpected   []Unexpected        `json:"unexpected"`
	Message      string              `json:"message,omitempty"`
}

// ExpectationResult is one expectation's row.
type ExpectationResult struct {
	Symbol      string       `json:"symbol"`
	Result      string       `json:"result"`
	Capability  string       `json:"capability,omitempty"`
	Actual      Answer       `json:"actual"`
	Suppression *Suppression `json:"suppression,omitempty"`
	Message     string       `json:"message,omitempty"`
}

// Answer is what the analyzer reported for one expectation, carrying only the
// members the row names, in the expectation file's vocabulary.
type Answer struct { //nolint:govet // the members in the results schema's order, which Encode writes
	Report            string     `json:"report"`
	SymbolKind        string     `json:"symbol_kind,omitempty"`
	Confidence        string     `json:"confidence,omitempty"`
	ReachabilityClass string     `json:"reachability_class,omitempty"`
	LivenessRelation  string     `json:"liveness_relation,omitempty"`
	Fixability        string     `json:"fixability,omitempty"`
	Configurations    []string   `json:"configurations,omitempty"`
	RetainedBy        []string   `json:"retained_by,omitempty"`
	Details           *Details   `json:"details,omitempty"`
	Component         *Component `json:"component,omitempty"`
}

// Suppression is the second phase's outcome for one row.
type Suppression struct {
	Suppressed bool `json:"suppressed"`
	Stale      bool `json:"stale"`
}

// Unexpected is one finding or record no expectation names, by the rendering's path.
type Unexpected struct { //nolint:govet // the members in the results schema's order, which Encode writes
	File   string `json:"file"`
	Line   int    `json:"line"`
	Report string `json:"report"`
}

// Encode renders a results document as the products commit it: two-space
// indentation in the schema's member order and a final newline.
func Encode(results *Results) ([]byte, error) {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// tally sets the totals and the overall result from the fixture rows.
func (r *Results) tally() {
	r.Totals = Totals{Fixtures: len(r.Fixtures)}
	for _, row := range r.Fixtures {
		switch row.Result {
		case Pass:
			r.Totals.Pass++
		case Gap:
			r.Totals.Gap++
		case Fail:
			r.Totals.Fail++
		case Pending:
			r.Totals.Pending++
		}
	}
	r.Result = Pass
	if r.Totals.Fail > 0 {
		r.Result = Fail
	}
}

// The ways a results document and the corpus disagree about what was answered.
var (
	ErrFixtureUnanswered     = errors.New("results omit a fixture the corpus selects for the product's language")
	ErrFixtureNotSelected    = errors.New("results answer a fixture the corpus does not select for the product's language")
	ErrUnknownFixture        = errors.New("results name a fixture the corpus does not hold")
	ErrExpectationUnanswered = errors.New("results neither answer the expectation nor declare a gap for it")
	ErrExpectationUnexpected = errors.New("results answer a symbol the fixture does not expect")
	ErrGapUndeclared         = errors.New("results report a gap no declared gap covers")
)

// Unanswered reports every place a results document and the corpus disagree: a
// selected fixture the results omit, an answered fixture the corpus does not
// select or hold, an expectation neither answered nor covered by a declared gap,
// a gap no declared gap covers, and a row for a symbol the fixture does not expect.
func Unanswered(fixtures []Fixture, results *Results, gaps []GapRow) []error {
	answered := make(map[string]*FixtureResult, len(results.Fixtures))
	for i := range results.Fixtures {
		answered[results.Fixtures[i].Fixture] = &results.Fixtures[i]
	}
	var errs []error
	held := map[string]bool{}
	for i := range fixtures {
		doc := &fixtures[i].Expect
		held[doc.Name] = true
		selected := slices.Contains(doc.Languages, results.Product.Language)
		got, ok := answered[doc.Name]
		switch {
		case selected && !ok:
			errs = append(errs, fmt.Errorf("%w: %s, language %q", ErrFixtureUnanswered, doc.Name, results.Product.Language))
		case !selected && ok:
			errs = append(errs, fmt.Errorf("%w: %s lists %v, and the product answered %q", ErrFixtureNotSelected, doc.Name, doc.Languages, results.Product.Language))
		case selected:
			errs = append(errs, answeredFixture(doc, got, gaps)...)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(answered)) {
		if !held[name] {
			errs = append(errs, fmt.Errorf("%w: %q", ErrUnknownFixture, name))
		}
	}
	return errs
}

func answeredFixture(doc *Expectation, got *FixtureResult, gaps []GapRow) []error {
	rows := make(map[string]*ExpectationResult, len(got.Expectations))
	for i := range got.Expectations {
		rows[got.Expectations[i].Symbol] = &got.Expectations[i]
	}
	var errs []error
	for i := range doc.Expect {
		want := &doc.Expect[i]
		row, answered := rows[want.Symbol]
		delete(rows, want.Symbol)
		covered := declaredCapabilities(gaps, doc.Name, want)
		switch {
		case !answered && len(covered) == 0:
			errs = append(errs, fmt.Errorf("%w: %s %s exercises %v", ErrExpectationUnanswered, doc.Name, want.Symbol, Capabilities(want)))
		case answered && row.Result == Gap && !slices.Contains(covered, row.Capability):
			errs = append(errs, fmt.Errorf("%w: %s %s claims capability %q, and the declared gaps cover %v", ErrGapUndeclared, doc.Name, want.Symbol, row.Capability, covered))
		}
	}
	for _, symbol := range slices.Sorted(maps.Keys(rows)) {
		errs = append(errs, fmt.Errorf("%w: %s %s", ErrExpectationUnexpected, doc.Name, symbol))
	}
	return errs
}

// declaredCapabilities is every capability under which a declared gap covers one
// expectation of one fixture.
func declaredCapabilities(gaps []GapRow, fixture string, want *Row) []string {
	exercised := Capabilities(want)
	var covered []string
	for _, gap := range gaps {
		if gap.Fixture != fixture || (gap.Symbol != "" && gap.Symbol != want.Symbol) {
			continue
		}
		if slices.Contains(exercised, gap.Capability) && !slices.Contains(covered, gap.Capability) {
			covered = append(covered, gap.Capability)
		}
	}
	return covered
}
