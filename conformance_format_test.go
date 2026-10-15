package spec_test

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/corpus"
)

const (
	conformanceSchemaPath = "corpus/conformance.schema.json"
	resultsSchemaPath     = "corpus/conformance-results.schema.json"
	gapRowPath            = "properties/gaps/items"
	fixtureRowPath        = "properties/fixtures/items"
	expectationRowPath    = fixtureRowPath + "/properties/expectations/items"
	answerPath            = expectationRowPath + "/properties/actual"
)

var (
	errGapRowIncomplete = errors.New("gap row lacks a required field")

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
  "totals": {"fixtures": 1, "pass": 0, "gap": 0, "fail": 1, "pending": 0},
  "fixtures": [
    {
      "fixture": "planted",
      "result": "fail",
      "expectations": [
        {
          "symbol": "DeadExport",
          "result": "pass",
          "actual": {"report": "DS1001", "confidence": "certain", "reachability_class": "certain", "liveness_relation": "reference-counting", "fixability": "deletable"},
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
	if err := corpus.DecodeStrict([]byte(data), &doc); err != nil {
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
		{name: "results_totals", schema: resultsSchemaPath, at: "properties/totals", want: []string{"fail", "fixtures", "gap", "pass", "pending"}},
		{name: "fixture_row", schema: resultsSchemaPath, at: fixtureRowPath, want: []string{"expectations", "fixture", "message", "result", "unexpected"}},
		{name: "expectation_row", schema: resultsSchemaPath, at: expectationRowPath, want: []string{"actual", "capability", "message", "result", "suppression", "symbol"}},
		{name: "answer", schema: resultsSchemaPath, at: answerPath, want: []string{"component", "confidence", "configurations", "details", "fixability", "liveness_relation", "reachability_class", "report", "retained_by", "symbol_kind"}},
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
		{name: "fixture_result", at: fixtureRowPath + "/properties/result", want: append(slices.Clone(resultValues), "pending")},
		{name: "expectation_result", at: expectationRowPath + "/properties/result", want: resultValues},
		{name: "document_result", at: "properties/result", want: []string{"pass", "fail"}},
		{name: "language", at: "properties/product/properties/language", want: kinds.Languages},
		{name: "confidence", at: answerPath + "/properties/confidence", want: kinds.ReachabilityClasses},
		{name: "reachability_class", at: answerPath + "/properties/reachability_class", want: kinds.ReachabilityClasses},
		{name: "fixability", at: answerPath + "/properties/fixability", want: kinds.Fixabilities},
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
	var doc corpus.Results
	if err := corpus.DecodeStrict([]byte(plantedResults), &doc); err != nil {
		t.Fatalf("corpus.DecodeStrict(plantedResults) = %v, want a document carrying only declared fields", err)
	}
	if got := doc.Fixtures[0].Expectations[0].Suppression; got == nil || !got.Suppressed || got.Stale {
		t.Errorf("plantedResults DeadExport suppression = %+v, want suppressed and not stale", got)
	}
	planted := strings.Replace(plantedResults, `"result": "fail",`, `"result": "fail", "language": "go",`, 1)
	if err := corpus.DecodeStrict([]byte(planted), &doc); err == nil || !strings.Contains(err.Error(), `unknown field "language"`) {
		t.Errorf("corpus.DecodeStrict(results with a top-level language) error = %v, want an unknown-field error", err)
	}
	if got := doc.Fixtures[0].Expectations[0].Actual.Fixability; got != "deletable" {
		t.Errorf("plantedResults DeadExport actual fixability = %q, want %q", got, "deletable")
	}
}

func TestResultsSchemaFixabilityBindsToAReportedFinding(t *testing.T) {
	const deadExport = `"actual": {"report": "DS1001", "confidence": "certain", "reachability_class": "certain", "liveness_relation": "reference-counting", "fixability": "deletable"}`
	const at = "/fixtures/0/expectations/0/actual"
	cases := []struct {
		name   string
		actual string
		want   string
	}{
		{name: "a_reported_finding_echoes_its_fixability", actual: deadExport},
		{name: "a_value_outside_the_vocabulary", actual: `"actual": {"report": "DS1001", "confidence": "certain", "fixability": "removable"}`, want: at + "/fixability"},
		{name: "an_unreported_subject_carries_no_fixability", actual: `"actual": {"report": "none", "fixability": "deletable"}`, want: at},
		{name: "a_stale_suppression_carries_no_fixability", actual: `"actual": {"report": "DS9003", "confidence": "certain", "fixability": "manual"}`, want: at},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(plantedResults, deadExport) {
				t.Fatalf("Setup: plantedResults carries no DeadExport actual %s", deadExport)
			}
			document := strings.Replace(plantedResults, deadExport, tc.actual, 1)
			problems, err := validationProblems(resultsSchemaPath, []byte(document))
			if err != nil {
				t.Fatalf("Setup: validationProblems(%q, %s): %v", resultsSchemaPath, tc.name, err)
			}
			if tc.want == "" {
				if len(problems) != 0 {
					t.Errorf("validationProblems(%q, %s) = %v, want no violation", resultsSchemaPath, tc.name, problems)
				}
				return
			}
			if !slices.ContainsFunc(problems, func(p validationProblem) bool { return p.InstanceLocation == tc.want }) {
				t.Errorf("validationProblems(%q, %s) = %v, want a violation at %q", resultsSchemaPath, tc.name, problems, tc.want)
			}
		})
	}
}
