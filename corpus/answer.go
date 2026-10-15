package corpus

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// wireReport is the members of a report the runner answers from.
type wireReport struct {
	Findings               []wireFinding    `json:"findings"`
	StaleSuppressions      []wireStale      `json:"stale_suppressions"`
	EdgeEvaluations        []wireEvaluation `json:"edge_evaluations"`
	ConfigurationsNotBuilt []struct {
		ID string `json:"id"`
	} `json:"configurations_not_built"`
	TypeErrorSkips []struct {
		Path string `json:"path"`
		Line int    `json:"line"`
	} `json:"type_error_skips"`
	Notes              []Note       `json:"notes"`
	ConventionsApplied []Convention `json:"conventions_applied"`
	Totals             struct {
		SuppressionsInEffect int `json:"suppressions_in_effect"`
	} `json:"totals"`
}

type wirePosition struct {
	Path string `json:"path"`
	Line int    `json:"line"`
}

func (p wirePosition) site() Site { return Site{File: path.Join(TargetDir, p.Path), Line: p.Line} }

type wireFinding struct {
	Details   map[string]json.RawMessage `json:"details"`
	Component map[string]json.RawMessage `json:"component"`
	Symbol    struct {
		Ref  string `json:"ref"`
		Kind string `json:"kind"`
	} `json:"symbol"`
	Code              string       `json:"code"`
	Confidence        string       `json:"confidence"`
	ReachabilityClass string       `json:"reachability_class"`
	LivenessRelation  string       `json:"liveness_relation"`
	Fixability        string       `json:"fixability"`
	Position          wirePosition `json:"position"`
	Configurations    []string     `json:"configurations"`
}

type wireEntry struct {
	Code   string `json:"code"`
	Symbol string `json:"symbol"`
	Path   string `json:"path"`
}

type wireStale struct {
	Code      string       `json:"code"`
	Mechanism string       `json:"mechanism"`
	Entry     wireEntry    `json:"entry"`
	Position  wirePosition `json:"position"`
}

type wireEvaluation struct {
	Finding *wireFinding `json:"finding"`
	Edge    string       `json:"edge"`
	Side    string       `json:"side"`
	State   string       `json:"state"`
}

// The confidence and subject kind the report schema fixes for a stale-suppression
// record.
const (
	recordConfidence = "certain"
	recordSubject    = "suppression"
)

// configuredEntry is the code and subject reference a finding about the row's
// configured entry carries, where the row names one.
func configuredEntry(expect *Expectation, name string) (code, ref string, ok bool) {
	if root, isRoot := expect.ConfiguredRoots[name]; isRoot {
		return string(vocab.UnmatchedRoot), root, true
	}
	if declared, isDeclared := expect.ConfiguredDeclarations[name]; isDeclared {
		return string(vocab.UnmatchedConfiguredDeclaration), declared.Ref(), true
	}
	return "", "", false
}

func findingsAt(findings []wireFinding, site Site) []wireFinding {
	var held []wireFinding
	for i := range findings {
		if findings[i].Position.site() == site {
			held = append(held, findings[i])
		}
	}
	return held
}

func codesOf(found []wireFinding) string {
	codes := make([]string, len(found))
	for i := range found {
		codes[i] = found[i].Code
	}
	return strings.Join(codes, " ")
}

// answerOf is what the analyzer answered for one row and what differs from it,
// or an empty message.
func answerOf(expect *Expectation, row *Row, sites map[string]Site, held *analysis) (actual Answer, message string) {
	if code, ref, ok := configuredEntry(expect, row.Symbol); ok {
		return answerEntry(row, code, ref, sites, held)
	}
	site := sites[row.Symbol]
	if row.Report == string(vocab.StaleSuppression) {
		return answerStale(row, site, held)
	}
	found := findingsAt(held.report.Findings, site)
	if row.Report != ReportNone {
		return answered(row, found, sites, "finding at "+site.String())
	}
	if len(found) > 0 {
		return answerFrom(row, &found[0], sites), fmt.Sprintf("want no finding at %s, got %s", site, codesOf(found))
	}
	classes := held.retained[site]
	actual = Answer{Report: ReportNone, RetainedBy: classes}
	wanted := slices.Sorted(slices.Values(row.RetainedBy))
	if len(wanted) > 0 && !slices.Equal(wanted, classes) {
		return actual, fmt.Sprintf("want the retained listing to name %s at %s, got %s", jsonOf(wanted), site, jsonOf(nonNil(classes)))
	}
	return actual, ""
}

// answerEntry answers a row naming a configured entry from the findings whose
// subject is that entry, wherever they are positioned.
func answerEntry(row *Row, code, ref string, sites map[string]Site, held *analysis) (actual Answer, message string) {
	var found []wireFinding
	for i := range held.report.Findings {
		if one := &held.report.Findings[i]; one.Code == code && one.Symbol.Ref == ref {
			found = append(found, *one)
		}
	}
	if row.Report != ReportNone {
		return answered(row, found, sites, code+" naming "+ref)
	}
	if len(found) == 0 {
		return Answer{Report: ReportNone}, ""
	}
	return answerFrom(row, &found[0], sites), fmt.Sprintf("want no %s naming %s, got one", code, ref)
}

// answerStale answers a row naming the stale-suppression code from the report's
// records rather than its findings.
func answerStale(row *Row, site Site, held *analysis) (actual Answer, message string) {
	var records []wireStale
	for i := range held.report.StaleSuppressions {
		if held.report.StaleSuppressions[i].Position.site() == site {
			records = append(records, held.report.StaleSuppressions[i])
		}
	}
	if len(records) != 1 {
		actual = Answer{Report: ReportNone}
		if len(records) > 0 {
			actual = answerFromRecord(row, &records[0])
		}
		return actual, fmt.Sprintf("want exactly one %s record at %s, got %d", vocab.StaleSuppression, site, len(records))
	}
	actual = answerFromRecord(row, &records[0])
	message = differences(row, &actual)
	return actual, message
}

// answered is the answer to a row naming a code, from the findings that could
// answer it.
func answered(row *Row, found []wireFinding, sites map[string]Site, where string) (actual Answer, message string) {
	if len(found) == 0 {
		return Answer{Report: ReportNone}, fmt.Sprintf("want %s as the %s, got none", row.Report, where)
	}
	actual = answerFrom(row, &found[0], sites)
	if len(found) > 1 {
		return actual, fmt.Sprintf("want exactly one %s, got %s", where, codesOf(found))
	}
	message = differences(row, &actual)
	return actual, message
}

// answerFrom is one finding in the expectation file's vocabulary, carrying the
// members the row names.
func answerFrom(row *Row, found *wireFinding, sites map[string]Site) Answer {
	actual := Answer{Report: found.Code, Confidence: found.Confidence}
	if row.SymbolKind != "" {
		actual.SymbolKind = found.Symbol.Kind
	}
	if row.ReachabilityClass != "" {
		actual.ReachabilityClass = found.ReachabilityClass
	}
	if row.LivenessRelation != "" {
		actual.LivenessRelation = found.LivenessRelation
	}
	if row.Fixability != "" {
		actual.Fixability = found.Fixability
	}
	if len(row.Configurations) > 0 {
		actual.Configurations = slices.Clone(found.Configurations)
	}
	actual.Details = detailsOf(row.Details, found.Details, sites)
	actual.Component = componentOf(row.Component, found.Component)
	return actual
}

// detailsOf is the finding's details carrying the members the row pins, and nil
// where the row pins none or the finding carries none of them. A name_literal
// answers as the row's logical name when the finding's position is its site.
func detailsOf(want *Details, found map[string]json.RawMessage, sites map[string]Site) *Details {
	if want == nil {
		return nil
	}
	var actual Details
	text := func(member string) string {
		var s string
		_ = json.Unmarshal(found[member], &s)
		return s
	}
	if want.NarrowerVisibility != "" {
		actual.NarrowerVisibility = text("narrower_visibility")
	}
	if want.ExcludedBy != "" {
		actual.ExcludedBy = text("excluded_by")
	}
	if want.DependencyClass != "" {
		actual.DependencyClass = text("dependency_class")
	}
	if want.Replacement != "" {
		actual.Replacement = text("replacement")
	}
	if want.Mechanism != "" {
		actual.Mechanism = text("mechanism")
	}
	if want.Edge != "" {
		actual.Edge = text("edge")
	}
	if want.NameLiteral != "" {
		var at wirePosition
		if json.Unmarshal(found["name_literal"], &at) == nil && at.site() == sites[want.NameLiteral] {
			actual.NameLiteral = want.NameLiteral
		}
	}
	if len(want.Overlap) > 0 {
		_ = json.Unmarshal(found["overlap"], &actual.Overlap)
	}
	if len(actual.Named()) == 0 {
		return nil
	}
	return &actual
}

// componentOf is the finding's component carrying the members the row pins.
func componentOf(want *Component, found map[string]json.RawMessage) *Component {
	if want == nil {
		return nil
	}
	var actual Component
	if raw, ok := found["root"]; ok && want.Root != nil {
		actual.Root = new(bool)
		_ = json.Unmarshal(raw, actual.Root)
	}
	if raw, ok := found["symbol_count"]; ok && want.SymbolCount != nil {
		actual.SymbolCount = new(int)
		_ = json.Unmarshal(raw, actual.SymbolCount)
	}
	if raw, ok := found["deletable_lines"]; ok && want.DeletableLines != nil {
		actual.DeletableLines = new(int)
		_ = json.Unmarshal(raw, actual.DeletableLines)
	}
	if actual == (Component{}) {
		return nil
	}
	return &actual
}

// answerFromRecord is one stale-suppression record in the expectation file's
// vocabulary.
func answerFromRecord(row *Row, found *wireStale) Answer {
	actual := Answer{Report: found.Code, Confidence: recordConfidence}
	if row.SymbolKind != "" {
		actual.SymbolKind = recordSubject
	}
	if row.Details != nil && row.Details.Mechanism != "" {
		actual.Details = &Details{Mechanism: found.Mechanism}
	}
	return actual
}

// differences is every member the row states and the answer does not match, in
// one line, or "".
func differences(row *Row, actual *Answer) string {
	var held []string
	for _, one := range []struct{ member, want, got string }{
		{"report", row.Report, actual.Report},
		{"confidence", row.Confidence, actual.Confidence},
		{"symbol_kind", row.SymbolKind, actual.SymbolKind},
		{"reachability_class", row.ReachabilityClass, actual.ReachabilityClass},
		{"liveness_relation", row.LivenessRelation, actual.LivenessRelation},
		{"fixability", row.Fixability, actual.Fixability},
	} {
		if one.want != "" && one.want != one.got {
			held = append(held, fmt.Sprintf("%s want %s got %s", one.member, jsonOf(one.want), jsonOf(one.got)))
		}
	}
	if len(row.Configurations) > 0 && !slices.Equal(row.Configurations, actual.Configurations) {
		held = append(held, fmt.Sprintf("configurations want %s got %s", jsonOf(row.Configurations), jsonOf(nonNil(actual.Configurations))))
	}
	held = append(held, memberDifferences("details", row.Details, actual.Details)...)
	held = append(held, memberDifferences("component", row.Component, actual.Component)...)
	return strings.Join(held, "; ")
}

// memberDifferences compares the members an object of a row names with the
// answer's object of the same name, member by member.
func memberDifferences(name string, want, got any) []string {
	wanted, actual := members(want), members(got)
	var held []string
	for _, member := range slices.Sorted(maps.Keys(wanted)) {
		value, ok := actual[member]
		if ok && reflect.DeepEqual(wanted[member], value) {
			continue
		}
		spelled := "nothing"
		if ok {
			spelled = jsonOf(value)
		}
		held = append(held, fmt.Sprintf("%s.%s want %s got %s", name, member, jsonOf(wanted[member]), spelled))
	}
	return held
}

// members is an object's members by name as JSON spells them, nothing for nil.
func members(v any) map[string]any {
	held := map[string]any{}
	if rv := reflect.ValueOf(v); !rv.IsValid() || (rv.Kind() == reflect.Pointer && rv.IsNil()) {
		return held
	}
	data, err := json.Marshal(v)
	if err != nil {
		return held
	}
	_ = json.Unmarshal(data, &held)
	return held
}

func jsonOf(v any) string {
	data, _ := json.Marshal(v) //nolint:errchkjson // values of this package's types
	return string(data)
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// unexpectedOf is every finding and record no row names, by position and code,
// ordered by file, line and code. A finding about a configured entry is named by
// a row naming that entry under that code.
func unexpectedOf(report *wireReport, expect *Expectation, sites map[string]Site) []Unexpected {
	named := namedByRows(expect, sites)
	held := []Unexpected{}
	add := func(code, ref string, at wirePosition) {
		key := at.site().String() + " " + code
		if code == string(vocab.UnmatchedRoot) || code == string(vocab.UnmatchedConfiguredDeclaration) {
			key = code + " " + ref
		}
		if !named[key] {
			held = append(held, Unexpected{File: path.Join(TargetDir, at.Path), Line: at.Line, Report: code})
		}
	}
	for i := range report.Findings {
		add(report.Findings[i].Code, report.Findings[i].Symbol.Ref, report.Findings[i].Position)
	}
	for i := range report.StaleSuppressions {
		add(report.StaleSuppressions[i].Code, "", report.StaleSuppressions[i].Position)
	}
	slices.SortFunc(held, func(a, b Unexpected) int {
		if c := strings.Compare(a.File, b.File); c != 0 {
			return c
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(a.Report, b.Report)
	})
	return held
}

// namedByRows is what the rows name: each site and code, and each configured
// entry's code and subject reference.
func namedByRows(expect *Expectation, sites map[string]Site) map[string]bool {
	named := map[string]bool{}
	for i := range expect.Expect {
		row := &expect.Expect[i]
		if code, ref, ok := configuredEntry(expect, row.Symbol); ok {
			if row.Report == code {
				named[code+" "+ref] = true
			}
			continue
		}
		if site, ok := sites[row.Symbol]; ok && row.Report != ReportNone {
			named[site.String()+" "+row.Report] = true
		}
	}
	return named
}

type ignoreEntry struct {
	Code   string `json:"code"`
	Symbol string `json:"symbol"`
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// suppressionPhase writes every finding the first analysis reported at a row's
// position that a record can bind to into the rendering's ignore file, analyzes
// again, and records for each such row whether its position fell silent and
// whether its entry was reported stale. It returns the digest of what it wrote,
// or "" where it wrote nothing.
func (r *runner) suppressionPhase(ctx context.Context, at *scratch, expect *Expectation, sites map[string]Site,
	first *analysis, rows []Row, held []ExpectationResult,
) (string, error) {
	covered, order := r.ignoreEntries(expect, sites, first)
	if len(covered) == 0 {
		return "", nil
	}
	body, err := writeIgnore(filepath.Join(at.rendering, TargetDir, ignoreFile), order)
	if err != nil {
		return "", err
	}
	second, err := r.analyze(ctx, at, "second", false)
	if err != nil {
		return "", err
	}
	for i := range rows {
		entry, ok := covered[rows[i].Symbol]
		if !ok {
			continue
		}
		suppressed := !slices.ContainsFunc(findingsAt(second.report.Findings, sites[rows[i].Symbol]), func(one wireFinding) bool {
			return one.Code == entry.Code
		})
		stale := slices.ContainsFunc(second.report.StaleSuppressions, func(one wireStale) bool {
			return one.Entry.Code == entry.Code && one.Entry.Symbol == entry.Symbol && one.Entry.Path == entry.Path
		})
		held[i].Suppression = &Suppression{Suppressed: suppressed, Stale: stale}
	}
	return fmt.Sprintf("%x", sha256.Sum256(body)), nil
}

// ignoreEntries is the entry the suppression phase writes for each row whose
// first-phase finding a record binds to, and the distinct entries in row order:
// two parts of one declaration under one code are one entry, which a second
// copy of would make stale.
func (r *runner) ignoreEntries(expect *Expectation, sites map[string]Site, first *analysis) (map[string]ignoreEntry, []ignoreEntry) {
	covered := map[string]ignoreEntry{}
	var order []ignoreEntry
	for i := range expect.Expect {
		row := &expect.Expect[i]
		if _, _, configured := configuredEntry(expect, row.Symbol); configured || row.Report == ReportNone || row.Report == string(vocab.StaleSuppression) {
			continue
		}
		found := findingsAt(first.report.Findings, sites[row.Symbol])
		if len(found) == 0 || r.rows[found[0].Symbol.Kind] {
			continue
		}
		entry := ignoreEntry{Code: found[0].Code, Symbol: found[0].Symbol.Ref, Path: found[0].Position.Path, Reason: runnerReason}
		covered[row.Symbol] = entry
		if !slices.Contains(order, entry) {
			order = append(order, entry)
		}
	}
	return covered, order
}

// writeIgnore adds the entries to the ignore file at name, keeping the records
// the rendering carries, and returns the bytes it wrote.
func writeIgnore(name string, entries []ignoreEntry) ([]byte, error) {
	document := map[string]any{"description": ignoreSummary, "ignore": []any{}}
	data, err := os.ReadFile(name)
	switch {
	case err == nil:
		if decodeErr := json.Unmarshal(data, &document); decodeErr != nil {
			return nil, fixtureError("the rendering's %s: %v", ignoreFile, decodeErr)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return nil, err
	}
	own, _ := document["ignore"].([]any)
	for _, entry := range entries {
		own = append(own, entry)
	}
	document["ignore"] = own
	body, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return nil, err
	}
	body = append(body, '\n')
	return body, os.WriteFile(name, body, 0o600)
}

// edgeDifferences is every way the report's edge evaluations differ from the
// fixture's, in one line, or "".
func edgeDifferences(expect *Expectation, manifest *Manifest, report *wireReport) string {
	var held []string
	for _, want := range expect.EdgeEvaluations {
		if message := evaluationDifference(&want, manifest, report); message != "" {
			held = append(held, message)
		}
	}
	for _, record := range report.EdgeEvaluations {
		if !slices.ContainsFunc(expect.EdgeEvaluations, func(one Evaluation) bool { return one.Edge == record.Edge && one.Side == record.Side }) {
			held = append(held, fmt.Sprintf("evaluation of %s %s, which no entry names", record.Edge, record.Side))
		}
	}
	return strings.Join(held, "; ")
}

// evaluationDifference is how the report answers one entry differently: no or
// several records of its edge and side, another state, or for a dead side a
// pending finding under another code or at another site.
func evaluationDifference(want *Evaluation, manifest *Manifest, report *wireReport) string {
	named := want.Edge + " " + want.Side
	var records []wireEvaluation
	for _, one := range report.EdgeEvaluations {
		if one.Edge == want.Edge && one.Side == want.Side {
			records = append(records, one)
		}
	}
	switch {
	case len(records) != 1:
		return fmt.Sprintf("want exactly one evaluation of %s, got %d", named, len(records))
	case records[0].State != want.State:
		return fmt.Sprintf("%s state want %s got %s", named, want.State, records[0].State)
	case want.State != "dead":
		return ""
	}
	site := ""
	if bound, ok := manifest.Symbols[want.Symbol]; ok && bound.File != "" && bound.Line != nil {
		site = Site{File: bound.File, Line: *bound.Line}.String()
	}
	found := records[0].Finding
	if found != nil && found.Code == want.Report && found.Position.site().String() == site {
		return ""
	}
	got := "none"
	if found != nil {
		got = found.Code + " at " + found.Position.site().String()
	}
	return fmt.Sprintf("%s want %s at %s, got %s", named, want.Report, site, got)
}

// notBuiltDifference is how the report's dropped configurations differ from the
// list the manifest names, or "" where the manifest names none.
func notBuiltDifference(manifest *Manifest, report *wireReport) string {
	if manifest.ConfigurationsNotBuilt == nil {
		return ""
	}
	got := []string{}
	for _, one := range report.ConfigurationsNotBuilt {
		got = append(got, one.ID)
	}
	if slices.Equal(got, manifest.ConfigurationsNotBuilt) {
		return ""
	}
	return fmt.Sprintf("configurations_not_built want %s got %s", jsonOf(manifest.ConfigurationsNotBuilt), jsonOf(got))
}

// skipDifferences is every way the report's type-error skips differ from the
// fixture's, or "" where a declared gap declines the capability.
func skipDifferences(expect *Expectation, manifest *Manifest, report *wireReport, declined bool) string {
	if declined {
		return ""
	}
	names := map[Site]string{}
	for _, name := range expect.TypeErrorSkips {
		bound := manifest.Symbols[name]
		names[Site{File: bound.File, Line: *bound.Line}] = name
	}
	reported := map[Site]bool{}
	var order []Site
	for _, one := range report.TypeErrorSkips {
		site := Site{File: path.Join(TargetDir, one.Path), Line: one.Line}
		if !reported[site] {
			order = append(order, site)
		}
		reported[site] = true
	}
	var held []string
	for _, name := range expect.TypeErrorSkips {
		bound := manifest.Symbols[name]
		if site := (Site{File: bound.File, Line: *bound.Line}); !reported[site] {
			held = append(held, fmt.Sprintf("want a type-error skip %s at %s, got none", name, site))
		}
	}
	for _, site := range order {
		if _, ok := names[site]; !ok {
			held = append(held, fmt.Sprintf("type-error skip at %s, which no name resolves to", site))
		}
	}
	return strings.Join(held, "; ")
}

// noteDifferences is every way the report's notes differ from the fixture's.
func noteDifferences(expect *Expectation, report *wireReport) string {
	var held []string
	for _, want := range expect.Notes {
		n := 0
		for _, one := range report.Notes {
			if one.Kind == want.Kind && one.Path == want.Path {
				n++
			}
		}
		if n != 1 {
			held = append(held, fmt.Sprintf("want exactly one %s note at %s, got %d", want.Kind, want.Path, n))
		}
	}
	for _, record := range report.Notes {
		if !slices.ContainsFunc(expect.Notes, func(one Note) bool { return one.Kind == record.Kind && one.Path == record.Path }) {
			held = append(held, fmt.Sprintf("%s note at %s, which no entry names", record.Kind, record.Path))
		}
	}
	return strings.Join(held, "; ")
}

// conventionDifferences is every way the report's applied convention rows differ
// from the fixture's, or "" where a declared gap declines a row the fixture names.
func conventionDifferences(expect *Expectation, report *wireReport, declined bool) string {
	if declined {
		return ""
	}
	var held []string
	for _, want := range expect.ConventionsApplied {
		if !slices.Contains(report.ConventionsApplied, want) {
			held = append(held, fmt.Sprintf("want the row %s applied", want))
		}
	}
	for _, listed := range report.ConventionsApplied {
		if !slices.Contains(expect.ConventionsApplied, listed) {
			held = append(held, fmt.Sprintf("the row %s applied, which no entry names", listed))
		}
	}
	return strings.Join(held, "; ")
}

// inEffectDifference is how the first phase's suppressions in effect differ from
// the count the fixture pins, or "".
func inEffectDifference(expect *Expectation, report *wireReport) string {
	if expect.SuppressionsInEffect == nil || *expect.SuppressionsInEffect == report.Totals.SuppressionsInEffect {
		return ""
	}
	return fmt.Sprintf("suppressions_in_effect want %d got %d", *expect.SuppressionsInEffect, report.Totals.SuppressionsInEffect)
}
