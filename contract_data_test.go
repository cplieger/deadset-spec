package spec_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	spec "github.com/cplieger/deadset-spec/v8"
)

const (
	languagesPath  = "contract/languages.json"
	cliPath        = "contract/cli.json"
	confidencePath = "contract/confidence.json"
	analysisDir    = "contract/grammar/analysis"
)

// strictDecode decodes an embedded Contract document, refusing a member the
// target type does not declare, so a key a document gains fails until a test
// reads it.
func strictDecode(t *testing.T, path string, into any) {
	t.Helper()
	data, err := fs.ReadFile(spec.Contract, path)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", path, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		t.Fatalf("Setup: decoding %s: %v", path, err)
	}
}

type languagesDocument struct {
	Description        string        `json:"description"`
	SkippedDirectories []string      `json:"skipped_directories"`
	Languages          []languageRow `json:"languages"`
}

type languageRow struct {
	IdentitySource       map[string]string `json:"identity_source"`
	Language             string            `json:"language"`
	Kind                 string            `json:"kind"`
	Scheme               string            `json:"scheme"`
	Runner               string            `json:"runner"`
	Analyzer             string            `json:"analyzer"`
	Markers              []string          `json:"markers"`
	Extensions           []string          `json:"extensions"`
	Companions           []string          `json:"companions"`
	WorkspaceFiles       []string          `json:"workspace_files"`
	ProgramWithoutMarker bool              `json:"program_without_marker"`
	SkipsUnderscored     bool              `json:"skips_underscored"`
	TakesTargetKind      bool              `json:"takes_target_kind"`
}

// TestLanguagesAreTheVocabularysLanguages pins languages.json to the language
// vocabulary of kinds.json and each row's own closed values: every language a
// kind can name has one row, and a row that reads a program from source files
// alone names the companions that tell its tree apart.
func TestLanguagesAreTheVocabularysLanguages(t *testing.T) {
	var doc languagesDocument
	strictDecode(t, languagesPath, &doc)
	var ids []string
	for _, row := range doc.Languages {
		ids = append(ids, row.Language)
		t.Run(row.Language, func(t *testing.T) {
			if row.Kind != "analyzed" && row.Kind != "schema" {
				t.Errorf("%s kind = %q, want analyzed or schema", row.Language, row.Kind)
			}
			if row.Runner != "process" && row.Runner != "in-process" {
				t.Errorf("%s runner = %q, want process or in-process", row.Language, row.Runner)
			}
			if row.Runner == "process" && row.Analyzer == "" {
				t.Errorf("%s runs as a process and names no analyzer", row.Language)
			}
			if len(row.Markers) == 0 && !row.ProgramWithoutMarker {
				t.Errorf("%s has no marker and reads no program without one, so nothing puts it in scope", row.Language)
			}
			if row.ProgramWithoutMarker && len(row.Companions) == 0 {
				t.Errorf("%s reads a program without a marker and names no companion", row.Language)
			}
			if row.IdentitySource["file"] == "" || row.IdentitySource["field"] == "" {
				t.Errorf("%s identity_source = %v, want a file and a field", row.Language, row.IdentitySource)
			}
		})
	}
	if want := loadKinds(t).Languages; !slices.Equal(ids, want) {
		t.Errorf("%s languages = %v, want the vocabulary's %v", languagesPath, ids, want)
	}
}

type cliDocument struct {
	Description   string      `json:"description"`
	Options       []cliOption `json:"options"`
	AnalyzerVerbs []cliVerb   `json:"analyzer_verbs"`
	Products      []struct {
		Product   string          `json:"product"`
		Role      string          `json:"role"`
		Languages []string        `json:"languages"`
		Verbs     json.RawMessage `json:"verbs"`
	} `json:"products"`
}

type cliOption struct {
	Option     string `json:"option"`
	Value      string `json:"value"`
	Setting    string `json:"setting"`
	Summary    string `json:"summary"`
	Repeatable bool   `json:"repeatable"`
}

type cliVerb struct {
	Options   json.RawMessage `json:"options"`
	Verb      string          `json:"verb"`
	Summary   string          `json:"summary"`
	Stdout    string          `json:"stdout"`
	Arguments []string        `json:"arguments"`
	ExitCodes []int           `json:"exit_codes"`
}

// TestCLIOptionsNameDeclaredSettingsAndExitCodes holds cli.json to the other
// documents: a setting option supplies a key the configuration schema declares,
// every verb's option is one the table declares, and every exit code a verb
// lists is one exit-codes.json declares.
func TestCLIOptionsNameDeclaredSettingsAndExitCodes(t *testing.T) {
	var doc cliDocument
	strictDecode(t, cliPath, &doc)
	declared := map[string]bool{}
	config := loadConfigSchema(t)
	for _, o := range doc.Options {
		if declared[o.Option] {
			t.Errorf("option %q is declared twice", o.Option)
		}
		declared[o.Option] = true
		if o.Setting != "" && objectAt(config, "properties/"+strings.ReplaceAll(o.Setting, ".", "/properties/")) == nil {
			t.Errorf("option %q supplies %q, a key %s does not declare", o.Option, o.Setting, configSchemaPath)
		}
	}
	codes := map[int]bool{}
	for _, row := range mustLoadExitCodes(t).ExitCodes {
		codes[row.Code] = true
	}
	verbs := slices.Clone(doc.AnalyzerVerbs)
	for _, p := range doc.Products {
		var own []cliVerb
		if json.Unmarshal(p.Verbs, &own) == nil {
			verbs = append(verbs, own...)
		}
	}
	for _, v := range verbs {
		var byLanguage map[string][]string
		var flat []string
		if json.Unmarshal(v.Options, &byLanguage) != nil {
			if err := json.Unmarshal(v.Options, &flat); err != nil {
				t.Errorf("verb %q options = %s, want a list or a list per language", v.Verb, v.Options)
			}
		}
		for _, list := range byLanguage {
			flat = append(flat, list...)
		}
		for _, name := range flat {
			if !declared[name] {
				t.Errorf("verb %q takes %q, an option the table does not declare", v.Verb, name)
			}
		}
		for _, code := range v.ExitCodes {
			if !codes[code] {
				t.Errorf("verb %q ends with %d, a code %s does not declare", v.Verb, code, exitCodesPath)
			}
		}
	}
}

type confidenceDocument struct {
	Description string `json:"description"`
	Evidence    []struct {
		Evidence    string   `json:"evidence"`
		Rule        string   `json:"rule"`
		Class       string   `json:"class"`
		Description string   `json:"description"`
		Languages   []string `json:"languages"`
		Codes       []string `json:"codes"`
	} `json:"evidence"`
}

// TestConfidenceEvidenceNamesLiveCodesAndStatedRules holds each evidence row to
// a live code, a class below certain and a rule id one analysis page states.
func TestConfidenceEvidenceNamesLiveCodesAndStatedRules(t *testing.T) {
	var doc confidenceDocument
	strictDecode(t, confidencePath, &doc)
	kinds := loadKinds(t)
	rules := analysisRuleIDs(t)
	for _, e := range doc.Evidence {
		t.Run(e.Evidence+"_"+strings.Join(e.Languages, "_"), func(t *testing.T) {
			if e.Class == "certain" || !slices.Contains(kinds.ReachabilityClasses, e.Class) {
				t.Errorf("class = %q, want a class below certain", e.Class)
			}
			if !slices.Contains(rules, e.Rule) {
				t.Errorf("rule = %q, want a rule id an analysis page states", e.Rule)
			}
			for _, code := range e.Codes {
				if !slices.ContainsFunc(kinds.Kinds, func(k kindRow) bool { return k.Code == code }) {
					t.Errorf("codes name %q, want a live code", code)
				}
			}
		})
	}
}

var ruleID = regexp.MustCompile(`\*\*([a-z]+(?:-[a-z]+)*)\.([0-9]+)\*\*`)

// analysisRuleIDs lists every rule id the analysis pages state.
func analysisRuleIDs(t *testing.T) []string {
	t.Helper()
	pages, err := fs.Glob(spec.Contract, analysisDir+"/*.md")
	if err != nil || len(pages) == 0 {
		t.Fatalf("Setup: fs.Glob(%s) = %v, %v, want the rule pages", analysisDir, pages, err)
	}
	var ids []string
	for _, page := range pages {
		data, err := fs.ReadFile(spec.Contract, page)
		if err != nil {
			t.Fatalf("Setup: fs.ReadFile(%s): %v", page, err)
		}
		for _, m := range ruleID.FindAllStringSubmatch(string(data), -1) {
			ids = append(ids, m[1]+"."+m[2])
		}
	}
	return ids
}

// TestAnalysisRulesAreNumberedInOrder pins the rule ids of every page: each is
// the page's own name and a number, the numbers run from 1 without a gap, and
// the page's opening line states the range, so a rule cited by id names one
// paragraph of one page.
func TestAnalysisRulesAreNumberedInOrder(t *testing.T) {
	pages, err := fs.Glob(spec.Contract, analysisDir+"/*.md")
	if err != nil {
		t.Fatalf("Setup: fs.Glob(%s): %v", analysisDir, err)
	}
	for _, page := range pages {
		name := strings.TrimSuffix(page[len(analysisDir)+1:], ".md")
		t.Run(name, func(t *testing.T) {
			data, err := fs.ReadFile(spec.Contract, page)
			if err != nil {
				t.Fatalf("Setup: fs.ReadFile(%s): %v", page, err)
			}
			matches := ruleID.FindAllStringSubmatch(string(data), -1)
			for i, m := range matches {
				if m[1] != name || m[2] != strconv.Itoa(i+1) {
					t.Errorf("rule %d of %s is %s.%s, want %s.%d", i+1, page, m[1], m[2], name, i+1)
				}
			}
			want := "Rules `" + name + ".1` to `" + name + "." + strconv.Itoa(len(matches)) + "`"
			if !strings.Contains(string(data), want) {
				t.Errorf("%s does not open with %q", page, want)
			}
		})
	}
}
