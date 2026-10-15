package spec_test

import (
	"bytes"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cplieger/deadset-spec/v8/internal/gen"
)

// TestGeneratedFilesAreFresh regenerates every derived file from the sources on
// disk and compares it byte for byte with the committed file: a source edit
// nobody regenerated fails here. Run go generate to rewrite them.
func TestGeneratedFilesAreFresh(t *testing.T) {
	files, err := gen.Generate(os.DirFS("."))
	if err != nil {
		t.Fatalf("gen.Generate(.) = %v", err)
	}
	for _, name := range gen.Paths(files) {
		t.Run(strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			got, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("Setup: os.ReadFile(%s) = %v, want the committed generated file", name, err)
			}
			if !bytes.Equal(got, files[name]) {
				t.Errorf("%s differs from what deadset-gen writes from its sources: run go generate", name)
			}
		})
	}
}

// TestNoStaleGeneratedPage pins that every page under docs/kinds is one the
// generator writes, so a family that leaves the Contract takes its page with it.
func TestNoStaleGeneratedPage(t *testing.T) {
	files, err := gen.Generate(os.DirFS("."))
	if err != nil {
		t.Fatalf("gen.Generate(.) = %v", err)
	}
	pages, err := fs.Glob(os.DirFS("."), "docs/kinds/*.md")
	if err != nil {
		t.Fatalf("Setup: fs.Glob = %v", err)
	}
	for _, page := range pages {
		if _, ok := files[page]; !ok {
			t.Errorf("%s is no page the generator writes: delete it", page)
		}
	}
}

// TestAPlantedKindsEditMovesTheGeneratedFiles proves the freshness test bites: a
// kind's renamed row changes the pages and the finding schema generated from it.
func TestAPlantedKindsEditMovesTheGeneratedFiles(t *testing.T) {
	disk := os.DirFS(".")
	planted := fstest.MapFS{}
	err := fs.WalkDir(disk, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !generatorInput(path) {
			return err
		}
		data, err := fs.ReadFile(disk, path)
		planted[path] = &fstest.MapFile{Data: data}
		return err
	})
	if err != nil {
		t.Fatalf("Setup: walking the module = %v", err)
	}
	kinds := planted["contract/kinds.json"]
	kinds.Data = bytes.Replace(kinds.Data, []byte(`"name": "unused-member"`), []byte(`"name": "planted-member"`), 1)
	kinds.Data = bytes.Replace(kinds.Data, []byte(`"holds_live": true`), []byte(`"holds_live": false`), 1)
	before, err := gen.Generate(disk)
	if err != nil {
		t.Fatalf("gen.Generate(.) = %v", err)
	}
	after, err := gen.Generate(planted)
	if err != nil {
		t.Fatalf("gen.Generate(planted) = %v", err)
	}
	for _, name := range []string{"docs/kinds/unused.md", "contract/finding.schema.json", "vocab/vocab.gen.go"} {
		if bytes.Equal(before[name], after[name]) {
			t.Errorf("%s is unchanged by a planted contract/kinds.json edit, want it regenerated from the row", name)
		}
	}
}

// generatorInput reports whether the generator reads a file of the module.
func generatorInput(path string) bool {
	return path == "README.md" || path == "corpus/corpus.json" || strings.HasSuffix(path, "/expect.json") ||
		slices.ContainsFunc([]string{"contract/", "authoring/"}, func(p string) bool { return strings.HasPrefix(path, p) })
}
