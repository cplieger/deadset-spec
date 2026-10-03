package spec_test

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cplieger/deadset-spec/v5"
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
	Target                 json.RawMessage   `json:"target"`
	SchemaVersion          string            `json:"schema_version"`
	Analyzer               mergeAnalyzer     `json:"analyzer"`
	Configurations         []json.RawMessage `json:"configurations"`
	ConfigurationsNotBuilt []json.RawMessage `json:"configurations_not_built"`
	Consumers              mergeConsumers    `json:"consumers"`
	Findings               []mergeFinding    `json:"findings"`
	EdgeEvaluations        []mergeEvaluation `json:"edge_evaluations"`
	StaleSuppressions      []json.RawMessage `json:"stale_suppressions"`
	TestFileRules          []mergeRule       `json:"test_file_rules"`
	Totals                 mergeTotals       `json:"totals"`
}

// mergeRule is one test_file_rules entry, its fields in report.schema.json's order, so its JSON
// encoding is the entry's compact encoding.
type mergeRule struct {
	Rule    string `json:"rule"`
	Matched int    `json:"matched"`
}

type mergeConsumers struct {
	Loaded      []json.RawMessage `json:"loaded"`
	Unavailable []json.RawMessage `json:"unavailable"`
}

type mergeAnalyzer struct {
	Name        string           `json:"name"`
	Version     string           `json:"version"`
	Languages   []string         `json:"languages"`
	Conformance mergeConformance `json:"conformance"`
}

type mergeConformance struct {
	Result string `json:"result"`
}

type mergeFinding struct {
	Code      string         `json:"code"`
	Severity  string         `json:"severity"`
	Analyzer  string         `json:"analyzer"`
	Position  mergePosition  `json:"position"`
	Symbol    mergeSymbol    `json:"symbol"`
	Component mergeComponent `json:"component"`
}

type mergeComponent struct {
	ID string `json:"id"`
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
	exit      int
}

// mergeShape is one of the shapes the case set covers, as a predicate over a decoded case: the
// check turns on what a case holds rather than on what its directory is called.
type mergeShape struct {
	holds func(mergeCase) bool
	name  string
}

// mergeShapes are the shapes the case set covers. Each predicate holds what makes its case a pin,
// so a case stripped of it fails the coverage check even where its expected bytes stay the same.
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
		name:  "a pending finding whose pair is live and a finding that falls with it",
		holds: func(c mergeCase) bool { return c.carriesFallenMember(mergeStateLive) },
	},
	{
		name:  "a pending finding whose pair is dead promoted into the merged report",
		holds: mergeCase.promotesPairedDead,
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
	{
		name:  "a pending finding whose pair is absent and a finding that falls with it",
		holds: func(c mergeCase) bool { return c.carriesFallenMember(mergeStateAbsent) },
	},
	{
		name:  "a component pending on one edge whose pair is live and on another whose pair is dead",
		holds: func(c mergeCase) bool { return c.pendsBesideADeadPair(mergeStateLive) },
	},
	{
		name:  "a component pending on one edge whose pair is absent and on another whose pair is dead",
		holds: func(c mergeCase) bool { return c.pendsBesideADeadPair(mergeStateAbsent) },
	},
	{
		name:  "two reports naming different targets",
		holds: mergeCase.refusedOnTargets,
	},
	{
		name:  "a report that omitted a finding",
		holds: mergeCase.refusedOnOmitted,
	},
	{
		name:  "two configurations entries under one id whose identity members differ",
		holds: func(c mergeCase) bool { return c.refusedOnIdentity(mergeConfigurations) },
	},
	{
		name:  "two configurations_not_built entries under one id whose errors alone differ, merged with the first report's error",
		holds: func(c mergeCase) bool { return c.mergesFreeText(mergeNotBuilt) },
	},
	{
		name:  "a configuration one report built and another could not build",
		holds: func(c mergeCase) bool { return c.refusedAcross(mergeConfigurations, mergeNotBuilt) },
	},
	{
		name:  "two consumers.loaded entries under one id whose identity members differ",
		holds: func(c mergeCase) bool { return c.refusedOnIdentity(mergeLoaded) },
	},
	{
		name:  "two consumers.unavailable entries under one id whose reasons alone differ, merged with the first report's reason",
		holds: func(c mergeCase) bool { return c.mergesFreeText(mergeUnavailable) },
	},
	{
		name:  "a consumer one report loaded and another lists as unavailable",
		holds: func(c mergeCase) bool { return c.refusedAcross(mergeLoaded, mergeUnavailable) },
	},
	{
		name:  "two reports one analyzer artifact wrote",
		holds: mergeCase.refusedOnOneArtifact,
	},
	{
		name:  mergeOneNameShape,
		holds: mergeCase.refusedOnOneNameTwice,
	},
	{
		name:  mergeOneRuleShape,
		holds: mergeCase.mergesOneRuleCountedTwoWays,
	},
}

const (
	mergeOneNameShape = "two reports one analyzer name wrote at two versions from two artifacts"
	mergeOneRuleShape = "two reports counting one test-file rule two ways, merged with both entries, " +
		"the first holding the lesser count, whose encoding sorts second"
)

// mergeEntryArray is one id-keyed array of a report, with the member of its entries that is free
// text rather than identity, where its entries carry one.
type mergeEntryArray struct {
	entries func(mergeReport) []json.RawMessage
	free    string
}

// identity decodes an entry without its free-text member, which is what two entries under one id
// must agree on. An entry that does not decode reads as nil.
func (a mergeEntryArray) identity(entry json.RawMessage) map[string]any {
	var m map[string]any
	if json.Unmarshal(entry, &m) != nil {
		return nil
	}
	delete(m, a.free)
	return m
}

var (
	mergeConfigurations = mergeEntryArray{entries: func(r mergeReport) []json.RawMessage { return r.Configurations }}
	mergeNotBuilt       = mergeEntryArray{entries: func(r mergeReport) []json.RawMessage { return r.ConfigurationsNotBuilt }, free: "error"}
	mergeLoaded         = mergeEntryArray{entries: func(r mergeReport) []json.RawMessage { return r.Consumers.Loaded }}
	mergeUnavailable    = mergeEntryArray{entries: func(r mergeReport) []json.RawMessage { return r.Consumers.Unavailable }, free: "reason"}
)

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

// pending decodes the finding a dead evaluation carries. A finding that does not decode reads as
// the zero finding, which names no symbol and no component.
func (e mergeEvaluation) pending() mergeFinding {
	var f mergeFinding
	if json.Unmarshal(e.Finding, &f) != nil {
		return mergeFinding{}
	}
	return f
}

// pairedState is the strongest state, live then dead then absent, among the evaluations of the
// case on the other sides of e's edge, or "" where no other side holds one.
func (c mergeCase) pairedState(e mergeEvaluation) string {
	strongest := ""
	for _, other := range c.evaluations() {
		if other.Edge != e.Edge || other.Side == e.Side {
			continue
		}
		if strongest == "" || stateStrength[other.State] < stateStrength[strongest] {
			strongest = other.State
		}
	}
	return strongest
}

// carriesFallenMember reports whether a report of the case holds a pending finding whose pair
// reads as state and carries, in its findings, a finding of the pending finding's component.
func (c mergeCase) carriesFallenMember(state string) bool {
	for _, r := range c.inputs {
		for _, e := range r.EdgeEvaluations {
			if e.State != mergeStateDead || c.pairedState(e) != state {
				continue
			}
			id := e.pending().Component.ID
			if slices.ContainsFunc(r.Findings, func(f mergeFinding) bool { return f.Component.ID == id }) {
				return true
			}
		}
	}
	return false
}

// promotesPairedDead reports whether a pending finding whose pair is dead reaches the case's
// merged report.
func (c mergeCase) promotesPairedDead() bool {
	if c.expected == nil {
		return false
	}
	for _, e := range c.evaluations() {
		if e.State != mergeStateDead || c.pairedState(e) != mergeStateDead {
			continue
		}
		ref := e.pending().Symbol.Ref
		if slices.ContainsFunc(c.expected.Findings, func(f mergeFinding) bool { return f.Symbol.Ref == ref }) {
			return true
		}
	}
	return false
}

// pendsBesideADeadPair reports whether one report holds two pending findings of one component on
// two edges, the pair of one reading as state and the pair of the other dead.
func (c mergeCase) pendsBesideADeadPair(state string) bool {
	for _, r := range c.inputs {
		for _, dropping := range r.EdgeEvaluations {
			if dropping.State != mergeStateDead || c.pairedState(dropping) != state {
				continue
			}
			id := dropping.pending().Component.ID
			if slices.ContainsFunc(r.EdgeEvaluations, func(dead mergeEvaluation) bool {
				return dead.State == mergeStateDead && dead.Edge != dropping.Edge &&
					c.pairedState(dead) == mergeStateDead && dead.pending().Component.ID == id
			}) {
				return true
			}
		}
	}
	return false
}

// refused reports whether the case ends at the failure code before a merged report exists.
func (c mergeCase) refused() bool {
	return c.expected == nil && c.exit == mergeFailureExit
}

// refusedOnTargets reports whether the case is refused and two of its inputs name targets that
// are not one JSON value.
func (c mergeCase) refusedOnTargets() bool {
	if !c.refused() {
		return false
	}
	for i, a := range c.inputs {
		if slices.ContainsFunc(c.inputs[i+1:], func(b mergeReport) bool { return !sameRawJSON(a.Target, b.Target) }) {
			return true
		}
	}
	return false
}

// refusedOnOneArtifact reports whether the case is refused and two of its inputs carry one
// analyzer name and one analyzer version, and caller.json gives the two one digest.
func (c mergeCase) refusedOnOneArtifact() bool {
	if !c.refused() || len(c.digests) != len(c.inputs) {
		return false
	}
	for i, a := range c.inputs {
		for j := i + 1; j < len(c.inputs); j++ {
			b := c.inputs[j]
			if a.Analyzer.Name == b.Analyzer.Name && a.Analyzer.Version == b.Analyzer.Version &&
				c.digests[i] != "" && c.digests[i] == c.digests[j] {
				return true
			}
		}
	}
	return false
}

// refusedOnOneNameTwice reports whether the case is refused and two of its inputs carry one
// analyzer name at two versions, and caller.json gives the two two digests.
func (c mergeCase) refusedOnOneNameTwice() bool {
	if !c.refused() || len(c.digests) != len(c.inputs) {
		return false
	}
	for i, a := range c.inputs {
		for j := i + 1; j < len(c.inputs); j++ {
			b := c.inputs[j]
			if a.Analyzer.Name == b.Analyzer.Name && a.Analyzer.Version != b.Analyzer.Version &&
				c.digests[i] != "" && c.digests[j] != "" && c.digests[i] != c.digests[j] {
				return true
			}
		}
	}
	return false
}

// mergesOneRuleCountedTwoWays reports whether the case merges two inputs holding entries under one
// rule, the input read first holding the lesser count, whose encoding sorts after the other's,
// and the merged report carries both. The merged order is then neither the order the inputs are
// read in nor the order of the counts.
func (c mergeCase) mergesOneRuleCountedTwoWays() bool {
	if c.expected == nil {
		return false
	}
	for i, a := range c.inputs {
		for _, b := range c.inputs[i+1:] {
			for _, x := range a.TestFileRules {
				if slices.ContainsFunc(b.TestFileRules, func(y mergeRule) bool {
					return x.Rule == y.Rule && x.Matched < y.Matched && bytes.Compare(x.encoding(), y.encoding()) > 0 &&
						slices.Contains(c.expected.TestFileRules, x) && slices.Contains(c.expected.TestFileRules, y)
				}) {
					return true
				}
			}
		}
	}
	return false
}

// encoding is the entry's compact JSON encoding, or nil where it does not encode.
func (r mergeRule) encoding() []byte {
	data, err := json.Marshal(r)
	if err != nil {
		return nil
	}
	return data
}

// refusedOnOmitted reports whether the case is refused and one of its inputs omitted a finding.
func (c mergeCase) refusedOnOmitted() bool {
	return c.refused() && slices.ContainsFunc(c.inputs, func(r mergeReport) bool { return r.Totals.Omitted != 0 })
}

// anySharedID reports whether two inputs of the case carry entries under one id, the first input's
// in the array one names and the second's in the array other names, for which holds.
func (c mergeCase) anySharedID(one, other mergeEntryArray, holds func(a, b mergeReport, x, y json.RawMessage) bool) bool {
	for i, a := range c.inputs {
		for j, b := range c.inputs {
			if i == j {
				continue
			}
			for _, x := range one.entries(a) {
				if slices.ContainsFunc(other.entries(b), func(y json.RawMessage) bool {
					return mergeEntryID(x) == mergeEntryID(y) && holds(a, b, x, y)
				}) {
					return true
				}
			}
		}
	}
	return false
}

// refusedOnIdentity reports whether the case is refused and two of its inputs carry entries of
// array under one id whose identity members differ.
func (c mergeCase) refusedOnIdentity(array mergeEntryArray) bool {
	return c.refused() && c.anySharedID(array, array, func(_, _ mergeReport, x, y json.RawMessage) bool {
		return !reflect.DeepEqual(array.identity(x), array.identity(y))
	})
}

// refusedAcross reports whether the case is refused and two of its inputs carry one id, the first
// in the array one names and the second in the array other names.
func (c mergeCase) refusedAcross(one, other mergeEntryArray) bool {
	return c.refused() && c.anySharedID(one, other, func(_, _ mergeReport, _, _ json.RawMessage) bool { return true })
}

// mergesFreeText reports whether the case merges two entries of array under one id that agree on
// every identity member and differ in free text, the merged report carrying the entry of the input
// merged_from names first.
func (c mergeCase) mergesFreeText(array mergeEntryArray) bool {
	if c.expected == nil {
		return false
	}
	return c.anySharedID(array, array, func(a, b mergeReport, x, y json.RawMessage) bool {
		return a.Analyzer.Name < b.Analyzer.Name && !sameRawJSON(x, y) && reflect.DeepEqual(array.identity(x), array.identity(y)) &&
			slices.ContainsFunc(array.entries(*c.expected), func(z json.RawMessage) bool { return sameRawJSON(z, x) })
	})
}

// mergeEntryID is the id an entry of an id-keyed array carries, or "" where it carries none.
func mergeEntryID(entry json.RawMessage) string {
	var e struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(entry, &e) != nil {
		return ""
	}
	return e.ID
}

// sameRawJSON reports whether two documents are one JSON value: objects by their members in any
// order, arrays element by element. Two documents neither of which decodes are equal.
func sameRawJSON(a, b json.RawMessage) bool {
	var x, y any
	errA, errB := json.Unmarshal(a, &x), json.Unmarshal(b, &y)
	return (errA == nil) == (errB == nil) && reflect.DeepEqual(x, y)
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

// TestMissingMergeShapesNamesEveryUncoveredShape drives the coverage check over a synthetic case
// set, so each predicate is exercised in both directions before any case exists.
func TestMissingMergeShapesNamesEveryUncoveredShape(t *testing.T) {
	full := mergeShapeCases()
	if missing := missingMergeShapes(full); len(missing) > 0 {
		t.Fatalf("missingMergeShapes(one case per shape) = %q, want none", missing)
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

const mergeOneArtifactDir = "one-artifact-run-twice"

// TestMissingMergeShapesTellsOneArtifactFromItsNearMisses strips the synthetic one-artifact case
// of each value its shape turns on, so a predicate that reads only some of them fails here.
func TestMissingMergeShapesTellsOneArtifactFromItsNearMisses(t *testing.T) {
	const shape = "two reports one analyzer artifact wrote"
	full := mergeShapeCases()
	at := slices.IndexFunc(full, func(c mergeCase) bool { return c.dir == mergeOneArtifactDir })
	if at < 0 || slices.Contains(missingMergeShapes(full), shape) {
		t.Fatalf("Setup: mergeShapeCases() holds no %s case covering %q", mergeOneArtifactDir, shape)
	}
	cases := []struct {
		strip func(c *mergeCase)
		name  string
	}{
		{name: "two_digests", strip: func(c *mergeCase) { c.digests = []string{mergePlantedGoDigest, mergePlantedTSDigest} }},
		{name: "two_versions", strip: func(c *mergeCase) { c.inputs[1].Analyzer.Version = "1.1.0" }},
		{name: "two_names", strip: func(c *mergeCase) { c.inputs[1].Analyzer.Name = "other-go" }},
		{name: "no_digest", strip: func(c *mergeCase) { c.digests = []string{"", ""} }},
		{name: "a_merged_report", strip: func(c *mergeCase) { c.exit, c.expected = mergeFindingsExit, &mergeReport{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stripped := slices.Clone(full)
			c := stripped[at]
			c.inputs = slices.Clone(c.inputs)
			tc.strip(&c)
			stripped[at] = c
			if missing := missingMergeShapes(stripped); !slices.Contains(missing, shape) {
				t.Errorf("missingMergeShapes(the set with the %s case given %s) = %q, want it to name %q", mergeOneArtifactDir, tc.name, missing, shape)
			}
		})
	}
}

const mergeOneNameDir = "one-analyzer-name-twice"

// TestMissingMergeShapesTellsOneNameTwiceFromItsNearMisses strips the synthetic one-name case of
// each value its shape turns on, so a predicate that reads only some of them fails here.
func TestMissingMergeShapesTellsOneNameTwiceFromItsNearMisses(t *testing.T) {
	cases := []struct {
		strip func(c *mergeCase)
		name  string
	}{
		{name: "one_version", strip: func(c *mergeCase) { c.inputs[1].Analyzer.Version = c.inputs[0].Analyzer.Version }},
		{name: "one_digest", strip: func(c *mergeCase) { c.digests = []string{mergePlantedGoDigest, mergePlantedGoDigest} }},
		{name: "no_second_digest", strip: func(c *mergeCase) { c.digests = []string{mergePlantedGoDigest, ""} }},
		{name: "two_names", strip: func(c *mergeCase) { c.inputs[1].Analyzer.Name = "other-go" }},
		{name: "a_merged_report", strip: func(c *mergeCase) { c.exit, c.expected = mergeFindingsExit, &mergeReport{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stripped := mergeShapeCases()
			at := slices.IndexFunc(stripped, func(c mergeCase) bool { return c.dir == mergeOneNameDir })
			if at < 0 || slices.Contains(missingMergeShapes(stripped), mergeOneNameShape) {
				t.Fatalf("Setup: mergeShapeCases() holds no %s case covering %q", mergeOneNameDir, mergeOneNameShape)
			}
			tc.strip(&stripped[at])
			if missing := missingMergeShapes(stripped); !slices.Contains(missing, mergeOneNameShape) {
				t.Errorf("missingMergeShapes(the set with the %s case given %s) = %q, want it to name %q", mergeOneNameDir, tc.name, missing, mergeOneNameShape)
			}
		})
	}
}

const mergeOneRuleDir = "test-file-rule-counted-two-ways"

// TestMissingMergeShapesTellsOneRuleCountedTwoWaysFromItsNearMisses strips the synthetic
// one-rule case of each value its shape turns on, so a predicate that reads only some of them
// fails here.
func TestMissingMergeShapesTellsOneRuleCountedTwoWaysFromItsNearMisses(t *testing.T) {
	// counted gives the first input x and the second y, and the merged report both.
	counted := func(c *mergeCase, x, y mergeRule) {
		c.inputs[0].TestFileRules, c.inputs[1].TestFileRules = []mergeRule{x}, []mergeRule{y}
		c.expected = &mergeReport{TestFileRules: []mergeRule{y, x}}
	}
	suffix := func(matched int) mergeRule { return mergeRule{Rule: "go-test-suffix", Matched: matched} }
	cases := []struct {
		strip func(c *mergeCase)
		name  string
	}{
		{name: "two_rules", strip: func(c *mergeCase) { counted(c, suffix(9), mergeRule{Rule: "go-test-file", Matched: 10}) }},
		{name: "the_greater_count_read_first", strip: func(c *mergeCase) { counted(c, suffix(9), suffix(8)) }},
		{name: "the_encodings_in_count_order", strip: func(c *mergeCase) { counted(c, suffix(2), suffix(3)) }},
		{name: "the_lesser_count_not_merged", strip: func(c *mergeCase) { c.expected = &mergeReport{TestFileRules: []mergeRule{suffix(10)}} }},
		{name: "the_greater_count_not_merged", strip: func(c *mergeCase) { c.expected = &mergeReport{TestFileRules: []mergeRule{suffix(9)}} }},
		{name: "a_refused_case", strip: func(c *mergeCase) { c.exit, c.expected = mergeFailureExit, nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stripped := mergeShapeCases()
			at := slices.IndexFunc(stripped, func(c mergeCase) bool { return c.dir == mergeOneRuleDir })
			if at < 0 || slices.Contains(missingMergeShapes(stripped), mergeOneRuleShape) {
				t.Fatalf("Setup: mergeShapeCases() holds no %s case covering %q", mergeOneRuleDir, mergeOneRuleShape)
			}
			tc.strip(&stripped[at])
			if missing := missingMergeShapes(stripped); !slices.Contains(missing, mergeOneRuleShape) {
				t.Errorf("missingMergeShapes(the set with the %s case given %s) = %q, want it to name %q", mergeOneRuleDir, tc.name, missing, mergeOneRuleShape)
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
		`"type_error_skips": [], "notes": [], "unanswered_questions": [], "conventions_applied": [], ` +
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
		`"type_error_skips": [], "notes": [], "unanswered_questions": [], "conventions_applied": [], ` +
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
		`"type_error_skips": [], "notes": [], "unanswered_questions": [], "conventions_applied": [], ` +
		`"totals": {"findings": 2, "by_severity": {"allow": 0, "warn": 0, "deny": 2}, "deletable_lines": 2, ` +
		`"suppressions_in_effect": 0, "reasons_recorded": 0, "stale_suppressions": 0, "pending": 0, "omitted": 0}}` + "\n"
)

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
	pendingOn := func(edge, side, component, ref string) mergeEvaluation {
		return mergeEvaluation{
			Edge: edge, Side: side, State: mergeStateDead,
			Finding: json.RawMessage(`{"symbol": {"ref": "` + ref + `"}, "component": {"id": "` + component + `"}}`),
		}
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

	goRoot := pendingOn("wire/ServerEvent", "provides", "deadset-go/c-1", "go://example.com/app#ServerEvent")
	goMember := pendingOn("wire/EventKind", "provides", "deadset-go/c-1", "go://example.com/app#eventKind")
	tsPair := pendingOn("wire/EventKind", "used_by", "deadset-ts/c-1", "ts://@example/app/wire.ts#EventKind")
	withMember := func(r mergeReport) mergeReport {
		r.Findings = []mergeFinding{{Component: mergeComponent{ID: "deadset-go/c-1"}}}
		return r
	}
	promoted := &mergeReport{Findings: []mergeFinding{{Symbol: mergeSymbol{Ref: "go://example.com/app#ServerEvent"}}}}

	refused := func(dir string, edit func(a, b *mergeReport)) mergeCase {
		a, b := goReport(live), tsReport()
		edit(&a, &b)
		return mergeCase{dir: dir, accepted: "1.0.0", exit: mergeFailureExit, inputs: []mergeReport{a, b}}
	}
	mergedCase := func(dir string, edit func(a, b, m *mergeReport)) mergeCase {
		a, b, m := goReport(live), tsReport(), &mergeReport{}
		edit(&a, &b, m)
		return mergeCase{dir: dir, accepted: "1.0.0", exit: mergeFindingsExit, expected: m, inputs: []mergeReport{a, b}}
	}
	entry := func(id, members string) []json.RawMessage {
		return []json.RawMessage{json.RawMessage(`{"id": "` + id + `"` + members + `}`)}
	}
	oneArtifact := refused(mergeOneArtifactDir, func(a, b *mergeReport) { b.Analyzer = a.Analyzer })
	oneArtifact.digests = []string{mergePlantedGoDigest, mergePlantedGoDigest}
	oneName := refused(mergeOneNameDir, func(a, b *mergeReport) {
		b.Analyzer = a.Analyzer
		b.Analyzer.Version = "1.1.0"
	})
	oneName.digests = []string{mergePlantedGoDigest, mergePlantedTSDigest}
	lesser, greater := mergeRule{Rule: "go-test-suffix", Matched: 9}, mergeRule{Rule: "go-test-suffix", Matched: 10}
	oneRule := mergedCase(mergeOneRuleDir, func(a, b, m *mergeReport) {
		a.TestFileRules, b.TestFileRules = []mergeRule{lesser}, []mergeRule{greater}
		m.TestFileRules = []mergeRule{greater, lesser}
	})

	return []mergeCase{
		{dir: "one-report", accepted: "1.0.0", exit: mergeCleanExit, expected: merged, inputs: []mergeReport{goReport()}},
		{dir: "no-edges", accepted: "1.0.0", exit: mergeCleanExit, expected: merged, inputs: []mergeReport{goReport(), tsReport()}},
		{dir: "pair-live", accepted: "1.0.0", exit: mergeCleanExit, expected: merged, inputs: []mergeReport{
			withMember(goReport(goRoot)), tsReport(evaluation("used_by", mergeStateLive)),
		}},
		{dir: "pair-dead", accepted: "1.0.0", exit: mergeFindingsExit, expected: promoted, inputs: []mergeReport{
			goReport(goRoot), tsReport(pendingOn("wire/ServerEvent", "used_by", "deadset-ts/c-1", "ts://@example/app/wire.ts#ServerEvent")),
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
		{dir: "pair-absent", accepted: "1.0.0", exit: mergeFindingsExit, expected: merged, inputs: []mergeReport{
			withMember(goReport(goRoot)), tsReport(evaluation("used_by", mergeStateAbsent)),
		}},
		{dir: "member-on-a-second-edge", accepted: "1.0.0", exit: mergeCleanExit, expected: merged, inputs: []mergeReport{
			goReport(goRoot, goMember), tsReport(evaluation("used_by", mergeStateLive), tsPair),
		}},
		{dir: "member-beside-a-stale-edge", accepted: "1.0.0", exit: mergeFindingsExit, expected: merged, inputs: []mergeReport{
			goReport(goRoot, goMember), tsReport(evaluation("used_by", mergeStateAbsent), tsPair),
		}},
		refused("targets-differ", func(a, b *mergeReport) {
			a.Target, b.Target = json.RawMessage(`{"root": "."}`), json.RawMessage(`{"root": "web"}`)
		}),
		refused("findings-omitted", func(_, b *mergeReport) { b.Totals.Omitted = 1 }),
		refused("configuration-entries-differ", func(a, b *mergeReport) {
			a.Configurations, b.Configurations = entry("linux-amd64", `, "tags": []`), entry("linux-amd64", `, "tags": ["netgo"]`)
		}),
		mergedCase("configuration-not-built-errors-merged", func(a, b, m *mergeReport) {
			a.ConfigurationsNotBuilt, b.ConfigurationsNotBuilt = entry("windows-amd64", `, "error": "a"`), entry("windows-amd64", `, "error": "b"`)
			m.ConfigurationsNotBuilt = a.ConfigurationsNotBuilt
		}),
		refused("configuration-built-and-not-built", func(a, b *mergeReport) {
			a.Configurations, b.ConfigurationsNotBuilt = entry("linux-amd64", ""), entry("linux-amd64", `, "error": "e"`)
		}),
		refused("consumer-entries-differ", func(a, b *mergeReport) {
			a.Consumers.Loaded, b.Consumers.Loaded = entry("example.com/cli", `, "path": "../cli"`), entry("example.com/cli", `, "path": "../tools/cli"`)
		}),
		mergedCase("consumer-unavailable-reasons-merged", func(a, b, m *mergeReport) {
			a.Consumers.Unavailable, b.Consumers.Unavailable = entry("example.com/cli", `, "reason": "a"`), entry("example.com/cli", `, "reason": "b"`)
			m.Consumers.Unavailable = a.Consumers.Unavailable
		}),
		refused("consumer-loaded-and-unavailable", func(a, b *mergeReport) {
			a.Consumers.Loaded, b.Consumers.Unavailable = entry("example.com/cli", `, "path": "../cli"`), entry("example.com/cli", `, "reason": "r"`)
		}),
		oneArtifact,
		oneName,
		oneRule,
	}
}
