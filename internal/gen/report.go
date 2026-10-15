package gen

import (
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"slices"
	"strings"
)

const (
	reportTypesPath   = "report/report_gen.go"
	reportSchemaPath  = "contract/report.schema.json"
	findingSchemaPath = "contract/finding.schema.json"
)

// reportNames is the Go type of every schema location whose type is not the
// one its JSON type gives: each object, each closed vocabulary that is a Go
// type, and each value written by hand in the report package. A location an
// object sits at with no row here fails the generation, so a new object of a
// schema gets its name before it gets a type.
var reportNames = map[string]string{
	"report#":                     "Report",
	"report#/properties/analyzer": "Analyzer",
	"report#/properties/analyzer/properties/languages/items":                       "vocab.Language",
	"report#/properties/analyzer/properties/conformance":                           "Conformance",
	"report#/properties/analyzer/properties/conformance/properties/result":         "enum Result",
	"report#/properties/merged_from/items":                                         "InputReport",
	"report#/properties/target":                                                    "Target",
	"report#/properties/target/properties/kind":                                    "enum TargetKind",
	"report#/properties/target/properties/language_kinds":                          "map[vocab.Language]TargetKind",
	"report#/properties/configurations/items":                                      "hand Configuration",
	"report#/properties/configurations_not_built/items":                            "hand ConfigurationNotBuilt",
	"report#/properties/consumers":                                                 "Consumers",
	"report#/properties/consumers/properties/loaded/items":                         "LoadedConsumer",
	"report#/properties/consumers/properties/unavailable/items":                    "UnavailableConsumer",
	"report#/properties/components/items":                                          "ComponentListing",
	"report#/properties/edge_evaluations/items":                                    "EdgeEvaluation",
	"report#/properties/edge_evaluations/items/properties/side":                    "enum Side",
	"report#/properties/edge_evaluations/items/properties/state":                   "enum State",
	"report#/properties/stale_suppressions/items":                                  "StaleSuppression",
	"report#/properties/stale_suppressions/items/properties/code":                  vocabCode,
	"report#/properties/stale_suppressions/items/properties/entry":                 "SuppressionEntry",
	"report#/properties/stale_suppressions/items/properties/entry/properties/code": vocabCode,
	"report#/properties/stale_suppressions/items/properties/position":              "SuppressionPosition",
	"report#/properties/declared_gaps/items":                                       "DeclaredGap",
	"report#/properties/test_file_rules/items":                                     "TestFileRule",
	"report#/properties/type_error_skips/items":                                    "TypeErrorSkip",
	"report#/properties/notes/items":                                               "Note",
	"report#/properties/notes/items/properties/kind":                               "enum NoteKind",
	"report#/properties/unanswered_questions/items":                                "UnansweredQuestion",
	"report#/properties/conventions_applied/items":                                 "ConventionApplied",
	"report#/properties/totals":                                                    "Totals",
	"report#/properties/totals/properties/by_severity":                             "BySeverity",
	"report#/properties/totals/properties/withheld":                                "Withheld",
	"finding#":                                                            "Finding",
	"finding#/properties/code":                                            vocabCode,
	"finding#/properties/language":                                        "vocab.Language",
	"finding#/properties/symbol":                                          "Symbol",
	"finding#/properties/reachability_class":                              "vocab.Class",
	"finding#/properties/confidence":                                      "vocab.Class",
	"finding#/properties/liveness_relation":                               "enum Relation",
	"finding#/properties/component":                                       "Component",
	"finding#/properties/fixability":                                      "vocab.Fixability",
	"finding#/properties/severity":                                        "vocab.Severity",
	"finding#/properties/details":                                         "Details",
	"finding#/properties/details/properties/entry":                        "Entry",
	"finding#/properties/details/properties/entry/properties/code":        vocabCode,
	"finding#/properties/details/properties/sides/items":                  "EdgeSide",
	"finding#/properties/details/properties/sides/items/properties/side":  "enum Side",
	"finding#/properties/details/properties/sides/items/properties/state": "enum State",
	"finding#/$defs/position":                                             "Position",
	"finding#/$defs/positioned_symbol":                                    "Positioned",
}

// vocabCode is the Go type of an issue-kind code.
const vocabCode = "vocab.Code"

// initialisms are the member-name words a Go name spells in capitals.
var initialisms = map[string]string{"id": "ID", "os": "OS"}

// reportGen is one generation of the report package's types.
type reportGen struct {
	docs    map[string]*value
	structs []string
	emitted map[string]bool
	enums   map[string][]string
	order   []string
}

// reportTypes writes the report package's document types from the report and
// finding schemas: one struct per object in the schema's member order, a
// required member written whatever its value and an optional one omitted at
// its zero value, and one string type with its constants per closed
// vocabulary the package owns.
func reportTypes(fsys fs.FS, out map[string][]byte) error {
	g := &reportGen{docs: map[string]*value{}, emitted: map[string]bool{}, enums: map[string][]string{}}
	for doc, path := range map[string]string{"report": reportSchemaPath, "finding": findingSchemaPath} {
		data, held := out[path]
		if !held {
			var err error
			if data, err = fs.ReadFile(fsys, path); err != nil {
				return err
			}
		}
		v, err := parse(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		g.docs[doc] = v
	}
	if _, err := g.typeOf("report", "", g.docs["report"], true); err != nil {
		return err
	}
	w := &writer{}
	w.f("// Code generated by deadset-gen from contract/report.schema.json and contract/finding.schema.json. DO NOT EDIT.\n\n")
	w.f("package report\n\nimport \"github.com/cplieger/deadset-spec/v8/vocab\"\n\n")
	for _, name := range g.order {
		g.writeEnum(w, name)
	}
	for _, s := range g.structs {
		w.f("%s\n", s)
	}
	src, err := format.Source([]byte(w.b.String()))
	if err != nil {
		return fmt.Errorf("%s: %w", reportTypesPath, err)
	}
	out[reportTypesPath] = src
	return nil
}

// typeOf is the Go type of the schema s at doc#pointer, emitting every type it
// names that is not yet emitted.
func (g *reportGen) typeOf(doc, pointer string, s *value, required bool) (string, error) {
	if ref := s.get("$ref").str(); ref != "" {
		return g.refType(doc, ref, required)
	}
	named, isNamed := reportNames[doc+"#"+pointer]
	kind, name, _ := strings.Cut(named, " ")
	switch {
	case isNamed && kind == "enum":
		return g.enum(name, s.get("enum").strs())
	case isNamed && kind == "hand":
		return name, nil
	case isNamed && name == "":
		if s.get("type").str() == "object" && !strings.Contains(named, ".") && !strings.HasPrefix(named, "map[") {
			return g.object(doc, pointer, s, named, required)
		}
		return named, nil
	}
	return scalarType(doc, pointer, s, required, g)
}

// scalarType is the Go type of a schema no row of reportNames names: a string,
// an integer, a boolean or an array of a named or scalar item.
func scalarType(doc, pointer string, s *value, required bool, g *reportGen) (string, error) {
	switch s.get("type").str() {
	case "string":
		return "string", nil
	case "integer":
		return "int", nil
	case "boolean":
		if required {
			return "bool", nil
		}
		return "*bool", nil
	case "array":
		item, err := g.typeOf(doc, pointer+"/items", s.get("items"), true)
		return "[]" + item, err
	}
	return "", fmt.Errorf("%s#%s: no Go type: name the location in reportNames", doc, pointer)
}

// refType is the Go type a $ref names, from the document doc holds it in.
func (g *reportGen) refType(doc, ref string, required bool) (string, error) {
	file, pointer, _ := strings.Cut(ref, "#")
	target := doc
	if file != "" {
		target = strings.TrimSuffix(file, ".schema.json")
	}
	s := g.docs[target]
	for token := range strings.SplitSeq(strings.TrimPrefix(pointer, "/"), "/") {
		if token != "" {
			s = s.get(token)
		}
	}
	if s == nil {
		return "", fmt.Errorf("%s: $ref %s names no schema", doc, ref)
	}
	return g.typeOf(target, pointer, s, required)
}

// object emits the struct the object schema s declares, named name, and is its
// Go type: the struct, or a pointer to it where the member is optional.
func (g *reportGen) object(doc, pointer string, s *value, name string, required bool) (string, error) {
	typ := name
	if !required {
		typ = "*" + name
	}
	if g.emitted[name] {
		return typ, nil
	}
	g.emitted[name] = true
	props := s.get("properties")
	if props == nil {
		return "", fmt.Errorf("%s#%s: an object with no properties", doc, pointer)
	}
	needed := s.get("required").strs()
	var b strings.Builder
	fmt.Fprintf(&b, "// %s is the object at %s.schema.json#%s.\ntype %s struct {\n", name, doc, pointer, name)
	for _, member := range props.keys {
		isRequired := slices.Contains(needed, member)
		field, err := g.typeOf(doc, pointer+"/properties/"+member, props.get(member), isRequired)
		if err != nil {
			return "", err
		}
		tag := member
		if !isRequired {
			tag += ",omitzero"
		}
		fmt.Fprintf(&b, "\t%s %s `json:%q`\n", goName(member), field, tag)
	}
	b.WriteString("}\n")
	g.structs = append(g.structs, b.String())
	return typ, nil
}

// enum records the closed vocabulary values under the Go type name, which two
// locations may share only with the same values.
func (g *reportGen) enum(name string, values []string) (string, error) {
	if len(values) == 0 {
		return "", fmt.Errorf("the vocabulary %s lists no value", name)
	}
	if held, seen := g.enums[name]; seen {
		if !slices.Equal(held, values) {
			return "", errors.New("two locations name the vocabulary " + name + " with different values")
		}
		return name, nil
	}
	g.enums[name] = values
	g.order = append(g.order, name)
	return name, nil
}

// writeEnum writes one vocabulary's type and a constant per value.
func (g *reportGen) writeEnum(w *writer, name string) {
	w.f("// %s is a closed vocabulary of the report schemas.\ntype %s string\n\n// The values of %s.\nconst (\n", name, name, name)
	for _, v := range g.enums[name] {
		w.f("\t%s%s %s = %q\n", name, goName(strings.ReplaceAll(v, "-", "_")), name, v)
	}
	w.f(")\n\n")
}

// goName is a member name spelled as an exported Go name.
func goName(member string) string {
	var b strings.Builder
	for word := range strings.SplitSeq(member, "_") {
		if spelled, held := initialisms[word]; held {
			b.WriteString(spelled)
			continue
		}
		if word != "" {
			b.WriteString(strings.ToUpper(word[:1]) + word[1:])
		}
	}
	return b.String()
}
