package spec_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cplieger/deadset-spec/v8/internal/gen"
)

// TestGeneratedFilesAreFresh regenerates every derived file from the sources on
// disk and compares it byte for byte with the committed file: a source edit
// nobody regenerated fails here. Run go generate to rewrite them.
func TestGeneratedFilesAreFresh(t *testing.T) {
	files, err := gen.Generate(os.DirFS("."))
	if err != nil {
		t.Fatalf("gen.Generate(.) = %v", err)
	}
	for _, name := range gen.Paths(files) {
		t.Run(strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			got, err := os.ReadFile(name)
			if err != nil {
				t.Fatalf("Setup: os.ReadFile(%s) = %v, want the committed generated file", name, err)
			}
			if !bytes.Equal(got, files[name]) {
				t.Errorf("%s differs from what deadset-gen writes from its sources: run go generate", name)
			}
		})
	}
}

// TestNoStaleGeneratedPage pins that every page under docs/kinds is one the
// generator writes, so a family that leaves the Contract takes its page with it.
func TestNoStaleGeneratedPage(t *testing.T) {
	files, err := gen.Generate(os.DirFS("."))
	if err != nil {
		t.Fatalf("gen.Generate(.) = %v", err)
	}
	pages, err := fs.Glob(os.DirFS("."), "docs/kinds/*.md")
	if err != nil {
		t.Fatalf("Setup: fs.Glob = %v", err)
	}
	for _, page := range pages {
		if _, ok := files[page]; !ok {
			t.Errorf("%s is no page the generator writes: delete it", page)
		}
	}
}

// TestAPlantedKindsEditMovesTheGeneratedFiles proves the freshness test bites: a
// kind's renamed row changes the pages and the finding schema generated from it.
func TestAPlantedKindsEditMovesTheGeneratedFiles(t *testing.T) {
	disk := os.DirFS(".")
	planted := generatorInputs(t)
	kinds := planted["contract/kinds.json"]
	kinds.Data = bytes.Replace(kinds.Data, []byte(`"name": "unused-member"`), []byte(`"name": "planted-member"`), 1)
	kinds.Data = bytes.Replace(kinds.Data, []byte(`"holds_live": true`), []byte(`"holds_live": false`), 1)
	before, err := gen.Generate(disk)
	if err != nil {
		t.Fatalf("gen.Generate(.) = %v", err)
	}
	after, err := gen.Generate(planted)
	if err != nil {
		t.Fatalf("gen.Generate(planted) = %v", err)
	}
	for _, name := range []string{"docs/kinds/unused.md", "contract/finding.schema.json", "vocab/vocab_gen.go"} {
		if bytes.Equal(before[name], after[name]) {
			t.Errorf("%s is unchanged by a planted contract/kinds.json edit, want it regenerated from the row", name)
		}
	}
}

// generatorInputs is a copy of every file of the module the generator reads.
func generatorInputs(t *testing.T) fstest.MapFS {
	t.Helper()
	disk := os.DirFS(".")
	inputs := fstest.MapFS{}
	err := fs.WalkDir(disk, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !generatorInput(path) {
			return err
		}
		data, err := fs.ReadFile(disk, path)
		inputs[path] = &fstest.MapFile{Data: data}
		return err
	})
	if err != nil {
		t.Fatalf("Setup: walking the module = %v", err)
	}
	return inputs
}

// plant replaces the first old in one input file with planted, failing where
// the file holds no old, so a reworded source cannot make a planted test vacuous.
func plant(t *testing.T, inputs fstest.MapFS, path, old, planted string) {
	t.Helper()
	file := inputs[path]
	if file == nil || !bytes.Contains(file.Data, []byte(old)) {
		t.Fatalf("Setup: %s holds no %q to plant over", path, old)
	}
	file.Data = bytes.Replace(file.Data, []byte(old), []byte(planted), 1)
}

// generatorInput reports whether the generator reads a file of the module.
func generatorInput(path string) bool {
	return path == "README.md" || path == "corpus/corpus.json" || strings.HasSuffix(path, "/expect.json") ||
		slices.ContainsFunc([]string{"contract/", "authoring/", "vectors/config/"}, func(p string) bool { return strings.HasPrefix(path, p) })
}

// settingsRow is what the configuration tests read of a contract/settings.json row.
type settingsRow struct {
	Key         string   `json:"key"`
	Delivered   []string `json:"delivered"`
	Values      []string `json:"values"`
	PerLanguage []string `json:"per_language"`
}

// settingsRowOf decodes generated contract/settings.json and returns one key's row.
func settingsRowOf(t *testing.T, files map[string][]byte, key string) settingsRow {
	t.Helper()
	var doc struct {
		Keys []settingsRow `json:"keys"`
	}
	if err := json.Unmarshal(files["contract/settings.json"], &doc); err != nil {
		t.Fatalf("decoding the generated contract/settings.json = %v", err)
	}
	for _, row := range doc.Keys {
		if row.Key == key {
			return row
		}
	}
	t.Fatalf("the generated contract/settings.json holds no row for %s", key)
	return settingsRow{}
}

// TestAPlantedConfigurationEditReachesEveryGeneratedTable proves one edit of the
// Contract's data moves the generated files that carry it: a setting reaches the
// key table, its Go handle and the configuration vectors, a language the
// language constants, the per-language members, the delivery lists and the
// published schema, a fixed kind the severity vocabulary.
func TestAPlantedConfigurationEditReachesEveryGeneratedTable(t *testing.T) {
	t.Run("a setting", func(t *testing.T) {
		inputs := generatorInputs(t)
		plant(t, inputs, "authoring/config.schema.json", `        "formats": {`,
			`        "planted": { "description": "d", "type": "boolean", "default": false, "x-owner": "analyzer" },
        "formats": {`)
		files, err := gen.Generate(inputs)
		if err != nil {
			t.Fatalf("gen.Generate(planted) = %v", err)
		}
		if got := settingsRowOf(t, files, "reporters.planted").Delivered; !slices.Equal(got, []string{"go", "ts"}) {
			t.Errorf("reporters.planted delivered = %q, want every analyzer that reads it, [go ts]", got)
		}
		if !regexp.MustCompile(`KeyReportersPlanted\s+Key = "reporters.planted"`).Match(files["vocab/config_gen.go"]) {
			t.Error("vocab/config_gen.go declares no KeyReportersPlanted, want the planted setting's handle")
		}
		for path, want := range map[string]string{
			"vectors/config/template-delimiters-configured/expected.json":      `"reporters.planted": "default"`,
			"vectors/config/resolved-configuration-round-trip/repository.json": `"reporters.planted": "repository: repository.json"`,
		} {
			if !bytes.Contains(files[path], []byte(want)) {
				t.Errorf("the generated %s holds no %s, want the planted setting resolved", path, want)
			}
		}
	})
	t.Run("a language that takes a target kind", func(t *testing.T) {
		inputs := generatorInputs(t)
		plant(t, inputs, "contract/languages.json", `  "languages": [
`, `  "languages": [
    {
      "language": "py", "name": "Python", "kind": "analyzed", "scheme": "py", "runner": "process", "analyzer": "deadset-py",
      "markers": ["pyproject.toml"], "extensions": [".py"], "companions": [], "program_without_marker": false,
      "skipped_directories": [], "identity_source": { "file": "pyproject.toml", "field": "name" },
      "workspace_files": [], "takes_target_kind": true
    },
`)
		files, err := gen.Generate(inputs)
		if err != nil {
			t.Fatalf("gen.Generate(planted) = %v", err)
		}
		if got := settingsRowOf(t, files, "target.kind").PerLanguage; !slices.Equal(got, []string{"py", "go", "ts"}) {
			t.Errorf("target.kind per_language = %q, want [py go ts]", got)
		}
		if got := settingsRowOf(t, files, "analysis.min_confidence").Delivered; !slices.Contains(got, "py") {
			t.Errorf("analysis.min_confidence delivered = %q, want the new analyzer among them", got)
		}
		if !regexp.MustCompile(`PY\s+Language = "py"`).Match(files["vocab/vocab_gen.go"]) {
			t.Error("vocab/vocab_gen.go declares no PY, want the Language constant the generated tables name")
		}
		for _, want := range []string{`"enum": ["py", "go", "ts"]`, `"required": ["py", "go", "ts"]`, `"The kind of the target's Python code."`} {
			if !bytes.Contains(files["contract/config.schema.json"], []byte(want)) {
				t.Errorf("the published configuration schema holds no %s, want the new language written in", want)
			}
		}
	})
	t.Run("a fixed kind", func(t *testing.T) {
		inputs := generatorInputs(t)
		plant(t, inputs, "contract/kinds.json", `"fixed": false`, `"fixed": true`)
		files, err := gen.Generate(inputs)
		if err != nil {
			t.Fatalf("gen.Generate(planted) = %v", err)
		}
		got := settingsRowOf(t, files, "severity").Values
		if slices.Contains(got, "DS1001") || slices.Contains(got, "DS10") {
			t.Errorf("severity values = %q, want neither the fixed code DS1001 nor its family DS10", got)
		}
		if !slices.Contains(got, "DS1002") {
			t.Errorf("severity values = %q, want DS1002, a kind the Contract still leaves configurable", got)
		}
	})
}

// TestAPlantedConfigurationDefectFailsGeneration proves each configuration
// metadata defect it lists fails generation.
func TestAPlantedConfigurationDefectFailsGeneration(t *testing.T) {
	const schema = "authoring/config.schema.json"
	for _, tc := range []struct {
		name, path, old, planted, want string
	}{
		{name: "an owner naming no language", path: schema, old: `"x-owner": "orchestrator"`, planted: `"x-owner": "py-analyzer"`, want: `x-owner "py-analyzer"`},
		{name: "a delivery list written by hand", path: schema, old: `"x-delivered": false`, planted: `"x-delivered": ["go"]`, want: "x-delivered is"},
		{name: "a section kept by the orchestrator", path: schema, old: `"x-owner": "go-analyzer",`, planted: `"x-owner": "go-analyzer", "x-delivered": false,`, want: "x-delivered on a section"},
		{name: "an unknown value list", path: schema, old: `"x-enum": "severities"`, planted: `"x-enum": "planted"`, want: `x-enum "planted"`},
		{name: "an unknown vocabulary", path: schema, old: `"x-vocabulary": "exemption-classes"`, planted: `"x-vocabulary": "planted"`, want: `x-vocabulary "planted"`},
		{name: "a per-language column that is none", path: schema, old: `"when": "takes_target_kind"`, planted: `"when": "kind"`, want: `when "kind"`},
		{name: "an option naming no setting", path: "contract/cli.json", old: `"setting": "reporters.sort"`, planted: `"setting": "reporters.sorts"`, want: `"reporters.sorts", which is no setting`},
		{name: "a language column the registry lacks", path: "contract/languages.json", old: `"language": "go",`, planted: `"language": "go", "planted": true,`, want: `unknown field "planted"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputs := generatorInputs(t)
			plant(t, inputs, tc.path, tc.old, tc.planted)
			_, err := gen.Generate(inputs)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("gen.Generate(planted) = %v, want an error naming %s", err, tc.want)
			}
		})
	}
}
