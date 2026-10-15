package report

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// Input is what one analysis answered, from which [Build] assembles its report.
type Input struct {
	// Members is what each component lists, by its identifier, where the run
	// lists components in full, and nil where it does not.
	Members map[string][]Positioned

	Target                 Target
	Analyzer               Analyzer
	Configurations         []Configuration
	ConfigurationsNotBuilt []ConfigurationNotBuilt
	Loaded                 []LoadedConsumer
	Unavailable            []UnavailableConsumer

	// Findings is every finding the run reports, the stale suppressions among
	// them under their own code: Build moves each into the record the report
	// declares for it.
	Findings        []Finding
	EdgeEvaluations []EdgeEvaluation
	DeclaredGaps    []DeclaredGap
	ExcludedByCgo   []string
	TestFileRules   []TestFileRule
	TypeErrorSkips  []TypeErrorSkip
	Notes           []Note

	UnansweredQuestions []UnansweredQuestion
	ConventionsApplied  []ConventionApplied

	SuppressionsInEffect int
	ReasonsRecorded      int
	Withheld             Withheld
}

// Build assembles the report of one analysis at the schema and Contract
// versions this module states: every array in canonical order and none nil,
// the consumer roles and count, the components listing over the findings and
// then the edge evaluations' findings, and every total. It refuses a
// configuration named as built and as not built, the one rule between two
// arrays a reader of the report could not otherwise tell is broken; [Encode]
// refuses the rest of what the schemas refuse.
func Build(in *Input) (*Report, error) {
	for i := range in.ConfigurationsNotBuilt {
		dropped := in.ConfigurationsNotBuilt[i].ID
		if slices.ContainsFunc(in.Configurations, func(c Configuration) bool { return c.ID == dropped }) {
			return nil, fmt.Errorf("%w: the configuration %s is named as built and as not built", ErrInvalid, dropped)
		}
	}
	findings, stale := partition(in.Findings)
	r := &Report{
		SchemaVersion:   vocab.SchemaVersion,
		ContractVersion: vocab.ContractVersion,
		Analyzer:        in.Analyzer,
		Target:          in.Target,
		Configurations:  listed(in.Configurations),
		Consumers: Consumers{
			Declared:    len(in.Loaded) + len(in.Unavailable),
			Loaded:      listed(in.Loaded),
			Unavailable: listed(in.Unavailable),
		},
		ConfigurationsNotBuilt: listed(in.ConfigurationsNotBuilt),
		Findings:               findings,
		EdgeEvaluations:        listed(in.EdgeEvaluations),
		StaleSuppressions:      stale,
		DeclaredGaps:           listed(in.DeclaredGaps),
		ExcludedByCgo:          listed(in.ExcludedByCgo),
		TestFileRules:          listed(in.TestFileRules),
		TypeErrorSkips:         listed(in.TypeErrorSkips),
		Notes:                  listed(in.Notes),
		UnansweredQuestions:    listed(in.UnansweredQuestions),
		ConventionsApplied:     listed(in.ConventionsApplied),
	}
	r.Analyzer.SchemaVersionsAccepted = vocab.SchemaVersions()
	for i := range r.Consumers.Loaded {
		r.Consumers.Loaded[i].Role = consumerRole
	}
	for i := range r.Consumers.Unavailable {
		r.Consumers.Unavailable[i].Role = consumerRole
	}
	Order(r)
	if in.Members != nil {
		r.Components = r.listing(in.Members)
	}
	Recount(r)
	r.Totals.SuppressionsInEffect = in.SuppressionsInEffect
	r.Totals.ReasonsRecorded = in.ReasonsRecorded
	r.Totals.Withheld = in.Withheld
	for i := range r.EdgeEvaluations {
		if r.EdgeEvaluations[i].State == StateDead {
			r.Totals.Pending++
		}
	}
	return r, nil
}

// consumerRole is the one role the schema admits on a consumer entry.
const consumerRole = "consumer"

// partition moves every stale-suppression finding out of findings into the
// record of its own the report declares for it.
func partition(findings []Finding) ([]Finding, []StaleSuppression) {
	kept := make([]Finding, 0, len(findings))
	stale := []StaleSuppression{}
	for i := range findings {
		found := &findings[i]
		if found.Code != vocab.StaleSuppression {
			kept = append(kept, *found)
			continue
		}
		record := StaleSuppression{
			Code:      found.Code,
			Mechanism: found.Details.Mechanism,
			Position:  SuppressionPosition{Path: found.Position.Path, Line: found.Position.Line, Column: found.Position.Column},
			Symbol:    found.Symbol.Ref,
			Message:   found.Message,
		}
		if entry := found.Details.Entry; entry != nil {
			record.Entry = SuppressionEntry{Code: entry.Code, Symbol: entry.Symbol, Path: entry.Path, Reason: entry.Reason}
		}
		stale = append(stale, record)
	}
	return kept, stale
}

// listed is a copy of items, empty rather than nil, because the schemas admit
// no absent array.
func listed[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return slices.Clone(items)
}

// Recount sets the counts of r's totals its records decide: the findings, the
// findings per severity, the deletable lines of every root component a
// finding names counted once per component, and the stale suppressions.
func Recount(r *Report) {
	r.Totals.Findings = len(r.Findings)
	r.Totals.BySeverity = BySeverity{}
	r.Totals.DeletableLines = 0
	counted := map[string]bool{}
	for i := range r.Findings {
		found := &r.Findings[i]
		switch found.Severity {
		case vocab.Allow:
			r.Totals.BySeverity.Allow++
		case vocab.Warn:
			r.Totals.BySeverity.Warn++
		case vocab.Deny:
			r.Totals.BySeverity.Deny++
		}
		if component := &found.Component; component.Root && !counted[component.ID] {
			counted[component.ID] = true
			r.Totals.DeletableLines += component.DeletableLines
		}
	}
	r.Totals.StaleSuppressions = len(r.StaleSuppressions)
}

// BySize is a copy of findings ordered by the lines deleting each one's
// component removes, largest first, then by the size of its subject, then
// canonically: the order reporters.sort size renders findings in.
func BySize(findings []Finding) []Finding {
	held := slices.Clone(findings)
	slices.SortStableFunc(held, func(a, b Finding) int {
		return cmp.Or(
			cmp.Compare(b.Component.DeletableLines, a.Component.DeletableLines),
			cmp.Compare(b.Symbol.SizeLines, a.Symbol.SizeLines),
			CompareFindings(&a, &b),
		)
	})
	return held
}

// Cap keeps the first most findings, of the size order where bySize holds and
// of the canonical order otherwise, counts the rest as omitted, and keeps the
// findings in canonical order and the components listing to what they name.
// Every other total keeps describing the whole set. A most of zero or less
// keeps every finding.
func Cap(r *Report, most int, bySize bool) {
	if most <= 0 || len(r.Findings) <= most {
		return
	}
	kept := r.Findings
	if bySize {
		kept = BySize(kept)
	}
	kept = slices.Clone(kept[:most])
	SortTotal(kept, CompareFindings)
	r.Totals.Omitted += len(r.Findings) - most
	r.Findings = kept
	if r.Components != nil {
		r.Components = r.listing(r.Members())
	}
}
