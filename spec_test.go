package spec_test

import (
	"embed"
	"encoding/json"
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec"
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
