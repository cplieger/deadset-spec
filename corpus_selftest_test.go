package spec_test

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v2"
)

const (
	// expectSchemaRowPath is the path into the expectation schema of one
	// expectation row, which declares two of the closed vocabularies below.
	expectSchemaRowPath = "properties/expect/items"

	// expectSchemaDetailsPath is the path into the expectation schema of the
	// details object a row pins, whose members mirror the details members
	// contract/finding.schema.json declares.
	expectSchemaDetailsPath = expectSchemaRowPath + "/properties/details"

	// resultsSchemaActualPath is the path into the results schema of the answer
	// a runner records for one expectation, which echoes the row's own members.
	resultsSchemaActualPath = "properties/fixtures/items/properties/expectations/items/properties/actual"

	// gapResult is the result value that stands for a declared gap, and
	// reportedNone the report value for a subject an analyzer must not report.
	gapResult    = "gap"
	reportedNone = "none"

	// The fixture the omission plants below use, and the two subjects they
	// need from it: one an analyzer must report, and one that exercises no
	// capability and so admits no declared gap. A setup check names each, so a
	// corpus change fails there rather than making a plant vacuous.
	omissionFixture   = "unused-exported-consumer"
	omissionLanguage  = "go"
	omissionReported  = "DeadExport"
	omissionCode      = "DS1001"
	omissionUnreached = "UsedByConsumer"
)

// The corpus fails itself rather than an analyzer for each of these, so every
// error wraps the rule it broke and names the fixture and the subject it
// caught.
var (
	errOutsideVocabulary     = errors.New("expectation names a value outside its closed vocabulary")
	errNoRenderingForm       = errors.New("fixture declares a language the corpus states no rendering form for")
	errRenderingFormAbsent   = errors.New("fixture declares a language whose rendering the fixture does not hold")
	errFixtureUnanswered     = errors.New("results omit a fixture the corpus selects for the product's language")
	errFixtureNotSelected    = errors.New("results answer a fixture the corpus does not select for the product's language")
	errExpectationUnanswered = errors.New("results neither answer the expectation nor declare a gap for it")
	errExpectationUnexpected = errors.New("results answer a symbol the fixture does not expect")
	errGapUndeclared         = errors.New("results report a gap no declared gap covers")
)

// declaredGap is one row of the conformance.json a product commits: the
// capability the product declines, and the expectations that declination
// covers.
type declaredGap struct {
	Fixture    string `json:"fixture"`
	Symbol     string `json:"symbol"`
	Capability string `json:"capability"`
	Reason     string `json:"reason"`
}

// expectationVocabularies are the closed value sets an expectation file draws
// from, each read from the document that declares it: the reachability classes
// and the languages from contract/kinds.json, which is where corpus.json's
// vocabularies block sends a reader for them, the symbol kinds from
// contract/finding.schema.json, which owns that vocabulary, and the target
// kinds and the liveness relations from the expectation schema, the only
// document that declares those two.
type expectationVocabularies struct {
	ReachabilityClasses []string
	Languages           []string
	SymbolKinds         []string
	TargetKinds         []string
	LivenessRelations   []string
}

// loadExpectationVocabularies reads the five value sets, failing the test when
// one is empty, because an empty set lets every value through.
func loadExpectationVocabularies(t *testing.T) expectationVocabularies {
	t.Helper()
	kinds := loadKinds(t)
	schema := loadSchema(t, spec.Corpus, expectSchemaPath)
	v := expectationVocabularies{
		ReachabilityClasses: kinds.ReachabilityClasses,
		Languages:           kinds.Languages,
		SymbolKinds:         enumAt(loadFindingSchema(t), "properties/symbol/properties/kind"),
		TargetKinds:         enumAt(schema, "properties/target_kind"),
		LivenessRelations:   enumAt(schema, expectSchemaRowPath+"/properties/liveness_relation"),
	}
	sets := map[string][]string{
		"reachability_classes": v.ReachabilityClasses,
		"languages":            v.Languages,
		"symbol_kind":          v.SymbolKinds,
		"target_kind":          v.TargetKinds,
		"liveness_relation":    v.LivenessRelations,
	}
	for _, name := range slices.Sorted(maps.Keys(sets)) {
		if len(sets[name]) == 0 {
			t.Fatalf("Setup: vocabulary %q is empty, want the values %s, %s and %s declare", name, kindsPath, findingSchemaPath, expectSchemaPath)
		}
	}
	return v
}

// checkExpectationVocabularies reports every value the expectation file names
// outside the closed vocabulary of its field. It covers the fields nothing
// else resolves: a report code resolves against contract/kinds.json in
// vocabulary_test.go and a retained_by class against contract/exemptions.json
// in exemptions_test.go, over these same fixtures. A symbol_kind resolves here
// against the vocabulary contract/finding.schema.json owns, and the same file's
// rule on the liveness relation is read from it in vocabulary_test.go. An
// absent optional value is not a violation; target_kind has no default, so an
// absent one is.
func checkExpectationVocabularies(doc *expectationDocument, v expectationVocabularies) []error {
	var errs []error
	value := func(subject, field, got string, want []string) {
		if !slices.Contains(want, got) {
			errs = append(errs, fmt.Errorf("%w: %s %s %s = %q, want one of %v", errOutsideVocabulary, doc.Name, subject, field, got, want))
		}
	}
	optional := func(subject, field, got string, want []string) {
		if got != "" {
			value(subject, field, got, want)
		}
	}
	value("fixture", "target_kind", doc.TargetKind, v.TargetKinds)
	for _, language := range doc.Languages {
		value("fixture", "languages", language, v.Languages)
	}
	for _, row := range doc.Expect {
		optional(row.Symbol, "symbol_kind", row.SymbolKind, v.SymbolKinds)
		optional(row.Symbol, "confidence", row.Confidence, v.ReachabilityClasses)
		optional(row.Symbol, "reachability_class", row.ReachabilityClass, v.ReachabilityClasses)
		optional(row.Symbol, "liveness_relation", row.LivenessRelation, v.LivenessRelations)
	}
	return errs
}

// checkRenderingForms reports every language the fixture declares that the
// corpus states no rendering form for, and every declared form the fixture
// directory does not hold. The forms come from the corpus document rather than
// from a list of this suite's own, so a language the contract gains is checked
// here the day corpus.json states its form.
func checkRenderingForms(fsys fs.FS, dir string, doc *expectationDocument, renderings map[string]string) []error {
	var errs []error
	for _, language := range doc.Languages {
		form, stated := renderings[language]
		if !stated {
			errs = append(errs, fmt.Errorf("%w: %s declares %q, and %s states a form for %v", errNoRenderingForm, doc.Name, language, corpusPath, slices.Sorted(maps.Keys(renderings))))
			continue
		}
		if _, err := fs.Stat(fsys, path.Join(dir, strings.TrimSuffix(form, "/"))); err != nil {
			errs = append(errs, fmt.Errorf("%w: %s declares %q, whose form is %s: %v", errRenderingFormAbsent, doc.Name, language, form, err))
		}
	}
	return errs
}

// expectationCapabilities lists what an expectation exercises, which is what a
// declared gap may name: its report code when that is not the literal none,
// and every class in its retained_by. An expectation reporting none with no
// retained_by exercises nothing, so no gap covers it and its absence is a
// failure whatever a product declares.
func expectationCapabilities(row expectationRow) []string {
	var capabilities []string
	if row.Report != reportedNone && row.Report != "" {
		capabilities = append(capabilities, row.Report)
	}
	return append(capabilities, row.RetainedBy...)
}

// declaredCapabilities lists the capabilities under which the declared gaps
// cover one expectation of one fixture: a gap counts when it names the
// fixture, names a capability the expectation exercises, and either names no
// symbol or names this expectation's.
func declaredCapabilities(gaps []declaredGap, fixture string, want expectationRow) []string {
	exercised := expectationCapabilities(want)
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

// unansweredExpectations reports every place a results document and the corpus
// disagree about what was answered: a fixture the corpus selects for the
// product's language that the results omit, a fixture the results answer that
// the corpus does not select for that language, an expectation no row answers
// and no declared gap covers, an expectation a row reports as a gap that no
// declared gap covers, and a row naming a symbol the fixture does not expect.
// A fixture the results name that the corpus does not hold at all is
// checkResultsFixtures's finding, not this one's.
func unansweredExpectations(fixtures []expectationDocument, results *resultsDocument, gaps []declaredGap) []error {
	answered := make(map[string]*fixtureResult, len(results.Fixtures))
	for i := range results.Fixtures {
		answered[results.Fixtures[i].Fixture] = &results.Fixtures[i]
	}
	var errs []error
	for i := range fixtures {
		doc := &fixtures[i]
		selected := slices.Contains(doc.Languages, results.Product.Language)
		got, ok := answered[doc.Name]
		switch {
		case selected && !ok:
			errs = append(errs, fmt.Errorf("%w: %s, language %q", errFixtureUnanswered, doc.Name, results.Product.Language))
		case !selected && ok:
			errs = append(errs, fmt.Errorf("%w: %s lists %v, and the product answered %q", errFixtureNotSelected, doc.Name, doc.Languages, results.Product.Language))
		case selected && ok:
			errs = append(errs, checkAnsweredFixture(doc, got, gaps)...)
		}
	}
	return errs
}

// checkAnsweredFixture pairs one fixture's expectations with the rows one
// results document holds for it, in the fixture's order, and reports the rows
// that are missing, unbacked or surplus.
func checkAnsweredFixture(doc *expectationDocument, got *fixtureResult, gaps []declaredGap) []error {
	rows := make(map[string]expectationResult, len(got.Expectations))
	for _, row := range got.Expectations {
		rows[row.Symbol] = row
	}
	var errs []error
	for _, want := range doc.Expect {
		row, answered := rows[want.Symbol]
		delete(rows, want.Symbol)
		covered := declaredCapabilities(gaps, doc.Name, want)
		switch {
		case !answered && len(covered) == 0:
			errs = append(errs, fmt.Errorf("%w: %s %s exercises %v", errExpectationUnanswered, doc.Name, want.Symbol, expectationCapabilities(want)))
		case answered && row.Result == gapResult && !slices.Contains(covered, row.Capability):
			errs = append(errs, fmt.Errorf("%w: %s %s claims capability %q, and the declared gaps cover %v", errGapUndeclared, doc.Name, want.Symbol, row.Capability, covered))
		}
	}
	for _, symbol := range slices.Sorted(maps.Keys(rows)) {
		errs = append(errs, fmt.Errorf("%w: %s %s", errExpectationUnexpected, doc.Name, symbol))
	}
	return errs
}

// corpusExpectations decodes every fixture's expectation file on the embedded
// corpus, in path order. There is at least one, because a check over an empty
// set passes by having nothing to read.
func corpusExpectations(t *testing.T) []expectationDocument {
	t.Helper()
	pattern := path.Join(fixturesDir, "*", expectFile)
	paths, err := fs.Glob(spec.Corpus, pattern)
	if err != nil {
		t.Fatalf("Setup: fs.Glob(Corpus, %s): %v", pattern, err)
	}
	if len(paths) == 0 {
		t.Fatalf("Setup: fs.Glob(Corpus, %s) matched nothing, want the corpus fixtures", pattern)
	}
	docs := make([]expectationDocument, 0, len(paths))
	for _, p := range paths {
		data, err := fs.ReadFile(spec.Corpus, p)
		if err != nil {
			t.Fatalf("Setup: fs.ReadFile(Corpus, %q): %v", p, err)
		}
		var doc expectationDocument
		if err = decodeStrict(data, &doc); err != nil {
			t.Fatalf("Setup: decoding %s: %v", p, err)
		}
		docs = append(docs, doc)
	}
	return docs
}

// answeredResults is the results document a conforming product writes: it
// answers every fixture the corpus selects for one language, exactly as the
// expectations state, with the suppression phase held for every reported
// finding. Each plant below carries one difference from it, so every failure
// has one cause.
func answeredResults(t *testing.T, fixtures []expectationDocument, language string) resultsDocument {
	t.Helper()
	doc := resultsDocument{
		CorpusVersion: loadCorpus(t).CorpusVersion,
		Product:       productIdentity{Name: "deadset-" + language, Version: "1.0.0", Language: language},
		Result:        "pass",
	}
	for _, fixture := range fixtures {
		if !slices.Contains(fixture.Languages, language) {
			continue
		}
		result := fixtureResult{Fixture: fixture.Name, Result: "pass", Unexpected: []unexpectedFinding{}}
		for _, want := range fixture.Expect {
			row := expectationResult{
				Symbol: want.Symbol,
				Result: "pass",
				Actual: answer{
					Report:            want.Report,
					SymbolKind:        want.SymbolKind,
					Confidence:        want.Confidence,
					ReachabilityClass: want.ReachabilityClass,
					LivenessRelation:  want.LivenessRelation,
					Configurations:    want.Configurations,
					RetainedBy:        want.RetainedBy,
				},
			}
			if want.Report != reportedNone {
				row.Suppression = &suppressionResult{Suppressed: true}
			}
			result.Expectations = append(result.Expectations, row)
		}
		doc.Fixtures = append(doc.Fixtures, result)
	}
	if len(doc.Fixtures) == 0 {
		t.Fatalf("Setup: the corpus selects no fixture for language %q, want at least one", language)
	}
	doc.Totals = resultTotals{Fixtures: len(doc.Fixtures), Pass: len(doc.Fixtures)}
	return doc
}

// withFixtures returns a copy of doc carrying the fixture rows given, so a
// plant replaces the row list without touching the document it came from.
func withFixtures(doc resultsDocument, rows []fixtureResult) resultsDocument {
	out := doc
	out.Fixtures = rows
	return out
}

// mapFixture returns a copy of doc with the named fixture's row replaced by
// what change makes of a copy of it.
func mapFixture(doc resultsDocument, fixture string, change func(*fixtureResult)) resultsDocument {
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
func withoutExpectation(doc resultsDocument, fixture, symbol string) resultsDocument {
	return mapFixture(doc, fixture, func(row *fixtureResult) {
		row.Expectations = slices.DeleteFunc(row.Expectations, func(e expectationResult) bool {
			return e.Symbol == symbol
		})
	})
}

// withoutFixture removes one fixture row.
func withoutFixture(doc resultsDocument, fixture string) resultsDocument {
	rows := slices.DeleteFunc(slices.Clone(doc.Fixtures), func(row fixtureResult) bool {
		return row.Fixture == fixture
	})
	return withFixtures(doc, rows)
}

// withGapAnswer reports one expectation as a gap under the capability given,
// which is what a product writes for a capability it declines.
func withGapAnswer(doc resultsDocument, fixture, symbol, capability string) resultsDocument {
	return mapFixture(doc, fixture, func(row *fixtureResult) {
		row.Result = gapResult
		for i := range row.Expectations {
			if row.Expectations[i].Symbol != symbol {
				continue
			}
			row.Expectations[i].Result = gapResult
			row.Expectations[i].Capability = capability
			row.Expectations[i].Actual = answer{Report: reportedNone}
			row.Expectations[i].Suppression = nil
		}
	})
}

// withSurplusAnswer adds a row for a symbol no expectation names.
func withSurplusAnswer(doc resultsDocument, fixture, symbol string) resultsDocument {
	return mapFixture(doc, fixture, func(row *fixtureResult) {
		row.Expectations = append(row.Expectations, expectationResult{
			Symbol: symbol,
			Result: "pass",
			Actual: answer{Report: reportedNone},
		})
	})
}

// omissionRows returns the two expectation rows the omission plants need from
// the fixture they name, failing the test when the corpus no longer holds
// them in that shape.
func omissionRows(t *testing.T, fixtures []expectationDocument) (reported, unreached expectationRow) {
	t.Helper()
	var doc *expectationDocument
	for i := range fixtures {
		if fixtures[i].Name == omissionFixture {
			doc = &fixtures[i]
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
		case row.Symbol == omissionUnreached && row.Report == reportedNone && len(row.RetainedBy) == 0:
			unreached = row
		}
	}
	if reported.Symbol == "" || unreached.Symbol == "" {
		t.Fatalf("Setup: fixture %q expects %q under %s and %q under none with no retained_by, and holds %+v", omissionFixture, omissionReported, omissionCode, omissionUnreached, doc.Expect)
	}
	return reported, unreached
}

func TestCorpusExpectationValuesDrawFromClosedVocabularies(t *testing.T) {
	vocabularies := loadExpectationVocabularies(t)

	for _, doc := range corpusExpectations(t) {
		t.Run("fixture_"+doc.Name, func(t *testing.T) {
			if got := checkExpectationVocabularies(&doc, vocabularies); len(got) != 0 {
				t.Errorf("checkExpectationVocabularies(%s) = %v, want every value inside its vocabulary", doc.Name, got)
			}
		})
	}

	// One planted document per field, each naming a value its vocabulary does
	// not hold, so a field that stopped being resolved reports here.
	good := expectationDocument{
		Name:       "planted",
		TargetKind: "library",
		Languages:  []string{"go", "ts"},
		Expect: []expectationRow{
			{Symbol: "DeadExport", Report: "DS1001", Confidence: "certain", ReachabilityClass: "certain", LivenessRelation: "reference-counting"},
			{Symbol: "UsedByConsumer", Report: reportedNone},
		},
	}
	planted := []struct {
		mutate  func(*expectationDocument)
		name    string
		wantMsg []string
	}{
		{
			name:   "well_formed",
			mutate: func(*expectationDocument) {},
		},
		{
			name:    "target_kind_outside_the_schema_enum",
			mutate:  func(d *expectationDocument) { d.TargetKind = "module" },
			wantMsg: []string{"target_kind", `"module"`},
		},
		{
			name:    "target_kind_absent",
			mutate:  func(d *expectationDocument) { d.TargetKind = "" },
			wantMsg: []string{"target_kind", `""`},
		},
		{
			name:    "language_outside_the_contract",
			mutate:  func(d *expectationDocument) { d.Languages = []string{"go", "rust"} },
			wantMsg: []string{"languages", `"rust"`},
		},
		{
			name:   "symbol_kind_inside_the_finding_schema_vocabulary",
			mutate: func(d *expectationDocument) { d.Expect[0].SymbolKind = "function" },
		},
		{
			name:    "symbol_kind_outside_the_finding_schema_vocabulary",
			mutate:  func(d *expectationDocument) { d.Expect[0].SymbolKind = "subroutine" },
			wantMsg: []string{"DeadExport", "symbol_kind", `"subroutine"`},
		},
		{
			name:    "confidence_outside_the_reachability_classes",
			mutate:  func(d *expectationDocument) { d.Expect[0].Confidence = "likely" },
			wantMsg: []string{"DeadExport", "confidence", `"likely"`},
		},
		{
			name:    "reachability_class_outside_the_reachability_classes",
			mutate:  func(d *expectationDocument) { d.Expect[0].ReachabilityClass = "unknown" },
			wantMsg: []string{"DeadExport", "reachability_class", `"unknown"`},
		},
		{
			name:    "liveness_relation_outside_the_schema_enum",
			mutate:  func(d *expectationDocument) { d.Expect[0].LivenessRelation = "mark-and-sweep" },
			wantMsg: []string{"DeadExport", "liveness_relation", `"mark-and-sweep"`},
		},
	}
	for _, tc := range planted {
		t.Run("planted_"+tc.name, func(t *testing.T) {
			doc := good
			doc.Languages = slices.Clone(good.Languages)
			doc.Expect = slices.Clone(good.Expect)
			tc.mutate(&doc)

			got := checkExpectationVocabularies(&doc, vocabularies)
			if len(tc.wantMsg) == 0 {
				if len(got) != 0 {
					t.Errorf("checkExpectationVocabularies(%s) = %v, want nil", tc.name, got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("checkExpectationVocabularies(%s) = %v, want one error wrapping %v", tc.name, got, errOutsideVocabulary)
			}
			if !errors.Is(got[0], errOutsideVocabulary) {
				t.Errorf("checkExpectationVocabularies(%s) = %v, want it to wrap %v", tc.name, got[0], errOutsideVocabulary)
			}
			for _, want := range tc.wantMsg {
				if !strings.Contains(got[0].Error(), want) {
					t.Errorf("checkExpectationVocabularies(%s) = %q, want it to name %s", tc.name, got[0], want)
				}
			}
		})
	}
}

func TestCorpusFixtureLanguagesHaveARendering(t *testing.T) {
	renderings := loadCorpus(t).Layout.Renderings

	for _, doc := range corpusExpectations(t) {
		t.Run("fixture_"+doc.Name, func(t *testing.T) {
			dir := path.Join(fixturesDir, doc.Name)
			if got := checkRenderingForms(spec.Corpus, dir, &doc, renderings); len(got) != 0 {
				t.Errorf("checkRenderingForms(%q) = %v, want a rendering for every declared language", dir, got)
			}
		})
	}

	plantedDir := path.Join(fixturesDir, "planted")

	t.Run("planted_language_with_no_stated_form", func(t *testing.T) {
		doc := expectationDocument{Name: "planted", Languages: []string{"go", "rust"}}
		got := checkRenderingForms(plantedFixture(), plantedDir, &doc, renderings)
		if len(got) != 1 || !errors.Is(got[0], errNoRenderingForm) {
			t.Fatalf("checkRenderingForms(planted declaring rust) = %v, want one error wrapping %v", got, errNoRenderingForm)
		}
		for _, want := range []string{"planted", `"rust"`, corpusPath} {
			if !strings.Contains(got[0].Error(), want) {
				t.Errorf("checkRenderingForms(planted declaring rust) = %q, want it to name %s", got[0], want)
			}
		}
	})

	t.Run("planted_stated_form_absent", func(t *testing.T) {
		fsys := plantedFixture()
		delete(fsys, path.Join(plantedDir, goRendering))
		doc := expectationDocument{Name: "planted", Languages: []string{"go", "ts"}}
		got := checkRenderingForms(fsys, plantedDir, &doc, renderings)
		if len(got) != 1 || !errors.Is(got[0], errRenderingFormAbsent) {
			t.Fatalf("checkRenderingForms(planted without its go rendering) = %v, want one error wrapping %v", got, errRenderingFormAbsent)
		}
		for _, want := range []string{"planted", `"go"`, goRendering} {
			if !strings.Contains(got[0].Error(), want) {
				t.Errorf("checkRenderingForms(planted without its go rendering) = %q, want it to name %s", got[0], want)
			}
		}
	})

	t.Run("planted_directory_form_resolves", func(t *testing.T) {
		doc := expectationDocument{Name: "planted", Languages: []string{"ts"}}
		if got := checkRenderingForms(plantedFixture(), plantedDir, &doc, renderings); len(got) != 0 {
			t.Errorf("checkRenderingForms(planted declaring ts) = %v, want the ts/ form to resolve", got)
		}
	})
}

func TestCorpusResultsAnswerEveryExpectation(t *testing.T) {
	fixtures := corpusExpectations(t)
	reported, unreached := omissionRows(t, fixtures)
	complete := answeredResults(t, fixtures, omissionLanguage)
	if got := unansweredExpectations(fixtures, &complete, nil); len(got) != 0 {
		t.Fatalf("unansweredExpectations(a complete %s results document) = %v, want nil so every plant below has one cause", omissionLanguage, got)
	}

	// A fixture the corpus does not select for this language, which a product
	// answering it names a rendering it must not have loaded.
	unselected := ""
	for _, doc := range fixtures {
		if !slices.Contains(doc.Languages, omissionLanguage) {
			unselected = doc.Name
		}
	}
	if unselected == "" {
		t.Fatalf("Setup: every corpus fixture lists %q, want one that does not", omissionLanguage)
	}

	declared := declaredGap{Fixture: omissionFixture, Capability: omissionCode, Reason: "Unused exported declarations are not reported yet."}
	cases := []struct {
		wantErr  error
		name     string
		results  resultsDocument
		gaps     []declaredGap
		wantMsg  []string
		wantNone bool
	}{
		{
			name:    "expectation_omitted_with_no_gap",
			results: withoutExpectation(complete, omissionFixture, reported.Symbol),
			wantErr: errExpectationUnanswered,
			wantMsg: []string{omissionFixture, reported.Symbol, omissionCode},
		},
		{
			name:     "expectation_omitted_under_a_declared_gap",
			results:  withoutExpectation(complete, omissionFixture, reported.Symbol),
			gaps:     []declaredGap{declared},
			wantNone: true,
		},
		{
			name:    "expectation_omitted_under_a_gap_for_another_symbol",
			results: withoutExpectation(complete, omissionFixture, reported.Symbol),
			gaps:    []declaredGap{{Fixture: omissionFixture, Symbol: "AnotherSubject", Capability: omissionCode, Reason: declared.Reason}},
			wantErr: errExpectationUnanswered,
			wantMsg: []string{omissionFixture, reported.Symbol},
		},
		{
			name:    "expectation_exercising_no_capability_omitted_under_a_gap",
			results: withoutExpectation(complete, omissionFixture, unreached.Symbol),
			gaps:    []declaredGap{declared},
			wantErr: errExpectationUnanswered,
			wantMsg: []string{omissionFixture, unreached.Symbol},
		},
		{
			name:    "fixture_omitted",
			results: withoutFixture(complete, omissionFixture),
			wantErr: errFixtureUnanswered,
			wantMsg: []string{omissionFixture, omissionLanguage},
		},
		{
			name:    "fixture_the_corpus_does_not_select",
			results: withFixtures(complete, append(slices.Clone(complete.Fixtures), fixtureResult{Fixture: unselected, Result: "pass"})),
			wantErr: errFixtureNotSelected,
			wantMsg: []string{unselected, omissionLanguage},
		},
		{
			name:    "symbol_the_fixture_does_not_expect",
			results: withSurplusAnswer(complete, omissionFixture, "NeverExpected"),
			wantErr: errExpectationUnexpected,
			wantMsg: []string{omissionFixture, "NeverExpected"},
		},
		{
			name:    "gap_reported_with_no_declaration",
			results: withGapAnswer(complete, omissionFixture, reported.Symbol, omissionCode),
			wantErr: errGapUndeclared,
			wantMsg: []string{omissionFixture, reported.Symbol, omissionCode},
		},
		{
			name:    "gap_reported_under_another_capability",
			results: withGapAnswer(complete, omissionFixture, reported.Symbol, "interface-satisfaction"),
			gaps:    []declaredGap{declared},
			wantErr: errGapUndeclared,
			wantMsg: []string{omissionFixture, reported.Symbol, "interface-satisfaction"},
		},
		{
			name:     "gap_reported_under_its_declaration",
			results:  withGapAnswer(complete, omissionFixture, reported.Symbol, omissionCode),
			gaps:     []declaredGap{declared},
			wantNone: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := unansweredExpectations(fixtures, &tc.results, tc.gaps)
			if tc.wantNone {
				if len(got) != 0 {
					t.Errorf("unansweredExpectations(%s) = %v, want nil", tc.name, got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("unansweredExpectations(%s) = %v, want one error wrapping %v", tc.name, got, tc.wantErr)
			}
			if !errors.Is(got[0], tc.wantErr) {
				t.Errorf("unansweredExpectations(%s) = %v, want it to wrap %v", tc.name, got[0], tc.wantErr)
			}
			for _, want := range tc.wantMsg {
				if !strings.Contains(got[0].Error(), want) {
					t.Errorf("unansweredExpectations(%s) = %q, want it to name %s", tc.name, got[0], want)
				}
			}
		})
	}
}
