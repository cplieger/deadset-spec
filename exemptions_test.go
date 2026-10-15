package spec_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v8"
)

const exemptionsPath = "contract/exemptions.json"

// exemptionsDocument mirrors contract/exemptions.json closely enough that an
// unknown key fails the decode; key presence is checked separately on the raw
// rows, because a missing boolean decodes as false.
type exemptionsDocument struct {
	Description string           `json:"description"`
	Exemptions  []exemptionClass `json:"exemptions"`
	Sinks       []exemptionSink  `json:"sinks"`
}

// exemptionSink is one destination of the sink vocabulary.
type exemptionSink struct {
	Sink        string   `json:"sink"`
	Reads       string   `json:"reads"`
	Description string   `json:"description"`
	Languages   []string `json:"languages"`
}

type exemptionClass struct {
	Mechanism    map[string]string   `json:"mechanism"`
	RetainsKinds map[string][]string `json:"retains_kinds"`
	Visibility   *memberVisibility   `json:"typescript_visibility"`
	Class        string              `json:"class"`
	Confidence   string              `json:"confidence"`
	Rule         string              `json:"rule"`
	Retains      string              `json:"retains"`
	Sinks        []string            `json:"sinks"`
	Languages    []string            `json:"languages"`
}

type memberVisibility struct {
	Private     bool `json:"private"`
	PrivateName bool `json:"private_name"`
}

var (
	// classNamePattern is the shape of a class token: lowercase words joined
	// by single hyphens, so a `retained_by` value never needs quoting or
	// escaping in any reporter.
	classNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	knownLanguages   = []string{"go", "ts"}
	knownConfidences = []string{"certain", "probable", "possible"}
	rowKeys          = []string{"class", "languages", "confidence", "rule", "retains", "retains_kinds", "mechanism"}
)

// loadExemptions decodes contract/exemptions.json, failing the test on any
// setup error so every caller reads a document that exists and parses.
func loadExemptions(t *testing.T) exemptionsDocument {
	t.Helper()
	data, err := fs.ReadFile(spec.Contract, exemptionsPath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", exemptionsPath, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var doc exemptionsDocument
	if err = dec.Decode(&doc); err != nil {
		t.Fatalf("Setup: decoding %s: %v", exemptionsPath, err)
	}
	return doc
}

func TestExemptionsDocumentShape(t *testing.T) {
	doc := loadExemptions(t)
	if doc.Description == "" {
		t.Errorf("exemptions.json description = %q, want a non-empty description", doc.Description)
	}
	if len(doc.Exemptions) == 0 {
		t.Fatalf("exemptions.json exemptions = %d rows, want at least one", len(doc.Exemptions))
	}
	seen := make(map[string]bool, len(doc.Exemptions))
	for _, c := range doc.Exemptions {
		t.Run(c.Class, func(t *testing.T) {
			if !classNamePattern.MatchString(c.Class) {
				t.Errorf("class %q does not match %s", c.Class, classNamePattern)
			}
			if seen[c.Class] {
				t.Errorf("class %q is declared twice, want each class once", c.Class)
			}
			seen[c.Class] = true

			if len(c.Languages) == 0 {
				t.Errorf("class %q languages = %v, want at least one of %v", c.Class, c.Languages, knownLanguages)
			}
			for _, lang := range c.Languages {
				if !slices.Contains(knownLanguages, lang) {
					t.Errorf("class %q names language %q, want one of %v", c.Class, lang, knownLanguages)
				}
			}
			if dup := slices.Compact(slices.Sorted(slices.Values(c.Languages))); len(dup) != len(c.Languages) {
				t.Errorf("class %q languages = %v, want each language once", c.Class, c.Languages)
			}
			if !slices.Contains(knownConfidences, c.Confidence) {
				t.Errorf("class %q confidence = %q, want one of %v", c.Class, c.Confidence, knownConfidences)
			}
			if c.Rule == "" {
				t.Errorf("class %q rule = %q, want a detection rule", c.Class, c.Rule)
			}
			if c.Retains == "" {
				t.Errorf("class %q retains = %q, want what the class keeps", c.Class, c.Retains)
			}

			mechanismLangs := slices.Sorted(maps.Keys(c.Mechanism))
			wantLangs := slices.Sorted(slices.Values(c.Languages))
			if !slices.Equal(mechanismLangs, wantLangs) {
				t.Errorf("class %q mechanism languages = %v, want exactly its languages %v", c.Class, mechanismLangs, wantLangs)
			}
			for lang, text := range c.Mechanism {
				if text == "" {
					t.Errorf("class %q mechanism[%q] = %q, want the rule in that language's terms", c.Class, lang, text)
				}
			}

			runsOnTS := slices.Contains(c.Languages, "ts")
			switch {
			case runsOnTS && c.Visibility == nil:
				t.Errorf("class %q runs on ts with no typescript_visibility, want the private versus #private answer", c.Class)
			case !runsOnTS && c.Visibility != nil:
				t.Errorf("class %q carries typescript_visibility %+v without running on ts, want the field absent", c.Class, *c.Visibility)
			case runsOnTS && c.Visibility.PrivateName:
				t.Errorf("class %q typescript_visibility.private_name = true, want false: no class reaches a #private member", c.Class)
			}
		})
	}
}

func TestExemptionsRowsCarryEveryField(t *testing.T) {
	data, err := fs.ReadFile(spec.Contract, exemptionsPath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", exemptionsPath, err)
	}
	var raw struct {
		Exemptions []map[string]json.RawMessage `json:"exemptions"`
	}
	if err = json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Setup: decoding %s: %v", exemptionsPath, err)
	}
	for i, row := range raw.Exemptions {
		var class string
		_ = json.Unmarshal(row["class"], &class)
		var languages []string
		_ = json.Unmarshal(row["languages"], &languages)

		want := slices.Clone(rowKeys)
		if slices.Contains(languages, "ts") {
			want = append(want, "typescript_visibility")
		}
		if _, ok := row["sinks"]; ok {
			want = append(want, "sinks")
		}
		slices.Sort(want)
		got := slices.Sorted(maps.Keys(row))
		if !slices.Equal(got, want) {
			t.Errorf("exemptions[%d] (%q) keys = %v, want %v", i, class, got, want)
		}
	}
}

// TestExemptionsPublicTextsHoldNoSemicolon pins that every text an analyzer
// copies into its output, a class's rule, what it retains and each mechanism,
// is written as plain sentences with no semicolon, because explanations and
// the retained-symbol listing show it to users verbatim.
func TestExemptionsPublicTextsHoldNoSemicolon(t *testing.T) {
	for _, class := range loadExemptions(t).Exemptions {
		texts := map[string]string{"rule": class.Rule, "retains": class.Retains}
		for lang, text := range class.Mechanism {
			texts["mechanism["+lang+"]"] = text
		}
		for field, text := range texts {
			if strings.Contains(text, ";") {
				t.Errorf("Class(%s).%s = %q, want no semicolon", class.Class, field, text)
			}
		}
	}
}
