package suppress

import (
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// noDials allows no code.
var noDials = Dials{Allows: func(vocab.Code) bool { return false }}

func declaration(code vocab.Code, ref string, line int) Finding {
	decl := "a.go:" + strconv.Itoa(line) + ":6"
	return Finding{Code: code, SymbolKind: "function", Ref: ref, Path: "a.go", Decl: decl, ID: decl}
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
		record(vocab.UnusedUnexported, "go://x#f", found.Decl, Inline, 2),
		record(vocab.UnusedUnexported, "go://x#f", found.Decl, Ignore, 1),
		record(vocab.UnusedExported, "go://x#f", found.Decl, Ignore, 2),
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
		return Finding{Code: vocab.UnusedParameter, SymbolKind: "parameter", Ref: "go://x#f(" + name + ")", Holder: "go://x#f", Path: "a.go", Decl: "a.go:3:6", ID: "a.go:3:" + strconv.Itoa(column)}
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

// A record whose every finding something withholds, and a bound record of an
// allowed code that claims none, are dormant.
func TestDecideHoldsADialledRecordDormant(t *testing.T) {
	t.Parallel()

	found := declaration(vocab.UnusedUnexported, "go://x#f", 3)
	found.Withheld = WithheldByMinConfidence
	dials := Dials{Allows: func(vocab.Code) bool { return true }}
	records := []Record{
		record(vocab.UnusedUnexported, "go://x#f", found.Decl, Inline, 2),
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

	dependency := Finding{Code: vocab.UnusedDependency, SymbolKind: "dependency", Ref: "go://x#y:require", Path: "go.mod", ID: "go.mod:3:2"}
	file := Finding{Code: vocab.FileNeverBuilt, SymbolKind: "file", Ref: "go://x#a.go:file", Path: "a.go", ID: "a.go:1:1"}
	records := []Record{
		{Code: dependency.Code, Symbol: dependency.Ref, Path: "go.mod", Bound: "go.mod:3:2", Mechanism: Inline},
		{Code: dependency.Code, Symbol: dependency.Ref, Path: "go.mod", Mechanism: Ignore},
		{Code: file.Code, Symbol: file.Ref, Path: "a.go", Bound: "a.go:1:1", Mechanism: Ignore},
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

// Two directives a line directive renders at one site are two suppressions,
// told apart by where each is written.
func TestStaleRecordsTellsTwoDirectivesRenderedAtOneSiteApart(t *testing.T) {
	t.Parallel()

	kept := record(vocab.UnusedUnexported, "go://x#alpha", "a.go:3:6", Inline, 1)
	kept.Site.Written = "a.go@40"
	used := record(vocab.UnusedUnexported, "go://x#bravo", "b.go:3:6", Inline, 1)
	used.Site.Written = "b.go@40"
	records := []Record{kept, used}
	ledger := &Ledger{Verdicts: []Verdict{InEffect, Stale}}
	if got := StaleRecords(records, ledger); len(got) != 1 || got[0].Symbol != "go://x#bravo" {
		t.Errorf("StaleRecords = %+v, want the directive bound to bravo", got)
	}
	if inEffect, reasons := Counts(records, ledger); inEffect != 1 || reasons != 2 {
		t.Errorf("Counts = %d in effect and %d reasons, want 1 and 2", inEffect, reasons)
	}
	if got := (&MalformedError{Text: "x", Want: "y", Mechanism: Inline, Site: kept.Site}).Error(); got != "suppress: inline a.go:1:1: x: want y" {
		t.Errorf("Error() = %q, want the rendered site alone", got)
	}
}

// Bind resolves the entries of its own scheme only, leaving another scheme's
// unread however it is spelled, refuses one with no reason or no path with a
// finding, binds a part-kind entry naming a part to the declaration holding
// it, and refuses a reference of its scheme outside the grammar.
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
	}
	entries := []Entry{
		{Code: vocab.UnusedUnexported, Symbol: "go://x#f", Path: text("a.go"), Reason: text("kept")},
		{Code: vocab.UnusedParameter, Symbol: "go://x#f(a)", Path: text("a.go"), Reason: text("kept")},
		{Code: vocab.UnusedUnexported, Symbol: "ts://@x/a.ts#f", Path: text("a.ts"), Reason: text("kept")},
		{Code: vocab.UnusedUnexported, Symbol: "ts://no-file#!", Path: text("a.ts"), Reason: text("kept")},
		{Code: vocab.UnusedUnexported, Symbol: "proto://pkg#Message", Path: text("a.proto"), Reason: text("kept")},
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

// Only a bound record whose code reports a declaration marks it live: a part,
// a dependency row, a binding, a file and a record of a document are no
// declaration the record could keep.
func TestMarksLiveOnlyForACodeAboutADeclaration(t *testing.T) {
	t.Parallel()

	for _, kind := range vocab.Kinds() {
		code := kind.Code
		bound := record(code, "go://x#f", "a.go:3:6", Inline, 2)
		want := code.Subject() == vocab.SubjectDeclaration
		if got := bound.MarksLive(); got != want {
			t.Errorf("MarksLive(%s, subject %s) = %t, want %t", code, code.Subject(), got, want)
		}
		if unbound := record(code, "go://x#f", "", Inline, 2); unbound.MarksLive() {
			t.Errorf("MarksLive(%s bound to nothing) = true, want false", code)
		}
	}
}

// Two findings of one code that render at one position are two findings, each
// claimed by the record bound to its own declaration.
func TestDecideTellsFindingsAtOnePositionApartByIdentity(t *testing.T) {
	t.Parallel()

	alpha := Finding{Code: vocab.UnusedUnexported, SymbolKind: "function", Ref: "go://x#alpha", Path: "main.go", Decl: "a.go@40", ID: "a.go@40"}
	bravo := Finding{Code: vocab.UnusedUnexported, SymbolKind: "function", Ref: "go://x#bravo", Path: "main.go", Decl: "b.go@40", ID: "b.go@40"}
	records := []Record{
		record(vocab.UnusedUnexported, "go://x#alpha", "a.go@40", Inline, 19),
		record(vocab.UnusedUnexported, "go://x#bravo", "b.go@40", Inline, 19),
	}
	ledger := Decide(records, []Finding{alpha, bravo}, &noDials)
	if want := []Verdict{InEffect, InEffect}; !slices.Equal(ledger.Verdicts, want) {
		t.Errorf("Decide = %v, want %v", ledger.Verdicts, want)
	}
	if !ledger.Withheld(&alpha) || !ledger.Withheld(&bravo) {
		t.Errorf("Withheld = %t and %t, want both", ledger.Withheld(&alpha), ledger.Withheld(&bravo))
	}
}

// A record bound to one declaration claims no part of another declaration of
// the same reference and file, as one init is of another, and an entry naming
// a parameter's own reference claims that parameter alone. An entry resolving
// to two declarations gives each record its own declaration's findings.
func TestDecideKeepsABoundRecordToItsOwnDeclaration(t *testing.T) {
	t.Parallel()

	store := func(decl string) Finding {
		return Finding{Code: vocab.DeadStore, SymbolKind: "store", Ref: "go://x#init", Path: "a.go", Decl: decl, ID: decl + "/v"}
	}
	first, second := store("a.go:3:6"), store("a.go:9:6")
	records := []Record{record(vocab.DeadStore, "go://x#init", "a.go:9:6", Inline, 8)}
	ledger := Decide(records, []Finding{first, second}, &noDials)
	if ledger.Withheld(&first) || !ledger.Withheld(&second) {
		t.Errorf("Withheld = %t and %t, want the second init's store alone", ledger.Withheld(&first), ledger.Withheld(&second))
	}

	parameter := func(name string) Finding {
		return Finding{Code: vocab.UnusedParameter, SymbolKind: "parameter", Ref: "go://x#f(" + name + ")", Holder: "go://x#f", Path: "a.go", Decl: "a.go:3:6", ID: "a.go:3:6(" + name + ")"}
	}
	a, b := parameter("a"), parameter("b")
	named := []Record{record(vocab.UnusedParameter, "go://x#f(a)", "a.go:3:6", Ignore, 1)}
	ledger = Decide(named, []Finding{a, b}, &noDials)
	if !ledger.Withheld(&a) || ledger.Withheld(&b) {
		t.Errorf("Withheld = %t and %t, want the named parameter alone", ledger.Withheld(&a), ledger.Withheld(&b))
	}

	// Two platform alternatives of one function share its reference and path.
	alternative := func(decl string) Finding {
		return Finding{Code: vocab.UnreachableExport, SymbolKind: "function", Ref: "go://x#Ping", Path: "ping.proto", Decl: decl, ID: decl}
	}
	linux, windows := alternative("a_linux.go:6:6"), alternative("z_windows.go:7:6")
	bound := []Record{record(vocab.UnreachableExport, "go://x#Ping", "z_windows.go:7:6", Inline, 10)}
	bound[0].Path = "ping.proto"
	ledger = Decide(bound, []Finding{linux, windows}, &noDials)
	if ledger.Withheld(&linux) || !ledger.Withheld(&windows) {
		t.Errorf("Withheld = %t and %t, want the windows alternative alone", ledger.Withheld(&linux), ledger.Withheld(&windows))
	}

	assertion := Finding{Code: vocab.UnusedSatisfactionAssertion, SymbolKind: "satisfaction assertion", Ref: "go://x#writer", Path: "a.go", Decl: "a.go:20:5", ID: "a.go:20:5", Named: []string{"a.go:8:6"}}
	other := assertion
	other.Decl, other.ID, other.Named = "a.go:21:5", "a.go:21:5", []string{"a.go:9:6"}
	// Both interfaces render at the entry's path, so it binds both, and each
	// record claims the assertion naming its own interface.
	onInterface := []Record{record(vocab.UnusedSatisfactionAssertion, "go://x#writer", "a.go:8:6", Ignore, 1)}
	onInterface = append(onInterface, onInterface[0])
	onInterface[1].Bound = "a.go:9:6"
	ledger = Decide(onInterface, []Finding{other, assertion}, &noDials)
	if !ledger.Withheld(&other) || !ledger.Withheld(&assertion) || !slices.Equal(ledger.Verdicts, []Verdict{InEffect, InEffect}) {
		t.Errorf("Withheld = %t and %t, verdicts %v, want both, each record in effect", ledger.Withheld(&other), ledger.Withheld(&assertion), ledger.Verdicts)
	}
	ledger = Decide(onInterface, []Finding{assertion}, &noDials)
	if !slices.Equal(ledger.Verdicts, []Verdict{InEffect, Stale}) {
		t.Errorf("Decide = %v, want the record bound to the asserted interface alone in effect", ledger.Verdicts)
	}

	// One interface asserted in two files: a record bound to the interface
	// names the assertion at the path it names.
	elsewhere := assertion
	elsewhere.Path, elsewhere.Decl, elsewhere.ID, elsewhere.Named = "a.go", "a.go:7:5", "a.go:7:5", []string{"z.go:5:6"}
	beside := assertion
	beside.Path, beside.Decl, beside.ID, beside.Named = "z.go", "z.go:13:5", "z.go:13:5", []string{"z.go:5:6"}
	atPath := []Record{record(vocab.UnusedSatisfactionAssertion, "go://x#writer", "z.go:5:6", Ignore, 1)}
	atPath[0].Path = "z.go"
	ledger = Decide(atPath, []Finding{elsewhere, beside}, &noDials)
	if ledger.Withheld(&elsewhere) || !ledger.Withheld(&beside) {
		t.Errorf("Withheld = %t and %t, want the assertion in z.go alone", ledger.Withheld(&elsewhere), ledger.Withheld(&beside))
	}
}

// Under every subject, two alternatives a line directive renders at one
// reference and path are told apart by the declaration a record bound; a file
// or a document record takes no record at all.
func TestDecideTellsFoldedAlternativesApartUnderEverySubject(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		code             vocab.Code
		symbolKind, held string
		named            bool
		want             bool
	}{
		{vocab.UnreachableExport, "function", "", false, true},
		{vocab.UnusedSatisfactionAssertion, "satisfaction-assertion", "", true, true},
		{vocab.UnusedImport, "import", "", false, true},
		{vocab.DeadStore, "store", "", false, true},
		{vocab.UnusedParameter, "parameter", "go://x#Ping", false, true},
		{vocab.FileNeverBuilt, "file", "", false, false},
		{vocab.StaleSuppression, "suppression", "", false, false},
	} {
		alternative := func(decl string) Finding {
			found := Finding{Code: c.code, SymbolKind: c.symbolKind, Ref: "go://x#Ping", Holder: c.held, Path: "ping.proto", Decl: decl, ID: decl}
			if c.named {
				found.Named = []string{"w.go:3:6"}
			}
			return found
		}
		linux, windows := alternative("a_linux.go@40"), alternative("z_windows.go@40")
		bound := []Record{record(c.code, "go://x#Ping", "z_windows.go@40", Inline, 10)}
		bound[0].Path = "ping.proto"
		ledger := Decide(bound, []Finding{linux, windows}, &noDials)
		if ledger.Withheld(&linux) || ledger.Withheld(&windows) != c.want {
			t.Errorf("%s: Withheld = %t and %t, want false and %t", c.code, ledger.Withheld(&linux), ledger.Withheld(&windows), c.want)
		}
	}
}

// ReadDirective follows the grammar's decision procedure, and a malformed
// directive's error is of the malformed class.
func TestReadDirective(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		comment string
		want    Directive
	}{
		{"// a comment", Directive{Verdict: NotADirective}},
		{"//deadset:ignore DS1002,DS1404 -- kept for the plugin  ", Directive{Verdict: WellFormed, Codes: []vocab.Code{"DS1002", "DS1404"}, Reason: "kept for the plugin"}},
		{"// deadset:ignore DS1002 --", Directive{Verdict: Reasonless, Codes: []vocab.Code{"DS1002"}}},
		{"//deadset:ignore DS12 -- short", Directive{Verdict: Malformed}},
	} {
		got := ReadDirective(c.comment)
		if got.Verdict != c.want.Verdict || got.Reason != c.want.Reason || !slices.Equal(got.Codes, c.want.Codes) {
			t.Errorf("ReadDirective(%q) = %+v, want %+v", c.comment, got, c.want)
		}
	}
	if err := MalformedDirective("//deadset:ignore DS12", Site{Path: "a.go", Line: 5, Column: 1}); !errors.Is(err, ErrMalformed) {
		t.Errorf("MalformedDirective = %v, want ErrMalformed", err)
	}
}

// ValidReference reads the forms of the language a reference's scheme names.
func TestValidReference(t *testing.T) {
	t.Parallel()

	for ref, want := range map[string]bool{
		"go://example.com/app#helper":         true,
		"go://example.com/app#!":              false,
		"ts://@x/y/a.ts#f":                    true,
		"ts://no-file#f":                      false,
		"proto://pkg#Message":                 true,
		"rust://crate#f":                      false,
		"go:example.com/app#helper":           false,
		"go://example.com/app#helper(unused)": true,
	} {
		if got := ValidReference(ref); got != want {
			t.Errorf("ValidReference(%q) = %t, want %t", ref, got, want)
		}
	}
}
