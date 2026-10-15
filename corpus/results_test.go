package corpus

import (
	"errors"
	"slices"
	"strings"
	"testing"

	spec "github.com/cplieger/deadset-spec/v8"
)

// The fixture the omission plants use, and the two subjects they need from it:
// one an analyzer must report, and one that exercises no capability and so admits
// no declared gap. A setup check names each, so a corpus change fails there
// rather than making a plant vacuous.
const (
	omissionFixture   = "unused-exported-consumer"
	omissionLanguage  = "go"
	omissionReported  = "DeadExport"
	omissionCode      = "DS1001"
	omissionUnreached = "UsedByConsumer"
)

func corpusFixtures(t *testing.T) []Fixture {
	t.Helper()
	fixtures, err := Load(spec.Corpus)
	if err != nil {
		t.Fatalf("Setup: Load: %v", err)
	}
	return fixtures
}

func corpusVersion(t *testing.T) string {
	t.Helper()
	doc, err := ReadDocument(spec.Corpus)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	return doc.CorpusVersion
}

// answeredResults builds the complete fixture and symbol inventory for one
// language that unansweredExpectations checks. Actual values and suppression
// semantics are outside that check. Each plant below changes one inventory or
// gap-coverage fact, so every failure has one cause.
func answeredResults(t *testing.T, fixtures []Fixture, language string) Results {
	t.Helper()
	doc := Results{
		CorpusVersion: corpusVersion(t),
		Product:       Product{Name: "deadset-" + language, Version: "1.0.0", Language: language},
		Result:        "pass",
	}
	for i := range fixtures {
		fixture := &fixtures[i].Expect
		if !slices.Contains(fixture.Languages, language) {
			continue
		}
		result := FixtureResult{Fixture: fixture.Name, Result: "pass", Unexpected: []Unexpected{}}
		for _, want := range fixture.Expect {
			row := ExpectationResult{
				Symbol: want.Symbol,
				Result: "pass",
				Actual: Answer{
					Report:            want.Report,
					SymbolKind:        want.SymbolKind,
					Confidence:        want.Confidence,
					ReachabilityClass: want.ReachabilityClass,
					LivenessRelation:  want.LivenessRelation,
					Fixability:        want.Fixability,
					Configurations:    want.Configurations,
					RetainedBy:        want.RetainedBy,
				},
			}
			if want.Report != ReportNone {
				row.Suppression = &Suppression{Suppressed: true}
			}
			result.Expectations = append(result.Expectations, row)
		}
		doc.Fixtures = append(doc.Fixtures, result)
	}
	if len(doc.Fixtures) == 0 {
		t.Fatalf("Setup: the corpus selects no fixture for language %q, want at least one", language)
	}
	doc.Totals = Totals{Fixtures: len(doc.Fixtures), Pass: len(doc.Fixtures)}
	return doc
}

// withFixtures returns a copy of doc carrying the fixture rows given, so a
// plant replaces the row list without touching the document it came from.
func withFixtures(doc Results, rows []FixtureResult) Results {
	out := doc
	out.Fixtures = rows
	return out
}

// mapFixture returns a copy of doc with the named fixture's row replaced by
// what change makes of a copy of it.
func mapFixture(doc Results, fixture string, change func(*FixtureResult)) Results {
	rows := slices.Clone(doc.Fixtures)
	for i := range rows {
		if rows[i].Fixture != fixture {
			continue
		}
		rows[i].Expectations = slices.Clone(rows[i].Expectations)
		change(&rows[i])
	}
	return withFixtures(doc, rows)
}

// withoutExpectation removes one expectation row: the silent omission the
// corpus exists to catch.
func withoutExpectation(doc Results, fixture, symbol string) Results {
	return mapFixture(doc, fixture, func(row *FixtureResult) {
		row.Expectations = slices.DeleteFunc(row.Expectations, func(e ExpectationResult) bool {
			return e.Symbol == symbol
		})
	})
}

// withoutFixture removes one fixture row.
func withoutFixture(doc Results, fixture string) Results {
	rows := slices.DeleteFunc(slices.Clone(doc.Fixtures), func(row FixtureResult) bool {
		return row.Fixture == fixture
	})
	return withFixtures(doc, rows)
}

// withGapAnswer reports one expectation as a gap under the capability given,
// which is what a product writes for a capability it declines.
func withGapAnswer(doc Results, fixture, symbol, capability string) Results {
	return mapFixture(doc, fixture, func(row *FixtureResult) {
		row.Result = Gap
		for i := range row.Expectations {
			if row.Expectations[i].Symbol != symbol {
				continue
			}
			row.Expectations[i].Result = Gap
			row.Expectations[i].Capability = capability
			row.Expectations[i].Actual = Answer{Report: ReportNone}
			row.Expectations[i].Suppression = nil
		}
	})
}

// withSurplusAnswer adds a row for a symbol no expectation names.
func withSurplusAnswer(doc Results, fixture, symbol string) Results {
	return mapFixture(doc, fixture, func(row *FixtureResult) {
		row.Expectations = append(row.Expectations, ExpectationResult{
			Symbol: symbol,
			Result: "pass",
			Actual: Answer{Report: ReportNone},
		})
	})
}

// omissionRows returns the two expectation rows the omission plants need from
// the fixture they name, failing the test when the corpus no longer holds
// them in that shape.
func omissionRows(t *testing.T, fixtures []Fixture) (reported, unreached Row) {
	t.Helper()
	var doc *Expectation
	for i := range fixtures {
		if fixtures[i].Name == omissionFixture {
			doc = &fixtures[i].Expect
		}
	}
	if doc == nil {
		t.Fatalf("Setup: the corpus holds no fixture %q, which the omission plants name", omissionFixture)
	}
	if !slices.Contains(doc.Languages, omissionLanguage) {
		t.Fatalf("Setup: fixture %q lists languages %v, want it to list %q", omissionFixture, doc.Languages, omissionLanguage)
	}
	for _, row := range doc.Expect {
		switch {
		case row.Symbol == omissionReported && row.Report == omissionCode:
			reported = row
		case row.Symbol == omissionUnreached && row.Report == ReportNone && len(row.RetainedBy) == 0:
			unreached = row
		}
	}
	if reported.Symbol == "" || unreached.Symbol == "" {
		t.Fatalf("Setup: fixture %q expects %q under %s and %q under none with no retained_by, and holds %+v", omissionFixture, omissionReported, omissionCode, omissionUnreached, doc.Expect)
	}
	return reported, unreached
}

func TestCorpusResultsAnswerEveryExpectation(t *testing.T) {
	fixtures := corpusFixtures(t)
	reported, unreached := omissionRows(t, fixtures)
	complete := answeredResults(t, fixtures, omissionLanguage)
	if got := Unanswered(fixtures, &complete, nil); len(got) != 0 {
		t.Fatalf("Unanswered(a complete %s results document) = %v, want nil so every plant below has one cause", omissionLanguage, got)
	}

	// A fixture the corpus does not select for this language, which a product
	// answering it names a rendering it must not have loaded.
	unselected := ""
	for _, fixture := range fixtures {
		if !slices.Contains(fixture.Expect.Languages, omissionLanguage) {
			unselected = fixture.Name
		}
	}
	if unselected == "" {
		t.Fatalf("Setup: every corpus fixture lists %q, want one that does not", omissionLanguage)
	}

	declared := GapRow{Fixture: omissionFixture, Capability: omissionCode, Reason: "Unused exported declarations are not reported yet."}
	cases := []struct {
		wantErr  error
		name     string
		results  Results
		gaps     []GapRow
		wantMsg  []string
		wantNone bool
	}{
		{
			name:    "expectation_omitted_with_no_gap",
			results: withoutExpectation(complete, omissionFixture, reported.Symbol),
			wantErr: ErrExpectationUnanswered,
			wantMsg: []string{omissionFixture, reported.Symbol, omissionCode},
		},
		{
			name:     "expectation_omitted_under_a_declared_gap",
			results:  withoutExpectation(complete, omissionFixture, reported.Symbol),
			gaps:     []GapRow{declared},
			wantNone: true,
		},
		{
			name:    "expectation_omitted_under_a_gap_for_another_symbol",
			results: withoutExpectation(complete, omissionFixture, reported.Symbol),
			gaps:    []GapRow{{Fixture: omissionFixture, Symbol: "AnotherSubject", Capability: omissionCode, Reason: declared.Reason}},
			wantErr: ErrExpectationUnanswered,
			wantMsg: []string{omissionFixture, reported.Symbol},
		},
		{
			name:    "expectation_exercising_no_capability_omitted_under_a_gap",
			results: withoutExpectation(complete, omissionFixture, unreached.Symbol),
			gaps:    []GapRow{declared},
			wantErr: ErrExpectationUnanswered,
			wantMsg: []string{omissionFixture, unreached.Symbol},
		},
		{
			name:    "fixture_omitted",
			results: withoutFixture(complete, omissionFixture),
			wantErr: ErrFixtureUnanswered,
			wantMsg: []string{omissionFixture, omissionLanguage},
		},
		{
			name:    "fixture_the_corpus_does_not_select",
			results: withFixtures(complete, append(slices.Clone(complete.Fixtures), FixtureResult{Fixture: unselected, Result: "pass"})),
			wantErr: ErrFixtureNotSelected,
			wantMsg: []string{unselected, omissionLanguage},
		},
		{
			name:    "symbol_the_fixture_does_not_expect",
			results: withSurplusAnswer(complete, omissionFixture, "NeverExpected"),
			wantErr: ErrExpectationUnexpected,
			wantMsg: []string{omissionFixture, "NeverExpected"},
		},
		{
			name:    "gap_reported_with_no_declaration",
			results: withGapAnswer(complete, omissionFixture, reported.Symbol, omissionCode),
			wantErr: ErrGapUndeclared,
			wantMsg: []string{omissionFixture, reported.Symbol, omissionCode},
		},
		{
			name:    "gap_reported_under_another_capability",
			results: withGapAnswer(complete, omissionFixture, reported.Symbol, "interface-satisfaction"),
			gaps:    []GapRow{declared},
			wantErr: ErrGapUndeclared,
			wantMsg: []string{omissionFixture, reported.Symbol, "interface-satisfaction"},
		},
		{
			name:     "gap_reported_under_its_declaration",
			results:  withGapAnswer(complete, omissionFixture, reported.Symbol, omissionCode),
			gaps:     []GapRow{declared},
			wantNone: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Unanswered(fixtures, &tc.results, tc.gaps)
			if tc.wantNone {
				if len(got) != 0 {
					t.Errorf("Unanswered(%s) = %v, want nil", tc.name, got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("Unanswered(%s) = %v, want one error wrapping %v", tc.name, got, tc.wantErr)
			}
			if !errors.Is(got[0], tc.wantErr) {
				t.Errorf("Unanswered(%s) = %v, want it to wrap %v", tc.name, got[0], tc.wantErr)
			}
			for _, want := range tc.wantMsg {
				if !strings.Contains(got[0].Error(), want) {
					t.Errorf("Unanswered(%s) = %q, want it to name %s", tc.name, got[0], want)
				}
			}
		})
	}
}
