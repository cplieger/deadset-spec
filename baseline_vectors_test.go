package spec_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	spec "github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/corpus"
	"github.com/cplieger/deadset-spec/v8/report"
	"github.com/cplieger/deadset-spec/v8/suppress"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// baselineVectorsDir holds one directory per published baseline case: the report of one
// round, the document the write starts from where the case has one, and the outcome.
const baselineVectorsDir = "vectors/baseline"

// baselineRow is one row of a written baseline. path and reason are pointers, because
// a row of the other scheme a write keeps holds exactly the members its document held.
type baselineRow struct {
	Code   string  `json:"code"`
	Symbol string  `json:"symbol"`
	Path   *string `json:"path,omitempty"`
	Reason *string `json:"reason,omitempty"`
}

// baselineDocument is a baseline as an analyzer writes it.
type baselineDocument struct {
	Description string        `json:"description"`
	Baseline    []baselineRow `json:"baseline"`
}

// baselineRefusal is the outcome of a case whose write refuses the document it finds.
type baselineRefusal struct {
	ExitCode int `json:"exit_code"`
}

// baselineCase is one case as read from its directory. existing is nil where the case
// starts from no document, and exactly one of expected and refusal is set.
type baselineCase struct {
	expected *baselineDocument
	written  []byte
	refusal  *baselineRefusal
	report   []byte
	existing []byte
}

// baselineRound is the part of a report one round's rows are drawn from.
type baselineRound struct {
	Analyzer struct {
		Languages []string `json:"languages"`
	} `json:"analyzer"`
	Findings []struct {
		Code     string `json:"code"`
		Position struct {
			Path string `json:"path"`
		} `json:"position"`
		Symbol struct {
			Ref  string `json:"ref"`
			Kind string `json:"kind"`
		} `json:"symbol"`
	} `json:"findings"`
}

// documentRowKinds is the symbol kinds corpus/corpus.json names a row of a document,
// the subjects grammar/suppression.md records no baseline row for.
func documentRowKinds(t *testing.T) []string {
	t.Helper()
	raw, err := fs.ReadFile(spec.Corpus, "corpus/corpus.json")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	var doc struct {
		SubjectShapes struct {
			Row []string `json:"row"`
		} `json:"subject_shapes"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil || len(doc.SubjectShapes.Row) == 0 {
		t.Fatalf("Setup: corpus.json subject_shapes.row = %v, %v", doc.SubjectShapes.Row, err)
	}
	return doc.SubjectShapes.Row
}

// baselineCaseFiles are the file sets a case may hold, each sorted.
var baselineCaseFiles = [][]string{
	{"expected.json", "report.json"},
	{"existing.json", "expected.json", "report.json"},
	{"existing.json", "expected-error.json", "report.json"},
}

// loadBaselineCase reads one case's files, strictly.
func loadBaselineCase(fsys fs.FS, dir string) (baselineCase, error) {
	at := path.Join(baselineVectorsDir, dir)
	entries, err := fs.ReadDir(fsys, at)
	if err != nil {
		return baselineCase{}, err
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !slices.ContainsFunc(baselineCaseFiles, func(set []string) bool { return slices.Equal(set, names) }) {
		return baselineCase{}, fmt.Errorf("files %v, want one of %v", names, baselineCaseFiles)
	}
	read := func(name string) []byte {
		if err == nil && slices.Contains(names, name) {
			var data []byte
			data, err = fs.ReadFile(fsys, path.Join(at, name))
			return data
		}
		return nil
	}
	c := baselineCase{report: read("report.json"), existing: read("existing.json")}
	if raw := read("expected-error.json"); raw != nil {
		c.refusal = &baselineRefusal{}
		if err := corpus.DecodeStrict(raw, c.refusal); err != nil {
			return baselineCase{}, fmt.Errorf("expected-error.json: %w", err)
		}
	}
	if raw := read("expected.json"); raw != nil {
		c.written = raw
		c.expected = &baselineDocument{}
		if err := corpus.DecodeStrict(raw, c.expected); err != nil {
			return baselineCase{}, fmt.Errorf("expected.json: %w", err)
		}
		if c.expected.Baseline == nil {
			return baselineCase{}, errors.New("expected.json carries no baseline array")
		}
	}
	return c, err
}

// writtenBaseline is the document one round of a case writes through the suppress
// package: a row per recordable finding of the round's uncapped report, in report
// order and each once, and every row of the document it starts from whose scheme is
// not the round's, as held.
func writtenBaseline(c *baselineCase) ([]byte, error) {
	r, err := report.Decode(c.report)
	if err != nil {
		return nil, err
	}
	if r.Totals.Omitted != 0 {
		return nil, fmt.Errorf("the report omits %d findings, and a round records from the uncapped set", r.Totals.Omitted)
	}
	var rows []suppress.Recorded
	for i := range r.Findings {
		if found := &r.Findings[i]; suppress.Recordable(found) {
			row := suppress.Recorded{Code: found.Code, Symbol: found.Symbol.Ref, Path: found.Position.Path}
			if !slices.Contains(rows, row) {
				rows = append(rows, row)
			}
		}
	}
	entries, err := suppress.ReadBaseline(c.existing)
	if err != nil {
		return nil, err
	}
	var written bytes.Buffer
	provenance := suppress.Provenance{Analyzer: r.Analyzer.Name, Version: r.Analyzer.Version}
	err = suppress.WriteBaseline(&written, rows, provenance, suppress.OtherRows(entries, string(r.Analyzer.Languages[0])+"://"))
	return written.Bytes(), err
}

// baselineReproductionProblems computes one case's outcome and compares it with its own.
func baselineReproductionProblems(fsys fs.FS, dir string) []string {
	c, err := loadBaselineCase(fsys, dir)
	if err != nil {
		return []string{err.Error()}
	}
	written, err := writtenBaseline(&c)
	switch {
	case c.refusal != nil && !errors.Is(err, suppress.ErrMalformed):
		return []string{fmt.Sprintf("the write = %s, %v, want it refused", written, err)}
	case c.refusal != nil && c.refusal.ExitCode != vocab.ExitFailure:
		return []string{fmt.Sprintf("exit_code = %d, want %d, the code a write that leaves the document as it is ends with", c.refusal.ExitCode, vocab.ExitFailure)}
	case c.refusal != nil:
		return nil
	case err != nil:
		return []string{err.Error()}
	case !bytes.Equal(written, c.written):
		return []string{fmt.Sprintf("the written document =\n%s\nwant the bytes of expected.json:\n%s", written, c.written)}
	}
	return nil
}

// TestBaselineVectorsCoverEveryRowKindLeftOut holds the case set to a finding of every
// document-row kind an analyzer reports, a part finding written twice under one code,
// and a round that records nothing.
func TestBaselineVectorsCoverEveryRowKindLeftOut(t *testing.T) {
	seen := map[string]bool{}
	empty, twice := false, false
	for _, dir := range vectorCaseDirs(t, spec.Vectors, "baseline") {
		c, err := loadBaselineCase(spec.Vectors, dir)
		if err != nil {
			t.Fatalf("loadBaselineCase(%s): %v", dir, err)
		}
		var round baselineRound
		if err := json.Unmarshal(c.report, &round); err != nil {
			t.Fatalf("decode %s: %v", dir, err)
		}
		keys := map[string]int{}
		for _, found := range round.Findings {
			seen[found.Symbol.Kind] = true
			keys[found.Code+"\n"+found.Symbol.Ref+"\n"+found.Position.Path]++
		}
		for _, n := range keys {
			twice = twice || n > 1
		}
		empty = empty || c.existing == nil && len(c.expected.Baseline) == 0
	}
	// An analyzer never reports a declared edge: the merge does.
	for _, kind := range documentRowKinds(t) {
		if kind != "edge" && !seen[kind] {
			t.Errorf("cases holding a finding about a %s = 0, want at least one", kind)
		}
	}
	if !twice || !empty {
		t.Errorf("a case with two findings sharing code, symbol and path = %t, a case recording no row = %t, want both", twice, empty)
	}
}

// TestBaselineReproductionRefuses plants one departure from the page per rule into a copy
// of a published case and requires the reproduction to see it.
func TestBaselineReproductionRefuses(t *testing.T) {
	tests := []struct {
		name, dir, file, old, replacement string
	}{
		{name: "a_document_row_recorded", dir: "document-rows-left-out", file: "report.json", old: `"kind": "root"`, replacement: `"kind": "function"`},
		{name: "a_reason_without_the_version", dir: "declarations-and-parts", file: "expected.json", old: `"recorded by deadset-go 1.20.2"`, replacement: `"recorded by deadset-go"`},
		{name: "a_part_row_written_twice", dir: "declarations-and-parts", file: "expected.json", old: "\n  ]\n}", replacement: `,
    {"code": "DS1401", "symbol": "go://example.com/app#scanner.feed", "path": "catalog.go", "reason": "recorded by deadset-go 1.20.2"}
  ]
}`},
		{name: "an_own_old_row_kept", dir: "other-scheme-rows-kept", file: "expected.json", old: "  ]\n}", replacement: `,
    {"code": "DS1002", "symbol": "go://example.com/app#gone", "path": "main.go", "reason": "recorded by deadset-go 1.20.1"}
  ]
}`},
		{name: "a_kept_row_given_a_member_it_left_out", dir: "other-scheme-rows-kept", file: "expected.json", old: `"symbol": "ts://@example/app/src/panel.ts#Panel.size",`, replacement: `"symbol": "ts://@example/app/src/panel.ts#Panel.size",
      "path": "src/panel.ts",`},
		{name: "the_round_read_as_the_other_language", dir: "one-document-written-by-typescript", file: "report.json", old: `"ts"
    ],`, replacement: `"go"
    ],`},
		{name: "an_unreadable_row_written_over", dir: "other-scheme-row-unreadable", file: "existing.json", old: `,
      "note": "kept by hand"`, replacement: ""},
		{name: "a_document_with_no_baseline_written_over", dir: "existing-document-not-a-baseline", file: "existing.json", old: `"Rows go here."`, replacement: `"Rows go here.",
  "baseline": []`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := fstest.MapFS{}
			at := path.Join(baselineVectorsDir, tc.dir)
			entries, err := fs.ReadDir(spec.Vectors, at)
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}
			for _, entry := range entries {
				data, err := fs.ReadFile(spec.Vectors, path.Join(at, entry.Name()))
				if err != nil {
					t.Fatalf("Setup: %v", err)
				}
				m[path.Join(at, entry.Name())] = &fstest.MapFile{Data: data}
			}
			planted := m[path.Join(at, tc.file)]
			if !bytes.Contains(planted.Data, []byte(tc.old)) {
				t.Fatalf("Setup: %s holds no %q", tc.file, tc.old)
			}
			planted.Data = bytes.Replace(planted.Data, []byte(tc.old), []byte(tc.replacement), 1)
			if len(baselineReproductionProblems(m, tc.dir)) == 0 {
				t.Errorf("baselineReproductionProblems(%s with %s %q) = none, want the departure named", tc.dir, tc.file, tc.replacement)
			}
		})
	}
}

// TestBaselineVectorsWriteOneDocumentWhicheverAnalyzerWrites holds every two cases that
// start from the same document bytes, with reports of different languages, to one
// expected document, and requires such a pair, so the order two analyzers write in is
// pinned to decide nothing.
func TestBaselineVectorsWriteOneDocumentWhicheverAnalyzerWrites(t *testing.T) {
	type started struct {
		c         baselineCase
		dir       string
		languages []string
	}
	var cases []started
	for _, dir := range vectorCaseDirs(t, spec.Vectors, "baseline") {
		c, err := loadBaselineCase(spec.Vectors, dir)
		if err != nil {
			t.Fatalf("loadBaselineCase(%s): %v", dir, err)
		}
		var round baselineRound
		if err := json.Unmarshal(c.report, &round); err != nil {
			t.Fatalf("decode %s: %v", dir, err)
		}
		if c.existing != nil && c.expected != nil {
			cases = append(cases, started{c: c, dir: dir, languages: round.Analyzer.Languages})
		}
	}
	pairs := 0
	for i, a := range cases {
		for _, b := range cases[i+1:] {
			if !bytes.Equal(a.c.existing, b.c.existing) || slices.Equal(a.languages, b.languages) {
				continue
			}
			pairs++
			if !bytes.Equal(a.c.written, b.c.written) {
				t.Errorf("%s and %s start from one document and expect\n%s\nand\n%s\nwant the same bytes", a.dir, b.dir,
					a.c.written, b.c.written)
			}
		}
	}
	if pairs == 0 {
		t.Error("cases starting from one document with reports of two languages = 0, want at least one pair")
	}
}

// TestBaselineVectorsCoverTheDocumentAWriteFinds requires a refused case for a row of each
// scheme in no record's shape and for a document holding no baseline array, and a kept
// row that leaves out a member.
func TestBaselineVectorsCoverTheDocumentAWriteFinds(t *testing.T) {
	refused := map[string]bool{}
	leftOut := false
	for _, dir := range vectorCaseDirs(t, spec.Vectors, "baseline") {
		c, err := loadBaselineCase(spec.Vectors, dir)
		if err != nil {
			t.Fatalf("loadBaselineCase(%s): %v", dir, err)
		}
		if c.expected != nil {
			for _, row := range c.expected.Baseline {
				leftOut = leftOut || row.Path == nil || row.Reason == nil
			}
			continue
		}
		var held struct {
			Baseline []struct {
				Symbol string `json:"symbol"`
			} `json:"baseline"`
		}
		if err := json.Unmarshal(c.existing, &held); err != nil || held.Baseline == nil {
			refused["no baseline array"] = true
			continue
		}
		var round baselineRound
		if err := json.Unmarshal(c.report, &round); err != nil {
			t.Fatalf("decode %s: %v", dir, err)
		}
		for _, row := range held.Baseline {
			if _, err := suppress.ReadBaseline([]byte(`{"baseline": [` + rowJSON(t, c.existing, row.Symbol) + `]}`)); err != nil {
				own := slices.ContainsFunc(round.Analyzer.Languages, func(l string) bool { return strings.HasPrefix(row.Symbol, l+"://") })
				refused[map[bool]string{true: "own scheme", false: "other scheme"}[own]] = true
			}
		}
	}
	for _, want := range []string{"own scheme", "other scheme", "no baseline array"} {
		if !refused[want] {
			t.Errorf("refused cases for %s = 0, want at least one", want)
		}
	}
	if !leftOut {
		t.Error("expected rows leaving out a path or a reason = 0, want at least one")
	}
}

// rowJSON is the one row of a held document whose symbol is symbol, re-encoded.
func rowJSON(t *testing.T, existing []byte, symbol string) string {
	t.Helper()
	var held struct {
		Baseline []map[string]any `json:"baseline"`
	}
	if err := json.Unmarshal(existing, &held); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	for _, row := range held.Baseline {
		if row["symbol"] == symbol {
			text, err := json.Marshal(row)
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}
			return string(text)
		}
	}
	t.Fatalf("Setup: no row names %s", symbol)
	return ""
}

// baselineFamily is the baseline vectors: each case's report, and the document
// the suppress package writes from it.
func baselineFamily() vectorFamily {
	return vectorFamily{
		name:   "baseline",
		readme: true,
		reports: func(fsys fs.FS, dir string) ([][]byte, error) {
			c, err := loadBaselineCase(fsys, dir)
			return [][]byte{c.report}, err
		},
		problems: func(t *testing.T, fsys fs.FS, dir string) []string {
			t.Helper()
			return baselineReproductionProblems(fsys, dir)
		},
		render: func(t *testing.T, dir string) (string, []byte, bool) {
			t.Helper()
			c, err := loadBaselineCase(os.DirFS("."), dir)
			if err != nil {
				t.Fatalf("loadBaselineCase(%s): %v", dir, err)
			}
			if c.refusal != nil {
				return "", nil, false
			}
			written, err := writtenBaseline(&c)
			if err != nil {
				t.Fatalf("writtenBaseline(%s): %v", dir, err)
			}
			return "expected.json", written, true
		},
	}
}
