package spec_test

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/corpus"
)

// This file is the merge contract/grammar/merge.md states, run over each published case's
// inputs, accepted range and caller's facts, so a case whose expected bytes the three do not
// determine fails here rather than in the first implementation that runs it.

const mergeCallerFile = "caller.json"

// jsonKind is the kind of one decoded JSON value.
type jsonKind int

const (
	jsonNull jsonKind = iota
	jsonBool
	jsonNumber
	jsonString
	jsonArray
	jsonObject
)

// jsonValue is a decoded JSON value that keeps its members in document order, so a carried
// record is written back with the field order its writer gave it.
type jsonValue struct {
	members []jsonMember
	items   []*jsonValue
	text    string
	kind    jsonKind
}

type jsonMember struct {
	value *jsonValue
	name  string
}

func decodeJSONValue(data []byte) (*jsonValue, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := readJSONValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("content after the document: %v", err)
	}
	return v, nil
}

func readJSONValue(dec *json.Decoder) (*jsonValue, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		if t == '{' {
			return readJSONObject(dec)
		}
		return readJSONArray(dec)
	case string:
		return &jsonValue{kind: jsonString, text: t}, nil
	case json.Number:
		return &jsonValue{kind: jsonNumber, text: t.String()}, nil
	case bool:
		return &jsonValue{kind: jsonBool, text: strconv.FormatBool(t)}, nil
	}
	return &jsonValue{kind: jsonNull, text: "null"}, nil
}

func readJSONObject(dec *json.Decoder) (*jsonValue, error) {
	v := &jsonValue{kind: jsonObject}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		name, _ := tok.(string)
		member, err := readJSONValue(dec)
		if err != nil {
			return nil, err
		}
		v.members = append(v.members, jsonMember{name: name, value: member})
	}
	_, err := dec.Token()
	return v, err
}

func readJSONArray(dec *json.Decoder) (*jsonValue, error) {
	v := &jsonValue{kind: jsonArray, items: []*jsonValue{}}
	for dec.More() {
		item, err := readJSONValue(dec)
		if err != nil {
			return nil, err
		}
		v.items = append(v.items, item)
	}
	_, err := dec.Token()
	return v, err
}

func jsonText(s string) *jsonValue { return &jsonValue{kind: jsonString, text: s} }

func jsonNumberOf(n int) *jsonValue { return &jsonValue{kind: jsonNumber, text: strconv.Itoa(n)} }

func jsonBoolOf(b bool) *jsonValue { return &jsonValue{kind: jsonBool, text: strconv.FormatBool(b)} }

func jsonArrayOf(items ...*jsonValue) *jsonValue {
	return &jsonValue{kind: jsonArray, items: append([]*jsonValue{}, items...)}
}

func jsonStrings(values []string) *jsonValue {
	out := jsonArrayOf()
	for _, s := range values {
		out.items = append(out.items, jsonText(s))
	}
	return out
}

// jsonObjectOf builds an object from alternating names and values.
func jsonObjectOf(pairs ...any) *jsonValue {
	v := &jsonValue{kind: jsonObject}
	for i := 0; i+1 < len(pairs); i += 2 {
		name, _ := pairs[i].(string)
		member, _ := pairs[i+1].(*jsonValue)
		v.members = append(v.members, jsonMember{name: name, value: member})
	}
	return v
}

// get returns the named member, or nil where the object holds none.
func (v *jsonValue) get(name string) *jsonValue {
	if v == nil {
		return nil
	}
	for _, m := range v.members {
		if m.name == name {
			return m.value
		}
	}
	return nil
}

// at follows a path of member names.
func (v *jsonValue) at(path ...string) *jsonValue {
	for _, name := range path {
		v = v.get(name)
	}
	return v
}

func (v *jsonValue) str() string {
	if v == nil {
		return ""
	}
	return v.text
}

func (v *jsonValue) truth() bool { return v != nil && v.kind == jsonBool && v.text == "true" }

func (v *jsonValue) num() int {
	if v == nil {
		return 0
	}
	n, err := strconv.Atoi(v.text)
	if err != nil {
		return 0
	}
	return n
}

func (v *jsonValue) clone() *jsonValue {
	out := &jsonValue{kind: v.kind, text: v.text}
	for _, m := range v.members {
		out.members = append(out.members, jsonMember{name: m.name, value: m.value.clone()})
	}
	if v.kind == jsonArray {
		out.items = []*jsonValue{}
		for _, item := range v.items {
			out.items = append(out.items, item.clone())
		}
	}
	return out
}

// set replaces the named member's value, or inserts the member before the member named before,
// or at the end where before names nothing the object holds.
func (v *jsonValue) set(name string, value *jsonValue, before string) {
	for i, m := range v.members {
		if m.name == name {
			v.members[i].value = value
			return
		}
	}
	member := jsonMember{name: name, value: value}
	for i, m := range v.members {
		if m.name == before {
			v.members = slices.Insert(v.members, i, member)
			return
		}
	}
	v.members = append(v.members, member)
}

// compact writes the value with no insignificant whitespace, every string in the escaping the
// merge vectors' README states.
func (v *jsonValue) compact(buf *bytes.Buffer) error {
	switch v.kind {
	case jsonString:
		var s bytes.Buffer
		enc := json.NewEncoder(&s)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(v.text); err != nil {
			return err
		}
		buf.Write(bytes.TrimSuffix(s.Bytes(), []byte("\n")))
	case jsonArray:
		buf.WriteByte('[')
		for i, item := range v.items {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := item.compact(buf); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
	case jsonObject:
		buf.WriteByte('{')
		for i, m := range v.members {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := jsonText(m.name).compact(buf); err != nil {
				return err
			}
			buf.WriteByte(':')
			if err := m.value.compact(buf); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
	default:
		buf.WriteString(v.text)
	}
	return nil
}

func (v *jsonValue) compactBytes() []byte {
	var buf bytes.Buffer
	if err := v.compact(&buf); err != nil {
		return []byte(err.Error())
	}
	return buf.Bytes()
}

// render writes the value in the encoding a merged report is compared in: two-space
// indentation, one member or element to a line, and one trailing newline.
func (v *jsonValue) render() ([]byte, error) {
	var compact, out bytes.Buffer
	if err := v.compact(&compact); err != nil {
		return nil, err
	}
	if err := json.Indent(&out, compact.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	out.WriteByte('\n')
	return out.Bytes(), nil
}

// mergeKey is the canonical key of one record, read from the members the record's schema maps
// to each component.
type mergeKey struct {
	path, code, ref, analyzer string
	line, column              int
}

func findingKey(f *jsonValue) mergeKey {
	return mergeKey{
		path:     f.at("position", "path").str(),
		line:     f.at("position", "line").num(),
		column:   f.at("position", "column").num(),
		code:     f.get("code").str(),
		ref:      f.at("symbol", "ref").str(),
		analyzer: f.get("analyzer").str(),
	}
}

func staleKey(r *jsonValue) mergeKey {
	return mergeKey{
		path:     r.at("position", "path").str(),
		line:     r.at("position", "line").num(),
		column:   r.at("position", "column").num(),
		code:     r.get("code").str(),
		ref:      r.get("symbol").str(),
		analyzer: r.get("analyzer").str(),
	}
}

func gapKey(r *jsonValue) mergeKey { return mergeKey{analyzer: r.get("analyzer").str()} }

func (k mergeKey) compare(o mergeKey) int {
	return cmp.Or(
		strings.Compare(k.path, o.path),
		cmp.Compare(k.line, o.line),
		cmp.Compare(k.column, o.column),
		strings.Compare(k.code, o.code),
		strings.Compare(k.ref, o.ref),
		strings.Compare(k.analyzer, o.analyzer),
	)
}

// sortByKey orders records by the canonical key, two records equal on every component by their
// compact encodings.
func sortByKey(records []*jsonValue, key func(*jsonValue) mergeKey) {
	slices.SortStableFunc(records, func(a, b *jsonValue) int {
		return cmp.Or(key(a).compare(key(b)), bytes.Compare(a.compactBytes(), b.compactBytes()))
	})
}

// unionEntries carries every entry of the named array of every input once, ordered by the
// named member and then by the compact encoding.
func unionEntries(inputs []mergeInput, trace mergeTrace, array string, by ...string) *jsonValue {
	seen := map[string]bool{}
	var out []*jsonValue
	for _, in := range inputs {
		for _, entry := range in.report.get(array).items {
			key := string(entry.compactBytes())
			if !seen[key] {
				seen[key] = true
				out = append(out, entry.clone())
			}
		}
	}
	read := slices.Clone(out)
	slices.SortStableFunc(out, func(a, b *jsonValue) int {
		for _, member := range by {
			x, y := a.get(member), b.get(member)
			order := strings.Compare(x.str(), y.str())
			if x.kind == jsonNumber {
				order = cmp.Compare(x.num(), y.num())
			}
			if order != 0 {
				return order
			}
		}
		order := bytes.Compare(a.compactBytes(), b.compactBytes())
		if order != 0 && slices.Index(read, a) < slices.Index(read, b) == (order > 0) {
			trace.hit(traceTieAgainstReadOrder + array)
		}
		return order
	})
	return jsonArrayOf(out...)
}

// mergeInput is one input report, the file the case holds it in and the digest the caller gave for
// it.
type mergeInput struct {
	report *jsonValue
	file   string
	name   string
	digest string
}

// mergeCaller is a case's caller.json.
type mergeCaller struct {
	Digests         map[string]string `json:"digests"`
	SchemaVersion   string            `json:"schema_version"`
	ContractVersion string            `json:"contract_version"`
	FailOn          string            `json:"fail_on"`
	Analyzer        struct {
		Conformance json.RawMessage `json:"conformance"`
		Name        string          `json:"name"`
		Version     string          `json:"version"`
	} `json:"analyzer"`
}

// referenceEvaluation is one edge evaluation of the working set, with the analyzer that carried it.
type referenceEvaluation struct {
	record   *jsonValue
	edge     string
	side     string
	state    string
	analyzer string
}

var stateStrength = map[string]int{mergeStateLive: 0, mergeStateDead: 1, mergeStateAbsent: 2}

// severityRank orders the severities a caller's fail_on names, lowest first.
var severityRank = map[string]int{"allow": 0, "warn": 1, "deny": 2}

// classRank orders the confidences, highest first.
var classRank = map[string]int{"certain": 0, "probable": 1, "possible": 2}

// idKeyedArray is one array whose entries a merge unions under their id, with the member of an
// entry that is free text rather than identity, where the array's entries carry one.
type idKeyedArray struct {
	freeText string
	path     []string
}

// identity is the entry without its free-text member: what two entries under one id must agree on.
func (a idKeyedArray) identity(entry *jsonValue) *jsonValue {
	out := entry.clone()
	out.members = slices.DeleteFunc(out.members, func(m jsonMember) bool { return m.name == a.freeText })
	return out
}

func (a idKeyedArray) name() string { return strings.Join(a.path, ".") }

// idKeyedPairs pairs each id-keyed array with the array that holds the same identifiers in their
// other state.
var idKeyedPairs = [][2]idKeyedArray{
	{{path: []string{"configurations"}}, {path: []string{"configurations_not_built"}, freeText: "error"}},
	{{path: []string{"consumers", "loaded"}}, {path: []string{"consumers", "unavailable"}, freeText: "reason"}},
}

// mergeTrace records the branches of contract/grammar/merge.md the reference merge takes, so
// the case set's coverage is read from what the merge does rather than restated beside it.
type mergeTrace map[string]bool

func (t mergeTrace) hit(point string) {
	if t != nil {
		t[point] = true
	}
}

// The trace points the reference merge records; a point ending in a space is completed by the
// state, array or path the branch turns on.
const (
	traceOneReport               = "one report only"
	traceNoEdges                 = "two reports with no edges"
	traceWithheldSummed          = "two reports that each withheld findings, the counts summed"
	traceStaleSuppression        = "a report holding a stale suppression"
	traceSchemaVersion           = "a schema version outside the accepted range"
	traceConformance             = "an analyzer with no conformance pass"
	traceOmitted                 = "a report that omitted a finding"
	traceTargets                 = "two reports naming different targets"
	traceMixedNotLanguages       = "a mixed report whose language kinds are not its languages"
	traceTwoKindsOneLanguage     = "two reports claiming two kinds for one language"
	traceTwoKindsMixed           = "two single-kind reports of two kinds, merged as mixed"
	traceTwoAnalyzersOneLanguage = "two analyzers claiming one language, merged"
	traceOneArtifact             = "two reports one analyzer artifact wrote"
	traceOneNameTwoVersions      = "two reports one analyzer name wrote at two versions from two artifacts"
	traceOneNameOther            = "two reports carrying one analyzer name, neither one artifact nor two versions"
	traceUnresolvedEdge          = "a pending finding whose edge appears in no other report"
	tracePromotedDeadPair        = "a pending finding whose pair is dead promoted into the merged report"
	traceFallsWithPair           = "a finding falling with its pending component, whose pair is "
	traceDroppedBesideDeadPair   = "a component pending beside a dead pair, dropped by a pair that is "
	traceEdgeAbsent              = "an edge side every evaluation reports absent"
	traceRootMatchedElsewhere    = "an unmatched root another analyzer reading it matched, not carried"
	tracePromotedOnce            = "a finding two dead evaluations carry, promoted once"
	traceJoinedConfidence        = "a joined component's finding lowered to its roots' lowest confidence"
	traceFreeTextMerged          = "two entries under one id whose free text alone differs, the first kept, in "
	traceTieAgainstReadOrder     = "two entries tied on their order, sorted by encoding against the read order, in "
)

var errUnresolvedEdge = errors.New("a dead evaluation meets no evaluation of its edge's other side")

// referenceMerge runs the seven steps of contract/grammar/merge.md and returns the merged report
// and the exit code, or a nil report and code 3 where a step ends the run.
func referenceMerge(inputs []mergeInput, accepted []string, caller mergeCaller, kinds map[string]kindRow, trace mergeTrace) (*jsonValue, int, error) {
	if err := admit(inputs, accepted, trace); err != nil {
		return nil, mergeFailureExit, err
	}
	if len(inputs) == 1 {
		trace.hit(traceOneReport)
	}

	var findings, stale, gaps []*jsonValue
	var evaluations []referenceEvaluation
	for _, in := range inputs {
		for _, r := range in.report.get("stale_suppressions").items {
			trace.hit(traceStaleSuppression)
			stale = append(stale, carried(r, in.name, ""))
		}
		for _, r := range in.report.get("declared_gaps").items {
			gaps = append(gaps, carried(r, in.name, ""))
		}
		for _, e := range in.report.get("edge_evaluations").items {
			evaluations = append(evaluations, referenceEvaluation{
				record: e, edge: e.get("edge").str(), side: e.get("side").str(), state: e.get("state").str(), analyzer: in.name,
			})
		}
	}

	if len(inputs) > 1 && len(evaluations) == 0 {
		trace.hit(traceNoEdges)
	}
	promoted, dropped, err := resolveEvaluations(evaluations, trace)
	if err != nil {
		return nil, mergeFailureExit, err
	}
	matched := matchedRoots(inputs)
	for _, in := range inputs {
		for _, f := range in.report.get("findings").items {
			if pair := dropped[f.at("component", "id").str()]; pair != "" {
				trace.hit(traceFallsWithPair + pair)
				continue
			}
			if f.get("code").str() == unmatchedRootCode && matched[f.at("symbol", "ref").str()] {
				trace.hit(traceRootMatchedElsewhere)
				continue
			}
			findings = append(findings, carried(f, in.name, "details"))
		}
	}
	for _, p := range promoted {
		findings = append(findings, p.finding)
	}
	joined := unionComponents(findings, promoted, trace)
	staleEdge := staleEdges(evaluations, caller.Analyzer.Name, kinds)
	if len(staleEdge) > 0 {
		trace.hit(traceEdgeAbsent)
	}
	findings = append(findings, staleEdge...)

	sortByKey(findings, findingKey)
	sortByKey(stale, staleKey)
	sortByKey(gaps, gapKey)

	report := mergedEnvelope(inputs, accepted, caller, trace)
	report.set("findings", jsonArrayOf(findings...), "")
	report.set("edge_evaluations", keptEvaluations(evaluations), "")
	if listed := listedComponents(inputs, findings, joined); listed != nil {
		report.set("components", listed, "edge_evaluations")
	}
	report.set("stale_suppressions", jsonArrayOf(stale...), "")
	report.set("declared_gaps", jsonArrayOf(gaps...), "")
	report.set("excluded_by_cgo", unionStrings(inputs, "excluded_by_cgo"), "")
	report.set("test_file_rules", unionEntries(inputs, trace, "test_file_rules", "rule"), "")
	report.set("type_error_skips", unionEntries(inputs, trace, "type_error_skips", "path", "line"), "")
	report.set("notes", unionEntries(inputs, trace, "notes", "kind", "path"), "")
	report.set("unanswered_questions", unionEntries(inputs, trace, "unanswered_questions", "configuration"), "")
	report.set("conventions_applied", unionEntries(inputs, trace, "conventions_applied", "name", "manifest"), "")
	report.set("totals", mergedTotals(inputs, findings, stale, trace), "")

	exit := mergeCleanExit
	failing := func(f *jsonValue) bool { return severityRank[f.get("severity").str()] >= severityRank[caller.FailOn] }
	if len(stale) > 0 || slices.ContainsFunc(findings, failing) {
		exit = mergeFindingsExit
	}
	return report, exit, nil
}

// admit runs step 1 over every input and every two inputs, naming the first refusal.
func admit(inputs []mergeInput, accepted []string, trace mergeTrace) error {
	for _, in := range inputs {
		if !slices.Contains(accepted, in.report.get("schema_version").str()) {
			trace.hit(traceSchemaVersion)
			return fmt.Errorf("%s: schema_version %s is outside %q", in.file, in.report.get("schema_version").str(), accepted)
		}
		if in.report.at("analyzer", "conformance", "result").str() != "pass" {
			trace.hit(traceConformance)
			return fmt.Errorf("%s: %s records no conformance pass", in.file, in.name)
		}
		if omitted := in.report.at("totals", "omitted").num(); omitted != 0 {
			trace.hit(traceOmitted)
			return fmt.Errorf("%s: %s omitted %d findings", in.file, in.name, omitted)
		}
	}
	if _, err := claimedKinds(inputs, trace); err != nil {
		return err
	}
	for i, in := range inputs {
		for _, other := range inputs[i+1:] {
			for _, member := range []string{"root", "identity"} {
				if a, b := in.report.at("target", member), other.report.at("target", member); !sameJSON(a, b) {
					trace.hit(traceTargets)
					return fmt.Errorf("%s names the target %s %s and %s names %s", in.file, member, a.compactBytes(), other.file, b.compactBytes())
				}
			}
			if entries, id, held := conflictingEntry(in.report, other.report); held {
				trace.hit(entries)
				return fmt.Errorf("%s and %s carry %s under the id %q", in.file, other.file, entries, id)
			}
			if in.name == other.name {
				trace.hit(oneNameTwice(in, other))
				return fmt.Errorf("%s and %s carry one analyzer name, %s", in.file, other.file, in.name)
			}
		}
	}
	return nil
}

// oneNameTwice is the trace point of two reports carrying one analyzer name: one artifact run
// twice, or one name at two versions from two artifacts.
func oneNameTwice(in, other mergeInput) string {
	sameVersion := sameJSON(in.report.at("analyzer", "version"), other.report.at("analyzer", "version"))
	switch {
	case sameVersion && in.digest != "" && in.digest == other.digest:
		return traceOneArtifact
	case !sameVersion && in.digest != other.digest:
		return traceOneNameTwoVersions
	}
	return traceOneNameOther
}

// claimedKinds is the kind every input report claims for each language its analyzer.languages
// names, refusing a mixed kind whose language_kinds is not exactly those languages or names
// one kind for all of them, and two reports claiming two kinds for one language.
func claimedKinds(inputs []mergeInput, trace mergeTrace) (map[string]string, error) {
	claims, from := map[string]string{}, map[string]string{}
	for _, in := range inputs {
		kind, perLanguage := in.report.at("target", "kind").str(), in.report.at("target", "language_kinds")
		var languages []string
		for _, l := range in.report.at("analyzer", "languages").items {
			languages = append(languages, l.str())
		}
		own := map[string]string{}
		for _, language := range languages {
			own[language] = kind
		}
		if kind == "mixed" {
			own = map[string]string{}
			for _, m := range perLanguage.members {
				own[m.name] = m.value.str()
			}
			distinct := slices.Compact(slices.Sorted(maps.Values(own)))
			if !slices.Equal(slices.Sorted(maps.Keys(own)), slices.Sorted(slices.Values(languages))) || len(distinct) < 2 {
				trace.hit(traceMixedNotLanguages)
				return nil, fmt.Errorf("%s names the kinds %s for the languages %q", in.file, perLanguage.compactBytes(), languages)
			}
		}
		for _, language := range slices.Sorted(maps.Keys(own)) {
			if prior, ok := claims[language]; ok && prior != own[language] {
				trace.hit(traceTwoKindsOneLanguage)
				return nil, fmt.Errorf("%s gives %s the kind %s and %s gives it %s", from[language], language, prior, in.file, own[language])
			}
			claims[language], from[language] = own[language], in.file
		}
	}
	return claims, nil
}

// contractLanguages is the language vocabulary in the order contract/languages.json lists it,
// which is the order report.schema.json lists the members of language_kinds in.
func contractLanguages() []string {
	var doc struct {
		Languages []struct {
			Language string `json:"language"`
		} `json:"languages"`
	}
	data, err := fs.ReadFile(spec.Contract, languagesPath)
	if err != nil || json.Unmarshal(data, &doc) != nil {
		return nil
	}
	ids := make([]string, 0, len(doc.Languages))
	for _, row := range doc.Languages {
		ids = append(ids, row.Language)
	}
	return ids
}

// mergedTarget is the target of the merged report: the inputs' root and identity, and the
// one kind every claimed language shares, or mixed with each language's kind in schema order.
func mergedTarget(inputs []mergeInput, kinds map[string]string, languageOrder []string, trace mergeTrace) *jsonValue {
	first := inputs[0].report.get("target")
	distinct := slices.Compact(slices.Sorted(maps.Values(kinds)))
	if len(distinct) == 1 {
		return jsonObjectOf("kind", jsonText(distinct[0]), "root", first.get("root").clone(), "identity", first.get("identity").clone())
	}
	if !slices.ContainsFunc(inputs, func(in mergeInput) bool { return in.report.at("target", "kind").str() == "mixed" }) {
		trace.hit(traceTwoKindsMixed)
	}
	perLanguage := jsonObjectOf()
	for _, language := range languageOrder {
		if kind, ok := kinds[language]; ok {
			perLanguage.set(language, jsonText(kind), "")
		}
	}
	return jsonObjectOf("kind", jsonText("mixed"), "language_kinds", perLanguage,
		"root", first.get("root").clone(), "identity", first.get("identity").clone())
}

// sameJSON reports whether two values are equal as JSON values: objects by their members in any
// order, arrays element by element.
func sameJSON(a, b *jsonValue) bool {
	var x, y any
	if json.Unmarshal(a.compactBytes(), &x) != nil || json.Unmarshal(b.compactBytes(), &y) != nil {
		return false
	}
	return reflect.DeepEqual(x, y)
}

// conflictingEntry names the first id the two reports collide on: two entries of one array whose
// identity members differ, or one entry in each array of a pair.
func conflictingEntry(a, b *jsonValue) (entries, id string, held bool) {
	for _, pair := range idKeyedPairs {
		for i, array := range pair {
			byID := map[string]*jsonValue{}
			for _, entry := range a.at(array.path...).items {
				byID[entry.get("id").str()] = entry
			}
			for _, entry := range b.at(array.path...).items {
				if prior, ok := byID[entry.get("id").str()]; ok && !sameJSON(array.identity(prior), array.identity(entry)) {
					return "two " + array.name() + " entries whose identity members differ", entry.get("id").str(), true
				}
			}
			other := pair[1-i]
			for _, entry := range b.at(other.path...).items {
				if _, ok := byID[entry.get("id").str()]; ok {
					return "a " + pair[0].name() + " entry and a " + pair[1].name() + " entry", entry.get("id").str(), true
				}
			}
		}
	}
	return "", "", false
}

// unmatchedRootCode is the kind whose findings step 2 carries only where no analyzer reading the
// configured string matched it.
const unmatchedRootCode = "DS9004"

// matchedRoots is every configured string a DS9004 finding names that an input report reading it
// holds no DS9004 finding for.
func matchedRoots(inputs []mergeInput) map[string]bool {
	reported := make([]map[string]bool, len(inputs))
	var configured []string
	for i, in := range inputs {
		reported[i] = map[string]bool{}
		for _, f := range in.report.get("findings").items {
			if f.get("code").str() == unmatchedRootCode {
				reported[i][f.at("symbol", "ref").str()] = true
				configured = append(configured, f.at("symbol", "ref").str())
			}
		}
	}
	matched := map[string]bool{}
	for _, root := range configured {
		prefix, _, found := strings.Cut(root, "://")
		for i, in := range inputs {
			reads := !found || strings.ContainsAny(prefix, "*?")
			for _, language := range in.report.at("analyzer", "languages").items {
				reads = reads || language.str() == prefix
			}
			if reads && !reported[i][root] {
				matched[root] = true
			}
		}
	}
	return matched
}

// carried copies a record from an input report with the analyzer member naming that report,
// inserted before the member named before or at the end.
func carried(record *jsonValue, analyzer, before string) *jsonValue {
	out := record.clone()
	out.set("analyzer", jsonText(analyzer), before)
	return out
}

// promotion is one finding step 4 promoted, with every evaluation that carried it.
type promotion struct {
	finding     *jsonValue
	evaluations []referenceEvaluation
}

// resolveEvaluations runs step 4 over every dead evaluation: rule 1 in canonical order of the
// findings, rules 2 and 4 to a fixpoint over the components they drop, then rule 3. It returns
// the findings it promotes, in canonical order, and the components it drops, each with the
// state of the pair that dropped it.
func resolveEvaluations(evaluations []referenceEvaluation, trace mergeTrace) ([]promotion, map[string]string, error) {
	var dead []referenceEvaluation
	for _, e := range evaluations {
		if e.state == mergeStateDead {
			dead = append(dead, e)
		}
	}
	slices.SortStableFunc(dead, func(a, b referenceEvaluation) int {
		return findingKey(carried(a.record.get("finding"), a.analyzer, "details")).compare(findingKey(carried(b.record.get("finding"), b.analyzer, "details")))
	})
	dropped, droppedOn := map[string]string{}, map[string]string{}
	for _, e := range dead {
		if strongestOther(e, evaluations, dropped) == "" {
			trace.hit(traceUnresolvedEdge)
			return nil, nil, fmt.Errorf("%w: edge %s side %s", errUnresolvedEdge, e.edge, e.side)
		}
	}
	for grew := true; grew; {
		grew = false
		for _, e := range dead {
			if pair := strongestOther(e, evaluations, dropped); dropped[e.component()] == "" && pair != mergeStateDead {
				dropped[e.component()], droppedOn[e.component()] = pair, e.edge
				grew = true
			}
		}
	}
	var promoted []promotion
	for _, e := range dead {
		pair := dropped[e.component()]
		if pair == "" {
			trace.hit(tracePromotedDeadPair)
			finding := carried(e.record.get("finding"), e.analyzer, "details")
			if n := len(promoted); n > 0 && findingKey(promoted[n-1].finding).compare(findingKey(finding)) == 0 {
				trace.hit(tracePromotedOnce)
				promoted[n-1].evaluations = append(promoted[n-1].evaluations, e)
				continue
			}
			promoted = append(promoted, promotion{finding: finding, evaluations: []referenceEvaluation{e}})
		} else if e.edge != droppedOn[e.component()] && strongestOther(e, evaluations, nil) == mergeStateDead {
			trace.hit(traceDroppedBesideDeadPair + pair)
		}
	}
	return promoted, dropped, nil
}

// component is the identifier of the component a dead evaluation's finding names.
func (e referenceEvaluation) component() string {
	return e.record.at("finding", "component", "id").str()
}

// strongestOther is the strongest state the evaluations on the other sides of e's edge read as,
// a dead evaluation of a dropped component reading as live, or "" where no other side holds one.
func strongestOther(e referenceEvaluation, evaluations []referenceEvaluation, dropped map[string]string) string {
	strongest := ""
	for _, other := range evaluations {
		if other.edge != e.edge || other.side == e.side {
			continue
		}
		state := other.state
		if state == mergeStateDead && dropped[other.component()] != "" {
			state = mergeStateLive
		}
		if strongest == "" || stateStrength[state] < stateStrength[strongest] {
			strongest = state
		}
	}
	return strongest
}

// unionComponents joins the components of promoted findings on the other sides of one edge, in
// canonical order, the identifier the loop reaches first surviving however many edges a join
// spans, and rewrites every finding whose component the union named, its confidence lowered to
// the lowest of the joined component's roots that are not test-only. It returns the surviving
// identifier of every component the union reached.
func unionComponents(findings []*jsonValue, promoted []promotion, trace mergeTrace) map[string]string {
	parent := map[string]string{}
	reached := map[string]int{}
	var find func(id string) string
	find = func(id string) string {
		if p, ok := parent[id]; ok && p != id {
			return find(p)
		}
		return id
	}
	reach := func(id string) {
		if _, ok := parent[id]; !ok {
			parent[id] = id
			reached[id] = len(reached)
		}
	}
	paired := func(f, g promotion) bool {
		for _, e := range f.evaluations {
			for _, o := range g.evaluations {
				if e.edge == o.edge && e.side != o.side {
					return true
				}
			}
		}
		return false
	}
	for _, f := range promoted {
		id := f.finding.at("component", "id").str()
		reach(id)
		for _, g := range promoted {
			if !paired(f, g) {
				continue
			}
			other := g.finding.at("component", "id").str()
			reach(other)
			a, b := find(id), find(other)
			if reached[b] < reached[a] {
				a, b = b, a
			}
			if a != b {
				parent[b] = a
			}
		}
	}
	counts := map[string][2]int{}
	for _, f := range findings {
		id := f.at("component", "id").str()
		if _, ok := counts[id]; !ok {
			counts[id] = [2]int{f.at("component", "symbol_count").num(), f.at("component", "deletable_lines").num()}
		}
	}
	sums := map[string][2]int{}
	for id := range parent {
		root := find(id)
		sums[root] = [2]int{sums[root][0] + counts[id][0], sums[root][1] + counts[id][1]}
	}
	lowest := map[string]string{}
	for _, f := range findings {
		id := f.at("component", "id").str()
		if _, unioned := parent[id]; !unioned || !f.at("component", "root").truth() || f.get("test_only").truth() {
			continue
		}
		if held, ok := lowest[find(id)]; !ok || classRank[f.get("confidence").str()] > classRank[held] {
			lowest[find(id)] = f.get("confidence").str()
		}
	}
	for _, f := range findings {
		id := f.at("component", "id").str()
		if _, unioned := parent[id]; !unioned {
			continue
		}
		root := find(id)
		if cap, ok := lowest[root]; ok && classRank[cap] > classRank[f.get("confidence").str()] {
			trace.hit(traceJoinedConfidence)
			f.set("confidence", jsonText(cap), "")
		}
		component := f.get("component")
		component.set("id", jsonText(root), "")
		component.set("symbol_count", jsonNumberOf(sums[root][0]), "")
		component.set("deletable_lines", jsonNumberOf(sums[root][1]), "")
	}
	joined := make(map[string]string, len(parent))
	for id := range parent {
		joined[id] = find(id)
	}
	return joined
}

// listedComponents is step 6's components listing, nil where no input lists components: one
// entry per component the ordered findings name, in the order of the first finding naming it,
// holding every member an input lists under it or under a component the union joined into it,
// once each, ordered by path, line, column and reference.
func listedComponents(inputs []mergeInput, findings []*jsonValue, joined map[string]string) *jsonValue {
	lists := false
	members := map[string][]*jsonValue{}
	for _, in := range inputs {
		listing := in.report.get("components")
		if listing == nil {
			continue
		}
		lists = true
		for _, entry := range listing.items {
			id := entry.get("id").str()
			if root, ok := joined[id]; ok {
				id = root
			}
			members[id] = append(members[id], entry.get("members").items...)
		}
	}
	if !lists {
		return nil
	}
	out := jsonArrayOf()
	seen := map[string]bool{}
	for _, f := range findings {
		id := f.at("component", "id").str()
		held, ok := members[id]
		if !ok && f.get("code").str() == "DS9005" {
			held, ok = []*jsonValue{jsonObjectOf("ref", f.at("symbol", "ref"), "name", f.at("symbol", "name"), "position", f.get("position"))}, true
		}
		if !ok || seen[id] {
			continue
		}
		seen[id] = true
		slices.SortStableFunc(held, func(a, b *jsonValue) int {
			return cmp.Or(
				strings.Compare(a.at("position", "path").str(), b.at("position", "path").str()),
				cmp.Compare(a.at("position", "line").num(), b.at("position", "line").num()),
				cmp.Compare(a.at("position", "column").num(), b.at("position", "column").num()),
				strings.Compare(a.get("ref").str(), b.get("ref").str()),
				bytes.Compare(a.compactBytes(), b.compactBytes()),
			)
		})
		held = slices.CompactFunc(held, func(a, b *jsonValue) bool { return bytes.Equal(a.compactBytes(), b.compactBytes()) })
		out.items = append(out.items, jsonObjectOf("id", jsonText(id), "members", jsonArrayOf(held...)))
	}
	return out
}

// staleEdges runs step 5: one DS9005 per edge with a side whose every evaluation is absent.
func staleEdges(evaluations []referenceEvaluation, merger string, kinds map[string]kindRow) []*jsonValue {
	sides := map[string]map[string][]referenceEvaluation{}
	for _, e := range evaluations {
		if sides[e.edge] == nil {
			sides[e.edge] = map[string][]referenceEvaluation{}
		}
		sides[e.edge][e.side] = append(sides[e.edge][e.side], e)
	}
	row := kinds["DS9005"]
	var out []*jsonValue
	for _, edge := range slices.Sorted(maps.Keys(sides)) {
		var staleSides []string
		details := jsonArrayOf()
		for _, side := range []string{"provides", "used_by"} {
			held := sides[edge][side]
			if len(held) == 0 {
				continue
			}
			strongest := held[0].state
			for _, e := range held {
				if stateStrength[e.state] < stateStrength[strongest] {
					strongest = e.state
				}
			}
			if strongest == mergeStateAbsent {
				staleSides = append(staleSides, side)
			}
			details.items = append(details.items, jsonObjectOf(
				"side", jsonText(side), "symbol", jsonText(held[0].record.get("symbol").str()), "state", jsonText(strongest),
			))
		}
		if len(staleSides) == 0 {
			continue
		}
		symbol := sides[edge][staleSides[0]][0].record.get("symbol").str()
		message := "no analyzer enumerates the symbol either side of the edge names"
		if len(staleSides) == 1 {
			message = "no analyzer enumerates the symbol the " + staleSides[0] + " side of the edge names"
		}
		language, _, _ := strings.Cut(symbol, "://")
		out = append(out, jsonObjectOf(
			"code", jsonText(row.Code),
			"kind", jsonText(row.Name),
			"language", jsonText(language),
			"position", jsonObjectOf("path", jsonText("deadset-edges.json"), "line", jsonNumberOf(1), "column", jsonNumberOf(1), "end_line", jsonNumberOf(1)),
			"symbol", jsonObjectOf("ref", jsonText(symbol), "kind", jsonText("edge"), "name", jsonText(edge), "size_lines", jsonNumberOf(1)),
			"reachability_class", jsonText("certain"),
			"confidence", jsonText("certain"),
			"test_only", jsonBoolOf(false),
			"generated", jsonBoolOf(false),
			"component", jsonObjectOf("id", jsonText(fmt.Sprintf("%s/c-%d", merger, len(out)+1)), "root", jsonBoolOf(true), "symbol_count", jsonNumberOf(1), "deletable_lines", jsonNumberOf(0)),
			"retained_by", jsonArrayOf(),
			"configurations", jsonArrayOf(),
			"consumers_loaded", jsonArrayOf(),
			"fixability", jsonText(row.Fixability),
			"severity", jsonText(row.DefaultSeverity),
			"message", jsonText(message),
			"details", jsonObjectOf("edge", jsonText(edge), "sides", details),
		))
	}
	return out
}

// keptEvaluations are the live and absent evaluations as the merged report carries them, ordered by
// edge, side and analyzer, two records equal on all three by their compact encodings.
func keptEvaluations(evaluations []referenceEvaluation) *jsonValue {
	var kept []*jsonValue
	for _, e := range evaluations {
		if e.state != mergeStateDead {
			kept = append(kept, carried(e.record, e.analyzer, ""))
		}
	}
	slices.SortStableFunc(kept, func(a, b *jsonValue) int {
		return cmp.Or(
			strings.Compare(a.get("edge").str(), b.get("edge").str()),
			strings.Compare(a.get("side").str(), b.get("side").str()),
			strings.Compare(a.get("analyzer").str(), b.get("analyzer").str()),
			bytes.Compare(a.compactBytes(), b.compactBytes()),
		)
	})
	return jsonArrayOf(kept...)
}

func unionStrings(inputs []mergeInput, array string) *jsonValue {
	seen := map[string]bool{}
	for _, in := range inputs {
		for _, item := range in.report.get(array).items {
			seen[item.str()] = true
		}
	}
	return jsonStrings(slices.Sorted(maps.Keys(seen)))
}

// mergedEnvelope builds every member step 6 takes from the caller or derives from the inputs
// ahead of the record arrays, in report.schema.json's order.
func mergedEnvelope(inputs []mergeInput, accepted []string, caller mergeCaller, trace mergeTrace) *jsonValue {
	languages := map[string]bool{}
	for _, in := range inputs {
		for _, l := range in.report.at("analyzer", "languages").items {
			if languages[l.str()] {
				trace.hit(traceTwoAnalyzersOneLanguage)
			}
			languages[l.str()] = true
		}
	}
	conformance, err := decodeJSONValue(caller.Analyzer.Conformance)
	if err != nil {
		conformance = jsonText(err.Error())
	}
	claims, err := claimedKinds(inputs, nil)
	if err != nil {
		claims = map[string]string{err.Error(): ""}
	}
	target := mergedTarget(inputs, claims, contractLanguages(), trace)
	from := slices.Clone(inputs)
	slices.SortStableFunc(from, mergedFromOrder)
	mergedFrom := jsonArrayOf()
	for _, in := range from {
		mergedFrom.items = append(mergedFrom.items, jsonObjectOf(
			"name", jsonText(in.name), "version", jsonText(in.report.at("analyzer", "version").str()), "digest", jsonText(in.digest),
			"languages", in.report.at("analyzer", "languages"), "totals", in.report.at("totals"),
		))
	}
	loaded, unavailable := unionByID(inputs, trace, "consumers", "loaded"), unionByID(inputs, trace, "consumers", "unavailable")
	return jsonObjectOf(
		"schema_version", jsonText(caller.SchemaVersion),
		"contract_version", jsonText(caller.ContractVersion),
		"analyzer", jsonObjectOf(
			"name", jsonText(caller.Analyzer.Name),
			"version", jsonText(caller.Analyzer.Version),
			"languages", jsonStrings(slices.Sorted(maps.Keys(languages))),
			"schema_versions_accepted", jsonStrings(accepted),
			"conformance", conformance,
		),
		"merged_from", mergedFrom,
		"target", target,
		"configurations", unionByID(inputs, trace, "configurations"),
		"configurations_not_built", unionByID(inputs, trace, "configurations_not_built"),
		"consumers", jsonObjectOf(
			"declared", jsonNumberOf(len(loaded.items)+len(unavailable.items)),
			"loaded", loaded,
			"unavailable", unavailable,
		),
	)
}

// mergedFromOrder orders two inputs as merged_from orders their entries: by name.
func mergedFromOrder(a, b mergeInput) int { return strings.Compare(a.name, b.name) }

// unionByID carries one entry per id of the id-keyed array at path, ordered by id: the entry of
// the input that comes first in merged_from.
func unionByID(inputs []mergeInput, trace mergeTrace, path ...string) *jsonValue {
	type held struct {
		entry *jsonValue
		in    mergeInput
	}
	first := map[string]held{}
	for _, in := range inputs {
		for _, entry := range in.report.at(path...).items {
			id := entry.get("id").str()
			prior, ok := first[id]
			if ok && !sameJSON(prior.entry, entry) {
				trace.hit(traceFreeTextMerged + strings.Join(path, "."))
			}
			if !ok || mergedFromOrder(in, prior.in) < 0 {
				first[id] = held{entry: entry, in: in}
			}
		}
	}
	out := jsonArrayOf()
	for _, id := range slices.Sorted(maps.Keys(first)) {
		out.items = append(out.items, first[id].entry.clone())
	}
	return out
}

// mergedTotals recomputes the counts over the merged arrays, summing the two that count records
// no merged array holds.
func mergedTotals(inputs []mergeInput, findings, stale []*jsonValue, trace mergeTrace) *jsonValue {
	severities := map[string]int{}
	lines := map[string]int{}
	for _, f := range findings {
		severities[f.get("severity").str()]++
		if f.at("component", "root").str() == "true" {
			lines[f.at("component", "id").str()] = f.at("component", "deletable_lines").num()
		}
	}
	deletable := 0
	for _, n := range lines {
		deletable += n
	}
	inEffect, reasons := 0, 0
	withheld := map[string]int{}
	withholding := 0
	for _, in := range inputs {
		inEffect += in.report.at("totals", "suppressions_in_effect").num()
		reasons += in.report.at("totals", "reasons_recorded").num()
		held := 0
		for _, level := range withheldLevels {
			held += in.report.at("totals", "withheld", level).num()
			withheld[level] += in.report.at("totals", "withheld", level).num()
		}
		if held > 0 {
			withholding++
		}
	}
	if withholding > 1 {
		trace.hit(traceWithheldSummed)
	}
	return jsonObjectOf(
		"findings", jsonNumberOf(len(findings)),
		"by_severity", jsonObjectOf("allow", jsonNumberOf(severities["allow"]), "warn", jsonNumberOf(severities["warn"]), "deny", jsonNumberOf(severities["deny"])),
		"deletable_lines", jsonNumberOf(deletable),
		"suppressions_in_effect", jsonNumberOf(inEffect),
		"reasons_recorded", jsonNumberOf(reasons),
		"stale_suppressions", jsonNumberOf(len(stale)),
		"pending", jsonNumberOf(0),
		"omitted", jsonNumberOf(0),
		"withheld", jsonObjectOf("certain", jsonNumberOf(withheld["certain"]), "probable", jsonNumberOf(withheld["probable"]), "possible", jsonNumberOf(withheld["possible"])),
	)
}

// withheldLevels are the confidence levels totals.withheld counts, in the order the
// report schema lists them.
var withheldLevels = []string{"certain", "probable", "possible"}

// loadReferenceCase reads one case as the reference merge takes it: the input reports in
// file-name order, the accepted versions and the caller's facts.
func loadReferenceCase(fsys fs.FS, root, dir string) ([]mergeInput, []string, mergeCaller, error) {
	var caller mergeCaller
	data, held, err := mergeReadFile(fsys, root, dir, mergeCallerFile)
	switch {
	case err != nil:
		return nil, nil, caller, err
	case !held:
		return nil, nil, caller, fmt.Errorf("the case holds no %s, want the facts the caller supplies", mergeCallerFile)
	}
	if err := corpus.DecodeStrict(data, &caller); err != nil {
		return nil, nil, caller, fmt.Errorf("decoding %s: %w", mergeCallerFile, err)
	}
	if _, ranked := severityRank[caller.FailOn]; !ranked {
		return nil, nil, caller, fmt.Errorf("%s fail_on = %q, want allow, warn or deny", mergeCallerFile, caller.FailOn)
	}
	accepted, _, err := mergeReadFile(fsys, root, dir, mergeAcceptedFile)
	if err != nil {
		return nil, nil, caller, err
	}
	at := root + "/" + dir + "/" + mergeInputsDir
	entries, err := fs.ReadDir(fsys, at)
	if err != nil {
		return nil, nil, caller, err
	}
	var inputs []mergeInput
	for _, entry := range entries {
		raw, err := fs.ReadFile(fsys, at+"/"+entry.Name())
		if err != nil {
			return nil, nil, caller, err
		}
		report, err := decodeJSONValue(raw)
		if err != nil {
			return nil, nil, caller, fmt.Errorf("decoding %s/%s: %w", mergeInputsDir, entry.Name(), err)
		}
		inputs = append(inputs, mergeInput{report: report, file: entry.Name(), name: report.at("analyzer", "name").str(), digest: caller.Digests[entry.Name()]})
	}
	files := make([]string, 0, len(inputs))
	for _, in := range inputs {
		files = append(files, in.file)
	}
	if named := slices.Sorted(maps.Keys(caller.Digests)); !slices.Equal(named, files) {
		return nil, nil, caller, fmt.Errorf("%s digests name %q, want one digest per input report %q", mergeCallerFile, named, files)
	}
	return inputs, mergeVersion.FindAllString(string(accepted), -1), caller, nil
}

// firstDifference names the first line at which two documents differ.
func firstDifference(got, want []byte) string {
	gotLines, wantLines := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	for i := range max(len(gotLines), len(wantLines)) {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			return fmt.Sprintf("line %d is %q, want %q", i+1, g, w)
		}
	}
	return "the documents are equal"
}

// mergeReproductionProblems runs the reference merge over one case and names every way its
// result departs from what the case expects.
func mergeReproductionProblems(fsys fs.FS, root, dir string, kinds map[string]kindRow, trace mergeTrace) []string {
	inputs, accepted, caller, err := loadReferenceCase(fsys, root, dir)
	if err != nil {
		return []string{err.Error()}
	}
	c, err := loadMergeCase(fsys, root, dir)
	if err != nil {
		return []string{err.Error()}
	}
	report, exit, mergeErr := referenceMerge(inputs, accepted, caller, kinds, trace)
	if exit != c.exit {
		return []string{fmt.Sprintf("the merge returns %d (%v), want %s = %d", exit, mergeErr, mergeExitFile, c.exit)}
	}
	if report == nil {
		return nil
	}
	got, err := report.render()
	if err != nil {
		return []string{fmt.Sprintf("rendering the merged report: %v", err)}
	}
	want, _, err := mergeReadFile(fsys, root, dir, mergeExpectedFile)
	if err != nil {
		return []string{err.Error()}
	}
	if !bytes.Equal(got, want) {
		return []string{fmt.Sprintf("the merged report differs from %s: %s", mergeExpectedFile, firstDifference(got, want))}
	}
	return nil
}

func kindsByCode(t *testing.T) map[string]kindRow {
	t.Helper()
	out := map[string]kindRow{}
	for _, k := range loadKinds(t).Kinds {
		out[k.Code] = k
	}
	return out
}

// copiedMergeCase copies one published case into memory, so a planted change exercises the
// reproduction against a real case.
func copiedMergeCase(t *testing.T, dir string) fstest.MapFS {
	t.Helper()
	m := fstest.MapFS{}
	at := mergeVectorsDir + "/" + dir
	err := fs.WalkDir(spec.Vectors, at, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(spec.Vectors, p)
		m[p] = &fstest.MapFile{Data: data}
		return err
	})
	if err != nil {
		t.Fatalf("Setup: copying %s: %v", at, err)
	}
	return m
}

func TestMergeReproductionRefuses(t *testing.T) {
	cases := []struct {
		mutate  func(t *testing.T, m fstest.MapFS, at string)
		name    string
		dir     string
		wantMsg string
	}{
		{
			name: "a_digest_the_caller_does_not_give",
			dir:  "two-reports-no-edges",
			mutate: func(t *testing.T, m fstest.MapFS, at string) {
				t.Helper()
				replaceIn(t, m, at+"/"+mergeCallerFile, "sha256:ca44", "sha256:da44")
			},
			wantMsg: "the merged report differs from expected.json",
		},
		{
			name: "a_line_separator_written_raw",
			dir:  "line-separators-escaped",
			mutate: func(t *testing.T, m fstest.MapFS, at string) {
				t.Helper()
				replaceIn(t, m, at+"/"+mergeExpectedFile, `\u2028`, "\u2028")
			},
			wantMsg: "the merged report differs from expected.json",
		},
		{
			name: "two_kinds_merged_as_the_first_reports",
			dir:  "target-kinds-of-two-languages",
			mutate: func(t *testing.T, m fstest.MapFS, at string) {
				t.Helper()
				replaceIn(t, m, at+"/"+mergeExpectedFile, "\"kind\": \"mixed\",\n    \"language_kinds\": {\n      \"go\": \"application\",\n      \"ts\": \"library\"\n    },", "\"kind\": \"application\",")
			},
			wantMsg: "the merged report differs from expected.json",
		},
		{
			name: "two_kinds_claimed_for_one_language_admitted",
			dir:  "target-kind-claims-differ",
			mutate: func(t *testing.T, m fstest.MapFS, at string) {
				t.Helper()
				replaceIn(t, m, at+"/inputs/01-go.json", "\"kind\": \"library\"", "\"kind\": \"application\"")
			},
			wantMsg: "want expected_exit = 3",
		},
		{
			name: "language_kinds_naming_one_kind",
			dir:  "target-kinds-in-one-report",
			mutate: func(t *testing.T, m fstest.MapFS, at string) {
				t.Helper()
				replaceIn(t, m, at+"/inputs/00-go.json", "\"ts\": \"library\"", "\"ts\": \"application\"")
			},
			wantMsg: "want expected_exit = 1",
		},
		{
			name: "no_caller_facts",
			dir:  "one-report",
			mutate: func(_ *testing.T, m fstest.MapFS, at string) {
				delete(m, at+"/"+mergeCallerFile)
			},
			wantMsg: "the case holds no caller.json",
		},
		{
			name: "a_digest_for_no_input",
			dir:  "one-report",
			mutate: func(t *testing.T, m fstest.MapFS, at string) {
				t.Helper()
				replaceIn(t, m, at+"/"+mergeCallerFile, `"00-go.json"`, `"00-ts.json"`)
			},
			wantMsg: "want one digest per input report",
		},
		{
			name: "a_verdict_the_findings_do_not_give",
			dir:  "one-report",
			mutate: func(t *testing.T, m fstest.MapFS, at string) {
				t.Helper()
				replaceIn(t, m, at+"/"+mergeExitFile, "1", "0")
			},
			wantMsg: "the merge returns 1 (<nil>), want expected_exit = 0",
		},
	}
	kinds := kindsByCode(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := copiedMergeCase(t, tc.dir)
			at := mergeVectorsDir + "/" + tc.dir
			if problems := mergeReproductionProblems(m, mergeVectorsDir, tc.dir, kinds, nil); len(problems) != 0 {
				t.Fatalf("Setup: mergeReproductionProblems(%s unchanged) = %q, want none", tc.dir, problems)
			}
			tc.mutate(t, m, at)
			problems := mergeReproductionProblems(m, mergeVectorsDir, tc.dir, kinds, nil)
			if !slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, tc.wantMsg) }) {
				t.Errorf("mergeReproductionProblems(%s with %s) = %q, want a problem containing %q", tc.dir, tc.name, problems, tc.wantMsg)
			}
		})
	}
}

// replaceIn plants one change in one file of an in-memory case, failing when the text it
// replaces is absent.
func replaceIn(t *testing.T, m fstest.MapFS, p, old, replacement string) {
	t.Helper()
	file, held := m[p]
	if !held || !bytes.Contains(file.Data, []byte(old)) {
		t.Fatalf("Setup: %s does not hold %q, want the text the change replaces", p, old)
	}
	file.Data = bytes.Replace(file.Data, []byte(old), []byte(replacement), 1)
}
