package corpus

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	spec "github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// readSchema decodes one schema of a tree into the generic form its paths walk.
func readSchema(fsys fs.FS, name string) (map[string]any, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return schema, nil
}

// objectAt is the object at a slash-separated path of object keys and array
// indexes into a decoded schema, or nil.
func objectAt(schema map[string]any, p string) map[string]any {
	var node any = schema
	for key := range strings.SplitSeq(p, "/") {
		switch n := node.(type) {
		case map[string]any:
			node = n[key]
		case []any:
			var i int
			if _, err := fmt.Sscan(key, &i); err != nil || i < 0 || i >= len(n) {
				return nil
			}
			node = n[i]
		default:
			return nil
		}
	}
	held, _ := node.(map[string]any)
	return held
}

// enumAt is the string enum of the schema at a path, or nil.
func enumAt(schema map[string]any, p string) []string {
	values, _ := objectAt(schema, p)["enum"].([]any)
	var out []string
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

const findingSchemaPath = "contract/finding.schema.json"

// subjectKindLanguages reads, from the symbol.kind description of the finding
// schema, the languages that declare each subject kind whose clause names a
// language: a clause names a language by its contract/languages.json name, and
// "either language" names Go and TypeScript. A kind whose clause names no
// language is absent, because every language may name it.
func subjectKindLanguages() (map[string][]string, error) {
	schema, err := readSchema(spec.Contract, findingSchemaPath)
	if err != nil {
		return nil, err
	}
	description, _ := objectAt(schema, "properties/symbol/properties/kind")["description"].(string)
	const lead = "The values, each with the subject it names: "
	_, list, found := strings.Cut(description, lead)
	if !found {
		return nil, fmt.Errorf("the %s symbol.kind description carries no %q", findingSchemaPath, lead)
	}
	vocabulary := enumAt(schema, "properties/symbol/properties/kind")
	out := map[string][]string{}
	seen := map[string]bool{}
	for clause := range strings.SplitSeq(strings.TrimSuffix(list, "."), "; ") {
		value, subject, ok := strings.Cut(clause, ", ")
		if !ok || !slices.Contains(vocabulary, value) || seen[value] {
			return nil, fmt.Errorf("the %s symbol.kind clause %q names no value of the vocabulary once", findingSchemaPath, clause)
		}
		seen[value] = true
		if languages := clauseLanguages(subject); len(languages) != 0 {
			out[value] = languages
		}
	}
	if missing := slices.IndexFunc(vocabulary, func(value string) bool { return !seen[value] }); missing >= 0 {
		return nil, fmt.Errorf("the %s symbol.kind description names no clause for %q", findingSchemaPath, vocabulary[missing])
	}
	return out, nil
}

// clauseLanguages are the languages a clause's subject names, in the registry's
// order.
func clauseLanguages(subject string) []string {
	var languages []string
	rows := vocab.LanguageRows()
	for i := range rows {
		row := &rows[i]
		either := strings.Contains(subject, "either language") && (row.Language == "go" || row.Language == "ts")
		if either || regexp.MustCompile(`\b`+regexp.QuoteMeta(row.Name)+`\b`).MatchString(subject) {
			languages = append(languages, string(row.Language))
		}
	}
	return languages
}
