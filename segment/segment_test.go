package segment

import (
	"encoding/hex"
	"encoding/json"
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"

	spec "github.com/cplieger/deadset-spec/v8"
)

// segmentSets is each set by the name segment-corpus.json gives it.
var segmentSets = map[string]Set{"go-element": GoElement, "source-path": SourcePath, "file-operand": FileOperand}

type segmentCase struct {
	Set       string `json:"set"`
	Spelled   string `json:"spelled"`
	RawHex    string `json:"raw_hex"`
	Reason    string `json:"reason"`
	Canonical bool   `json:"canonical"`
}

func TestSpellAndReadDecideEveryCorpusCase(t *testing.T) {
	t.Parallel()

	body, err := spec.Contract.ReadFile("contract/grammar/segment-corpus.json")
	if err != nil {
		t.Fatalf("Setup: read the segment corpus: %v", err)
	}
	var cases []segmentCase
	if err := json.Unmarshal(body, &cases); err != nil || len(cases) == 0 {
		t.Fatalf("Setup: decode the segment corpus: %d cases, %v", len(cases), err)
	}
	for _, c := range cases {
		set, known := segmentSets[c.Set]
		if !known {
			t.Fatalf("the corpus names the set %q, which no Set is declared for", c.Set)
		}
		got, canonical := Read(set, c.Spelled)
		if !c.Canonical {
			if canonical {
				t.Errorf("Read(%s, %q) = %q, true, want it refused as no canonical spelling: %s", c.Set, c.Spelled, got, c.Reason)
			}
			continue
		}
		raw, err := hex.DecodeString(c.RawHex)
		if err != nil {
			t.Fatalf("Setup: the case %q carries raw_hex %q: %v", c.Spelled, c.RawHex, err)
		}
		if spelled := Spell(set, string(raw)); spelled != c.Spelled {
			t.Errorf("Spell(%s, %x) = %q, want %q: %s", c.Set, raw, spelled, c.Spelled, c.Reason)
		}
		if !canonical || got != string(raw) {
			t.Errorf("Read(%s, %q) = %x, %t, want %x, true: %s", c.Set, c.Spelled, got, canonical, raw, c.Reason)
		}
	}
}

// Every segment, whatever bytes it holds, reads back from its spelling, and the
// spelling is well-formed UTF-8 holding no separator. A segment of . or .. has a
// spelling that reads back as none, because it names no file.
func TestEverySegmentReadsBackFromItsSpelling(t *testing.T) {
	t.Parallel()

	units := []string{
		"a", "Z", "0", "9", ".", "_", "~", "-", "+", " ", "#", "%", "*", "?", "'", "[", "]", "\\", ":",
		"\x00", "\x1f", "\x7f", "\x80", "\xbf", "\xc3", "\xa9", "\xed", "\xa0", "\xff",
		"é", "€", "😀", "\u0085", "\u009f", "\u00a0", "\u00a1", "\u1680", "\u2000", "\u200a", "\u200b",
		"\u2028", "\u2029", "\u202f", "\u205f", "\u3000", "\ufeff",
	}
	for name, set := range segmentSets {
		r := rand.New(rand.NewPCG(uint64(set), 7))
		for range 2000 {
			var b strings.Builder
			for range 1 + r.IntN(8) {
				b.WriteString(units[r.IntN(len(units))])
			}
			raw := b.String()
			spelled := Spell(set, raw)
			if strings.Contains(spelled, "/") || !utf8.ValidString(spelled) {
				t.Fatalf("Spell(%s, %x) = %q, want well-formed UTF-8 with no /", name, raw, spelled)
			}
			got, canonical := Read(set, spelled)
			if raw == "." || raw == ".." {
				if canonical {
					t.Fatalf("Read(%s, %q) = %q, true, want no canonical spelling", name, spelled, got)
				}
				continue
			}
			if !canonical || got != raw {
				t.Fatalf("Read(%s, %q) = %x, %t, want %x, true", name, spelled, got, canonical, raw)
			}
		}
	}
}

func TestSpellPathSpellsEachSegmentAndKeepsTheSeparator(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		path, want string
		set        Set
	}{
		{set: SourcePath, path: "src/my file.ts", want: "src/my%20file.ts"},
		{set: SourcePath, path: "app/[id]/a#b.tsx", want: "app/[id]/a%23b.tsx"},
		{set: GoElement, path: "example.com/app/c++", want: "example.com/app/c%2B%2B"},
		{set: GoElement, path: "example.com/app", want: "example.com/app"},
		{set: FileOperand, path: "my protos/a:b.proto", want: "my%20protos/a%3Ab.proto"},
	} {
		if got := SpellPath(c.set, c.path); got != c.want {
			t.Errorf("SpellPath(%d, %q) = %q, want %q", c.set, c.path, got, c.want)
		}
	}
}

// Unsafe holds exactly the set symbol-ref.md's table writes: each range's ends,
// and the code points beside them, decide it.
func TestUnsafeHoldsTheTablesRangesAndNothingBeside(t *testing.T) {
	t.Parallel()

	inside := []rune{0x00, 0x09, 0x0A, 0x1F, 0x7F, 0x9B, 0x9F, 0x61C, 0x200E, 0x200F, 0x2028, 0x2029, 0x202A, 0x202E, 0x2066, 0x2069}
	outside := []rune{0x20, 0x7E, 0xA0, 0x61B, 0x61D, 0x200D, 0x2010, 0x2027, 0x202F, 0x2065, 0x206A, 0xFEFF, 0xD800, 0x10FFFF}
	for _, r := range inside {
		if !Unsafe(r) {
			t.Errorf("Unsafe(%U) = false, want true", r)
		}
	}
	for _, r := range outside {
		if Unsafe(r) {
			t.Errorf("Unsafe(%U) = true, want false", r)
		}
	}
}
