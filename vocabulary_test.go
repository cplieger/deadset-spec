package spec_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	spec "github.com/cplieger/deadset-spec/v8"
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
	return codeRange{Start: start, End: end, Family: family, Title: "Planted", Question: "A planted question.", Description: "A planted range.", Status: "live"}
}

// plantReservedDS19 declares DS1900-DS1999 reserved, before the configuration
// range, as the family a kind has not yet been assigned to.
func plantReservedDS19(doc *kindsDocument) {
	reserved := liveCodeRange("DS1900", "DS1999", "planted-reserved")
	reserved.Status = "reserved"
	i := slices.IndexFunc(doc.Ranges, func(r codeRange) bool { return r.Start == "DS9000" })
	doc.Ranges = slices.Insert(doc.Ranges, i, reserved)
}

// rangeAt is the index of the range starting at start.
func rangeAt(doc *kindsDocument, start string) int {
	return slices.IndexFunc(doc.Ranges, func(r codeRange) bool { return r.Start == start })
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
				plantReservedDS19(doc)
				doc.Ranges[rangeAt(doc, "DS1900")].End = "DS1850"
			},
			want: []plantedRule{{sentinel: errRangeSpan, names: `"DS1900".."DS1850"`}},
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
			name: "a_live_row_at_a_retired_code",
			plant: func(doc *kindsDocument) {
				doc.Retired = append(doc.Retired, retiredKind{Code: "DS1010", Name: "planted-gone"})
				doc.Kinds = append(doc.Kinds, liveKindRow("DS1010", "planted-resurrection"))
			},
			want: []plantedRule{{sentinel: errRetiredCodeReused, names: "DS1010"}},
		},
		{
			name: "a_live_row_inside_a_retired_range",
			plant: func(doc *kindsDocument) {
				plantReservedDS19(doc)
				doc.Ranges[rangeAt(doc, "DS1900")].Status = "retired"
				doc.Kinds = append(doc.Kinds, liveKindRow("DS1901", "planted-retired-range"))
			},
			want: []plantedRule{{sentinel: errLiveRowInRetiredRange, names: "DS1901"}},
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
	case doc.Ranges[i].Status != "live":
		return fmt.Errorf("%w: severity[%q] = range %s..%s, which holds no live kind, want a live range", errSeverityKeyUnknown, key, doc.Ranges[i].Start, doc.Ranges[i].End)
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
	shape := severityKeyShape(t)
	retireDS1010 := func(doc *kindsDocument) {
		doc.Retired = append(doc.Retired, retiredKind{Code: "DS1010", Name: "planted-gone"})
	}
	retireDS19 := func(doc *kindsDocument) {
		plantReservedDS19(doc)
		doc.Ranges[rangeAt(doc, "DS1900")].Status = "retired"
	}
	cases := []struct {
		plant  func(*kindsDocument)
		name   string
		config string
		want   []plantedRule
	}{
		{
			name:   "a_live_code",
			config: `{"target":{"kind":"library"},"severity":{"DS1501":"warn"}}`,
		},
		{
			name:   "a_live_family_prefix",
			config: `{"target":{"kind":"library"},"severity":{"DS14":"allow"}}`,
		},
		{
			name:   "a_retired_code",
			plant:  retireDS1010,
			config: `{"target":{"kind":"library"},"severity":{"DS1010":"warn"}}`,
			want:   []plantedRule{{sentinel: errSeverityKeyRetired, names: "DS1010"}},
		},
		{
			name:   "a_retired_family_prefix",
			plant:  retireDS19,
			config: `{"target":{"kind":"library"},"severity":{"DS19":"allow"}}`,
			want:   []plantedRule{{sentinel: errSeverityKeyRetired, names: "DS19"}},
		},
		{
			name:   "a_reserved_family_prefix",
			plant:  plantReservedDS19,
			config: `{"target":{"kind":"library"},"severity":{"DS19":"allow"}}`,
			want:   []plantedRule{{sentinel: errSeverityKeyUnknown, names: "DS19"}},
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
			config: `{"target":{"kind":"library"},"severity":{"DS9003":"allow"}}`,
			want:   []plantedRule{{sentinel: errSeverityKeyFixed, names: "DS9003"}},
		},
		{
			name:   "a_key_of_the_wrong_shape",
			config: `{"target":{"kind":"library"},"severity":{"DS1":"warn"}}`,
			want:   []plantedRule{{sentinel: errSeverityKeyShape, names: "DS1"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plant := tc.plant
			if plant == nil {
				plant = func(*kindsDocument) {}
			}
			doc := plantedKinds(t, plant)
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

// TestVocabularySeverityVectorsAgreeWithTheVocabulary reads the published configuration cases
// through the same resolver: a case refused at a severity key names a key the vocabulary refuses,
// and every severity key a resolved configuration prints is one it resolves.
func TestVocabularySeverityVectorsAgreeWithTheVocabulary(t *testing.T) {
	doc := loadKinds(t)
	shape := severityKeyShape(t)
	var refused int
	for _, dir := range vectorCaseDirs(t, spec.Vectors, "config") {
		t.Run(dir, func(t *testing.T) {
			if names, ok := strings.CutPrefix(refusalNamesIfAny(t, dir), "severity."); ok {
				refused++
				if errs := severityKeyErrors(map[string]string{names: "warn"}, doc, shape); len(errs) == 0 {
					t.Errorf("%s/%s refuses severity.%s, want a key %s resolves to nothing, and it resolves", configVectorsDir, dir, names, kindsPath)
				}
			}
			data, ok := readVectorFile(t, dir, vectorExpectedFile)
			if !ok {
				return
			}
			var resolved struct {
				Severity map[string]string `json:"severity"`
			}
			if err := json.Unmarshal(data, &resolved); err != nil {
				t.Fatalf("Setup: json.Unmarshal(%s/%s/%s): %v", configVectorsDir, dir, vectorExpectedFile, err)
			}
			if errs := severityKeyErrors(resolved.Severity, doc, shape); len(errs) != 0 {
				t.Errorf("%s/%s/%s prints severity keys the vocabulary refuses: %s", configVectorsDir, dir, vectorExpectedFile, joined(errs))
			}
		})
	}
	if refused == 0 {
		t.Errorf("no case under %s is refused at a severity key, want one naming a key no live kind carries", configVectorsDir)
	}
}

// refusalNamesIfAny returns the key a refused case names, and the empty string for a case that
// resolves.
func refusalNamesIfAny(t *testing.T, dir string) string {
	t.Helper()
	if _, ok := readVectorFile(t, dir, vectorErrorFile); !ok {
		return ""
	}
	return refusalNames(t, dir)
}
