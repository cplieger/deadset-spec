package spec_test

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// pathMember is one schema member that carries a path, and whether it may name
// the target root itself as a single dot.
type pathMember struct {
	schema string
	at     string
	dot    bool
}

// pathMembers are every member of the embedded schemas whose value is a path.
var pathMembers = []pathMember{
	{schema: findingSchemaPath, at: "/" + findingPathDefPath + "/pattern"},
	{schema: configSchemaPath, at: "/properties/analysis/properties/configurations/items/oneOf/1/properties/project/pattern"},
	{schema: reportSchemaPath, at: "/properties/target/properties/root/pattern", dot: true},
	{schema: reportSchemaPath, at: "/" + reportConfigurationProjectPath + "/properties/project/pattern"},
	{schema: reportSchemaPath, at: "/" + reportNotBuiltProjectPath + "/properties/project/pattern"},
	{schema: reportSchemaPath, at: "/" + reportStaleItemPath + "/properties/entry/properties/path/pattern"},
	{schema: reportSchemaPath, at: "/" + reportStaleItemPath + "/properties/position/properties/path/pattern"},
	{schema: reportSchemaPath, at: "/properties/excluded_by_cgo/items/pattern"},
	{schema: reportSchemaPath, at: "/properties/type_error_skips/items/properties/path/pattern"},
	{schema: reportSchemaPath, at: "/properties/notes/items/properties/path/pattern", dot: true},
	{schema: reportSchemaPath, at: "/properties/conventions_applied/items/properties/manifest/pattern"},
	{schema: "corpus/conformance-results.schema.json", at: "/properties/fixtures/items/properties/unexpected/items/properties/file/pattern"},
	{schema: "corpus/expect.schema.json", at: "/properties/notes/items/properties/path/pattern", dot: true},
	{schema: "corpus/expect.schema.json", at: "/properties/conventions_applied/items/properties/manifest/pattern"},
	{schema: "corpus/fixture.schema.json", at: "/properties/symbols/additionalProperties/properties/file/pattern"},
}

// pathMemberNames are the member names that hold a path wherever a schema
// declares them; a pattern under one of them that pathMembers does not list is
// a path spelled outside the path form.
var pathMemberNames = []string{"excluded_by_cgo", "file", "manifest", "path", "project", "relative_path", "root"}

// pathForms returns the path form finding.schema.json declares and the form
// that also admits the single dot naming the target root.
func pathForms(t *testing.T) (form, dotForm string) {
	t.Helper()
	form, _ = objectAt(loadFindingSchema(t), findingPathDefPath)["pattern"].(string)
	if !strings.HasPrefix(form, "^") || !strings.HasSuffix(form, "$") {
		t.Fatalf("Setup: %s declares %q at %s, want an anchored path form", findingSchemaPath, form, findingPathDefPath)
	}
	return form, `^(?:\.|` + form[1:len(form)-1] + `)$`
}

// memberName is the name of the member a pattern location constrains: the
// property that holds it, or the array whose items it constrains.
func memberName(at string) string {
	keys := strings.Split(strings.TrimSuffix(at, "/pattern"), "/")
	name := keys[len(keys)-1]
	if name == "items" && len(keys) > 1 {
		name = keys[len(keys)-2]
	}
	return name
}

func TestEveryPathMemberIsSpelledInThePathForm(t *testing.T) {
	form, dotForm := pathForms(t)
	documents, err := embeddedSchemas()
	if err != nil {
		t.Fatalf("Setup: embeddedSchemas(): %v", err)
	}
	all, err := schemaPatterns(documents)
	if err != nil {
		t.Fatalf("Setup: schemaPatterns(): %v", err)
	}
	listed := map[[2]string]pathMember{}
	for _, m := range pathMembers {
		listed[[2]string{m.schema, m.at}] = m
	}
	seen := map[[2]string]bool{}
	for _, p := range all {
		key := [2]string{p.schema, p.at}
		m, ok := listed[key]
		if !ok {
			if slices.Contains(pathMemberNames, memberName(p.at)) || p.expr == form || p.expr == dotForm {
				t.Errorf("%s%s carries %q and is no listed path member, want it listed and spelled in the path form", p.schema, p.at, p.expr)
			}
			continue
		}
		seen[key] = true
		want := form
		if m.dot {
			want = dotForm
		}
		if p.expr != want {
			t.Errorf("%s%s = %q, want %q, the path form %s declares at %s", p.schema, p.at, p.expr, want, findingSchemaPath, findingPathDefPath)
		}
	}
	for _, m := range pathMembers {
		if !seen[[2]string{m.schema, m.at}] {
			t.Errorf("%s%s declares no pattern, want the path form", m.schema, m.at)
		}
	}
}

// Route files of SvelteKit, Next.js, Remix and TanStack name their segments
// with characters a narrower form would refuse, so the path form must accept
// them, as it accepts a backslash, which is a character of a name; the refused
// paths step out of the target, are not normalized, or carry a control character
// a text line would hand to a terminal.
var (
	pathFormAccepted = []string{
		"main.go",
		"src/routes/(app)/+layout.svelte",
		"dashboard/src/routes/+page.server.ts",
		"app/$id.tsx",
		"pages/api/auth/[...nextauth].ts",
		"src/routes/__root.tsx",
		"src/lib/date utils/format.ts",
		"internal/catalog/résumé.go",
		".github/workflows/ci.yaml",
		"...",
		"..rc",
		"a:b%20c#d",
		`internal\legacy\catalog.go`,
	}
	pathFormRefused = []string{
		"",
		".",
		"..",
		"a/../b",
		"a/./b",
		"./x",
		"a//b",
		"a/",
		"/abs",
		"a\x00b",
		"a\tb",
		"a\nb",
		"a\x1b[31mb",
		"a\x1fb",
		"a\x7fb",
	}
)

func TestPathFormAcceptsRouteFileNamesAndRefusesWhatLeavesTheForm(t *testing.T) {
	documents, err := embeddedSchemas()
	if err != nil {
		t.Fatalf("Setup: embeddedSchemas(): %v", err)
	}
	for _, m := range pathMembers {
		var root any
		if err := json.Unmarshal(documents[m.schema], &root); err != nil {
			t.Fatalf("Setup: decode %s: %v", m.schema, err)
		}
		expr := patterns(root, "")[m.at]
		re, err := regexp.Compile(expr)
		if err != nil {
			t.Fatalf("Setup: regexp.Compile(%s%s): %v", m.schema, m.at, err)
		}
		for _, p := range pathFormAccepted {
			if !re.MatchString(p) {
				t.Errorf("%s%s refuses %q, want it accepted", m.schema, m.at, p)
			}
		}
		for _, p := range pathFormRefused {
			want := p == "." && m.dot
			if got := re.MatchString(p); got != want {
				t.Errorf("%s%s.MatchString(%q) = %t, want %t", m.schema, m.at, p, got, want)
			}
		}
	}
}

// setupRecordsWith returns the setup-records report example with one member
// of its first note and of its first type error skip replaced.
func setupRecordsWith(t *testing.T, notePath, skipPath string) []byte {
	t.Helper()
	var report map[string]any
	if err := json.Unmarshal(readExample(t, "reports", "setup-records.json"), &report); err != nil {
		t.Fatalf("Setup: decode setup-records.json: %v", err)
	}
	notes, _ := report["notes"].([]any)
	skips, _ := report["type_error_skips"].([]any)
	if len(notes) == 0 || len(skips) == 0 {
		t.Fatalf("Setup: setup-records.json holds %d notes and %d type error skips, want one of each", len(notes), len(skips))
	}
	notes[0].(map[string]any)["path"] = notePath
	skips[0].(map[string]any)["path"] = skipPath
	out, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Setup: encode the report: %v", err)
	}
	return out
}

// TestReportNamesTheTargetRootOfANoteAsADot pins the one place a record path
// names the root: a note about the package at the target root names it as a
// single dot, a source file path never does, and no other spelling of the root
// is a note's path.
func TestReportNamesTheTargetRootOfANoteAsADot(t *testing.T) {
	const routeFile = "src/routes/(app)/+layout.svelte"
	cases := []struct {
		name     string
		notePath string
		skipPath string
		refused  string
	}{
		{name: "root_note_and_route_file", notePath: ".", skipPath: routeFile},
		{name: "nested_note", notePath: "cmd/(gen)/client", skipPath: routeFile},
		{name: "root_note_spelled_dot_slash", notePath: "./", skipPath: routeFile, refused: "/notes/0/path"},
		{name: "root_note_spelled_empty", notePath: "", skipPath: routeFile, refused: "/notes/0/path"},
		{name: "skip_names_the_root", notePath: ".", skipPath: ".", refused: "/type_error_skips/0/path"},
		{name: "skip_steps_out", notePath: ".", skipPath: "src/../../x.ts", refused: "/type_error_skips/0/path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			problems, err := validationProblems(reportSchemaPath, setupRecordsWith(t, tc.notePath, tc.skipPath))
			if err != nil {
				t.Fatalf("Setup: validationProblems(%s): %v", reportSchemaPath, err)
			}
			if tc.refused == "" {
				if len(problems) != 0 {
					t.Errorf("validationProblems(note %q, skip %q) = %v, want none", tc.notePath, tc.skipPath, problems)
				}
				return
			}
			if len(problems) != 1 || problems[0].InstanceLocation != tc.refused || !problems[0].names("pattern") {
				t.Errorf("validationProblems(note %q, skip %q) = %v, want one pattern problem at %s", tc.notePath, tc.skipPath, problems, tc.refused)
			}
		})
	}
}
