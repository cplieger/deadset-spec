package spec_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v2"
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

// escapeJSONPointer spells one member name as a JSON Pointer token (RFC 6901), so a name
// carrying a slash or a tilde names one member rather than a path.
func escapeJSONPointer(name string) string {
	return strings.ReplaceAll(strings.ReplaceAll(name, "~", "~0"), "/", "~1")
}

// duplicateMembers walks a JSON document token by token and returns the JSON Pointer of every
// object member the document writes twice at the same level, at any depth. A decoder that
// unmarshals into a map or a struct cannot report this: the second value silently replaces the
// first, so the document reads as though it named the member once.
func duplicateMembers(data []byte) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	var found []string
	if err := walkDuplicateMembers(dec, "", &found); err != nil {
		return nil, err
	}
	if dec.More() {
		return nil, fmt.Errorf("want one document, a second value follows it")
	}
	return found, nil
}

// walkDuplicateMembers reads the one value at the decoder's position, appending the pointer of
// every repeated member it and its descendants carry. at is the pointer of that value.
func walkDuplicateMembers(dec *json.Decoder, at string, found *[]string) error {
	tok, err := dec.Token()
	if err != nil {
		return err
	}
	delim, isDelim := tok.(json.Delim)
	if !isDelim {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			name, err := dec.Token()
			if err != nil {
				return err
			}
			member, isName := name.(string)
			if !isName {
				return fmt.Errorf("%q: want a member name, got %v", at, name)
			}
			pointer := at + "/" + escapeJSONPointer(member)
			if seen[member] {
				*found = append(*found, pointer)
			}
			seen[member] = true
			if err = walkDuplicateMembers(dec, pointer, found); err != nil {
				return err
			}
		}
	case '[':
		for i := 0; dec.More(); i++ {
			if err = walkDuplicateMembers(dec, fmt.Sprintf("%s/%d", at, i), found); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("%q: want the start of an object or an array, got %v", at, delim)
	}
	_, err = dec.Token()
	return err
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
				repeated, err := duplicateMembers(data)
				if err != nil {
					t.Errorf("duplicateMembers(%s): %v, want one JSON document", p, err)
					return nil
				}
				for _, pointer := range repeated {
					t.Errorf("%s names the member at %s twice, want each member named once, because a decoder that refuses a repeated member cannot read this document", p, pointer)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("Setup: fs.WalkDir(%s): %v", name, err)
			}
		})
	}
}

// TestDuplicateMembersReportsAPlantedRepeat plants a repeated member at each position the walk
// reaches, so the sweep above is known to see one rather than only known to stay silent.
func TestDuplicateMembersReportsAPlantedRepeat(t *testing.T) {
	cases := []struct {
		name     string
		document string
		want     []string
	}{
		{
			name:     "no_repeat",
			document: `{"a": 1, "b": {"a": 2}, "c": [{"a": 3}]}`,
			want:     nil,
		},
		{
			name:     "at_the_top_level",
			document: `{"a": 1, "a": 2}`,
			want:     []string{"/a"},
		},
		{
			name:     "inside_a_nested_object",
			document: `{"a": {"b": 1, "b": 2}}`,
			want:     []string{"/a/b"},
		},
		{
			name:     "inside_an_array_element",
			document: `{"a": [0, {"b": 1, "b": 2}]}`,
			want:     []string{"/a/1/b"},
		},
		{
			name:     "in_a_member_name_holding_a_slash",
			document: `{"a/b": 1, "a/b": 2}`,
			want:     []string{"/a~1b"},
		},
		{
			name:     "twice_over",
			document: `{"a": 1, "a": 2, "a": 3}`,
			want:     []string{"/a", "/a"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := duplicateMembers([]byte(tc.document))
			if err != nil {
				t.Fatalf("duplicateMembers(%s) errored: %v, want the repeated members", tc.document, err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("duplicateMembers(%s) = %q, want %q", tc.document, got, tc.want)
			}
		})
	}
}

// TestDuplicateMembersRefusesADocumentItCannotWalk pins the failure the sweep reports rather
// than passing over: a file that is not one JSON document is a defect in the file.
func TestDuplicateMembersRefusesADocumentItCannotWalk(t *testing.T) {
	cases := map[string]string{
		"a_truncated_object":  `{"a": 1`,
		"two_documents":       `{"a": 1} {"b": 2}`,
		"a_trailing_fragment": `{"a": 1} nonsense`,
	}
	for _, name := range slices.Sorted(maps.Keys(cases)) {
		t.Run(name, func(t *testing.T) {
			if _, err := duplicateMembers([]byte(cases[name])); err == nil {
				t.Errorf("duplicateMembers(%s) = nil error, want a refusal", cases[name])
			}
		})
	}
}
