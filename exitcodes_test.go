package spec_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

const (
	exitCodesPath         = "contract/exit-codes.json"
	setupFailuresPagePath = "contract/grammar/analysis/setup-failures.md"
	confidencePagePath    = "contract/grammar/analysis/confidence.md"
)

type exitCode struct {
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
	Code    int    `json:"code"`
}

type exitCodeTable struct {
	Description    string     `json:"description"`
	ExitCodes      []exitCode `json:"exit_codes"`
	FailureClasses struct {
		Setup failureGroup `json:"setup"`
		Run   failureGroup `json:"run"`
	} `json:"failure_classes"`
}

// failureGroup is one group of the classes code 3 names: the prefix its lines open with, where
// its causes lie, and its classes.
type failureGroup struct {
	Prefix  string         `json:"prefix"`
	Meaning string         `json:"meaning"`
	Classes []failureClass `json:"classes"`
}

// failureClass is one class of failure code 3 names: what fails and the fix the line on standard
// error states, and the languages (a setup class) or the products (a run class) that end with it.
type failureClass struct {
	Class     string   `json:"class"`
	Meaning   string   `json:"meaning"`
	Fix       string   `json:"fix"`
	Languages []string `json:"languages"`
	Products  []string `json:"products"`
}

// groups is each group of the failure classes under its name, the word its prefix opens with.
func (table *exitCodeTable) groups() map[string]*failureGroup {
	return map[string]*failureGroup{"setup": &table.FailureClasses.Setup, "run": &table.FailureClasses.Run}
}

// mustLoadExitCodes refuses unknown fields, so a key the table gains fails
// here instead of passing unread.
func mustLoadExitCodes(t *testing.T) exitCodeTable {
	t.Helper()
	data, err := fs.ReadFile(spec.Contract, exitCodesPath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", exitCodesPath, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var table exitCodeTable
	if err := dec.Decode(&table); err != nil {
		t.Fatalf("Setup: decoding %s: %v", exitCodesPath, err)
	}
	return table
}

func TestExitCodesAreExactlyZeroToFour(t *testing.T) {
	table := mustLoadExitCodes(t)
	codes := make([]int, 0, len(table.ExitCodes))
	for _, row := range table.ExitCodes {
		codes = append(codes, row.Code)
	}
	slices.Sort(codes)
	want := []int{0, 1, 2, 3, 4}
	if !slices.Equal(codes, want) {
		t.Errorf("%s codes = %v, want exactly %v, each once", exitCodesPath, codes, want)
	}
}

func TestExitCodesEachCarryADistinctMeaning(t *testing.T) {
	table := mustLoadExitCodes(t)
	nameRows := map[string][]int{}
	meaningRows := map[string][]int{}
	for i, row := range table.ExitCodes {
		nameRows[row.Name] = append(nameRows[row.Name], i)
		meaningRows[strings.TrimSpace(row.Meaning)] = append(meaningRows[strings.TrimSpace(row.Meaning)], i)
	}
	for i, row := range table.ExitCodes {
		t.Run(row.Name, func(t *testing.T) {
			if row.Code < 0 || row.Code > 4 {
				t.Errorf("%s[%d].code = %d, want 0 to 4", exitCodesPath, i, row.Code)
			}
			if strings.TrimSpace(row.Name) == "" {
				t.Errorf("%s[%d].name = %q, want a non-empty name", exitCodesPath, i, row.Name)
			}
			if owners := nameRows[row.Name]; len(owners) > 1 {
				t.Errorf("%s[%d].name = %q, want a name no other row carries, rows %v carry it", exitCodesPath, i, row.Name, owners)
			}
			meaning := strings.TrimSpace(row.Meaning)
			if meaning == "" {
				t.Errorf("%s[%d].meaning = %q, want a non-empty meaning", exitCodesPath, i, row.Meaning)
			}
			if owners := meaningRows[meaning]; len(owners) > 1 {
				t.Errorf("%s[%d].meaning = %q, want a meaning no other row carries, rows %v carry it", exitCodesPath, i, row.Meaning, owners)
			}
		})
	}
}

// failureClassSpelling is the spelling of a failure's class: lowercase words joined by single
// hyphens, the token the line on standard error carries after its group's prefix.
var failureClassSpelling = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

func TestExitCodesFailureClassesAreDistinctClassesWithAFix(t *testing.T) {
	table := mustLoadExitCodes(t)
	products := map[string]bool{}
	for _, product := range cliProducts(t) {
		products[product] = true
	}
	seen := map[string]string{}
	for name, group := range table.groups() {
		if len(group.Classes) == 0 || strings.TrimSpace(group.Meaning) == "" {
			t.Errorf("%s failure_classes.%s holds %d classes and meaning %q, want classes and a meaning", exitCodesPath, name, len(group.Classes), group.Meaning)
		}
		for i, row := range group.Classes {
			where := fmt.Sprintf("%s failure_classes.%s.classes[%d] (%s)", exitCodesPath, name, i, row.Class)
			if !failureClassSpelling.MatchString(row.Class) {
				t.Errorf("%s class = %q, want lowercase words joined by hyphens", where, row.Class)
			}
			if other, ok := seen[row.Class]; ok {
				t.Errorf("%s class is also a class of the %s group, want a class no other row of either group carries", where, other)
			}
			seen[row.Class] = name
			if strings.TrimSpace(row.Meaning) == "" || strings.TrimSpace(row.Fix) == "" {
				t.Errorf("%s meaning = %q, fix = %q, want both non-empty", where, row.Meaning, row.Fix)
			}
			switch name {
			case "setup":
				if len(row.Languages) == 0 || len(row.Products) != 0 {
					t.Errorf("%s languages = %v, products = %v, want the languages whose analysis ends with it and no products", where, row.Languages, row.Products)
				}
			case "run":
				if len(row.Products) == 0 || len(row.Languages) != 0 {
					t.Errorf("%s products = %v, languages = %v, want the products whose run ends with it and no languages", where, row.Products, row.Languages)
				}
				for _, product := range row.Products {
					if !products[product] {
						t.Errorf("%s names the product %q, which contract/cli.json does not", where, product)
					}
				}
			}
		}
	}
}

// memoryExhaustedDetail is the text the memory-exhausted class's line opens its detail with, with
// N and M as the placeholders the class's fix defines.
const memoryExhaustedDetail = `"at least N GB were needed, M GB are available"`

func TestExitCodesStateTheMemoryExhaustionLine(t *testing.T) {
	table := mustLoadExitCodes(t)
	i := slices.IndexFunc(table.FailureClasses.Run.Classes, func(row failureClass) bool { return row.Class == "memory-exhausted" })
	if i < 0 {
		t.Fatalf("%s failure_classes.run holds no memory-exhausted class", exitCodesPath)
	}
	if fix := table.FailureClasses.Run.Classes[i].Fix; !strings.Contains(fix, memoryExhaustedDetail) {
		t.Errorf("%s memory-exhausted fix = %q, want it to quote %s", exitCodesPath, fix, memoryExhaustedDetail)
	}
	for _, row := range table.ExitCodes {
		if row.Code == 3 && !strings.Contains(row.Meaning, "more memory than the machine makes available") {
			t.Errorf("%s code 3 meaning = %q, want it to name memory exhaustion", exitCodesPath, row.Meaning)
		}
	}
}

// TestExitCodesSetupFailureClassesAreTheOnesTheAnalysisPageStates holds the two documents to one
// set: every class the table carries is stated on grammar/analysis/setup-failures.md, and the page states none the
// table lacks.
func TestExitCodesSetupFailureClassesAreTheOnesTheAnalysisPageStates(t *testing.T) {
	table := mustLoadExitCodes(t)
	page, err := fs.ReadFile(spec.Contract, setupFailuresPagePath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", setupFailuresPagePath, err)
	}
	stated := map[string]bool{}
	for _, m := range regexp.MustCompile("`setup failure: ([a-z0-9-]+)`").FindAllSubmatch(page, -1) {
		stated[string(m[1])] = true
	}
	for _, row := range table.FailureClasses.Setup.Classes {
		if !stated[row.Class] {
			t.Errorf("%s states no `setup failure: %s`, want every class of %s", setupFailuresPagePath, row.Class, exitCodesPath)
		}
		delete(stated, row.Class)
	}
	for class := range stated {
		t.Errorf("%s states `setup failure: %s`, a class %s does not carry", setupFailuresPagePath, class, exitCodesPath)
	}
}

// TestExitCodesPrefixNamesTheGroup pins each group's prefix, the text the products read from the
// data, to the group it names, and the page rule that states each group's line to that prefix.
func TestExitCodesPrefixNamesTheGroup(t *testing.T) {
	table := mustLoadExitCodes(t)
	page, err := fs.ReadFile(spec.Contract, setupFailuresPagePath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", setupFailuresPagePath, err)
	}
	for name, group := range table.groups() {
		if want := name + " failure: "; group.Prefix != want {
			t.Errorf("%s failure_classes.%s.prefix = %q, want %q, the prefix naming its group", exitCodesPath, name, group.Prefix, want)
		}
		if !bytes.Contains(page, []byte("starts with `"+strings.TrimSuffix(group.Prefix, " ")+"`, a space, the class")) {
			t.Errorf("%s states no line that starts with %q, want the line of the %s group", setupFailuresPagePath, group.Prefix, name)
		}
	}
}

// Products use this vocabulary instead of copying the failure-class table.
func TestExitCodesFailureClassesAreTheGeneratedVocabulary(t *testing.T) {
	t.Parallel()

	table := mustLoadExitCodes(t)
	var setup, run []string
	for _, c := range vocab.SetupClasses() {
		languages := make([]string, len(c.Languages))
		for i, language := range c.Languages {
			languages[i] = string(language)
		}
		setup = append(setup, c.Name+" "+strings.Join(languages, ","))
	}
	for _, c := range vocab.RunClasses() {
		run = append(run, c.Name+" "+strings.Join(c.Products, ","))
	}
	var wantSetup, wantRun []string
	for _, c := range table.FailureClasses.Setup.Classes {
		wantSetup = append(wantSetup, c.Class+" "+strings.Join(c.Languages, ","))
	}
	for _, c := range table.FailureClasses.Run.Classes {
		wantRun = append(wantRun, c.Class+" "+strings.Join(c.Products, ","))
	}
	if !slices.Equal(setup, wantSetup) || !slices.Equal(run, wantRun) {
		t.Errorf("vocab.SetupClasses() = %q and vocab.RunClasses() = %q, want %q and %q", setup, run, wantSetup, wantRun)
	}
}
