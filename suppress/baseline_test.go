package suppress

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// A write ends with the first round that records and removes nothing, keeps
// every row in the order recorded, and never records a removed row again.
func TestFixpointEndsWithTheFirstRoundThatChangesNothing(t *testing.T) {
	t.Parallel()

	a := Recorded{Code: "DS1001", Symbol: "go://x#A", Path: "a.go"}
	b := Recorded{Code: "DS1002", Symbol: "go://x#b", Path: "b.go"}
	var reads [][]Recorded
	got, err := Fixpoint(func(read []Recorded) ([]Recorded, []Recorded, error) {
		reads = append(reads, read)
		switch len(reads) {
		case 1:
			return []Recorded{a, b}, nil, nil
		case 2:
			return []Recorded{b}, []Recorded{b}, nil
		default:
			return []Recorded{b}, nil, nil
		}
	})
	if err != nil {
		t.Fatalf("Fixpoint = %v", err)
	}
	if !slices.Equal(got, []Recorded{a}) || len(reads) != 3 {
		t.Errorf("Fixpoint = %v after %d rounds, want [%v] after 3: a removed row is never recorded again", got, len(reads), a)
	}
	if !slices.Equal(reads[1], []Recorded{a, b}) || !slices.Equal(reads[2], []Recorded{a}) {
		t.Errorf("the rounds read %v, want the rows recorded so far", reads)
	}
}

// A refused round ends the write with its error.
func TestFixpointEndsWithARoundsError(t *testing.T) {
	t.Parallel()

	refused := errors.New("the analysis failed")
	if _, err := Fixpoint(func([]Recorded) ([]Recorded, []Recorded, error) { return nil, nil, refused }); !errors.Is(err, refused) {
		t.Errorf("Fixpoint = %v, want %v", err, refused)
	}
}

// A write names its analyzer in every reason, and refuses an identity or a
// finding that cannot give a row one.
func TestWriteBaselineRefusesARowWithoutProvenance(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		rows []Recorded
		p    Provenance
	}{
		"no version":         {p: Provenance{Analyzer: "deadset-go"}},
		"a row's path":       {rows: []Recorded{{Code: "DS1001", Symbol: "go://x#A"}}, p: Provenance{Analyzer: "deadset-go", Version: "1.0.0"}},
		"no analyzer at all": {},
	} {
		if err := WriteBaseline(&strings.Builder{}, tt.rows, tt.p, nil); !errors.Is(err, errProvenance) {
			t.Errorf("WriteBaseline(%s) = %v, want %v", name, err, errProvenance)
		}
	}
}
