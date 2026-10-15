package corpus

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	spec "github.com/cplieger/deadset-spec/v8"
)

// TestCheckHoldsTheEmbeddedCorpus is the corpus's own gate: every fixture the
// spec module publishes meets the format and the coverage rules.
func TestCheckHoldsTheEmbeddedCorpus(t *testing.T) {
	for _, err := range Check(spec.Corpus) {
		t.Error(err)
	}
}

// plantedCorpus is files beside the corpus document and the expectation schema
// the spec module publishes, which is the tree Check reads.
func plantedCorpus(files fstest.MapFS) fstest.MapFS {
	for _, name := range []string{DocumentPath, ExpectSchemaPath} {
		data, err := spec.Corpus.ReadFile(name)
		if err != nil {
			panic(err)
		}
		files[name] = &fstest.MapFile{Data: data}
	}
	return files
}

// checkPlanted is every defect Check names in a planted tree, joined.
func checkPlanted(files fstest.MapFS) error {
	return errors.Join(Check(plantedCorpus(files))...)
}

func TestResolveExpectationsBinds(t *testing.T) {
	line := func(n int) *int { return &n }
	cases := []struct {
		name     string
		expect   Expectation
		manifest Manifest
		want     map[string]Site
	}{
		{
			name: "one_reported_one_retained",
			expect: Expectation{Expect: []Row{
				{Symbol: "DeadExport", Report: "DS1001", Confidence: "certain"},
				{Symbol: "SatisfiesWriter", Report: "none", RetainedBy: []string{"interface-satisfaction"}},
			}},
			manifest: Manifest{Symbols: map[string]ManifestPosition{
				"DeadExport":      {File: "target/dead.go", Line: line(5)},
				"SatisfiesWriter": {File: "target/writer.go", Line: line(12)},
			}},
			want: map[string]Site{
				"DeadExport":      {File: "target/dead.go", Line: 5},
				"SatisfiesWriter": {File: "target/writer.go", Line: 12},
			},
		},
		{
			name:     "dotted_member",
			expect:   Expectation{Expect: []Row{{Symbol: "Sink.Write", Report: "none"}}},
			manifest: Manifest{Symbols: map[string]ManifestPosition{"Sink.Write": {File: "target/sink.ts", Line: line(3)}}},
			want:     map[string]Site{"Sink.Write": {File: "target/sink.ts", Line: 3}},
		},
		{
			// A configured root resolves to the entry the fixture configures and
			// so to no position, because the finding about it is positioned in
			// the configuration document the runner wrote.
			name: "a_configured_root_beside_a_declaration",
			expect: Expectation{
				ConfiguredRoots: map[string]string{"AbsentPattern": "go://example.com/app#absent*"},
				Expect: []Row{
					{Symbol: "AbsentPattern", Report: "DS9004", Confidence: "certain"},
					{Symbol: "PlainDead", Report: "DS1002", Confidence: "certain"},
				},
			},
			manifest: Manifest{Symbols: map[string]ManifestPosition{"PlainDead": {File: "target/main.go", Line: line(12)}}},
			want:     map[string]Site{"PlainDead": {File: "target/main.go", Line: 12}},
		},
		{
			// A configured declaration resolves as a configured root does.
			name: "a_configured_declaration_beside_a_declaration",
			expect: Expectation{
				ConfiguredDeclarations: map[string]ConfiguredDeclaration{"AbsentEntry": {Key: "ts.serializers", Entry: map[string]string{"global": "JSON.absent"}}},
				Expect: []Row{
					{Symbol: "AbsentEntry", Report: "DS9006", Confidence: "certain"},
					{Symbol: "PlainDead", Report: "DS1002", Confidence: "certain"},
				},
			},
			manifest: Manifest{Symbols: map[string]ManifestPosition{"PlainDead": {File: "target/main.ts", Line: line(12)}}},
			want:     map[string]Site{"PlainDead": {File: "target/main.ts", Line: 12}},
		},
		{
			name: "a_name_literal_two_rows_share",
			expect: Expectation{Expect: []Row{
				{Symbol: "Spelled", Report: "DS1002", Confidence: "possible", Details: &Details{NameLiteral: "SpelledLiteral"}},
				{Symbol: "SpelledToo", Report: "DS1002", Confidence: "possible", Details: &Details{NameLiteral: "SpelledLiteral"}},
			}},
			manifest: Manifest{Symbols: map[string]ManifestPosition{
				"Spelled":        {File: "target/a.go", Line: line(3)},
				"SpelledToo":     {File: "target/a.go", Line: line(4)},
				"SpelledLiteral": {File: "target/b.go", Line: line(7)},
			}},
			want: map[string]Site{
				"Spelled":        {File: "target/a.go", Line: 3},
				"SpelledToo":     {File: "target/a.go", Line: 4},
				"SpelledLiteral": {File: "target/b.go", Line: 7},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(&tc.expect, &tc.manifest)
			if err != nil {
				t.Fatalf("resolveExpectations(%+v) error = %v, want nil", tc.expect.Expect, err)
			}
			if !maps.Equal(got, tc.want) {
				t.Errorf("resolveExpectations(%+v) = %v, want %v", tc.expect.Expect, got, tc.want)
			}
		})
	}
}

func TestResolveExpectationsRefuses(t *testing.T) {
	line := func(n int) *int { return &n }
	twoRows := Expectation{Expect: []Row{
		{Symbol: "DeadExport", Report: "DS1001", Confidence: "certain"},
		{Symbol: "UsedByConsumer", Report: "none"},
	}}
	cases := []struct {
		name       string
		wantErr    error
		wantSymbol string
		expect     Expectation
		manifest   Manifest
	}{
		{
			name:       "name_absent_from_manifest",
			expect:     twoRows,
			manifest:   Manifest{Symbols: map[string]ManifestPosition{"DeadExport": {File: "target/dead.go", Line: line(5)}}},
			wantErr:    ErrUnboundSymbol,
			wantSymbol: "UsedByConsumer",
		},
		{
			name:   "entry_without_line",
			expect: twoRows,
			manifest: Manifest{Symbols: map[string]ManifestPosition{
				"DeadExport":     {File: "target/dead.go", Line: line(5)},
				"UsedByConsumer": {File: "target/used.go"},
			}},
			wantErr:    ErrNoLine,
			wantSymbol: "UsedByConsumer",
		},
		{
			name:   "entry_with_line_zero",
			expect: twoRows,
			manifest: Manifest{Symbols: map[string]ManifestPosition{
				"DeadExport":     {File: "target/dead.go", Line: line(5)},
				"UsedByConsumer": {File: "target/used.go", Line: line(0)},
			}},
			wantErr:    ErrNoLine,
			wantSymbol: "UsedByConsumer",
		},
		{
			name:   "entry_without_file",
			expect: twoRows,
			manifest: Manifest{Symbols: map[string]ManifestPosition{
				"DeadExport":     {File: "target/dead.go", Line: line(5)},
				"UsedByConsumer": {Line: line(9)},
			}},
			wantErr:    ErrNoFile,
			wantSymbol: "UsedByConsumer",
		},
		{
			name:   "manifest_name_no_expectation_uses",
			expect: twoRows,
			manifest: Manifest{Symbols: map[string]ManifestPosition{
				"DeadExport":     {File: "target/dead.go", Line: line(5)},
				"UsedByConsumer": {File: "target/used.go", Line: line(9)},
				"Forgotten":      {File: "target/old.go", Line: line(1)},
			}},
			wantErr:    ErrOrphanSymbol,
			wantSymbol: "Forgotten",
		},
		{
			name: "configured_root_no_expectation_uses",
			expect: Expectation{
				ConfiguredRoots: map[string]string{"Forgotten": "go://example.com/app#forgotten*"},
				Expect:          []Row{{Symbol: "DeadExport", Report: "DS1001", Confidence: "certain"}},
			},
			manifest:   Manifest{Symbols: map[string]ManifestPosition{"DeadExport": {File: "target/dead.go", Line: line(5)}}},
			wantErr:    ErrOrphanRoot,
			wantSymbol: "Forgotten",
		},
		{
			name: "configured_root_the_manifest_binds_as_well",
			expect: Expectation{
				ConfiguredRoots: map[string]string{"AbsentPattern": "go://example.com/app#absent*"},
				Expect:          []Row{{Symbol: "AbsentPattern", Report: "DS9004", Confidence: "certain"}},
			},
			manifest:   Manifest{Symbols: map[string]ManifestPosition{"AbsentPattern": {File: "target/main.go", Line: line(3)}}},
			wantErr:    ErrRootAlsoBound,
			wantSymbol: "AbsentPattern",
		},
		{
			name: "configured_declaration_no_expectation_uses",
			expect: Expectation{
				ConfiguredDeclarations: map[string]ConfiguredDeclaration{"Forgotten": {Key: "ts.serializers", Entry: map[string]string{"global": "JSON.forgotten"}}},
				Expect:                 []Row{{Symbol: "DeadExport", Report: "DS1001", Confidence: "certain"}},
			},
			manifest:   Manifest{Symbols: map[string]ManifestPosition{"DeadExport": {File: "target/dead.ts", Line: line(5)}}},
			wantErr:    ErrOrphanEntry,
			wantSymbol: "Forgotten",
		},
		{
			name: "configured_declaration_the_manifest_binds_as_well",
			expect: Expectation{
				ConfiguredDeclarations: map[string]ConfiguredDeclaration{"AbsentEntry": {Key: "ts.serializers", Entry: map[string]string{"global": "JSON.absent"}}},
				Expect:                 []Row{{Symbol: "AbsentEntry", Report: "DS9006", Confidence: "certain"}},
			},
			manifest:   Manifest{Symbols: map[string]ManifestPosition{"AbsentEntry": {File: "target/main.ts", Line: line(3)}}},
			wantErr:    ErrEntryAlsoBound,
			wantSymbol: "AbsentEntry",
		},
		{
			name: "configured_declaration_configured_roots_names_as_well",
			expect: Expectation{
				ConfiguredRoots:        map[string]string{"AbsentEntry": "ts://@example/app/src/index.ts#absent*"},
				ConfiguredDeclarations: map[string]ConfiguredDeclaration{"AbsentEntry": {Key: "ts.serializers", Entry: map[string]string{"global": "JSON.absent"}}},
				Expect:                 []Row{{Symbol: "AbsentEntry", Report: "DS9006", Confidence: "certain"}},
			},
			manifest:   Manifest{Symbols: map[string]ManifestPosition{}},
			wantErr:    ErrEntryAlsoBound,
			wantSymbol: "AbsentEntry",
		},
		{
			name: "name_literal_absent_from_manifest",
			expect: Expectation{Expect: []Row{
				{Symbol: "Spelled", Report: "DS1002", Confidence: "possible", Details: &Details{NameLiteral: "SpelledLiteral"}},
			}},
			manifest:   Manifest{Symbols: map[string]ManifestPosition{"Spelled": {File: "target/a.go", Line: line(3)}}},
			wantErr:    ErrUnboundSymbol,
			wantSymbol: "SpelledLiteral",
		},
		{
			name: "name_literal_a_row_names_as_its_subject",
			expect: Expectation{Expect: []Row{
				{Symbol: "Spelled", Report: "DS1002", Confidence: "possible", Details: &Details{NameLiteral: "Spelled"}},
			}},
			manifest:   Manifest{Symbols: map[string]ManifestPosition{"Spelled": {File: "target/a.go", Line: line(3)}}},
			wantErr:    ErrDuplicateSymbol,
			wantSymbol: "Spelled",
		},
		{
			name: "name_literal_a_type_error_names_as_well",
			expect: Expectation{
				TypeErrorSkips: []string{"Broken"},
				Expect: []Row{
					{Symbol: "Spelled", Report: "DS1002", Confidence: "possible", Details: &Details{NameLiteral: "Broken"}},
				},
			},
			manifest: Manifest{Symbols: map[string]ManifestPosition{
				"Spelled": {File: "target/a.go", Line: line(3)},
				"Broken":  {File: "target/b.go", Line: line(5)},
			}},
			wantErr:    ErrDuplicateSymbol,
			wantSymbol: "Broken",
		},
		{
			name: "logical_name_expected_twice",
			expect: Expectation{Expect: []Row{
				{Symbol: "DeadExport", Report: "DS1001", Confidence: "certain"},
				{Symbol: "DeadExport", Report: "none"},
			}},
			manifest:   Manifest{Symbols: map[string]ManifestPosition{"DeadExport": {File: "target/dead.go", Line: line(5)}}},
			wantErr:    ErrDuplicateSymbol,
			wantSymbol: "DeadExport",
		},
		{
			name: "a_name_literal_at_a_reported_position",
			expect: Expectation{Expect: []Row{
				{Symbol: "Spelled", Report: "DS1002", Confidence: "possible", Details: &Details{NameLiteral: "SpelledLiteral"}},
			}},
			manifest: Manifest{Symbols: map[string]ManifestPosition{
				"Spelled":        {File: "target/a.go", Line: line(3)},
				"SpelledLiteral": {File: "target/a.go", Line: line(3)},
			}},
			wantErr:    ErrSharedPosition,
			wantSymbol: "SpelledLiteral",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(&tc.expect, &tc.manifest)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("resolveExpectations(%s) error = %v, want %v", tc.name, err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), fmt.Sprintf("%q", tc.wantSymbol)) {
				t.Errorf("resolveExpectations(%s) error = %q, want it to name %q", tc.name, err, tc.wantSymbol)
			}
			if got != nil {
				t.Errorf("resolveExpectations(%s) = %v, want nil positions beside the error", tc.name, got)
			}
		})
	}
}

func TestCheckConfigurations(t *testing.T) {
	matrix := []string{"linux-amd64", "linux-arm64"}
	rows := func(configurations ...[]string) Expectation {
		doc := Expectation{}
		for i, ids := range configurations {
			doc.Expect = append(doc.Expect, Row{
				Symbol:         "Subject" + strconv.Itoa(i),
				Report:         "DS1002",
				Configurations: ids,
			})
		}
		return doc
	}
	cases := []struct {
		wantErr error
		name    string
		expect  Expectation
		matrix  []string
	}{
		{
			name:   "every_configuration_of_the_matrix",
			expect: rows([]string{"linux-amd64", "linux-arm64"}),
			matrix: matrix,
		},
		{
			name:   "one_configuration_of_the_matrix",
			expect: rows([]string{"linux-arm64"}),
			matrix: matrix,
		},
		{
			name:   "no_matrix_and_no_expectation_names_one",
			expect: rows(nil),
		},
		{
			name:    "configuration_outside_the_matrix",
			expect:  rows([]string{"windows-amd64"}),
			matrix:  matrix,
			wantErr: ErrUnknownConfig,
		},
		{
			name:    "configurations_out_of_the_matrix_order",
			expect:  rows([]string{"linux-arm64", "linux-amd64"}),
			matrix:  matrix,
			wantErr: ErrConfigsOutOrder,
		},
		{
			name:    "one_configuration_named_twice",
			expect:  rows([]string{"linux-amd64", "linux-amd64"}),
			matrix:  matrix,
			wantErr: ErrConfigsOutOrder,
		},
		{
			name:    "no_matrix_declared",
			expect:  rows([]string{"linux-amd64"}),
			wantErr: ErrNoMatrix,
		},
		{
			name:    "matrix_declared_out_of_order",
			expect:  rows([]string{"linux-arm64"}),
			matrix:  []string{"linux-arm64", "linux-amd64"},
			wantErr: ErrMatrixUnordered,
		},
		{
			name:    "matrix_no_expectation_names",
			expect:  rows(nil),
			matrix:  matrix,
			wantErr: ErrMatrixUnused,
		},
		{
			name: "matrix_the_closed_world_declaration_configures",
			expect: func() Expectation {
				doc := rows(nil)
				doc.ClosedWorld = []string{WorldMatrix}
				return doc
			}(),
			matrix: matrix,
		},
		{
			name: "the_matrix_declared_complete_and_none_rendered",
			expect: func() Expectation {
				doc := rows(nil)
				doc.ClosedWorld = []string{WorldMatrix}
				return doc
			}(),
			wantErr: ErrMatrixUndeclared,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkConfigurations(&tc.expect, "go", tc.matrix)
			if tc.wantErr == nil {
				if len(got) != 0 {
					t.Errorf("checkConfigurations(%s) = %v, want nil", tc.name, got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("checkConfigurations(%s) = %v, want one error wrapping %v", tc.name, got, tc.wantErr)
			}
			if !errors.Is(got[0], tc.wantErr) {
				t.Errorf("checkConfigurations(%s) = %v, want it to wrap %v", tc.name, got[0], tc.wantErr)
			}
		})
	}
}

// plantedFixture is a complete two-language fixture in memory, laid out as
// the corpus lays one out on disk, so the walk is exercised before the first
// fixture lands and keeps being exercised on the same code afterwards.
func plantedFixture() fstest.MapFS {
	expect := `{"name":"planted","description":"A planted fixture.","languages":["go","ts"],"target_kind":"library","consumers":["consumer"],"expect":[{"symbol":"DeadExport","report":"DS1001","confidence":"certain"},{"symbol":"UsedByConsumer","report":"none"}]}`
	goArchive := "-- fixture.json --\n{\"symbols\":{\"DeadExport\":{\"file\":\"target/lib.go\",\"line\":3},\"UsedByConsumer\":{\"file\":\"target/lib.go\",\"line\":5}}}\n" +
		"-- target/go.mod --\nmodule example.test/target\n" +
		"-- target/lib.go --\npackage target\n\nfunc DeadExport() {}\n\nfunc UsedByConsumer() {}\n" +
		"-- consumer/go.mod --\nmodule example.test/consumer\n\nrequire example.test/target v0.0.0\n\nreplace example.test/target => ../target\n" +
		"-- consumer/main.go --\npackage main\n"
	return fstest.MapFS{
		"corpus/fixtures/planted/expect.json":              {Data: []byte(expect)},
		"corpus/fixtures/planted/go.txtar":                 {Data: []byte(goArchive)},
		"corpus/fixtures/planted/ts/fixture.json":          {Data: []byte(`{"symbols":{"DeadExport":{"file":"target/lib.ts","line":1},"UsedByConsumer":{"file":"target/lib.ts","line":2}}}`)},
		"corpus/fixtures/planted/ts/target/package.json":   {Data: []byte(`{"name":"target"}`)},
		"corpus/fixtures/planted/ts/target/lib.ts":         {Data: []byte("export const DeadExport = 1;\nexport const UsedByConsumer = 2;")},
		"corpus/fixtures/planted/ts/consumer/package.json": {Data: []byte(`{"name":"consumer"}`)},
	}
}

func TestCheckFixtureAcceptsAWellFormedFixture(t *testing.T) {
	if err := checkPlanted(plantedFixture()); err != nil {
		t.Errorf("checkFixture(planted) = %v, want nil", err)
	}
}

func TestCheckFixtureAcceptsAWorkspaceDependencyWithNoReplace(t *testing.T) {
	spellings := map[string][2]string{
		"dot_prefixed":   {"./target", "./dep"},
		"bare":           {"target", "dep"},
		"trailing_slash": {"./target/", "dep/"},
	}
	for name, uses := range spellings {
		t.Run(name, func(t *testing.T) {
			fsys := plantedFixture()
			archive := fsys["corpus/fixtures/planted/go.txtar"]
			archive.Data = bytes.Replace(archive.Data,
				[]byte("-- target/go.mod --\nmodule example.test/target\n"),
				[]byte("-- target/go.mod --\nmodule example.test/target\n\nrequire example.test/dep v0.0.0\n"), 1)
			archive.Data = append(archive.Data,
				"-- go.work --\ngo 1.27.0\n\nuse (\n\t"+uses[0]+"\n\t"+uses[1]+"\n)\n-- dep/go.mod --\nmodule example.test/dep\n"...)
			if err := checkPlanted(fsys); err != nil {
				t.Errorf("checkFixture(planted with a workspace dependency used as %q) = %v, want nil", uses, err)
			}
		})
	}
}

func TestCheckFixtureRefuses(t *testing.T) {
	cases := []struct {
		mutate  func(fstest.MapFS)
		name    string
		wantMsg string
	}{
		{
			name: "name_differs_from_directory",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/expect.json"].Data = bytes.Replace(m["corpus/fixtures/planted/expect.json"].Data, []byte(`"planted"`), []byte(`"other"`), 1)
			},
			wantMsg: `name = "other"`,
		},
		{
			name:    "listed_rendering_missing",
			mutate:  func(m fstest.MapFS) { delete(m, "corpus/fixtures/planted/go.txtar") },
			wantMsg: `rendering "go" listed but unreadable`,
		},
		{
			name: "unlisted_rendering_present",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/expect.json"].Data = bytes.Replace(m["corpus/fixtures/planted/expect.json"].Data, []byte(`["go","ts"]`), []byte(`["go"]`), 1)
			},
			wantMsg: `rendering "ts" present but not listed`,
		},
		{
			name: "manifest_missing_a_name",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/ts/fixture.json"].Data = []byte(`{"symbols":{"DeadExport":{"file":"target/lib.ts","line":1}}}`)
			},
			wantMsg: ErrUnboundSymbol.Error() + `: "UsedByConsumer"`,
		},
		{
			name: "manifest_binds_two_names_to_one_line",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/ts/fixture.json"].Data = bytes.Replace(m["corpus/fixtures/planted/ts/fixture.json"].Data,
					[]byte(`"line":2`), []byte(`"line":1`), 1)
			},
			wantMsg: ErrSharedPosition.Error(),
		},
		{
			name:    "manifest_names_a_file_the_rendering_lacks",
			mutate:  func(m fstest.MapFS) { delete(m, "corpus/fixtures/planted/ts/target/lib.ts") },
			wantMsg: `names file "target/lib.ts", which the rendering does not hold`,
		},
		{
			name: "manifest_line_past_end_of_file",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/ts/target/lib.ts"].Data = []byte("export const DeadExport = 1;\n")
			},
			wantMsg: `names target/lib.ts:2, want a line within the file's 1`,
		},
		{
			name:    "consumer_directory_missing",
			mutate:  func(m fstest.MapFS) { delete(m, "corpus/fixtures/planted/ts/consumer/package.json") },
			wantMsg: `rendering "ts" has no consumer/ directory`,
		},
		{
			name: "go_consumer_does_not_replace_the_target",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/go.txtar"].Data = bytes.Replace(m["corpus/fixtures/planted/go.txtar"].Data,
					[]byte("\nreplace example.test/target => ../target\n"), []byte("\n"), 1)
			},
			wantMsg: ErrNoReplace.Error() + `: consumer/go.mod replaces "example.test/target" with ""`,
		},
		{
			name: "go_consumer_does_not_require_the_target",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/go.txtar"].Data = bytes.Replace(m["corpus/fixtures/planted/go.txtar"].Data,
					[]byte("\nrequire example.test/target v0.0.0\n"), []byte("\n"), 1)
			},
			wantMsg: ErrNoRequire.Error() + `: consumer/go.mod names no require of "example.test/target"`,
		},
		{
			name: "go_dependency_the_target_does_not_replace",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/go.txtar"].Data = append(m["corpus/fixtures/planted/go.txtar"].Data,
					"-- dep/go.mod --\nmodule example.test/dep\n"...)
			},
			wantMsg: ErrNoDependencyReplace.Error() + `: target/go.mod replaces "example.test/dep" with ""`,
		},
		{
			name: "go_dependency_the_target_does_not_require",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/go.txtar"].Data = bytes.Replace(m["corpus/fixtures/planted/go.txtar"].Data,
					[]byte("-- target/go.mod --\nmodule example.test/target\n"),
					[]byte("-- target/go.mod --\nmodule example.test/target\n\nreplace example.test/dep v0.0.0 => ../dep\n"), 1)
				m["corpus/fixtures/planted/go.txtar"].Data = append(m["corpus/fixtures/planted/go.txtar"].Data,
					"-- dep/go.mod --\nmodule example.test/dep\n"...)
			},
			wantMsg: ErrNoDependencyRequire.Error() + `: target/go.mod names no require of "example.test/dep"`,
		},
		{
			name: "go_workspace_without_the_target",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/go.txtar"].Data = append(m["corpus/fixtures/planted/go.txtar"].Data,
					"-- go.work --\ngo 1.27.0\n\nuse ./dep\n-- dep/go.mod --\nmodule example.test/dep\n"...)
			},
			wantMsg: ErrWorkspaceNoTarget.Error() + `: go.work`,
		},
		{
			name: "go_workspace_without_the_target_still_needs_the_replace",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/go.txtar"].Data = append(m["corpus/fixtures/planted/go.txtar"].Data,
					"-- go.work --\ngo 1.27.0\n\nuse ./dep\n-- dep/go.mod --\nmodule example.test/dep\n"...)
			},
			wantMsg: ErrNoDependencyReplace.Error() + `: target/go.mod replaces "example.test/dep" with ""`,
		},
		{
			name: "go_workspace_lists_a_consumer",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/go.txtar"].Data = append(m["corpus/fixtures/planted/go.txtar"].Data,
					"-- go.work --\ngo 1.27.0\n\nuse (\n\t./target\n\t./consumer\n)\n"...)
			},
			wantMsg: ErrWorkspaceUse.Error() + `: go.work uses "./consumer"`,
		},
		{
			name: "go_workspace_uses_a_directory_below_the_target",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/go.txtar"].Data = append(m["corpus/fixtures/planted/go.txtar"].Data,
					"-- go.work --\ngo 1.27.0\n\nuse (\n\t./target\n\t./target/sub\n)\n"...)
			},
			wantMsg: ErrWorkspaceUse.Error() + `: go.work uses "./target/sub"`,
		},
		{
			name: "go_workspace_uses_an_absolute_path",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/go.txtar"].Data = append(m["corpus/fixtures/planted/go.txtar"].Data,
					"-- go.work --\ngo 1.27.0\n\nuse (\n\t./target\n\t/target\n)\n"...)
			},
			wantMsg: ErrWorkspaceUse.Error() + `: go.work uses "/target"`,
		},
		{
			name: "go_workspace_dependency_the_target_does_not_require",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/go.txtar"].Data = append(m["corpus/fixtures/planted/go.txtar"].Data,
					"-- go.work --\ngo 1.27.0\n\nuse (\n\t./target\n\t./dep\n)\n-- dep/go.mod --\nmodule example.test/dep\n"...)
			},
			wantMsg: ErrNoDependencyRequire.Error() + `: target/go.mod names no require of "example.test/dep"`,
		},
		{
			name: "consumer_tests_without_a_consumer",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/expect.json"].Data = bytes.Replace(m["corpus/fixtures/planted/expect.json"].Data, []byte(`"consumers":["consumer"],`), []byte(`"consumer_tests":"production",`), 1)
			},
			wantMsg: `holds consumer_tests and names no consumer`,
		},
		{
			name: "entry_files_in_a_two_language_fixture",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/expect.json"].Data = bytes.Replace(m["corpus/fixtures/planted/expect.json"].Data, []byte(`"expect":`), []byte(`"entry_files":["lib.ts"],"expect":`), 1)
			},
			wantMsg: `holds entry_files and lists the languages [go ts], want ts alone`,
		},
		{
			name: "disabled_conventions_in_a_two_language_fixture",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/expect.json"].Data = bytes.Replace(m["corpus/fixtures/planted/expect.json"].Data, []byte(`"expect":`), []byte(`"disabled_conventions":["next"],"expect":`), 1)
			},
			wantMsg: `holds disabled_conventions and lists the languages [go ts], want ts alone`,
		},
		{
			name: "setup_failure_beside_runner_keys",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/expect.json"].Data = []byte(`{"name":"planted","description":"A planted fixture.","languages":["ts"],"target_kind":"application",` +
					`"setup_failure":{"class":"missing-module","names":["lib"]},"entry_files":["lib.ts"],"disabled_conventions":["next"],"expect":[]}`)
				delete(m, "corpus/fixtures/planted/go.txtar")
			},
			wantMsg: `holds setup_failure and disabled_conventions, want no member that configures or answers a report` +
				"\n" + ExpectFile + ` holds setup_failure and entry_files, want no member that configures or answers a report`,
		},
		{
			name: "expectation_carries_an_undeclared_field",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/expect.json"].Data = bytes.Replace(m["corpus/fixtures/planted/expect.json"].Data, []byte(`"report":"none"`), []byte(`"report":"none","go":"x"`), 1)
			},
			wantMsg: `unknown field "go"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fsys := plantedFixture()
			tc.mutate(fsys)
			err := checkPlanted(fsys)
			if err == nil {
				t.Fatalf("checkFixture(%s) = nil, want an error containing %q", tc.name, tc.wantMsg)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("checkFixture(%s) = %q, want it to contain %q", tc.name, err, tc.wantMsg)
			}
		})
	}
}

func TestCheckConfigurationsNotBuilt(t *testing.T) {
	derived := Expectation{}
	complete := Expectation{ClosedWorld: []string{WorldMatrix}}
	built := []string{"linux-amd64"}
	cases := []struct {
		name    string
		expect  *Expectation
		dropped []string
		wantErr error
	}{
		{name: "absent", expect: &derived, dropped: nil},
		{name: "empty", expect: &derived, dropped: []string{}},
		{name: "bytewise_order", expect: &derived, dropped: []string{"linux-amd64-legacy", "linux-amd64-race"}},
		{name: "out_of_order", expect: &derived, dropped: []string{"linux-amd64-race", "linux-amd64-legacy"}, wantErr: ErrNotBuiltOrder},
		{name: "named_twice", expect: &derived, dropped: []string{"linux-amd64-legacy", "linux-amd64-legacy"}, wantErr: ErrNotBuiltOrder},
		{name: "dropped_and_built", expect: &derived, dropped: []string{"linux-amd64"}, wantErr: ErrNotBuiltBuilt},
		{name: "empty_under_a_complete_matrix", expect: &complete, dropped: []string{}},
		{name: "dropped_under_a_complete_matrix", expect: &complete, dropped: []string{"linux-amd64-legacy"}, wantErr: ErrNotBuiltDeclared},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkConfigurationsNotBuilt(tc.expect, "go", built, tc.dropped)
			if tc.wantErr == nil {
				if len(got) != 0 {
					t.Errorf("checkConfigurationsNotBuilt(%q) = %v, want nil", tc.dropped, got)
				}
				return
			}
			if len(got) != 1 || !errors.Is(got[0], tc.wantErr) {
				t.Errorf("checkConfigurationsNotBuilt(%q) = %v, want one error wrapping %v", tc.dropped, got, tc.wantErr)
			}
		})
	}
}

func TestCheckSetupFailureRefusesTheMembersThatConfigureARun(t *testing.T) {
	count := 0
	cases := []struct {
		plant  func(*Expectation)
		member string
	}{
		{member: "severity", plant: func(d *Expectation) { d.Severity = map[string]string{"DS1002": "allow"} }},
		{member: "suppressions_in_effect", plant: func(d *Expectation) { d.SuppressionsInEffect = &count }},
	}
	for _, tc := range cases {
		t.Run(tc.member, func(t *testing.T) {
			doc := Expectation{SetupFailure: &SetupFailure{Class: "missing-module", Names: []string{"example.com/dep"}}}
			if got := checkSetupFailure(&doc); len(got) != 0 {
				t.Fatalf("checkSetupFailure(setup failure alone) = %v, want nil so the plant below is the one cause", got)
			}
			tc.plant(&doc)
			got := checkSetupFailure(&doc)
			if len(got) != 1 || !strings.Contains(got[0].Error(), tc.member) {
				t.Errorf("checkSetupFailure(setup failure beside %s) = %v, want one error naming %s", tc.member, got, tc.member)
			}
		})
	}
}

func TestCorpusConfiguredRootsSpellTheFixturesOwnLanguage(t *testing.T) {
	planted := []struct {
		name      string
		languages []string
		roots     map[string]string
		wantMsg   string
	}{
		{
			name:      "one_language_and_its_own_prefix",
			languages: []string{"go"},
			roots:     map[string]string{"AbsentPattern": "go://example.com/app#absent*"},
		},
		{
			name:      "two_languages",
			languages: []string{"go", "ts"},
			roots:     map[string]string{"AbsentPattern": "go://example.com/app#absent*"},
			wantMsg:   "lists [go ts]",
		},
		{
			name:      "the_other_languages_prefix",
			languages: []string{"go"},
			roots:     map[string]string{"AbsentPattern": "ts://@example/app/src/index.ts#absent*"},
			wantMsg:   `want it to carry the fixture's own language prefix "go://"`,
		},
	}
	for _, tc := range planted {
		t.Run("planted_"+tc.name, func(t *testing.T) {
			doc := Expectation{Name: "planted", TargetKind: TargetKind{One: "application"}, Languages: tc.languages, ConfiguredRoots: tc.roots}
			got := checkConfiguredRootsScope(&doc)
			if tc.wantMsg == "" {
				if len(got) != 0 {
					t.Errorf("checkConfiguredRootsScope(planted %s) = %v, want nil", tc.name, got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("checkConfiguredRootsScope(planted %s) = %v, want one error", tc.name, got)
			}
			if !strings.Contains(got[0].Error(), tc.wantMsg) {
				t.Errorf("checkConfiguredRootsScope(planted %s) = %q, want it to name %s", tc.name, got[0], tc.wantMsg)
			}
		})
	}
}

func TestCorpusConfiguredDeclarationsSpellTheFixturesOwnLanguage(t *testing.T) {
	entry := map[string]ConfiguredDeclaration{"AbsentEntry": {Key: "ts.serializers", Entry: map[string]string{"global": "JSON.absent"}}}
	for _, tc := range []struct {
		name      string
		languages []string
		wantErr   bool
	}{
		{name: "the_keys_language_alone", languages: []string{"ts"}},
		{name: "two_languages", languages: []string{"go", "ts"}, wantErr: true},
		{name: "the_other_language", languages: []string{"go"}, wantErr: true},
	} {
		t.Run("planted_"+tc.name, func(t *testing.T) {
			doc := Expectation{Name: "planted", Languages: tc.languages, ConfiguredDeclarations: entry}
			if got := checkConfiguredDeclarationsScope(&doc); (len(got) != 0) != tc.wantErr {
				t.Errorf("checkConfiguredDeclarationsScope(planted %s) = %v, want an error %t", tc.name, got, tc.wantErr)
			}
		})
	}
}

func TestCorpusRowsNameASubjectKindEveryListedLanguageDeclares(t *testing.T) {
	kindLanguages, err := subjectKindLanguages()
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	planted := []struct {
		name      string
		kind      string
		wantMsg   string
		languages []string
	}{
		{name: "a_go_subject_in_a_go_fixture", kind: "satisfaction-assertion", languages: []string{"go"}},
		{name: "a_go_subject_in_a_two_language_fixture", kind: "satisfaction-assertion", languages: []string{"go", "ts"}, wantMsg: `"satisfaction-assertion", a subject only go declares, and the fixture lists [go ts]`},
		{name: "a_go_subject_in_a_typescript_fixture", kind: "receiver", languages: []string{"ts"}, wantMsg: `"receiver", a subject only go declares`},
		{name: "a_typescript_subject_in_a_two_language_fixture", kind: "class", languages: []string{"go", "ts"}, wantMsg: `"class", a subject only ts declares`},
		{name: "a_subject_of_either_language_in_a_two_language_fixture", kind: "function", languages: []string{"go", "ts"}},
		{name: "a_subject_both_languages_name_in_a_two_language_fixture", kind: "enum-member", languages: []string{"go", "ts"}},
	}
	for _, tc := range planted {
		t.Run("planted_"+tc.name, func(t *testing.T) {
			doc := Expectation{
				Name:       "planted",
				TargetKind: TargetKind{One: "application"},
				Languages:  tc.languages,
				Expect: []Row{
					{Symbol: "Subject", Report: "DS1002", SymbolKind: tc.kind, Confidence: "certain"},
				},
			}
			got := checkSubjectKindLanguageScope(&doc, kindLanguages)
			if tc.wantMsg == "" {
				if len(got) != 0 {
					t.Errorf("checkSubjectKindLanguageScope(planted %s) = %v, want nil", tc.name, got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("checkSubjectKindLanguageScope(planted %s) = %v, want one error", tc.name, got)
			}
			if !strings.Contains(got[0].Error(), tc.wantMsg) {
				t.Errorf("checkSubjectKindLanguageScope(planted %s) = %q, want it to name %s", tc.name, got[0], tc.wantMsg)
			}
		})
	}
}

func TestCorpusDetailsPinNoValueAnotherRenderingCannotCarry(t *testing.T) {
	// One planted document per per-language member, in a two-language fixture,
	// so a member that stopped being scoped reports here.
	for _, member := range perLanguageDetails {
		t.Run("planted_"+member, func(t *testing.T) {
			details := &Details{}
			switch member {
			case "dependency_class":
				details.DependencyClass = "require"
			case "excluded_by":
				details.ExcludedBy = "handwritten"
			case "overlap":
				details.Overlap = []string{"staticcheck U1000"}
			case "replacement":
				details.Replacement = "../local"
			}
			doc := Expectation{
				Name:       "planted",
				TargetKind: TargetKind{One: "library"},
				Languages:  []string{"go", "ts"},
				Expect: []Row{
					{Symbol: "Subject", Report: "DS1001", Confidence: "certain", Details: details},
				},
			}
			got := checkDetailsLanguageScope(&doc)
			if len(got) != 1 {
				t.Fatalf("checkDetailsLanguageScope(planted %s) = %v, want one error", member, got)
			}
			if !strings.Contains(got[0].Error(), "details."+member) {
				t.Errorf("checkDetailsLanguageScope(planted %s) = %q, want it to name details.%s", member, got[0], member)
			}
		})
	}
}

func TestCorpusComponentCountsPinNoValueAnotherRenderingCannotCarry(t *testing.T) {
	root, count, lines := true, 4, 7
	planted := []struct {
		component Component
		name      string
		wantMsg   string
		languages []string
	}{
		{name: "the_counts_in_a_one_language_fixture", component: Component{SymbolCount: &count, DeletableLines: &lines}, languages: []string{"go"}},
		{name: "the_root_flag_in_a_two_language_fixture", component: Component{Root: &root}, languages: []string{"go", "ts"}},
		{name: "deletable_lines_in_a_two_language_fixture", component: Component{DeletableLines: &lines}, languages: []string{"go", "ts"}, wantMsg: "component.deletable_lines"},
		{name: "symbol_count_in_a_two_language_fixture", component: Component{SymbolCount: &count}, languages: []string{"go", "ts"}, wantMsg: "component.symbol_count"},
	}
	for _, tc := range planted {
		t.Run("planted_"+tc.name, func(t *testing.T) {
			doc := Expectation{
				Name:       "planted",
				TargetKind: TargetKind{One: "application"},
				Languages:  tc.languages,
				Expect: []Row{
					{Symbol: "Ledger", Report: "DS1002", Confidence: "certain", Component: &tc.component},
				},
			}
			got := checkComponentLanguageScope(&doc)
			if tc.wantMsg == "" {
				if len(got) != 0 {
					t.Errorf("checkComponentLanguageScope(planted %s) = %v, want nil", tc.name, got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("checkComponentLanguageScope(planted %s) = %v, want one error", tc.name, got)
			}
			if !strings.Contains(got[0].Error(), tc.wantMsg) {
				t.Errorf("checkComponentLanguageScope(planted %s) = %q, want it to name %s", tc.name, got[0], tc.wantMsg)
			}
		})
	}
}

// TestCorpusEdgeEvaluationsNameEveryDeclaredSideOfTheRenderingsLanguage holds the
// edge_evaluations member to the edges document of its rendering, and plants one
// departure per rule into a copy of the fixture that names the member.
func TestCorpusEdgeEvaluationsNameEveryDeclaredSideOfTheRenderingsLanguage(t *testing.T) {
	const fixture = "part-finding-never-pending"
	doc0, err := ReadDocument(spec.Corpus)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	read, err := LoadFixture(spec.Corpus, doc0, fixture)
	if err != nil {
		t.Fatalf("Setup: LoadFixture(%s): %v", fixture, err)
	}
	files := read.Renderings["go"].Files
	load := func() Expectation {
		held, err := LoadFixture(spec.Corpus, doc0, fixture)
		if err != nil {
			t.Fatalf("Setup: LoadFixture(%s): %v", fixture, err)
		}
		return held.Expect
	}
	doc := load()
	if got := checkEdgeEvaluations(&doc, "go", files); len(got) != 0 {
		t.Fatalf("checkEdgeEvaluations(%s) = %v, want none", fixture, got)
	}
	withoutDocument := maps.Clone(files)
	delete(withoutDocument, edgesDocumentPath)
	tests := []struct {
		plant func(*Expectation)
		files map[string][]byte
		name  string
	}{
		{name: "a_side_left_out", plant: func(d *Expectation) { d.EdgeEvaluations = d.EdgeEvaluations[1:] }},
		{name: "a_side_named_twice", plant: func(d *Expectation) {
			d.EdgeEvaluations = append(d.EdgeEvaluations, d.EdgeEvaluations[0])
		}},
		{name: "the_other_languages_side", plant: func(d *Expectation) { d.EdgeEvaluations[0].Side = "used_by" }},
		{name: "an_undeclared_edge", plant: func(d *Expectation) { d.EdgeEvaluations[0].Edge = "wire/undeclared" }},
		{name: "two_languages", plant: func(d *Expectation) { d.Languages = []string{"go", "ts"} }},
		{name: "no_edges_document", plant: func(*Expectation) {}, files: withoutDocument},
		{name: "no_member_beside_the_document", plant: func(d *Expectation) { d.EdgeEvaluations = nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			planted := load()
			tc.plant(&planted)
			held := files
			if tc.files != nil {
				held = tc.files
			}
			if got := checkEdgeEvaluations(&planted, "go", held); len(got) == 0 {
				t.Errorf("checkEdgeEvaluations(%s with %s) = none, want the departure named", fixture, tc.name)
			}
		})
	}
}

// TestCorpusInstalledPackagesNameTheirPath plants a manifest whose name departs from
// its path, and an installed tree in a Go rendering, and requires both refused.
func TestCorpusInstalledPackagesNameTheirPath(t *testing.T) {
	doc, err := ReadDocument(spec.Corpus)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	read, err := LoadFixture(spec.Corpus, doc, "dependency-commands-and-peers")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	files := read.Renderings["ts"].Files
	if got := checkInstalledPackages("ts", files); len(got) != 0 {
		t.Fatalf("checkInstalledPackages(dependency-commands-and-peers) = %v, want none", got)
	}
	renamed := maps.Clone(files)
	renamed["installed/used-runtime/package.json"] = []byte(`{"name": "used-runtime-2", "private": true}`)
	if got := checkInstalledPackages("ts", renamed); len(got) == 0 {
		t.Errorf("checkInstalledPackages(a manifest naming another package) = none, want it refused")
	}
	if got := checkInstalledPackages("go", files); len(got) == 0 {
		t.Errorf("checkInstalledPackages(go, an installed tree) = none, want it refused")
	}
	inTarget := map[string][]byte{"target/installed/@example/tool/package.json": []byte(`{"name": "@example/tool", "private": true}`)}
	if got := checkInstalledPackages("ts", inTarget); len(got) != 0 {
		t.Errorf("checkInstalledPackages(ts, a tree inside the target) = %v, want none", got)
	}
	inTarget["target/installed/@example/tool/package.json"] = []byte(`{"name": "@example/other", "private": true}`)
	if got := checkInstalledPackages("ts", inTarget); len(got) == 0 {
		t.Errorf("checkInstalledPackages(ts, a manifest inside the target naming another package) = none, want it refused")
	}
	if got := checkInstalledPackages("go", map[string][]byte{"target/installed/tool/package.json": []byte(`{"name": "tool", "private": true}`)}); len(got) == 0 {
		t.Errorf("checkInstalledPackages(go, a tree inside the target) = none, want it refused")
	}
}

// TestPinnableDetailsFollowsTheKindsAndTheLanguageList pins the rule the check
// applies on the two cases a fixture cannot state: a member a kind carries in one
// language the fixture does not list, and a member whose value one language
// spells in a fixture that lists several.
func TestPinnableDetailsFollowsTheKindsAndTheLanguageList(t *testing.T) {
	schema, err := readSchema(spec.Corpus, ExpectSchemaPath)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	admitted := slices.Sorted(maps.Keys(objectAt(schema, "properties/expect/items/properties/details/properties")))
	cases := []struct {
		name      string
		code      string
		languages []string
		want      []string
	}{
		{name: "a_go_only_never_built_row_pins_the_constraint", code: "DS1601", languages: []string{"go"}, want: []string{"excluded_by"}},
		{name: "a_typescript_never_built_row_pins_nothing", code: "DS1601", languages: []string{"ts"}, want: nil},
		{name: "a_two_language_never_built_row_pins_nothing", code: "DS1601", languages: []string{"go", "ts"}, want: nil},
		{name: "a_narrowing_row_pins_its_visibility_in_either_language", code: "DS1501", languages: []string{"go", "ts"}, want: []string{"narrower_visibility"}},
		{name: "a_two_language_dependency_row_pins_nothing", code: "DS1701", languages: []string{"go", "ts"}, want: nil},
		{name: "a_go_only_dependency_row_pins_its_section", code: "DS1701", languages: []string{"go"}, want: []string{"dependency_class"}},
		{name: "an_interface_row_pins_nothing_the_row_can_state", code: "DS1201", languages: []string{"go"}, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := PinnableDetails(tc.code, tc.languages, admitted); !slices.Equal(got, tc.want) {
				t.Errorf("PinnableDetails(%s, %v) = %v, want %v", tc.code, tc.languages, got, tc.want)
			}
		})
	}
}
