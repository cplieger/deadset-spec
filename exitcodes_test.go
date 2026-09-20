package spec_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v2"
)

const exitCodesPath = "contract/exit-codes.json"

type exitCode struct {
	Name    string `json:"name"`
	Meaning string `json:"meaning"`
	Code    int    `json:"code"`
}

type exitCodeTable struct {
	Description string     `json:"description"`
	ExitCodes   []exitCode `json:"exit_codes"`
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
