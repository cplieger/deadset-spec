package spec_test

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
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

// templateVectorsDir holds one directory per published template case.
const templateVectorsDir = "vectors/template"

// The files of a template case: the template, the report it renders, and either the
// output it renders or the exit code it ends the run with.
const (
	templateFile         = "template.tmpl"
	templateReportFile   = "report.json"
	templateExpectedFile = "expected.txt"
	templateExitFile     = "expected_exit"
)

// templateCase is one published case, read from its directory.
type templateCase struct {
	source   string
	report   []byte
	expected []byte
	exit     int
}

// renderTemplateCase runs one case and returns the output or the exit code it ends
// with: the usage code for a template refused before any analysis, and the failure
// code for a rendering that fails, which writes nothing.
func renderTemplateCase(c templateCase) ([]byte, int, error) {
	parsed, err := render.ParseTemplate(c.source)
	if err != nil {
		return nil, vocab.ExitUsage, nil
	}
	r, err := report.Decode(c.report)
	if err != nil {
		return nil, 0, err
	}
	var out bytes.Buffer
	if err := parsed.Render(&out, r); err != nil {
		if out.Len() != 0 {
			return nil, 0, fmt.Errorf("the failed rendering wrote %q, want nothing", out.String())
		}
		return nil, vocab.ExitFailure, nil
	}
	return out.Bytes(), 0, nil
}

// loadTemplateCase reads one case, refusing a layout grammar/template.md's vector
// README does not declare.
func loadTemplateCase(fsys fs.FS, dir string) (templateCase, error) {
	at := path.Join(templateVectorsDir, dir)
	entries, err := fs.ReadDir(fsys, at)
	if err != nil {
		return templateCase{}, err
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	var c templateCase
	source, err := fs.ReadFile(fsys, path.Join(at, templateFile))
	if err != nil {
		return templateCase{}, err
	}
	if !utf8.Valid(source) {
		return templateCase{}, fmt.Errorf("%s is not UTF-8", templateFile)
	}
	c.source = string(source)
	if c.report, err = fs.ReadFile(fsys, path.Join(at, templateReportFile)); err != nil {
		return templateCase{}, err
	}
	expected, hasExpected := slices.Contains(names, templateExpectedFile), slices.Contains(names, templateExitFile)
	switch {
	case expected == hasExpected:
		return templateCase{}, fmt.Errorf("files %v, want exactly one of %s and %s", names, templateExpectedFile, templateExitFile)
	case expected:
		c.expected, err = fs.ReadFile(fsys, path.Join(at, templateExpectedFile))
	default:
		var raw []byte
		if raw, err = fs.ReadFile(fsys, path.Join(at, templateExitFile)); err == nil {
			c.exit, err = strconv.Atoi(strings.TrimSuffix(string(raw), "\n"))
		}
		if err == nil && c.exit != vocab.ExitUsage && c.exit != vocab.ExitFailure {
			err = fmt.Errorf("%s = %d, want %d or %d", templateExitFile, c.exit, vocab.ExitUsage, vocab.ExitFailure)
		}
	}
	if err != nil {
		return templateCase{}, err
	}
	if len(names) != 3 {
		return templateCase{}, fmt.Errorf("files %v, want %s, %s and one outcome file", names, templateFile, templateReportFile)
	}
	return c, nil
}

// templateReproductionProblems renders one case and compares the outcome with the
// case's own.
func templateReproductionProblems(fsys fs.FS, dir string) []string {
	c, err := loadTemplateCase(fsys, dir)
	if err != nil {
		return []string{err.Error()}
	}
	got, exit, err := renderTemplateCase(c)
	switch {
	case err != nil:
		return []string{err.Error()}
	case exit != c.exit:
		return []string{fmt.Sprintf("exit = %d, want %d", exit, c.exit)}
	case exit == 0 && !bytes.Equal(got, c.expected):
		return []string{fmt.Sprintf("rendering = %q, want %q", got, c.expected)}
	}
	return nil
}

// TestTemplateVectorsCoverEveryOutcome holds the case set to the three outcomes the
// page states: a rendering, a refusal before analysis and a failed rendering.
func TestTemplateVectorsCoverEveryOutcome(t *testing.T) {
	seen := map[int]int{}
	for _, dir := range vectorCaseDirs(t, spec.Vectors, "template") {
		c, err := loadTemplateCase(spec.Vectors, dir)
		if err != nil {
			t.Fatalf("loadTemplateCase(%s): %v", dir, err)
		}
		seen[c.exit]++
	}
	for _, exit := range []int{0, vocab.ExitUsage, vocab.ExitFailure} {
		if seen[exit] == 0 {
			t.Errorf("cases ending with %d = 0, want at least one", exit)
		}
	}
}

// TestTemplateReproductionRefuses plants one departure from the page per rule into a
// copy of a published case and requires the reproduction to see it.
func TestTemplateReproductionRefuses(t *testing.T) {
	tests := []struct {
		name, dir, file, old, replacement string
	}{
		{name: "a_missing_member_rendered", dir: "member-the-document-lacks", file: templateExitFile, old: "3", replacement: "0"},
		{name: "a_function_the_subset_omits_rendered", dir: "function-outside-the-subset", file: templateExitFile, old: "2", replacement: "3"},
		{name: "an_output_byte_changed", dir: "findings-by-member-name", file: templateExpectedFile, old: "41 deletable", replacement: "42 deletable"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := fstest.MapFS{}
			at := path.Join(templateVectorsDir, tc.dir)
			for _, name := range []string{templateFile, templateReportFile, tc.file} {
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
			if len(templateReproductionProblems(m, tc.dir)) == 0 {
				t.Errorf("templateReproductionProblems(%s with %s %q) = none, want the departure named", tc.dir, tc.file, tc.replacement)
			}
		})
	}
}

// templateFamily is the template vectors: each case's report, rendered through
// its template by the render package.
func templateFamily() vectorFamily {
	return vectorFamily{
		name:   "template",
		readme: true,
		reports: func(fsys fs.FS, dir string) ([][]byte, error) {
			report, err := fs.ReadFile(fsys, path.Join(templateVectorsDir, dir, templateReportFile))
			return [][]byte{report}, err
		},
		problems: func(t *testing.T, fsys fs.FS, dir string) []string {
			t.Helper()
			return templateReproductionProblems(fsys, dir)
		},
		render: func(t *testing.T, dir string) (string, []byte, bool) {
			t.Helper()
			c, err := loadTemplateCase(os.DirFS("."), dir)
			if err != nil {
				t.Fatalf("loadTemplateCase(%s): %v", dir, err)
			}
			got, exit, err := renderTemplateCase(c)
			if err != nil {
				t.Fatalf("renderTemplateCase(%s): %v", dir, err)
			}
			return templateExpectedFile, got, exit == 0
		},
	}
}
