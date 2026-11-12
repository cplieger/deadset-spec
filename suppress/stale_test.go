package suppress

import (
	"slices"
	"testing"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// The records of one site make one finding, records of another mechanism at
// the same position another, in reading order.
func TestStaleSitesGroupByMechanismAndSite(t *testing.T) {
	t.Parallel()

	a := Site{Path: "a.proto", File: "a.proto", Line: 3, Column: 1}
	records := []Record{
		{Code: "DS1801", Mechanism: Inline, Site: a},
		{Code: "DS1802", Mechanism: Ignore, Site: Site{Path: "deadset-ignore.json", Line: 2, Column: 3}},
		{Code: "DS1803", Mechanism: Inline, Site: a},
		{Code: "DS1804", Mechanism: Ignore, Site: a},
	}
	sites := StaleSites(records)
	if len(sites) != 3 || len(sites[0]) != 2 || sites[0][1].Code != "DS1803" || sites[1][0].Code != "DS1802" || sites[2][0].Code != "DS1804" {
		t.Errorf("StaleSites = %+v, want the two inline records together, then each ignore record alone", sites)
	}
}

// A stale site names its codes once each in order, and every other code a
// declaration it bound would report, each with its finding's confidence and what
// withholds it, once per distinct entry, never a code of a declaration it did not
// bind or of no declaration.
func TestStaleMessageNamesWhatTheDeclarationReports(t *testing.T) {
	t.Parallel()

	site := []Record{
		{Code: "DS1803", Bound: "a.proto:4:3", Mechanism: Inline},
		{Code: "DS1801", Bound: "a.proto:4:3", Mechanism: Inline},
		{Code: "DS1801", Bound: "a.proto:9:3", Mechanism: Inline},
	}
	would := []Finding{
		{Code: "DS1802", Decl: "a.proto:4:3", Confidence: vocab.Possible, Withheld: WithheldByMinConfidence},
		{Code: "DS1801", Decl: "a.proto:4:3", Confidence: vocab.Certain},
		{Code: "DS1805", Decl: "a.proto:9:3", Confidence: vocab.Certain},
		{Code: "DS1802", Decl: "a.proto:9:3", Confidence: vocab.Certain, Withheld: WithheldBySeverity},
		{Code: "DS1802", Decl: "a.proto:4:3", Confidence: vocab.Certain},
		{Code: "DS1805", Decl: "a.proto:4:3", Confidence: vocab.Certain},
		{Code: "DS1804", Decl: "a.proto:20:1", Confidence: vocab.Certain},
		{Code: vocab.StaleSuppression, Confidence: vocab.Certain},
	}
	held := []Finding{
		{Code: "DS1802", Decl: "a.proto:4:3", Confidence: vocab.Probable, Withheld: WithheldByTypeError},
	}
	for _, c := range []struct {
		name  string
		site  []Record
		would []Finding
		want  string
	}{
		{name: "beside", site: site, would: would, want: "inline directive for DS1801 and DS1803 matches no current finding, and its declaration reports " +
			"DS1802 at certain; DS1802 at certain, withheld by severity allow; DS1802 at possible, withheld by analysis.min_confidence; and DS1805 at certain"},
		{name: "one-beside", site: site[:1], would: would[:1], want: "inline directive for DS1803 matches no current finding, and its declaration reports DS1802 at possible, withheld by analysis.min_confidence"},
		{name: "held", site: site[:1], would: held, want: "inline directive for DS1803 matches no current finding, and its declaration reports DS1802 at probable, withheld by a type error"},
		{name: "nothing-beside", site: site[:1], would: would[6:], want: "inline directive for DS1803 matches no current finding"},
		{name: "bound-to-nothing", site: []Record{{Code: "DS1801", Mechanism: Baseline}}, would: would, want: "baseline row for DS1801 matches no current finding"},
	} {
		if got := StaleMessage(c.site, c.would, Decide(nil, nil, &noDials)); got != c.want {
			t.Errorf("StaleMessage(%s) = %q, want %q", c.name, got, c.want)
		}
	}
}

// A finding beside a stale site that another record matches is named withheld by
// that record's mechanism, unless a dial withholds it, which is then named.
func TestStaleMessageNamesTheRecordThatMatchesAFindingBeside(t *testing.T) {
	t.Parallel()

	const ref = "go://x#F"
	would := []Finding{
		declaration(vocab.UnusedExported, ref, 3),
		declaration(vocab.UnreachableExport, ref, 3),
		declaration(vocab.TestOnlyUse, ref, 3),
		declaration(vocab.DeprecatedUnused, ref, 3),
		declaration(vocab.WriteOnlySymbol, ref, 3),
	}
	for i := range would {
		would[i].Confidence = vocab.Certain
	}
	would[3].Withheld = WithheldBySeverity
	would[4].Confidence = vocab.Possible
	decl := would[0].Decl
	records := []Record{
		record(vocab.UnusedUnexported, ref, decl, Inline, 2),
		record(vocab.UnusedExported, ref, decl, Inline, 2),
		record(vocab.UnreachableExport, ref, decl, Ignore, 1),
		record(vocab.TestOnlyUse, ref, decl, Baseline, 1),
		record(vocab.DeprecatedUnused, ref, decl, Baseline, 2),
	}
	ledger := Decide(records, would, &noDials)
	if want := []Verdict{Stale, InEffect, InEffect, InEffect, Dormant}; !slices.Equal(ledger.Verdicts, want) {
		t.Fatalf("Decide = %v, want %v", ledger.Verdicts, want)
	}
	sites := StaleSites(StaleRecords(records, ledger))
	want := "inline directive for DS1002 matches no current finding, and its declaration reports " +
		"DS1001 at certain, withheld by an inline directive; DS1101 at certain, withheld by an ignore entry; " +
		"DS1102 at certain, withheld by a baseline row; DS1104 at certain, withheld by severity allow; and DS1301 at possible"
	if len(sites) != 1 {
		t.Fatalf("StaleSites = %+v, want the DS1002 directive alone", sites)
	}
	if got := StaleMessage(sites[0], would, ledger); got != want {
		t.Errorf("StaleMessage(a site beside findings other records match) = %q, want %q", got, want)
	}
}

// Each mechanism is named as a message names one suppression of it.
func TestMechanismWritten(t *testing.T) {
	t.Parallel()

	for m, want := range map[Mechanism]string{Inline: "inline directive", Ignore: "ignore entry", Baseline: "baseline row"} {
		if got := m.Written(); got != want {
			t.Errorf("%s.Written() = %q, want %q", m, got, want)
		}
	}
}
