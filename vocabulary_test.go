package spec_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec"
)

// This suite proves the closed-vocabulary rules of contract/kinds.json,
// contract/exemptions.json and contract/exit-codes.json refuse a violation,
// by driving the same functions the committed documents go through over
// planted copies that differ in one thing. The committed documents are read
// as they are; every plant lives in memory.
var (
	errMeaningShared      = errors.New("two exit codes carry one meaning")
	errClassDeclaredTwice = errors.New("exemption class declared twice")
	errSeverityKeyShape   = errors.New("severity key is neither a code nor a family prefix")
	errSeverityKeyUnknown = errors.New("severity key names no code of the space")
	errSeverityKeyRetired = errors.New("severity key names a retired code or range")
	errSeverityKeyFixed   = errors.New("severity key names a kind whose severity the Contract fixes")
)

// plantedRule is what a planted violation must produce: an error carrying the
// rule's sentinel and naming the code, class, key or meaning the plant
// carries.
type plantedRule struct {
	sentinel error
	names    string
}

// satisfies reports whether an error is the rule firing on the planted subject.
func (p plantedRule) satisfies(err error) bool {
	return errors.Is(err, p.sentinel) && strings.Contains(err.Error(), p.names)
}

// String names the rule and its subject: the fields are unexported, so without
// this a failure message prints the sentinel's address.
func (p plantedRule) String() string { return fmt.Sprintf("%v naming %q", p.sentinel, p.names) }

var _ fmt.Stringer = plantedRule{}

// joined renders an error list for a failure message, so several violations
// read as a list rather than as one run-on sentence.
func joined(errs []error) string {
	if len(errs) == 0 {
		return "no error"
	}
	texts := make([]string, 0, len(errs))
	for _, err := range errs {
		texts = append(texts, err.Error())
	}
	return "[" + strings.Join(texts, "; ") + "]"
}

// missingRules returns the expectations no returned error satisfies, so a
// failure names the rule that stayed silent.
func missingRules(got []error, want []plantedRule) []plantedRule {
	var missing []plantedRule
	for _, w := range want {
		if !slices.ContainsFunc(got, w.satisfies) {
			missing = append(missing, w)
		}
	}
	return missing
}

// surplusErrors returns the errors no expectation accounts for, so a planted
// case pins which rules fire and not only the one it was written for. A case
// expecting nothing is checked by this alone.
func surplusErrors(got []error, want []plantedRule) []error {
	var surplus []error
	for _, err := range got {
		if !slices.ContainsFunc(want, func(w plantedRule) bool { return w.satisfies(err) }) {
			surplus = append(surplus, err)
		}
	}
	return surplus
}

// plantedKinds decodes the committed vocabulary and applies one plant, so a
// case differs from the document the products read in that one thing.
func plantedKinds(t *testing.T, plant func(*kindsDocument)) kindsDocument {
	t.Helper()
	doc := loadKinds(t)
	plant(&doc)
	return doc
}

// liveKindRow is a complete live row at a code, so planting one adds no
// violation of its own.
func liveKindRow(code, name string) kindRow {
	enabled, fixed := true, false
	return kindRow{
		Code:            code,
		Name:            name,
		Languages:       []string{"go", "ts"},
		Rule:            "A planted row.",
		DefaultEnabled:  &enabled,
		DefaultSeverity: "deny",
		MaxClass:        "certain",
		Fixability:      "deletable",
		Fixed:           &fixed,
	}
}

// liveCodeRange is a live range spanning start to end.
func liveCodeRange(start, end, family string) codeRange {
	retired := false
	return codeRange{Start: start, End: end, Family: family, Description: "A planted range.", Retired: &retired}
}

func TestVocabularyCodeAssignmentRefusesAPlantedCode(t *testing.T) {
	cases := []struct {
		plant func(*kindsDocument)
		name  string
		want  []plantedRule
	}{
		{
			name:  "an_unassigned_live_code",
			plant: func(doc *kindsDocument) { doc.Kinds = append(doc.Kinds, liveKindRow("DS1010", "planted-live")) },
		},
		{
			name:  "a_code_a_live_row_already_carries",
			plant: func(doc *kindsDocument) { doc.Kinds = append(doc.Kinds, liveKindRow("DS1001", "planted-duplicate")) },
			want:  []plantedRule{{sentinel: errCodeAssignedTwice, names: "DS1001"}},
		},
		{
			name:  "a_code_of_the_wrong_shape",
			plant: func(doc *kindsDocument) { doc.Kinds = append(doc.Kinds, liveKindRow("DS999", "planted-short-code")) },
			want:  []plantedRule{{sentinel: errCodeShape, names: "DS999"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkCodeAssignment(plantedKinds(t, tc.plant))
			if missing := missingRules(got, tc.want); len(missing) != 0 {
				t.Errorf("checkCodeAssignment(%s plus %s) = %s, want %v, missing %v", kindsPath, tc.name, joined(got), tc.want, missing)
			}
			if surplus := surplusErrors(got, tc.want); len(surplus) != 0 {
				t.Errorf("checkCodeAssignment(%s plus %s) = %s, want only %v, surplus %s", kindsPath, tc.name, joined(got), tc.want, joined(surplus))
			}
		})
	}
}

func TestVocabularyRangeAssignmentRefusesAPlantedRange(t *testing.T) {
	cases := []struct {
		plant func(*kindsDocument)
		name  string
		want  []plantedRule
	}{
		{
			name:  "a_live_row_inside_a_declared_range",
			plant: func(doc *kindsDocument) { doc.Kinds = append(doc.Kinds, liveKindRow("DS1010", "planted-live")) },
		},
		{
			name: "a_range_straddling_a_declared_range",
			plant: func(doc *kindsDocument) {
				doc.Ranges = append(doc.Ranges, liveCodeRange("DS1001", "DS1001", "planted-straddle"))
			},
			want: []plantedRule{
				{sentinel: errRangesStraddle, names: "DS1000..DS1099"},
				{sentinel: errCodeInSeveralRanges, names: "DS1001"},
			},
		},
		{
			name: "a_range_running_backwards",
			plant: func(doc *kindsDocument) {
				i := slices.IndexFunc(doc.Ranges, func(r codeRange) bool { return r.Start == "DS1400" })
				doc.Ranges[i].End = "DS1350"
			},
			want: []plantedRule{{sentinel: errRangeSpan, names: `"DS1400".."DS1350"`}},
		},
		{
			name:  "a_live_row_outside_every_range",
			plant: func(doc *kindsDocument) { doc.Kinds = append(doc.Kinds, liveKindRow("DS1901", "planted-unranged")) },
			want:  []plantedRule{{sentinel: errCodeOutsideRanges, names: "DS1901"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkRangeAssignment(plantedKinds(t, tc.plant))
			if missing := missingRules(got, tc.want); len(missing) != 0 {
				t.Errorf("checkRangeAssignment(%s plus %s) = %s, want %v, missing %v", kindsPath, tc.name, joined(got), tc.want, missing)
			}
			if surplus := surplusErrors(got, tc.want); len(surplus) != 0 {
				t.Errorf("checkRangeAssignment(%s plus %s) = %s, want only %v, surplus %s", kindsPath, tc.name, joined(got), tc.want, joined(surplus))
			}
		})
	}
}

func TestVocabularyRetirementRefusesAPlantedReassignment(t *testing.T) {
	cases := []struct {
		plant func(*kindsDocument)
		name  string
		want  []plantedRule
	}{
		{
			name:  "a_live_row_at_a_code_never_assigned",
			plant: func(doc *kindsDocument) { doc.Kinds = append(doc.Kinds, liveKindRow("DS1010", "planted-live")) },
		},
		{
			name:  "a_live_row_at_retired_DS1402",
			plant: func(doc *kindsDocument) { doc.Kinds = append(doc.Kinds, liveKindRow("DS1402", "planted-resurrection")) },
			want: []plantedRule{
				{sentinel: errRetiredCodeReused, names: "DS1402"},
				{sentinel: errLiveRowInRetiredRange, names: "DS1402"},
			},
		},
		{
			name:  "a_live_row_at_retired_DS1607",
			plant: func(doc *kindsDocument) { doc.Kinds = append(doc.Kinds, liveKindRow("DS1607", "planted-resurrection")) },
			want:  []plantedRule{{sentinel: errRetiredCodeReused, names: "DS1607"}},
		},
		{
			name: "a_live_row_inside_the_retired_range",
			plant: func(doc *kindsDocument) {
				doc.Kinds = append(doc.Kinds, liveKindRow("DS1450", "planted-retired-range"))
			},
			want: []plantedRule{{sentinel: errLiveRowInRetiredRange, names: "DS1450"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkRetirement(plantedKinds(t, tc.plant))
			if missing := missingRules(got, tc.want); len(missing) != 0 {
				t.Errorf("checkRetirement(%s plus %s) = %s, want %v, missing %v", kindsPath, tc.name, joined(got), tc.want, missing)
			}
			if surplus := surplusErrors(got, tc.want); len(surplus) != 0 {
				t.Errorf("checkRetirement(%s plus %s) = %s, want only %v, surplus %s", kindsPath, tc.name, joined(got), tc.want, joined(surplus))
			}
		})
	}
}

// checkExitCodeMeanings reports every meaning more than one row of the exit
// code table carries: one meaning belongs to one code.
func checkExitCodeMeanings(table exitCodeTable) []error {
	carriers := map[string][]string{}
	for _, row := range table.ExitCodes {
		meaning := strings.TrimSpace(row.Meaning)
		carriers[meaning] = append(carriers[meaning], row.Name)
	}
	var errs []error
	for _, meaning := range slices.Sorted(maps.Keys(carriers)) {
		if names := carriers[meaning]; len(names) > 1 {
			errs = append(errs, fmt.Errorf("%w: meaning %q carried by %v, want one row", errMeaningShared, meaning, names))
		}
	}
	return errs
}

func TestVocabularyExitCodeMeaningsRefuseAPlantedRepeat(t *testing.T) {
	table := mustLoadExitCodes(t)
	if got := checkExitCodeMeanings(table); len(got) != 0 {
		t.Fatalf("Setup: checkExitCodeMeanings(%s) = %s, want the committed table to differ from the plant in one thing", exitCodesPath, joined(got))
	}
	if len(table.ExitCodes) < 2 {
		t.Fatalf("Setup: len(ExitCodes(%s)) = %d, want at least two rows to plant a repeat", exitCodesPath, len(table.ExitCodes))
	}

	planted := exitCodeTable{Description: table.Description, ExitCodes: slices.Clone(table.ExitCodes)}
	planted.ExitCodes[1].Meaning = planted.ExitCodes[0].Meaning
	want := []plantedRule{{sentinel: errMeaningShared, names: planted.ExitCodes[0].Meaning}}

	got := checkExitCodeMeanings(planted)
	if missing := missingRules(got, want); len(missing) != 0 {
		t.Errorf("checkExitCodeMeanings(%s with %s carrying %s's meaning) = %s, want %v, missing %v", exitCodesPath, planted.ExitCodes[1].Name, planted.ExitCodes[0].Name, joined(got), want, missing)
	}
	if surplus := surplusErrors(got, want); len(surplus) != 0 {
		t.Errorf("checkExitCodeMeanings(%s with one planted repeat) = %s, want only %v, surplus %s", exitCodesPath, joined(got), want, joined(surplus))
	}
}

// checkExemptionClasses reports every class token more than one row of the
// exemption document declares: one class name belongs to one class.
func checkExemptionClasses(doc exemptionsDocument) []error {
	rows := make(map[string]int, len(doc.Exemptions))
	for _, c := range doc.Exemptions {
		rows[c.Class]++
	}
	var errs []error
	for _, class := range slices.Sorted(maps.Keys(rows)) {
		if rows[class] > 1 {
			errs = append(errs, fmt.Errorf("%w: class %q declared by %d rows, want 1", errClassDeclaredTwice, class, rows[class]))
		}
	}
	return errs
}

func TestVocabularyExemptionClassesRefuseAPlantedRepeat(t *testing.T) {
	doc := loadExemptions(t)
	if got := checkExemptionClasses(doc); len(got) != 0 {
		t.Fatalf("Setup: checkExemptionClasses(%s) = %s, want the committed document to differ from the plant in one thing", exemptionsPath, joined(got))
	}

	planted := exemptionsDocument{Description: doc.Description, Exemptions: append(slices.Clone(doc.Exemptions), doc.Exemptions[0])}
	want := []plantedRule{{sentinel: errClassDeclaredTwice, names: doc.Exemptions[0].Class}}

	got := checkExemptionClasses(planted)
	if missing := missingRules(got, want); len(missing) != 0 {
		t.Errorf("checkExemptionClasses(%s with %q declared twice) = %s, want %v, missing %v", exemptionsPath, doc.Exemptions[0].Class, joined(got), want, missing)
	}
	if surplus := surplusErrors(got, want); len(surplus) != 0 {
		t.Errorf("checkExemptionClasses(%s with one planted repeat) = %s, want only %v, surplus %s", exemptionsPath, joined(got), want, joined(surplus))
	}
}

// liveCodes returns the codes the vocabulary assigns to a live kind.
func liveCodes(doc kindsDocument) map[string]bool {
	codes := make(map[string]bool, len(doc.Kinds))
	for _, k := range doc.Kinds {
		codes[k.Code] = true
	}
	return codes
}

// unreportable lists, in order and each once, the report values an expectation
// names that are neither the literal none nor the code of a live kind: a
// retired code, a code the space never assigned, and anything that is not a
// code at all.
func unreportable(rows []expectationRow, live map[string]bool) []string {
	var refused []string
	for _, row := range rows {
		if row.Report == "none" || live[row.Report] {
			continue
		}
		if !slices.Contains(refused, row.Report) {
			refused = append(refused, row.Report)
		}
	}
	return refused
}

func TestVocabularyExpectationReportCodesNameALiveKind(t *testing.T) {
	live := liveCodes(loadKinds(t))

	// Every fixture on disk. There may be none yet; the planted expectations
	// below keep the resolution under test either way.
	fixtures, err := fs.Glob(spec.Corpus, path.Join(fixturesDir, "*", expectFile))
	if err != nil {
		t.Fatalf("Setup: fs.Glob(Corpus, %s): %v", path.Join(fixturesDir, "*", expectFile), err)
	}
	for _, p := range fixtures {
		t.Run("fixture_"+path.Base(path.Dir(p)), func(t *testing.T) {
			data, err := fs.ReadFile(spec.Corpus, p)
			if err != nil {
				t.Fatalf("Setup: fs.ReadFile(Corpus, %q): %v", p, err)
			}
			var doc expectationDocument
			if err := decodeStrict(data, &doc); err != nil {
				t.Fatalf("Setup: decoding %s: %v", p, err)
			}
			if got := unreportable(doc.Expect, live); len(got) != 0 {
				t.Errorf("unreportable(%q) = %v, want every report code a live row of %s", p, got, kindsPath)
			}
		})
	}

	planted := []struct {
		name string
		rows []expectationRow
		want []string
	}{
		{
			name: "a_live_code",
			rows: []expectationRow{{Symbol: "DeadExport", Report: "DS1001", Confidence: "certain"}},
		},
		{
			name: "the_literal_none",
			rows: []expectationRow{{Symbol: "SatisfiesWriter", Report: "none", RetainedBy: []string{"interface-satisfaction"}}},
		},
		{
			name: "a_retired_code",
			rows: []expectationRow{{Symbol: "Forwarder", Report: "DS1402"}},
			want: []string{"DS1402"},
		},
		{
			name: "a_code_retired_from_a_live_range",
			rows: []expectationRow{{Symbol: "WorkspaceUse", Report: "DS1607"}},
			want: []string{"DS1607"},
		},
		{
			name: "a_code_inside_the_retired_range",
			rows: []expectationRow{{Symbol: "Alias", Report: "DS1450"}},
			want: []string{"DS1450"},
		},
		{
			name: "a_code_the_space_never_assigned",
			rows: []expectationRow{{Symbol: "Whatever", Report: "DS1999"}},
			want: []string{"DS1999"},
		},
		{
			name: "a_kind_name_instead_of_a_code",
			rows: []expectationRow{{Symbol: "DeadExport", Report: "unused-exported"}},
			want: []string{"unused-exported"},
		},
		{
			name: "a_retired_code_beside_a_live_one",
			rows: []expectationRow{
				{Symbol: "DeadExport", Report: "DS1001"},
				{Symbol: "Forwarder", Report: "DS1402"},
				{Symbol: "AlsoForwarder", Report: "DS1402"},
			},
			want: []string{"DS1402"},
		},
	}
	for _, tc := range planted {
		t.Run("planted_"+tc.name, func(t *testing.T) {
			if got := unreportable(tc.rows, live); !slices.Equal(got, tc.want) {
				t.Errorf("unreportable(%+v) = %v, want %v", tc.rows, got, tc.want)
			}
		})
	}
}

// relationOnALiveSubject lists the report codes of the rows that name a liveness
// relation for a code whose finding carries none, so the expectation could never
// be answered.
func relationOnALiveSubject(rows []expectationRow) []string {
	var refused []string
	for _, row := range rows {
		if row.LivenessRelation == "" || !slices.Contains(findingLiveSubjectCodes, row.Report) {
			continue
		}
		if !slices.Contains(refused, row.Report) {
			refused = append(refused, row.Report)
		}
	}
	return refused
}

// TestVocabularyExpectationsNameNoLivenessRelationForALiveSubject pins the
// corpus against the finding schema's own rule: a finding whose subject the
// analysis holds live carries no liveness relation, so an expectation naming one
// for such a code asks for a field the answer cannot hold.
func TestVocabularyExpectationsNameNoLivenessRelationForALiveSubject(t *testing.T) {
	fixtures, err := fs.Glob(spec.Corpus, path.Join(fixturesDir, "*", expectFile))
	if err != nil {
		t.Fatalf("Setup: fs.Glob(Corpus, %s): %v", path.Join(fixturesDir, "*", expectFile), err)
	}
	for _, p := range fixtures {
		t.Run("fixture_"+path.Base(path.Dir(p)), func(t *testing.T) {
			data, err := fs.ReadFile(spec.Corpus, p)
			if err != nil {
				t.Fatalf("Setup: fs.ReadFile(Corpus, %q): %v", p, err)
			}
			var doc expectationDocument
			if err := decodeStrict(data, &doc); err != nil {
				t.Fatalf("Setup: decoding %s: %v", p, err)
			}
			if got := relationOnALiveSubject(doc.Expect); len(got) != 0 {
				t.Errorf("relationOnALiveSubject(%q) = %v, want no liveness relation under a code whose subject %s holds live", p, got, findingSchemaPath)
			}
		})
	}

	planted := []struct {
		name string
		rows []expectationRow
		want []string
	}{
		{
			name: "a_relation_on_a_dead_subject",
			rows: []expectationRow{{Symbol: "DeadExport", Report: "DS1001", LivenessRelation: "reference-counting"}},
		},
		{
			name: "no_relation_on_a_live_subject",
			rows: []expectationRow{{Symbol: "UnreadPrivate", Report: "DS1301", Confidence: "certain"}},
		},
		{
			name: "a_relation_on_a_write_only_subject",
			rows: []expectationRow{{Symbol: "UnreadPrivate", Report: "DS1301", LivenessRelation: "reference-counting"}},
			want: []string{"DS1301"},
		},
		{
			name: "a_relation_on_a_narrowing_subject",
			rows: []expectationRow{{Symbol: "Normalize", Report: "DS1101", LivenessRelation: "reachability"}},
			want: []string{"DS1101"},
		},
	}
	for _, tc := range planted {
		t.Run("planted_"+tc.name, func(t *testing.T) {
			if got := relationOnALiveSubject(tc.rows); !slices.Equal(got, tc.want) {
				t.Errorf("relationOnALiveSubject(%+v) = %v, want %v", tc.rows, got, tc.want)
			}
		})
	}
}

// severityKeyErrors reports every key of a configuration's severity object the
// vocabulary does not resolve: a key the schema's own pattern refuses, a code
// no live kind carries, a two-digit family prefix no live range carries, and a
// code whose severity the Contract fixes.
func severityKeyErrors(severity map[string]string, doc kindsDocument, keyShape *regexp.Regexp) []error {
	retired := make(map[string]bool, len(doc.Retired))
	for _, r := range doc.Retired {
		retired[r.Code] = true
	}
	var errs []error
	for _, key := range slices.Sorted(maps.Keys(severity)) {
		if !keyShape.MatchString(key) {
			errs = append(errs, fmt.Errorf("%w: severity[%q], want a key matching %s", errSeverityKeyShape, key, keyShape))
			continue
		}
		if digits := strings.TrimPrefix(key, doc.Prefix); len(digits) == 2 {
			if err := familyKeyError(key, digits, doc); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		i := slices.IndexFunc(doc.Kinds, func(k kindRow) bool { return k.Code == key })
		switch {
		case i < 0 && retired[key]:
			errs = append(errs, fmt.Errorf("%w: severity[%q], want a code %s assigns to a live kind", errSeverityKeyRetired, key, kindsPath))
		case i < 0:
			errs = append(errs, fmt.Errorf("%w: severity[%q], want a code %s assigns to a live kind", errSeverityKeyUnknown, key, kindsPath))
		case isSet(doc.Kinds[i].Fixed) && *doc.Kinds[i].Fixed:
			errs = append(errs, fmt.Errorf("%w: severity[%q] = %q, want the key absent", errSeverityKeyFixed, key, severity[key]))
		}
	}
	return errs
}

// familyKeyError resolves a two-digit family prefix against the ranges: the
// prefix names the hundred it spans, which must be a live range.
func familyKeyError(key, digits string, doc kindsDocument) error {
	hundred, err := strconv.Atoi(digits)
	if err != nil {
		return fmt.Errorf("%w: severity[%q], want two digits after %s", errSeverityKeyShape, key, doc.Prefix)
	}
	low, high := hundred*100, hundred*100+99
	i := slices.IndexFunc(doc.Ranges, func(r codeRange) bool {
		start, end, ok := span(r)
		return ok && start == low && end == high
	})
	switch {
	case i < 0:
		return fmt.Errorf("%w: severity[%q], want a family %s declares as a range", errSeverityKeyUnknown, key, kindsPath)
	case isRetired(doc.Ranges[i]):
		return fmt.Errorf("%w: severity[%q] = range %s..%s, want a live range", errSeverityKeyRetired, key, doc.Ranges[i].Start, doc.Ranges[i].End)
	}
	return nil
}

// severityKeyShape returns the key pattern contract/config.schema.json itself
// declares for the severity object, so this suite resolves the keys the schema
// admits rather than a shape of its own.
func severityKeyShape(t *testing.T) *regexp.Regexp {
	t.Helper()
	props, ok := loadConfigSchema(t)["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Setup: %s has no properties object", configSchemaPath)
	}
	severity, ok := props["severity"].(schemaNode)
	if !ok {
		t.Fatalf("Setup: %s declares no severity object", configSchemaPath)
	}
	pats, ok := severity["patternProperties"].(map[string]any)
	if !ok {
		t.Fatalf("Setup: %s severity declares no patternProperties", configSchemaPath)
	}
	keys := slices.Sorted(maps.Keys(pats))
	if len(keys) != 1 {
		t.Fatalf("Setup: %s severity patternProperties = %v, want one key pattern", configSchemaPath, keys)
	}
	shape, err := regexp.Compile(keys[0])
	if err != nil {
		t.Fatalf("Setup: regexp.Compile(%q): %v", keys[0], err)
	}
	return shape
}

func TestVocabularySeverityKeysNameALiveKindOrFamily(t *testing.T) {
	doc := loadKinds(t)
	shape := severityKeyShape(t)
	cases := []struct {
		name   string
		config string
		want   []plantedRule
	}{
		{
			name:   "a_live_code",
			config: `{"target":{"kind":"library"},"severity":{"DS1101":"warn"}}`,
		},
		{
			name:   "a_live_family_prefix",
			config: `{"target":{"kind":"library"},"severity":{"DS18":"allow"}}`,
		},
		{
			name:   "a_retired_code",
			config: `{"target":{"kind":"library"},"severity":{"DS1402":"warn"}}`,
			want:   []plantedRule{{sentinel: errSeverityKeyRetired, names: "DS1402"}},
		},
		{
			name:   "a_code_retired_from_a_live_range",
			config: `{"target":{"kind":"library"},"severity":{"DS1607":"deny"}}`,
			want:   []plantedRule{{sentinel: errSeverityKeyRetired, names: "DS1607"}},
		},
		{
			name:   "the_retired_family_prefix",
			config: `{"target":{"kind":"library"},"severity":{"DS14":"allow"}}`,
			want:   []plantedRule{{sentinel: errSeverityKeyRetired, names: "DS14"}},
		},
		{
			name:   "a_code_the_space_never_assigned",
			config: `{"target":{"kind":"library"},"severity":{"DS1999":"warn"}}`,
			want:   []plantedRule{{sentinel: errSeverityKeyUnknown, names: "DS1999"}},
		},
		{
			name:   "a_family_prefix_no_range_declares",
			config: `{"target":{"kind":"library"},"severity":{"DS19":"warn"}}`,
			want:   []plantedRule{{sentinel: errSeverityKeyUnknown, names: "DS19"}},
		},
		{
			name:   "the_code_whose_severity_is_fixed",
			config: `{"target":{"kind":"library"},"severity":{"DS1703":"allow"}}`,
			want:   []plantedRule{{sentinel: errSeverityKeyFixed, names: "DS1703"}},
		},
		{
			name:   "a_key_of_the_wrong_shape",
			config: `{"target":{"kind":"library"},"severity":{"DS1":"warn"}}`,
			want:   []plantedRule{{sentinel: errSeverityKeyShape, names: "DS1"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var planted struct {
				Severity map[string]string `json:"severity"`
			}
			if err := json.Unmarshal([]byte(tc.config), &planted); err != nil {
				t.Fatalf("Setup: json.Unmarshal(%s): %v", tc.config, err)
			}
			got := severityKeyErrors(planted.Severity, doc, shape)
			if missing := missingRules(got, tc.want); len(missing) != 0 {
				t.Errorf("severityKeyErrors(%s) = %s, want %v, missing %v", tc.config, joined(got), tc.want, missing)
			}
			if surplus := surplusErrors(got, tc.want); len(surplus) != 0 {
				t.Errorf("severityKeyErrors(%s) = %s, want only %v, surplus %s", tc.config, joined(got), tc.want, joined(surplus))
			}
		})
	}
}
