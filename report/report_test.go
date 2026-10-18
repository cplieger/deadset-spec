package report

import "testing"

// The withheld line names the probable and the possible count, each only where
// it is not 0, and the lowest confidence it names as the setting that shows
// them all; a run that withheld nothing it would name writes no line.
func TestTheWithheldLineNamesEachCountItHolds(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		withheld Withheld
		want     string
	}{
		{name: "none", want: ""},
		{
			name: "probable only", withheld: Withheld{Probable: 1},
			want: "withheld by analysis.min_confidence: 1 probable, shown with analysis.min_confidence set to probable",
		},
		{
			name: "possible only", withheld: Withheld{Possible: 5},
			want: "withheld by analysis.min_confidence: 5 possible, shown with analysis.min_confidence set to possible",
		},
		{
			name: "both", withheld: Withheld{Probable: 2, Possible: 5},
			want: "withheld by analysis.min_confidence: 2 probable, 5 possible, shown with analysis.min_confidence set to possible",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			if got := c.withheld.Line(); got != c.want {
				t.Errorf("%+v.Line() = %q, want %q", c.withheld, got, c.want)
			}
		})
	}
}

func TestDistinctLinesCountsALineSeveralPositionsSpanOnce(t *testing.T) {
	t.Parallel()

	at := func(path string, line, end int) Position {
		return Position{Path: path, Line: line, Column: 1, EndLine: end}
	}
	cases := []struct {
		name      string
		positions []Position
		want      int
	}{
		{name: "none", want: 0},
		{name: "one", positions: []Position{at("a.go", 3, 9)}, want: 7},
		{name: "two on one line", positions: []Position{at("a.go", 5, 5), at("a.go", 5, 5)}, want: 1},
		{name: "a field inside its struct", positions: []Position{at("a.go", 4, 4), at("a.go", 2, 6)}, want: 5},
		{name: "overlapping", positions: []Position{at("a.go", 2, 6), at("a.go", 5, 8)}, want: 7},
		{name: "adjacent", positions: []Position{at("a.go", 2, 3), at("a.go", 4, 5)}, want: 4},
		{name: "disjoint", positions: []Position{at("a.go", 9, 9), at("a.go", 2, 3)}, want: 3},
		{name: "one line in two files", positions: []Position{at("b.go", 5, 5), at("a.go", 5, 5)}, want: 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := DistinctLines(c.positions); got != c.want {
				t.Errorf("DistinctLines(%+v) = %d, want %d", c.positions, got, c.want)
			}
		})
	}
}
