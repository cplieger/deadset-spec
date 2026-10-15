package report

import (
	"testing"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// A report's deletable lines are the sum over the distinct root components its
// findings name, each counted once by its own declared count: a component two
// root findings name counts once, and a component no root finding names counts
// not at all.
func TestBuildCountsEachRootComponentOnce(t *testing.T) {
	t.Parallel()

	finding := func(line int, id string, root bool, lines int) Finding {
		return Finding{
			Code:      vocab.UnusedExported,
			Position:  Position{Path: "a.go", Line: line, Column: 1, EndLine: line},
			Symbol:    Symbol{Ref: "go://example.com/a#F" + id},
			Severity:  vocab.Deny,
			Component: Component{ID: id, Root: root, SymbolCount: 1, DeletableLines: lines},
		}
	}
	built, err := Build(&Input{Findings: []Finding{
		finding(1, "c-1", true, 10),
		finding(5, "c-1", true, 10),
		finding(9, "c-2", true, 4),
		finding(12, "c-3", false, 100),
	}})
	if err != nil {
		t.Fatalf("Build() = %v, want a report", err)
	}
	if got, want := built.Totals.DeletableLines, 14; got != want {
		t.Errorf("Build().Totals.DeletableLines = %d, want %d", got, want)
	}
}
