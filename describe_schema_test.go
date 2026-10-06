package spec_test

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v6"
)

const (
	describeSchemaPath = "contract/describe.schema.json"
	describeExamples   = "describe"
)

// TestDescribeExamplesAreInstancesOfTheDescribeSchema holds every published describe
// document, the one with a conformance record and the one without, to the schema.
func TestDescribeExamplesAreInstancesOfTheDescribeSchema(t *testing.T) {
	files := exampleFiles(t, describeExamples)
	for _, file := range files {
		t.Run(exampleName(file), func(t *testing.T) {
			validateAgainst(t, describeSchemaPath, readExample(t, describeExamples, file))
		})
	}
	var recorded, unrecorded int
	for _, file := range files {
		var doc struct {
			Conformance *struct{} `json:"conformance"`
		}
		if err := json.Unmarshal(readExample(t, describeExamples, file), &doc); err != nil {
			t.Fatalf("Setup: json.Unmarshal(%s): %v", file, err)
		}
		if doc.Conformance == nil {
			unrecorded++
		} else {
			recorded++
		}
	}
	if recorded == 0 || unrecorded == 0 {
		t.Errorf("describe examples with a conformance record = %d, without = %d, want at least one of each", recorded, unrecorded)
	}
}

// TestDescribeExamplesRecordThePublishedCorpusVersion ties the conformance record of
// every document under examples/, a negative included, to corpus/corpus.json: a
// describe document's and a report's analyzer member state the analyzer of this
// contract version, so its record names the corpus this repository publishes.
func TestDescribeExamplesRecordThePublishedCorpusVersion(t *testing.T) {
	want := loadCorpus(t).CorpusVersion
	var seen int
	err := fs.WalkDir(spec.Examples, examplesDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".json") {
			return err
		}
		data, err := fs.ReadFile(spec.Examples, p)
		if err != nil {
			return err
		}
		type record struct {
			CorpusVersion string `json:"corpus_version"`
		}
		var doc struct {
			Conformance *record `json:"conformance"`
			Analyzer    *struct {
				Conformance *record `json:"conformance"`
			} `json:"analyzer"`
		}
		if json.Unmarshal(data, &doc) != nil {
			return nil
		}
		records := map[string]*record{"conformance": doc.Conformance}
		if doc.Analyzer != nil {
			records["analyzer.conformance"] = doc.Analyzer.Conformance
		}
		for member, rec := range records {
			if rec == nil {
				continue
			}
			seen++
			if got := rec.CorpusVersion; got != want {
				t.Errorf("Document(%s).%s.corpus_version = %q, want the one %s publishes, %q", p, member, got, corpusPath, want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Setup: fs.WalkDir(%s): %v", examplesDir, err)
	}
	if seen == 0 {
		t.Fatalf("Setup: describe documents with a conformance record under %s = 0, want the published ones", examplesDir)
	}
}

// TestDescribeSchemaStatesEachMemberAsTheReportsAnalyzerDoes pins every member the two
// documents share to one shape, so an analyzer's describe document and its report's
// analyzer object cannot drift apart.
func TestDescribeSchemaStatesEachMemberAsTheReportsAnalyzerDoes(t *testing.T) {
	describe := decodeContractSchema(t, describeSchemaPath)
	report := decodeContractSchema(t, reportSchemaPath)
	analyzer := schemaAt(t, report, "analyzer")
	shared := map[string][]string{
		"name":                     {"analyzer", "name"},
		"version":                  {"analyzer", "version"},
		"contract_version":         {"contract_version"},
		"schema_versions_accepted": {"analyzer", "schema_versions_accepted"},
		"languages":                {"analyzer", "languages"},
		"conformance":              {"analyzer", "conformance"},
	}
	for member, at := range shared {
		t.Run(member, func(t *testing.T) {
			got, want := withoutDescriptions(schemaAt(t, describe, member)), withoutDescriptions(schemaAt(t, report, at...))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("describe schema %s = %v, want the report schema's %v", member, got, want)
			}
		})
	}
	required, _ := describe["required"].([]any)
	analyzerRequired, _ := analyzer["required"].([]any)
	var got, want []string
	for _, r := range required {
		got = append(got, r.(string))
	}
	for _, r := range analyzerRequired {
		if name := r.(string); name != "conformance" {
			want = append(want, name)
		}
	}
	want = append(want, "contract_version")
	if slices.Sort(got); !slices.Equal(got, sortedStrings(want)) {
		t.Errorf("describe schema required = %v, want the analyzer object's members without conformance, and contract_version: %v", got, sortedStrings(want))
	}
}

func sortedStrings(values []string) []string {
	out := slices.Clone(values)
	slices.Sort(out)
	return out
}

// TestDescribeSchemaIsClosedAndDescribed holds the describe schema to the dialect and
// the closed key sets every published schema carries.
func TestDescribeSchemaIsClosedAndDescribed(t *testing.T) {
	schema := decodeContractSchema(t, describeSchemaPath)
	if schema["$schema"] != reportSchemaDialect {
		t.Errorf("%s $schema = %v, want %q", describeSchemaPath, schema["$schema"], reportSchemaDialect)
	}
	if got := openObjects(schema, ""); len(got) != 0 {
		t.Errorf("openObjects(%s) = %v, want every object schema to close its key set", describeSchemaPath, got)
	}
	if got := undescribedProperties(schema, ""); len(got) != 0 {
		t.Errorf("undescribedProperties(%s) = %v, want a description on every property", describeSchemaPath, got)
	}
}
