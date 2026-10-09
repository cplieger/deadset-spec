package spec_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	spec "github.com/cplieger/deadset-spec/v7"
)

// baselineVectorsDir holds one directory per published baseline case: the report of one
// round and the baseline document that round records.
const baselineVectorsDir = "vectors/baseline"

// baselineRow is one row of a baseline document.
type baselineRow struct {
	Code   string `json:"code"`
	Symbol string `json:"symbol"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// baselineDocument is a baseline as an analyzer writes it.
type baselineDocument struct {
	Description string        `json:"description"`
	Baseline    []baselineRow `json:"baseline"`
}

// baselineRound is the part of a report one round's rows are drawn from.
type baselineRound struct {
	Analyzer struct {
		Name    string `json:"name"`
		Version string `json:"version"`
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
		row := baselineRow{
			Code: found.Code, Symbol: found.Symbol.Ref, Path: found.Position.Path,
			Reason: "recorded by " + round.Analyzer.Name + " " + round.Analyzer.Version,
		}
		if !slices.Contains(rows, row) {
			rows = append(rows, row)
		}
	}
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

// loadBaselineCase reads one case's report and its expected document, strictly.
func loadBaselineCase(fsys fs.FS, dir string) ([]byte, baselineDocument, error) {
	at := path.Join(baselineVectorsDir, dir)
	entries, err := fs.ReadDir(fsys, at)
	if err != nil {
		return nil, baselineDocument{}, err
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	if !slices.Equal(names, []string{"expected.json", "report.json"}) {
		return nil, baselineDocument{}, fmt.Errorf("files %v, want expected.json and report.json", names)
	}
	report, err := fs.ReadFile(fsys, path.Join(at, "report.json"))
	if err != nil {
		return nil, baselineDocument{}, err
	}
	raw, err := fs.ReadFile(fsys, path.Join(at, "expected.json"))
	if err != nil {
		return nil, baselineDocument{}, err
	}
	var expected baselineDocument
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&expected); err != nil {
		return nil, baselineDocument{}, fmt.Errorf("expected.json: %w", err)
	}
	if expected.Baseline == nil {
		return nil, baselineDocument{}, fmt.Errorf("expected.json carries no baseline array")
	}
	return report, expected, nil
}

// baselineReproductionProblems computes one case's rows and compares them with its own.
func baselineReproductionProblems(rowKinds []string, fsys fs.FS, dir string) []string {
	report, expected, err := loadBaselineCase(fsys, dir)
	if err != nil {
		return []string{err.Error()}
	}
	rows, err := recordedRows(report, rowKinds)
	if err != nil {
		return []string{err.Error()}
	}
	if !slices.Equal(rows, expected.Baseline) {
		return []string{fmt.Sprintf("rows = %+v, want %+v", rows, expected.Baseline)}
	}
	return nil
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
			report, _, err := loadBaselineCase(spec.Vectors, dir)
			if err != nil {
				t.Fatalf("loadBaselineCase(%s): %v", dir, err)
			}
			validateAgainst(t, reportSchemaPath, report)
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
		report, expected, err := loadBaselineCase(spec.Vectors, dir)
		if err != nil {
			t.Fatalf("loadBaselineCase(%s): %v", dir, err)
		}
		var round baselineRound
		if err := json.Unmarshal(report, &round); err != nil {
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
		empty = empty || len(expected.Baseline) == 0
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
	}
	rowKinds := documentRowKinds(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := fstest.MapFS{}
			at := path.Join(baselineVectorsDir, tc.dir)
			for _, name := range []string{"report.json", "expected.json"} {
				data, err := fs.ReadFile(spec.Vectors, path.Join(at, name))
				if err != nil {
					t.Fatalf("Setup: %v", err)
				}
				m[path.Join(at, name)] = &fstest.MapFile{Data: data}
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
