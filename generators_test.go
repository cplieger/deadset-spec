package spec_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v8/vocab"
	"golang.org/x/mod/semver"
)

// firstRow is the first generator row, in document order, of one plugin's
// output under one header and runtime import that a version matches: a row
// with no interval matches every version, as bindings.4's fallback does.
func firstRow(rows []vocab.GeneratorRow, of *vocab.GeneratorRow, version string) *vocab.GeneratorRow {
	for i := range rows {
		row := &rows[i]
		if row.Plugin != of.Plugin || row.Language != of.Language || row.Header != of.Header || row.RuntimeImport != of.RuntimeImport {
			continue
		}
		if (row.VersionMin == "" || semver.Compare(version, row.VersionMin) >= 0) &&
			(row.VersionMax == "" || semver.Compare(version, row.VersionMax) <= 0) {
			return row
		}
	}
	return nil
}

// nextPatch and previous are the versions just above and just below v.
func nextPatch(v string) string {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	patch, _ := strconv.Atoi(parts[2])
	return fmt.Sprintf("v%s.%s.%d", parts[0], parts[1], patch+1)
}

func previous(v string) string {
	parts := strings.Split(strings.TrimPrefix(v, "v"), ".")
	minor, _ := strconv.Atoi(parts[1])
	patch, _ := strconv.Atoi(parts[2])
	switch {
	case patch > 0:
		return fmt.Sprintf("v%s.%d.%d", parts[0], minor, patch-1)
	case minor > 0:
		return fmt.Sprintf("v%s.%d.999", parts[0], minor-1)
	}
	return ""
}

// TestEveryBoundedGeneratorRowHasALaterRowAtItsBounds holds the output one
// version past each end of a row's interval to a row of the same plugin,
// header and runtime import, so bindings.4's fallback has a row to bind by.
func TestEveryBoundedGeneratorRowHasALaterRowAtItsBounds(t *testing.T) {
	rows := vocab.GeneratorRows()
	for i := range rows {
		row := &rows[i]
		if row.VersionMin == "" && row.VersionMax == "" {
			continue
		}
		var outside []string
		if row.VersionMin != "" {
			outside = append(outside, previous(row.VersionMin))
		}
		if row.VersionMax != "" {
			outside = append(outside, nextPatch(row.VersionMax))
		}
		for _, version := range outside {
			if version == "" {
				continue
			}
			t.Run(row.ID+"@"+version, func(t *testing.T) {
				if matched := firstRow(rows, row, version); matched == nil {
					t.Errorf("output of %s %s matches no row, want a row binding it", row.Plugin, version)
				}
			})
		}
	}
}

// TestGeneratorBoundsMatchTheFallbackRows pins each bounded family's outputs
// at its bounds to the row they bind by.
func TestGeneratorBoundsMatchTheFallbackRows(t *testing.T) {
	rows := vocab.GeneratorRows()
	byID := map[string]*vocab.GeneratorRow{}
	for i := range rows {
		byID[rows[i].ID] = &rows[i]
	}
	for _, c := range []struct{ row, version, want string }{
		{"protoc-gen-go-1", "v1.36.12", "protoc-gen-go-1"},
		{"protoc-gen-go-1", "v1.36.13", "protoc-gen-go-1-fallback"},
		{"protoc-gen-go-1", "v1.34.0", "protoc-gen-go-1-fallback"},
		{"protoc-gen-go-grpc-1", "v1.6.2", "protoc-gen-go-grpc-1"},
		{"protoc-gen-go-grpc-1", "v1.6.3", "protoc-gen-go-grpc-1-fallback"},
		{"protoc-gen-go-grpc-1", "v1.0.1", "protoc-gen-go-grpc-unsupported"},
		{"protoc-gen-es-2", "v2.16.1", "protoc-gen-es-2-fallback"},
		{"protoc-gen-es-2", "v2.9.0", "protoc-gen-es-2-fallback"},
		{"ts-proto-2", "v2.15.1", "ts-proto-2-fallback"},
		{"ts-proto-2", "v2.11.3", "ts-proto-2-fallback"},
	} {
		t.Run(c.row+"@"+c.version, func(t *testing.T) {
			of := byID[c.row]
			if of == nil {
				t.Fatalf("Setup: no row %s", c.row)
			}
			if got := firstRow(rows, of, c.version); got == nil || got.ID != c.want {
				t.Errorf("first row = %v, want %s", got, c.want)
			}
		})
	}
}
