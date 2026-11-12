package corpus

import (
	"slices"
	"testing"

	"github.com/cplieger/deadset-spec/v8/suppress"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// besideOf reads back every entry suppress.Beside writes, in its order, and
// reads none from a message that names none or spells an entry otherwise.
func TestBesideOfReadsTheEntriesTheStaleMessageWrites(t *testing.T) {
	t.Parallel()
	site := []suppress.Record{{Code: "DS1001", Bound: "a.go:3:6", Mechanism: suppress.Inline}}
	would := []suppress.Finding{
		{Code: "DS1102", Decl: "a.go:3:6", Confidence: vocab.Possible, Withheld: suppress.WithheldByMinConfidence},
		{Code: "DS1403", Decl: "a.go:3:6", Confidence: vocab.Certain},
		{Code: "DS1104", Decl: "a.go:3:6", Confidence: vocab.Certain, Withheld: suppress.WithheldBySeverity},
	}
	dials := &suppress.Dials{Allows: func(vocab.Code) bool { return false }}
	none := suppress.Decide(nil, nil, dials)
	claims := []suppress.Record{site[0], {Code: "DS1403", Bound: "a.go:3:6", Mechanism: suppress.Inline}}
	claimed := suppress.Decide(claims, would[1:2], dials)
	for _, c := range []struct {
		name, message string
		want          []Beside
	}{
		{name: "three", message: suppress.StaleMessage(site, would, none), want: []Beside{
			{Code: "DS1102", Confidence: "possible", Withheld: "analysis.min_confidence"},
			{Code: "DS1104", Confidence: "certain", Withheld: "severity allow"},
			{Code: "DS1403", Confidence: "certain"},
		}},
		{name: "one", message: suppress.StaleMessage(site, would[1:2], none), want: []Beside{{Code: "DS1403", Confidence: "certain"}}},
		{name: "claimed", message: suppress.StaleMessage(site, would[1:2], claimed), want: []Beside{
			{Code: "DS1403", Confidence: "certain", Withheld: "an inline directive"},
		}},
		{name: "none", message: suppress.StaleMessage(site, nil, none), want: []Beside{}},
		{name: "no-confidence", message: "inline directive for DS1001 matches no current finding, and its declaration reports DS1002", want: []Beside{}},
		{name: "no-and", message: "x, and its declaration reports DS1002 at certain; DS1102 at possible", want: []Beside{}},
	} {
		if got := besideOf(c.message); !slices.Equal(got, c.want) {
			t.Errorf("besideOf(%s: %q) = %+v, want %+v", c.name, c.message, got, c.want)
		}
	}
}
