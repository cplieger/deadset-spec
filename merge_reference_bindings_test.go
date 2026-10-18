package spec_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"slices"
	"strings"

	"github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/report"
)

// This file is the binding pass of contract/grammar/merge.md, step 4 "Bindings.", and the facts
// admission of step 1, beside the edge resolution merge_reference_test.go holds.

// The trace points of the binding pass and of the facts admission.
const (
	traceFactsAnalyzer       = "a facts document naming another analyzer or version than its report"
	traceFactsLanguage       = "a facts document about a language its report does not name"
	traceFactsFinding        = "a facts finding naming a component or a symbol a finding of its report names"
	traceKindNotClaimed      = "a report language whose row takes no target kind, its kind not claimed"
	traceKindlessTarget      = "a merge of languages taking no target kind, its target without a kind"
	traceBindingNeutral      = "a dead schema record no binding binds, dropped"
	traceBindingLive         = "a dead schema record a live binding keeps, dropped"
	traceBindingStateless    = "a dead schema record a binding with no state keeps, dropped"
	traceBindingUser         = "a dead schema record a live schema record uses, dropped"
	traceBindingMember       = "a dead schema record a live member keeps, dropped"
	traceBindingPromoted     = "a dead schema record every binding of which reads unreferenced, promoted"
	traceBindingCap          = "a promoted schema finding a binding's cap lowers to possible"
	traceBindingCapCertain   = "a promoted certain schema finding a binding's cap leaves certain"
	traceBindingLibrary      = "a promoted schema finding a library binding language lowers to possible"
	traceViaUnreferenced     = "a via consumer whose implementers stay dead, read unreferenced"
	traceImplementerNoSchema = "a dead implementer no schema record matches, dropped"
	traceImplementerVia      = "a dead implementer another implementer it names keeps, dropped"
	traceImplementerSchema   = "a dead implementer a live schema record keeps, dropped"
	traceImplementerPromoted = "a dead implementer of a promoted schema record, promoted"
	traceMemberFolds         = "a promoted schema finding joined into its promoted container, root false"
	traceCarriedShared       = "a finding two implementers carry, promoted once"
	traceCarriedOneComponent = "a finding two records of one component carry, counted once"
	traceBindingWithheld     = "a schema finding a binding lowers below the caller's minimum, withheld"
	traceWithheldKeptLive    = "a finding a withheld schema record keeps from promotion, counted withheld"
	traceBlockedDebited      = "a finding a dropped component carries, taken from the counts of a kept carrier"
	traceBlockedAcross       = "a finding a dropped component of the other relation carries, not promoted"
	traceBindingAndEdge      = "a finding a dead evaluation and a dead binding record carry, promoted once"
	traceUnboundNote         = "an unbound-schema note for a schema file no binding reaches"
	traceUnsupportedNote     = "a binding-unsupported note for a generated file of an unsupported row"
)

const (
	roleSchema      = "schema"
	roleConsumer    = "consumer"
	roleImplementer = "implementer"
	roleFallback    = "fallback"
	stateUnread     = "unreferenced"

	generatorsPath = "contract/generators.json"
	factsSuffix    = ".facts.json"

	unboundSchemaMessage      = "no analyzed language binds a declaration of this schema file; proto.plugins names the generators that do"
	bindingUnsupportedMessage = "the generator of this file is outside every supported version or option set, so no declaration of it binds"
)

// referenceBinding is one binding record of the working set, with the analyzer that carried it.
type referenceBinding struct {
	record   *jsonValue
	role     string
	symbol   string
	file     string
	schema   string
	state    string
	analyzer string
	library  bool // the binding's language is a library under its report's target kind
}

func (b referenceBinding) component() string { return b.record.at("finding", "component", "id").str() }

func (b referenceBinding) list(member string) []string {
	var out []string
	held := b.record.get(member)
	if held == nil {
		return nil
	}
	for _, item := range held.items {
		out = append(out, item.str())
	}
	return out
}

// sameSchemaFile is the merge's file rule: equal, or one the other preceded by whole segments.
// values is the array member's values, nil where the record carries none.
func (b referenceBinding) values(member string) []*jsonValue {
	if held := b.record.get(member); held != nil {
		return held.items
	}
	return nil
}

func sameSchemaFile(a, b string) bool {
	return a == b || strings.HasSuffix(a, "/"+b) || strings.HasSuffix(b, "/"+a)
}

// bindingsOf is every binding record of every input's facts document.
func bindingsOf(inputs []mergeInput) []referenceBinding {
	var out []referenceBinding
	for _, in := range inputs {
		if in.facts == nil {
			continue
		}
		kind := in.report.at("target", "kind").str()
		if kind == "mixed" {
			kind = in.report.at("target", "language_kinds", in.facts.get("language").str()).str()
		}
		for _, r := range in.facts.get("bindings").items {
			out = append(out, referenceBinding{
				record: r, role: r.get("role").str(), symbol: r.get("symbol").str(), file: r.get("schema_file").str(),
				schema: r.get("schema").str(), state: r.get("state").str(), analyzer: in.name, library: kind == "library",
			})
		}
	}
	return out
}

// bindingPass holds the working set's binding records and reads them against the components
// the fixpoint has dropped.
type bindingPass struct {
	dropped map[string]string
	all     []referenceBinding
	minimum string            // the caller's min_confidence, "" for none
	blocked map[mergeKey]bool // every finding a dropped component carries
	debits  map[string][2]int // per kept component, what unpromoted takes from its counts
}

func (p *bindingPass) dead(role string) []referenceBinding {
	var out []referenceBinding
	for _, b := range p.all {
		if b.role == role && b.state == mergeStateDead {
			out = append(out, b)
		}
	}
	return out
}

// reads is the state a record reads as.
func (p *bindingPass) reads(b referenceBinding) string {
	switch {
	case b.role == roleFallback, b.state == "":
		return mergeStateLive
	case b.state == mergeStateDead && p.dropped[b.component()] != "":
		return mergeStateLive
	case b.role == roleConsumer && b.record.get("via") != nil:
		for _, name := range b.list("via") {
			implementer, held := p.implementer(b.analyzer, name)
			if !held || p.reads(implementer) == mergeStateLive {
				return mergeStateLive
			}
		}
		return stateUnread
	}
	return b.state
}

func (p *bindingPass) implementer(analyzer, symbol string) (referenceBinding, bool) {
	for _, b := range p.all {
		if b.role == roleImplementer && b.analyzer == analyzer && b.symbol == symbol {
			return b, true
		}
	}
	return referenceBinding{}, false
}

// bindings is a schema record's bindings: the consumers that match it and the fallbacks whose
// file matches and whose scope covers its finding's kind.
func (p *bindingPass) bindings(s referenceBinding) []referenceBinding {
	kind := s.record.at("finding", "symbol", "kind").str()
	var out []referenceBinding
	for _, b := range p.all {
		switch {
		case b.role == roleConsumer && b.schema == s.schema && sameSchemaFile(b.file, s.file):
		case b.role == roleFallback && sameSchemaFile(b.file, s.file) &&
			(b.record.get("scope").str() == "all" || kind != "service" && kind != "rpc"):
		default:
			continue
		}
		out = append(out, b)
	}
	return out
}

// schemaRecords is every schema record matching a reference and a file.
func (p *bindingPass) schemaRecords(schema, file string) []referenceBinding {
	var out []referenceBinding
	for _, b := range p.all {
		if b.role == roleSchema && b.schema == schema && sameSchemaFile(b.file, file) {
			out = append(out, b)
		}
	}
	return out
}

// decideSchema is the trace point of the rule that drops a dead schema record, or "" where
// none does.
func (p *bindingPass) decideSchema(s referenceBinding) string {
	bindings := p.bindings(s)
	if len(bindings) == 0 {
		return traceBindingNeutral
	}
	for _, b := range bindings {
		if p.reads(b) == mergeStateLive {
			if b.role != roleFallback && b.state == "" {
				return traceBindingStateless
			}
			return traceBindingLive
		}
	}
	for _, r := range p.all {
		if r.role == roleSchema && slices.Contains(r.list("uses"), s.schema) && p.reads(r) == mergeStateLive {
			return traceBindingUser
		}
	}
	for _, member := range s.list("members") {
		for _, r := range p.schemaRecords(member, s.file) {
			if p.reads(r) == mergeStateLive {
				return traceBindingMember
			}
		}
	}
	return ""
}

// decideImplementer is the trace point of the rule that drops a dead implementer, or "".
func (p *bindingPass) decideImplementer(i referenceBinding) string {
	schemas := p.schemaRecords(i.schema, i.file)
	if len(schemas) == 0 {
		return traceImplementerNoSchema
	}
	for _, s := range schemas {
		if p.reads(s) == mergeStateLive {
			return traceImplementerSchema
		}
	}
	for _, name := range i.list("via") {
		if other, held := p.implementer(i.analyzer, name); held && p.reads(other) == mergeStateLive {
			return traceImplementerVia
		}
	}
	return ""
}

// drop runs one pass of the binding rules over the dead records whose components are not
// dropped, reporting whether it dropped one.
func (p *bindingPass) drop(trace mergeTrace) bool {
	grew := false
	for _, s := range p.dead(roleSchema) {
		if p.dropped[s.component()] != "" {
			continue
		}
		if point := p.decideSchema(s); point != "" {
			trace.hit(point)
			p.dropped[s.component()] = droppedByBinding
			grew = true
		}
	}
	for _, i := range p.dead(roleImplementer) {
		if p.dropped[i.component()] != "" {
			continue
		}
		if point := p.decideImplementer(i); point != "" {
			trace.hit(point)
			p.dropped[i.component()] = droppedByBinding
			grew = true
		}
	}
	return grew
}

// promotedClass is the confidence rule 3 promotes a dead schema record's finding at, with the
// trace point of each lowering.
func (p *bindingPass) promotedClass(s referenceBinding) (string, []string) {
	class := s.record.at("finding", "confidence").str()
	var points []string
	lower := false
	if slices.ContainsFunc(p.bindings(s), func(b referenceBinding) bool { return b.record.get("cap") != nil }) {
		if class == "certain" {
			points = append(points, traceBindingCapCertain)
		} else {
			points = append(points, traceBindingCap)
			lower = true
		}
	}
	if slices.ContainsFunc(p.bindings(s), func(b referenceBinding) bool { return b.library }) {
		points = append(points, traceBindingLibrary)
		lower = true
	}
	if lower && classRank[class] < classRank["possible"] {
		class = "possible"
	}
	return class, points
}

// withhold drops, before the first pass, every dead schema record whose finding rule 3 would
// promote below the caller's minimum.
func (p *bindingPass) withhold(trace mergeTrace) {
	if p.minimum == "" {
		return
	}
	for _, s := range p.dead(roleSchema) {
		if class, _ := p.promotedClass(s); classRank[class] > classRank[p.minimum] {
			trace.hit(traceBindingWithheld)
			p.dropped[s.component()] = droppedByBinding
		}
	}
}

// blockedKeys is the key of every finding a dropped component carries, in either relation.
func (p *bindingPass) blockedKeys(dead []referenceEvaluation) map[mergeKey]bool {
	keys := map[mergeKey]bool{}
	for _, e := range dead {
		if p.dropped[e.component()] != "" {
			keys[findingKey(carried(e.record.get("finding"), e.analyzer, "details"))] = true
		}
	}
	for _, b := range p.all {
		if b.state != mergeStateDead || (b.role != roleSchema && b.role != roleImplementer) || p.dropped[b.component()] == "" {
			continue
		}
		keys[findingKey(carried(b.record.get("finding"), b.analyzer, "details"))] = true
		for _, c := range b.values("carried") {
			keys[findingKey(carried(c, b.analyzer, "details"))] = true
		}
	}
	return keys
}

// unpromoted is, per component no pass dropped, the debit of the findings it carries that blocked
// holds: a dropped component carries each of them too, so it stays live. The debit is one symbol
// per such finding and the distinct lines they span that no other finding of the component spans;
// held is the carried report findings of no dropped component.
func (p *bindingPass) unpromoted(held []*jsonValue, evaluations []referenceEvaluation, trace mergeTrace) map[string][2]int {
	type carriedBy struct {
		key mergeKey
		id  string
	}
	debits := map[string][2]int{}
	all, kept := map[string][]report.Position{}, map[string][]report.Position{}
	seen := map[carriedBy]bool{}
	hold := func(id string, f *jsonValue) {
		at := carriedBy{key: findingKey(f), id: id}
		if p.dropped[id] != "" || seen[at] {
			return
		}
		seen[at] = true
		position := positionOf(f)
		all[id] = append(all[id], position)
		if !p.blocked[at.key] {
			kept[id] = append(kept[id], position)
			return
		}
		trace.hit(traceBlockedDebited)
		debits[id] = [2]int{debits[id][0] + 1}
	}
	for _, f := range held {
		hold(f.at("component", "id").str(), f)
	}
	for _, e := range evaluations {
		if e.state == mergeStateDead {
			hold(e.component(), carried(e.record.get("finding"), e.analyzer, "details"))
		}
	}
	for _, b := range p.all {
		if b.state != mergeStateDead || (b.role != roleSchema && b.role != roleImplementer) {
			continue
		}
		hold(b.component(), carried(b.record.get("finding"), b.analyzer, "details"))
		for _, c := range b.values("carried") {
			hold(b.component(), carried(c, b.analyzer, "details"))
		}
	}
	for id, debit := range debits {
		debits[id] = [2]int{debit[0], report.DistinctLines(all[id]) - report.DistinctLines(kept[id])}
	}
	return debits
}

func positionOf(f *jsonValue) report.Position {
	return report.Position{
		Path: f.at("position", "path").str(), Line: f.at("position", "line").num(),
		Column: f.at("position", "column").num(), EndLine: f.at("position", "end_line").num(),
	}
}

// carrier is one finding a dead record carries, the component carrying it, and the record.
type carrier struct {
	finding *jsonValue
	record  referenceBinding
	carried bool
}

// carriers is every finding the dead binding records carry, each copied with its analyzer, a
// schema record's at the confidence rule 3 promotes it at.
func (p *bindingPass) carriers(trace mergeTrace) []carrier {
	var out []carrier
	for _, s := range p.dead(roleSchema) {
		f := carried(s.record.get("finding"), s.analyzer, "details")
		class, points := p.promotedClass(s)
		f.set("confidence", jsonText(class), "")
		if p.dropped[s.component()] == "" {
			trace.hit(traceBindingPromoted)
			for _, point := range points {
				trace.hit(point)
			}
			for _, b := range p.bindings(s) {
				if b.record.get("via") != nil && p.reads(b) == stateUnread {
					trace.hit(traceViaUnreferenced)
				}
			}
		}
		out = append(out, carrier{finding: f, record: s})
	}
	for _, i := range p.dead(roleImplementer) {
		if p.dropped[i.component()] == "" {
			trace.hit(traceImplementerPromoted)
		}
		out = append(out, carrier{finding: carried(i.record.get("finding"), i.analyzer, "details"), record: i})
		for _, c := range i.values("carried") {
			out = append(out, carrier{finding: carried(c, i.analyzer, "details"), record: i, carried: true})
		}
	}
	return out
}

// joins is the components a promoted binding finding joins before the edge joins, and whether
// it joins as a member of its container.
func (p *bindingPass) joins(c carrier, promoted map[string]bool, trace mergeTrace) (ids []string, member bool) {
	if c.carried {
		return nil, false
	}
	switch c.record.role {
	case roleImplementer:
		for _, s := range p.schemaRecords(c.record.schema, c.record.file) {
			if s.state == mergeStateDead && promoted[s.component()] {
				ids = append(ids, s.component())
			}
		}
	case roleSchema:
		for _, b := range p.bindings(c.record) {
			if b.record.get("via") == nil || p.reads(b) != stateUnread {
				continue
			}
			for _, name := range b.list("via") {
				if i, held := p.implementer(b.analyzer, name); held && i.state == mergeStateDead && promoted[i.component()] {
					ids = append(ids, i.component())
				}
			}
		}
		for _, container := range p.all {
			if container.role == roleSchema && container.state == mergeStateDead && promoted[container.component()] &&
				slices.Contains(container.list("members"), c.record.schema) && sameSchemaFile(container.file, c.record.file) {
				trace.hit(traceMemberFolds)
				ids, member = append(ids, container.component()), true
			}
		}
	}
	return ids, member
}

// mergeNotes is the notes step 6 has the merge write: unbound-schema and binding-unsupported.
func mergeNotes(inputs []mergeInput, p *bindingPass, trace mergeTrace) []*jsonValue {
	bound := map[string]bool{}
	var files []string
	for _, s := range p.dead(roleSchema) {
		path := s.record.at("finding", "position", "path").str()
		if _, seen := bound[path]; !seen {
			files = append(files, path)
			bound[path] = false
		}
		bound[path] = bound[path] || len(p.bindings(s)) > 0
	}
	var out []*jsonValue
	for _, path := range files {
		if !bound[path] {
			trace.hit(traceUnboundNote)
			out = append(out, jsonObjectOf("kind", jsonText("unbound-schema"), "path", jsonText(path), "key", jsonText("proto.plugins"), "message", jsonText(unboundSchemaMessage)))
		}
	}
	unsupported := unsupportedRows()
	for _, in := range inputs {
		if in.facts == nil {
			continue
		}
		for _, g := range in.facts.get("generated").items {
			if unsupported[g.at("generator", "row").str()] {
				trace.hit(traceUnsupportedNote)
				out = append(out, jsonObjectOf("kind", jsonText("binding-unsupported"), "path", g.get("path").clone(), "message", jsonText(bindingUnsupportedMessage)))
			}
		}
	}
	return out
}

// unsupportedRows is the ids of the generators.json rows whose cell is unsupported.
func unsupportedRows() map[string]bool {
	var doc struct {
		Generators []struct {
			ID   string `json:"id"`
			Cell string `json:"cell"`
		} `json:"generators"`
	}
	out := map[string]bool{}
	data, err := fs.ReadFile(spec.Contract, generatorsPath)
	if err != nil || json.Unmarshal(data, &doc) != nil {
		return out
	}
	for _, row := range doc.Generators {
		if row.Cell == "unsupported" {
			out[row.ID] = true
		}
	}
	return out
}

// kindTakingLanguages is the languages whose row of contract/languages.json takes a target kind.
func kindTakingLanguages() map[string]bool {
	var doc struct {
		Languages []struct {
			Language        string `json:"language"`
			TakesTargetKind bool   `json:"takes_target_kind"`
		} `json:"languages"`
	}
	out := map[string]bool{}
	data, err := fs.ReadFile(spec.Contract, languagesPath)
	if err != nil || json.Unmarshal(data, &doc) != nil {
		return out
	}
	for _, row := range doc.Languages {
		out[row.Language] = row.TakesTargetKind
	}
	return out
}

// admitFacts runs admission rules 8 to 10 over one input and its facts document.
func admitFacts(in mergeInput, trace mergeTrace) error {
	if in.facts == nil {
		return nil
	}
	for _, member := range []string{"name", "version"} {
		if a, b := in.facts.at("analyzer", member).str(), in.report.at("analyzer", member).str(); a != b {
			trace.hit(traceFactsAnalyzer)
			return fmt.Errorf("%s: its facts document names the analyzer %s %q, the report %q", in.file, member, a, b)
		}
	}
	language := in.facts.get("language").str()
	if !slices.ContainsFunc(in.report.at("analyzer", "languages").items, func(l *jsonValue) bool { return l.str() == language }) {
		trace.hit(traceFactsLanguage)
		return fmt.Errorf("%s: its facts document is about %s, which the report does not name", in.file, language)
	}
	reported := map[string]bool{}
	for _, f := range in.report.get("findings").items {
		reported[f.at("component", "id").str()], reported[f.at("symbol", "ref").str()] = true, true
	}
	for _, b := range in.facts.get("bindings").items {
		var embedded []*jsonValue
		if c := b.get("carried"); c != nil {
			embedded = append(embedded, c.items...)
		}
		if f := b.get("finding"); f != nil {
			embedded = append(embedded, f)
		}
		for _, f := range embedded {
			if id, ref := f.at("component", "id").str(), f.at("symbol", "ref").str(); reported[id] || reported[ref] {
				trace.hit(traceFactsFinding)
				return fmt.Errorf("%s: its facts document carries a finding of %s %s, which a finding of the report names", in.file, id, ref)
			}
		}
	}
	return nil
}

// droppedByBinding is the reason the dropped set records for a component a binding rule drops.
const droppedByBinding = "binding"

// promoteBindings adds the findings of the dead binding records whose components are not
// dropped to the edge promotions: once per canonical key, whichever records carry it, and a
// finding only where no dropped component of either relation carries it.
func promoteBindings(promoted []promotion, pass *bindingPass, trace mergeTrace) []promotion {
	groups := map[mergeKey][]carrier{}
	var keys []mergeKey
	for _, c := range pass.carriers(trace) {
		key := findingKey(c.finding)
		if _, seen := groups[key]; !seen {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], c)
	}
	for _, key := range keys {
		group := groups[key]
		if pass.blocked[key] {
			if !slices.ContainsFunc(group, func(c carrier) bool { return pass.dropped[c.record.component()] != "" }) {
				trace.hit(traceBlockedAcross)
			}
			continue
		}
		if len(group) > 1 && group[0].carried {
			trace.hit(traceCarriedShared)
		}
		if i := slices.IndexFunc(promoted, func(p promotion) bool { return findingKey(p.finding).compare(key) == 0 }); i >= 0 {
			trace.hit(traceBindingAndEdge)
			promoted[i].records = append(promoted[i].records, group...)
			continue
		}
		promoted = append(promoted, promotion{finding: group[0].finding, records: group})
	}
	slices.SortStableFunc(promoted, func(a, b promotion) int { return findingKey(a.finding).compare(findingKey(b.finding)) })
	return promoted
}
