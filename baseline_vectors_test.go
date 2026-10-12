package spec_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	spec "github.com/cplieger/deadset-spec/v7"
)

// baselineVectorsDir holds one directory per published baseline case: the report of one
// round, the document the write starts from where the case has one, and the outcome.
const baselineVectorsDir = "vectors/baseline"

// baselineRow is one row of a baseline document. path and reason are pointers, because
// a row of the other scheme a write keeps holds exactly the members its document held.
type baselineRow struct {
	Code   string  `json:"code"`
	Symbol string  `json:"symbol"`
	Path   *string `json:"path,omitempty"`
	Reason *string `json:"reason,omitempty"`
}

// sameRow reports whether two rows hold the same members with the same values.
func sameRow(a, b baselineRow) bool {
	samePart := func(x, y *string) bool { return (x == nil) == (y == nil) && (x == nil || *x == *y) }
	return a.Code == b.Code && a.Symbol == b.Symbol && samePart(a.Path, b.Path) && samePart(a.Reason, b.Reason)
}

// baselineDocument is a baseline as an analyzer writes it.
type baselineDocument struct {
	Description string        `json:"description"`
	Baseline    []baselineRow `json:"baseline"`
}

// baselineDescription is the description grammar/suppression.md fixes for a written
// baseline.
const baselineDescription = "Recorded findings: a run fails only on a finding this document does not hold. " +
	"Every reason is the analyzer that recorded the row, never an adjudication."

// writtenDocument is the bytes grammar/suppression.md fixes for a baseline holding rows:
// encoding/json with no HTML escaping writes U+2028 and U+2029 as escapes and every
// other character JSON leaves unescaped as itself, which is that form.
func writtenDocument(rows []baselineRow) ([]byte, error) {
	var written bytes.Buffer
	encoder := json.NewEncoder(&written)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(baselineDocument{Description: baselineDescription, Baseline: rows}); err != nil {
		return nil, err
	}
	return written.Bytes(), nil
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

// baselineCodeForm and baselineSchemes are the value forms the reference checks a held
// row against: the code shape, and the two schemes a reference opens with.
var (
	baselineCodeForm = regexp.MustCompile(`^DS[0-9]{4}$`)
	baselineSchemes  = []string{"go://", "ts://"}
)

// baselineRound is the part of a report one round's rows are drawn from.
type baselineRound struct {
	Analyzer struct {
		Name      string   `json:"name"`
		Version   string   `json:"version"`
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
	Totals struct {
		Omitted int `json:"omitted"`
	} `json:"totals"`
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

// recordedRows is the rows one round records from its report, as grammar/suppression.md
// states them: one per finding in report order, none for a document row, none identical
// to a row already recorded, each reason naming the analyzer that recorded it.
func recordedRows(report []byte, rowKinds []string) ([]baselineRow, error) {
	var round baselineRound
	if err := json.Unmarshal(report, &round); err != nil {
		return nil, err
	}
	if round.Totals.Omitted != 0 {
		return nil, fmt.Errorf("the report omits %d findings, and a round records from the uncapped set", round.Totals.Omitted)
	}
	rows := []baselineRow{}
	for _, found := range round.Findings {
		if slices.Contains(rowKinds, found.Symbol.Kind) {
			continue
		}
		where, reason := found.Position.Path, "recorded by "+round.Analyzer.Name+" "+round.Analyzer.Version
		row := baselineRow{Code: found.Code, Symbol: found.Symbol.Ref, Path: &where, Reason: &reason}
		if !slices.ContainsFunc(rows, func(held baselineRow) bool { return sameRow(held, row) }) {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// errUnreadableBaseline marks a document a write refuses rather than overwrite.
var errUnreadableBaseline = errors.New("the baseline document cannot be read")

// heldRows decodes the document a write starts from as grammar/suppression.md requires:
// an object of the closed key list holding a baseline array, each row of the closed key
// list with string members, a code of the code form and a symbol of a known scheme.
func heldRows(existing []byte) ([]baselineRow, error) {
	var held struct {
		Description *string        `json:"description"`
		Baseline    *[]baselineRow `json:"baseline"`
	}
	decoder := json.NewDecoder(bytes.NewReader(existing))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&held); err != nil {
		return nil, fmt.Errorf("%w: %w", errUnreadableBaseline, err)
	}
	if decoder.More() || held.Baseline == nil {
		return nil, fmt.Errorf("%w: want one object holding a baseline array", errUnreadableBaseline)
	}
	for i, row := range *held.Baseline {
		known := slices.ContainsFunc(baselineSchemes, func(scheme string) bool { return strings.HasPrefix(row.Symbol, scheme) })
		if !baselineCodeForm.MatchString(row.Code) || !known {
			return nil, fmt.Errorf("%w: row %d is in no record's shape", errUnreadableBaseline, i)
		}
	}
	return *held.Baseline, nil
}

// writtenRows is the baseline array one write produces: the round's own rows, and every
// row of existing whose scheme is not the round's, as held, the go:// rows ahead of the
// ts:// rows. A document heldRows refuses refuses the write.
func writtenRows(report, existing []byte, rowKinds []string) ([]baselineRow, error) {
	own, err := recordedRows(report, rowKinds)
	if err != nil || existing == nil {
		return own, err
	}
	held, err := heldRows(existing)
	if err != nil {
		return nil, err
	}
	var round baselineRound
	if err := json.Unmarshal(report, &round); err != nil {
		return nil, err
	}
	ours := func(row baselineRow) bool {
		return slices.ContainsFunc(round.Analyzer.Languages, func(language string) bool { return strings.HasPrefix(row.Symbol, language+"://") })
	}
	rows := slices.Clone(own)
	for _, row := range held {
		if !ours(row) {
			rows = append(rows, row)
		}
	}
	slices.SortStableFunc(rows, func(a, b baselineRow) int {
		return slices.IndexFunc(baselineSchemes, func(s string) bool { return strings.HasPrefix(a.Symbol, s) }) -
			slices.IndexFunc(baselineSchemes, func(s string) bool { return strings.HasPrefix(b.Symbol, s) })
	})
	return rows, nil
}

// baselineCaseDirs names every case directory under the baseline vectors.
func baselineCaseDirs(t *testing.T, fsys fs.FS) []string {
	t.Helper()
	entries, err := fs.ReadDir(fsys, baselineVectorsDir)
	if err != nil {
		t.Fatalf("Setup: fs.ReadDir(%s): %v", baselineVectorsDir, err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(dirs) == 0 {
		t.Fatalf("Setup: %s holds no case", baselineVectorsDir)
	}
	return dirs
}

// baselineCaseFiles are the file sets a case may hold, each sorted.
var baselineCaseFiles = [][]string{
	{"expected.json", "report.json"},
	{"existing.json", "expected.json", "report.json"},
	{"existing.json", "expected-error.json", "report.json"},
}

// decodeStrictly decodes one document of a case into v with the closed key list v declares.
func decodeStrictly(raw []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
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
		if err := decodeStrictly(raw, c.refusal); err != nil {
			return baselineCase{}, fmt.Errorf("expected-error.json: %w", err)
		}
	}
	if raw := read("expected.json"); raw != nil {
		c.written = raw
		c.expected = &baselineDocument{}
		if err := decodeStrictly(raw, c.expected); err != nil {
			return baselineCase{}, fmt.Errorf("expected.json: %w", err)
		}
		if c.expected.Baseline == nil {
			return baselineCase{}, errors.New("expected.json carries no baseline array")
		}
	}
	return c, err
}

// sameRows reports whether two baseline arrays hold the same rows in the same order.
func sameRows(a, b []baselineRow) bool {
	return slices.EqualFunc(a, b, sameRow)
}

// baselineReproductionProblems computes one case's outcome and compares it with its own.
func baselineReproductionProblems(rowKinds []string, fsys fs.FS, dir string) []string {
	c, err := loadBaselineCase(fsys, dir)
	if err != nil {
		return []string{err.Error()}
	}
	rows, err := writtenRows(c.report, c.existing, rowKinds)
	switch {
	case c.refusal != nil && !errors.Is(err, errUnreadableBaseline):
		return []string{fmt.Sprintf("the write = %+v, %v, want it refused", rows, err)}
	case c.refusal != nil && c.refusal.ExitCode != 3:
		return []string{fmt.Sprintf("exit_code = %d, want 3, the code a write that leaves the document as it is ends with", c.refusal.ExitCode)}
	case c.refusal != nil:
		return nil
	case err != nil:
		return []string{err.Error()}
	case !sameRows(rows, c.expected.Baseline):
		return []string{fmt.Sprintf("rows = %s, want %s", rowsText(rows), rowsText(c.expected.Baseline))}
	}
	written, err := writtenDocument(rows)
	if err != nil {
		return []string{err.Error()}
	}
	if !bytes.Equal(written, c.written) {
		return []string{fmt.Sprintf("the written document =\n%s\nwant the bytes of expected.json:\n%s", written, c.written)}
	}
	return nil
}

// rowsText renders rows with the members each holds, for a failure message.
func rowsText(rows []baselineRow) string {
	text, err := json.Marshal(rows)
	if err != nil {
		return err.Error()
	}
	return string(text)
}

// TestBaselineVectorCasesReproduceFromTheirReports computes every case's rows with the
// reference grammar/suppression.md states, so a row the report does not determine fails here.
func TestBaselineVectorCasesReproduceFromTheirReports(t *testing.T) {
	rowKinds := documentRowKinds(t)
	for _, dir := range baselineCaseDirs(t, spec.Vectors) {
		t.Run(dir, func(t *testing.T) {
			for _, problem := range baselineReproductionProblems(rowKinds, spec.Vectors, dir) {
				t.Errorf("%s/%s: %s", baselineVectorsDir, dir, problem)
			}
		})
	}
}

// TestBaselineVectorReportsAreInstancesOfTheReportSchema holds every case's report to the
// document a round reads its findings from.
func TestBaselineVectorReportsAreInstancesOfTheReportSchema(t *testing.T) {
	for _, dir := range baselineCaseDirs(t, spec.Vectors) {
		t.Run(dir, func(t *testing.T) {
			c, err := loadBaselineCase(spec.Vectors, dir)
			if err != nil {
				t.Fatalf("loadBaselineCase(%s): %v", dir, err)
			}
			validateAgainst(t, reportSchemaPath, c.report)
		})
	}
}

// TestBaselineVectorsCoverEveryRowKindLeftOut holds the case set to a finding of every
// document-row kind an analyzer reports, a part finding written twice under one code,
// and a round that records nothing.
func TestBaselineVectorsCoverEveryRowKindLeftOut(t *testing.T) {
	seen := map[string]bool{}
	empty, twice := false, false
	for _, dir := range baselineCaseDirs(t, spec.Vectors) {
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

// TestBaselineVectorsReadmeNamesEveryCase pins the table of the vectors' README to the
// case directories, one row each.
func TestBaselineVectorsReadmeNamesEveryCase(t *testing.T) {
	readme, err := fs.ReadFile(spec.Vectors, path.Join(baselineVectorsDir, "README.md"))
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	for _, dir := range baselineCaseDirs(t, spec.Vectors) {
		if n := strings.Count(string(readme), "| `"+dir+"` |"); n != 1 {
			t.Errorf("README rows naming %s = %d, want 1", dir, n)
		}
	}
}

// TestBaselineReproductionRefuses plants one departure from the page per rule into a copy
// of a published case and requires the reference to see it.
func TestBaselineReproductionRefuses(t *testing.T) {
	tests := []struct {
		name, dir, file, old, replacement string
	}{
		{name: "a_document_row_recorded", dir: "document-rows-left-out", file: "report.json", old: `"kind": "root"`, replacement: `"kind": "function"`},
		{name: "a_reason_without_the_version", dir: "declarations-and-parts", file: "expected.json", old: `"recorded by deadset-go 1.20.2"`, replacement: `"recorded by deadset-go"`},
		{name: "a_part_row_written_twice", dir: "declarations-and-parts", file: "expected.json", old: "\n  ]\n}", replacement: `,
    {"code": "DS1801", "symbol": "go://example.com/app#scanner.feed", "path": "catalog.go", "reason": "recorded by deadset-go 1.20.2"}
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
	rowKinds := documentRowKinds(t)
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
			if len(baselineReproductionProblems(rowKinds, m, tc.dir)) == 0 {
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
	for _, dir := range baselineCaseDirs(t, spec.Vectors) {
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
	for _, dir := range baselineCaseDirs(t, spec.Vectors) {
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
			if _, err := heldRows([]byte(`{"baseline": [` + rowJSON(t, c.existing, row.Symbol) + `]}`)); err != nil {
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
