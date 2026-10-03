package spec_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v5"
)

const (
	exitCodesPath    = "contract/exit-codes.json"
	analysisPagePath = "contract/grammar/analysis.md"
)

type exitCode struct {
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
	Code    int    `json:"code"`
}

type exitCodeTable struct {
	Description      string         `json:"description"`
	MemoryExhaustion string         `json:"memory_exhaustion"`
	ExitCodes        []exitCode     `json:"exit_codes"`
	SetupFailures    []setupFailure `json:"setup_failures"`
}

// setupFailure is one class of the setup failures code 3 names: what is missing and the fix the
// line on standard error states.
type setupFailure struct {
	Class   string `json:"class"`
	Meaning string `json:"meaning"`
	Fix     string `json:"fix"`
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

// setupFailureClass is the spelling of a setup failure's class: lowercase words joined by single
// hyphens, the token the line on standard error carries after "setup failure: ".
var setupFailureClass = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

func TestExitCodesSetupFailuresAreDistinctClassesWithAFix(t *testing.T) {
	table := mustLoadExitCodes(t)
	if len(table.SetupFailures) == 0 {
		t.Fatalf("%s setup_failures is empty, want the classes code 3 names", exitCodesPath)
	}
	seen := map[string]bool{}
	for i, row := range table.SetupFailures {
		if !setupFailureClass.MatchString(row.Class) {
			t.Errorf("%s setup_failures[%d].class = %q, want lowercase words joined by hyphens", exitCodesPath, i, row.Class)
		}
		if seen[row.Class] {
			t.Errorf("%s setup_failures[%d].class = %q, want a class no other row carries", exitCodesPath, i, row.Class)
		}
		seen[row.Class] = true
		if strings.TrimSpace(row.Meaning) == "" || strings.TrimSpace(row.Fix) == "" {
			t.Errorf("%s setup_failures[%d] (%s) meaning = %q, fix = %q, want both non-empty", exitCodesPath, i, row.Class, row.Meaning, row.Fix)
		}
	}
}

// memoryExhaustedLine is the line the memory_exhaustion member quotes, with N and M as the
// placeholders the member defines.
const memoryExhaustedLine = `"memory exhausted: at least N GB were needed, M GB are available"`

func TestExitCodesStateTheMemoryExhaustionLine(t *testing.T) {
	table := mustLoadExitCodes(t)
	if !strings.Contains(table.MemoryExhaustion, memoryExhaustedLine) {
		t.Errorf("%s memory_exhaustion = %q, want it to quote %s", exitCodesPath, table.MemoryExhaustion, memoryExhaustedLine)
	}
	for _, row := range table.ExitCodes {
		if row.Code == 3 && !strings.Contains(row.Meaning, "more memory than the machine makes available") {
			t.Errorf("%s code 3 meaning = %q, want it to name memory exhaustion", exitCodesPath, row.Meaning)
		}
	}
}

// TestExitCodesSetupFailureClassesAreTheOnesTheAnalysisPageStates holds the two documents to one
// set: every class the table carries is stated on grammar/analysis.md, and the page states none the
// table lacks.
func TestExitCodesSetupFailureClassesAreTheOnesTheAnalysisPageStates(t *testing.T) {
	table := mustLoadExitCodes(t)
	page, err := fs.ReadFile(spec.Contract, analysisPagePath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", analysisPagePath, err)
	}
	stated := map[string]bool{}
	for _, m := range regexp.MustCompile("`setup failure: ([a-z0-9-]+)`").FindAllSubmatch(page, -1) {
		stated[string(m[1])] = true
	}
	for _, row := range table.SetupFailures {
		if !stated[row.Class] {
			t.Errorf("%s states no `setup failure: %s`, want every class of %s", analysisPagePath, row.Class, exitCodesPath)
		}
		delete(stated, row.Class)
	}
	for class := range stated {
		t.Errorf("%s states `setup failure: %s`, a class %s does not carry", analysisPagePath, class, exitCodesPath)
	}
}
