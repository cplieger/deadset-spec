package spec_test

import (
	"io/fs"
	"regexp"
	"slices"
	"testing"

	"github.com/cplieger/deadset-spec"
)

const (
	reportSchemaPath = "contract/report.schema.json"

	// reportFindingRef is the one cross-file reference this contract allows:
	// the finding object is declared once and referenced from the two places a
	// finding appears in a report.
	reportFindingRef        = "finding.schema.json"
	reportFindingRefPath    = "contract/" + reportFindingRef
	reportEdgeItemPath      = "properties/edge_evaluations/items"
	reportStaleItemPath     = "properties/stale_suppressions/items"
	reportConformancePath   = "properties/analyzer/properties/conformance"
	reportDigestPattern     = "^sha256:[0-9a-f]{64}$"
	reportStaleCode         = "DS1703"
	reportSchemaDialect     = "https://json-schema.org/draft/2020-12/schema"
	reportFindingRefKeyword = "$ref"
)

// reportFindingRefPaths are the two places a report carries a finding, as
// paths into the schema tree: the reported findings and the pending finding an
// edge evaluation carries.
var reportFindingRefPaths = []string{
	"/properties/edge_evaluations/items/properties/finding/$ref",
	"/properties/findings/items/$ref",
}

func loadReportSchema(t *testing.T) map[string]any {
	t.Helper()
	return loadSchema(t, spec.Contract, reportSchemaPath)
}

// reportConstAt returns the const of the object schema at a slash-separated
// path into schema, and the empty string when the path holds no string const.
func reportConstAt(schema map[string]any, p string) string {
	value, _ := objectAt(schema, p)["const"].(string)
	return value
}

// reportRequiredAt returns the required list at a slash-separated path into
// schema, the document root for an empty path.
func reportRequiredAt(schema map[string]any, p string) []string {
	if p == "" {
		return stringSlice(schema["required"])
	}
	return requiredAt(schema, p)
}

func TestReportSchemaIsClosedAndDescribed(t *testing.T) {
	schema := loadReportSchema(t)
	if schema["$schema"] != reportSchemaDialect {
		t.Errorf("%s $schema = %v, want %q", reportSchemaPath, schema["$schema"], reportSchemaDialect)
	}
	if got := openObjects(schema, ""); len(got) != 0 {
		t.Errorf("openObjects(%s) = %v, want every object schema to close its key set", reportSchemaPath, got)
	}
	if got := undescribedProperties(schema, ""); len(got) != 0 {
		t.Errorf("undescribedProperties(%s) = %v, want a description on every property", reportSchemaPath, got)
	}
	for at, pattern := range patterns(schema, "") {
		if _, err := regexp.Compile(pattern); err != nil {
			t.Errorf("regexp.Compile(%s %s = %q) = %v, want a pattern both dialects accept", reportSchemaPath, at, pattern, err)
		}
	}
}

// TestReportSchemaReferencesOnlyTheFindingSchema pins the one reference the
// envelope makes outside its own file, and that the file it names is in the
// tree: a report carries findings in two places and the finding object has one
// owner, so neither place restates it.
func TestReportSchemaReferencesOnlyTheFindingSchema(t *testing.T) {
	schema := loadReportSchema(t)
	if got := references(schema, ""); !slices.Equal(got, reportFindingRefPaths) {
		t.Errorf("references(%s) = %v, want %v", reportSchemaPath, got, reportFindingRefPaths)
	}
	for _, at := range []string{"properties/findings/items", reportEdgeItemPath + "/properties/finding"} {
		node := objectAt(schema, at)
		if got, _ := node[reportFindingRefKeyword].(string); got != reportFindingRef {
			t.Errorf("schema[%s].$ref = %q, want %q", at, got, reportFindingRef)
		}
		if desc, _ := node["description"].(string); desc == "" {
			t.Errorf("schema[%s].description = %v, want a description beside the reference", at, node["description"])
		}
	}
	if _, err := fs.Stat(spec.Contract, reportFindingRefPath); err != nil {
		t.Errorf("fs.Stat(Contract, %q) = %v, want the referenced schema present in the tree", reportFindingRefPath, err)
	}
}

func TestReportSchemaProperties(t *testing.T) {
	schema := loadReportSchema(t)
	cases := []struct {
		name string
		at   string
		want []string
	}{
		{
			name: "envelope",
			at:   "",
			want: []string{
				"analyzer", "configurations", "consumers", "contract_version", "declared_gaps",
				"edge_evaluations", "excluded_by_cgo", "findings", "merged_from", "schema_version",
				"stale_suppressions", "target", "test_file_rules", "totals",
			},
		},
		{name: "analyzer", at: "properties/analyzer", want: []string{"conformance", "languages", "name", "schema_versions_accepted", "version"}},
		{name: "conformance", at: reportConformancePath, want: []string{"corpus_version", "digest", "result"}},
		{name: "merged_from_entry", at: "properties/merged_from/items", want: []string{"digest", "name", "version"}},
		{name: "target", at: "properties/target", want: []string{"identity", "kind", "root"}},
		{name: "configuration", at: "properties/configurations/items", want: []string{"arch", "id", "os", "tags"}},
		{name: "consumers", at: "properties/consumers", want: []string{"declared", "loaded", "unavailable"}},
		{name: "consumer_loaded", at: "properties/consumers/properties/loaded/items", want: []string{"id", "path", "role"}},
		{name: "consumer_unavailable", at: "properties/consumers/properties/unavailable/items", want: []string{"id", "reason", "role"}},
		{name: "edge_evaluation", at: reportEdgeItemPath, want: []string{"analyzer", "edge", "finding", "side", "state", "symbol"}},
		{name: "stale_suppression", at: reportStaleItemPath, want: []string{"analyzer", "code", "entry", "mechanism", "message", "position", "symbol"}},
		{name: "stale_suppression_entry", at: reportStaleItemPath + "/properties/entry", want: []string{"code", "path", "reason", "symbol"}},
		{name: "stale_suppression_position", at: reportStaleItemPath + "/properties/position", want: []string{"column", "line", "path"}},
		{name: "declared_gap", at: "properties/declared_gaps/items", want: []string{"analyzer", "capability", "fixture", "reason", "symbol"}},
		{name: "test_file_rule", at: "properties/test_file_rules/items", want: []string{"matched", "rule"}},
		{
			name: "totals",
			at:   "properties/totals",
			want: []string{
				"by_severity", "deletable_lines", "findings", "omitted", "pending",
				"reasons_recorded", "stale_suppressions", "suppressions_in_effect",
			},
		},
		{name: "by_severity", at: "properties/totals/properties/by_severity", want: []string{"allow", "deny", "warn"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := propertyNames(schema, tc.at); !slices.Equal(got, tc.want) {
				t.Errorf("propertyNames(%s, %q) = %v, want %v", reportSchemaPath, tc.at, got, tc.want)
			}
		})
	}
}

// TestReportSchemaRequiredMembers pins what a report must state rather than
// leave to a reader's default: a report is machine output, so every member of
// the envelope and of every record is present.
func TestReportSchemaRequiredMembers(t *testing.T) {
	schema := loadReportSchema(t)
	cases := []struct {
		name string
		at   string
		want []string
	}{
		{
			name: "envelope",
			at:   "",
			want: []string{
				"schema_version", "contract_version", "analyzer", "target", "configurations",
				"consumers", "findings", "edge_evaluations", "stale_suppressions", "declared_gaps",
				"excluded_by_cgo", "test_file_rules", "totals",
			},
		},
		{name: "analyzer", at: "properties/analyzer", want: []string{"name", "version", "languages", "schema_versions_accepted", "conformance"}},
		{name: "conformance", at: reportConformancePath, want: []string{"corpus_version", "result", "digest"}},
		{name: "target", at: "properties/target", want: []string{"kind", "root", "identity"}},
		{name: "configuration", at: "properties/configurations/items", want: []string{"id", "os", "arch", "tags"}},
		{name: "consumers", at: "properties/consumers", want: []string{"declared", "loaded", "unavailable"}},
		{name: "edge_evaluation", at: reportEdgeItemPath, want: []string{"edge", "side", "symbol", "state"}},
		{name: "stale_suppression", at: reportStaleItemPath, want: []string{"code", "mechanism", "entry", "position", "symbol", "message"}},
		{name: "stale_suppression_entry", at: reportStaleItemPath + "/properties/entry", want: []string{"code", "path", "reason"}},
		{name: "stale_suppression_position", at: reportStaleItemPath + "/properties/position", want: []string{"path", "line", "column"}},
		{name: "declared_gap", at: "properties/declared_gaps/items", want: []string{"fixture", "capability", "reason"}},
		{name: "test_file_rule", at: "properties/test_file_rules/items", want: []string{"rule", "matched"}},
		{
			name: "totals",
			at:   "properties/totals",
			want: []string{
				"findings", "by_severity", "deletable_lines", "suppressions_in_effect",
				"reasons_recorded", "stale_suppressions", "pending", "omitted",
			},
		},
		{name: "by_severity", at: "properties/totals/properties/by_severity", want: []string{"allow", "warn", "deny"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := reportRequiredAt(schema, tc.at); !slices.Equal(got, tc.want) {
				t.Errorf("reportRequiredAt(%s, %q) = %v, want %v", reportSchemaPath, tc.at, got, tc.want)
			}
		})
	}
}

// TestReportSchemaVocabulariesAreTheContractVocabularies pins that the
// envelope names no vocabulary of its own where a committed document already
// declares one.
func TestReportSchemaVocabulariesAreTheContractVocabularies(t *testing.T) {
	schema := loadReportSchema(t)
	kinds := loadKinds(t)
	results := loadSchema(t, spec.Corpus, resultsSchemaPath)
	config := loadSchema(t, spec.Contract, configSchemaPath)

	cases := []struct {
		name   string
		at     string
		want   []string
		source string
	}{
		{name: "languages", at: "properties/analyzer/properties/languages/items", want: kinds.Languages, source: kindsPath},
		{name: "conformance_result", at: reportConformancePath + "/properties/result", want: enumAt(results, "properties/result"), source: resultsSchemaPath},
		{name: "target_kind", at: "properties/target/properties/kind", want: enumAt(config, "properties/target/properties/kind"), source: configSchemaPath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.want) == 0 {
				t.Fatalf("Setup: %s declares no vocabulary for %s", tc.source, tc.name)
			}
			if got := enumAt(schema, tc.at); !slices.Equal(got, tc.want) {
				t.Errorf("enumAt(%s, %q) = %v, want %v from %s", reportSchemaPath, tc.at, got, tc.want, tc.source)
			}
		})
	}

	t.Run("by_severity", func(t *testing.T) {
		if got := propertyNames(schema, "properties/totals/properties/by_severity"); !slices.Equal(got, slices.Sorted(slices.Values(kinds.Severities))) {
			t.Errorf("propertyNames(%s, by_severity) = %v, want the severities %s declares, %v", reportSchemaPath, got, kindsPath, kinds.Severities)
		}
	})

	own := []struct {
		name string
		at   string
		want []string
	}{
		{name: "edge_side", at: reportEdgeItemPath + "/properties/side", want: []string{"provides", "used_by"}},
		{name: "edge_state", at: reportEdgeItemPath + "/properties/state", want: []string{"live", "dead", "absent"}},
		{name: "suppression_mechanism", at: reportStaleItemPath + "/properties/mechanism", want: []string{"inline", "ignore", "baseline"}},
		{name: "consumer_role", at: "properties/consumers/properties/loaded/items/properties/role", want: []string{"consumer"}},
	}
	for _, tc := range own {
		t.Run(tc.name, func(t *testing.T) {
			if got := enumAt(schema, tc.at); !slices.Equal(got, tc.want) {
				t.Errorf("enumAt(%s, %q) = %v, want %v", reportSchemaPath, tc.at, got, tc.want)
			}
		})
	}
}

// TestReportSchemaEdgeEvaluationCarriesAFindingExactlyWhenDead pins the
// conditional that makes a pending finding inseparable from the state that
// produces it: a dead side carries the finding, and every other state carries
// none.
func TestReportSchemaEdgeEvaluationCarriesAFindingExactlyWhenDead(t *testing.T) {
	schema := loadReportSchema(t)
	item := objectAt(schema, reportEdgeItemPath)

	if got := reportConstAt(item, "if/properties/state"); got != "dead" {
		t.Errorf("schema[%s].if.properties.state.const = %q, want %q", reportEdgeItemPath, got, "dead")
	}
	if got := requiredAt(item, "then"); !slices.Equal(got, []string{"finding"}) {
		t.Errorf("schema[%s].then.required = %v, want [finding]", reportEdgeItemPath, got)
	}
	if got := requiredAt(item, "else/not"); !slices.Equal(got, []string{"finding"}) {
		t.Errorf("schema[%s].else.not.required = %v, want [finding], so a state other than dead carries none", reportEdgeItemPath, got)
	}
	if slices.Contains(requiredAt(schema, reportEdgeItemPath), "finding") {
		t.Errorf("schema[%s].required = %v, want finding required by the conditional alone", reportEdgeItemPath, requiredAt(schema, reportEdgeItemPath))
	}
}

// TestReportSchemaStaleSuppressionCodeIsFixed pins that a stale-suppression
// record can carry no code but DS1703, the kind that is fixed on at deny, and
// that the conformance digest is the one form a reader can recompute.
func TestReportSchemaStaleSuppressionCodeIsFixed(t *testing.T) {
	schema := loadReportSchema(t)
	if got := reportConstAt(schema, reportStaleItemPath+"/properties/code"); got != reportStaleCode {
		t.Errorf("schema[%s].properties.code.const = %q, want %q", reportStaleItemPath, got, reportStaleCode)
	}
	fixed := false
	for _, row := range loadKinds(t).Kinds {
		if row.Code == reportStaleCode {
			fixed = row.Fixed != nil && *row.Fixed
		}
	}
	if !fixed {
		t.Errorf("%s row %s carries fixed = %t, want the record's constant code to name the kind that is fixed on", kindsPath, reportStaleCode, fixed)
	}
	if got, _ := objectAt(schema, reportConformancePath+"/properties/digest")["pattern"].(string); got != reportDigestPattern {
		t.Errorf("schema[%s].properties.digest.pattern = %q, want %q", reportConformancePath, got, reportDigestPattern)
	}
}
