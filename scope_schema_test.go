package spec_test

import (
	"encoding/json"
	"io/fs"
	"maps"
	"slices"
	"testing"

	"github.com/cplieger/deadset-spec/v4"
)

const (
	scopeSchemaPath = "contract/scope.schema.json"
	scopeExamples   = "scope"
)

// scopeMemberPaths lists the member paths of a scope document, an array element written as
// [], so two documents, or a document and the schema, are compared by what they name.
func scopeMemberPaths(at string, node any) []string {
	var out []string
	switch v := node.(type) {
	case map[string]any:
		for _, name := range slices.Sorted(maps.Keys(v)) {
			child := at + "/" + name
			out = append(out, child)
			out = append(out, scopeMemberPaths(child, v[name])...)
		}
	case []any:
		for _, item := range v {
			out = append(out, scopeMemberPaths(at+"/[]", item)...)
		}
	}
	return out
}

// scopeSchemaMembers lists every member path the scope schema declares, in the same spelling.
func scopeSchemaMembers(at string, node map[string]any) []string {
	var out []string
	if props, ok := node["properties"].(map[string]any); ok {
		for _, name := range slices.Sorted(maps.Keys(props)) {
			child, _ := props[name].(map[string]any)
			out = append(out, at+"/"+name)
			out = append(out, scopeSchemaMembers(at+"/"+name, child)...)
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		out = append(out, scopeSchemaMembers(at+"/[]", items)...)
	}
	return out
}

func decodeContractSchema(t *testing.T, schemaPath string) map[string]any {
	t.Helper()
	data, err := fs.ReadFile(spec.Contract, schemaPath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", schemaPath, err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("Setup: json.Unmarshal(%q): %v", schemaPath, err)
	}
	return schema
}

// schemaAt walks a decoded schema along properties and items, "[]" naming an array's items.
func schemaAt(t *testing.T, schema map[string]any, path ...string) map[string]any {
	t.Helper()
	node := schema
	for _, step := range path {
		var next any
		if step == "[]" {
			next = node["items"]
		} else {
			props, _ := node["properties"].(map[string]any)
			next = props[step]
		}
		child, ok := next.(map[string]any)
		if !ok {
			t.Fatalf("Setup: the schema declares nothing at %q, want a subschema", path)
		}
		node = child
	}
	return node
}

func TestScopeExamplesAreInstancesOfTheScopeSchema(t *testing.T) {
	for _, name := range exampleFiles(t, scopeExamples) {
		t.Run(exampleName(name), func(t *testing.T) {
			validateAgainst(t, scopeSchemaPath, readExample(t, scopeExamples, name))
		})
	}
}

// TestScopeExamplesNameEveryMemberTheSchemaDeclares holds the accepted documents to the whole key
// list, so a decoder run over them reads every member a scope document can carry.
func TestScopeExamplesNameEveryMemberTheSchemaDeclares(t *testing.T) {
	named := map[string]bool{}
	for _, name := range exampleFiles(t, scopeExamples) {
		var doc any
		if err := json.Unmarshal(readExample(t, scopeExamples, name), &doc); err != nil {
			t.Fatalf("Setup: json.Unmarshal(%s/%s): %v", scopeExamples, name, err)
		}
		for _, member := range scopeMemberPaths("", doc) {
			named[member] = true
		}
	}
	for _, member := range scopeSchemaMembers("", decodeContractSchema(t, scopeSchemaPath)) {
		if !named[member] {
			t.Errorf("examples/%s names %s in no document, want every member %s declares named by at least one", scopeExamples, member, scopeSchemaPath)
		}
	}
}

// TestScopeRolesAreTheRolesAReportRecords ties the two documents that spell a module's role: a
// report's consumer entries record the role the scope document gave the module.
func TestScopeRolesAreTheRolesAReportRecords(t *testing.T) {
	scope := decodeContractSchema(t, scopeSchemaPath)
	report := decodeContractSchema(t, reportSchemaPath)
	want := stringSlice(schemaAt(t, scope, "consumers", "[]", "role")["enum"])
	if len(want) == 0 {
		t.Fatalf("Setup: %s declares no consumer role, want the enum a consumer's role takes", scopeSchemaPath)
	}
	for _, list := range []string{"loaded", "unavailable"} {
		if got := stringSlice(schemaAt(t, report, "consumers", list, "[]", "role")["enum"]); !slices.Equal(got, want) {
			t.Errorf("%s consumers.%s[].role = %q, want %q, the consumer role %s declares", reportSchemaPath, list, got, want, scopeSchemaPath)
		}
	}
}
