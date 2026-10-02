package spec_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// schemaWhitespaceClasses are the class escapes RE2 and ECMAScript read as different sets: RE2's
// \s is the ASCII class [\t\n\f\r ] while ECMAScript's is every Unicode space and line
// terminator, so a value of one no-break space passes one dialect and fails the other.
var schemaWhitespaceClasses = []string{`\s`, `\S`}

// bareDot reports whether expr holds a . that is neither escaped nor inside a character class:
// RE2 reads it as every character but LF and ECMAScript as every character but four line
// terminators, so a value holding U+2028 passes one dialect and fails the other.
func bareDot(expr string) bool {
	inClass := false
	for i := 0; i < len(expr); i++ {
		switch c := expr[i]; {
		case c == '\\':
			i++
		case inClass:
			inClass = c != ']'
		case c == '[':
			inClass = true
			if strings.HasPrefix(expr[i+1:], "^") {
				i++
			}
			if strings.HasPrefix(expr[i+1:], "]") {
				i++
			}
		case c == '.':
			return true
		}
	}
	return false
}

// schemaPattern is one regular expression a schema publishes, with where it publishes it.
type schemaPattern struct {
	schema string
	at     string
	expr   string
}

// schemaPatterns returns every pattern keyword value and every patternProperties key of the
// embedded schemas, ordered by schema and then by location.
func schemaPatterns(documents map[string][]byte) ([]schemaPattern, error) {
	var out []schemaPattern
	for _, name := range slices.Sorted(maps.Keys(documents)) {
		var root any
		if err := json.Unmarshal(documents[name], &root); err != nil {
			return nil, fmt.Errorf("decoding %s: %w", name, err)
		}
		out = append(out, patternsUnder(name, "", root)...)
	}
	return out, nil
}

func patternsUnder(schema, at string, node any) []schemaPattern {
	var out []schemaPattern
	switch v := node.(type) {
	case map[string]any:
		for _, key := range slices.Sorted(maps.Keys(v)) {
			child := at + "/" + key
			if expr, ok := v[key].(string); ok && key == "pattern" {
				out = append(out, schemaPattern{schema: schema, at: child, expr: expr})
			}
			if properties, ok := v[key].(map[string]any); ok && key == "patternProperties" {
				for _, expr := range slices.Sorted(maps.Keys(properties)) {
					out = append(out, schemaPattern{schema: schema, at: child, expr: expr})
				}
			}
			out = append(out, patternsUnder(schema, child, v[key])...)
		}
	case []any:
		for i, item := range v {
			out = append(out, patternsUnder(schema, fmt.Sprintf("%s/%d", at, i), item)...)
		}
	}
	return out
}

// patternDialectProblems names every pattern that does not compile under RE2 or that carries a
// class escape or a bare dot the two dialects read differently.
func patternDialectProblems(patterns []schemaPattern) []string {
	var out []string
	for _, p := range patterns {
		if _, err := regexp.Compile(p.expr); err != nil {
			out = append(out, fmt.Sprintf("%s%s: %q does not compile under RE2: %v", p.schema, p.at, p.expr, err))
		}
		for _, class := range schemaWhitespaceClasses {
			if strings.Contains(p.expr, class) {
				out = append(out, fmt.Sprintf("%s%s: %q carries %s, want the explicit class [ \\t\\r\\n] or its complement, which both dialects read alike", p.schema, p.at, p.expr, class))
			}
		}
		if bareDot(p.expr) {
			out = append(out, fmt.Sprintf("%s%s: %q carries a . outside a character class, want the explicit class [^\\r\\n], which both dialects read alike", p.schema, p.at, p.expr))
		}
	}
	return out
}

func TestSchemaPatternsReadAlikeInBothDialects(t *testing.T) {
	documents, err := embeddedSchemas()
	if err != nil {
		t.Fatalf("Setup: embeddedSchemas(): %v", err)
	}
	patterns, err := schemaPatterns(documents)
	if err != nil {
		t.Fatalf("Setup: schemaPatterns(): %v", err)
	}
	if len(patterns) == 0 {
		t.Fatalf("Setup: schemaPatterns() found no pattern in %d schemas, want the patterns they publish", len(documents))
	}
	for _, problem := range patternDialectProblems(patterns) {
		t.Error(problem)
	}
}
