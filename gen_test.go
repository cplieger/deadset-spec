package spec_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
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

// TestAPlantedEdgeIdentifierEditReachesEveryCorpusSchema proves the edge
// identifier form has one authored source: a planted edit of the finding
// schema's definition reaches every inline copy the corpus schemas state.
func TestAPlantedEdgeIdentifierEditReachesEveryCorpusSchema(t *testing.T) {
	const source, planted = "authoring/finding.schema.json", `"pattern": "^planted$"`
	inputs := generatorInputs(t)
	var finding struct {
		Defs struct {
			EdgeIdentifier struct {
				Pattern string `json:"pattern"`
			} `json:"edge_identifier"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(inputs[source].Data, &finding); err != nil {
		t.Fatalf("Setup: decode %s: %v", source, err)
	}
	pattern, _ := json.Marshal(finding.Defs.EdgeIdentifier.Pattern)
	old := `"pattern": ` + string(pattern)
	plant(t, inputs, source, old, planted)
	files, err := gen.Generate(inputs)
	if err != nil {
		t.Fatalf("gen.Generate(planted) = %v", err)
	}
	for _, name := range []string{"corpus/expect.schema.json", "corpus/conformance-results.schema.json"} {
		got := files[name]
		if bytes.Contains(got, []byte(old)) || !bytes.Contains(got, []byte(planted)) {
			t.Errorf("%s keeps an edge identifier the planted definition does not state, want every copy written from it", name)
		}
		if bytes.Contains(got, []byte("x-inline")) {
			t.Errorf("%s carries x-inline, want the definition written in its place", name)
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
	ReadBy      []string `json:"read_by"`
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
// language constants, the per-language members, the delivery lists and every
// language list of every published schema, a fixed kind the severity vocabulary and
// the key names the published configuration schema admits.
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
		if got := settingsRowOf(t, files, "reporters.planted"); !slices.Equal(got.ReadBy, []string{"go", "ts"}) || !slices.Equal(got.Delivered, []string{"go", "ts"}) {
			t.Errorf("reporters.planted read_by, delivered = %q, %q, want the analyzed languages, [go ts], for both", got.ReadBy, got.Delivered)
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
	t.Run("a setting every analysis reads", func(t *testing.T) {
		inputs := generatorInputs(t)
		plant(t, inputs, "authoring/config.schema.json", `        "formats": {`,
			`        "planted": { "description": "d", "type": "boolean", "default": false, "x-owner": "analysis" },
        "formats": {`)
		files, err := gen.Generate(inputs)
		if err != nil {
			t.Fatalf("gen.Generate(planted) = %v", err)
		}
		if got := settingsRowOf(t, files, "reporters.planted"); !slices.Equal(got.ReadBy, []string{"go", "ts", "proto"}) || !slices.Equal(got.Delivered, []string{"go", "ts"}) {
			t.Errorf("reporters.planted read_by, delivered = %q, %q, want [go ts proto] read and only the process analyzers [go ts] delivered", got.ReadBy, got.Delivered)
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
		for _, path := range []string{"contract/config.schema.json", "contract/report.schema.json", "corpus/expect.schema.json"} {
			if want := `"The kind of the target's Python code."`; !bytes.Contains(files[path], []byte(want)) {
				t.Errorf("the published %s holds no %s, want the new language's member written in", path, want)
			}
		}
		wantLists := map[string][]string{
			"contract/config.schema.json": {
				"/properties/analysis/properties/languages/items/enum",
				"/properties/providers/properties/analyzers/items/oneOf/0/properties/languages/items/enum",
				"/properties/providers/properties/analyzers/items/oneOf/1/properties/languages/items/enum",
				"/properties/target/properties/kind/oneOf/1/required",
			},
			"contract/describe.schema.json":          {"/properties/languages/items/enum"},
			"contract/facts.schema.json":             {"/properties/language/enum"},
			"contract/finding.schema.json":           {"/properties/language/enum"},
			"contract/report.schema.json":            {"/properties/analyzer/properties/languages/items/enum"},
			"corpus/conformance-results.schema.json": {"/properties/product/properties/language/enum"},
			"corpus/expect.schema.json":              {"/properties/facts/items/properties/languages/items/enum", "/properties/languages/items/enum", "/properties/target_kind/oneOf/1/required"},
		}
		languages := []string{"py", "go", "ts", "proto"}
		// A provider entry claims a process language and a target kind is taken by an
		// analyzed language, so those lists hold the planted row and not proto.
		narrowed := map[string][]string{
			"/properties/providers/properties/analyzers/items/oneOf/0/properties/languages/items/enum": {"py", "go", "ts"},
			"/properties/providers/properties/analyzers/items/oneOf/1/properties/languages/items/enum": {"py", "go", "ts"},
			"/properties/target/properties/kind/oneOf/1/required":                                      {"py", "go", "ts"},
			"/properties/target_kind/oneOf/1/required":                                                 {"py", "go", "ts"},
		}
		for _, path := range gen.Paths(files) {
			if !strings.HasSuffix(path, ".schema.json") {
				continue
			}
			lists := languageLists(t, path, files[path], languages)
			if got, want := slices.Sorted(maps.Keys(lists)), wantLists[path]; !slices.Equal(got, want) {
				t.Errorf("the published %s holds language lists at %q, want them at %q", path, got, want)
			}
			for pointer, got := range lists {
				want, ok := narrowed[pointer]
				if !ok {
					want = languages
				}
				if !slices.Equal(got, want) {
					t.Errorf("the published %s holds %q at %s, want %q", path, got, pointer, want)
				}
			}
			delete(wantLists, path)
		}
		if missing := slices.Sorted(maps.Keys(wantLists)); len(missing) > 0 {
			t.Errorf("the generator writes no %q, want every published schema that carries a language list", missing)
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
		var schema struct {
			Properties struct {
				Severity struct {
					PropertyNames struct {
						Enum []string `json:"enum"`
					} `json:"propertyNames"`
				} `json:"severity"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(files["contract/config.schema.json"], &schema); err != nil {
			t.Fatalf("decoding the generated contract/config.schema.json = %v", err)
		}
		if names := schema.Properties.Severity.PropertyNames.Enum; !slices.Equal(names, got) {
			t.Errorf("the published severity key names = %q, want the severity vocabulary %q", names, got)
		}
	})
}

// TestAPlantedCommandLineEditReachesTheVocabulary proves the command line's
// generated handles move with their sources: an analyzer verb's option list,
// a kind of value an option takes, and a choice a setting's schema lists.
func TestAPlantedCommandLineEditReachesTheVocabulary(t *testing.T) {
	for _, tc := range []struct {
		name, path, old, planted, file, want string
	}{
		{
			name: "an analyzer verb's option", path: "contract/cli.json",
			old:     `"go": ["target", "config", "central", "min-confidence"],`,
			planted: `"go": ["target", "config", "central", "min-confidence", "sort"],`,
			file:    "vocab/vocab_gen.go", want: `Go: []string{"target", "config", "central", "min-confidence", "sort"}`,
		},
		{
			name: "a kind of value", path: "contract/cli.json",
			old: `"value": "integer"`, planted: `"value": "count"`,
			file: "vocab/vocab_gen.go", want: `OptionValueCount OptionValue = "count"`,
		},
		{
			name: "a choice", path: "authoring/config.schema.json",
			old: `"enum": ["position", "size"]`, planted: `"enum": ["position", "size", "planted"]`,
			file: "vocab/config_gen.go", want: `ReportersSortPlanted Choice = "planted"`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputs := generatorInputs(t)
			plant(t, inputs, tc.path, tc.old, tc.planted)
			files, err := gen.Generate(inputs)
			if err != nil {
				t.Fatalf("gen.Generate(planted) = %v", err)
			}
			if !regexp.MustCompile(strings.ReplaceAll(regexp.QuoteMeta(tc.want), " ", `\s+`)).Match(files[tc.file]) {
				t.Errorf("%s holds no %s, want the planted edit generated", tc.file, tc.want)
			}
		})
	}
}

// languageLists returns, by JSON pointer, every enum and required list of the
// schema doc that names two or more of languages. A list naming one language is
// a condition on that language, not a copy of the vocabulary.
func languageLists(t *testing.T, path string, doc []byte, languages []string) map[string][]string {
	t.Helper()
	var root any
	if err := json.Unmarshal(doc, &root); err != nil {
		t.Fatalf("decoding the generated %s = %v", path, err)
	}
	escape := strings.NewReplacer("~", "~0", "/", "~1")
	lists := map[string][]string{}
	var walk func(v any, pointer string)
	walk = func(v any, pointer string) {
		switch v := v.(type) {
		case map[string]any:
			for name, member := range v {
				at := pointer + "/" + escape.Replace(name)
				if items, ok := member.([]any); ok && (name == "enum" || name == "required") {
					values := make([]string, 0, len(items))
					named := 0
					for _, item := range items {
						s, _ := item.(string)
						values = append(values, s)
						if slices.Contains(languages, s) {
							named++
						}
					}
					if named >= 2 {
						lists[at] = values
					}
				}
				walk(member, at)
			}
		case []any:
			for i, item := range v {
				walk(item, pointer+"/"+strconv.Itoa(i))
			}
		}
	}
	walk(root, "")
	return lists
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
		{name: "an added value the list holds", path: "authoring/report.schema.json", old: `"enum": ["mixed"]`, planted: `"enum": ["library"]`, want: `repeats "library"`},
		{name: "a per-language column that is none", path: schema, old: `"when": "takes_target_kind"`, planted: `"when": "kind"`, want: `when "kind"`},
		{name: "an option naming no setting", path: "contract/cli.json", old: `"setting": "reporters.sort"`, planted: `"setting": "reporters.sorts"`, want: `"reporters.sorts", which is no setting`},
		{name: "a language column the registry lacks", path: "contract/languages.json", old: `"language": "go",`, planted: `"language": "go", "planted": true,`, want: `unknown field "planted"`},
		{name: "an inline copy of no definition", path: "authoring/expect.schema.json", old: `"x-inline": "authoring/finding.schema.json#/$defs/edge_identifier"`, planted: `"x-inline": "authoring/finding.schema.json#/$defs/planted"`, want: `x-inline "authoring/finding.schema.json#/$defs/planted" names no definition`},
		{name: "an inline copy beside a member it writes", path: "authoring/expect.schema.json", old: `"x-inline": "authoring/finding.schema.json#/$defs/edge_identifier"`, planted: `"type": "string", ` + `"x-inline": "authoring/finding.schema.json#/$defs/edge_identifier"`, want: `member "type" is both a sibling`},
		{name: "an inline copy of a definition that references another", path: "authoring/expect.schema.json", old: `"x-inline": "authoring/finding.schema.json#/$defs/edge_identifier"`, planted: `"x-inline": "authoring/finding.schema.json#/$defs/position"`, want: "references another"},
		{name: "an inline copy named by a number", path: "authoring/expect.schema.json", old: `"x-inline": "authoring/finding.schema.json#/$defs/edge_identifier"`, planted: `"x-inline": 1`, want: "x-inline is not a non-empty string"},
		{name: "an inline copy named by an empty string", path: "authoring/expect.schema.json", old: `"x-inline": "authoring/finding.schema.json#/$defs/edge_identifier"`, planted: `"x-inline": ""`, want: "x-inline is not a non-empty string"},
		{name: "an inline copy of a whole file", path: "authoring/expect.schema.json", old: `"x-inline": "authoring/finding.schema.json#/$defs/edge_identifier"`, planted: `"x-inline": "authoring/finding.schema.json"`, want: `x-inline "authoring/finding.schema.json" names no definition`},
		{name: "an inline copy of a boolean definition", path: "authoring/finding.schema.json", old: `"edge_identifier": {`, planted: `"edge_identifier": false, "planted": {`, want: "not an object"},
		{name: "an inline copy of a definition that inlines another", path: "authoring/finding.schema.json", old: `"edge_identifier": {`, planted: `"edge_identifier": { "x-inline": "authoring/finding.schema.json#/$defs/position",`, want: "inlines another"},
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
