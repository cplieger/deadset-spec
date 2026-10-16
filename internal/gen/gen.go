// Package gen writes every file of this module that is derived from the
// Contract's data: the published configuration schema expanded from its
// authoring source, the configuration key table derived from it and the
// configuration vectors' resolved configurations, the per-code branches of the
// finding schema, the closed lists the x-enum members of every authoring source
// name, the reference pages under docs/, the vocab package's data and the
// report package's document types. A freshness test regenerates and
// compares, so a derived file never drifts from the data it is derived from.
package gen

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"
)

// Generate returns every derived file, keyed by its slash path from the module
// root, from the sources fsys holds.
func Generate(fsys fs.FS) (map[string][]byte, error) {
	out := map[string][]byte{}
	steps := []func(fs.FS, map[string][]byte) error{
		configuration,
		findingSchema,
		publishedSchemas,
		docsPages,
		readmeCounts,
		vocabSource,
		reportTypes,
	}
	for _, step := range steps {
		if err := step(fsys, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// Paths lists the files Generate writes, in a stable order.
func Paths(files map[string][]byte) []string {
	return slices.Sorted(maps.Keys(files))
}

func readValue(fsys fs.FS, name string) (*value, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	v, err := parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return v, nil
}
