package report

import (
	"cmp"
	"slices"
	"strings"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// Order puts every array of r in the order contract/grammar/merge.md fixes for
// it, each tie broken by the bytewise order of the records' compact encodings,
// so the arrays do not depend on the order they were built in. The arrays the
// grammar holds to one entry per distinct entry lose their repeats, and the
// components listing follows the findings that name its entries.
func Order(r *Report) {
	byString := func(a, b *string) int { return strings.Compare(*a, *b) }
	r.Analyzer.Languages = Distinct(r.Analyzer.Languages, func(a, b *vocab.Language) int { return strings.Compare(string(*a), string(*b)) })
	slices.SortStableFunc(r.MergedFrom, func(a, b InputReport) int { return strings.Compare(a.Name, b.Name) })
	slices.SortStableFunc(r.Configurations, func(a, b Configuration) int { return strings.Compare(a.ID, b.ID) })
	slices.SortStableFunc(r.ConfigurationsNotBuilt, func(a, b ConfigurationNotBuilt) int { return strings.Compare(a.ID, b.ID) })
	slices.SortStableFunc(r.Consumers.Loaded, func(a, b LoadedConsumer) int { return strings.Compare(a.ID, b.ID) })
	slices.SortStableFunc(r.Consumers.Unavailable, func(a, b UnavailableConsumer) int { return strings.Compare(a.ID, b.ID) })
	SortTotal(r.Findings, CompareFindings)
	SortTotal(r.StaleSuppressions, func(a, b *StaleSuppression) int {
		return compareKeys(a.Position.Path, a.Position.Line, a.Position.Column, a.Code, a.Symbol, a.Analyzer,
			b.Position.Path, b.Position.Line, b.Position.Column, b.Code, b.Symbol, b.Analyzer)
	})
	SortTotal(r.DeclaredGaps, func(a, b *DeclaredGap) int { return strings.Compare(a.Analyzer, b.Analyzer) })
	SortTotal(r.EdgeEvaluations, CompareEvaluations)
	r.ExcludedByCgo = Distinct(r.ExcludedByCgo, byString)
	r.TestFileRules = Distinct(r.TestFileRules, func(a, b *TestFileRule) int { return strings.Compare(a.Rule, b.Rule) })
	r.TypeErrorSkips = Distinct(r.TypeErrorSkips, func(a, b *TypeErrorSkip) int {
		return cmp.Or(strings.Compare(a.Path, b.Path), cmp.Compare(a.Line, b.Line))
	})
	r.Notes = Distinct(r.Notes, func(a, b *Note) int {
		return cmp.Or(strings.Compare(string(a.Kind), string(b.Kind)), strings.Compare(a.Path, b.Path))
	})
	r.UnansweredQuestions = Distinct(r.UnansweredQuestions, func(a, b *UnansweredQuestion) int {
		return strings.Compare(a.Configuration, b.Configuration)
	})
	r.ConventionsApplied = Distinct(r.ConventionsApplied, func(a, b *ConventionApplied) int {
		return cmp.Or(strings.Compare(a.Name, b.Name), strings.Compare(a.Manifest, b.Manifest))
	})
	if r.Components != nil {
		r.Components = r.listing(r.Members())
	}
}

// CompareEvaluations orders two edge evaluations by edge, side and the
// carrying analyzer's name.
func CompareEvaluations(a, b *EdgeEvaluation) int {
	return cmp.Or(strings.Compare(a.Edge, b.Edge), strings.Compare(string(a.Side), string(b.Side)), strings.Compare(a.Analyzer, b.Analyzer))
}

// CompareFindings orders two findings by the canonical key: path, line,
// column, code, symbol reference and the carrying analyzer's name.
func CompareFindings(a, b *Finding) int {
	return compareKeys(a.Position.Path, a.Position.Line, a.Position.Column, a.Code, a.Symbol.Ref, a.Analyzer,
		b.Position.Path, b.Position.Line, b.Position.Column, b.Code, b.Symbol.Ref, b.Analyzer)
}

// ComparePositioned orders two component members, or any two positioned
// symbols, by the canonical key read from each one's position and reference.
func ComparePositioned(a, b *Positioned) int {
	return cmp.Or(
		strings.Compare(a.Position.Path, b.Position.Path), cmp.Compare(a.Position.Line, b.Position.Line),
		cmp.Compare(a.Position.Column, b.Position.Column), strings.Compare(a.Ref, b.Ref),
	)
}

func compareKeys(pathA string, lineA, columnA int, codeA vocab.Code, refA, analyzerA string,
	pathB string, lineB, columnB int, codeB vocab.Code, refB, analyzerB string,
) int {
	return cmp.Or(
		strings.Compare(pathA, pathB), cmp.Compare(lineA, lineB), cmp.Compare(columnA, columnB),
		strings.Compare(string(codeA), string(codeB)), strings.Compare(refA, refB), strings.Compare(analyzerA, analyzerB),
	)
}

// listing is the listing of every component the findings and then the edge
// evaluations' findings name, once, in the order of the first naming it, each
// holding members' entry for it.
func (r *Report) listing(members map[string][]Positioned) []ComponentListing {
	listed := []ComponentListing{}
	seen := map[string]bool{}
	for _, id := range r.componentsNamed() {
		if held, ok := members[id]; ok && !seen[id] {
			seen[id] = true
			listed = append(listed, ComponentListing{ID: id, Members: held})
		}
	}
	return listed
}

type encoded[T any] struct {
	record   T
	encoding string
}

// SortTotal sorts records by compare and then by their compact encodings, so
// the order does not depend on the order they arrived in.
func SortTotal[T any](records []T, compare func(a, b *T) int) {
	held := sortEncoded(records, compare)
	for i := range held {
		records[i] = held[i].record
	}
}

// Distinct is records in the order compare and then their compact encodings
// give, each record whose encoding repeats another's appearing once. An empty
// or nil input is an empty list.
func Distinct[T any](records []T, compare func(a, b *T) int) []T {
	held := slices.CompactFunc(sortEncoded(records, compare), func(a, b encoded[T]) bool { return a.encoding == b.encoding })
	kept := make([]T, len(held))
	for i := range held {
		kept[i] = held[i].record
	}
	return kept
}

func sortEncoded[T any](records []T, compare func(a, b *T) int) []encoded[T] {
	held := make([]encoded[T], len(records))
	for i := range records {
		held[i] = encoded[T]{record: records[i], encoding: Compact(&records[i])}
	}
	slices.SortFunc(held, func(a, b encoded[T]) int {
		return cmp.Or(compare(&a.record, &b.record), strings.Compare(a.encoding, b.encoding))
	})
	return held
}

// Compact is a record's compact encoding, the bytes the canonical order's last
// tiebreak compares. A record of a report always encodes.
func Compact(v any) string {
	written, err := marshal(v)
	if err != nil {
		panic("report: a record does not encode: " + err.Error())
	}
	return string(written)
}
