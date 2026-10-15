package corpus

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	spec "github.com/cplieger/deadset-spec/v8"
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

var languageNamePattern = regexp.MustCompile(`\b(Go|TypeScript)\b`)

// languageNames are the words the symbol.kind description names a language by.
var languageNames = map[string]string{"Go": "go", "TypeScript": "ts"}

// subjectKindLanguages reads the language of each subject kind only one language
// declares from the symbol.kind description of the finding schema, whose clauses
// name each value once with its subject; a clause naming one language is that
// language's kind.
func subjectKindLanguages() (map[string]string, error) {
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
	out := map[string]string{}
	seen := map[string]bool{}
	for clause := range strings.SplitSeq(strings.TrimSuffix(list, "."), "; ") {
		value, subject, ok := strings.Cut(clause, ", ")
		if !ok || !slices.Contains(vocabulary, value) || seen[value] {
			return nil, fmt.Errorf("the %s symbol.kind clause %q names no value of the vocabulary once", findingSchemaPath, clause)
		}
		seen[value] = true
		if language, single := clauseLanguage(subject); single {
			out[value] = language
		}
	}
	if missing := slices.IndexFunc(vocabulary, func(value string) bool { return !seen[value] }); missing >= 0 {
		return nil, fmt.Errorf("the %s symbol.kind description names no clause for %q", findingSchemaPath, vocabulary[missing])
	}
	return out, nil
}

// clauseLanguage is the one language a clause's subject names, if it names one
// and not "either language".
func clauseLanguage(subject string) (string, bool) {
	named := map[string]bool{}
	for _, word := range languageNamePattern.FindAllString(subject, -1) {
		named[languageNames[word]] = true
	}
	if len(named) != 1 || strings.Contains(subject, "either language") {
		return "", false
	}
	for language := range named {
		return language, true
	}
	return "", false
}
