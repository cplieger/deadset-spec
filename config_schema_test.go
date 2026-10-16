package spec_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"testing"

	"github.com/cplieger/deadset-spec/v8"
)

const configSchemaPath = "contract/config.schema.json"

// ownerVocabulary is the closed set of x-owner values the schema's description declares:
// product, analyzer, orchestrator and the analyzer of each language contract/languages.json lists.
func ownerVocabulary() []string {
	owners := []string{"product", "analyzer", "orchestrator"}
	for _, language := range contractLanguages() {
		owners = append(owners, language+"-analyzer")
	}
	return owners
}

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
// patternProperties, items and the branches of a oneOf. A child that is not a JSON object is
// listed with a nil node so the structural test reports it instead of skipping it.
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
		for i, branch := range branchesOf(node) {
			child, _ := branch.(schemaNode)
			visit(child, branchPath(path, i), false, false)
		}
	}
	visit(root, "root", false, false)
	return keys
}

// branchesOf returns the subschemas of a node's oneOf, and nil for a node that states one
// shape.
func branchesOf(node schemaNode) []any {
	branches, _ := node["oneOf"].([]any)
	return branches
}

// branchPath is the path walkSchema gives the i-th branch of a node's oneOf.
func branchPath(parent string, i int) string {
	return joinPath(parent, fmt.Sprintf("<oneOf.%d>", i))
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
			if branches := branchesOf(key.node); len(branches) != 0 {
				// A key whose value takes one of several shapes states each
				// shape in a branch and nothing of its own, so the branches
				// carry the type and the closed key list and this node carries
				// neither: a type here would admit a value no branch describes.
				if len(branches) < 2 {
					t.Errorf("schema[%s].oneOf holds %d branch(es), want the shapes the key admits", key.path, len(branches))
				}
				for _, keyword := range []string{"type", "properties", "patternProperties", "items", "enum", "pattern"} {
					if value, ok := key.node[keyword]; ok {
						t.Errorf("schema[%s].%s = %v, want it declared in every branch of the oneOf instead", key.path, keyword, value)
					}
				}
				return
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
	// No key is required at the root: each source is optional and the
	// resolved configuration is what must name a target kind.
	if target := lookupKey(t, keys, "target"); target.required {
		t.Errorf("schema[root].required contains target = %t, want false", target.required)
	}
	if root := lookupKey(t, keys, "root"); len(stringSlice(root.node["required"])) != 0 {
		t.Errorf("schema[root].required = %v, want empty", root.node["required"])
	}
}

// TestConfigSchema_TargetKindIsOneKindOrOnePerLanguage pins the two shapes of the target kind:
// a string of the two kinds, and an object naming every language contract/languages.json marks
// takes_target_kind, each member required and a kind, with no default anywhere, because a kind
// is never defaulted.
func TestConfigSchema_TargetKindIsOneKindOrOnePerLanguage(t *testing.T) {
	kind := lookupKey(t, walkSchema(loadConfigSchema(t)), "target.kind")
	branches := branchesOf(kind.node)
	if len(branches) != 2 {
		t.Fatalf("schema[target.kind].oneOf holds %d shapes, want 2", len(branches))
	}
	one, _ := branches[0].(schemaNode)
	perLanguage, _ := branches[1].(schemaNode)
	kinds := []string{"application", "library"}
	if one["type"] != "string" || !slices.Equal(stringSlice(one["enum"]), kinds) {
		t.Errorf("schema[target.kind].oneOf[0] = %v, want a string of %q", one, kinds)
	}
	var registry languagesDocument
	strictDecode(t, languagesPath, &registry)
	var languages []string
	for _, row := range registry.Languages {
		if row.TakesTargetKind {
			languages = append(languages, row.Language)
		}
	}
	members, _ := perLanguage["properties"].(schemaNode)
	if perLanguage["type"] != "object" || perLanguage["additionalProperties"] != false {
		t.Errorf("schema[target.kind].oneOf[1] type = %v, additionalProperties = %v, want a closed object", perLanguage["type"], perLanguage["additionalProperties"])
	}
	if got := slices.Sorted(maps.Keys(members)); !slices.Equal(got, slices.Sorted(slices.Values(languages))) {
		t.Errorf("schema[target.kind].oneOf[1] members = %q, want the languages that take a target kind, %q", got, languages)
	}
	if got := stringSlice(perLanguage["required"]); !slices.Equal(got, languages) {
		t.Errorf("schema[target.kind].oneOf[1].required = %q, want every language that takes a target kind, in the order contract/languages.json lists them, %q", got, languages)
	}
	for _, language := range languages {
		member, _ := members[language].(schemaNode)
		if member["type"] != "string" || !slices.Equal(stringSlice(member["enum"]), kinds) {
			t.Errorf("schema[target.kind].oneOf[1].%s = %v, want a string of %q", language, member, kinds)
		}
		if def, ok := member["default"]; ok {
			t.Errorf("schema[target.kind].oneOf[1].%s.default = %v, want absent", language, def)
		}
	}
	for i, branch := range []schemaNode{one, perLanguage} {
		if def, ok := branch["default"]; ok {
			t.Errorf("schema[target.kind].oneOf[%d].default = %v, want absent", i, def)
		}
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
			if owner, _ := key.node["x-owner"].(string); !slices.Contains(ownerVocabulary(), owner) {
				t.Errorf("schema[%s].x-owner = %v, want one of %v", key.path, key.node["x-owner"], ownerVocabulary())
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

// TestConfigSchema_EveryObjectIsASectionOrASetting pins the rule the schema's
// description states, so the presence of a default never again decides which of
// the two an object is. A setting carries x-setting true, the default a run uses
// when no source names it, and members that are all required and carry no
// default of their own. A section carries no mark: a section of declared keys
// carries no default, because each of its keys carries one, and a section keyed
// by a pattern carries the default a run uses when no source names a key. The
// provenance object is neither, and states so with x-ignored-by-resolution.
func TestConfigSchema_EveryObjectIsASectionOrASetting(t *testing.T) {
	for _, key := range walkSchema(loadConfigSchema(t)) {
		if !key.property || key.node["type"] != "object" {
			continue
		}
		if ignored, _ := key.node["x-ignored-by-resolution"].(bool); ignored {
			continue
		}
		t.Run(key.path, func(t *testing.T) {
			props, hasProps := key.node["properties"].(map[string]any)
			_, hasPatterns := key.node["patternProperties"].(map[string]any)
			_, hasDefault := key.node["default"]
			marked, _ := key.node["x-setting"].(bool)

			if !marked {
				if present, ok := key.node["x-setting"]; ok {
					t.Errorf("schema[%s].x-setting = %v, want true or the keyword absent", key.path, present)
				}
				if want := hasPatterns && !hasProps; hasDefault != want {
					t.Errorf("schema[%s] carries no x-setting and has default = %t, want %t: a section of declared keys carries none and a section keyed by a pattern carries one", key.path, hasDefault, want)
				}
				return
			}
			if !hasDefault {
				t.Errorf("schema[%s].default = absent, want the value a run uses when no source names the setting", key.path)
			}
			if hasPatterns {
				t.Errorf("schema[%s].patternProperties = declared, want a setting to declare its members under properties", key.path)
			}
			if !hasProps {
				t.Fatalf("schema[%s].properties = absent, want a setting to declare the members it is replaced with", key.path)
			}
			if got, want := stringSlice(key.node["required"]), slices.Sorted(maps.Keys(props)); !slices.Equal(slices.Sorted(slices.Values(got)), want) {
				t.Errorf("schema[%s].required = %v, want every member %v: a setting is replaced whole", key.path, got, want)
			}
			for _, name := range slices.Sorted(maps.Keys(props)) {
				member, _ := props[name].(schemaNode)
				if _, ok := member["default"]; ok {
					t.Errorf("schema[%s].%s.default = %v, want absent: a member of a setting is never resolved on its own", key.path, name, member["default"])
				}
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

// configurationsItemsPath is the path walkSchema gives one entry of the declared build
// matrix, whose two branches are the two shapes a configuration takes.
const configurationsItemsPath = "analysis.configurations.<items>"

// TestConfigSchema_AConfigurationEntryIsOnePlatformOrOneProject pins the two shapes the
// build matrix admits and the members that tell them apart: a platform names an operating
// system, an architecture and its build tags, a project names one compiler configuration
// file, and each closes its key set over its own members, so an entry carrying members of
// both is refused rather than read as the shape a reader assumed.
func TestConfigSchema_AConfigurationEntryIsOnePlatformOrOneProject(t *testing.T) {
	keys := walkSchema(loadConfigSchema(t))
	if got := len(branchesOf(lookupKey(t, keys, configurationsItemsPath).node)); got != 2 {
		t.Fatalf("schema[%s].oneOf holds %d branch(es), want the two shapes a configuration takes", configurationsItemsPath, got)
	}
	for _, tc := range []struct {
		name       string
		properties []string
		required   []string
		branch     int
	}{
		{name: "platform", branch: 0, properties: []string{"arch", "id", "os", "tags"}, required: []string{"id", "os", "arch"}},
		{name: "project", branch: 1, properties: []string{"id", "project"}, required: []string{"id", "project"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			at := branchPath(configurationsItemsPath, tc.branch)
			node := lookupKey(t, keys, at).node
			props, _ := node["properties"].(map[string]any)
			if got := slices.Sorted(maps.Keys(props)); !slices.Equal(got, tc.properties) {
				t.Errorf("schema[%s].properties = %v, want %v", at, got, tc.properties)
			}
			if got := stringSlice(node["required"]); !slices.Equal(got, tc.required) {
				t.Errorf("schema[%s].required = %v, want %v", at, got, tc.required)
			}
		})
	}
}
