package spec_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec"
)

const (
	findingSchemaPath    = "contract/finding.schema.json"
	findingSchemaDialect = "https://json-schema.org/draft/2020-12/schema"

	// findingPathDefPath is the definition every path of a finding takes: the
	// finding's own position, a write position, and the path a suppression
	// record names.
	findingPathDefPath = "$defs/relative_path"
)

// findingFields are the fields of a finding, in the order the schema declares
// them, which the schema's description makes the order of the object.
var findingFields = []string{
	"code", "kind", "language", "position", "symbol", "reachability_class",
	"confidence", "liveness_relation", "test_only", "generated", "component",
	"retained_by", "configurations", "consumers_loaded", "fixability",
	"severity", "message", "analyzer", "details",
}

// findingOptionalFields are the fields a finding may omit. An analyzer's own
// report states the analyzer once in the envelope, so that field is present
// only on a finding a merged report carries; a finding no relation decided
// carries no liveness relation, and the two halves of that rule are
// findingNoRelationKinds and findingLiveSubjectCodes below.
var findingOptionalFields = []string{"analyzer", "liveness_relation"}

// findingNoRelationKinds are the symbol kinds that are not declarations, so a
// finding about one carries no liveness relation. Five are artifacts the run
// read rather than symbols it swept: a source file, a dependency, a module
// directive, a suppression record and a configured root. Six are parts of the
// declaration the finding's symbol.ref names: a parameter, a receiver, a
// result, a statement, a case and a store, each decided inside its declaration
// rather than by a relation over the reference graph. The declared
// cross-language edge is not here: a merge emits that finding from the edge's
// evaluations and it carries a relation, which the example and the first merge
// vector pin.
var findingNoRelationKinds = []string{
	"case", "dependency", "file", "module-directive", "parameter", "receiver",
	"result", "root", "statement", "store", "suppression",
}

// findingLiveSubjectCodes are the codes whose subject the analysis holds live,
// so their findings carry no liveness relation: the three narrowing kinds
// report a symbol its own package references, the write-only kind reports a
// symbol production code writes, and the unused-assertion kind reports an
// assertion the program declares. A kind added to the vocabulary whose subject
// is live fails the test below until it is named here.
var findingLiveSubjectCodes = []string{"DS1101", "DS1102", "DS1104", "DS1204", "DS1301"}

// findingCodeInText finds a code inside a line of a published page, where the
// anchored shape of a code cannot be used.
var findingCodeInText = regexp.MustCompile(`DS[0-9]{4}`)

// findingKindColumn is the kind column of a text line, as text-line.md's
// published expression spells it: a symbol kind outside this shape cannot be
// rendered by a reporter.
var findingKindColumn = regexp.MustCompile(`^[a-z][a-z-]*$`)

// findingLinePattern reads the kind out of a rendered text line, by the
// leftmost split text-line.md fixes: the shortest prefix followed by a line, a
// column and a space is the path, and the token after it is the symbol kind.
var findingLinePattern = regexp.MustCompile(`^[^\r\n]+?:[1-9][0-9]*:[1-9][0-9]*: ([a-z][a-z-]*) `)

// findingDetailsBranch is one discriminated branch of the details object: the
// codes it names and the details fields those codes carry.
type findingDetailsBranch struct {
	codes  []string
	fields []string
}

// findingDetailsBranches are the branches the schema declares, in order. Each
// set of codes carries its fields and no other code may.
var findingDetailsBranches = []findingDetailsBranch{
	{codes: []string{"DS1101", "DS1102", "DS1104"}, fields: []string{"narrower_visibility"}},
	{codes: []string{"DS1201", "DS1203", "DS1204"}, fields: []string{"implementations"}},
	{codes: []string{"DS1301", "DS1807"}, fields: []string{"write_positions"}},
	{codes: []string{"DS1501"}, fields: []string{"excluded_by"}},
	{codes: []string{"DS1601"}, fields: []string{"dependency_class"}},
	{codes: []string{"DS1605"}, fields: []string{"replacement"}},
	{codes: []string{"DS1701", "DS1702", "DS1703"}, fields: []string{"mechanism", "entry"}},
	{codes: []string{"DS1705"}, fields: []string{"edge", "sides"}},
	{codes: []string{"DS1801", "DS1802", "DS1803", "DS1805", "DS1807", "DS1809"}, fields: []string{"overlap"}},
}

// findingDeletableOnlyField is the one details field no code discriminates: it
// rides on any deletable finding whose fix drops a dependency's last use.
const findingDeletableOnlyField = "removes_last_use_of"

// findingEmptyDetailsCodes are the live codes no branch names, so their details
// object is empty.
var findingEmptyDetailsCodes = []string{
	"DS1001", "DS1002", "DS1003", "DS1004", "DS1005", "DS1006",
	"DS1103", "DS1302", "DS1303", "DS1502", "DS1704",
}

// findingFieldsForCode lists the details fields a code carries, sorted.
func findingFieldsForCode(code string) []string {
	var fields []string
	for _, branch := range findingDetailsBranches {
		if slices.Contains(branch.codes, code) {
			fields = append(fields, branch.fields...)
		}
	}
	return findingSorted(fields)
}

// findingSorted returns a sorted copy of a list of names.
func findingSorted(names []string) []string {
	out := slices.Clone(names)
	slices.Sort(out)
	return out
}

func loadFindingSchema(t *testing.T) map[string]any {
	t.Helper()
	return loadSchema(t, spec.Contract, findingSchemaPath)
}

// findingSchemaBytes reads the schema as it is written, for the checks that are
// about the document's own order.
func findingSchemaBytes(t *testing.T) []byte {
	t.Helper()
	data, err := fs.ReadFile(spec.Contract, findingSchemaPath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", findingSchemaPath, err)
	}
	return data
}

// findingPropertyOrder returns the keys of one JSON object in the order the
// document writes them.
func findingPropertyOrder(t *testing.T, where string, raw []byte) []string {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	open, err := dec.Token()
	if err != nil {
		t.Fatalf("Setup: reading %s: %v", where, err)
	}
	if delim, ok := open.(json.Delim); !ok || delim != '{' {
		t.Fatalf("Setup: %s opens with %v, want an object", where, open)
	}
	var names []string
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			t.Fatalf("Setup: reading a key of %s: %v", where, err)
		}
		name, ok := key.(string)
		if !ok {
			t.Fatalf("Setup: %s carries the key %v, want a string", where, key)
		}
		names = append(names, name)
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			t.Fatalf("Setup: reading the value of %s.%s: %v", where, name, err)
		}
	}
	return names
}

// findingDiscriminator returns the codes one branch's if names, and nil for a
// condition on any other field.
func findingDiscriminator(node any) []string {
	condition, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	props, ok := condition["properties"].(map[string]any)
	if !ok {
		return nil
	}
	code, ok := props["code"].(map[string]any)
	if !ok {
		return nil
	}
	if single, ok := code["const"].(string); ok {
		return []string{single}
	}
	return stringSlice(code["enum"])
}

// findingRequiredDetails lists every details field the arm requires, at any
// depth, skipping the arms that forbid a field rather than require one.
func findingRequiredDetails(node any) []string {
	var found []string
	switch n := node.(type) {
	case map[string]any:
		if details, ok := n["details"].(map[string]any); ok {
			found = append(found, stringSlice(details["required"])...)
		}
		for key, child := range n {
			if key == "not" || key == "else" {
				continue
			}
			found = append(found, findingRequiredDetails(child)...)
		}
	case []any:
		for _, child := range n {
			found = append(found, findingRequiredDetails(child)...)
		}
	}
	slices.Sort(found)
	return slices.Compact(found)
}

// findingForbiddenDetails lists the details fields one branch's else arm
// forbids.
func findingForbiddenDetails(node any) []string {
	arm, ok := node.(map[string]any)
	if !ok {
		return nil
	}
	details := objectAt(arm, "properties/details")
	var found []string
	if alternatives, ok := objectAt(details, "not")["anyOf"].([]any); ok {
		for _, alternative := range alternatives {
			branch, _ := alternative.(map[string]any)
			found = append(found, stringSlice(branch["required"])...)
		}
	}
	found = append(found, stringSlice(objectAt(details, "not")["required"])...)
	slices.Sort(found)
	return slices.Compact(found)
}

// findingSchemaArms returns the schema's discriminated branches, in order, and
// the arms that condition on something other than the code.
func findingSchemaArms(t *testing.T, schema map[string]any) (branches []findingDetailsBranch, others []map[string]any) {
	t.Helper()
	arms, ok := schema["allOf"].([]any)
	if !ok || len(arms) == 0 {
		t.Fatalf("Setup: %s allOf = %v, want the discriminated branches", findingSchemaPath, schema["allOf"])
	}
	for _, arm := range arms {
		node, ok := arm.(map[string]any)
		if !ok {
			t.Fatalf("Setup: %s allOf holds %v, want a subschema object", findingSchemaPath, arm)
		}
		fields := findingRequiredDetails(node["then"])
		codes := findingDiscriminator(node["if"])
		if len(codes) == 0 || len(fields) == 0 {
			// An arm that requires no details field decides a top-level
			// member instead: the deletable-only field and the liveness
			// relation are the two, and each has its own test. The liveness
			// arm also conditions on the subject, so it names no code at
			// the top level of its condition.
			others = append(others, node)
			continue
		}
		branches = append(branches, findingDetailsBranch{codes: codes, fields: fields})
	}
	return branches, others
}

// findingLivenessArm returns the one arm of allOf that decides the liveness
// relation, being the arm that requires that member of every finding its
// condition does not match.
func findingLivenessArm(t *testing.T, schema map[string]any) map[string]any {
	t.Helper()
	_, others := findingSchemaArms(t, schema)
	var found []map[string]any
	for _, arm := range others {
		if slices.Contains(stringSlice(objectAt(arm, "else")["required"]), "liveness_relation") {
			found = append(found, arm)
		}
	}
	if len(found) != 1 {
		t.Fatalf("arms requiring liveness_relation of every finding they do not match = %d, want 1", len(found))
	}
	return found[0]
}

// findingLivenessCondition returns the codes and the symbol kinds the liveness
// arm's condition names: its alternatives are one enum over symbol.kind and one
// over code, and neither is read without the other.
func findingLivenessCondition(t *testing.T, arm map[string]any) (codes, kinds []string) {
	t.Helper()
	alternatives, ok := objectAt(arm, "if")["anyOf"].([]any)
	if !ok {
		t.Fatalf("Setup: the liveness arm's condition = %v, want alternatives over the code and the symbol kind", arm["if"])
	}
	for _, alternative := range alternatives {
		branch, ok := alternative.(map[string]any)
		if !ok {
			t.Fatalf("Setup: the liveness arm holds %v, want a subschema object", alternative)
		}
		codes = append(codes, findingDiscriminator(branch)...)
		kinds = append(kinds, enumAt(branch, "properties/symbol/properties/kind")...)
	}
	return findingSorted(codes), findingSorted(kinds)
}

// findingLiveCodes lists every live code of the vocabulary, sorted.
func findingLiveCodes(doc kindsDocument) []string {
	codes := make([]string, 0, len(doc.Kinds))
	for _, k := range doc.Kinds {
		codes = append(codes, k.Code)
	}
	slices.Sort(codes)
	return codes
}

// findingCodesMatching lists the live codes whose row satisfies want, sorted.
func findingCodesMatching(doc kindsDocument, want func(kindRow) bool) []string {
	var codes []string
	for _, k := range doc.Kinds {
		if want(k) {
			codes = append(codes, k.Code)
		}
	}
	slices.Sort(codes)
	return codes
}

// findingRowMentions reports whether a row's published rule or precondition
// names a subject, which is how the vocabulary says a kind carries extra data.
func findingRowMentions(k kindRow, phrase string) bool {
	return strings.Contains(k.Rule, phrase) || strings.Contains(k.Precondition, phrase)
}

// findingInRange reports whether a code falls in a family's range.
func findingInRange(code string, start, end int) bool {
	n := codeNumber(code)
	return start <= n && n <= end
}

func TestFindingSchemaIsClosedAndDescribed(t *testing.T) {
	schema := loadFindingSchema(t)
	if schema["$schema"] != findingSchemaDialect {
		t.Errorf("%s $schema = %v, want %q", findingSchemaPath, schema["$schema"], findingSchemaDialect)
	}
	if got := openObjects(schema, ""); len(got) != 0 {
		t.Errorf("openObjects(%s) = %v, want every object schema to close its key set", findingSchemaPath, got)
	}
	if got := undescribedProperties(schema, ""); len(got) != 0 {
		t.Errorf("undescribedProperties(%s) = %v, want a description on every property", findingSchemaPath, got)
	}
	for at, pattern := range patterns(schema, "") {
		if _, err := regexp.Compile(pattern); err != nil {
			t.Errorf("regexp.Compile(%s %s = %q) = %v, want a pattern both dialects accept", findingSchemaPath, at, pattern, err)
		}
	}
}

// TestFindingSchemaReferencesOnlyItsOwnDefinitions pins the one-file rule: a
// finding is declared here whole, so every reference is to a definition of this
// document and no reference reaches another file.
func TestFindingSchemaReferencesOnlyItsOwnDefinitions(t *testing.T) {
	schema := loadFindingSchema(t)
	defs, ok := schema["$defs"].(map[string]any)
	if !ok {
		t.Fatalf("Setup: %s $defs = %v, want the shared definitions", findingSchemaPath, schema["$defs"])
	}
	refs := findingReferenceTargets(schema, "")
	if len(refs) == 0 {
		t.Fatalf("Setup: %s declares no reference, want the shared definitions referenced", findingSchemaPath)
	}
	for _, at := range slices.Sorted(maps.Keys(refs)) {
		target := refs[at]
		t.Run(subtestName(at), func(t *testing.T) {
			name, found := strings.CutPrefix(target, "#/$defs/")
			if !found {
				t.Fatalf("%s %s = %q, want a reference to a definition of this document", findingSchemaPath, at, target)
			}
			if _, ok := defs[name]; !ok {
				t.Errorf("%s %s names %q, want one of %v", findingSchemaPath, at, name, slices.Sorted(maps.Keys(defs)))
			}
		})
	}
}

// findingReferenceTargets maps the path of every reference to the value it
// names.
func findingReferenceTargets(node any, at string) map[string]string {
	found := map[string]string{}
	switch n := node.(type) {
	case map[string]any:
		if target, ok := n["$ref"].(string); ok {
			found[at+"/$ref"] = target
		}
		for key, child := range n {
			maps.Copy(found, findingReferenceTargets(child, at+"/"+key))
		}
	case []any:
		for i, child := range n {
			maps.Copy(found, findingReferenceTargets(child, fmt.Sprintf("%s/%d", at, i)))
		}
	}
	return found
}

// TestFindingSchemaDeclaresEveryFieldInOrderAndRequiresEachOne pins the two
// properties a byte-identical report rests on: the field list is the design's,
// in one order, and no field is optional, so a consumer reads every field on
// every finding.
func TestFindingSchemaDeclaresEveryFieldInOrderAndRequiresEachOne(t *testing.T) {
	schema := loadFindingSchema(t)
	var document map[string]json.RawMessage
	if err := json.Unmarshal(findingSchemaBytes(t), &document); err != nil {
		t.Fatalf("Setup: json.Unmarshal(%q): %v", findingSchemaPath, err)
	}
	got := findingPropertyOrder(t, "the finding's properties", document["properties"])
	if !slices.Equal(got, findingFields) {
		t.Errorf("propertyOrder(%s) = %v, want %v", findingSchemaPath, got, findingFields)
	}
	required := stringSlice(schema["required"])
	var wantRequired []string
	for _, f := range findingFields {
		if !slices.Contains(findingOptionalFields, f) {
			wantRequired = append(wantRequired, f)
		}
	}
	if want := findingSorted(wantRequired); !slices.Equal(findingSorted(required), want) {
		t.Errorf("required(%s) = %v, want every field but %v required: %v", findingSchemaPath, required, findingOptionalFields, want)
	}
}

func TestFindingSchemaRequiredMembers(t *testing.T) {
	schema := loadFindingSchema(t)
	cases := []struct {
		at   string
		want []string
	}{
		{at: "properties/symbol", want: []string{"kind", "name", "ref", "size_lines"}},
		{at: "properties/component", want: []string{"deletable_lines", "id", "root", "symbol_count"}},
		{at: "properties/details/properties/entry", want: []string{"code"}},
		{at: "properties/details/properties/sides/items", want: []string{"side", "state", "symbol"}},
		{at: "$defs/position", want: []string{"column", "end_line", "line", "path"}},
		{at: "$defs/positioned_symbol", want: []string{"name", "position", "ref"}},
	}
	for _, tc := range cases {
		t.Run(subtestName(tc.at), func(t *testing.T) {
			got := findingSorted(requiredAt(schema, tc.at))
			if !slices.Equal(got, tc.want) {
				t.Errorf("required(%s %s) = %v, want %v", findingSchemaPath, tc.at, got, tc.want)
			}
			for _, name := range tc.want {
				if !slices.Contains(propertyNames(schema, tc.at), name) {
					t.Errorf("%s %s requires %q, want it declared under properties %v", findingSchemaPath, tc.at, name, propertyNames(schema, tc.at))
				}
			}
		})
	}
}

func TestFindingSchemaEnumsAreTheContractVocabularies(t *testing.T) {
	schema := loadFindingSchema(t)
	kinds := loadKinds(t)
	expect := loadSchema(t, spec.Corpus, expectSchemaPath)
	cases := []struct {
		name string
		at   string
		want []string
	}{
		{name: "language", at: "properties/language", want: kinds.Languages},
		{name: "reachability_class", at: "properties/reachability_class", want: kinds.ReachabilityClasses},
		{name: "confidence", at: "properties/confidence", want: kinds.ReachabilityClasses},
		{name: "fixability", at: "properties/fixability", want: kinds.Fixabilities},
		{name: "severity", at: "properties/severity", want: kinds.Severities},
		{
			name: "liveness_relation",
			at:   "properties/liveness_relation",
			want: enumAt(expect, "properties/expect/items/properties/liveness_relation"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.want) == 0 {
				t.Fatalf("Setup: the vocabulary for %s is empty, want the committed values", tc.name)
			}
			if got := enumAt(schema, tc.at); !slices.Equal(got, tc.want) {
				t.Errorf("enumAt(%s, %q) = %v, want %v from the committed vocabulary", findingSchemaPath, tc.at, got, tc.want)
			}
		})
	}
	t.Run("code", func(t *testing.T) {
		got := patterns(schema, "")["/properties/code/pattern"]
		want := "^" + kinds.Prefix + "[0-9]{4}$"
		if got != want {
			t.Errorf("code pattern = %q, want %q", got, want)
		}
	})
}

// TestFindingSchemaSymbolKindsAreClosedAndRenderable pins the vocabulary this
// schema owns: every value is a token a text line can render, the description
// states what each one is, and every kind the published lines render is
// declared.
func TestFindingSchemaSymbolKindsAreClosedAndRenderable(t *testing.T) {
	schema := loadFindingSchema(t)
	declared := enumAt(schema, "properties/symbol/properties/kind")
	if len(declared) == 0 {
		t.Fatalf("Setup: %s declares no symbol kind, want the closed vocabulary", findingSchemaPath)
	}
	description, _ := objectAt(schema, "properties/symbol/properties/kind")["description"].(string)
	for _, kind := range declared {
		t.Run(kind, func(t *testing.T) {
			if !findingKindColumn.MatchString(kind) {
				t.Errorf("symbol kind %q does not match the kind column %s, want a renderable token", kind, findingKindColumn)
			}
			if !strings.Contains(description, "; "+kind+", ") && !strings.Contains(description, ": "+kind+", ") {
				t.Errorf("the description of symbol.kind does not state what %q is, want one line per value", kind)
			}
		})
	}
	if got := len(slices.Compact(findingSorted(declared))); got != len(declared) {
		t.Errorf("symbol kinds = %v, want %d distinct values, got %d", declared, len(declared), got)
	}
	for _, kind := range findingRenderedKinds(t) {
		if !slices.Contains(declared, kind) {
			t.Errorf("text-line.md renders the kind %q, want it declared among %v", kind, declared)
		}
	}
}

// findingRenderedKinds returns the symbol kinds the accepted set of
// text-line.md renders, read out of the page.
func findingRenderedKinds(t *testing.T) []string {
	t.Helper()
	lines := onlyFence(t, pageSection(t, grammarPage(t, textLinePagePath), "### Accepted"), "the accepted set of text-line.md")
	var kinds []string
	for _, line := range lines {
		match := findingLinePattern.FindStringSubmatch(line)
		if match == nil {
			t.Fatalf("Setup: the accepted line %q holds no kind column, want one", line)
		}
		if !slices.Contains(kinds, match[1]) {
			kinds = append(kinds, match[1])
		}
	}
	if len(kinds) == 0 {
		t.Fatalf("Setup: the accepted set of %s renders no kind, want at least one", textLinePagePath)
	}
	return kinds
}

// TestFindingSchemaPathFormClassifiesTheSuppressionCorpus pins the path form
// against the one corpus that carries accepted and refused paths: an entry is
// matched on the finding's own path, so the two forms are one form.
func TestFindingSchemaPathFormClassifiesTheSuppressionCorpus(t *testing.T) {
	pattern := patterns(loadFindingSchema(t), "")["/"+findingPathDefPath+"/pattern"]
	if pattern == "" {
		t.Fatalf("Setup: %s declares no pattern at %s, want the path form", findingSchemaPath, findingPathDefPath)
	}
	form, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("Setup: regexp.Compile(%q): %v", pattern, err)
	}
	entries := findingRecordPaths(t)
	if len(entries) == 0 {
		t.Fatalf("Setup: the suppression corpus supplied no path, want the accepted and the refused ones")
	}
	for _, entry := range entries {
		t.Run(entry.rule+"_"+subtestName(entry.path), func(t *testing.T) {
			if got := form.MatchString(entry.path); got != entry.want {
				t.Errorf("pathForm.MatchString(%q) = %t, want %t: the corpus %s it under %q", entry.path, got, entry.want, entry.verdict(), entry.rule)
			}
		})
	}
}

// findingRecordPath is one path the suppression corpus carries, with the
// verdict the path form owes it.
type findingRecordPath struct {
	rule string
	path string
	want bool
}

func (p findingRecordPath) verdict() string {
	if p.want {
		return "accepts"
	}
	return "refuses"
}

// findingRecordPaths returns the paths of every ignore entry and baseline row
// of the suppression corpus: the path form accepts the one an accepted record
// names and refuses the one a record refused for its path names.
func findingRecordPaths(t *testing.T) []findingRecordPath {
	t.Helper()
	var out []findingRecordPath
	for _, c := range loadSuppressionCorpus(t) {
		if c.Kind != "ignore-entry" && c.Kind != "baseline-row" {
			continue
		}
		var record struct {
			Path *string `json:"path"`
		}
		if err := json.Unmarshal(c.Input, &record); err != nil {
			t.Fatalf("Setup: decode the record %s: %v", c.Input, err)
		}
		if record.Path == nil {
			continue
		}
		switch {
		case c.Accepted:
			out = append(out, findingRecordPath{rule: c.Rule, path: *record.Path, want: true})
		case c.Rule == "path-form" || c.Rule == "path-required":
			out = append(out, findingRecordPath{rule: c.Rule, path: *record.Path, want: false})
		}
	}
	return out
}

// TestFindingSchemaRetainedByAdmitsEveryExemptionClass pins the field against
// the vocabulary it names: a class the exemption vocabulary declares is a value
// a retained-symbol listing can carry.
func TestFindingSchemaRetainedByAdmitsEveryExemptionClass(t *testing.T) {
	pattern := patterns(loadFindingSchema(t), "")["/properties/retained_by/items/pattern"]
	if pattern == "" {
		t.Fatalf("Setup: %s declares no pattern for a retained_by item, want the class shape", findingSchemaPath)
	}
	shape, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("Setup: regexp.Compile(%q): %v", pattern, err)
	}
	for _, class := range loadExemptions(t).Exemptions {
		t.Run(class.Class, func(t *testing.T) {
			if !shape.MatchString(class.Class) {
				t.Errorf("retained_by item shape %s does not match the class %q of %s, want every class admitted", shape, class.Class, exemptionsPath)
			}
		})
	}
}

// TestFindingSchemaDetailsBranchesFollowTheKinds pins the discriminated details
// object: one branch per set of codes that carries extra data, each branch
// forbidding its own fields under every other code, and the branch list the one
// the vocabulary and the grammar pages imply.
func TestFindingSchemaDetailsBranchesFollowTheKinds(t *testing.T) {
	schema := loadFindingSchema(t)
	kinds := loadKinds(t)
	branches, others := findingSchemaArms(t, schema)

	t.Run("branches", func(t *testing.T) {
		if len(branches) != len(findingDetailsBranches) {
			t.Fatalf("branches(%s) = %d, want %d", findingSchemaPath, len(branches), len(findingDetailsBranches))
		}
		for i, want := range findingDetailsBranches {
			got := branches[i]
			if !slices.Equal(got.codes, want.codes) || !slices.Equal(got.fields, findingSorted(want.fields)) {
				t.Errorf("branch %d = %v carrying %v, want %v carrying %v", i, got.codes, got.fields, want.codes, want.fields)
			}
		}
	})

	t.Run("every_branch_forbids_its_fields_elsewhere", func(t *testing.T) {
		arms, _ := schema["allOf"].([]any)
		for i, want := range findingDetailsBranches {
			arm, _ := arms[i].(map[string]any)
			got := findingForbiddenDetails(arm["else"])
			if !slices.Equal(got, findingSorted(want.fields)) {
				t.Errorf("branch %v forbids %v under another code, want %v", want.codes, got, want.fields)
			}
		}
	})

	t.Run("the_deletable_field_rides_on_no_code", func(t *testing.T) {
		var found []map[string]any
		for _, other := range others {
			if len(objectAt(other, "if/properties/fixability")) != 0 {
				found = append(found, other)
			}
		}
		if len(found) != 1 {
			t.Fatalf("arms conditioning on the fixability = %d, want 1 for %s", len(found), findingDeletableOnlyField)
		}
		arm := found[0]
		if got := findingForbiddenDetails(arm["else"]); !slices.Equal(got, []string{findingDeletableOnlyField}) {
			t.Errorf("the fixability arm forbids %v, want %v", got, []string{findingDeletableOnlyField})
		}
		if got, _ := objectAt(arm, "if/properties/fixability")["const"].(string); got != "deletable" {
			t.Errorf("the fixability arm conditions on %q, want %q", got, "deletable")
		}
	})

	t.Run("every_declared_field_belongs_to_a_branch", func(t *testing.T) {
		var declared []string
		declared = append(declared, propertyNames(schema, "properties/details")...)
		want := []string{findingDeletableOnlyField}
		for _, branch := range findingDetailsBranches {
			want = append(want, branch.fields...)
		}
		if got := findingSorted(declared); !slices.Equal(got, findingSorted(want)) {
			t.Errorf("details properties = %v, want %v", got, findingSorted(want))
		}
	})

	t.Run("every_branch_names_a_live_code", func(t *testing.T) {
		live := findingLiveCodes(kinds)
		for _, branch := range findingDetailsBranches {
			for _, code := range branch.codes {
				if !slices.Contains(live, code) {
					t.Errorf("branch %v names %q, want a live code of %s", branch.codes, code, kindsPath)
				}
			}
		}
	})

	// Every live code either carries a branch or carries an empty details, and
	// which of the two is a decision: a kind added to the vocabulary fails here
	// until it is taken.
	t.Run("every_live_code_is_branched_or_carries_an_empty_details", func(t *testing.T) {
		var branched, empty []string
		for _, code := range findingLiveCodes(kinds) {
			if len(findingFieldsForCode(code)) == 0 {
				empty = append(empty, code)
				continue
			}
			branched = append(branched, code)
		}
		if got := len(branched) + len(empty); got != len(kinds.Kinds) {
			t.Errorf("branched and empty codes = %d, want the %d live rows of %s", got, len(kinds.Kinds), kindsPath)
		}
		if !slices.Equal(empty, findingEmptyDetailsCodes) {
			t.Errorf("codes carrying an empty details = %v, want %v", empty, findingEmptyDetailsCodes)
		}
	})

	// The vocabulary and the grammar pages say which codes carry extra data;
	// each row below derives one branch from them rather than restating it.
	derivations := []struct {
		name  string
		field string
		want  []string
	}{
		{
			name:  "the_rows_that_name_a_narrower_visibility",
			field: "narrower_visibility",
			want: findingCodesMatching(kinds, func(k kindRow) bool {
				return findingRowMentions(k, "narrower visibility")
			}),
		},
		{
			name:  "the_rows_that_name_a_write_position",
			field: "write_positions",
			want: findingCodesMatching(kinds, func(k kindRow) bool {
				return findingRowMentions(k, "write position")
			}),
		},
		{
			name:  "the_rows_that_name_a_build_constraint",
			field: "excluded_by",
			want: findingCodesMatching(kinds, func(k kindRow) bool {
				return findingRowMentions(k, "build constraint")
			}),
		},
		{
			name:  "the_interface_family",
			field: "implementations",
			want: findingCodesMatching(kinds, func(k kindRow) bool {
				return findingInRange(k.Code, 1200, 1299)
			}),
		},
		{
			name:  "the_rows_that_carry_an_overlap_list",
			field: "overlap",
			want: findingCodesMatching(kinds, func(k kindRow) bool {
				return len(k.Overlap) != 0
			}),
		},
		{
			name:  "the_row_about_a_declared_edge",
			field: "edge",
			want: findingCodesMatching(kinds, func(k kindRow) bool {
				return strings.Contains(k.Name, "edge")
			}),
		},
		{
			name:  "the_codes_the_suppression_page_hands_this_schema",
			field: "mechanism",
			want:  findingSuppressionCodes(t),
		},
	}
	for _, d := range derivations {
		t.Run(d.name, func(t *testing.T) {
			if len(d.want) == 0 {
				t.Fatalf("Setup: the derivation for %q yielded no code, want the codes that carry it", d.field)
			}
			got := findingBranchCarrying(d.field)
			if !slices.Equal(got, d.want) {
				t.Errorf("the branch carrying %q = %v, want %v", d.field, got, d.want)
			}
		})
	}

	t.Run("the_dependency_family_is_split_by_its_two_fields", func(t *testing.T) {
		var got []string
		got = append(got, findingBranchCarrying("dependency_class")...)
		got = append(got, findingBranchCarrying("replacement")...)
		slices.Sort(got)
		want := findingCodesMatching(kinds, func(k kindRow) bool { return findingInRange(k.Code, 1600, 1699) })
		if !slices.Equal(got, want) {
			t.Errorf("the branches carrying a dependency or a directive = %v, want the dependency family %v", got, want)
		}
	})
}

// TestFindingSchemaLiveSubjectCodesCarryNoLivenessRelation pins the one arm of
// allOf that decides a top-level member by the subject: a finding whose subject
// is not a declaration, and a finding whose subject the analysis holds live,
// carry no liveness relation, and every other finding carries one. The field is
// therefore optional in the schema's own required list and mandatory or
// forbidden per subject here.
func TestFindingSchemaLiveSubjectCodesCarryNoLivenessRelation(t *testing.T) {
	schema := loadFindingSchema(t)
	arm := findingLivenessArm(t, schema)
	codes, kinds := findingLivenessCondition(t, arm)

	t.Run("codes", func(t *testing.T) {
		if want := findingSorted(findingLiveSubjectCodes); !slices.Equal(codes, want) {
			t.Errorf("the liveness arm names %v, want the live-subject codes %v", codes, want)
		}
	})

	t.Run("subject_kinds", func(t *testing.T) {
		if want := findingSorted(findingNoRelationKinds); !slices.Equal(kinds, want) {
			t.Errorf("the liveness arm names %v, want the kinds that are no declaration %v", kinds, want)
		}
	})

	t.Run("every_named_kind_is_in_the_vocabulary", func(t *testing.T) {
		vocabulary := enumAt(schema, "properties/symbol/properties/kind")
		for _, kind := range kinds {
			if !slices.Contains(vocabulary, kind) {
				t.Errorf("the liveness arm names %q, want a symbol kind of %s", kind, findingSchemaPath)
			}
		}
	})

	t.Run("a_live_subject_carries_none", func(t *testing.T) {
		got := stringSlice(objectAt(arm, "then/not")["required"])
		if want := []string{"liveness_relation"}; !slices.Equal(got, want) {
			t.Errorf("the liveness arm forbids %v under a live-subject code, want %v", got, want)
		}
	})

	t.Run("every_other_code_carries_one", func(t *testing.T) {
		got := stringSlice(objectAt(arm, "else")["required"])
		if want := []string{"liveness_relation"}; !slices.Equal(got, want) {
			t.Errorf("the liveness arm requires %v under every other code, want %v", got, want)
		}
	})

	t.Run("every_named_code_is_live", func(t *testing.T) {
		live := findingLiveCodes(loadKinds(t))
		for _, code := range findingLiveSubjectCodes {
			if !slices.Contains(live, code) {
				t.Errorf("the liveness arm names %q, want a live code of %s", code, kindsPath)
			}
		}
	})
}

// findingBranchCarrying returns the codes of the branch that carries a field,
// sorted.
func findingBranchCarrying(field string) []string {
	for _, branch := range findingDetailsBranches {
		if slices.Contains(branch.fields, field) {
			return findingSorted(branch.codes)
		}
	}
	return nil
}

// findingSuppressionCodes returns the codes suppression.md hands this schema,
// read off the line of that page which names the finding schema.
func findingSuppressionCodes(t *testing.T) []string {
	t.Helper()
	const marker = "`finding.schema.json`"
	var codes []string
	for line := range strings.SplitSeq(grammarPage(t, suppressionPagePath), "\n") {
		if !strings.Contains(line, marker) {
			continue
		}
		for _, code := range findingCodeInText.FindAllString(strings.ReplaceAll(line, "`", " "), -1) {
			if !slices.Contains(codes, code) {
				codes = append(codes, code)
			}
		}
	}
	slices.Sort(codes)
	return codes
}
