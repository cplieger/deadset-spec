package spec_test

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cplieger/deadset-spec/v2"
)

const (
	conformanceSchemaPath = "corpus/conformance.schema.json"
	resultsSchemaPath     = "corpus/conformance-results.schema.json"
	gapRowPath            = "properties/gaps/items"
	fixtureRowPath        = "properties/fixtures/items"
	expectationRowPath    = fixtureRowPath + "/properties/expectations/items"
	answerPath            = expectationRowPath + "/properties/actual"
)

// resultsDocument mirrors conformance-results.json closely enough that an
// unknown key fails a strict decode.
type resultsDocument struct {
	Fixtures      []fixtureResult `json:"fixtures"`
	CorpusVersion string          `json:"corpus_version"`
	Result        string          `json:"result"`
	Product       productIdentity `json:"product"`
	Totals        resultTotals    `json:"totals"`
}

type productIdentity struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Language string `json:"language"`
}

type resultTotals struct {
	Fixtures int `json:"fixtures"`
	Pass     int `json:"pass"`
	Gap      int `json:"gap"`
	Fail     int `json:"fail"`
}

type fixtureResult struct {
	Expectations []expectationResult `json:"expectations"`
	Unexpected   []unexpectedFinding `json:"unexpected"`
	Fixture      string              `json:"fixture"`
	Result       string              `json:"result"`
	Message      string              `json:"message"`
}

type expectationResult struct {
	Suppression *suppressionResult `json:"suppression"`
	Symbol      string             `json:"symbol"`
	Result      string             `json:"result"`
	Capability  string             `json:"capability"`
	Message     string             `json:"message"`
	Actual      answer             `json:"actual"`
}

type answer struct {
	Configurations    []string `json:"configurations"`
	RetainedBy        []string `json:"retained_by"`
	Report            string   `json:"report"`
	SymbolKind        string   `json:"symbol_kind"`
	Confidence        string   `json:"confidence"`
	ReachabilityClass string   `json:"reachability_class"`
	LivenessRelation  string   `json:"liveness_relation"`
}

type suppressionResult struct {
	Suppressed bool `json:"suppressed"`
	Stale      bool `json:"stale"`
}

type unexpectedFinding struct {
	File   string `json:"file"`
	Report string `json:"report"`
	Line   int    `json:"line"`
}

var (
	errGapRowIncomplete = errors.New("gap row lacks a required field")
	errUnknownFixture   = errors.New("results name a fixture the corpus does not hold")

	// resultValues is the closed result set of a fixture and of an expectation.
	resultValues = []string{"pass", "gap", "fail"}
)

// requiredAt returns the required list of the object schema at a
// slash-separated path into schema, or nil.
func requiredAt(schema map[string]any, p string) []string {
	values, ok := objectAt(schema, p)["required"].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// propertyNames returns the sorted property names of the object schema at a
// slash-separated path into schema, the root for an empty path, or nil.
func propertyNames(schema map[string]any, p string) []string {
	node := schema
	if p != "" {
		node = objectAt(schema, p)
	}
	props, ok := node["properties"].(map[string]any)
	if !ok {
		return nil
	}
	return slices.Sorted(maps.Keys(props))
}

// missingRequired lists, in required's order, the keys absent from row.
func missingRequired(row map[string]any, required []string) []string {
	var missing []string
	for _, key := range required {
		if _, ok := row[key]; !ok {
			missing = append(missing, key)
		}
	}
	return missing
}

// checkGapRows applies the gap-row schema's own required list to every row of
// a decoded declared-gap document, so the check follows the schema rather
// than a list of its own. Every error names the row and the keys it lacks.
func checkGapRows(schema, doc map[string]any) error {
	required := requiredAt(schema, gapRowPath)
	rows, _ := doc["gaps"].([]any)
	var errs []error
	for i, r := range rows {
		row, ok := r.(map[string]any)
		if !ok {
			errs = append(errs, fmt.Errorf("gaps[%d] = %v, want an object", i, r))
			continue
		}
		if missing := missingRequired(row, required); len(missing) != 0 {
			errs = append(errs, fmt.Errorf("%w: gaps[%d] (fixture %v) lacks %q", errGapRowIncomplete, i, row["fixture"], missing))
		}
	}
	return errors.Join(errs...)
}

// fixtureNames returns the set of fixture directory names under
// corpus/fixtures in fsys. An absent fixtures directory is the empty set.
func fixtureNames(fsys fs.FS) (map[string]bool, error) {
	entries, err := fs.ReadDir(fsys, fixturesDir)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names[e.Name()] = true
		}
	}
	return names, nil
}

// checkResultsFixtures refuses a results document naming a fixture outside
// known; the error names every such fixture.
func checkResultsFixtures(doc *resultsDocument, known map[string]bool) error {
	var errs []error
	for _, f := range doc.Fixtures {
		if !known[f.Fixture] {
			errs = append(errs, fmt.Errorf("%w: %q", errUnknownFixture, f.Fixture))
		}
	}
	return errors.Join(errs...)
}

// knownFixtures is the fixture set the results checks run against: what the
// embedded corpus holds today plus the planted fixture, so the check has a
// member before the first fixture lands and keeps the real set afterwards.
func knownFixtures(t *testing.T) map[string]bool {
	t.Helper()
	known, err := fixtureNames(spec.Corpus)
	if err != nil {
		t.Fatalf("Setup: fixtureNames(Corpus): %v", err)
	}
	planted, err := fixtureNames(plantedFixture())
	if err != nil {
		t.Fatalf("Setup: fixtureNames(planted): %v", err)
	}
	maps.Copy(known, planted)
	return known
}

// plantedGaps is a well-formed declared-gap document.
const plantedGaps = `{
  "corpus_version": "1.0.0",
  "product": {"name": "deadset-go", "version": "1.0.0"},
  "gaps": [
    {"fixture": "planted", "symbol": "DeadExport", "capability": "DS1001", "reason": "Unused exported declarations are not reported yet."},
    {"fixture": "template-field-reference", "capability": "template-field", "reason": "No template directory scanner is implemented."}
  ]
}`

// plantedGapsWithoutReason is plantedGaps with the second row's reason
// removed.
const plantedGapsWithoutReason = `{
  "corpus_version": "1.0.0",
  "product": {"name": "deadset-go"},
  "gaps": [
    {"fixture": "planted", "symbol": "DeadExport", "capability": "DS1001", "reason": "Unused exported declarations are not reported yet."},
    {"fixture": "template-field-reference", "capability": "template-field"}
  ]
}`

// plantedResults is a well-formed results document over the planted fixture,
// carrying every field the schema declares.
const plantedResults = `{
  "corpus_version": "1.0.0",
  "product": {"name": "deadset-go", "version": "1.0.0", "language": "go"},
  "result": "fail",
  "totals": {"fixtures": 1, "pass": 0, "gap": 0, "fail": 1},
  "fixtures": [
    {
      "fixture": "planted",
      "result": "fail",
      "expectations": [
        {
          "symbol": "DeadExport",
          "result": "pass",
          "actual": {"report": "DS1001", "confidence": "certain", "reachability_class": "certain", "liveness_relation": "reference-counting"},
          "suppression": {"suppressed": true, "stale": false}
        },
        {
          "symbol": "UsedByConsumer",
          "result": "gap",
          "capability": "interface-satisfaction",
          "actual": {"report": "none", "retained_by": ["interface-satisfaction"]},
          "message": "interface-satisfaction is declared as a gap."
        }
      ],
      "unexpected": [
        {"file": "target/lib.go", "line": 7, "report": "DS1002"}
      ],
      "message": "one finding at a position no expectation resolves to"
    }
  ]
}`

func decodeGaps(t *testing.T, data string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := decodeStrict([]byte(data), &doc); err != nil {
		t.Fatalf("Setup: decoding the planted declared-gap document: %v", err)
	}
	return doc
}

func TestConformanceSchemasAreClosedAndDescribed(t *testing.T) {
	for _, p := range []string{conformanceSchemaPath, resultsSchemaPath} {
		t.Run(subtestName(p), func(t *testing.T) {
			schema := loadSchema(t, spec.Corpus, p)
			if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
				t.Errorf("%s $schema = %v, want JSON Schema 2020-12", p, schema["$schema"])
			}
			if got := openObjects(schema, ""); len(got) != 0 {
				t.Errorf("openObjects(%s) = %v, want every object schema to close its key set", p, got)
			}
			if got := undescribedProperties(schema, ""); len(got) != 0 {
				t.Errorf("undescribedProperties(%s) = %v, want a description on every property", p, got)
			}
			for at, pattern := range patterns(schema, "") {
				if _, err := regexp.Compile(pattern); err != nil {
					t.Errorf("regexp.Compile(%s %s = %q) = %v, want a pattern both dialects accept", p, at, pattern, err)
				}
			}
			if refs := references(schema, ""); len(refs) != 0 {
				t.Errorf("%s $ref at %v, want every shape inline", p, refs)
			}
		})
	}
}

func TestConformanceSchemaProperties(t *testing.T) {
	cases := []struct {
		name   string
		schema string
		at     string
		want   []string
	}{
		{name: "gaps_document", schema: conformanceSchemaPath, at: "", want: []string{"corpus_version", "gaps", "product"}},
		{name: "gaps_product", schema: conformanceSchemaPath, at: "properties/product", want: []string{"name", "version"}},
		{name: "gap_row", schema: conformanceSchemaPath, at: gapRowPath, want: []string{"capability", "fixture", "reason", "symbol"}},
		{name: "results_document", schema: resultsSchemaPath, at: "", want: []string{"corpus_version", "fixtures", "product", "result", "totals"}},
		{name: "results_product", schema: resultsSchemaPath, at: "properties/product", want: []string{"language", "name", "version"}},
		{name: "results_totals", schema: resultsSchemaPath, at: "properties/totals", want: []string{"fail", "fixtures", "gap", "pass"}},
		{name: "fixture_row", schema: resultsSchemaPath, at: fixtureRowPath, want: []string{"expectations", "fixture", "message", "result", "unexpected"}},
		{name: "expectation_row", schema: resultsSchemaPath, at: expectationRowPath, want: []string{"actual", "capability", "message", "result", "suppression", "symbol"}},
		{name: "answer", schema: resultsSchemaPath, at: answerPath, want: []string{"confidence", "configurations", "details", "liveness_relation", "reachability_class", "report", "retained_by", "symbol_kind"}},
		{name: "suppression", schema: resultsSchemaPath, at: expectationRowPath + "/properties/suppression", want: []string{"stale", "suppressed"}},
		{name: "unexpected_finding", schema: resultsSchemaPath, at: fixtureRowPath + "/properties/unexpected/items", want: []string{"file", "line", "report"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := loadSchema(t, spec.Corpus, tc.schema)
			if got := propertyNames(schema, tc.at); !slices.Equal(got, tc.want) {
				t.Errorf("propertyNames(%s, %q) = %v, want %v", tc.schema, tc.at, got, tc.want)
			}
		})
	}
}

func TestConformanceResultVocabulary(t *testing.T) {
	results := loadSchema(t, spec.Corpus, resultsSchemaPath)
	kinds := loadKinds(t)

	enums := []struct {
		name string
		at   string
		want []string
	}{
		{name: "fixture_result", at: fixtureRowPath + "/properties/result", want: resultValues},
		{name: "expectation_result", at: expectationRowPath + "/properties/result", want: resultValues},
		{name: "document_result", at: "properties/result", want: []string{"pass", "fail"}},
		{name: "language", at: "properties/product/properties/language", want: kinds.Languages},
		{name: "confidence", at: answerPath + "/properties/confidence", want: kinds.ReachabilityClasses},
		{name: "reachability_class", at: answerPath + "/properties/reachability_class", want: kinds.ReachabilityClasses},
	}
	for _, tc := range enums {
		t.Run(tc.name, func(t *testing.T) {
			if got := enumAt(results, tc.at); !slices.Equal(got, tc.want) {
				t.Errorf("enumAt(%s, %q) = %v, want %v from %s", resultsSchemaPath, tc.at, got, tc.want, kindsPath)
			}
		})
	}
}

func TestConformanceCapabilityPatternsAreTheContractVocabularies(t *testing.T) {
	kinds := loadKinds(t)
	capabilityPattern := "^(" + kinds.Prefix + "[0-9]{4}|[a-z][a-z0-9]*(-[a-z0-9]+)*)$"
	cases := []struct {
		name   string
		schema string
		at     string
		want   string
	}{
		{name: "answer_report", schema: resultsSchemaPath, at: answerPath + "/properties/report", want: "^(" + kinds.Prefix + "[0-9]{4}|none)$"},
		{name: "unexpected_report", schema: resultsSchemaPath, at: fixtureRowPath + "/properties/unexpected/items/properties/report", want: "^" + kinds.Prefix + "[0-9]{4}$"},
		{name: "expectation_capability", schema: resultsSchemaPath, at: expectationRowPath + "/properties/capability", want: capabilityPattern},
		{name: "gap_capability", schema: conformanceSchemaPath, at: gapRowPath + "/properties/capability", want: capabilityPattern},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			schema := loadSchema(t, spec.Corpus, tc.schema)
			if got := patterns(schema, "")["/"+tc.at+"/pattern"]; got != tc.want {
				t.Errorf("pattern at %s %s = %q, want %q", tc.schema, tc.at, got, tc.want)
			}
		})
	}
}

func TestGapRowRequiresReason(t *testing.T) {
	schema := loadSchema(t, spec.Corpus, conformanceSchemaPath)

	t.Run("required_list", func(t *testing.T) {
		want := []string{"fixture", "capability", "reason"}
		if got := requiredAt(schema, gapRowPath); !slices.Equal(got, want) {
			t.Errorf("requiredAt(%s, %q) = %v, want %v", conformanceSchemaPath, gapRowPath, got, want)
		}
	})

	t.Run("accepts_a_reasoned_row", func(t *testing.T) {
		if err := checkGapRows(schema, decodeGaps(t, plantedGaps)); err != nil {
			t.Errorf("checkGapRows(plantedGaps) = %v, want nil", err)
		}
	})

	t.Run("refuses_a_row_without_reason", func(t *testing.T) {
		err := checkGapRows(schema, decodeGaps(t, plantedGapsWithoutReason))
		if !errors.Is(err, errGapRowIncomplete) {
			t.Fatalf("checkGapRows(plantedGapsWithoutReason) error = %v, want %v", err, errGapRowIncomplete)
		}
		for _, want := range []string{`"reason"`, "template-field-reference"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("checkGapRows(plantedGapsWithoutReason) error = %q, want it to name %s", err, want)
			}
		}
	})

	// The refusal above must come from the schema's required list and not
	// from a list of the check's own: a copy of the schema that stops
	// requiring reason accepts the same document.
	t.Run("follows_the_schema", func(t *testing.T) {
		relaxed := loadSchema(t, spec.Corpus, conformanceSchemaPath)
		row := objectAt(relaxed, gapRowPath)
		if row == nil {
			t.Fatalf("Setup: %s %s is not an object", conformanceSchemaPath, gapRowPath)
		}
		row["required"] = []any{"fixture", "capability"}
		if err := checkGapRows(relaxed, decodeGaps(t, plantedGapsWithoutReason)); err != nil {
			t.Errorf("checkGapRows(relaxed schema, plantedGapsWithoutReason) = %v, want nil once reason is not required", err)
		}
	})
}

func TestResultsDocumentDecodesStrictly(t *testing.T) {
	var doc resultsDocument
	if err := decodeStrict([]byte(plantedResults), &doc); err != nil {
		t.Fatalf("decodeStrict(plantedResults) = %v, want a document carrying only declared fields", err)
	}
	if got := doc.Fixtures[0].Expectations[0].Suppression; got == nil || !got.Suppressed || got.Stale {
		t.Errorf("plantedResults DeadExport suppression = %+v, want suppressed and not stale", got)
	}
	planted := strings.Replace(plantedResults, `"result": "fail",`, `"result": "fail", "language": "go",`, 1)
	if err := decodeStrict([]byte(planted), &doc); err == nil || !strings.Contains(err.Error(), `unknown field "language"`) {
		t.Errorf("decodeStrict(results with a top-level language) error = %v, want an unknown-field error", err)
	}
}

func TestResultsNameOnlyCorpusFixtures(t *testing.T) {
	known := knownFixtures(t)
	if !known["planted"] {
		t.Fatalf("Setup: knownFixtures = %v, want it to hold the planted fixture", slices.Sorted(maps.Keys(known)))
	}

	t.Run("accepts_corpus_fixtures", func(t *testing.T) {
		doc := resultsDocument{Fixtures: []fixtureResult{{Fixture: "planted", Result: "pass"}}}
		if err := checkResultsFixtures(&doc, known); err != nil {
			t.Errorf("checkResultsFixtures(planted) = %v, want nil", err)
		}
	})

	t.Run("refuses_a_fixture_absent_from_the_corpus", func(t *testing.T) {
		doc := resultsDocument{Fixtures: []fixtureResult{
			{Fixture: "planted", Result: "pass"},
			{Fixture: "absent-fixture", Result: "pass"},
		}}
		err := checkResultsFixtures(&doc, known)
		if !errors.Is(err, errUnknownFixture) {
			t.Fatalf("checkResultsFixtures(absent-fixture) error = %v, want %v", err, errUnknownFixture)
		}
		if !strings.Contains(err.Error(), `"absent-fixture"`) || strings.Contains(err.Error(), `"planted"`) {
			t.Errorf("checkResultsFixtures(absent-fixture) error = %q, want it to name only the absent fixture", err)
		}
	})

	t.Run("no_fixtures_directory_is_the_empty_set", func(t *testing.T) {
		got, err := fixtureNames(fstest.MapFS{"corpus/corpus.json": {Data: []byte("{}")}})
		if err != nil || len(got) != 0 {
			t.Errorf("fixtureNames(no fixtures directory) = %v, %v, want an empty set and nil", got, err)
		}
	})
}
