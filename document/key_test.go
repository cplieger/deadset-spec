package document

import "testing"

func TestKeySpellsTheRefusedPlaceAsADottedKey(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		refused Error
		want    string
	}{
		{refused: Error{}, want: ""},
		{refused: Error{Keyword: additionalKeyword, Member: "extra"}, want: "extra"},
		{refused: Error{Pointer: "/target/kind"}, want: "target.kind"},
		{refused: Error{Pointer: "/target/kind", Keyword: additionalKeyword, Member: "zz"}, want: "target.kind.zz"},
		{refused: Error{Pointer: "/target", Keyword: requiredKeyword, Member: "kind"}, want: "target.kind"},
		{refused: Error{Pointer: "/severity", Keyword: namesKeyword, Member: "DS0"}, want: "severity.DS0"},
		{refused: Error{Pointer: "/providers/analyzers/1/name"}, want: "providers.analyzers[1].name"},
		{refused: Error{Pointer: "/build/0/1"}, want: "build[0][1]"},
		{refused: Error{Pointer: "/a~1b/c~0d"}, want: "a/b.c~d"},
		{refused: Error{Pointer: "/a//b"}, want: "a..b"},
		{refused: Error{Pointer: "/severity/+1"}, want: "severity.+1"},
		{refused: Error{Pointer: "//b"}, want: ".b"},
		{refused: Error{Pointer: "/a/", Keyword: additionalKeyword, Member: "b"}, want: "a..b"},
		{refused: Error{Pointer: "/analysis/template_delimiters", Keyword: additionalKeyword, Member: ""}, want: "analysis.template_delimiters."},
		{refused: Error{Pointer: "/analysis", Keyword: additionalKeyword, Member: "1"}, want: "analysis.1"},
		{refused: Error{Pointer: "/analysis", Keyword: "type"}, want: "analysis"},
	} {
		if got := tc.refused.Key(); got != tc.want {
			t.Errorf("Error{Pointer: %q, Keyword: %q, Member: %q}.Key() = %q, want %q", tc.refused.Pointer, tc.refused.Keyword, tc.refused.Member, got, tc.want)
		}
	}
}
