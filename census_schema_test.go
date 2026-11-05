package spec_test

import (
	"maps"
	"reflect"
	"testing"
)

const (
	censusSchemaPath = "contract/census.schema.json"
	censusExamples   = "census"
)

// TestCensusExamplesAreInstancesOfTheCensusSchema holds every published census to
// the schema.
func TestCensusExamplesAreInstancesOfTheCensusSchema(t *testing.T) {
	for _, file := range exampleFiles(t, censusExamples) {
		t.Run(exampleName(file), func(t *testing.T) {
			validateAgainst(t, censusSchemaPath, readExample(t, censusExamples, file))
		})
	}
}

// TestCensusSchemaIsClosedAndDescribed holds the census schema to the dialect and
// the closed key sets every published schema carries.
func TestCensusSchemaIsClosedAndDescribed(t *testing.T) {
	schema := decodeContractSchema(t, censusSchemaPath)
	if schema["$schema"] != reportSchemaDialect {
		t.Errorf("%s $schema = %v, want %q", censusSchemaPath, schema["$schema"], reportSchemaDialect)
	}
	if got := openObjects(schema, ""); len(got) != 0 {
		t.Errorf("openObjects(%s) = %v, want every object schema to close its key set", censusSchemaPath, got)
	}
	if got := undescribedProperties(schema, ""); len(got) != 0 {
		t.Errorf("undescribedProperties(%s) = %v, want a description on every property", censusSchemaPath, got)
	}
}

// TestCensusNamesAnalyzersAsTheProviderListDoes holds the census's analyzer entry
// to a provider entry of the configuration schema: the name and the languages
// admit what the provider list admits, so every entry the run invokes is one the
// census can name.
func TestCensusNamesAnalyzersAsTheProviderListDoes(t *testing.T) {
	census := decodeContractSchema(t, censusSchemaPath)
	config := decodeContractSchema(t, "contract/config.schema.json")
	entry := schemaAt(t, config, "providers", "analyzers", "[]")
	installed, _ := entry["oneOf"].([]any)
	if len(installed) == 0 {
		t.Fatal("Setup: providers.analyzers items carry no oneOf")
	}
	provider, _ := installed[0].(map[string]any)
	for _, member := range []string{"name", "languages"} {
		got := withoutAnnotations(schemaAt(t, census, "analyzers", "[]", member))
		want := withoutAnnotations(schemaAt(t, provider, member))
		if !reflect.DeepEqual(got, want) {
			t.Errorf("census analyzers[].%s = %v, want the provider entry's %v", member, got, want)
		}
	}
}

// withoutAnnotations is a schema's validation keywords: the members no
// validator reads are dropped, the description and the x- annotations.
func withoutAnnotations(schema map[string]any) map[string]any {
	kept := maps.Clone(schema)
	for key := range kept {
		if key == "description" || len(key) > 2 && key[:2] == "x-" {
			delete(kept, key)
		}
	}
	return kept
}

// A provenance and a left-out reason name the paths the run was handed, and a
// path may hold a carriage return or a line feed, so the census admits both:
// JSON escapes them, and the document stays one line per member.
func TestCensusAdmitsALineBreakInAPathItNames(t *testing.T) {
	validateAgainst(t, censusSchemaPath, []byte(`{
  "languages": [{"language": "go", "provenance": "central: /w/a\nb\r.json"}],
  "analyzers": [{"name": "deadset-go", "languages": ["go"]}],
  "left_out": [{"language": "proto", "reason": "the consumer /w/sche\nma", "consumers": ["/w/sche\nma"]}]
}`))
}
