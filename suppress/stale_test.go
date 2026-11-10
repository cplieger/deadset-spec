package suppress

import (
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
// declaration it bound would report, never a code of a declaration it did not
// bind or of no declaration.
func TestStaleMessageNamesWhatTheDeclarationReports(t *testing.T) {
	t.Parallel()

	site := []Record{
		{Code: "DS1803", Bound: "a.proto:4:3", Mechanism: Inline},
		{Code: "DS1801", Bound: "a.proto:4:3", Mechanism: Inline},
		{Code: "DS1801", Bound: "a.proto:9:3", Mechanism: Inline},
	}
	would := []Finding{
		{Code: "DS1802", Decl: "a.proto:4:3"},
		{Code: "DS1801", Decl: "a.proto:4:3"},
		{Code: "DS1805", Decl: "a.proto:9:3"},
		{Code: "DS1802", Decl: "a.proto:9:3"},
		{Code: "DS1804", Decl: "a.proto:20:1"},
		{Code: vocab.StaleSuppression},
	}
	for _, c := range []struct {
		name  string
		site  []Record
		would []Finding
		want  string
	}{
		{name: "beside", site: site, would: would, want: "inline directive for DS1801 and DS1803 matches no current finding, and its declaration reports DS1802 and DS1805"},
		{name: "nothing-beside", site: site[:1], would: would[4:], want: "inline directive for DS1803 matches no current finding"},
		{name: "bound-to-nothing", site: []Record{{Code: "DS1801", Mechanism: Baseline}}, would: would, want: "baseline row for DS1801 matches no current finding"},
	} {
		if got := StaleMessage(c.site, c.would); got != c.want {
			t.Errorf("StaleMessage(%s) = %q, want %q", c.name, got, c.want)
		}
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
