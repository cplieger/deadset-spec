package spec_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/facts"
	"github.com/cplieger/deadset-spec/v8/report"
	"github.com/cplieger/deadset-spec/v8/vocab"
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
var mergeCaseEntries = []string{mergeInputsDir, mergeAcceptedFile, mergeCallerFile, mergeExpectedFile, mergeExitFile}

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
	Findings          []mergeFinding    `json:"findings"`
	EdgeEvaluations   []mergeEvaluation `json:"edge_evaluations"`
	StaleSuppressions []json.RawMessage `json:"stale_suppressions"`
	Totals            mergeTotals       `json:"totals"`
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

// mergeCase is one decoded case directory of vectors/merge. digests[i] is the digest caller.json
// gives inputs[i], or "" where it gives none.
type mergeCase struct {
	expected  *mergeReport
	dir       string
	accepted  string
	inputs    []mergeReport
	digests   []string
	documents []mergeDocument
	facts     []mergeDocument
	exit      int
}

// mergeKeyOf is the part of one finding the canonical order contract/grammar/merge.md
// fixes reads.
func mergeKeyOf(f mergeFinding) report.Finding {
	return report.Finding{
		Position: report.Position{Path: f.Position.Path, Line: f.Position.Line, Column: f.Position.Column},
		Code:     vocab.Code(f.Code),
		Symbol:   report.Symbol{Ref: f.Symbol.Ref},
		Analyzer: f.Analyzer,
	}
}

// mergeKeyText spells one finding's canonical key for a failure message.
func mergeKeyText(f mergeFinding) string {
	return fmt.Sprintf("(%q, %d, %d, %s, %q, %q)", f.Position.Path, f.Position.Line, f.Position.Column, f.Code, f.Symbol.Ref, f.Analyzer)
}

// mergeCases loads every published case, failing the run on a case that does not decode.
func mergeCases(t *testing.T) []mergeCase {
	t.Helper()
	dirs := vectorCaseDirs(t, spec.Vectors, "merge")
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
// reports in file-name order with the digest caller.json gives each, and its expected merged
// report where the case holds one. A missing accepted range, exit code or caller.json leaves the
// field empty, because mergeLayoutProblems owns which files a case holds; a document that does
// not decode is an error, because nothing can be checked then.
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
	callerFacts, held, err := mergeReadFile(fsys, root, dir, mergeCallerFile)
	if err != nil {
		return c, err
	}
	var caller mergeCaller
	if held {
		if err = json.Unmarshal(callerFacts, &caller); err != nil {
			return c, fmt.Errorf("decoding %s: %w", mergeCallerFile, err)
		}
	}
	if err = c.loadInputs(fsys, root, caller.Digests); err != nil {
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

// loadInputs decodes every report under the case's inputs directory, in file-name order, each
// with its digest from digests, which is keyed by file name.
func (c *mergeCase) loadInputs(fsys fs.FS, root string, digests map[string]string) error {
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
		if strings.HasSuffix(name, factsSuffix) {
			c.facts = append(c.facts, mergeDocument{path: mergeInputsDir + "/" + name, data: data})
			continue
		}
		var report mergeReport
		if err = json.Unmarshal(data, &report); err != nil {
			return fmt.Errorf("decoding %s/%s/%s: %w", mergeInputsDir, c.dir, name, err)
		}
		c.inputs = append(c.inputs, report)
		c.digests = append(c.digests, digests[name])
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
	if _, held, err := mergeReadFile(fsys, root, dir, mergeCallerFile); err != nil || !held {
		out = append(out, fmt.Sprintf("holds no %s (%v), want the facts the caller supplies", mergeCallerFile, err))
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
		if report.CompareFindings(&previous, &current) > 0 {
			out = append(out, fmt.Sprintf("findings[%d] key %s sorts before findings[%d] key %s, want the canonical key ascending",
				i, mergeKeyText(r.Findings[i]), i-1, mergeKeyText(r.Findings[i-1])))
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
	for _, dir := range vectorCaseDirs(t, spec.Vectors, "merge") {
		t.Run(dir, func(t *testing.T) {
			for _, problem := range mergeLayoutProblems(spec.Vectors, mergeVectorsDir, dir) {
				t.Errorf("%s/%s %s", mergeVectorsDir, dir, problem)
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

func TestMergeVectorFactsInputsAreFactsDocuments(t *testing.T) {
	for _, c := range mergeCases(t) {
		for _, doc := range c.facts {
			if _, err := facts.Decode(doc.data); err != nil {
				t.Errorf("%s/%s/%s: %v, want a facts document the facts schema and its rules admit", mergeVectorsDir, c.dir, doc.path, err)
			}
		}
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

// mergeTracePoints are the branches of the merge the case set reaches, as the reference merge
// traces them: a case the set loses leaves a point unreached, and a branch a new case reaches
// joins this list.
var mergeTracePoints = []string{
	traceOneReport,
	traceNoEdges,
	traceWithheldSummed,
	traceStaleSuppression,
	traceSchemaVersion,
	traceConformance,
	traceOmitted,
	traceTargets,
	traceMixedNotLanguages,
	traceTwoKindsOneLanguage,
	traceTwoKindsMixed,
	traceTwoAnalyzersOneLanguage,
	traceOneArtifact,
	traceOneNameTwoVersions,
	traceUnresolvedEdge,
	tracePromotedDeadPair,
	traceFallsWithPair + mergeStateLive,
	traceFallsWithPair + mergeStateAbsent,
	traceDroppedBesideDeadPair + mergeStateLive,
	traceDroppedBesideDeadPair + mergeStateAbsent,
	traceEdgeAbsent,
	traceRootMatchedElsewhere,
	tracePromotedOnce,
	traceJoinedConfidence,
	"two configurations entries whose identity members differ",
	"two consumers.loaded entries whose identity members differ",
	"a configurations entry and a configurations_not_built entry",
	"a consumers.loaded entry and a consumers.unavailable entry",
	traceFreeTextMerged + "configurations_not_built",
	traceFreeTextMerged + "consumers.unavailable",
	traceTieAgainstReadOrder + "test_file_rules",
	traceFactsAnalyzer,
	traceFactsLanguage,
	traceFactsFinding,
	traceKindNotClaimed,
	traceKindlessTarget,
	traceBindingNeutral,
	traceBindingLive,
	traceBindingStateless,
	traceBindingUser,
	traceBindingMember,
	traceBindingPromoted,
	traceBindingCap, traceBindingCapCertain, traceBindingLibrary,
	traceViaUnreferenced,
	traceImplementerNoSchema,
	traceImplementerVia,
	traceImplementerSchema,
	traceImplementerPromoted,
	traceMemberFolds,
	traceCarriedShared,
	traceBindingAndEdge,
	traceUnboundNote,
	traceUnsupportedNote,
	traceCarriedOneComponent,
	traceBindingWithheld,
	traceWithheldKeptLive,
	traceBlockedAcross,
	traceBlockedDebited,
}

// TestMergeVectorCasesReachEveryTracedBranch runs the reference merge over every published case
// and holds the branches it takes, together, to mergeTracePoints.
func TestMergeVectorCasesReachEveryTracedBranch(t *testing.T) {
	entries, err := fs.ReadDir(spec.Vectors, mergeVectorsDir)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	trace := mergeTrace{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		inputs, accepted, caller, err := loadReferenceCase(spec.Vectors, mergeVectorsDir, entry.Name())
		if err != nil {
			t.Fatalf("Setup: loadReferenceCase(%s): %v", entry.Name(), err)
		}
		_, _, _ = referenceMerge(inputs, accepted, caller, kindsByCode(t), trace)
	}
	reached := slices.Sorted(maps.Keys(trace))
	if want := slices.Sorted(slices.Values(mergeTracePoints)); !slices.Equal(reached, want) {
		t.Errorf("%s reaches %q, want mergeTracePoints %q", mergeVectorsDir, reached, want)
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
	for _, doc := range c.facts {
		if _, err := facts.Decode(doc.data); err != nil {
			out = append(out, doc.path+": "+err.Error())
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
				mergeReplace(t, m, mergeExpectedFile, `"pending": 0, "omitted": 0, "withheld": {"certain": 0, "probable": 0, "possible": 0}}}`+"\n", `"pending": 1, "omitted": 0, "withheld": {"certain": 0, "probable": 0, "possible": 0}}}`+"\n")
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
			name: "case_holds_no_caller_facts",
			mutate: func(_ *testing.T, m fstest.MapFS) {
				delete(m, mergeVectorsDir+"/"+mergePlantedDir+"/"+mergeCallerFile)
			},
			wantMsg: "holds no caller.json",
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
		`"type_error_skips": [], "notes": [], "unanswered_questions": [], "conventions_applied": [], ` +
		`"totals": {"findings": 1, "by_severity": {"allow": 0, "warn": 0, "deny": 1}, "deletable_lines": 1, ` +
		`"suppressions_in_effect": 0, "reasons_recorded": 0, "stale_suppressions": 0, "pending": 0, "omitted": 0, "withheld": {"certain": 0, "probable": 0, "possible": 0}}}` + "\n"
	mergePlantedTSReport = `{"schema_version": "1.0.0", "contract_version": "1.0.0", ` +
		`"analyzer": {"name": "deadset-ts", "version": "1.0.0", "languages": ["ts"], "schema_versions_accepted": ["1.0.0"], ` +
		`"conformance": {"corpus_version": "1.0.0", "result": "pass", "digest": "` + mergePlantedTSDigest + `"}}, ` +
		mergePlantedTarget +
		`"findings": [` + mergePlantedBetaInput + `], ` +
		`"edge_evaluations": [{"edge": "wire/ServerEvent", "side": "used_by", "symbol": "ts://@example/app/beta.ts#ServerEvent", "state": "live"}], ` +
		`"stale_suppressions": [], "declared_gaps": [], "excluded_by_cgo": [], ` +
		`"test_file_rules": [{"rule": "ts-test-pattern", "matched": 0}], ` +
		`"type_error_skips": [], "notes": [], "unanswered_questions": [], "conventions_applied": [], ` +
		`"totals": {"findings": 1, "by_severity": {"allow": 0, "warn": 0, "deny": 1}, "deletable_lines": 1, ` +
		`"suppressions_in_effect": 0, "reasons_recorded": 0, "stale_suppressions": 0, "pending": 0, "omitted": 0, "withheld": {"certain": 0, "probable": 0, "possible": 0}}}` + "\n"

	// mergePlantedMergedReport is what the merge returns over those two reports: both findings in
	// canonical order, both live evaluations ordered by edge and then by side, and recomputed
	// totals. Two deny findings, so the verdict step returns the findings code.
	mergePlantedMergedReport = `{"schema_version": "1.0.0", "contract_version": "1.0.0", ` +
		`"analyzer": {"name": "deadset", "version": "1.0.0", "languages": ["go", "ts"], "schema_versions_accepted": ["1.0.0"], ` +
		`"conformance": {"corpus_version": "1.0.0", "result": "pass", "digest": "` + mergePlantedMergedDigest + `"}}, ` +
		`"merged_from": [{"name": "deadset-go", "version": "1.0.0", "digest": "` + mergePlantedGoDigest + `", "languages": ["go"], ` + mergePlantedInputTotals + `}, ` +
		`{"name": "deadset-ts", "version": "1.0.0", "digest": "` + mergePlantedTSDigest + `", "languages": ["ts"], ` + mergePlantedInputTotals + `}], ` +
		mergePlantedTarget +
		`"findings": [` + mergePlantedAlpha + `, ` + mergePlantedBeta + `], ` +
		`"edge_evaluations": [` +
		`{"edge": "wire/ServerEvent", "side": "provides", "symbol": "go://example.com/app#ServerEvent", "state": "live", "analyzer": "deadset-go"}, ` +
		`{"edge": "wire/ServerEvent", "side": "used_by", "symbol": "ts://@example/app/beta.ts#ServerEvent", "state": "live", "analyzer": "deadset-ts"}], ` +
		`"stale_suppressions": [], "declared_gaps": [], "excluded_by_cgo": [], ` +
		`"test_file_rules": [{"rule": "go-test-file", "matched": 0}, {"rule": "ts-test-pattern", "matched": 0}], ` +
		`"type_error_skips": [], "notes": [], "unanswered_questions": [], "conventions_applied": [], ` +
		`"totals": {"findings": 2, "by_severity": {"allow": 0, "warn": 0, "deny": 2}, "deletable_lines": 2, ` +
		`"suppressions_in_effect": 0, "reasons_recorded": 0, "stale_suppressions": 0, "pending": 0, "omitted": 0, "withheld": {"certain": 0, "probable": 0, "possible": 0}}}` + "\n"
)

// mergePlantedInputTotals is the totals each planted input writes, which its
// merged_from entry carries.
const mergePlantedInputTotals = `"totals": {"findings": 1, "by_severity": {"allow": 0, "warn": 0, "deny": 1}, "deletable_lines": 1, "suppressions_in_effect": 0, "reasons_recorded": 0, "stale_suppressions": 0, "pending": 0, "omitted": 0, "withheld": {"certain": 0, "probable": 0, "possible": 0}}`

// mergePlantedCaller is the planted case's caller.json: the merging product the planted merged
// report names, and one digest per planted input.
const mergePlantedCaller = `{"schema_version": "1.0.0", "contract_version": "1.0.0", ` +
	`"analyzer": {"name": "deadset", "version": "1.0.0", "conformance": {"corpus_version": "1.0.0", "result": "pass", "digest": "` + mergePlantedMergedDigest + `"}}, ` +
	`"digests": {"00-go.json": "` + mergePlantedGoDigest + `", "01-ts.json": "` + mergePlantedTSDigest + `"}, "fail_on": "deny"}` + "\n"

// plantedMergeCase is one complete case in memory, laid out as vectors/merge lays a case out on
// disk, so every check runs against a well-formed case before the first one lands and keeps
// running against the same code afterwards.
func plantedMergeCase() fstest.MapFS {
	dir := mergeVectorsDir + "/" + mergePlantedDir
	return fstest.MapFS{
		dir + "/" + mergeAcceptedFile:              {Data: []byte("1.0.0\n")},
		dir + "/" + mergeExitFile:                  {Data: []byte("1\n")},
		dir + "/" + mergeCallerFile:                {Data: []byte(mergePlantedCaller)},
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

// mergeFamily is the merge vectors: every input and expected report a case
// holds, reproduced by the reference merge.
func mergeFamily() vectorFamily {
	return vectorFamily{
		name:   "merge",
		readme: true,
		reports: func(fsys fs.FS, dir string) ([][]byte, error) {
			c, err := loadMergeCase(fsys, mergeVectorsDir, dir)
			if err != nil {
				return nil, err
			}
			documents := make([][]byte, len(c.documents))
			for i := range c.documents {
				documents[i] = c.documents[i].data
			}
			return documents, nil
		},
		problems: func(t *testing.T, fsys fs.FS, dir string) []string {
			t.Helper()
			return mergeReproductionProblems(fsys, mergeVectorsDir, dir, kindsByCode(t), nil)
		},
		render: func(t *testing.T, dir string) (string, []byte, bool) {
			t.Helper()
			inputs, accepted, caller, err := loadReferenceCase(os.DirFS("."), mergeVectorsDir, dir)
			if err != nil {
				t.Fatalf("loadReferenceCase(%s): %v", dir, err)
			}
			report, _, _ := referenceMerge(inputs, accepted, caller, kindsByCode(t), nil)
			if report == nil {
				return "", nil, false
			}
			data, err := report.render()
			if err != nil {
				t.Fatalf("render %s: %v", dir, err)
			}
			return mergeExpectedFile, data, true
		},
	}
}

// The blocked-carrier and shared-carrier line debits count lines from position spans, so a case
// whose position disagrees with its symbol.size_lines tests them on a finding finding.schema.json forbids.
func TestMergeVectorFindingsSpanTheirSizeLines(t *testing.T) {
	names, err := fs.Glob(spec.Vectors, mergeVectorsDir+"/*/*.json")
	if err != nil {
		t.Fatalf("Setup: glob: %v", err)
	}
	inputs, err := fs.Glob(spec.Vectors, mergeVectorsDir+"/*/inputs/*.json")
	if err != nil {
		t.Fatalf("Setup: glob: %v", err)
	}
	for _, name := range append(names, inputs...) {
		data, err := fs.ReadFile(spec.Vectors, name)
		if err != nil {
			t.Fatalf("Setup: read %s: %v", name, err)
		}
		var doc any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("Setup: decode %s: %v", name, err)
		}
		for _, problem := range sizeLinesProblems(doc) {
			t.Errorf("%s: %s", name, problem)
		}
	}
}

// sizeLinesProblems names every finding under v whose position.end_line is not position.line
// plus symbol.size_lines minus one.
func sizeLinesProblems(v any) []string {
	var problems []string
	switch v := v.(type) {
	case map[string]any:
		position, _ := v["position"].(map[string]any)
		symbol, _ := v["symbol"].(map[string]any)
		if size, ok := symbol["size_lines"].(float64); ok && position != nil {
			line, end := position["line"].(float64), position["end_line"].(float64)
			if end != line+size-1 {
				problems = append(problems, fmt.Sprintf("finding %v spans lines %v to %v, want size_lines %v", symbol["ref"], line, end, size))
			}
		}
		for _, member := range v {
			problems = append(problems, sizeLinesProblems(member)...)
		}
	case []any:
		for _, item := range v {
			problems = append(problems, sizeLinesProblems(item)...)
		}
	}
	return problems
}
