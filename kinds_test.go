package spec_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec"
)

const kindsPath = "contract/kinds.json"

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
	Retired           *bool  `json:"retired"`
	RetiredByDecision *int   `json:"retired_by_decision"`
	Start             string `json:"start"`
	End               string `json:"end"`
	Family            string `json:"family"`
	Description       string `json:"description"`
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

type retiredKind struct {
	RetiredByDecision *int   `json:"retired_by_decision"`
	Code              string `json:"code"`
	Name              string `json:"name"`
	Reason            string `json:"reason"`
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

func TestKindsRangesAreEightLiveAndOneRetired(t *testing.T) {
	doc := loadKinds(t)
	var live, retired int
	for _, r := range doc.Ranges {
		t.Run(r.Start, func(t *testing.T) {
			if r.Family == "" || r.Description == "" || !isSet(r.Retired) {
				t.Errorf("Range(%s) fields = family %q, description %q, retired %s, want every field present", r.Start, r.Family, r.Description, optional(r.Retired))
			}
			start, end := codeNumber(r.Start), codeNumber(r.End)
			if start < 0 || end < start {
				t.Errorf("Range(%s) span = %q..%q, want two DSnnnn codes with start <= end", r.Start, r.Start, r.End)
			}
			if isSet(r.Retired) && *r.Retired {
				retired++
				if r.RetiredByDecision == nil {
					t.Errorf("Range(%s) retired_by_decision = absent, want the number of the decision that retired it", r.Start)
				}
			} else {
				live++
			}
		})
	}
	if live != 8 || retired != 1 {
		t.Errorf("Ranges(%s) = %d live, %d retired, want 8 live, 1 retired", kindsPath, live, retired)
	}
}

func TestKindsEveryLiveRowIsUniqueRangedAndComplete(t *testing.T) {
	doc := loadKinds(t)
	if got := len(doc.Kinds); got != 31 {
		t.Errorf("len(Kinds(%s)) = %d, want 31", kindsPath, got)
	}
	seen := make(map[string]bool, len(doc.Kinds))
	for _, k := range doc.Kinds {
		t.Run(k.Code, func(t *testing.T) {
			n := codeNumber(k.Code)
			if n < 0 {
				t.Fatalf("Kind(%q).code = %q, want %s followed by four digits", k.Code, k.Code, doc.Prefix)
			}
			if seen[k.Code] {
				t.Errorf("Kind(%s) occurrences = 2, want 1", k.Code)
			}
			seen[k.Code] = true

			hits := rangesContaining(doc.Ranges, n)
			switch {
			case len(hits) != 1:
				t.Errorf("Kind(%s) ranges = %d, want exactly 1", k.Code, len(hits))
			case isSet(hits[0].Retired) && *hits[0].Retired:
				t.Errorf("Kind(%s) range = %s (retired), want a live range", k.Code, hits[0].Start)
			}

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

func TestKindsOnlyStaleSuppressionIsFixed(t *testing.T) {
	doc := loadKinds(t)
	for _, k := range doc.Kinds {
		t.Run(k.Code, func(t *testing.T) {
			want := k.Code == "DS1703"
			if !isSet(k.Fixed) || *k.Fixed != want {
				t.Errorf("Kind(%s).fixed = %s, want %v", k.Code, optional(k.Fixed), want)
			}
			if want && (!isSet(k.DefaultEnabled) || !*k.DefaultEnabled || k.DefaultSeverity != "deny") {
				t.Errorf("Kind(%s) enablement = %s at %q, want fixed on at \"deny\"", k.Code, optional(k.DefaultEnabled), k.DefaultSeverity)
			}
		})
	}
}

// narrowedKinds are the rows settled decisions 31 and 32 narrowed; each must
// state the precondition the analyzer checks before it reports.
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

// retiredCodes are the nineteen codes settled decisions 27 and 32 removed,
// each with the decision that retired it. A code may leave this table only by
// a decision that re-admits it, never by re-use for another kind.
var retiredCodes = map[string]int{
	"DS1202": 27, "DS1401": 27, "DS1402": 27,
	"DS1503": 27, "DS1504": 27, "DS1505": 27, "DS1506": 27, "DS1507": 27,
	"DS1602": 27, "DS1603": 27, "DS1604": 27, "DS1606": 27,
	"DS1607": 32, "DS1608": 32,
	"DS1804": 27, "DS1806": 27, "DS1808": 27, "DS1810": 27, "DS1811": 27,
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
	for _, code := range slices.Sorted(maps.Keys(retiredCodes)) {
		t.Run(code, func(t *testing.T) {
			r, ok := listed[code]
			if !ok {
				t.Fatalf("Retired(%s) = absent, want a retired row", code)
			}
			if r.Name == "" || r.Reason == "" || r.RetiredByDecision == nil {
				t.Errorf("Retired(%s) fields = name %q, reason %q, decision %s, want every field present", code, r.Name, r.Reason, optional(r.RetiredByDecision))
			}
			if r.RetiredByDecision != nil && *r.RetiredByDecision != retiredCodes[code] {
				t.Errorf("Retired(%s).retired_by_decision = %d, want %d", code, *r.RetiredByDecision, retiredCodes[code])
			}
			if slices.ContainsFunc(doc.Kinds, func(k kindRow) bool { return k.Code == code }) {
				t.Errorf("Kinds(%s) contains %s, want the retired code absent from every live row", kindsPath, code)
			}
			if hits := rangesContaining(doc.Ranges, codeNumber(code)); len(hits) != 1 {
				t.Errorf("Retired(%s) ranges = %d, want exactly 1", code, len(hits))
			}
		})
	}
	for _, r := range doc.Retired {
		if _, ok := retiredCodes[r.Code]; !ok {
			t.Errorf("Retired(%s) lists %s, want only the nineteen retired codes", kindsPath, r.Code)
		}
	}
}

func TestKindsRetiredRangeHoldsNoLiveRow(t *testing.T) {
	doc := loadKinds(t)
	for _, r := range doc.Ranges {
		if !isSet(r.Retired) || !*r.Retired {
			continue
		}
		t.Run(r.Start, func(t *testing.T) {
			if r.Start != "DS1400" {
				t.Errorf("RetiredRange(%s) = %s..%s, want DS1400..DS1499 as the only retired range", r.Start, r.Start, r.End)
			}
			for _, k := range doc.Kinds {
				n := codeNumber(k.Code)
				if codeNumber(r.Start) <= n && n <= codeNumber(r.End) {
					t.Errorf("Kinds(%s) contains %s inside retired range %s, want none", kindsPath, k.Code, r.Start)
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
