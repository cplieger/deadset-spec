package document

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cplieger/deadset-spec/v8"
)

// compiled compiles one of the Contract's schemas from the published tree.
func compiled(t *testing.T, name string) *Schema {
	t.Helper()
	s, err := Compile(spec.Contract, name)
	if err != nil {
		t.Fatalf("Setup: Compile(%s) = %v", name, err)
	}
	return s
}

func TestEveryContractSchemaCompiles(t *testing.T) {
	t.Parallel()

	names, err := fs.Glob(spec.Contract, "contract/*.schema.json")
	if err != nil || len(names) == 0 {
		t.Fatalf("Setup: list the Contract's schemas: %v, %d found", err, len(names))
	}
	for _, name := range names {
		if _, err := Compile(spec.Contract, name); err != nil {
			t.Errorf("Compile(%s) = %v, want the compiled schema", name, err)
		}
	}
}

func TestCheckAcceptsEveryExample(t *testing.T) {
	t.Parallel()

	for dir, name := range map[string]string{
		"examples/reports":  "contract/report.schema.json",
		"examples/findings": "contract/finding.schema.json",
		"examples/scope":    "contract/scope.schema.json",
		"examples/describe": "contract/describe.schema.json",
	} {
		s := compiled(t, name)
		files, err := fs.Glob(spec.Examples, dir+"/*.json")
		if err != nil || len(files) == 0 {
			t.Fatalf("Setup: list %s: %v, %d found", dir, err, len(files))
		}
		for _, file := range files {
			body, err := spec.Examples.ReadFile(file)
			if err != nil {
				t.Fatalf("Setup: read %s: %v", file, err)
			}
			found, err := checkAll(s, body)
			if err != nil || len(found) != 0 {
				t.Errorf("Check(%s against %s) = %v, %v, want no violation", file, name, found, err)
			}
		}
	}
}

// negative is one row of the example set's index of refused documents.
type negative struct {
	File       string `json:"file"`
	Schema     string `json:"schema"`
	Constraint string `json:"constraint"`
	Pointer    string `json:"instance_path"`
}

func TestCheckRefusesEveryNegativeAtItsConstraint(t *testing.T) {
	t.Parallel()

	body, err := spec.Examples.ReadFile("examples/negatives/index.json")
	if err != nil {
		t.Fatalf("Setup: read the negatives index: %v", err)
	}
	var index struct {
		Negatives []negative `json:"negatives"`
	}
	if err := json.Unmarshal(body, &index); err != nil || len(index.Negatives) == 0 {
		t.Fatalf("Setup: decode the negatives index: %v, %d rows", err, len(index.Negatives))
	}
	for _, row := range index.Negatives {
		document, err := spec.Examples.ReadFile(path.Join("examples/negatives", row.File))
		if err != nil {
			t.Fatalf("Setup: read %s: %v", row.File, err)
		}
		s := compiled(t, row.Schema)
		found, err := checkAll(s, document)
		if err != nil {
			t.Errorf("Check(%s) = %v, want the document read", row.File, err)
			continue
		}
		if !refusedAt(found, row) {
			t.Errorf("Check(%s) = %v, want violations at %q only, one of them %s",
				row.File, found, row.Pointer, row.Constraint)
		}
	}
}

// refusedAt reports whether every violation is at the row's instance path and one
// is the row's constraint.
func refusedAt(found []Error, row negative) bool {
	if len(found) == 0 {
		return false
	}
	for _, v := range found {
		if v.Pointer != row.Pointer {
			return false
		}
	}
	return slices.ContainsFunc(found, func(v Error) bool { return v.Keyword == row.Constraint })
}

func TestCompileRefusesAKeywordItDoesNotEvaluate(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{"one.schema.json": {Data: []byte(`{"type": "string", "format": "uri", "x-owner": "a", "description": "d"}`)}}
	if _, err := Compile(fsys, "one.schema.json"); !errors.Is(err, errSchema) || !strings.Contains(err.Error(), `"format"`) {
		t.Errorf("Compile(a schema naming format) = %v, want errSchema naming the keyword", err)
	}
}

func TestMinPropertiesRefusesAnObjectWithTooFewMembers(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{"one.schema.json": {Data: []byte(`{
		"type": "object",
		"properties": {
			"streamed": {"type": "object", "minProperties": 2},
			"decoded": {"type": "object", "minProperties": 2, "not": {"required": ["x"]}}
		}
	}`)}}
	s, err := Compile(fsys, "one.schema.json")
	if err != nil {
		t.Fatalf("Setup: Compile = %v", err)
	}
	for _, tc := range []struct {
		name     string
		document string
		want     []Error
	}{
		{name: "enough", document: `{"streamed": {"a": 1, "b": 2}, "decoded": {"a": 1, "b": 2}}`},
		{name: "streamed_short", document: `{"streamed": {"a": 1}}`, want: []Error{{Keyword: "minProperties", Pointer: "/streamed"}}},
		{name: "decoded_short", document: `{"decoded": {}}`, want: []Error{{Keyword: "minProperties", Pointer: "/decoded"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			found, err := checkAll(s, []byte(tc.document))
			if err != nil {
				t.Fatalf("Check(%s) = %v", tc.document, err)
			}
			got := make([]Error, len(found))
			for i, v := range found {
				got[i] = Error{Keyword: v.Keyword, Pointer: v.Pointer}
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("Check(%s) = %v, want %v", tc.document, found, tc.want)
			}
		})
	}
}

// Decode refuses what a strict reader refuses: data after the value, and a
// member named twice.
func TestDecodeRefusesWhatJSONToleratesAndMore(t *testing.T) {
	t.Parallel()

	for _, data := range []string{`{} {}`, `{"a": 1, "a": 2}`, `{"a": [{"b": 1, "b": 1}]}`, `{`} {
		if _, err := Decode([]byte(data)); !errors.Is(err, ErrMalformed) {
			t.Errorf("Decode(%s) = %v, want ErrMalformed", data, err)
		}
	}
}

// checkAll is what s refuses in one strict document.
func checkAll(s *Schema, data []byte) ([]Error, error) {
	value, err := Decode(data)
	if err != nil {
		return nil, err
	}
	return s.CheckValue(value), nil
}
