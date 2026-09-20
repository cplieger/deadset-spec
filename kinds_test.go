package spec_test

import (
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

	"github.com/cplieger/deadset-spec/v2"
)

const kindsPath = "contract/kinds.json"

// The rules that govern the code space as a whole are pure functions over a
// decoded document, so the committed document and a planted copy carrying one
// violation go through the same code; vocabulary_test.go drives the planted
// copies. Each violation is one error wrapping the rule's sentinel, so a
// caller names both the rule and the code it caught.
var (
	errCodeShape             = errors.New("code is not the prefix followed by four digits")
	errCodeAssignedTwice     = errors.New("code assigned to two live kinds")
	errRangeSpan             = errors.New("range span is not two codes with start at or before end")
	errRangesStraddle        = errors.New("two ranges cover one code")
	errCodeOutsideRanges     = errors.New("code sits in no range")
	errCodeInSeveralRanges   = errors.New("code sits in more than one range")
	errRetiredCodeReused     = errors.New("live kind carries a retired code")
	errLiveRowInRetiredRange = errors.New("live kind sits in a retired range")
)

// kindsDocument mirrors contract/kinds.json. Booleans are pointers so an
// absent field is distinguishable from a false one.
type kindsDocument struct {
	Prefix              string        `json:"prefix"`
	Languages           []string      `json:"languages"`
	Severities          []string      `json:"severities"`
	ReachabilityClasses []string      `json:"reachability_classes"`
	Fixabilities        []string      `json:"fixabilities"`
	Ranges              []codeRange   `json:"ranges"`
	Kinds               []kindRow     `json:"kinds"`
	Retired             []retiredKind `json:"retired"`
}

type codeRange struct {
	Retired     *bool  `json:"retired"`
	Start       string `json:"start"`
	End         string `json:"end"`
	Family      string `json:"family"`
	Description string `json:"description"`
}

type kindRow struct {
	DefaultEnabled  *bool               `json:"default_enabled"`
	Fixed           *bool               `json:"fixed"`
	Overlap         map[string][]string `json:"overlap"`
	Code            string              `json:"code"`
	Name            string              `json:"name"`
	Rule            string              `json:"rule"`
	DefaultSeverity string              `json:"default_severity"`
	MaxClass        string              `json:"max_class"`
	Fixability      string              `json:"fixability"`
	Precondition    string              `json:"precondition"`
	Languages       []string            `json:"languages"`
	DerivedFrom     []string            `json:"derived_from"`
}

// retiredKind mirrors a row of the retired list, which carries the code and
// the name of the kind that once held it and no other field.
type retiredKind struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

var codePattern = regexp.MustCompile(`^DS[0-9]{4}$`)

// loadKinds decodes the embedded kinds document; a decode failure is a setup
// failure, not a vocabulary finding.
func loadKinds(t *testing.T) kindsDocument {
	t.Helper()
	data, err := fs.ReadFile(spec.Contract, kindsPath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", kindsPath, err)
	}
	var doc kindsDocument
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Setup: json.Unmarshal(%q): %v", kindsPath, err)
	}
	return doc
}

// codeNumber returns the four-digit part of a DSnnnn code, or -1 when the
// code does not have that shape.
func codeNumber(code string) int {
	if !codePattern.MatchString(code) {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimPrefix(code, "DS"))
	if err != nil {
		return -1
	}
	return n
}

// rangesContaining lists the ranges whose span covers n.
func rangesContaining(ranges []codeRange, n int) []codeRange {
	var hits []codeRange
	for _, r := range ranges {
		if codeNumber(r.Start) <= n && n <= codeNumber(r.End) {
			hits = append(hits, r)
		}
	}
	return hits
}

// isSet reports whether a pointer boolean field was present in the document.
func isSet(b *bool) bool { return b != nil }

// optional renders a pointer field for a failure message: its value, or
// "absent" when the document omitted it.
func optional[T any](p *T) string {
	if p == nil {
		return "absent"
	}
	return fmt.Sprint(*p)
}

// span returns a range's numeric bounds, and false when either end is not a
// code or the range runs backwards.
func span(r codeRange) (start, end int, ok bool) {
	start, end = codeNumber(r.Start), codeNumber(r.End)
	return start, end, start >= 0 && end >= start
}

// isRetired reports whether a range is marked retired; an absent marker reads
// as live, and the field's presence is checked separately.
func isRetired(r codeRange) bool { return isSet(r.Retired) && *r.Retired }

// checkCodeAssignment reports every live row whose code is not a code of the
// space and every code two live rows carry: one code names at most one kind.
func checkCodeAssignment(doc kindsDocument) []error {
	var errs []error
	rows := make(map[string]int, len(doc.Kinds))
	for _, k := range doc.Kinds {
		if codeNumber(k.Code) < 0 {
			errs = append(errs, fmt.Errorf("%w: Kind(%q).code = %q, want %s followed by four digits", errCodeShape, k.Name, k.Code, doc.Prefix))
			continue
		}
		rows[k.Code]++
	}
	for _, code := range slices.Sorted(maps.Keys(rows)) {
		if rows[code] > 1 {
			errs = append(errs, fmt.Errorf("%w: Kind(%s) live rows = %d, want 1", errCodeAssignedTwice, code, rows[code]))
		}
	}
	return errs
}

// checkRangeAssignment reports a range whose span is not two ordered codes,
// two ranges that cover one code, and every live code that does not sit in
// exactly one range.
func checkRangeAssignment(doc kindsDocument) []error {
	var errs []error
	for _, r := range doc.Ranges {
		if _, _, ok := span(r); !ok {
			errs = append(errs, fmt.Errorf("%w: Range(%s) span = %q..%q, want two %s codes with start at or before end", errRangeSpan, r.Start, r.Start, r.End, doc.Prefix))
		}
	}
	for i, outer := range doc.Ranges {
		outerStart, outerEnd, outerOK := span(outer)
		for _, inner := range doc.Ranges[i+1:] {
			innerStart, innerEnd, innerOK := span(inner)
			if !outerOK || !innerOK {
				continue
			}
			if outerStart <= innerEnd && innerStart <= outerEnd {
				errs = append(errs, fmt.Errorf("%w: Range(%s) = %s..%s and Range(%s) = %s..%s, want one range per code", errRangesStraddle, outer.Start, outer.Start, outer.End, inner.Start, inner.Start, inner.End))
			}
		}
	}
	for _, k := range doc.Kinds {
		n := codeNumber(k.Code)
		if n < 0 {
			continue // checkCodeAssignment owns the shape of a code.
		}
		hits := rangesContaining(doc.Ranges, n)
		switch {
		case len(hits) == 0:
			errs = append(errs, fmt.Errorf("%w: Kind(%s) ranges = 0, want 1", errCodeOutsideRanges, k.Code))
		case len(hits) > 1:
			errs = append(errs, fmt.Errorf("%w: Kind(%s) ranges = %d (%s, %s), want 1", errCodeInSeveralRanges, k.Code, len(hits), hits[0].Start, hits[1].Start))
		}
	}
	return errs
}

// checkRetirement reports every live row carrying a retired code and every
// live row inside a retired range: a code leaves the space once and never
// names another kind afterwards.
func checkRetirement(doc kindsDocument) []error {
	retired := make(map[string]retiredKind, len(doc.Retired))
	for _, r := range doc.Retired {
		retired[r.Code] = r
	}
	var errs []error
	for _, k := range doc.Kinds {
		if r, ok := retired[k.Code]; ok {
			errs = append(errs, fmt.Errorf("%w: Kind(%s) = %q, want the code left retired from %q", errRetiredCodeReused, k.Code, k.Name, r.Name))
		}
		for _, hit := range rangesContaining(doc.Ranges, codeNumber(k.Code)) {
			if isRetired(hit) {
				errs = append(errs, fmt.Errorf("%w: Kind(%s) range = %s..%s, want a live range", errLiveRowInRetiredRange, k.Code, hit.Start, hit.End))
			}
		}
	}
	return errs
}

func TestKindsRangesAreEightLiveAndOneRetired(t *testing.T) {
	doc := loadKinds(t)
	var live, retired int
	for _, r := range doc.Ranges {
		t.Run(r.Start, func(t *testing.T) {
			if r.Family == "" || r.Description == "" || !isSet(r.Retired) {
				t.Errorf("Range(%s) fields = family %q, description %q, retired %s, want every field present", r.Start, r.Family, r.Description, optional(r.Retired))
			}
			if isRetired(r) {
				retired++
			} else {
				live++
			}
		})
	}
	if live != 8 || retired != 1 {
		t.Errorf("Ranges(%s) = %d live, %d retired, want 8 live, 1 retired", kindsPath, live, retired)
	}
}

// TestKindsCodesAreAssignedOnce drives the code-assignment rule over the
// committed document; vocabulary_test.go drives it over a planted duplicate.
func TestKindsCodesAreAssignedOnce(t *testing.T) {
	for _, err := range checkCodeAssignment(loadKinds(t)) {
		t.Errorf("checkCodeAssignment(%s): %v", kindsPath, err)
	}
}

// TestKindsRangesCoverEveryLiveCodeOnce drives the range-assignment rule over
// the committed document; vocabulary_test.go drives it over a planted straddle.
func TestKindsRangesCoverEveryLiveCodeOnce(t *testing.T) {
	for _, err := range checkRangeAssignment(loadKinds(t)) {
		t.Errorf("checkRangeAssignment(%s): %v", kindsPath, err)
	}
}

// TestKindsRetiredCodesAreNotReassigned drives the retirement rule over the
// committed document; vocabulary_test.go drives it over planted re-assignments.
func TestKindsRetiredCodesAreNotReassigned(t *testing.T) {
	for _, err := range checkRetirement(loadKinds(t)) {
		t.Errorf("checkRetirement(%s): %v", kindsPath, err)
	}
}

func TestKindsEveryLiveRowIsCompleteAndInVocabulary(t *testing.T) {
	doc := loadKinds(t)
	if got := len(doc.Kinds); got != 31 {
		t.Errorf("len(Kinds(%s)) = %d, want 31", kindsPath, got)
	}
	for _, k := range doc.Kinds {
		t.Run(k.Code, func(t *testing.T) {
			missing := missingKindFields(k)
			if len(missing) != 0 {
				t.Errorf("Kind(%s) missing fields = %v, want none", k.Code, missing)
			}
			if !slices.Contains(doc.Severities, k.DefaultSeverity) {
				t.Errorf("Kind(%s).default_severity = %q, want one of %v", k.Code, k.DefaultSeverity, doc.Severities)
			}
			if !slices.Contains(doc.ReachabilityClasses, k.MaxClass) {
				t.Errorf("Kind(%s).max_class = %q, want one of %v", k.Code, k.MaxClass, doc.ReachabilityClasses)
			}
			if !slices.Contains(doc.Fixabilities, k.Fixability) {
				t.Errorf("Kind(%s).fixability = %q, want one of %v", k.Code, k.Fixability, doc.Fixabilities)
			}
			for _, lang := range k.Languages {
				if !slices.Contains(doc.Languages, lang) {
					t.Errorf("Kind(%s).languages contains %q, want only %v", k.Code, lang, doc.Languages)
				}
			}
			for _, from := range k.DerivedFrom {
				if !slices.ContainsFunc(doc.Kinds, func(o kindRow) bool { return o.Code == from }) {
					t.Errorf("Kind(%s).derived_from names %q, want a live code", k.Code, from)
				}
			}
		})
	}
}

// missingKindFields names the fields every live row must carry that k lacks.
func missingKindFields(k kindRow) []string {
	var missing []string
	if k.Name == "" {
		missing = append(missing, "name")
	}
	if len(k.Languages) == 0 {
		missing = append(missing, "languages")
	}
	if k.Rule == "" {
		missing = append(missing, "rule")
	}
	if !isSet(k.DefaultEnabled) {
		missing = append(missing, "default_enabled")
	}
	if k.DefaultSeverity == "" {
		missing = append(missing, "default_severity")
	}
	if k.MaxClass == "" {
		missing = append(missing, "max_class")
	}
	if k.Fixability == "" {
		missing = append(missing, "fixability")
	}
	if !isSet(k.Fixed) {
		missing = append(missing, "fixed")
	}
	return missing
}

// fixedKinds are the two self-check kinds a configuration cannot move: a
// suppression and a configured root that name something no longer present
// mean the same thing and carry the same posture.
var fixedKinds = []string{"DS1703", "DS1704"}

func TestKindsOnlyTheStaleConfigurationKindsAreFixed(t *testing.T) {
	doc := loadKinds(t)
	for _, k := range doc.Kinds {
		t.Run(k.Code, func(t *testing.T) {
			want := slices.Contains(fixedKinds, k.Code)
			if !isSet(k.Fixed) || *k.Fixed != want {
				t.Errorf("Kind(%s).fixed = %s, want %v", k.Code, optional(k.Fixed), want)
			}
			if want && (!isSet(k.DefaultEnabled) || !*k.DefaultEnabled || k.DefaultSeverity != "deny") {
				t.Errorf("Kind(%s) enablement = %s at %q, want fixed on at \"deny\"", k.Code, optional(k.DefaultEnabled), k.DefaultSeverity)
			}
		})
	}
}

// narrowedKinds are the rows whose rule holds only under a stated condition;
// each must carry the precondition the analyzer checks before it reports.
var narrowedKinds = []string{"DS1101", "DS1102", "DS1104", "DS1203", "DS1303", "DS1501", "DS1601", "DS1605"}

func TestKindsNarrowedRowsCarryAPrecondition(t *testing.T) {
	doc := loadKinds(t)
	for _, code := range narrowedKinds {
		t.Run(code, func(t *testing.T) {
			i := slices.IndexFunc(doc.Kinds, func(k kindRow) bool { return k.Code == code })
			if i < 0 {
				t.Fatalf("Kind(%s) = absent, want a live row", code)
			}
			if doc.Kinds[i].Precondition == "" {
				t.Errorf("Kind(%s).precondition = %q, want a non-empty condition", code, doc.Kinds[i].Precondition)
			}
		})
	}
}

// overlapSentinels are the two values an overlap list may carry instead of
// linter names; each must stand alone.
var overlapSentinels = []string{"none known", "not applicable"}

func TestKindsIntraFunctionRowsNameOverlapPerLanguage(t *testing.T) {
	doc := loadKinds(t)
	var group int
	for _, k := range doc.Kinds {
		t.Run(k.Code, func(t *testing.T) {
			n := codeNumber(k.Code)
			inGroup := 1800 <= n && n <= 1899
			if !inGroup {
				if k.Overlap != nil {
					t.Errorf("Kind(%s).overlap = %v, want absent outside the intra-function group", k.Code, k.Overlap)
				}
				return
			}
			group++
			if got := slices.Sorted(maps.Keys(k.Overlap)); !slices.Equal(got, doc.Languages) {
				t.Fatalf("Kind(%s).overlap languages = %v, want %v", k.Code, got, doc.Languages)
			}
			for _, lang := range doc.Languages {
				checkOverlapList(t, k, lang)
			}
		})
	}
	if group != 6 {
		t.Errorf("IntraFunctionKinds(%s) = %d, want 6", kindsPath, group)
	}
}

// checkOverlapList reports the ways one language's overlap list can be wrong:
// empty, an empty name, a sentinel beside a name, or a sentinel that
// disagrees with the row's language set.
func checkOverlapList(t *testing.T, k kindRow, lang string) {
	t.Helper()
	list := k.Overlap[lang]
	if len(list) == 0 {
		t.Errorf("Kind(%s).overlap[%q] = %v, want at least one linter or a sentinel", k.Code, lang, list)
		return
	}
	if slices.Contains(list, "") {
		t.Errorf("Kind(%s).overlap[%q] = %q, want no empty name", k.Code, lang, list)
	}
	sentinel := slices.ContainsFunc(list, func(s string) bool { return slices.Contains(overlapSentinels, s) })
	if sentinel && len(list) != 1 {
		t.Errorf("Kind(%s).overlap[%q] = %q, want a sentinel to stand alone", k.Code, lang, list)
	}
	applies := slices.Contains(k.Languages, lang)
	notApplicable := slices.Equal(list, []string{"not applicable"})
	if applies == notApplicable {
		t.Errorf("Kind(%s).overlap[%q] = %q with languages %v, want \"not applicable\" exactly when the kind does not apply to %s", k.Code, lang, list, k.Languages, lang)
	}
}

// retiredCodes are the nineteen codes the vocabulary has retired. A code may
// leave this list only by being re-admitted under its own name, never by
// re-use for another kind.
var retiredCodes = []string{
	"DS1202", "DS1401", "DS1402",
	"DS1503", "DS1504", "DS1505", "DS1506", "DS1507",
	"DS1602", "DS1603", "DS1604", "DS1606",
	"DS1607", "DS1608",
	"DS1804", "DS1806", "DS1808", "DS1810", "DS1811",
}

func TestKindsRetiredCodesStayRetired(t *testing.T) {
	doc := loadKinds(t)
	if got := len(doc.Retired); got != len(retiredCodes) {
		t.Errorf("len(Retired(%s)) = %d, want %d", kindsPath, got, len(retiredCodes))
	}
	listed := make(map[string]retiredKind, len(doc.Retired))
	for _, r := range doc.Retired {
		listed[r.Code] = r
	}
	for _, code := range retiredCodes {
		t.Run(code, func(t *testing.T) {
			r, ok := listed[code]
			if !ok {
				t.Fatalf("Retired(%s) = absent, want a retired row", code)
			}
			if r.Name == "" {
				t.Errorf("Retired(%s).name = %q, want the name of the kind that once held the code", code, r.Name)
			}
			if hits := rangesContaining(doc.Ranges, codeNumber(code)); len(hits) != 1 {
				t.Errorf("Retired(%s) ranges = %d, want exactly 1", code, len(hits))
			}
		})
	}
	for _, r := range doc.Retired {
		if !slices.Contains(retiredCodes, r.Code) {
			t.Errorf("Retired(%s) lists %s, want only the nineteen retired codes", kindsPath, r.Code)
		}
	}
}

// TestKindsRetiredRowsCarryCodeAndNameOnly pins the retired-row shape: a
// retired code is listed by its code and the name of the kind that once held
// it, and by no other field.
func TestKindsRetiredRowsCarryCodeAndNameOnly(t *testing.T) {
	data, err := fs.ReadFile(spec.Contract, kindsPath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", kindsPath, err)
	}
	var doc struct {
		Retired []map[string]json.RawMessage `json:"retired"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Setup: json.Unmarshal(%q): %v", kindsPath, err)
	}
	want := []string{"code", "name"}
	for _, row := range doc.Retired {
		code := strings.Trim(string(row["code"]), `"`)
		t.Run(code, func(t *testing.T) {
			if got := slices.Sorted(maps.Keys(row)); !slices.Equal(got, want) {
				t.Errorf("Retired(%s) keys = %v, want %v", code, got, want)
			}
		})
	}
}

// TestKindsTheOnlyRetiredRangeIsDS1400 pins which range is retired; that no
// live row sits inside it is the retirement rule, driven above.
func TestKindsTheOnlyRetiredRangeIsDS1400(t *testing.T) {
	doc := loadKinds(t)
	for _, r := range doc.Ranges {
		if !isRetired(r) {
			continue
		}
		if r.Start != "DS1400" || r.End != "DS1499" {
			t.Errorf("RetiredRange(%s) = %s..%s, want DS1400..DS1499 as the only retired range", r.Start, r.Start, r.End)
		}
	}
}

func TestKindsNoLiveRowDeclaresAClassBelowCertain(t *testing.T) {
	doc := loadKinds(t)
	for _, k := range doc.Kinds {
		t.Run(k.Code, func(t *testing.T) {
			if k.MaxClass != "certain" {
				t.Errorf("Kind(%s).max_class = %q, want %q", k.Code, k.MaxClass, "certain")
			}
		})
	}
}
