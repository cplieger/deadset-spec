package spec_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"testing"

	"github.com/cplieger/deadset-spec/v2"
)

const exemptionsPath = "contract/exemptions.json"

// exemptionsDocument mirrors contract/exemptions.json closely enough that an
// unknown key fails the decode; key presence is checked separately on the raw
// rows, because a missing boolean decodes as false.
type exemptionsDocument struct {
	Description string           `json:"description"`
	Exemptions  []exemptionClass `json:"exemptions"`
}

type exemptionClass struct {
	Mechanism  map[string]string `json:"mechanism"`
	Visibility *memberVisibility `json:"typescript_visibility"`
	Class      string            `json:"class"`
	Confidence string            `json:"confidence"`
	Rule       string            `json:"rule"`
	Retains    string            `json:"retains"`
	Languages  []string          `json:"languages"`
}

type memberVisibility struct {
	Private     bool `json:"private"`
	PrivateName bool `json:"private_name"`
}

// expectationFile is the part of a corpus expect.json this suite reads: the
// exemption classes its expectations name.
type expectationFile struct {
	Expect []struct {
		RetainedBy []string `json:"retained_by"`
	} `json:"expect"`
}

var (
	// classNamePattern is the shape of a class token: lowercase words joined
	// by single hyphens, so a `retained_by` value never needs quoting or
	// escaping in any reporter.
	classNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	knownLanguages   = []string{"go", "ts"}
	knownConfidences = []string{"certain", "probable", "possible"}
	rowKeys          = []string{"class", "languages", "confidence", "rule", "retains", "mechanism"}
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

// classNames returns the set of class tokens the document declares.
func classNames(doc exemptionsDocument) map[string]bool {
	names := make(map[string]bool, len(doc.Exemptions))
	for _, c := range doc.Exemptions {
		names[c.Class] = true
	}
	return names
}

// retainedBy lists, in document order, every class an expectation file names
// in a retained_by field.
func retainedBy(expect []byte) ([]string, error) {
	var file expectationFile
	if err := json.Unmarshal(expect, &file); err != nil {
		return nil, err
	}
	var refs []string
	for _, e := range file.Expect {
		refs = append(refs, e.RetainedBy...)
	}
	return refs, nil
}

// unresolved returns the references that name no declared class, in order,
// each once.
func unresolved(refs []string, classes map[string]bool) []string {
	var missing []string
	for _, ref := range refs {
		if !classes[ref] && !slices.Contains(missing, ref) {
			missing = append(missing, ref)
		}
	}
	return missing
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
		slices.Sort(want)
		got := slices.Sorted(maps.Keys(row))
		if !slices.Equal(got, want) {
			t.Errorf("exemptions[%d] (%q) keys = %v, want %v", i, class, got, want)
		}
	}
}

func TestCorpusRetainedByResolves(t *testing.T) {
	classes := classNames(loadExemptions(t))

	// Every fixture on disk. There may be none yet; the planted documents
	// below keep the resolution itself under test either way.
	fixtures, err := fs.Glob(spec.Corpus, "corpus/fixtures/*/expect.json")
	if err != nil {
		t.Fatalf("Setup: fs.Glob(Corpus, fixtures): %v", err)
	}
	for _, p := range fixtures {
		t.Run("fixture_"+path.Base(path.Dir(p)), func(t *testing.T) {
			data, err := fs.ReadFile(spec.Corpus, p)
			if err != nil {
				t.Fatalf("Setup: fs.ReadFile(Corpus, %q): %v", p, err)
			}
			refs, err := retainedBy(data)
			if err != nil {
				t.Fatalf("retainedBy(%q) = %v, want the expectation file to decode", p, err)
			}
			if got := unresolved(refs, classes); len(got) != 0 {
				t.Errorf("unresolved(%q retained_by %v) = %v, want every class declared in %s", p, refs, got, exemptionsPath)
			}
		})
	}

	// Planted expectation documents drive the same two functions, so the
	// check is live before the first fixture lands and fails if either
	// function stops discriminating.
	planted := []struct {
		name string
		doc  string
		want []string
	}{
		{
			name: "known_class",
			doc:  `{"expect":[{"symbol":"S","report":"none","retained_by":["interface-satisfaction"]}]}`,
		},
		{
			name: "known_shared_classes",
			doc:  `{"expect":[{"symbol":"S","report":"none","retained_by":["enum-group","template-field"]}]}`,
		},
		{
			name: "no_retained_by",
			doc:  `{"expect":[{"symbol":"S","report":"DS1001"}]}`,
		},
		{
			name: "unknown_class",
			doc:  `{"expect":[{"symbol":"S","report":"none","retained_by":["not-a-class"]}]}`,
			want: []string{"not-a-class"},
		},
		{
			name: "unknown_beside_known",
			doc:  `{"expect":[{"symbol":"S","report":"none","retained_by":["interface-satisfaction","not-a-class"]},{"symbol":"T","report":"none","retained_by":["not-a-class"]}]}`,
			want: []string{"not-a-class"},
		},
	}
	for _, tc := range planted {
		t.Run("planted_"+tc.name, func(t *testing.T) {
			refs, err := retainedBy([]byte(tc.doc))
			if err != nil {
				t.Fatalf("retainedBy(%s) = %v, want the planted document to decode", tc.doc, err)
			}
			got := unresolved(refs, classes)
			if !slices.Equal(got, tc.want) {
				t.Errorf("unresolved(%v) = %v, want %v", refs, got, tc.want)
			}
		})
	}
}

func TestRetainedByRejectsMalformedExpectation(t *testing.T) {
	if _, err := retainedBy([]byte(`{"expect":[`)); err == nil {
		t.Errorf("retainedBy(truncated document) = nil error, want a decode error")
	}
}
