package spec_test

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

const patternCorpusPath = "contract/grammar/pattern-corpus.json"

// patternCase is one case of the pattern corpus: a roots.patterns entry, the
// reference of a symbol an analysis enumerated, and whether the entry names it.
type patternCase struct {
	Pattern   string `json:"pattern"`
	Reference string `json:"reference"`
	Reason    string `json:"reason"`
	Matches   bool   `json:"matches"`
}

func loadPatternCorpus(t *testing.T) []patternCase {
	t.Helper()
	var cases []patternCase
	grammarCorpus(t, patternCorpusPath, &cases)
	if len(cases) == 0 {
		t.Fatalf("Setup: %q holds no case, want the published corpus", patternCorpusPath)
	}
	return cases
}

// patternExpression is the Patterns paragraph of symbol-ref.md written as an RE2
// expression over the whole reference: a star is any run, a question mark is one
// character, and every other character is itself. RE2 reads its subject as code
// points, which is the unit the paragraph counts a character in.
func patternExpression(pattern string) string {
	var b strings.Builder
	b.WriteString(`^(?s:`)
	for _, r := range pattern {
		switch r {
		case '*':
			b.WriteString(`.*`)
		case '?':
			b.WriteString(`.`)
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString(`)$`)
	return b.String()
}

// patternMatches answers one case under the paragraph's rule.
func patternMatches(t *testing.T, pattern, reference string) bool {
	t.Helper()
	return compileExpression(t, "the pattern "+strconv.Quote(pattern), patternExpression(pattern)).MatchString(reference)
}

func TestPatternCorpusAnswersThePatternsRule(t *testing.T) {
	forms := compileSymbolRefForms(t)
	for i, c := range loadPatternCorpus(t) {
		t.Run(caseName("pattern", strconv.Itoa(i)), func(t *testing.T) {
			if got := patternMatches(t, c.Pattern, c.Reference); got != c.Matches {
				t.Errorf("case %d: the pattern %q against %q matches = %t, want %t: %s", i, c.Pattern, c.Reference, got, c.Matches, c.Reason)
			}
			if c.Reason == "" {
				t.Errorf("case %d (%q against %q) carries no reason, want one", i, c.Pattern, c.Reference)
			}
			if c.Pattern == "" || strings.ContainsAny(c.Pattern, "\r\n") {
				t.Errorf("case %d carries the pattern %q, want a non-empty entry with no CR or LF", i, c.Pattern)
			}
			if !utf8.ValidString(c.Pattern) || !utf8.ValidString(c.Reference) {
				t.Errorf("case %d carries %q against %q, want both valid UTF-8", i, c.Pattern, c.Reference)
			}
			if matchingSymbolForms(forms, c.Reference) == nil {
				t.Errorf("case %d names the candidate %q, want the reference of a symbol: a string some form of %s accepts", i, c.Reference, symbolRefPagePath)
			}
		})
	}
}

// patternShape names which wildcards an entry holds, which is what decides the
// clause of the paragraph that answers it.
func patternShape(pattern string) string {
	switch {
	case strings.Contains(pattern, "*"):
		return "an entry holding a star"
	case strings.Contains(pattern, "?"):
		return "an entry holding a question mark and no star"
	default:
		return "an entry holding neither wildcard"
	}
}

// outsideTheBasicPlane reports whether s holds a code point that UTF-16 spells as
// two code units.
func outsideTheBasicPlane(s string) bool {
	return strings.ContainsFunc(s, func(r rune) bool { return r > 0xFFFF })
}

func TestPatternCorpusCoversEveryClauseBothWays(t *testing.T) {
	cases := loadPatternCorpus(t)
	answered := map[string][]bool{}
	for _, c := range cases {
		shape := patternShape(c.Pattern)
		if !slices.Contains(answered[shape], c.Matches) {
			answered[shape] = append(answered[shape], c.Matches)
		}
	}
	for _, shape := range []string{patternShape("*"), patternShape("?"), patternShape("")} {
		for _, matches := range []bool{true, false} {
			if !slices.Contains(answered[shape], matches) {
				t.Errorf("the corpus holds no case of %s whose answer is matches = %t, want one each way", shape, matches)
			}
		}
	}

	// The unit a question mark counts is the one trap an implementation on UTF-16
	// strings walks into, so the corpus answers it in both directions.
	astral := map[bool]int{}
	for _, c := range cases {
		if strings.Contains(c.Pattern, "?") && outsideTheBasicPlane(c.Reference) {
			astral[c.Matches]++
		}
	}
	for _, matches := range []bool{true, false} {
		if astral[matches] == 0 {
			t.Errorf("the corpus holds no case of a question mark against a reference holding a code point outside the Basic Multilingual Plane whose answer is matches = %t, want one each way", matches)
		}
	}
}

func TestPatternCorpusAnswersEverySampleThePageShows(t *testing.T) {
	samples := onlyFence(t, pageSection(t, grammarPage(t, symbolRefPagePath), "## Patterns"), "the patterns section")
	matched := map[string]bool{}
	for _, c := range loadPatternCorpus(t) {
		if c.Matches {
			matched[c.Pattern] = true
		}
	}
	for _, sample := range samples {
		if !matched[sample] {
			t.Errorf("the patterns section shows %q and the corpus holds no case it matches, want every sample answered", sample)
		}
	}
}
