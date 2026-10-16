package document

import "testing"

func TestKeySpellsTheRefusedPlaceAsADottedKey(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		under, document string
		refused         Error
		want            string
	}{
		{document: `{}`, refused: Error{}, want: ""},
		{document: `{"extra": 1}`, refused: Error{Keyword: additionalKeyword, Member: "extra"}, want: "extra"},
		{document: `{"target": {"kind": 1}}`, refused: Error{Pointer: "/target/kind"}, want: "target.kind"},
		{document: `{"target": {"kind": {"zz": 1}}}`, refused: Error{Pointer: "/target/kind", Keyword: additionalKeyword, Member: "zz"}, want: "target.kind.zz"},
		{document: `{"target": {}}`, refused: Error{Pointer: "/target", Keyword: requiredKeyword, Member: "kind"}, want: "target.kind"},
		{document: `{"severity": {"DS0": "warn"}}`, refused: Error{Pointer: "/severity", Keyword: namesKeyword, Member: "DS0"}, want: "severity.DS0"},
		{document: `{"providers": {"analyzers": [{}, {"name": 1}]}}`, refused: Error{Pointer: "/providers/analyzers/1/name"}, want: "providers.analyzers[1].name"},
		{document: `{"build": [[0, 1]]}`, refused: Error{Pointer: "/build/0/1"}, want: "build[0][1]"},
		{document: `{"a/b": {"c~d": 1}}`, refused: Error{Pointer: "/a~1b/c~0d"}, want: "a/b.c~d"},
		{document: `{"a": {"": {"b": 1}}}`, refused: Error{Pointer: "/a//b"}, want: "a..b"},
		{document: `{"severity": {"+1": 1}}`, refused: Error{Pointer: "/severity/+1"}, want: "severity.+1"},
		{document: `{"": {"b": 1}}`, refused: Error{Pointer: "//b"}, want: ".b"},
		{document: `{"a": {"": {"b": 1}}}`, refused: Error{Pointer: "/a/", Keyword: additionalKeyword, Member: "b"}, want: "a..b"},
		{document: `{"analysis": {"template_delimiters": {"": 1}}}`, refused: Error{Pointer: "/analysis/template_delimiters", Keyword: additionalKeyword, Member: ""}, want: "analysis.template_delimiters."},
		{document: `{"analysis": {"1": 1}}`, refused: Error{Pointer: "/analysis", Keyword: additionalKeyword, Member: "1"}, want: "analysis.1"},
		{document: `{"analysis": 1}`, refused: Error{Pointer: "/analysis", Keyword: "type"}, want: "analysis"},
		{document: `{"edges": {"1": {"provides": "go://x#A"}}}`, refused: Error{Pointer: "/edges/1", Keyword: requiredKeyword, Member: "used_by"}, want: "edges.1.used_by"},
		{document: `{"edges": [{"provides": "go://x#A"}]}`, refused: Error{Pointer: "/edges/0", Keyword: requiredKeyword, Member: "used_by"}, want: "edges[0].used_by"},
		{under: "target.kind.go", document: `1`, refused: Error{Keyword: "type"}, want: "target.kind.go"},
		{under: "exemptions.disabled", document: `["a", "b"]`, refused: Error{Pointer: "/1", Keyword: "enum"}, want: "exemptions.disabled[1]"},
		{under: "analysis.template_delimiters", document: `{"": 1}`, refused: Error{Keyword: additionalKeyword, Member: ""}, want: "analysis.template_delimiters."},
		{under: "severity.DS10", document: `{"1": {"": 1}}`, refused: Error{Pointer: "/1/"}, want: "severity.DS10.1."},
	} {
		value, err := Decode([]byte(tc.document))
		if err != nil {
			t.Fatalf("Setup: Decode(%s) = %v", tc.document, err)
		}
		if got := tc.refused.Key(tc.under, value); got != tc.want {
			t.Errorf("Error{Pointer: %q, Keyword: %q, Member: %q}.Key(%q, %s) = %q, want %q", tc.refused.Pointer, tc.refused.Keyword, tc.refused.Member, tc.under, tc.document, got, tc.want)
		}
	}
}
