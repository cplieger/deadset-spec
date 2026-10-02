package spec_test

import (
	"embed"
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v4"
)

// jsonFiles lists every *.json path under fsys, in walk order.
func jsonFiles(fsys embed.FS) ([]string, error) {
	var paths []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && path.Ext(p) == ".json" {
			paths = append(paths, p)
		}
		return nil
	})
	return paths, err
}

// subtestName maps an embedded path to a name -run can select: the runner
// treats "/" as the subtest separator.
func subtestName(p string) string {
	return strings.ReplaceAll(p, "/", "_")
}

func TestEmbeddedJSONDecodes(t *testing.T) {
	trees := []struct {
		fsys embed.FS
		name string
	}{
		{name: "contract", fsys: spec.Contract},
		{name: "corpus", fsys: spec.Corpus},
		{name: "vectors", fsys: spec.Vectors},
	}
	for _, tree := range trees {
		paths, err := jsonFiles(tree.fsys)
		if err != nil {
			t.Fatalf("Setup: walking %s: %v", tree.name, err)
		}
		for _, p := range paths {
			t.Run(subtestName(p), func(t *testing.T) {
				data, err := fs.ReadFile(tree.fsys, p)
				if err != nil {
					t.Fatalf("Setup: fs.ReadFile(%q): %v", p, err)
				}
				var doc any
				if err = json.Unmarshal(data, &doc); err != nil {
					t.Errorf("json.Unmarshal(%q) = %v, want a valid JSON document", p, err)
				}
			})
		}
	}
}

func TestContractDocumentsAreObjects(t *testing.T) {
	paths := []string{
		"contract/contract.json",
		"contract/exit-codes.json",
	}
	for _, p := range paths {
		t.Run(subtestName(p), func(t *testing.T) {
			data, err := fs.ReadFile(spec.Contract, p)
			if err != nil {
				t.Fatalf("fs.ReadFile(Contract, %q) = %v, want the document present", p, err)
			}
			var doc map[string]any
			if err = json.Unmarshal(data, &doc); err != nil {
				t.Fatalf("json.Unmarshal(%q) = %v, want a JSON object", p, err)
			}
			if len(doc) == 0 {
				t.Errorf("json.Unmarshal(%q) = %v, want a non-empty JSON object", p, doc)
			}
		})
	}
}

// modulePathMajor matches a major version of this module's path wherever a file
// names it: an import, a go get line, a link.
var modulePathMajor = regexp.MustCompile(`deadset-spec/v[0-9]+\b`)

// modulePathSkipped are the entries of the working tree that hold no file of the
// repository: git's own metadata and the directories .gitignore names.
var modulePathSkipped = []string{".git", ".agents", "__pycache__"}

// TestModulePathMajorIsTheOneEveryFileNames ties every file that names a major
// version of this module's path, prose and examples as much as Go source, to the
// major go.mod's module line declares, so a major bump that leaves one behind
// fails here rather than in a consumer's go get.
func TestModulePathMajorIsTheOneEveryFileNames(t *testing.T) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("Setup: os.ReadFile(go.mod): %v", err)
	}
	var module string
	for line := range strings.SplitSeq(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			module = strings.TrimSpace(rest)
			break
		}
	}
	if module == "" {
		t.Fatalf("Setup: go.mod declares no module line")
	}
	want := modulePathMajor.FindString(module)
	var named int
	err = fs.WalkDir(os.DirFS("."), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if slices.Contains(modulePathSkipped, d.Name()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		text, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(text), "\n") {
			for _, got := range modulePathMajor.FindAllString(line, -1) {
				named++
				if got != want {
					t.Errorf("File(%s:%d) names %q, want %q, the major the go.mod module line %q declares", p, i+1, got, want, module)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Setup: fs.WalkDir(.): %v", err)
	}
	if named < 2 {
		t.Fatalf("Setup: references to a major of the module path = %d, want go.mod's and the README's at least", named)
	}
}
