package suppress

import (
	"errors"
	"slices"
	"testing"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// noDials withholds nothing and allows no code.
var noDials = Dials{
	Allows:    func(vocab.Code) bool { return false },
	Withholds: func(*Finding) bool { return false },
}

func declaration(code vocab.Code, ref string, line int) Finding {
	return Finding{Code: code, SymbolKind: "function", Ref: ref, Path: "a.go", Line: line, Column: 6}
}

func record(code vocab.Code, ref, bound string, mechanism Mechanism, line int) Record {
	return Record{Code: code, Symbol: ref, Path: "a.go", Reason: "kept", Bound: bound, Mechanism: mechanism, Site: Site{Path: "a.go", Line: line, Column: 1}}
}

// The first record in reading order claims a finding; a second naming it, a
// record under another code and one bound to nothing are stale.
func TestDecideGivesAFindingToItsFirstRecord(t *testing.T) {
	t.Parallel()

	found := declaration(vocab.UnusedUnexported, "go://x#f", 3)
	records := []Record{
		record(vocab.UnusedUnexported, "go://x#f", found.Key(), Inline, 2),
		record(vocab.UnusedUnexported, "go://x#f", found.Key(), Ignore, 1),
		record(vocab.UnusedExported, "go://x#f", found.Key(), Ignore, 2),
		record(vocab.UnusedUnexported, "go://x#gone", "", Baseline, 1),
	}
	ledger := Decide(records, []Finding{found}, &noDials)
	if want := []Verdict{InEffect, Stale, Stale, Stale}; !slices.Equal(ledger.Verdicts, want) {
		t.Errorf("Decide = %v, want %v", ledger.Verdicts, want)
	}
	if !ledger.Withheld(&found) {
		t.Errorf("Withheld(the claimed finding) = false, want true")
	}
	if inEffect, reasons := Counts(records, ledger); inEffect != 1 || reasons != 4 {
		t.Errorf("Counts = %d in effect and %d reasons, want 1 and 4", inEffect, reasons)
	}
}

// A part-kind record claims every finding of its code about a part of its
// declaration, and marks nothing live.
func TestDecideGivesAPartKindRecordEveryPartOfItsDeclaration(t *testing.T) {
	t.Parallel()

	part := func(name string, column int) Finding {
		return Finding{Code: vocab.UnusedParameter, SymbolKind: "parameter", Ref: "go://x#f(" + name + ")", Holder: "go://x#f", Path: "a.go", Line: 3, Column: column}
	}
	would := []Finding{part("a", 8), part("b", 11)}
	records := []Record{record(vocab.UnusedParameter, "go://x#f", "", Ignore, 1), record(vocab.UnusedParameter, "go://x#f", "", Baseline, 1)}
	ledger := Decide(records, would, &noDials)
	if want := []Verdict{InEffect, Stale}; !slices.Equal(ledger.Verdicts, want) {
		t.Errorf("Decide = %v, want %v", ledger.Verdicts, want)
	}
	if !ledger.Withheld(&would[0]) || !ledger.Withheld(&would[1]) {
		t.Errorf("the part-kind record withholds %t and %t, want both parts", ledger.Withheld(&would[0]), ledger.Withheld(&would[1]))
	}
	bound := records[0]
	bound.Bound = "a.go:3:6"
	if bound.MarksLive() {
		t.Errorf("MarksLive(a part-kind record) = true, want false")
	}
}

// A record whose every finding the dials withhold, and a bound record of an
// allowed code that claims none, are dormant.
func TestDecideHoldsADialledRecordDormant(t *testing.T) {
	t.Parallel()

	found := declaration(vocab.UnusedUnexported, "go://x#f", 3)
	dials := Dials{Allows: func(vocab.Code) bool { return true }, Withholds: func(*Finding) bool { return true }}
	records := []Record{
		record(vocab.UnusedUnexported, "go://x#f", found.Key(), Inline, 2),
		record(vocab.UnusedExported, "go://x#g", "a.go:9:6", Inline, 8),
		record(vocab.UnusedExported, "go://x#h", "", Ignore, 1),
	}
	if got, want := Decide(records, []Finding{found}, &dials).Verdicts, []Verdict{Dormant, Dormant, Stale}; !slices.Equal(got, want) {
		t.Errorf("Decide = %v, want %v", got, want)
	}
}

// A dependency row is reached by an entry naming its reference and file, never
// by a directive, and no record reaches another row of a document.
func TestDecideReachesADependencyRowByItsReferenceAlone(t *testing.T) {
	t.Parallel()

	dependency := Finding{Code: vocab.UnusedDependency, SymbolKind: "dependency", Ref: "go://x#y:require", Path: "go.mod", Line: 3, Column: 2}
	file := Finding{Code: vocab.FileNeverBuilt, SymbolKind: "file", Ref: "go://x#a.go:file", Path: "a.go", Line: 1, Column: 1}
	records := []Record{
		{Code: dependency.Code, Symbol: dependency.Ref, Path: "go.mod", Bound: dependency.Key(), Mechanism: Inline},
		{Code: dependency.Code, Symbol: dependency.Ref, Path: "go.mod", Mechanism: Ignore},
		{Code: file.Code, Symbol: file.Ref, Path: "a.go", Bound: file.Key(), Mechanism: Ignore},
	}
	if got, want := Decide(records, []Finding{dependency, file}, &noDials).Verdicts, []Verdict{Stale, InEffect, Stale}; !slices.Equal(got, want) {
		t.Errorf("Decide = %v, want %v", got, want)
	}
}

// An entry naming two declarations is stale only where both of its records are.
func TestStaleRecordsReportsAnEntryOnceWhenEveryRecordOfItIsStale(t *testing.T) {
	t.Parallel()

	one := record(vocab.UnusedUnexported, "go://x#_", "a.go:3:6", Ignore, 1)
	two := one
	two.Bound = "a.go:4:6"
	other := record(vocab.UnusedUnexported, "go://x#y", "", Ignore, 2)
	ledger := &Ledger{Verdicts: []Verdict{InEffect, Stale, Stale}}
	if got := StaleRecords([]Record{one, two, other}, ledger); len(got) != 1 || got[0].Symbol != "go://x#y" {
		t.Errorf("StaleRecords = %+v, want the one entry whose every record is stale", got)
	}
	ledger.Verdicts = []Verdict{Stale, Stale, InEffect}
	if got := StaleRecords([]Record{one, two, other}, ledger); len(got) != 1 || got[0].Symbol != "go://x#_" {
		t.Errorf("StaleRecords = %+v, want the two-declaration entry once", got)
	}
}

// Bind resolves the entries of its own scheme only, refuses one with no reason
// or no path with a finding, binds a part-kind entry naming a part to the
// declaration holding it, and refuses a reference outside the grammar.
func TestBind(t *testing.T) {
	t.Parallel()

	text := func(s string) *string { return &s }
	binder := &Binder{
		Scheme: "go://",
		Lookup: func(symbol, path string) []string {
			if symbol == "go://x#f" && path == "a.go" {
				return []string{"a.go:3:6"}
			}
			return nil
		},
		Holder: func(symbol string) (string, bool) { return "go://x#f", symbol == "go://x#f(a)" },
		Valid:  func(symbol string) bool { return symbol != "go://x#!" },
	}
	entries := []Entry{
		{Code: vocab.UnusedUnexported, Symbol: "go://x#f", Path: text("a.go"), Reason: text("kept")},
		{Code: vocab.UnusedParameter, Symbol: "go://x#f(a)", Path: text("a.go"), Reason: text("kept")},
		{Code: vocab.UnusedUnexported, Symbol: "ts://@x/a.ts#f", Path: text("a.ts"), Reason: text("kept")},
		{Code: vocab.UnusedUnexported, Symbol: "go://x#g", Reason: text(" ")},
	}
	records, refusals, err := binder.Bind(entries, Ignore)
	if err != nil {
		t.Fatalf("Bind = %v", err)
	}
	if len(records) != 2 || records[0].Bound != "a.go:3:6" || records[1].Bound != "a.go:3:6" {
		t.Errorf("Bind records = %+v, want the declaration and the part each bound to a.go:3:6", records)
	}
	if len(refusals) != 2 || refusals[0].Reported != vocab.SuppressionWithoutReason || refusals[1].Reported != vocab.UnscopedIgnoreEntry {
		t.Errorf("Bind refusals = %+v, want one without a reason and one without a path", refusals)
	}
	if _, _, err := binder.Bind([]Entry{{Code: vocab.UnusedUnexported, Symbol: "go://x#!"}}, Ignore); !errors.Is(err, ErrMalformed) {
		t.Errorf("Bind(a reference outside the grammar) = %v, want ErrMalformed", err)
	}
}
