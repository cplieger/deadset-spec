package gen

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

const (
	readmePath   = "README.md"
	countsOpen   = "<!-- corpus-counts -->"
	countsClose  = "<!-- /corpus-counts -->"
	fixturesRoot = "corpus/fixtures"
)

// readmeCounts rewrites the README's sentence on the corpus from the fixtures
// on disk, so the counts it states are the corpus's.
func readmeCounts(fsys fs.FS, out map[string][]byte) error {
	data, err := fs.ReadFile(fsys, readmePath)
	if err != nil {
		return err
	}
	text := string(data)
	start, end := strings.Index(text, countsOpen), strings.Index(text, countsClose)
	if start < 0 || end < start {
		return fmt.Errorf("%s: no %s ... %s section", readmePath, countsOpen, countsClose)
	}
	entries, err := fs.ReadDir(fsys, fixturesRoot)
	if err != nil {
		return err
	}
	var total, both, goOnly, tsOnly int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		raw, err := fs.ReadFile(fsys, path.Join(fixturesRoot, e.Name(), "expect.json"))
		if err != nil {
			return err
		}
		var fixture struct {
			Languages []string `json:"languages"`
		}
		if err := json.Unmarshal(raw, &fixture); err != nil {
			return fmt.Errorf("%s/%s: %w", fixturesRoot, e.Name(), err)
		}
		total++
		switch {
		case slices.Contains(fixture.Languages, "go") && slices.Contains(fixture.Languages, "ts"):
			both++
		case slices.Contains(fixture.Languages, "go"):
			goOnly++
		default:
			tsOnly++
		}
	}
	sentence := fmt.Sprintf("`corpus/` holds %d fixture projects. For each one, an `expect.json` file names in language-neutral terms what an analyzer must and must not report for each issue kind and each exemption class. Of these, %d apply to both languages, %d to Go only and %d to TypeScript only.", total, both, goOnly, tsOnly)
	out[readmePath] = []byte(text[:start+len(countsOpen)] + sentence + text[end:])
	return nil
}
