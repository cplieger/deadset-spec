package suppress

import (
	"encoding/json"
	"testing"

	spec "github.com/cplieger/deadset-spec/v8"
)

// corpusCase is one case of a grammar corpus, its input left raw until its kind
// is known.
type corpusCase struct {
	Input    json.RawMessage `json:"input"`
	Kind     string          `json:"kind"`
	Rule     string          `json:"rule"`
	Language string          `json:"language"`
	Reason   string          `json:"reason"`
	Accepted bool            `json:"accepted"`
}

// readCorpus is every case of one published corpus.
func readCorpus(t *testing.T, path string) []corpusCase {
	t.Helper()
	body, err := spec.Contract.ReadFile(path)
	if err != nil {
		t.Fatalf("Setup: read %s: %v", path, err)
	}
	var cases []corpusCase
	if err := json.Unmarshal(body, &cases); err != nil {
		t.Fatalf("Setup: decode %s: %v", path, err)
	}
	if len(cases) == 0 {
		t.Fatalf("Setup: %s holds no case", path)
	}
	return cases
}

// refusedVerdicts is the step a refused inline case of each rule reaches. A rule
// whose refused comment is well formed all the same is refused by its placement,
// which is the analyzer's.
var refusedVerdicts = map[string]DirectiveVerdict{
	"spelling-no-space": WellFormed,
	"spelling-space":    WellFormed,
	"reason-text":       WellFormed,
	"code-list":         Malformed,
	"namespace":         NotADirective,
	"first-token":       NotADirective,
	"line-comment-only": NotADirective,
	"directive-name":    Malformed,
	"reason-separator":  Malformed,
	"reason-required":   Reasonless,
	"line-above":        WellFormed,
	"trailing":          WellFormed,
}

func TestReadDirectiveDecidesEveryCorpusCase(t *testing.T) {
	t.Parallel()

	for _, c := range readCorpus(t, "contract/grammar/suppression-corpus.json") {
		if c.Kind != "inline" {
			continue
		}
		var in struct {
			Comment string `json:"comment"`
		}
		if err := json.Unmarshal(c.Input, &in); err != nil {
			t.Fatalf("Setup: decode the %s case: %v", c.Rule, err)
		}
		want, known := refusedVerdicts[c.Rule]
		if !known {
			t.Fatalf("the corpus names the rule %s, which no verdict is declared for", c.Rule)
		}
		if c.Accepted {
			want = WellFormed
		}
		got := ReadDirective(in.Comment)
		if got.Verdict != want {
			t.Errorf("ReadDirective(%q) = %v, want %v: %s", in.Comment, got.Verdict, want, c.Reason)
		}
		if c.Accepted && (len(got.Codes) == 0 || got.Reason == "") {
			t.Errorf("ReadDirective(%q) = %+v, want a code and a reason: %s", in.Comment, got, c.Reason)
		}
	}
}

func TestValidReferenceDecidesEveryCorpusCase(t *testing.T) {
	t.Parallel()

	for _, c := range readCorpus(t, "contract/grammar/symbol-ref-corpus.json") {
		var ref string
		if err := json.Unmarshal(c.Input, &ref); err != nil {
			t.Fatalf("Setup: decode a %s case: %v", c.Language, err)
		}
		if got := ValidReference(ref); got != c.Accepted {
			t.Errorf("ValidReference(%q) = %t, want %t: %s", ref, got, c.Accepted, c.Reason)
		}
	}
}
