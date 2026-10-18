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

	"github.com/cplieger/deadset-spec/v8"
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
	Severities          []string      `json:"severities"`
	ReachabilityClasses []string      `json:"reachability_classes"`
	Fixabilities        []string      `json:"fixabilities"`
	Subjects            []string      `json:"subjects"`
	Ranges              []codeRange   `json:"ranges"`
	Kinds               []kindRow     `json:"kinds"`
	Retired             []retiredKind `json:"retired"`
}

type codeRange struct {
	Start       string `json:"start"`
	End         string `json:"end"`
	Family      string `json:"family"`
	Title       string `json:"title"`
	Question    string `json:"question"`
	Status      string `json:"status"`
	Description string `json:"description"`
}

type kindRow struct {
	DefaultEnabled      *bool               `json:"default_enabled"`
	Fixed               *bool               `json:"fixed"`
	HoldsLive           *bool               `json:"holds_live"`
	Overlap             map[string][]string `json:"overlap"`
	Details             map[string]any      `json:"details"`
	Subject             string              `json:"subject"`
	Code                string              `json:"code"`
	Name                string              `json:"name"`
	Rule                string              `json:"rule"`
	DefaultSeverity     string              `json:"default_severity"`
	MaxClass            string              `json:"max_class"`
	Fixability          string              `json:"fixability"`
	Precondition        string              `json:"precondition"`
	Languages           []string            `json:"languages"`
	DerivedFrom         []string            `json:"derived_from"`
	FindingFixabilities []string            `json:"finding_fixabilities"`
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

// isRetired reports whether a range is retired: it held kinds and holds none
// again.
func isRetired(r codeRange) bool { return r.Status == "retired" }

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

// rangeStatuses are the states a family range takes: live while it holds a live
// kind, reserved while it holds none yet, retired once every kind it held is.
var rangeStatuses = []string{"live", "reserved", "retired"}

func TestKindsRangesAreFamiliesOfAHundred(t *testing.T) {
	doc := loadKinds(t)
	for _, r := range doc.Ranges {
		t.Run(r.Start, func(t *testing.T) {
			if r.Family == "" || r.Title == "" || r.Question == "" || r.Description == "" {
				t.Errorf("Range(%s) fields = family %q, title %q, question %q, description %q, want every field present", r.Start, r.Family, r.Title, r.Question, r.Description)
			}
			if !slices.Contains(rangeStatuses, r.Status) {
				t.Errorf("Range(%s).status = %q, want one of %v", r.Start, r.Status, rangeStatuses)
			}
			start, end := codeNumber(r.Start), codeNumber(r.End)
			if start%100 != 0 || end != start+99 {
				t.Errorf("Range(%s) = %s..%s, want a family of a hundred codes, DSFF00 to DSFF99", r.Start, r.Start, r.End)
			}
			var held []int
			for _, k := range doc.Kinds {
				if n := codeNumber(k.Code); start <= n && n <= end {
					held = append(held, n)
				}
			}
			if (len(held) > 0) != (r.Status == "live") {
				t.Errorf("Range(%s) holds %d live kinds with status %q, want live exactly when it holds one", r.Start, len(held), r.Status)
			}
			for i, n := range held {
				if n != start+1+i {
					t.Errorf("Range(%s) kind codes = %v, want contiguous codes from %s01", r.Start, held, r.Start[:4])
					break
				}
			}
		})
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
	if got := len(doc.Kinds); got != 38 {
		t.Errorf("len(Kinds(%s)) = %d, want 38", kindsPath, got)
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
				if !slices.Contains(contractLanguages(), lang) {
					t.Errorf("Kind(%s).languages contains %q, want only %v, the languages %s lists", k.Code, lang, contractLanguages(), languagesPath)
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

// fixedKinds are the three self-check kinds a configuration cannot move: a
// suppression, a configured root and a configured declaration that name
// something no longer present mean the same thing and carry the same posture.
var fixedKinds = []string{"DS9003", "DS9004", "DS9006"}

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
var narrowedKinds = []string{"DS1501", "DS1502", "DS1503", "DS1202", "DS1006", "DS1601", "DS1701", "DS1702"}

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

// heldDeclarationKinds are the rows whose subject is a declaration that can
// be a positional member or a declaration one holds, so a finding of the kind
// may carry manual in place of its own fixability.
var heldDeclarationKinds = []string{"DS1001", "DS1002", "DS1003", "DS1005", "DS1101", "DS1102", "DS1103", "DS1104", "DS1201"}

func TestKindsFindingFixabilitiesAreAlternatives(t *testing.T) {
	doc := loadKinds(t)
	var carrying []string
	for _, k := range doc.Kinds {
		if k.FindingFixabilities == nil {
			continue
		}
		carrying = append(carrying, k.Code)
		t.Run(k.Code, func(t *testing.T) {
			if len(k.FindingFixabilities) == 0 {
				t.Errorf("Kind(%s).finding_fixabilities = [], want the member absent or non-empty", k.Code)
			}
			seen := map[string]bool{}
			for _, f := range k.FindingFixabilities {
				switch {
				case !slices.Contains(doc.Fixabilities, f):
					t.Errorf("Kind(%s).finding_fixabilities contains %q, want one of %v", k.Code, f, doc.Fixabilities)
				case f == k.Fixability:
					t.Errorf("Kind(%s).finding_fixabilities contains %q, want only values other than the row's own fixability", k.Code, f)
				case f == "none":
					t.Errorf("Kind(%s).finding_fixabilities contains %q, want no value a generated file's finding carries", k.Code, f)
				case seen[f]:
					t.Errorf("Kind(%s).finding_fixabilities lists %q twice, want each value once", k.Code, f)
				}
				seen[f] = true
			}
		})
	}
	if !slices.Equal(carrying, heldDeclarationKinds) {
		t.Errorf("Kinds(%s) carrying finding_fixabilities = %v, want %v", kindsPath, carrying, heldDeclarationKinds)
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
			inGroup := 1400 <= n && n <= 1499
			if !inGroup {
				if k.Overlap != nil {
					t.Errorf("Kind(%s).overlap = %v, want absent outside the intra-function group", k.Code, k.Overlap)
				}
				return
			}
			group++
			if got, want := slices.Sorted(maps.Keys(k.Overlap)), slices.Sorted(slices.Values(contractLanguages())); !slices.Equal(got, want) {
				t.Fatalf("Kind(%s).overlap languages = %v, want %v", k.Code, got, want)
			}
			for _, lang := range contractLanguages() {
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

// TestKindsRetiredCodesStayRetired pins that every retired row names a code
// inside one range and the kind that once held it.
func TestKindsRetiredCodesStayRetired(t *testing.T) {
	doc := loadKinds(t)
	for _, r := range doc.Retired {
		t.Run(r.Code, func(t *testing.T) {
			if r.Name == "" {
				t.Errorf("Retired(%s).name = %q, want the name of the kind that once held the code", r.Code, r.Name)
			}
			if hits := rangesContaining(doc.Ranges, codeNumber(r.Code)); len(hits) != 1 {
				t.Errorf("Retired(%s) ranges = %d, want exactly 1", r.Code, len(hits))
			}
		})
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

// TestKindsColumnsDrawFromTheirVocabularies pins the per-kind columns the
// finding schema and the generated pages are derived from.
func TestKindsColumnsDrawFromTheirVocabularies(t *testing.T) {
	doc := loadKinds(t)
	members := propertyNames(loadFindingSchema(t), "properties/details")
	for _, k := range doc.Kinds {
		t.Run(k.Code, func(t *testing.T) {
			if !slices.Contains(doc.Subjects, k.Subject) {
				t.Errorf("Kind(%s).subject = %q, want one of %v", k.Code, k.Subject, doc.Subjects)
			}
			if k.HoldsLive == nil {
				t.Errorf("Kind(%s).holds_live = absent, want a boolean", k.Code)
			}
			for member, presence := range k.Details {
				if !slices.Contains(members, member) || member == findingDeletableOnlyField {
					t.Errorf("Kind(%s).details names %q, want a member of the finding's details other than %s", k.Code, member, findingDeletableOnlyField)
				}
				switch v := presence.(type) {
				case string:
					if v != "required" && v != "optional" {
						t.Errorf("Kind(%s).details[%s] = %q, want required, optional or a language list", k.Code, member, v)
					}
				case []any:
					for _, lang := range v {
						if name, ok := lang.(string); !ok || !slices.Contains(k.Languages, name) {
							t.Errorf("Kind(%s).details[%s] names %v, want languages of the kind", k.Code, member, lang)
						}
					}
				default:
					t.Errorf("Kind(%s).details[%s] = %v, want required, optional or a language list", k.Code, member, presence)
				}
			}
		})
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

// TestKindsPublicTextsHoldNoSemicolon pins that every text an analyzer copies
// into its output, a range's description and a live kind's rule and
// precondition, is written as plain sentences with no semicolon, because
// reports, SARIF rule descriptors and explanations show it to users verbatim.
func TestKindsPublicTextsHoldNoSemicolon(t *testing.T) {
	doc := loadKinds(t)
	for _, r := range doc.Ranges {
		if strings.Contains(r.Description, ";") {
			t.Errorf("Range(%s).description = %q, want no semicolon", r.Start, r.Description)
		}
	}
	for _, k := range doc.Kinds {
		for field, text := range map[string]string{"rule": k.Rule, "precondition": k.Precondition} {
			if strings.Contains(text, ";") {
				t.Errorf("Kind(%s).%s = %q, want no semicolon", k.Code, field, text)
			}
		}
	}
}
