package spec_test

import (
	"io/fs"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/document"
)

// duplicateMemberTrees are the embedded trees whose JSON documents a product decodes: the
// contract, the conformance corpus, the published vectors and the example documents.
func duplicateMemberTrees() map[string]fs.FS {
	return map[string]fs.FS{
		"contract": spec.Contract,
		"corpus":   spec.Corpus,
		"vectors":  spec.Vectors,
		"examples": spec.Examples,
	}
}

// documentsNamingAMemberTwice are the committed documents that write a member twice on purpose,
// because refusing them is the behaviour the document exists to specify. Every other committed
// document names each member once.
func documentsNamingAMemberTwice(t *testing.T) []string {
	t.Helper()
	dir := caseCovering(t, "duplicated-key")
	return []string{configVectorsDir + "/" + dir + "/" + vectorRepositoryFile}
}

func TestCommittedDocumentsNameEachMemberOnce(t *testing.T) {
	deliberate := documentsNamingAMemberTwice(t)
	for _, name := range slices.Sorted(maps.Keys(duplicateMemberTrees())) {
		tree := duplicateMemberTrees()[name]
		t.Run(name, func(t *testing.T) {
			err := fs.WalkDir(tree, ".", func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() || !strings.HasSuffix(p, ".json") {
					return err
				}
				if slices.Contains(deliberate, p) {
					return nil
				}
				data, err := fs.ReadFile(tree, p)
				if err != nil {
					return err
				}
				if _, err := document.Decode(data); err != nil {
					t.Errorf("document.Decode(%s) = %v, want one strict JSON document, because a product refuses any other", p, err)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("Setup: fs.WalkDir(%s): %v", name, err)
			}
		})
	}
}
