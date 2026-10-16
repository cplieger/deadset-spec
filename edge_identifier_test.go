package spec_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// edgeIdentifierRef is the definition that owns the edge identifier form.
const edgeIdentifierRef = findingSchemaPath + "#/$defs/edge_identifier"

// edgeIdentifiers records the schema of every edge identifier under node by
// its location: each member named edge a properties keyword declares, and the
// member names of the edges document.
func edgeIdentifiers(node any, at string, found map[string]map[string]any) {
	switch v := node.(type) {
	case map[string]any:
		if properties, ok := v["properties"].(map[string]any); ok {
			if edge, ok := properties["edge"].(map[string]any); ok {
				found[at+"/properties/edge"] = edge
			}
			if edges, ok := properties["edges"].(map[string]any); ok {
				if names, ok := edges["propertyNames"].(map[string]any); ok {
					found[at+"/properties/edges/propertyNames"] = names
				}
			}
		}
		for _, key := range slices.Sorted(maps.Keys(v)) {
			edgeIdentifiers(v[key], at+"/"+key, found)
		}
	case []any:
		for i, item := range v {
			edgeIdentifiers(item, fmt.Sprintf("%s/%d", at, i), found)
		}
	}
}

// TestEveryEdgeIdentifierTakesTheOneDefinition pins the edge identifier form
// to one owner, finding.schema.json's edge_identifier definition: every edge
// identifier a contract schema declares references it, and a corpus schema,
// which states every shape inline, states exactly its shape.
func TestEveryEdgeIdentifierTakesTheOneDefinition(t *testing.T) {
	documents, err := embeddedSchemas()
	if err != nil {
		t.Fatalf("Setup: embeddedSchemas(): %v", err)
	}
	ownerFile, ownerPointer, _ := strings.Cut(edgeIdentifierRef, "#")
	var finding map[string]any
	if err := json.Unmarshal(documents[ownerFile], &finding); err != nil {
		t.Fatalf("Setup: decode %s: %v", ownerFile, err)
	}
	owner := objectAt(finding, strings.TrimPrefix(ownerPointer, "/"))
	if owner == nil {
		t.Fatalf("Setup: %s declares nothing, want the edge identifier definition", edgeIdentifierRef)
	}
	shape := withoutDescriptions(owner)

	contract, corpusInline := 0, 0
	for _, name := range slices.Sorted(maps.Keys(documents)) {
		var root any
		if err := json.Unmarshal(documents[name], &root); err != nil {
			t.Fatalf("Setup: decode %s: %v", name, err)
		}
		found := map[string]map[string]any{}
		edgeIdentifiers(root, "", found)
		for _, at := range slices.Sorted(maps.Keys(found)) {
			declared := found[at]
			if strings.HasPrefix(name, "corpus/") {
				corpusInline++
				if got := withoutDescriptions(declared); !reflect.DeepEqual(got, shape) {
					t.Errorf("%s%s states %s, want the shape of %s, %s", name, at, asJSON(t, got), edgeIdentifierRef, asJSON(t, shape))
				}
				continue
			}
			contract++
			ref, _ := declared["$ref"].(string)
			file, pointer, _ := strings.Cut(ref, "#")
			target := name
			if file != "" {
				target = path.Join(path.Dir(name), file)
			}
			if target+"#"+pointer != edgeIdentifierRef {
				t.Errorf("%s%s references %q, want %s", name, at, ref, edgeIdentifierRef)
			}
		}
	}
	// The edges document's names, a report's evaluation and a finding's
	// details: a missing one means a location moved out of this check.
	if contract < 3 || corpusInline == 0 {
		t.Errorf("found %d contract and %d corpus edge identifiers, want at least 3 and 1", contract, corpusInline)
	}
}
