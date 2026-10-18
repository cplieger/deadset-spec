package facts_test

import (
	"bytes"
	"errors"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	spec "github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/document"
	"github.com/cplieger/deadset-spec/v8/facts"
)

func TestDecodeRefusesAtTheBrokenRule(t *testing.T) {
	t.Parallel()

	for name, want := range map[string]document.Error{
		"generated-out-of-order":           {Pointer: "/generated/1", Keyword: "order"},
		"duplicate-generated-path":         {Pointer: "/generated/3", Keyword: "order"},
		"bindings-out-of-order":            {Pointer: "/bindings/1", Keyword: "order"},
		"duplicate-binding-tuple":          {Pointer: "/bindings/2", Keyword: "order"},
		"schema-files-out-of-order":        {Pointer: "/generated/0/schema_files", Keyword: "order"},
		"via-out-of-order":                 {Pointer: "/bindings/0/via", Keyword: "order"},
		"uses-out-of-order":                {Pointer: "/bindings/0/uses", Keyword: "order"},
		"members-out-of-order":             {Pointer: "/bindings/0/members", Keyword: "order"},
		"generated-file-dangling":          {Pointer: "/bindings/0/generated_file", Keyword: "generated_file"},
		"generated-file-without-generator": {Pointer: "/bindings/0/generated_file", Keyword: "generated_file"},
		"via-self":                         {Pointer: "/bindings/2/via/0", Keyword: "via"},
		"via-dangling":                     {Pointer: "/bindings/0/via/0", Keyword: "via"},
		"carried-other-component":          {Pointer: "/bindings/2/carried/0", Keyword: "carried"},
		"generator-row-language-mismatch":  {Pointer: "/generated/1/generator/row", Keyword: "generator"},
		"generator-version-outside-row":    {Pointer: "/generated/0/generator/version", Keyword: "generator"},
		"ts-symbol-in-go":                  {Pointer: "/bindings/1/symbol", Keyword: "scheme"},
		"schema-symbol-mismatch":           {Pointer: "/bindings/0/symbol", Keyword: "schema"},
		"schema-finding-not-ds18":          {Pointer: "/bindings/1/finding/code", Keyword: "finding"},
		"finding-other-symbol":             {Pointer: "/bindings/1/finding/symbol/ref", Keyword: "finding"},
		"schema-finding-other-file":        {Pointer: "/bindings/1/finding/position/path", Keyword: "finding"},
		"implementer-finding-ds18":         {Pointer: "/bindings/2/finding/code", Keyword: "finding"},
		"finding-component-other-analyzer": {Pointer: "/bindings/2/finding/component/id", Keyword: "finding"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := facts.Decode(read(t, filepath.Join("testdata", "refused", name+".json")))
			var got *document.Error
			if !errors.Is(err, facts.ErrInvalid) || !errors.As(err, &got) ||
				got.Pointer != want.Pointer || got.Keyword != want.Keyword {
				t.Errorf("Decode(%s) = %v, want %s refused at %q", name, err, want.Keyword, want.Pointer)
			}
		})
	}
}

func TestDecodeRefusesADuplicateKey(t *testing.T) {
	t.Parallel()

	_, err := facts.Decode(read(t, filepath.Join("testdata", "refused", "duplicate-key.json")))
	if !errors.Is(err, facts.ErrInvalid) || !errors.Is(err, document.ErrMalformed) {
		t.Errorf("Decode(duplicate-key) = %v, want ErrInvalid wrapping document.ErrMalformed", err)
	}
}

func TestBuildRestoresTheCanonicalOrder(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"go-evaluated", "ts-identities", "proto-reader"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want, err := facts.Decode(read(t, filepath.Join("..", "examples", "facts", name+".json")))
			if err != nil {
				t.Fatalf("Decode(%s) = %v, want the example admitted", name, err)
			}
			shuffled, _ := facts.Decode(read(t, filepath.Join("..", "examples", "facts", name+".json")))
			shuffled.FactsVersion = ""
			rand.Shuffle(len(shuffled.Generated), func(i, j int) {
				shuffled.Generated[i], shuffled.Generated[j] = shuffled.Generated[j], shuffled.Generated[i]
			})
			rand.Shuffle(len(shuffled.Bindings), func(i, j int) {
				shuffled.Bindings[i], shuffled.Bindings[j] = shuffled.Bindings[j], shuffled.Bindings[i]
			})
			for i := range shuffled.Bindings {
				m := shuffled.Bindings[i].Members
				rand.Shuffle(len(m), func(i, j int) { m[i], m[j] = m[j], m[i] })
			}
			got, err := facts.Build(shuffled)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Errorf("Build(shuffled %s) = %+v, %v, want the example", name, got, err)
			}
		})
	}
}

func read(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Setup: read %s: %v", path, err)
	}
	return data
}

// TestEncodeWritesEveryMergeVectorFactsDocumentBack holds the facts documents
// the merge vectors publish to the bytes Encode writes, so a writer through this
// package produces the inputs the vectors pin.
func TestEncodeWritesEveryMergeVectorFactsDocumentBack(t *testing.T) {
	names, err := fs.Glob(spec.Vectors, "vectors/merge/*/inputs/*.facts.json")
	if err != nil || len(names) == 0 {
		t.Fatalf("Setup: fs.Glob = %v, %v, want the published facts inputs", names, err)
	}
	for _, name := range names {
		body, err := fs.ReadFile(spec.Vectors, name)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := facts.Decode(body)
		if err != nil {
			t.Fatalf("Decode(%s) = %v", name, err)
		}
		var encoded bytes.Buffer
		if err := facts.Encode(&encoded, doc); err != nil {
			t.Fatalf("Encode(Decode(%s)) = %v", name, err)
		}
		if encoded.String() != string(body) {
			t.Errorf("Encode(Decode(%s)) =\n%s\nwant the file's bytes\n%s", name, encoded.Bytes(), body)
		}
	}
}
