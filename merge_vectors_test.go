package spec_test

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cplieger/deadset-spec"
)

const (
	mergeVectorsDir   = "vectors/merge"
	mergeInputsDir    = "inputs"
	mergeAcceptedFile = "accepted.txt"
	mergeExpectedFile = "expected.json"
	mergeExitFile     = "expected_exit"

	// mergeFailureExit is the code of the `failure` row of contract/exit-codes.json, which the
	// merge returns from its admission step and from an unresolved edge. A case ending there has
	// no merged report.
	mergeFailureExit = 3

	// mergeCleanExit and mergeFindingsExit are the codes of the `clean` and `findings` rows of
	// contract/exit-codes.json, the two verdicts a merge that produced a report returns.
	mergeCleanExit    = 0
	mergeFindingsExit = 1

	mergeStateLive   = "live"
	mergeStateDead   = "dead"
	mergeStateAbsent = "absent"
)

// mergeCaseEntries is every entry a case directory may hold.
var mergeCaseEntries = []string{mergeInputsDir, mergeAcceptedFile, mergeExpectedFile, mergeExitFile}

// mergeVersion matches a schema version wherever it is written, which is how accepted.txt is read
// for the version tokens it names without this file parsing the range syntax that page owns.
var mergeVersion = regexp.MustCompile(`[0-9]+\.[0-9]+\.[0-9]+`)

var (
	errMergeNoAccepted = errors.New("case names no accepted schema range")
	errMergeNoExit     = errors.New("case names no expected exit code")
	errMergeNoInput    = errors.New("case holds no input report")
)

// mergeReport is the part of a report envelope this file reads: what admission turns on, the
// arrays the canonical order and the totals cover, and the edge evaluations the merge resolves.
// The whole envelope is checked against report.schema.json rather than mirrored here.
type mergeReport struct {
	SchemaVersion     string            `json:"schema_version"`
	Analyzer          mergeAnalyzer     `json:"analyzer"`
	Findings          []mergeFinding    `json:"findings"`
	EdgeEvaluations   []mergeEvaluation `json:"edge_evaluations"`
	StaleSuppressions []json.RawMessage `json:"stale_suppressions"`
	Totals            mergeTotals       `json:"totals"`
}

type mergeAnalyzer struct {
	Name        string           `json:"name"`
	Languages   []string         `json:"languages"`
	Conformance mergeConformance `json:"conformance"`
}

type mergeConformance struct {
	Result string `json:"result"`
}

type mergeFinding struct {
	Code     string        `json:"code"`
	Severity string        `json:"severity"`
	Analyzer string        `json:"analyzer"`
	Position mergePosition `json:"position"`
	Symbol   mergeSymbol   `json:"symbol"`
}

type mergePosition struct {
	Path   string `json:"path"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

type mergeSymbol struct {
	Ref string `json:"ref"`
}

type mergeEvaluation struct {
	Edge    string          `json:"edge"`
	Side    string          `json:"side"`
	State   string          `json:"state"`
	Finding json.RawMessage `json:"finding"`
}

type mergeTotals struct {
	BySeverity        map[string]int `json:"by_severity"`
	Findings          int            `json:"findings"`
	StaleSuppressions int            `json:"stale_suppressions"`
	Pending           int            `json:"pending"`
	Omitted           int            `json:"omitted"`
}

// mergeDocument is one report document of a case, kept as bytes so the validator reads the
// document the vector publishes rather than a re-encoding of it.
type mergeDocument struct {
	path string
	data []byte
}

// mergeCase is one decoded case directory of vectors/merge.
type mergeCase struct {
	expected  *mergeReport
	dir       string
	accepted  string
	inputs    []mergeReport
	documents []mergeDocument
	exit      int
}

// mergeShape is one of the shapes the design's case set covers, as a predicate over a decoded
// case: the check turns on what a case holds rather than on what its directory is called.
type mergeShape struct {
	holds func(mergeCase) bool
	name  string
}

// mergeShapes are the ten shapes "Merge test vectors" names, in the order it names them.
var mergeShapes = []mergeShape{
	{
		name:  "one report only",
		holds: func(c mergeCase) bool { return len(c.inputs) == 1 },
	},
	{
		name:  "two reports with no edges",
		holds: func(c mergeCase) bool { return len(c.inputs) >= 2 && len(c.evaluations()) == 0 },
	},
	{
		name: "a pending finding whose pair is live",
		holds: func(c mergeCase) bool {
			return c.anyEdge(func(sides []mergeEvaluation) bool {
				return mergePairedState(sides, mergeStateLive)
			})
		},
	},
	{
		name: "a pending finding whose pair is dead",
		holds: func(c mergeCase) bool {
			return c.anyEdge(func(sides []mergeEvaluation) bool {
				return mergePairedState(sides, mergeStateDead)
			})
		},
	},
	{
		name: "a pending finding whose edge appears in no other report",
		holds: func(c mergeCase) bool {
			return c.anyEdge(func(sides []mergeEvaluation) bool {
				for _, e := range sides {
					if e.State != mergeStateDead {
						continue
					}
					if !slices.ContainsFunc(sides, func(other mergeEvaluation) bool { return other.Side != e.Side }) {
						return true
					}
				}
				return false
			})
		},
	},
	{
		name: "an edge every side reports absent",
		holds: func(c mergeCase) bool {
			return c.anyEdge(func(sides []mergeEvaluation) bool {
				return len(sides) > 0 && !slices.ContainsFunc(sides, func(e mergeEvaluation) bool {
					return e.State != mergeStateAbsent
				})
			})
		},
	},
	{
		name:  "two analyzers claiming one language",
		holds: mergeCase.twoAnalyzersClaimOneLanguage,
	},
	{
		name:  "a schema version outside the accepted range",
		holds: mergeCase.refusedOnSchemaVersion,
	},
	{
		name:  "an analyzer with no conformance pass",
		holds: mergeCase.refusedOnConformance,
	},
	{
		name: "a report holding a stale suppression",
		holds: func(c mergeCase) bool {
			return slices.ContainsFunc(c.inputs, func(r mergeReport) bool { return len(r.StaleSuppressions) > 0 })
		},
	},
}

// mergeFindingKey is the canonical key contract/grammar/merge.md fixes: the finding's path, line
// and column, its code, its symbol reference, and the name of the analyzer whose report carried
// the record, which a merged report keeps on the record and a finding the merge emitted leaves
// empty because no input report carried it.
type mergeFindingKey struct {
	path     string
	code     string
	ref      string
	analyzer string
	line     int
	column   int
}

func mergeKeyOf(f mergeFinding) mergeFindingKey {
	return mergeFindingKey{
		path:     f.Position.Path,
		code:     f.Code,
		ref:      f.Symbol.Ref,
		analyzer: f.Analyzer,
		line:     f.Position.Line,
		column:   f.Position.Column,
	}
}

// compare orders two keys component by component, the first component that differs deciding, with
// every string compared bytewise and every number ascending.
func (k mergeFindingKey) compare(other mergeFindingKey) int {
	return cmp.Or(
		strings.Compare(k.path, other.path),
		cmp.Compare(k.line, other.line),
		cmp.Compare(k.column, other.column),
		strings.Compare(k.code, other.code),
		strings.Compare(k.ref, other.ref),
		strings.Compare(k.analyzer, other.analyzer),
	)
}

func (k mergeFindingKey) String() string {
	return fmt.Sprintf("(%q, %d, %d, %s, %q, %q)", k.path, k.line, k.column, k.code, k.ref, k.analyzer)
}

// evaluations lists every edge evaluation of every input report of the case, in report order.
func (c mergeCase) evaluations() []mergeEvaluation {
	var out []mergeEvaluation
	for _, r := range c.inputs {
		out = append(out, r.EdgeEvaluations...)
	}
	return out
}

// anyEdge reports whether the evaluations of one edge identifier satisfy holds.
func (c mergeCase) anyEdge(holds func(sides []mergeEvaluation) bool) bool {
	byEdge := map[string][]mergeEvaluation{}
	for _, e := range c.evaluations() {
		byEdge[e.Edge] = append(byEdge[e.Edge], e)
	}
	for _, edge := range slices.Sorted(maps.Keys(byEdge)) {
		if holds(byEdge[edge]) {
			return true
		}
	}
	return false
}

// mergePairedState reports whether one side of an edge is dead while another side holds state.
func mergePairedState(sides []mergeEvaluation, state string) bool {
	for _, e := range sides {
		if e.State != mergeStateDead {
			continue
		}
		if slices.ContainsFunc(sides, func(other mergeEvaluation) bool {
			return other.Side != e.Side && other.State == state
		}) {
			return true
		}
	}
	return false
}

// twoAnalyzersClaimOneLanguage reports whether two input reports of the case were written by
// different analyzers that claim a language in common.
func (c mergeCase) twoAnalyzersClaimOneLanguage() bool {
	for i, a := range c.inputs {
		for _, b := range c.inputs[i+1:] {
			if a.Analyzer.Name == b.Analyzer.Name {
				continue
			}
			if slices.ContainsFunc(a.Analyzer.Languages, func(l string) bool {
				return slices.Contains(b.Analyzer.Languages, l)
			}) {
				return true
			}
		}
	}
	return false
}

// refusedOnSchemaVersion reports whether the case is the one admission refuses on the schema
// version: it ends before a merged report exists, every input passed conformance, and one input's
// schema version is not among the versions accepted.txt names.
func (c mergeCase) refusedOnSchemaVersion() bool {
	if c.expected != nil || c.exit != mergeFailureExit {
		return false
	}
	if slices.ContainsFunc(c.inputs, func(r mergeReport) bool { return r.Analyzer.Conformance.Result != "pass" }) {
		return false
	}
	accepted := mergeVersion.FindAllString(c.accepted, -1)
	return slices.ContainsFunc(c.inputs, func(r mergeReport) bool {
		return !slices.Contains(accepted, r.SchemaVersion)
	})
}

// refusedOnConformance reports whether the case is the one admission refuses on the conformance
// block: it ends before a merged report exists and one input's conformance result is not a pass.
func (c mergeCase) refusedOnConformance() bool {
	if c.expected != nil || c.exit != mergeFailureExit {
		return false
	}
	return slices.ContainsFunc(c.inputs, func(r mergeReport) bool { return r.Analyzer.Conformance.Result != "pass" })
}

// mergeCaseDirs lists the case directories under vectors/merge, in name order.
func mergeCaseDirs(t *testing.T) []string {
	t.Helper()
	entries, err := fs.ReadDir(spec.Vectors, mergeVectorsDir)
	if err != nil {
		t.Fatalf("Setup: fs.ReadDir(Vectors, %q): %v, want the published merge vector cases", mergeVectorsDir, err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(dirs) == 0 {
		t.Fatalf("Setup: fs.ReadDir(Vectors, %q) lists no case directory", mergeVectorsDir)
	}
	return dirs
}

// mergeCases loads every published case, failing the run on a case that does not decode.
func mergeCases(t *testing.T) []mergeCase {
	t.Helper()
	dirs := mergeCaseDirs(t)
	cases := make([]mergeCase, 0, len(dirs))
	for _, dir := range dirs {
		c, err := loadMergeCase(spec.Vectors, mergeVectorsDir, dir)
		if err != nil {
			t.Fatalf("Setup: loading %s/%s: %v", mergeVectorsDir, dir, err)
		}
		cases = append(cases, c)
	}
	return cases
}

// mergeReadFile reads one file of a case and reports whether the case holds it. Any error other
// than absence is returned.
func mergeReadFile(fsys fs.FS, root, dir, name string) ([]byte, bool, error) {
	p := root + "/" + dir + "/" + name
	data, err := fs.ReadFile(fsys, p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, false, nil
	case err != nil:
		return nil, false, fmt.Errorf("reading %s: %w", p, err)
	}
	return data, true, nil
}

// loadMergeCase decodes one case directory: its accepted range, its expected exit code, its input
// reports in file-name order and its expected merged report where the case holds one. A missing
// accepted range or exit code leaves the field empty, because mergeLayoutProblems owns which files
// a case holds; a document that does not decode is an error, because nothing can be checked then.
func loadMergeCase(fsys fs.FS, root, dir string) (mergeCase, error) {
	c := mergeCase{dir: dir, exit: -1}
	accepted, held, err := mergeReadFile(fsys, root, dir, mergeAcceptedFile)
	if err != nil {
		return c, err
	}
	if held {
		c.accepted = strings.TrimSpace(string(accepted))
	}
	exit, held, err := mergeReadFile(fsys, root, dir, mergeExitFile)
	if err != nil {
		return c, err
	}
	if held {
		if c.exit, err = strconv.Atoi(strings.TrimSpace(string(exit))); err != nil {
			c.exit = -1
		}
	}
	if err = c.loadInputs(fsys, root); err != nil {
		return c, err
	}
	expected, held, err := mergeReadFile(fsys, root, dir, mergeExpectedFile)
	if err != nil {
		return c, err
	}
	if held {
		var report mergeReport
		if err = json.Unmarshal(expected, &report); err != nil {
			return c, fmt.Errorf("decoding %s: %w", mergeExpectedFile, err)
		}
		c.expected = &report
		c.documents = append(c.documents, mergeDocument{path: mergeExpectedFile, data: expected})
	}
	return c, nil
}

// loadInputs decodes every report under the case's inputs directory, in file-name order.
func (c *mergeCase) loadInputs(fsys fs.FS, root string) error {
	inputs := root + "/" + c.dir + "/" + mergeInputsDir
	entries, err := fs.ReadDir(fsys, inputs)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading %s: %w", inputs, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		data, readErr := fs.ReadFile(fsys, inputs+"/"+name)
		if readErr != nil {
			return fmt.Errorf("reading %s/%s: %w", inputs, name, readErr)
		}
		var report mergeReport
		if err = json.Unmarshal(data, &report); err != nil {
			return fmt.Errorf("decoding %s/%s/%s: %w", mergeInputsDir, c.dir, name, err)
		}
		c.inputs = append(c.inputs, report)
		c.documents = append(c.documents, mergeDocument{path: mergeInputsDir + "/" + name, data: data})
	}
	return nil
}

// mergeLayoutProblems lists every place a case directory departs from the layout "Merge test
// vectors" publishes: an entry the layout does not name, a missing file, an exit code no step of
// contract/grammar/merge.md returns, and an expected report present where the case ends before a
// merged report exists or absent where it does not.
func mergeLayoutProblems(fsys fs.FS, root, dir string) []string {
	var out []string
	entries, err := fs.ReadDir(fsys, root+"/"+dir)
	if err != nil {
		return []string{fmt.Sprintf("reading %s/%s: %v", root, dir, err)}
	}
	for _, entry := range entries {
		if !slices.Contains(mergeCaseEntries, entry.Name()) {
			out = append(out, fmt.Sprintf("holds %q, want only %q", entry.Name(), mergeCaseEntries))
		}
	}
	accepted, held, err := mergeReadFile(fsys, root, dir, mergeAcceptedFile)
	switch {
	case err != nil:
		out = append(out, err.Error())
	case !held || strings.TrimSpace(string(accepted)) == "":
		out = append(out, fmt.Sprintf("%v: want %s naming the schema range the case is merged under", errMergeNoAccepted, mergeAcceptedFile))
	case strings.Count(strings.TrimSpace(string(accepted)), "\n") > 0:
		out = append(out, fmt.Sprintf("%s holds %d lines, want the range on one line", mergeAcceptedFile, strings.Count(strings.TrimSpace(string(accepted)), "\n")+1))
	}
	inputs, err := fs.ReadDir(fsys, root+"/"+dir+"/"+mergeInputsDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		out = append(out, fmt.Sprintf("reading %s/: %v", mergeInputsDir, err))
	}
	for _, entry := range inputs {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			out = append(out, fmt.Sprintf("holds %s/%s, want only the input reports", mergeInputsDir, entry.Name()))
		}
	}
	c, err := loadMergeCase(fsys, root, dir)
	if err != nil {
		return append(out, err.Error())
	}
	if c.exit < 0 {
		out = append(out, fmt.Sprintf("%v: want %s holding one integer of contract/exit-codes.json", errMergeNoExit, mergeExitFile))
	}
	if len(c.inputs) == 0 {
		out = append(out, fmt.Sprintf("%v: want at least one report under %s/", errMergeNoInput, mergeInputsDir))
	}
	return append(out, c.outcomeProblems()...)
}

// outcomeProblems states the one rule that ties a case's exit code to its expected report: the
// merge returns 0 or 1 from its verdict step, which is where a merged report exists, and 3 from
// admission or from an unresolved edge, which is where none does.
func (c mergeCase) outcomeProblems() []string {
	merged := slices.Contains([]int{mergeCleanExit, mergeFindingsExit}, c.exit)
	switch {
	case c.exit < 0:
		return nil
	case !merged && c.exit != mergeFailureExit:
		return []string{fmt.Sprintf("%s = %d, want %d, %d or %d, the codes contract/grammar/merge.md states the merge returns", mergeExitFile, c.exit, mergeCleanExit, mergeFindingsExit, mergeFailureExit)}
	case merged && c.expected == nil:
		return []string{fmt.Sprintf("%s = %d with no %s, want the merged report a verdict code names", mergeExitFile, c.exit, mergeExpectedFile)}
	case !merged && c.expected != nil:
		return []string{fmt.Sprintf("%s = %d with a %s, want no merged report where the merge ends before one exists", mergeExitFile, c.exit, mergeExpectedFile)}
	}
	return nil
}

// mergePendingProblems lists every edge evaluation of a merged report that carries a finding or
// stands at the dead state, which is what step 6 of contract/grammar/merge.md rules out.
func mergePendingProblems(r mergeReport) []string {
	var out []string
	for i, e := range r.EdgeEvaluations {
		if len(e.Finding) == 0 && e.State != mergeStateDead {
			continue
		}
		out = append(out, fmt.Sprintf("edge_evaluations[%d] names edge %q on side %q at state %q carrying a finding = %t, want a merged report to carry neither a dead evaluation nor a pending finding",
			i, e.Edge, e.Side, e.State, len(e.Finding) > 0))
	}
	return out
}

// mergeOrderProblems lists every adjacent pair of findings the canonical key puts the other way
// round, so a report ordered wrongly names the two records rather than the array.
func mergeOrderProblems(r mergeReport) []string {
	var out []string
	for i := 1; i < len(r.Findings); i++ {
		previous, current := mergeKeyOf(r.Findings[i-1]), mergeKeyOf(r.Findings[i])
		if previous.compare(current) > 0 {
			out = append(out, fmt.Sprintf("findings[%d] key %s sorts before findings[%d] key %s, want the canonical key ascending", i, current, i-1, previous))
		}
	}
	return out
}

// mergeTotalsProblems lists every total of a merged report that disagrees with the array it
// counts, which is the cross-check a merge vector needs because another product runs the vector
// and compares bytes.
func mergeTotalsProblems(r mergeReport) []string {
	var out []string
	if want := len(r.Findings) + r.Totals.Omitted; r.Totals.Findings != want {
		out = append(out, fmt.Sprintf("totals.findings = %d, want %d, the %d findings plus the %d omitted", r.Totals.Findings, want, len(r.Findings), r.Totals.Omitted))
	}
	counted := 0
	for _, severity := range slices.Sorted(maps.Keys(r.Totals.BySeverity)) {
		counted += r.Totals.BySeverity[severity]
	}
	if counted != r.Totals.Findings {
		out = append(out, fmt.Sprintf("totals.by_severity sums to %d, want totals.findings = %d", counted, r.Totals.Findings))
	}
	if r.Totals.Omitted == 0 {
		tally := map[string]int{"allow": 0, "warn": 0, "deny": 0}
		for _, f := range r.Findings {
			tally[f.Severity]++
		}
		for _, severity := range slices.Sorted(maps.Keys(tally)) {
			if got := r.Totals.BySeverity[severity]; got != tally[severity] {
				out = append(out, fmt.Sprintf("totals.by_severity[%q] = %d, want %d, the findings carrying it", severity, got, tally[severity]))
			}
		}
	}
	if want := len(r.StaleSuppressions); r.Totals.StaleSuppressions != want {
		out = append(out, fmt.Sprintf("totals.stale_suppressions = %d, want %d, the length of stale_suppressions", r.Totals.StaleSuppressions, want))
	}
	if r.Totals.Pending != 0 {
		out = append(out, fmt.Sprintf("totals.pending = %d, want 0, because a merged report holds no pending finding", r.Totals.Pending))
	}
	return out
}

// missingMergeShapes names every shape no case of the set covers, so a case the set loses fails
// here rather than passing unnoticed.
func missingMergeShapes(cases []mergeCase) []string {
	var out []string
	for _, shape := range mergeShapes {
		if !slices.ContainsFunc(cases, shape.holds) {
			out = append(out, shape.name)
		}
	}
	return out
}

// mergeExitCodes lists the codes contract/exit-codes.json declares.
func mergeExitCodes(t *testing.T) []int {
	t.Helper()
	table := mustLoadExitCodes(t)
	codes := make([]int, 0, len(table.ExitCodes))
	for _, row := range table.ExitCodes {
		codes = append(codes, row.Code)
	}
	return codes
}

func TestMergeVectorCasesHoldTheDeclaredLayout(t *testing.T) {
	for _, dir := range mergeCaseDirs(t) {
		t.Run(dir, func(t *testing.T) {
			for _, problem := range mergeLayoutProblems(spec.Vectors, mergeVectorsDir, dir) {
				t.Errorf("%s/%s %s", mergeVectorsDir, dir, problem)
			}
		})
	}
}

func TestMergeVectorReportsAgreeWithTheReportSchema(t *testing.T) {
	for _, c := range mergeCases(t) {
		t.Run(c.dir, func(t *testing.T) {
			if len(c.documents) == 0 {
				t.Fatalf("Setup: %s/%s holds no report document", mergeVectorsDir, c.dir)
			}
			for _, doc := range c.documents {
				for _, problem := range validationErrors(reportSchemaPath, doc.data) {
					t.Errorf("%s/%s/%s: %s, against %s", mergeVectorsDir, c.dir, doc.path, problem, reportSchemaPath)
				}
			}
		})
	}
}

func TestMergeVectorExpectedReportsCarryNoPendingFinding(t *testing.T) {
	for _, c := range mergeCases(t) {
		if c.expected == nil {
			continue
		}
		t.Run(c.dir, func(t *testing.T) {
			for _, problem := range mergePendingProblems(*c.expected) {
				t.Errorf("%s/%s/%s: %s", mergeVectorsDir, c.dir, mergeExpectedFile, problem)
			}
		})
	}
}

func TestMergeVectorExpectedFindingsAreInCanonicalOrder(t *testing.T) {
	for _, c := range mergeCases(t) {
		if c.expected == nil {
			continue
		}
		t.Run(c.dir, func(t *testing.T) {
			for _, problem := range mergeOrderProblems(*c.expected) {
				t.Errorf("%s/%s/%s: %s", mergeVectorsDir, c.dir, mergeExpectedFile, problem)
			}
		})
	}
}

func TestMergeVectorExpectedExitCodesAreInTheTable(t *testing.T) {
	codes := mergeExitCodes(t)
	for _, c := range mergeCases(t) {
		t.Run(c.dir, func(t *testing.T) {
			if !slices.Contains(codes, c.exit) {
				t.Errorf("%s/%s/%s = %d, want one of %v, the codes %s names", mergeVectorsDir, c.dir, mergeExitFile, c.exit, codes, exitCodesPath)
			}
		})
	}
}

func TestMergeVectorExpectedTotalsAgreeWithTheArraysTheyCount(t *testing.T) {
	for _, c := range mergeCases(t) {
		if c.expected == nil {
			continue
		}
		t.Run(c.dir, func(t *testing.T) {
			for _, problem := range mergeTotalsProblems(*c.expected) {
				t.Errorf("%s/%s/%s: %s", mergeVectorsDir, c.dir, mergeExpectedFile, problem)
			}
		})
	}
}

// TestMergeVectorCasesCoverEveryShapeTheDesignNames checks the case set by what each case holds
// rather than by what it is called, so a renamed directory keeps passing and a lost case fails.
func TestMergeVectorCasesCoverEveryShapeTheDesignNames(t *testing.T) {
	cases := mergeCases(t)
	names := make([]string, 0, len(cases))
	for _, c := range cases {
		names = append(names, c.dir)
	}
	for _, missing := range missingMergeShapes(cases) {
		t.Errorf("%s: no case covers %s, want one; the cases are %q", mergeVectorsDir, missing, names)
	}
}

// mergeCaseProblems runs every check this file makes over one case directory, so a planted case
// exercises the code the published vectors are checked with.
func mergeCaseProblems(fsys fs.FS, root, dir string, codes []int) []string {
	out := mergeLayoutProblems(fsys, root, dir)
	c, err := loadMergeCase(fsys, root, dir)
	if err != nil {
		return append(out, err.Error())
	}
	for _, doc := range c.documents {
		for _, problem := range validationErrors(reportSchemaPath, doc.data) {
			out = append(out, doc.path+": "+problem)
		}
	}
	if !slices.Contains(codes, c.exit) {
		out = append(out, fmt.Sprintf("%s = %d, want one of %v", mergeExitFile, c.exit, codes))
	}
	if c.expected != nil {
		out = append(out, mergePendingProblems(*c.expected)...)
		out = append(out, mergeOrderProblems(*c.expected)...)
		out = append(out, mergeTotalsProblems(*c.expected)...)
	}
	return out
}

func TestMergeCaseChecksAcceptAWellFormedCase(t *testing.T) {
	problems := mergeCaseProblems(plantedMergeCase(), mergeVectorsDir, mergePlantedDir, mergeExitCodes(t))
	for _, problem := range problems {
		t.Errorf("mergeCaseProblems(planted) reports %q, want no problem", problem)
	}
}

func TestMergeCaseChecksRefuse(t *testing.T) {
	cases := []struct {
		mutate  func(t *testing.T, m fstest.MapFS)
		name    string
		wantMsg string
	}{
		{
			name: "expected_report_holds_an_evaluation_with_a_finding",
			mutate: func(t *testing.T, m fstest.MapFS) {
				mergeReplace(t, m, mergeExpectedFile,
					`"state": "live", "analyzer": "deadset-go"`,
					`"state": "dead", "analyzer": "deadset-go", "finding": `+mergePlantedAlphaInput)
			},
			wantMsg: `at state "dead" carrying a finding = true`,
		},
		{
			name: "expected_findings_are_out_of_canonical_order",
			mutate: func(t *testing.T, m fstest.MapFS) {
				mergeReplace(t, m, mergeExpectedFile,
					mergePlantedAlpha+", "+mergePlantedBeta,
					mergePlantedBeta+", "+mergePlantedAlpha)
			},
			wantMsg: "want the canonical key ascending",
		},
		{
			name: "expected_report_names_a_member_the_schema_does_not_declare",
			mutate: func(t *testing.T, m fstest.MapFS) {
				mergeReplace(t, m, mergeExpectedFile, `"schema_version": "1.0.0"`, `"schema_version": "1.0.0", "merged_at": "now"`)
			},
			wantMsg: `expected.json: the whole document: additional properties 'merged_at' not allowed`,
		},
		{
			name: "input_report_omits_a_member_the_schema_requires",
			mutate: func(t *testing.T, m fstest.MapFS) {
				mergeReplace(t, m, mergeInputsDir+"/00-go.json", `"excluded_by_cgo": [], `, "")
			},
			wantMsg: `inputs/00-go.json: the whole document: missing property 'excluded_by_cgo'`,
		},
		{
			name: "input_finding_carries_a_detail_its_code_does_not",
			mutate: func(t *testing.T, m fstest.MapFS) {
				mergeReplace(t, m, mergeInputsDir+"/00-go.json", `"details": {}`, `"details": {"narrower_visibility": "package"}`)
			},
			wantMsg: `inputs/00-go.json: /findings/0/details: 'not' failed`,
		},
		{
			name: "expected_finding_carries_an_exemption_class",
			mutate: func(t *testing.T, m fstest.MapFS) {
				mergeReplace(t, m, mergeExpectedFile, `"retained_by": [], "configurations": ["linux-amd64"]`,
					`"retained_by": ["template-field"], "configurations": ["linux-amd64"]`)
			},
			wantMsg: "expected.json: /findings/0/retained_by: maxItems: got 1, want 0",
		},
		{
			name: "expected_evaluation_is_dead_with_no_finding",
			mutate: func(t *testing.T, m fstest.MapFS) {
				mergeReplace(t, m, mergeExpectedFile, `"state": "live", "analyzer": "deadset-go"`, `"state": "dead", "analyzer": "deadset-go"`)
			},
			wantMsg: `expected.json: /edge_evaluations/0: missing property 'finding'`,
		},
		{
			name: "expected_totals_disagree_with_the_findings_array",
			mutate: func(t *testing.T, m fstest.MapFS) {
				mergeReplace(t, m, mergeExpectedFile, `"findings": 2`, `"findings": 3`)
			},
			wantMsg: "totals.findings = 3, want 2",
		},
		{
			name: "expected_totals_report_a_pending_finding",
			mutate: func(t *testing.T, m fstest.MapFS) {
				mergeReplace(t, m, mergeExpectedFile, `"pending": 0`, `"pending": 1`)
			},
			wantMsg: "totals.pending = 1, want 0",
		},
		{
			name: "exit_code_is_outside_the_table",
			mutate: func(t *testing.T, m fstest.MapFS) {
				mergeReplace(t, m, mergeExitFile, "1", "9")
			},
			wantMsg: "= 9, want one of",
		},
		{
			name: "a_verdict_code_carries_no_merged_report",
			mutate: func(_ *testing.T, m fstest.MapFS) {
				delete(m, mergeVectorsDir+"/"+mergePlantedDir+"/"+mergeExpectedFile)
			},
			wantMsg: "want the merged report a verdict code names",
		},
		{
			name: "case_holds_an_entry_the_layout_does_not_name",
			mutate: func(_ *testing.T, m fstest.MapFS) {
				m[mergeVectorsDir+"/"+mergePlantedDir+"/notes.md"] = &fstest.MapFile{Data: []byte("x\n")}
			},
			wantMsg: `holds "notes.md", want only`,
		},
		{
			name: "inputs_directory_holds_something_other_than_a_report",
			mutate: func(_ *testing.T, m fstest.MapFS) {
				m[mergeVectorsDir+"/"+mergePlantedDir+"/"+mergeInputsDir+"/02-go.txtar"] = &fstest.MapFile{Data: []byte("-- a --\n")}
			},
			wantMsg: "holds inputs/02-go.txtar, want only the input reports",
		},
		{
			name: "case_names_no_accepted_range",
			mutate: func(_ *testing.T, m fstest.MapFS) {
				delete(m, mergeVectorsDir+"/"+mergePlantedDir+"/"+mergeAcceptedFile)
			},
			wantMsg: errMergeNoAccepted.Error(),
		},
	}
	codes := mergeExitCodes(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			planted := plantedMergeCase()
			tc.mutate(t, planted)
			problems := mergeCaseProblems(planted, mergeVectorsDir, mergePlantedDir, codes)
			if !slices.ContainsFunc(problems, func(problem string) bool { return strings.Contains(problem, tc.wantMsg) }) {
				t.Errorf("mergeCaseProblems(planted with %s) = %q, want a problem containing %q", tc.name, problems, tc.wantMsg)
			}
		})
	}
}

// TestMissingMergeShapesNamesEveryUncoveredShape drives the coverage check over a synthetic case
// set, so each of the ten predicates is exercised in both directions before any case exists.
func TestMissingMergeShapesNamesEveryUncoveredShape(t *testing.T) {
	full := mergeShapeCases()
	if missing := missingMergeShapes(full); len(missing) > 0 {
		t.Fatalf("missingMergeShapes(the ten shapes) = %q, want none", missing)
	}
	for i, shape := range mergeShapes {
		t.Run(mergeSubtestName(shape.name), func(t *testing.T) {
			short := slices.Delete(slices.Clone(full), i, i+1)
			missing := missingMergeShapes(short)
			if !slices.Contains(missing, shape.name) {
				t.Errorf("missingMergeShapes(the set without %q) = %q, want it to name that shape", shape.name, missing)
			}
			for _, other := range missing {
				if other != shape.name {
					t.Errorf("missingMergeShapes(the set without %q) also names %q, want one case per shape", shape.name, other)
				}
			}
		})
	}
}

// mergeSubtestName maps a shape's prose name to a name -run can select.
func mergeSubtestName(name string) string {
	return strings.ReplaceAll(name, " ", "_")
}

const (
	mergePlantedDir = "planted"

	// The digests below carry the shape report.schema.json fixes, sha256: and 64 lowercase
	// hexadecimal characters, at the lowest entropy a distinct value can have.
	mergePlantedGoDigest     = "sha256:1111111111111111111111111111111111111111111111111111111111111111"
	mergePlantedTSDigest     = "sha256:2222222222222222222222222222222222222222222222222222222222222222"
	mergePlantedMergedDigest = "sha256:3333333333333333333333333333333333333333333333333333333333333333"

	// mergePlantedTarget is what the three planted reports say about the target they read, which is
	// one target per case and the same target in a merged report.
	mergePlantedTarget = `"target": {"kind": "library", "root": ".", "identity": "example.com/app"}, ` +
		`"configurations": [{"id": "linux-amd64", "os": "linux", "arch": "amd64", "tags": []}], ` +
		`"configurations_not_built": [], ` +
		`"consumers": {"declared": 0, "loaded": [], "unavailable": []}, `

	// mergePlantedAlphaBody and mergePlantedBetaBody are two complete findings, one per language.
	// alpha.go sorts before beta.ts on the canonical key's first component.
	mergePlantedAlphaBody = `{"code": "DS1001", "kind": "unused-exported", "language": "go", ` +
		`"position": {"path": "alpha.go", "line": 3, "column": 6, "end_line": 3}, ` +
		`"symbol": {"ref": "go://example.com/app#Alpha", "kind": "function", "name": "Alpha", "size_lines": 1}, ` +
		`"reachability_class": "certain", "confidence": "certain", "liveness_relation": "reference-counting", ` +
		`"test_only": false, "generated": false, ` +
		`"component": {"id": "deadset-go/c-1", "root": true, "symbol_count": 1, "deletable_lines": 1}, ` +
		`"retained_by": [], "configurations": ["linux-amd64"], "consumers_loaded": [], ` +
		`"fixability": "deletable", "severity": "deny", ` +
		`"message": "exported function Alpha has no reference in the target", "details": {}`
	mergePlantedBetaBody = `{"code": "DS1001", "kind": "unused-exported", "language": "ts", ` +
		`"position": {"path": "beta.ts", "line": 1, "column": 14, "end_line": 1}, ` +
		`"symbol": {"ref": "ts://@example/app/beta.ts#Beta", "kind": "function", "name": "Beta", "size_lines": 1}, ` +
		`"reachability_class": "certain", "confidence": "certain", "liveness_relation": "reference-counting", ` +
		`"test_only": false, "generated": false, ` +
		`"component": {"id": "deadset-ts/c-1", "root": true, "symbol_count": 1, "deletable_lines": 1}, ` +
		`"retained_by": [], "configurations": ["linux-amd64"], "consumers_loaded": [], ` +
		`"fixability": "deletable", "severity": "deny", ` +
		`"message": "exported function Beta has no reference in the target", "details": {}`

	// A report an analyzer writes names its analyzer once, in the envelope; a merged report keeps
	// the name on each record, because the canonical key's last component turns on it.
	mergePlantedAlphaInput = mergePlantedAlphaBody + `}`
	mergePlantedBetaInput  = mergePlantedBetaBody + `}`
	mergePlantedAlpha      = mergePlantedAlphaBody + `, "analyzer": "deadset-go"}`
	mergePlantedBeta       = mergePlantedBetaBody + `, "analyzer": "deadset-ts"}`

	// mergePlantedGoReport and mergePlantedTSReport are the case's two input reports: one finding
	// each, and one live evaluation each of the two sides of one declared edge.
	mergePlantedGoReport = `{"schema_version": "1.0.0", "contract_version": "1.0.0", ` +
		`"analyzer": {"name": "deadset-go", "version": "1.0.0", "languages": ["go"], "schema_versions_accepted": ["1.0.0"], ` +
		`"conformance": {"corpus_version": "1.0.0", "result": "pass", "digest": "` + mergePlantedGoDigest + `"}}, ` +
		mergePlantedTarget +
		`"findings": [` + mergePlantedAlphaInput + `], ` +
		`"edge_evaluations": [{"edge": "wire/ServerEvent", "side": "provides", "symbol": "go://example.com/app#ServerEvent", "state": "live"}], ` +
		`"stale_suppressions": [], "declared_gaps": [], "excluded_by_cgo": [], ` +
		`"test_file_rules": [{"rule": "go-test-file", "matched": 0}], ` +
		`"totals": {"findings": 1, "by_severity": {"allow": 0, "warn": 0, "deny": 1}, "deletable_lines": 1, ` +
		`"suppressions_in_effect": 0, "reasons_recorded": 0, "stale_suppressions": 0, "pending": 0, "omitted": 0}}` + "\n"
	mergePlantedTSReport = `{"schema_version": "1.0.0", "contract_version": "1.0.0", ` +
		`"analyzer": {"name": "deadset-ts", "version": "1.0.0", "languages": ["ts"], "schema_versions_accepted": ["1.0.0"], ` +
		`"conformance": {"corpus_version": "1.0.0", "result": "pass", "digest": "` + mergePlantedTSDigest + `"}}, ` +
		mergePlantedTarget +
		`"findings": [` + mergePlantedBetaInput + `], ` +
		`"edge_evaluations": [{"edge": "wire/ServerEvent", "side": "used_by", "symbol": "ts://@example/app/beta.ts#ServerEvent", "state": "live"}], ` +
		`"stale_suppressions": [], "declared_gaps": [], "excluded_by_cgo": [], ` +
		`"test_file_rules": [{"rule": "ts-test-pattern", "matched": 0}], ` +
		`"totals": {"findings": 1, "by_severity": {"allow": 0, "warn": 0, "deny": 1}, "deletable_lines": 1, ` +
		`"suppressions_in_effect": 0, "reasons_recorded": 0, "stale_suppressions": 0, "pending": 0, "omitted": 0}}` + "\n"

	// mergePlantedMergedReport is what the merge returns over those two reports: both findings in
	// canonical order, both live evaluations ordered by edge and then by side, and recomputed
	// totals. Two deny findings, so the verdict step returns the findings code.
	mergePlantedMergedReport = `{"schema_version": "1.0.0", "contract_version": "1.0.0", ` +
		`"analyzer": {"name": "deadset", "version": "1.0.0", "languages": ["go", "ts"], "schema_versions_accepted": ["1.0.0"], ` +
		`"conformance": {"corpus_version": "1.0.0", "result": "pass", "digest": "` + mergePlantedMergedDigest + `"}}, ` +
		`"merged_from": [{"name": "deadset-go", "version": "1.0.0", "digest": "` + mergePlantedGoDigest + `"}, ` +
		`{"name": "deadset-ts", "version": "1.0.0", "digest": "` + mergePlantedTSDigest + `"}], ` +
		mergePlantedTarget +
		`"findings": [` + mergePlantedAlpha + `, ` + mergePlantedBeta + `], ` +
		`"edge_evaluations": [` +
		`{"edge": "wire/ServerEvent", "side": "provides", "symbol": "go://example.com/app#ServerEvent", "state": "live", "analyzer": "deadset-go"}, ` +
		`{"edge": "wire/ServerEvent", "side": "used_by", "symbol": "ts://@example/app/beta.ts#ServerEvent", "state": "live", "analyzer": "deadset-ts"}], ` +
		`"stale_suppressions": [], "declared_gaps": [], "excluded_by_cgo": [], ` +
		`"test_file_rules": [{"rule": "go-test-file", "matched": 0}, {"rule": "ts-test-pattern", "matched": 0}], ` +
		`"totals": {"findings": 2, "by_severity": {"allow": 0, "warn": 0, "deny": 2}, "deletable_lines": 2, ` +
		`"suppressions_in_effect": 0, "reasons_recorded": 0, "stale_suppressions": 0, "pending": 0, "omitted": 0}}` + "\n"
)

// plantedMergeCase is one complete case in memory, laid out as vectors/merge lays a case out on
// disk, so every check runs against a well-formed case before the first one lands and keeps
// running against the same code afterwards.
func plantedMergeCase() fstest.MapFS {
	dir := mergeVectorsDir + "/" + mergePlantedDir
	return fstest.MapFS{
		dir + "/" + mergeAcceptedFile:              {Data: []byte("1.0.0\n")},
		dir + "/" + mergeExitFile:                  {Data: []byte("1\n")},
		dir + "/" + mergeInputsDir + "/00-go.json": {Data: []byte(mergePlantedGoReport)},
		dir + "/" + mergeInputsDir + "/01-ts.json": {Data: []byte(mergePlantedTSReport)},
		dir + "/" + mergeExpectedFile:              {Data: []byte(mergePlantedMergedReport)},
	}
}

// mergeReplace plants one violation in one file of a planted case, failing the test when the text
// it replaces is absent, so a fixture the documents outgrew is a failure here rather than a check
// that reports nothing.
func mergeReplace(t *testing.T, m fstest.MapFS, name, old, replacement string) {
	t.Helper()
	p := mergeVectorsDir + "/" + mergePlantedDir + "/" + name
	file, held := m[p]
	if !held {
		t.Fatalf("Setup: the planted case holds no %s", p)
	}
	if !bytes.Contains(file.Data, []byte(old)) {
		t.Fatalf("Setup: %s does not hold %q, want the text the mutation replaces", p, old)
	}
	file.Data = bytes.Replace(file.Data, []byte(old), []byte(replacement), 1)
}

// mergeShapeCases is one synthetic case per shape of mergeShapes, in that order, each holding its
// own shape and no other, so the coverage check is driven in both directions.
func mergeShapeCases() []mergeCase {
	merged := &mergeReport{}
	goReport := func(evaluations ...mergeEvaluation) mergeReport {
		return mergeReport{
			SchemaVersion:   "1.0.0",
			Analyzer:        mergeAnalyzer{Name: "deadset-go", Languages: []string{"go"}, Conformance: mergeConformance{Result: "pass"}},
			EdgeEvaluations: evaluations,
		}
	}
	tsReport := func(evaluations ...mergeEvaluation) mergeReport {
		return mergeReport{
			SchemaVersion:   "1.0.0",
			Analyzer:        mergeAnalyzer{Name: "deadset-ts", Languages: []string{"ts"}, Conformance: mergeConformance{Result: "pass"}},
			EdgeEvaluations: evaluations,
		}
	}
	evaluation := func(side, state string) mergeEvaluation {
		e := mergeEvaluation{Edge: "wire/ServerEvent", Side: side, State: state}
		if state == mergeStateDead {
			e.Finding = json.RawMessage(`{}`)
		}
		return e
	}
	live := evaluation("provides", mergeStateLive)
	staleReport := tsReport(evaluation("used_by", mergeStateLive))
	staleReport.StaleSuppressions = []json.RawMessage{json.RawMessage(`{}`)}
	otherEdge := mergeEvaluation{Edge: "wire/Other", Side: "used_by", State: mergeStateLive}
	sameLanguage := mergeReport{
		SchemaVersion: "1.0.0",
		Analyzer:      mergeAnalyzer{Name: "other-go", Languages: []string{"go"}, Conformance: mergeConformance{Result: "pass"}},
	}
	newerReport := goReport()
	newerReport.SchemaVersion = "2.0.0"
	failedReport := tsReport()
	failedReport.Analyzer.Conformance.Result = "fail"

	return []mergeCase{
		{dir: "one-report", accepted: "1.0.0", exit: mergeCleanExit, expected: merged, inputs: []mergeReport{goReport()}},
		{dir: "no-edges", accepted: "1.0.0", exit: mergeCleanExit, expected: merged, inputs: []mergeReport{goReport(), tsReport()}},
		{dir: "pair-live", accepted: "1.0.0", exit: mergeCleanExit, expected: merged, inputs: []mergeReport{
			goReport(evaluation("provides", mergeStateDead)), tsReport(evaluation("used_by", mergeStateLive)),
		}},
		{dir: "pair-dead", accepted: "1.0.0", exit: mergeFindingsExit, expected: merged, inputs: []mergeReport{
			goReport(evaluation("provides", mergeStateDead)), tsReport(evaluation("used_by", mergeStateDead)),
		}},
		{dir: "unresolved-edge", accepted: "1.0.0", exit: mergeFailureExit, inputs: []mergeReport{
			goReport(evaluation("provides", mergeStateDead)), tsReport(otherEdge),
		}},
		{dir: "every-side-absent", accepted: "1.0.0", exit: mergeFindingsExit, expected: merged, inputs: []mergeReport{
			goReport(evaluation("provides", mergeStateAbsent)), tsReport(evaluation("used_by", mergeStateAbsent)),
		}},
		{dir: "two-analyzers-one-language", accepted: "1.0.0", exit: mergeCleanExit, expected: merged, inputs: []mergeReport{
			goReport(live), sameLanguage,
		}},
		{dir: "version-outside-range", accepted: "1.0.0", exit: mergeFailureExit, inputs: []mergeReport{
			newerReport, tsReport(live),
		}},
		{dir: "no-conformance-pass", accepted: "1.0.0", exit: mergeFailureExit, inputs: []mergeReport{
			goReport(live), failedReport,
		}},
		{dir: "stale-suppression", accepted: "1.0.0", exit: mergeFindingsExit, expected: merged, inputs: []mergeReport{
			goReport(live), staleReport,
		}},
	}
}
