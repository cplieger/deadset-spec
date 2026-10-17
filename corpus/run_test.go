package corpus

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	spec "github.com/cplieger/deadset-spec/v8"
)

func at(path string, line int) wirePosition { return wirePosition{Path: path, Line: line} }

func TestUnexpectedNamesEveryFindingAndRecordNoRowNames(t *testing.T) {
	expect := &Expectation{
		ConfiguredRoots: map[string]string{"Configured": "ts://@example/target#configured"},
		ConfiguredDeclarations: map[string]ConfiguredDeclaration{
			"Absent": {Key: "ts.serializers", Entry: map[string]string{"module": "@example/absent", "name": "encode"}},
		},
		Expect: []Row{
			{Symbol: "Kept", Report: "DS1001"},
			{Symbol: "Configured", Report: "DS9004"},
			{Symbol: "Absent", Report: "DS9006"},
		},
	}
	finding := func(code, ref string, p wirePosition) wireFinding {
		f := wireFinding{Code: code, Position: p}
		f.Symbol.Ref = ref
		return f
	}
	report := &wireReport{
		Findings: []wireFinding{
			finding("DS1001", "ts://@example/target/a.ts#kept", at("a.ts", 3)),
			finding("DS1503", "ts://@example/target/a.ts#kept", at("a.ts", 3)),
			finding("DS1002", "ts://@example/target/b.ts#extra", at("b.ts", 9)),
			finding("DS9004", "ts://@example/target#configured", at("deadset.json", 1)),
			finding("DS9004", "ts://@example/target#other", at("deadset.json", 1)),
			finding("DS9006", "@example/absent#encode", at("deadset.json", 1)),
			finding("DS9006", "#JSON.parse", at("deadset.json", 1)),
		},
		StaleSuppressions: []wireStale{{Code: "DS9003", Position: at("a.ts", 1)}},
	}
	got := unexpectedOf(report, expect, map[string]Site{"Kept": {File: "target/a.ts", Line: 3}})
	want := []Unexpected{
		{File: "target/a.ts", Line: 1, Report: "DS9003"},
		{File: "target/a.ts", Line: 3, Report: "DS1503"},
		{File: "target/b.ts", Line: 9, Report: "DS1002"},
		{File: "target/deadset.json", Line: 1, Report: "DS9004"},
		{File: "target/deadset.json", Line: 1, Report: "DS9006"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("unexpectedOf = %+v, want %+v", got, want)
	}
}

func TestConfiguredDeclarationRefSpellsEachEntryShape(t *testing.T) {
	for entry, want := range map[string]string{
		`{"symbol":"ts://@example/target/codec.ts#encode"}`: "ts://@example/target/codec.ts#encode",
		`{"module":"@example/absent","name":"encode"}`:      "@example/absent#encode",
		`{"global":"JSON.parse"}`:                           "#JSON.parse",
	} {
		var declared ConfiguredDeclaration
		if err := json.Unmarshal([]byte(entry), &declared.Entry); err != nil {
			t.Fatal(err)
		}
		if got := declared.Ref(); got != want {
			t.Errorf("Ref(%s) = %q, want %q", entry, got, want)
		}
	}
}

func TestEdgeDifferencesHoldTheReportToTheEntries(t *testing.T) {
	manifest := &Manifest{Symbols: map[string]ManifestPosition{"Handler": {File: "target/a.ts", Line: new(4)}}}
	finding := &wireFinding{Code: "DS1001", Position: at("a.ts", 4)}
	expect := &Expectation{EdgeEvaluations: []Evaluation{
		{Edge: "wire/one", Side: "provides", State: "dead", Symbol: "Handler", Report: "DS1001"},
		{Edge: "wire/two", Side: "used_by", State: "live"},
	}}
	agreeing := &wireReport{EdgeEvaluations: []wireEvaluation{
		{Edge: "wire/one", Side: "provides", State: "dead", Finding: finding},
		{Edge: "wire/two", Side: "used_by", State: "live"},
	}}
	if got := edgeDifferences(expect, manifest, agreeing); got != "" {
		t.Errorf("edgeDifferences(agreeing) = %q, want none", got)
	}
	expect.EdgeEvaluations[0].Report = "DS1002"
	expect.EdgeEvaluations = append(expect.EdgeEvaluations, Evaluation{Edge: "wire/three", Side: "provides", State: "absent"})
	differing := &wireReport{EdgeEvaluations: []wireEvaluation{
		{Edge: "wire/one", Side: "provides", State: "dead", Finding: finding},
		{Edge: "wire/two", Side: "used_by", State: "absent"},
		{Edge: "wire/four", Side: "used_by", State: "live"},
	}}
	want := "wire/one provides want DS1002 at target/a.ts:4, got DS1001 at target/a.ts:4; " +
		"wire/two used_by state want live got absent; " +
		"want exactly one evaluation of wire/three provides, got 0; " +
		"evaluation of wire/four used_by, which no entry names"
	if got := edgeDifferences(expect, manifest, differing); got != want {
		t.Errorf("edgeDifferences(differing) =\n%q\nwant\n%q", got, want)
	}
	if got := edgeDifferences(&Expectation{}, manifest, agreeing); !strings.Contains(got, "which no entry names") {
		t.Errorf("edgeDifferences(no entries) = %q, want every record named", got)
	}
}

func TestNotBuiltAndInEffectDifferences(t *testing.T) {
	report := &wireReport{}
	report.ConfigurationsNotBuilt = append(report.ConfigurationsNotBuilt, struct {
		ID string `json:"id"`
	}{ID: "a/tsconfig.json"})
	report.Totals.SuppressionsInEffect = 2
	if got := notBuiltDifference(&Manifest{}, report); got != "" {
		t.Errorf("notBuiltDifference(no list) = %q, want none", got)
	}
	if got := notBuiltDifference(&Manifest{ConfigurationsNotBuilt: []string{"a/tsconfig.json"}}, report); got != "" {
		t.Errorf("notBuiltDifference(the same list) = %q, want none", got)
	}
	if got, want := notBuiltDifference(&Manifest{ConfigurationsNotBuilt: []string{}}, report), `configurations_not_built want [] got ["a/tsconfig.json"]`; got != want {
		t.Errorf("notBuiltDifference(empty) = %q, want %q", got, want)
	}
	two, three := 2, 3
	if got := inEffectDifference(&Expectation{SuppressionsInEffect: &two}, report); got != "" {
		t.Errorf("inEffectDifference(the report's count) = %q, want none", got)
	}
	if got, want := inEffectDifference(&Expectation{SuppressionsInEffect: &three}, report), "suppressions_in_effect want 3 got 2"; got != want {
		t.Errorf("inEffectDifference(3) = %q, want %q", got, want)
	}
}

func TestConfigOfDeclaresTheWorldTheFixtureNames(t *testing.T) {
	manifest := &Manifest{Configurations: []string{"linux-amd64", "linux-arm64-netgo"}}
	expect := &Expectation{
		TargetKind:      TargetKind{One: "library"},
		ClosedWorld:     []string{WorldConsumers, WorldMatrix, WorldRoots},
		ConfiguredRoots: map[string]string{"B": "go://example.com/app#b*", "A": "go://example.com/app#a*"},
		MinConfidence:   "possible",
		Severity:        map[string]string{"DS1201": "warn"},
	}
	got, err := configOf(expect, manifest, "go")
	if err != nil {
		t.Fatalf("configOf = %v", err)
	}
	want := `{"analysis":{"configurations":[{"arch":"amd64","id":"linux-amd64","os":"linux"},{"arch":"arm64","id":"linux-arm64-netgo","os":"linux","tags":["netgo"]}],` +
		`"matrix":{"complete":true},"min_confidence":"possible"},"consumers":{"complete":true},` +
		`"roots":{"patterns":["go://example.com/app#a*","go://example.com/app#b*"]},"severity":{"DS1201":"warn"},"target":{"kind":"library"}}` + "\n"
	if string(got) != want {
		t.Errorf("configOf =\n%s\nwant\n%s", got, want)
	}
	expect.ClosedWorld = []string{WorldRoots}
	if got, err := configOf(expect, manifest, "ts"); err != nil || !strings.Contains(string(got), `"ts":{"roots_complete":true}`) {
		t.Errorf("configOf(ts, closed roots) = %s, %v, want ts.roots_complete set", got, err)
	}
	if _, err := configOf(&Expectation{ClosedWorld: []string{WorldMatrix}}, &Manifest{Configurations: []string{"linux"}}, "go"); !errors.Is(err, errFixture) {
		t.Errorf("configOf(an identifier with no architecture) = %v, want the fixture refused", err)
	}
}

// TestConfiguredDeclarationsAreEntriesOfTheirKey holds the configuration the
// runner writes for every fixture naming configured declarations to the
// configuration schema, so the analyzer reads it rather than refusing it.
func TestConfiguredDeclarationsAreEntriesOfTheirKey(t *testing.T) {
	for _, fixture := range corpusFixtures(t) {
		if len(fixture.Expect.ConfiguredDeclarations) == 0 {
			continue
		}
		language := fixture.Expect.Languages[0]
		data, err := configOf(&fixture.Expect, &fixture.Renderings[language].Manifest, language)
		if err != nil {
			t.Fatalf("configOf(%s) = %v", fixture.Name, err)
		}
		if err := Validate("contract/config.schema.json", data); err != nil {
			t.Errorf("the configuration written for %s does not meet the configuration schema: %v", fixture.Name, err)
		}
	}
	planted := &Expectation{TargetKind: TargetKind{One: "application"}, ConfiguredDeclarations: map[string]ConfiguredDeclaration{
		"BothShapes": {Key: "ts.serializers", Entry: map[string]string{"global": "JSON.stringify", "symbol": "ts://./main.ts#encode"}},
	}}
	data, err := configOf(planted, &Manifest{}, "ts")
	if err != nil {
		t.Fatal(err)
	}
	if Validate("contract/config.schema.json", data) == nil {
		t.Error("the configuration of an entry carrying two shapes meets the schema, want it refused")
	}
}

func TestRetainedListingReadsEachDeclarationsSiteAndClasses(t *testing.T) {
	out := "go://example.com/app#Sink.Write\tapp.go:13:16\tinterface-satisfaction\tapp.go:26:23\tsatisfies io.Writer\n" +
		"ts://app/src/a.ts#A.b\tsrc/a.ts:4:3\ttemplate-field\tt.html:1:1\tnamed by {{b}}\tdecorator\tsrc/a.ts:3:1\tdecorated by @x\n"
	got, err := retainedListing([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"interface-satisfaction"}; !slices.Equal(got[Site{File: "target/app.go", Line: 13}], want) {
		t.Errorf("retainedListing = %v, want %v at target/app.go:13", got, want)
	}
	if want := []string{"decorator", "template-field"}; !slices.Equal(got[Site{File: "target/src/a.ts", Line: 4}], want) {
		t.Errorf("retainedListing = %v, want %v at target/src/a.ts:4", got, want)
	}
	if _, err := retainedListing([]byte("go://example.com/app#Sink.Write\tinterface-satisfaction\tapp.go:26:23\tsatisfies io.Writer\n")); err == nil {
		t.Error("retainedListing(a line without the declaration's position) = nil, want it refused")
	}
}

func TestDecideGapsARowADeclaredGapCovers(t *testing.T) {
	row := &Row{Symbol: "DeadExport", Report: "DS1001"}
	gaps := []GapRow{{Fixture: "f", Capability: "DS1001", Reason: "r"}}
	got := ExpectationResult{Symbol: row.Symbol}
	if decide(row, &got, gaps, "f"); got.Result != Gap || got.Message != StaleGap {
		t.Errorf("decide(an answered row under a gap) = %+v, want a gap with the stale-gap message", got)
	}
	failed := ExpectationResult{Symbol: row.Symbol, Suppression: &Suppression{Suppressed: false}}
	if decide(row, &failed, nil, "f"); failed.Result != Fail || !strings.Contains(failed.Message, "suppressed is false") {
		t.Errorf("decide(a row the second phase did not silence) = %+v, want a failure naming the phase", failed)
	}
}

func TestCheckCoverageRefusesATwoLanguageRuleNoTwoLanguageFixtureCites(t *testing.T) {
	rules := Rules{"roots.1": {"go", "ts"}, "references-go.1": {"go"}}
	fixture := func(name string, languages []string, pending string, rules ...string) Fixture {
		return Fixture{Name: name, Expect: Expectation{Name: name, Languages: languages, Pending: pending, Rules: rules}}
	}
	cases := []struct {
		name     string
		fixtures []Fixture
		want     error
	}{
		{name: "a_two_language_fixture", fixtures: []Fixture{fixture("a", []string{"go", "ts"}, "", "roots.1"), fixture("b", []string{"go"}, "", "roots.1")}},
		{name: "one_language_alone", fixtures: []Fixture{fixture("a", []string{"go"}, "", "roots.1")}, want: ErrRuleOneLanguage},
		{name: "pending_alone", fixtures: []Fixture{fixture("a", []string{"go"}, "the rule lands later", "roots.1")}},
		{name: "a_rule_of_one_language", fixtures: []Fixture{fixture("a", []string{"go"}, "", "references-go.1")}},
		{name: "a_rule_of_another_language", fixtures: []Fixture{fixture("a", []string{"ts"}, "", "references-go.1")}, want: ErrRuleLanguageMissing},
		{name: "an_unknown_rule", fixtures: []Fixture{fixture("a", []string{"go"}, "", "roots.99")}, want: ErrUnknownRule},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := errors.Join(checkCoverage(tc.fixtures, rules)...)
			if tc.want == nil && got != nil || tc.want != nil && !errors.Is(got, tc.want) {
				t.Errorf("checkCoverage(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
	read, err := ReadRules()
	if err != nil {
		t.Fatal(err)
	}
	if langs := read["confidence.23"]; !slices.Equal(langs, []string{"go", "ts"}) {
		t.Errorf("ReadRules()[confidence.23] = %v, want every language", langs)
	}
	if langs := read["references-go.1"]; !slices.Equal(langs, []string{"go"}) {
		t.Errorf("ReadRules()[references-go.1] = %v, want go alone", langs)
	}
}

func TestCheckExpectationRefusesValuesOutsideTheVocabularies(t *testing.T) {
	env, err := checkEnvironment(spec.Corpus)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		row  Row
		want string
	}{
		{name: "a_live_code", row: Row{Symbol: "A", Report: "DS1001", Confidence: "certain"}},
		{name: "an_unassigned_code", row: Row{Symbol: "A", Report: "DS1010"}, want: `report = "DS1010"`},
		{name: "a_code_inside_the_reserved_range", row: Row{Symbol: "A", Report: "DS1850"}, want: `report = "DS1850"`},
		{name: "a_kind_name_instead_of_a_code", row: Row{Symbol: "A", Report: "unused-exported"}, want: `report = "unused-exported"`},
		{name: "an_unknown_class", row: Row{Symbol: "A", Report: "none", RetainedBy: []string{"no-such-class"}}, want: "retained_by"},
		{name: "a_relation_on_a_write_only_subject", row: Row{Symbol: "A", Report: "DS1301", LivenessRelation: "reference-counting"}, want: "holds live"},
		{name: "a_relation_on_a_part", row: Row{Symbol: "A", Report: "DS1401", SymbolKind: "parameter", LivenessRelation: "reference-counting"}, want: "no declaration"},
		{name: "a_relation_on_a_dead_declaration", row: Row{Symbol: "A", Report: "DS1001", SymbolKind: "function", LivenessRelation: "reference-counting"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expect := &Expectation{Name: "planted", Languages: []string{"go"}, TargetKind: TargetKind{One: "application"}, Expect: []Row{tc.row}}
			got := errors.Join(checkExpectation(expect, env)...)
			if tc.want == "" && got != nil || tc.want != "" && (got == nil || !strings.Contains(got.Error(), tc.want)) {
				t.Errorf("checkExpectation(%s) = %v, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestRunCasesHoldsEachRunToItsOutcome(t *testing.T) {
	vectors := fstest.MapFS{
		"vectors/cli/named/case.json": {Data: []byte(`{"description": "d", "products": ["p"], "argv": ["print-config", "--central=@TARGET@/missing.json"],
			"expect": {"exit": 2, "stderr_contains": ["missing.json"]}}`)},
		"vectors/cli/named/tree/deadset.json": {Data: []byte(`{"target": {"path": "@TARGET@"}}`)},
		"vectors/cli/pending/case.json":       {Data: []byte(`{"description": "d", "pending": "a rule", "products": ["p"], "argv": ["x"], "expect": {"exit": 0}}`)},
		"vectors/cli/pending/tree/a":          {Data: []byte("a\n")},
		"vectors/cli/other/case.json":         {Data: []byte(`{"description": "d", "products": ["q"], "argv": ["x"], "expect": {"exit": 0}}`)},
	}
	refusing := Analyzer{Command: []string{"/bin/sh", "-c", `grep -q "$PWD" deadset.json || exit 9; echo "cannot read ${2#--central=}" >&2; exit 2`, "sh"}}
	got, err := RunCases(t.Context(), vectors, "p", refusing)
	if err != nil {
		t.Fatal(err)
	}
	want := []CaseResult{{Case: "named", Result: Pass}, {Case: "pending", Result: Pending, Message: got[1].Message}}
	if !slices.Equal(got, want) {
		t.Errorf("RunCases = %+v, want %+v", got, want)
	}
	silent := Analyzer{Command: []string{"/bin/sh", "-c", "exit 0", "sh"}}
	if got, err := RunCases(t.Context(), vectors, "p", silent); err != nil || got[0].Result != Fail || !strings.Contains(got[0].Message, "exit 0, want 2") {
		t.Errorf("RunCases(an analyzer exiting 0) = %+v, %v, want the case failed on its exit", got, err)
	}
}

func TestRunCasesGivesTheRunTheCaseEnvironment(t *testing.T) {
	vectors := fstest.MapFS{
		"vectors/cli/env/case.json": {Data: []byte(`{"description": "d", "products": ["p"], "argv": ["x"], "env": {"CASE_DIR": "@TARGET@"},
			"expect": {"exit": 3}}`)},
		"vectors/cli/env/tree/a": {Data: []byte("a\n")},
	}
	reading := Analyzer{Command: []string{"/bin/sh", "-c", `[ "$CASE_DIR" = "$PWD" ] && echo "run failure: product-fault: f" >&2 && exit 3; exit 9`, "sh"}}
	got, err := RunCases(t.Context(), vectors, "p", reading)
	if err != nil || len(got) != 1 || got[0].Result != Pass {
		t.Errorf("RunCases(a case setting CASE_DIR to @TARGET@) = %+v, %v, want the run to see the target directory", got, err)
	}
}

func TestRunCasesHoldsARunEndingWithTheFailureCodeToOneClassLine(t *testing.T) {
	vectors := fstest.MapFS{
		"vectors/cli/ended/case.json": {Data: []byte(`{"description": "d", "products": ["p"], "argv": ["x"], "expect": {"exit": 3}}`)},
		"vectors/cli/ended/tree/a":    {Data: []byte("a\n")},
	}
	for _, tc := range []struct {
		name, stderr string
		want         string
	}{
		{name: "one class line and a further failure", stderr: `setup failure: missing-module: a\nmissing-module: b`, want: Pass},
		{name: "a run class line", stderr: `run failure: product-fault: a`, want: Pass},
		{name: "two class lines", stderr: `setup failure: missing-module: a\nsetup failure: missing-module: b`, want: Fail},
		{name: "no class line", stderr: `deadset-go: a`, want: Fail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ending := Analyzer{Command: []string{"/bin/sh", "-c", `printf '` + tc.stderr + `\n' >&2; exit 3`, "sh"}}
			got, err := RunCases(t.Context(), vectors, "p", ending)
			if err != nil || len(got) != 1 || got[0].Result != tc.want {
				t.Errorf("RunCases(an exit 3 printing %q) = %+v, %v, want %s", tc.stderr, got, err, tc.want)
			}
		})
	}
}

func TestSetupFailureDifferenceHoldsTheRunToOneClassLine(t *testing.T) {
	want := &SetupFailure{Class: "missing-module", Names: []string{"gen"}}
	if got := setupFailureDifference(want, 3, "setup failure: missing-module: gen\nmissing-module: other\n", 0); got != "" {
		t.Errorf("setupFailureDifference(one class line and a further failure) = %q, want \"\"", got)
	}
	got := setupFailureDifference(want, 3, "setup failure: missing-module: gen\nsetup failure: missing-module: other\n", 0)
	if !strings.Contains(got, "want exactly one class line, got 2") {
		t.Errorf("setupFailureDifference(two class lines) = %q, want the second class line refused", got)
	}
}

func TestRunCasesRunsTheTreeExecutablesAndInterruptsAtTheMarker(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	vectors := fstest.MapFS{
		"vectors/cli/stub/case.json": {Data: []byte(`{"description": "d", "products": ["p"], "argv": ["x"], "executables": ["bin/stub"],
			"interrupt": "@TARGET@/started", "expect": {"exit": 4}}`)},
		"vectors/cli/stub/tree/bin/stub": {Data: []byte("#!/bin/sh\ntouch \"$PWD/started\"\n")},
	}
	waiting := Analyzer{Command: []string{"/bin/sh", "-c", `trap 'exit 4' INT; ./bin/stub || exit 9; while :; do sleep 0.05; done`, "sh"}}
	got, err := RunCases(ctx, vectors, "p", waiting)
	if err != nil || len(got) != 1 || got[0].Result != Pass {
		t.Errorf("RunCases(an executable stub writing the interrupt marker) = %+v, %v, want the run interrupted at the marker", got, err)
	}
}
