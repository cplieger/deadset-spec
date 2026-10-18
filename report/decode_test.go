package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	spec "github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/document"
)

// TestDecodeReadsEveryExampleReportBack decodes every report the Contract
// publishes as an example or as the round of a baseline vector, and encodes
// it again, which must give the same document: the documents are written in
// another layout, so the two are compared as JSON values.
func TestDecodeReadsEveryExampleReportBack(t *testing.T) {
	t.Parallel()

	documents := map[string]fs.FS{}
	for _, name := range glob(t, spec.Examples, "examples/reports/*.json") {
		documents[name] = spec.Examples
	}
	for _, name := range glob(t, spec.Vectors, "vectors/baseline/*/report.json") {
		documents[name] = spec.Vectors
	}
	for name, tree := range documents {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body := read(t, tree, name)
			decoded, err := Decode(body)
			if err != nil {
				t.Fatalf("Decode(%s) = %v, want a report", name, err)
			}
			var encoded bytes.Buffer
			if err := Encode(&encoded, decoded); err != nil {
				t.Fatalf("Encode(Decode(%s)) = %v", name, err)
			}
			if got, want := value(t, encoded.Bytes()), value(t, body); !reflect.DeepEqual(got, want) {
				t.Errorf("Encode(Decode(%s)) =\n%s\nwant the document it was decoded from", name, encoded.Bytes())
			}
		})
	}
}

// TestDecodeReadsEveryMergeVectorBackByteForByte decodes every report a merge
// vector holds, its inputs and its expected merged report alike, and encodes
// it again, which must give the file's own bytes: the vectors are written in
// the encoding a merged report is compared in, except that an input may hold
// U+2028 and U+2029 unescaped, which the encoding writes as escapes.
func TestDecodeReadsEveryMergeVectorBackByteForByte(t *testing.T) {
	t.Parallel()

	names := append(glob(t, spec.Vectors, "vectors/merge/*/inputs/*.json"),
		glob(t, spec.Vectors, "vectors/merge/*/expected.json")...)
	for _, name := range names {
		if strings.HasSuffix(name, ".facts.json") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body := read(t, spec.Vectors, name)
			decoded, err := Decode(body)
			if err != nil {
				t.Fatalf("Decode(%s) = %v, want a report", name, err)
			}
			var encoded bytes.Buffer
			if err := Encode(&encoded, decoded); err != nil {
				t.Fatalf("Encode(Decode(%s)) = %v", name, err)
			}
			want := strings.NewReplacer("\u2028", `\u2028`, "\u2029", `\u2029`).Replace(string(body))
			if encoded.String() != want {
				t.Errorf("Encode(Decode(%s)) =\n%s\nwant the file's bytes, its line and paragraph separators escaped\n%s",
					name, encoded.Bytes(), want)
			}
		})
	}
}

// TestEncodeWritesEveryCharacterStrictJSONLeavesUnescapedAsItIs pins that a
// character encoding/json escapes by default, in a member a type writes by
// itself and in one it does not, is written as the document wrote it.
func TestEncodeWritesEveryCharacterStrictJSONLeavesUnescapedAsItIs(t *testing.T) {
	t.Parallel()

	original := string(read(t, spec.Vectors, "vectors/merge/one-report/inputs/00-go.json"))
	edited := strings.ReplaceAll(original, "linux-amd64", "linux<amd64>&")
	edited = strings.Replace(edited, `"message": "unexported function`, `"message": "<T> & unexported function`, 1)
	if strings.Count(edited, "<") != 3 {
		t.Fatalf("Setup: the edit placed %d angle brackets, want 3", strings.Count(edited, "<"))
	}

	decoded, err := Decode([]byte(edited))
	if err != nil {
		t.Fatalf("Decode(edited one-report input) = %v", err)
	}
	var encoded bytes.Buffer
	if err := Encode(&encoded, decoded); err != nil {
		t.Fatalf("Encode(Decode(edited one-report input)) = %v", err)
	}
	if encoded.String() != edited {
		t.Errorf("Encode(Decode(edited one-report input)) =\n%s\nwant the edited bytes\n%s", encoded.String(), edited)
	}
}

// TestDecodeReadsEveryExampleFindingBack holds every finding the Contract
// publishes as an example to the finding schema, and encodes it again.
func TestDecodeReadsEveryExampleFindingBack(t *testing.T) {
	t.Parallel()

	for _, name := range glob(t, spec.Examples, "examples/findings/*.json") {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			body := read(t, spec.Examples, name)
			decoded, err := decodeFinding(body)
			if err != nil {
				t.Fatalf("decodeFinding(%s) = %v, want a finding", name, err)
			}
			encoded, err := marshal(decoded)
			if err != nil {
				t.Fatalf("marshal(decodeFinding(%s)) = %v", name, err)
			}
			if got, want := value(t, encoded), value(t, body); !reflect.DeepEqual(got, want) {
				t.Errorf("marshal(decodeFinding(%s)) =\n%s\nwant the document it was decoded from", name, encoded)
			}
		})
	}
}

// TestDecodeRefusesEveryNegative decodes every refused document the Contract
// publishes against the report or the finding schema. Each must be refused at
// the value the index names or a member beneath it, for the reason the
// constraint the index names states.
func TestDecodeRefusesEveryNegative(t *testing.T) {
	t.Parallel()

	var index struct {
		Negatives []struct {
			File         string `json:"file"`
			Schema       string `json:"schema"`
			Constraint   string `json:"constraint"`
			InstancePath string `json:"instance_path"`
		} `json:"negatives"`
	}
	if err := json.Unmarshal(read(t, spec.Examples, "examples/negatives/index.json"), &index); err != nil {
		t.Fatalf("Setup: decode examples/negatives/index.json: %v", err)
	}
	decoders := map[string]func([]byte) error{
		"contract/report.schema.json": func(body []byte) error {
			_, err := Decode(body)
			return err
		},
		"contract/finding.schema.json": func(body []byte) error {
			_, err := decodeFinding(body)
			return err
		},
	}
	tested := 0
	for _, row := range index.Negatives {
		decode, ours := decoders[row.Schema]
		if !ours {
			continue
		}
		tested++
		t.Run(row.File, func(t *testing.T) {
			t.Parallel()

			err := decode(read(t, spec.Examples, "examples/negatives/"+row.File))
			var refused *document.Error
			if !errors.As(err, &refused) {
				t.Fatalf("decoding %s = %v, want an *document.Error refusing %s at %q", row.File, err, row.Constraint, row.InstancePath)
			}
			if refused.Pointer != row.InstancePath && !strings.HasPrefix(refused.Pointer, row.InstancePath+"/") {
				t.Errorf("decoding %s refused %q, want %q or a member beneath it: %v",
					row.File, refused.Pointer, row.InstancePath, err)
			}
			if refused.Keyword != row.Constraint {
				t.Errorf("decoding %s = %v, want the refusal %s states", row.File, err, row.Constraint)
			}
		})
	}
	if tested == 0 {
		t.Fatal("Setup: the negatives index names no document against the report or the finding schema")
	}
}

// TestDecodeRefusesWhatEncodingJSONAdmits pins the refusals the schemas and the
// strict reader make that encoding/json does not: a member spelled in another case, a
// member named twice, a null, an absent required member, and an optional
// member present with no value. Each case edits one valid report.
func TestDecodeRefusesWhatEncodingJSONAdmits(t *testing.T) {
	t.Parallel()

	valid := read(t, spec.Vectors, "vectors/merge/one-report/inputs/00-go.json")
	for name, tt := range map[string]struct {
		old, new    string
		wantPointer string
		wantKeyword string
	}{
		"a member spelled in another case": {
			old: `"findings": [`, new: `"Findings": [`,
			wantPointer: "", wantKeyword: "additionalProperties",
		},
		"a member named twice": {
			old: `"findings": [`, new: `"findings": [], "findings": [`,
			wantPointer: "", wantKeyword: "",
		},
		"a required array that is null": {
			old: `"excluded_by_cgo": []`, new: `"excluded_by_cgo": null`,
			wantPointer: "/excluded_by_cgo", wantKeyword: "type",
		},
		"an array element that is null": {
			old: `"languages": [`, new: `"languages": [null, `,
			wantPointer: "/analyzer/languages/0", wantKeyword: "type",
		},
		"an absent required member": {
			old: `"excluded_by_cgo": [],`, new: ``,
			wantPointer: "", wantKeyword: "required",
		},
		"an optional member present with no value": {
			old: `"parent": "go://example.com/app/internal/store#"`, new: `"parent": ""`,
			wantPointer: "/findings/0/symbol/parent", wantKeyword: "minLength",
		},
		"a value of the wrong type": {
			old: `"test_only": false`, new: `"test_only": "false"`,
			wantPointer: "/findings/0/test_only", wantKeyword: "type",
		},
		"a configuration of neither shape": {
			old: `"os": "linux",`, new: ``,
			wantPointer: "/configurations/0", wantKeyword: "oneOf",
		},
		"an error member on a configuration the analysis ran": {
			old: `"tags": []`, new: `"tags": [], "error": "go: no Go files"`,
			wantPointer: "/configurations/0", wantKeyword: "oneOf",
		},
		"a configuration repeated": {
			old: `"configurations": [`, new: `"configurations": [{"id": "linux-amd64", "os": "linux", "arch": "amd64", "tags": []}, `,
			wantPointer: "/configurations", wantKeyword: "uniqueItems",
		},
		"a mixed kind naming one language": {
			old: `"kind": "application",`, new: `"kind": "mixed", "language_kinds": {"go": "application"},`,
			wantPointer: "/target/language_kinds", wantKeyword: "minProperties",
		},
		"a language kind outside the two kinds": {
			old: `"kind": "application",`, new: `"kind": "mixed", "language_kinds": {"go": "application", "ts": "mixed"},`,
			wantPointer: "/target/language_kinds/ts", wantKeyword: "enum",
		},
		"a language kind present with no value": {
			old: `"kind": "application",`, new: `"kind": "mixed", "language_kinds": {"go": "application", "ts": ""},`,
			wantPointer: "/target/language_kinds/ts", wantKeyword: "enum",
		},
		"a whole document that is not an object": {
			old: string(valid), new: `null`,
			wantPointer: "", wantKeyword: "type",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			edited := strings.Replace(string(valid), tt.old, tt.new, 1)
			if edited == string(valid) {
				t.Fatalf("Setup: %q is not in the report", tt.old)
			}
			_, err := Decode([]byte(edited))
			if tt.wantKeyword == "" {
				if !errors.Is(err, document.ErrMalformed) {
					t.Errorf("Decode(%s) = %v, want %v", name, err, document.ErrMalformed)
				}
				return
			}
			var refused *document.Error
			if !errors.As(err, &refused) {
				t.Fatalf("Decode(%s) = %v, want an *document.Error at %q", name, err, tt.wantPointer)
			}
			if refused.Pointer != tt.wantPointer || refused.Keyword != tt.wantKeyword {
				t.Errorf("Decode(%s) = %v, want %s at %q", name, err, tt.wantKeyword, tt.wantPointer)
			}
		})
	}
}

// TestEncodeRefusesAReportDecodeWouldRefuse pins that Encode holds a report to
// the rules Decode applies and writes nothing when it refuses one.
func TestEncodeRefusesAReportDecodeWouldRefuse(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		edit        func(*Report)
		wantPointer string
		wantKeyword string
	}{
		"a required array left nil": {
			edit:        func(r *Report) { r.StaleSuppressions = nil },
			wantPointer: "/stale_suppressions", wantKeyword: "type",
		},
		"a configuration of both shapes": {
			edit:        func(r *Report) { r.Configurations[0].Project = "tsconfig.json" },
			wantPointer: "/configurations/0", wantKeyword: "oneOf",
		},
		"a dead evaluation with no pending finding": {
			edit: func(r *Report) {
				r.EdgeEvaluations = []EdgeEvaluation{{Edge: "wire/ServerEvent", Side: SideProvides, Symbol: "go://example.com/app#ServerEvent", State: StateDead}}
			},
			wantPointer: "/edge_evaluations/0", wantKeyword: "required",
		},
		"a finding that names a relation its code forbids": {
			edit:        func(r *Report) { r.Findings[0].Code = "DS1301" },
			wantPointer: "/findings/0/details", wantKeyword: "required",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			decoded, err := Decode(read(t, spec.Vectors, "vectors/merge/one-report/inputs/00-go.json"))
			if err != nil {
				t.Fatalf("Setup: Decode(one-report input) = %v", err)
			}
			tt.edit(decoded)
			var written bytes.Buffer
			err = Encode(&written, decoded)
			var refused *document.Error
			if !errors.As(err, &refused) {
				t.Fatalf("Encode(%s) = %v, want an *document.Error at %q", name, err, tt.wantPointer)
			}
			if refused.Pointer != tt.wantPointer || refused.Keyword != tt.wantKeyword {
				t.Errorf("Encode(%s) = %v, want %s at %q", name, err, tt.wantKeyword, tt.wantPointer)
			}
			if written.Len() != 0 {
				t.Errorf("Encode(%s) wrote %d bytes, want none on a refusal", name, written.Len())
			}
		})
	}
}

// TestEncodeRefusesEveryNegativeCount sets each count of a valid report's
// totals, nested ones included, to -1 in turn: the schema gives every one a
// minimum of 0, so each is refused at its own pointer.
func TestEncodeRefusesEveryNegativeCount(t *testing.T) {
	t.Parallel()

	var counts []string
	var walk func(prefix string, v reflect.Value)
	walk = func(prefix string, v reflect.Value) {
		for i := range v.NumField() {
			name, _, _ := strings.Cut(v.Type().Field(i).Tag.Get("json"), ",")
			switch field := v.Field(i); field.Kind() {
			case reflect.Int:
				counts = append(counts, prefix+"/"+name)
			case reflect.Struct:
				walk(prefix+"/"+name, field)
			default:
				t.Fatalf("Setup: %s/%s is a %s, want a count or an object of counts", prefix, name, field.Kind())
			}
		}
	}
	walk("/totals", reflect.ValueOf(Totals{}))
	for _, pointer := range counts {
		t.Run(strings.TrimPrefix(pointer, "/"), func(t *testing.T) {
			t.Parallel()

			decoded, err := Decode(read(t, spec.Vectors, "vectors/merge/one-report/inputs/00-go.json"))
			if err != nil {
				t.Fatalf("Setup: Decode(one-report input) = %v", err)
			}
			held := reflect.ValueOf(&decoded.Totals).Elem()
			for segment := range strings.SplitSeq(strings.TrimPrefix(pointer, "/totals/"), "/") {
				held = held.FieldByIndex(fieldNamed(t, held.Type(), segment))
			}
			held.SetInt(-1)
			err = Encode(io.Discard, decoded)
			var refused *document.Error
			if !errors.As(err, &refused) || refused.Pointer != pointer || refused.Keyword != "minimum" {
				t.Errorf("Encode(%s = -1) = %v, want minimum at %q", pointer, err, pointer)
			}
		})
	}
}

// TestEncodeRefusesEveryCountAboveItsSchemaMaximum sets each count of a valid
// report's totals that the report schema gives a maximum to one above it, so
// each is refused at its own pointer.
func TestEncodeRefusesEveryCountAboveItsSchemaMaximum(t *testing.T) {
	t.Parallel()

	bounded := map[string]int64{}
	var walk func(pointer string, node json.RawMessage)
	walk = func(pointer string, node json.RawMessage) {
		var keywords struct {
			Maximum    *int64                     `json:"maximum"`
			Properties map[string]json.RawMessage `json:"properties"`
		}
		decodeSchema(t, node, &keywords)
		if keywords.Maximum != nil {
			bounded[pointer] = *keywords.Maximum
		}
		for name, child := range keywords.Properties {
			walk(pointer+"/"+name, child)
		}
	}
	var root struct {
		Properties struct {
			Totals json.RawMessage `json:"totals"`
		} `json:"properties"`
	}
	decodeSchema(t, read(t, spec.Contract, "contract/report.schema.json"), &root)
	walk("/totals", root.Properties.Totals)
	if len(bounded) == 0 {
		t.Fatal("Setup: the report schema gives no count of totals a maximum")
	}
	for pointer, maximum := range bounded {
		t.Run(strings.TrimPrefix(pointer, "/"), func(t *testing.T) {
			t.Parallel()

			decoded, err := Decode(read(t, spec.Vectors, "vectors/merge/one-report/inputs/00-go.json"))
			if err != nil {
				t.Fatalf("Setup: Decode(one-report input) = %v", err)
			}
			held := reflect.ValueOf(&decoded.Totals).Elem()
			for segment := range strings.SplitSeq(strings.TrimPrefix(pointer, "/totals/"), "/") {
				held = held.FieldByIndex(fieldNamed(t, held.Type(), segment))
			}
			held.SetInt(maximum + 1)
			err = Encode(io.Discard, decoded)
			var refused *document.Error
			if !errors.As(err, &refused) || refused.Pointer != pointer || refused.Keyword != "maximum" {
				t.Errorf("Encode(%s = %d) = %v, want maximum at %q", pointer, maximum+1, err, pointer)
			}
		})
	}
}

// fieldNamed is the index of the field of held that JSON names name.
func fieldNamed(t *testing.T, held reflect.Type, name string) []int {
	t.Helper()

	for i := range held.NumField() {
		if tag, _, _ := strings.Cut(held.Field(i).Tag.Get("json"), ","); tag == name {
			return []int{i}
		}
	}
	t.Fatalf("Setup: %s has no field named %q", held, name)
	return nil
}

// decodeFinding holds one finding document to the finding schema, the way a
// report's decode holds each finding it carries.
func decodeFinding(data []byte) (*Finding, error) {
	compiled, err := document.Compile(spec.Contract, "contract/finding.schema.json")
	if err != nil {
		return nil, err
	}
	value, err := document.Decode(data)
	if err != nil {
		return nil, err
	}
	if found := compiled.CheckValue(value); len(found) > 0 {
		return nil, &found[0]
	}
	var read Finding
	if err := json.Unmarshal(data, &read); err != nil {
		return nil, err
	}
	return &read, nil
}

// glob is every file of tree that pattern matches, which must be one at least.
func glob(t *testing.T, tree fs.FS, pattern string) []string {
	t.Helper()

	names, err := fs.Glob(tree, pattern)
	if err != nil || len(names) == 0 {
		t.Fatalf("Setup: glob %s = %q, %v; want one file at least", pattern, names, err)
	}
	return names
}

// read is the bytes of one file of tree.
func read(t *testing.T, tree fs.FS, name string) []byte {
	t.Helper()

	body, err := fs.ReadFile(tree, name)
	if err != nil {
		t.Fatalf("Setup: read %s: %v", name, err)
	}
	return body
}

// value is the JSON value body holds, for comparing two documents whatever
// their layout.
func value(t *testing.T, body []byte) any {
	t.Helper()

	var held any
	if err := json.Unmarshal(body, &held); err != nil {
		t.Fatalf("Setup: decode %s: %v", body, err)
	}
	return held
}

// decodeSchema reads one schema node into into.
func decodeSchema(t *testing.T, node json.RawMessage, into any) {
	t.Helper()

	if err := json.Unmarshal(node, into); err != nil {
		t.Fatalf("Setup: decode a schema node: %v", err)
	}
}
