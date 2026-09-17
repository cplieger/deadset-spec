package spec_test

import (
	"encoding/json"
	"io/fs"
	"maps"
	"slices"
	"testing"

	"github.com/cplieger/deadset-spec"
)

const configSchemaPath = "contract/config.schema.json"

// ownerVocabulary is the closed set of x-owner values the schema's description declares.
var ownerVocabulary = []string{"product", "analyzer", "orchestrator", "go-analyzer", "ts-analyzer"}

// schemaTypes is the set of JSON Schema type names a subschema may declare.
var schemaTypes = []string{"object", "array", "string", "boolean", "integer", "number"}

type schemaNode = map[string]any

// declaredKey is one subschema and the setting path that reaches it: "root" for the
// document, "target.kind" for a property, "severity.<pattern>" for a patternProperties
// value and "analysis.configurations.<items>" for an array's element schema.
type declaredKey struct {
	node     schemaNode
	path     string
	property bool
	required bool
}

func loadConfigSchema(t *testing.T) schemaNode {
	t.Helper()
	data, err := fs.ReadFile(spec.Contract, configSchemaPath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", configSchemaPath, err)
	}
	var root schemaNode
	if err := json.Unmarshal(data, &root); err != nil {
		t.Fatalf("Setup: json.Unmarshal(%q): %v", configSchemaPath, err)
	}
	return root
}

// walkSchema lists every subschema under root, the root first, following properties,
// patternProperties and items. A child that is not a JSON object is listed with a nil node so
// the structural test reports it instead of skipping it.
func walkSchema(root schemaNode) []declaredKey {
	var keys []declaredKey
	var visit func(node schemaNode, path string, property, required bool)
	visit = func(node schemaNode, path string, property, required bool) {
		keys = append(keys, declaredKey{node: node, path: path, property: property, required: required})
		requiredNames := stringSlice(node["required"])
		if props, ok := node["properties"].(map[string]any); ok {
			for _, name := range slices.Sorted(maps.Keys(props)) {
				child, _ := props[name].(schemaNode)
				visit(child, joinPath(path, name), true, slices.Contains(requiredNames, name))
			}
		}
		if pats, ok := node["patternProperties"].(map[string]any); ok {
			for _, pattern := range slices.Sorted(maps.Keys(pats)) {
				child, _ := pats[pattern].(schemaNode)
				visit(child, joinPath(path, "<pattern>"), false, false)
			}
		}
		if items, ok := node["items"].(schemaNode); ok {
			visit(items, joinPath(path, "<items>"), false, false)
		}
	}
	visit(root, "root", false, false)
	return keys
}

func joinPath(parent, name string) string {
	if parent == "root" {
		return name
	}
	return parent + "." + name
}

// stringSlice returns the strings of a decoded JSON array, and nil for anything else.
func stringSlice(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func lookupKey(t *testing.T, keys []declaredKey, path string) declaredKey {
	t.Helper()
	i := slices.IndexFunc(keys, func(k declaredKey) bool { return k.path == path })
	if i < 0 {
		t.Fatalf("walkSchema(%q) declares no key %q, want it declared", configSchemaPath, path)
	}
	return keys[i]
}

func TestConfigSchema_NoKeyAdmitsArbitraryNesting(t *testing.T) {
	for _, key := range walkSchema(loadConfigSchema(t)) {
		t.Run(key.path, func(t *testing.T) {
			if key.node == nil {
				t.Fatalf("schema[%s] = %v, want a subschema object", key.path, key.node)
			}
			if ref, ok := key.node["$ref"]; ok {
				t.Errorf("schema[%s].$ref = %v, want no reference: every key is declared inline", key.path, ref)
			}
			typ, _ := key.node["type"].(string)
			if !slices.Contains(schemaTypes, typ) {
				t.Fatalf("schema[%s].type = %v, want one of %v", key.path, key.node["type"], schemaTypes)
			}
			switch typ {
			case "object":
				if closed, ok := key.node["additionalProperties"].(bool); !ok || closed {
					t.Errorf("schema[%s].additionalProperties = %v, want false", key.path, key.node["additionalProperties"])
				}
				_, hasProps := key.node["properties"].(map[string]any)
				_, hasPatterns := key.node["patternProperties"].(map[string]any)
				if !hasProps && !hasPatterns {
					t.Errorf("schema[%s].properties = %v, patternProperties = %v, want a key list on every object", key.path, key.node["properties"], key.node["patternProperties"])
				}
			case "array":
				if _, ok := key.node["items"].(schemaNode); !ok {
					t.Errorf("schema[%s].items = %v, want a subschema for every array", key.path, key.node["items"])
				}
			}
		})
	}
}

func TestConfigSchema_TargetKindIsRequiredWithNoDefault(t *testing.T) {
	keys := walkSchema(loadConfigSchema(t))
	kind := lookupKey(t, keys, "target.kind")
	if def, ok := kind.node["default"]; ok {
		t.Errorf("schema[target.kind].default = %v, want absent: the target kind is declared, never defaulted", def)
	}
	if !kind.required {
		t.Errorf("schema[target].required contains kind = %t, want true", kind.required)
	}
	if target := lookupKey(t, keys, "target"); !target.required {
		t.Errorf("schema[root].required contains target = %t, want true", target.required)
	}
}

func TestConfigSchema_OnlyProvenanceIsIgnoredByResolution(t *testing.T) {
	var ignored []string
	for _, key := range walkSchema(loadConfigSchema(t)) {
		if marked, ok := key.node["x-ignored-by-resolution"].(bool); ok && marked {
			ignored = append(ignored, key.path)
		}
	}
	if want := []string{"provenance"}; !slices.Equal(ignored, want) {
		t.Errorf("keys with x-ignored-by-resolution = %q, want %q", ignored, want)
	}
}

func TestConfigSchema_EveryKeyCarriesOwnerAndDescription(t *testing.T) {
	for _, key := range walkSchema(loadConfigSchema(t)) {
		if !key.property {
			continue
		}
		t.Run(key.path, func(t *testing.T) {
			if owner, _ := key.node["x-owner"].(string); !slices.Contains(ownerVocabulary, owner) {
				t.Errorf("schema[%s].x-owner = %v, want one of %v", key.path, key.node["x-owner"], ownerVocabulary)
			}
			if desc, _ := key.node["description"].(string); desc == "" {
				t.Errorf("schema[%s].description = %v, want a non-empty sentence", key.path, key.node["description"])
			}
		})
	}
}

// TestConfigSchema_OptionalLeavesCarryDefaults pins the rule the schema's description states: a
// required key has no default, and every other non-object key declares one, except
// contract_version, whose default is the product's own version and so has no literal.
func TestConfigSchema_OptionalLeavesCarryDefaults(t *testing.T) {
	for _, key := range walkSchema(loadConfigSchema(t)) {
		if !key.property || key.node["type"] == "object" || key.path == "contract_version" {
			continue
		}
		t.Run(key.path, func(t *testing.T) {
			_, hasDefault := key.node["default"]
			if hasDefault == key.required {
				t.Errorf("schema[%s] has default = %t with required = %t, want a default exactly when optional", key.path, hasDefault, key.required)
			}
		})
	}
}

func TestConfigSchema_RequiredNamesDeclaredKeys(t *testing.T) {
	for _, key := range walkSchema(loadConfigSchema(t)) {
		required := stringSlice(key.node["required"])
		if len(required) == 0 {
			continue
		}
		t.Run(key.path, func(t *testing.T) {
			props, _ := key.node["properties"].(map[string]any)
			for _, name := range required {
				if _, ok := props[name]; !ok {
					t.Errorf("schema[%s].required names %q, want every required name declared under properties %v", key.path, name, slices.Sorted(maps.Keys(props)))
				}
			}
		})
	}
}
