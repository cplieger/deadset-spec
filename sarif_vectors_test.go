package spec_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf8"

	spec "github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/render"
	"github.com/cplieger/deadset-spec/v8/report"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// sarifVectorsDir holds one directory per published SARIF case.
const sarifVectorsDir = "vectors/sarif"

// The files of a SARIF case: the report rendered, the input reports a merged report
// was merged from, the target files the line fingerprints read, and either the
// expected document or the exit code the rendering ends with.
const (
	sarifReportFile   = "report.json"
	sarifInputsDir    = "inputs"
	sarifSourcesFile  = "sources.json"
	sarifExpectedFile = "expected.json"
	sarifExitFile     = "expected_exit"
)

// sarifMergeRunID is the run identifier grammar/sarif.md gives the merge's own run.
const sarifMergeRunID = "deadset/merge/"

// sarifCase is one published case, read from its directory.
type sarifCase struct {
	sources  map[string]string
	expected any
	report   []byte
	inputs   [][]byte
	exit     int
}

// loadSARIFCase reads one case, refusing a layout the vectors' README does not declare.
func loadSARIFCase(fsys fs.FS, dir string) (sarifCase, error) {
	at := path.Join(sarifVectorsDir, dir)
	entries, err := fs.ReadDir(fsys, at)
	if err != nil {
		return sarifCase{}, err
	}
	var c sarifCase
	outcomes := 0
	for _, entry := range entries {
		name := path.Join(at, entry.Name())
		switch entry.Name() {
		case sarifReportFile:
			c.report, err = fs.ReadFile(fsys, name)
		case sarifSourcesFile:
			c.sources, err = readSources(fsys, name)
		case sarifInputsDir:
			c.inputs, err = readInputs(fsys, name)
		case sarifExpectedFile:
			outcomes++
			var raw []byte
			if raw, err = fs.ReadFile(fsys, name); err == nil {
				err = decodeSARIF(raw, &c.expected)
			}
		case sarifExitFile:
			outcomes++
			var raw []byte
			if raw, err = fs.ReadFile(fsys, name); err == nil {
				c.exit, err = strconv.Atoi(strings.TrimSuffix(string(raw), "\n"))
			}
			if err == nil && c.exit != vocab.ExitFailure {
				err = fmt.Errorf("%s = %d, want %d", sarifExitFile, c.exit, vocab.ExitFailure)
			}
		default:
			err = fmt.Errorf("file %s is not one the case layout declares", entry.Name())
		}
		if err != nil {
			return sarifCase{}, err
		}
	}
	switch {
	case c.report == nil || c.sources == nil:
		return sarifCase{}, fmt.Errorf("want %s and %s", sarifReportFile, sarifSourcesFile)
	case outcomes != 1:
		return sarifCase{}, fmt.Errorf("want exactly one of %s and %s", sarifExpectedFile, sarifExitFile)
	}
	return c, nil
}

// decodeSARIF decodes a document strictly: UTF-8 with no byte order mark, one JSON text.
func decodeSARIF(raw []byte, into *any) error {
	if bytes.HasPrefix(raw, []byte("\xef\xbb\xbf")) || !utf8.Valid(raw) {
		return errors.New("the document is not UTF-8 without a byte order mark")
	}
	return json.Unmarshal(raw, into)
}

// readSources decodes a case's sources document into its path-to-content map.
func readSources(fsys fs.FS, name string) (map[string]string, error) {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Files       map[string]string `json:"files"`
		Description string            `json:"description"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: %w", sarifSourcesFile, err)
	}
	if doc.Description == "" || len(doc.Files) == 0 {
		return nil, fmt.Errorf("%s: want a description and at least one file", sarifSourcesFile)
	}
	return doc.Files, nil
}

// readInputs reads every input report of a merged case, in file-name order.
func readInputs(fsys fs.FS, dir string) ([][]byte, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var inputs [][]byte
	for _, entry := range entries {
		raw, err := fs.ReadFile(fsys, path.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, raw)
	}
	return inputs, nil
}

// renderSARIFCase renders one case: the decoded document, or the exit code a
// rendering that fails ends the run with, which writes nothing.
func renderSARIFCase(c *sarifCase) (any, int, error) {
	r, err := report.Decode(c.report)
	if err != nil {
		return nil, 0, err
	}
	var out bytes.Buffer
	if err := render.SARIF(&out, r, func(file string) ([]byte, error) {
		content, held := c.sources[file]
		if !held {
			return nil, fmt.Errorf("%s: %w", file, fs.ErrNotExist)
		}
		return []byte(content), nil
	}); err != nil {
		if out.Len() != 0 {
			return nil, 0, fmt.Errorf("the failed rendering wrote %d bytes, want none", out.Len())
		}
		return nil, vocab.ExitFailure, nil
	}
	var decoded any
	return decoded, 0, decodeSARIF(out.Bytes(), &decoded)
}

// sarifReproductionProblems renders one case and compares the outcome with the case's own.
func sarifReproductionProblems(fsys fs.FS, dir string) []string {
	c, err := loadSARIFCase(fsys, dir)
	if err != nil {
		return []string{err.Error()}
	}
	got, exit, err := renderSARIFCase(&c)
	switch {
	case err != nil:
		return []string{err.Error()}
	case exit != c.exit:
		return []string{fmt.Sprintf("exit = %d, want %d", exit, c.exit)}
	case exit == 0 && !reflect.DeepEqual(got, c.expected):
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		return []string{fmt.Sprintf("document differs from %s; the rendering is:\n%s", sarifExpectedFile, gotJSON)}
	}
	return nil
}

// TestSARIFVectorsCoverTheMappingsCases holds the case set to what the page states:
// an analyzer's run, a merged log with the merge's own run and without it, a rendering
// that fails, a colon in a first segment and a capped related-location list.
func TestSARIFVectorsCoverTheMappingsCases(t *testing.T) {
	seen := map[string]bool{}
	for _, dir := range vectorCaseDirs(t, spec.Vectors, "sarif") {
		c, err := loadSARIFCase(spec.Vectors, dir)
		if err != nil {
			t.Fatalf("loadSARIFCase(%s): %v", dir, err)
		}
		if c.exit != 0 {
			seen["a failed rendering"] = true
			continue
		}
		raw, _ := json.Marshal(c.expected)
		document := string(raw)
		log, _ := c.expected.(map[string]any)
		runs, _ := log["runs"].([]any)
		switch {
		case len(c.inputs) == 0:
			seen["an analyzer's run"] = true
		case strings.Contains(document, `"`+sarifMergeRunID+`"`):
			seen["a merged log with the merge's own run"] = true
		case len(runs) == len(c.inputs):
			seen["a merged log without the merge's own run"] = true
		}
		if strings.Contains(document, `"uri":"a%3A`) {
			seen["a colon in a first segment"] = true
		}
		if strings.Contains(document, `"id":100,`) && !strings.Contains(document, `"id":101,`) {
			seen["a capped related-location list"] = true
		}
		if strings.Contains(document, `"withheld":"withheld by analysis.min_confidence: `) {
			seen["a withheld line"] = true
		}
	}
	wants := []string{
		"an analyzer's run", "a merged log with the merge's own run", "a merged log without the merge's own run",
		"a failed rendering", "a colon in a first segment", "a capped related-location list", "a withheld line",
	}
	for _, want := range wants {
		if !seen[want] {
			t.Errorf("cases holding %s = 0, want at least one", want)
		}
	}
}

// TestSARIFReproductionRefuses plants one departure from the page per rule into a copy
// of a published case and requires the reproduction to see it.
func TestSARIFReproductionRefuses(t *testing.T) {
	tests := []struct {
		name, dir, file, old, replacement string
	}{
		{name: "the_first_segment_colon_written_raw", dir: "path-segments-encoded", file: sarifExpectedFile, old: `"a%3Ab.go"`, replacement: `"a:b.go"`},
		{name: "a_run_carrying_the_merged_totals", dir: "merged-runs-and-totals", file: sarifReportFile, old: "        \"reasons_recorded\": 1,", replacement: "        \"reasons_recorded\": 0,"},
		{name: "a_failed_rendering_written", dir: "record-naming-no-run", file: sarifReportFile, old: `"deadset-rs"`, replacement: `"deadset-ts"`},
		{name: "a_line_fingerprint_changed", dir: "line-fingerprints", file: sarifSourcesFile, old: `"y\ny\n`, replacement: `"y\nz\n`},
		{name: "a_withheld_line_naming_a_zero_count", dir: "withheld-line", file: sarifReportFile, old: `"probable": 2`, replacement: `"probable": 0`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := fstest.MapFS{}
			at := path.Join(sarifVectorsDir, tc.dir)
			if err := fs.WalkDir(spec.Vectors, at, func(name string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				data, err := fs.ReadFile(spec.Vectors, name)
				m[name] = &fstest.MapFile{Data: data}
				return err
			}); err != nil {
				t.Fatalf("Setup: %v", err)
			}
			planted := m[path.Join(at, tc.file)]
			if planted == nil || bytes.Count(planted.Data, []byte(tc.old)) != 1 {
				t.Fatalf("Setup: %s holds no single %q", tc.file, tc.old)
			}
			planted.Data = bytes.Replace(planted.Data, []byte(tc.old), []byte(tc.replacement), 1)
			if len(sarifReproductionProblems(m, tc.dir)) == 0 {
				t.Errorf("sarifReproductionProblems(%s with %s %q) = none, want the departure named", tc.dir, tc.file, tc.replacement)
			}
		})
	}
}

// sarifFamily is the SARIF vectors: each case's report and input reports, and
// the log the render package writes from the report.
func sarifFamily() vectorFamily {
	return vectorFamily{
		name:   "sarif",
		readme: true,
		reports: func(fsys fs.FS, dir string) ([][]byte, error) {
			c, err := loadSARIFCase(fsys, dir)
			if err != nil {
				return nil, err
			}
			return append([][]byte{c.report}, c.inputs...), nil
		},
		problems: func(t *testing.T, fsys fs.FS, dir string) []string {
			t.Helper()
			return sarifReproductionProblems(fsys, dir)
		},
		render: func(t *testing.T, dir string) (string, []byte, bool) {
			t.Helper()
			c, err := loadSARIFCase(os.DirFS("."), dir)
			if err != nil {
				t.Fatalf("loadSARIFCase(%s): %v", dir, err)
			}
			if c.exit != 0 {
				return "", nil, false
			}
			log, exit, err := renderSARIFCase(&c)
			if err != nil || exit != 0 {
				t.Fatalf("renderSARIFCase(%s) = exit %d, %v, want the log", dir, exit, err)
			}
			return sarifExpectedFile, indented(t, log), true
		},
	}
}
