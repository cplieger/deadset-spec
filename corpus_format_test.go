package spec_test

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	spec "github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/corpus"
)

const (
	corpusPath        = corpus.DocumentPath
	expectSchemaPath  = corpus.ExpectSchemaPath
	fixtureSchemaPath = corpus.ManifestSchemaPath
	fixturesDir       = corpus.FixturesDir

	// corpusBaselinePath is the committed record of the fixture set and the
	// expectation-row members at the published corpus version, which is what
	// makes the version's own rule enforceable.
	corpusBaselinePath = "testdata/corpus-baseline.json"

	// expectSchemaRowPath is the path into the expectation schema of one
	// expectation row.
	expectSchemaRowPath = "properties/expect/items"

	// expectSchemaDetailsPath is the path of the details object a row pins,
	// whose members mirror the details members contract/finding.schema.json
	// declares.
	expectSchemaDetailsPath = expectSchemaRowPath + "/properties/details"

	// resultsSchemaActualPath is the path into the results schema of the answer
	// a runner records for one expectation, which echoes the row's own members.
	resultsSchemaActualPath = "properties/fixtures/items/properties/expectations/items/properties/actual"
)

var (
	semverPattern = regexp.MustCompile(`^([0-9]+)\.[0-9]+\.[0-9]+$`)

	// languageWords are the tokens that would bind an expectation to one
	// language, in any casing, as a property name or as an enumerated value.
	languageWords = []string{"go", "golang", "ts", "typescript", "javascript", "language", "languages"}

	// vocabularyFields are the expectation-file fields corpus.json must name
	// a closed vocabulary for.
	vocabularyFields = []string{"confidence", "details", "fixability", "languages", "notes", "reachability_class", "report", "retained_by", "setup_failure", "symbol_kind"}
)

// loadCorpus decodes corpus/corpus.json, failing the test on any setup error.
func loadCorpus(t *testing.T) *corpus.Document {
	t.Helper()
	doc, err := corpus.ReadDocument(spec.Corpus)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	return doc
}

// loadSchema decodes an embedded schema into the generic tree the structural
// checks walk.
func loadSchema(t *testing.T, fsys fs.FS, p string) map[string]any {
	t.Helper()
	data, err := fs.ReadFile(fsys, p)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(%q): %v", p, err)
	}
	var schema map[string]any
	if err = json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("Setup: json.Unmarshal(%q): %v", p, err)
	}
	return schema
}

// languageBindings lists, as paths into node, every property name and every
// enumerated or constant value that is a language word.
func languageBindings(node any, at string) []string {
	var hits []string
	switch n := node.(type) {
	case map[string]any:
		if props, ok := n["properties"].(map[string]any); ok {
			for name := range props {
				if isLanguageWord(name) {
					hits = append(hits, at+"/properties/"+name)
				}
			}
		}
		if values, ok := n["enum"].([]any); ok {
			for _, v := range values {
				if s, ok := v.(string); ok && isLanguageWord(s) {
					hits = append(hits, at+"/enum/"+s)
				}
			}
		}
		if s, ok := n["const"].(string); ok && isLanguageWord(s) {
			hits = append(hits, at+"/const/"+s)
		}
		for key, child := range n {
			hits = append(hits, languageBindings(child, at+"/"+key)...)
		}
	case []any:
		for i, child := range n {
			hits = append(hits, languageBindings(child, fmt.Sprintf("%s/%d", at, i))...)
		}
	}
	slices.Sort(hits)
	return hits
}

func isLanguageWord(s string) bool {
	return slices.Contains(languageWords, strings.ToLower(s))
}

// openObjects lists, as paths into node, every object schema that does not
// close its key set: additionalProperties absent or true. A schema-valued
// additionalProperties is a closed table keyed by a pattern and passes.
func openObjects(node any, at string) []string {
	var hits []string
	switch n := node.(type) {
	case map[string]any:
		if n["type"] == "object" {
			extra, present := n["additionalProperties"]
			if !present || extra == true {
				hits = append(hits, at)
			}
		}
		for key, child := range n {
			hits = append(hits, openObjects(child, at+"/"+key)...)
		}
	case []any:
		for i, child := range n {
			hits = append(hits, openObjects(child, fmt.Sprintf("%s/%d", at, i))...)
		}
	}
	slices.Sort(hits)
	return hits
}

// undescribedProperties lists, as paths into node, every declared property
// carrying a type but no non-empty description.
func undescribedProperties(node any, at string) []string {
	var hits []string
	switch n := node.(type) {
	case map[string]any:
		if props, ok := n["properties"].(map[string]any); ok {
			for name, sub := range props {
				schema, ok := sub.(map[string]any)
				if !ok || schema["type"] == nil {
					continue
				}
				if desc, _ := schema["description"].(string); desc == "" {
					hits = append(hits, at+"/properties/"+name)
				}
			}
		}
		for key, child := range n {
			hits = append(hits, undescribedProperties(child, at+"/"+key)...)
		}
	case []any:
		for i, child := range n {
			hits = append(hits, undescribedProperties(child, fmt.Sprintf("%s/%d", at, i))...)
		}
	}
	slices.Sort(hits)
	return hits
}

// patterns lists every regular expression the schema declares, by path.
func patterns(node any, at string) map[string]string {
	found := map[string]string{}
	switch n := node.(type) {
	case map[string]any:
		if p, ok := n["pattern"].(string); ok {
			found[at+"/pattern"] = p
		}
		for key, child := range n {
			maps.Copy(found, patterns(child, at+"/"+key))
		}
	case []any:
		for i, child := range n {
			maps.Copy(found, patterns(child, fmt.Sprintf("%s/%d", at, i)))
		}
	}
	return found
}

// objectAt returns the object at a slash-separated path into schema, or nil
// when the path leaves the tree or ends on a non-object. A component that is a
// decimal number indexes an array, which is how a path reaches one branch of a
// schema stating two shapes under oneOf.
func objectAt(schema map[string]any, p string) map[string]any {
	var node any = schema
	for key := range strings.SplitSeq(p, "/") {
		switch container := node.(type) {
		case map[string]any:
			node = container[key]
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(container) {
				return nil
			}
			node = container[i]
		default:
			return nil
		}
	}
	m, _ := node.(map[string]any)
	return m
}

// enumAt returns the enum of the object at a slash-separated path into
// schema, or nil.
func enumAt(schema map[string]any, p string) []string {
	values, ok := objectAt(schema, p)["enum"].([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func TestCorpusDocument(t *testing.T) {
	doc := loadCorpus(t)
	kinds := loadKinds(t)

	t.Run("version", func(t *testing.T) {
		m := semverPattern.FindStringSubmatch(doc.CorpusVersion)
		if m == nil || m[1] == "0" {
			t.Errorf("corpus_version = %q, want a semantic version with a major of at least 1", doc.CorpusVersion)
		}
		if doc.Description == "" {
			t.Errorf("description = %q, want the version rule stated", doc.Description)
		}
	})

	t.Run("layout", func(t *testing.T) {
		want := map[string]string{"go": "go.txtar", "ts": "ts/"}
		if !maps.Equal(doc.Layout.Renderings, want) {
			t.Errorf("layout.renderings = %v, want %v, the layout this suite walks", doc.Layout.Renderings, want)
		}
		if got := slices.Sorted(maps.Keys(doc.Layout.Renderings)); !slices.Equal(got, kinds.Languages) {
			t.Errorf("layout.renderings languages = %v, want %s languages %v", got, kindsPath, kinds.Languages)
		}
		gotPaths := []string{doc.Layout.FixtureDirectory, doc.Layout.ExpectationFile, doc.Layout.ManifestFile, doc.Layout.TargetDirectory, doc.Layout.ConsumerDirectory, doc.Layout.DependencyDirectory, doc.Layout.InstalledDirectory}
		wantPaths := []string{fixturesDir + "/<name>/", corpus.ExpectFile, corpus.ManifestFile, corpus.TargetDir + "/", "<consumer>/", "<dependency>/", corpus.InstalledDir + "/"}
		if !slices.Equal(gotPaths, wantPaths) {
			t.Errorf("layout paths = %q, want %q", gotPaths, wantPaths)
		}
	})

	t.Run("schemas", func(t *testing.T) {
		got := []string{doc.Schemas.ExpectationFile, doc.Schemas.ManifestFile, doc.Schemas.DeclaredGaps, doc.Schemas.Results}
		for _, p := range got {
			if _, err := fs.Stat(spec.Corpus, p); err != nil {
				t.Errorf("fs.Stat(Corpus, %q) = %v, want the schema corpus.json names present", p, err)
			}
		}
		want := []string{expectSchemaPath, fixtureSchemaPath, conformanceSchemaPath, resultsSchemaPath}
		if !slices.Equal(got, want) {
			t.Errorf("schemas = %q, want %q", got, want)
		}
	})

	t.Run("vocabularies", func(t *testing.T) {
		if got := slices.Sorted(maps.Keys(doc.Vocabularies.Fields)); !slices.Equal(got, vocabularyFields) {
			t.Errorf("vocabularies.fields keys = %v, want %v", got, vocabularyFields)
		}
		for name, v := range doc.Vocabularies.Fields {
			if _, err := fs.Stat(spec.Contract, v.File); err != nil {
				t.Errorf("vocabularies.fields[%q].file = %q: fs.Stat(Contract) = %v, want a contract file", name, v.File, err)
			}
			if v.Field == "" || v.Description == "" {
				t.Errorf("vocabularies.fields[%q] = %+v, want field and description present", name, v)
			}
			wantAlso := []string(nil)
			if name == "report" {
				wantAlso = []string{"none"}
			}
			if !slices.Equal(v.Also, wantAlso) {
				t.Errorf("vocabularies.fields[%q].also = %v, want %v", name, v.Also, wantAlso)
			}
		}
	})

	t.Run("subject_shapes", func(t *testing.T) {
		shapes := doc.SubjectShapes
		if shapes.Description == "" {
			t.Errorf("subject_shapes.description = %q, want the four shapes stated", shapes.Description)
		}
		vocabulary := enumAt(loadFindingSchema(t), "properties/symbol/properties/kind")
		if len(vocabulary) == 0 {
			t.Fatalf("Setup: %s declares no symbol.kind vocabulary", findingSchemaPath)
		}
		named := map[string]string{}
		for shape, kinds := range map[string][]string{"part": shapes.Part, "dependency": shapes.Dependency, "row": shapes.Row} {
			if len(kinds) == 0 {
				t.Errorf("subject_shapes.%s = %v, want the kinds of that shape", shape, kinds)
			}
			if !slices.IsSorted(kinds) {
				t.Errorf("subject_shapes.%s = %v, want the kinds in order", shape, kinds)
			}
			for _, kind := range kinds {
				if !slices.Contains(vocabulary, kind) {
					t.Errorf("subject_shapes.%s names %q, want a kind of the %s vocabulary %v", shape, kind, findingSchemaPath, vocabulary)
				}
				if other, twice := named[kind]; twice {
					t.Errorf("subject_shapes names %q under %s and under %s, want one shape per kind", kind, other, shape)
				}
				named[kind] = shape
			}
		}
		// The block's own claim: every kind the finding schema judges by no
		// liveness relation is a part or a row here, so a kind that list gains
		// is classified rather than silently read as a declaration.
		for _, kind := range findingRelationAbsentKinds(t) {
			if named[kind] == "" {
				t.Errorf("%s lists %q as carrying no liveness relation and subject_shapes names it under none of part, dependency and row", findingSchemaPath, kind)
			}
		}
	})

	t.Run("runner", func(t *testing.T) {
		for _, step := range []string{"select", "load", "resolve", "report_phase", "closed_world", "setup_failure", "suppression_phase", "capabilities", "results"} {
			if doc.Runner[step] == "" {
				t.Errorf("runner[%q] = %q, want the step stated", step, doc.Runner[step])
			}
		}
	})
}

// corpusBaselineDocument mirrors testdata/corpus-baseline.json; an unknown key
// fails the decode.
type corpusBaselineDocument struct {
	Description        string   `json:"description"`
	CorpusVersion      string   `json:"corpus_version"`
	Fixtures           []string `json:"fixtures"`
	FixtureMembers     []string `json:"fixture_members"`
	ExpectationMembers []string `json:"expectation_members"`
	DetailsMembers     []string `json:"details_members"`
	ManifestMembers    []string `json:"manifest_members"`
}

// loadCorpusBaseline decodes the committed baseline, failing the test on any
// setup error and on an empty set, because an empty set pins nothing.
func loadCorpusBaseline(t *testing.T) corpusBaselineDocument {
	t.Helper()
	data, err := os.ReadFile(corpusBaselinePath)
	if err != nil {
		t.Fatalf("Setup: os.ReadFile(%q): %v", corpusBaselinePath, err)
	}
	var doc corpusBaselineDocument
	if err = corpus.DecodeStrict(data, &doc); err != nil {
		t.Fatalf("Setup: decoding %s: %v", corpusBaselinePath, err)
	}
	if len(doc.Fixtures) == 0 || len(doc.FixtureMembers) == 0 || len(doc.ExpectationMembers) == 0 || len(doc.DetailsMembers) == 0 || len(doc.ManifestMembers) == 0 {
		t.Fatalf("Setup: %s records %d fixture(s), %d fixture member(s), %d expectation member(s), %d details member(s) and %d manifest member(s), want the published sets",
			corpusBaselinePath, len(doc.Fixtures), len(doc.FixtureMembers), len(doc.ExpectationMembers), len(doc.DetailsMembers), len(doc.ManifestMembers))
	}
	return doc
}

// findingRelationAbsentKinds is the subject kinds contract/finding.schema.json
// judges by no liveness relation, which is the list corpus.json's subject_shapes
// block partitions into parts and rows.
func findingRelationAbsentKinds(t *testing.T) []string {
	t.Helper()
	_, kinds := findingLivenessCondition(t, findingLivenessArm(t, loadFindingSchema(t)))
	if len(kinds) == 0 {
		t.Fatalf("Setup: %s names no subject kind that carries no liveness relation", findingSchemaPath)
	}
	return kinds
}

// corpusFixtureNames is the name of every fixture the corpus holds, sorted, which
// is the order fs.ReadDir returns.
func corpusFixtureNames(t *testing.T) []string {
	t.Helper()
	entries, err := fs.ReadDir(spec.Corpus, fixturesDir)
	if err != nil {
		t.Fatalf("Setup: fs.ReadDir(Corpus, %q): %v", fixturesDir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestCorpusVersionMovesWithTheFixtureSetAndTheExpectationMembers pins
// corpus_version, the fixture directories and the members of the expectation and
// fixture schemas to the committed baseline: the two name the same version, and a
// fixture or a member added or removed fails until the baseline is rewritten.
// Whether that change also moves the version is corpus.json's rule, which no
// comparison here can decide. The check is an equality over sets rather than a
// digest of the corpus, so a description or runner-rule edit never trips it.
func TestCorpusVersionMovesWithTheFixtureSetAndTheExpectationMembers(t *testing.T) {
	doc := loadCorpus(t)
	baseline := loadCorpusBaseline(t)

	if baseline.CorpusVersion != doc.CorpusVersion {
		t.Fatalf("%s records corpus_version %q and %s publishes %q: the two move in one change, so rewrite %s for the published version",
			corpusBaselinePath, baseline.CorpusVersion, corpusPath, doc.CorpusVersion, corpusBaselinePath)
	}
	if got := corpusFixtureNames(t); !slices.Equal(got, baseline.Fixtures) {
		t.Errorf("%s holds the fixtures %v and %s records %v at corpus_version %q: a fixture added or removed fails until %s records it, so rewrite it, moving the version in %s in the same change unless it already stands above the last release by that level",
			fixturesDir, got, corpusBaselinePath, baseline.Fixtures, doc.CorpusVersion, corpusBaselinePath, corpusPath)
	}
	schema := loadSchema(t, spec.Corpus, expectSchemaPath)
	fixtureMembers := objectAt(schema, "properties")
	if fixtureMembers == nil {
		t.Fatalf("Setup: %s properties is not an object", expectSchemaPath)
	}
	if got := slices.Sorted(maps.Keys(fixtureMembers)); !slices.Equal(got, baseline.FixtureMembers) {
		t.Errorf("%s declares the fixture members %v and %s records %v at corpus_version %q: a member added or removed fails until %s records it, so rewrite it, moving the version in %s in the same change unless it already stands above the last release by that level",
			expectSchemaPath, got, corpusBaselinePath, baseline.FixtureMembers, doc.CorpusVersion, corpusBaselinePath, corpusPath)
	}
	members := objectAt(schema, expectSchemaRowPath+"/properties")
	if members == nil {
		t.Fatalf("Setup: %s %s/properties is not an object", expectSchemaPath, expectSchemaRowPath)
	}
	if got := slices.Sorted(maps.Keys(members)); !slices.Equal(got, baseline.ExpectationMembers) {
		t.Errorf("%s declares the expectation members %v and %s records %v at corpus_version %q: a member added or removed fails until %s records it, so rewrite it, moving the version in %s in the same change unless it already stands above the last release by that level",
			expectSchemaPath, got, corpusBaselinePath, baseline.ExpectationMembers, doc.CorpusVersion, corpusBaselinePath, corpusPath)
	}
	details := objectAt(schema, expectSchemaDetailsPath+"/properties")
	if details == nil {
		t.Fatalf("Setup: %s %s/properties is not an object", expectSchemaPath, expectSchemaDetailsPath)
	}
	if got := slices.Sorted(maps.Keys(details)); !slices.Equal(got, baseline.DetailsMembers) {
		t.Errorf("%s declares the details members %v and %s records %v at corpus_version %q: a member added or removed fails until %s records it, so rewrite it, moving the version in %s in the same change unless it already stands above the last release by that level",
			expectSchemaPath, got, corpusBaselinePath, baseline.DetailsMembers, doc.CorpusVersion, corpusBaselinePath, corpusPath)
	}
	manifest := objectAt(loadSchema(t, spec.Corpus, fixtureSchemaPath), "properties")
	if manifest == nil {
		t.Fatalf("Setup: %s properties is not an object", fixtureSchemaPath)
	}
	if got := slices.Sorted(maps.Keys(manifest)); !slices.Equal(got, baseline.ManifestMembers) {
		t.Errorf("%s declares the manifest members %v and %s records %v at corpus_version %q: a member added or removed fails until %s records it, so rewrite it, moving the version in %s in the same change unless it already stands above the last release by that level",
			fixtureSchemaPath, got, corpusBaselinePath, baseline.ManifestMembers, doc.CorpusVersion, corpusBaselinePath, corpusPath)
	}
}

func TestExpectSchemaNamesNoLanguage(t *testing.T) {
	schema := loadSchema(t, spec.Corpus, expectSchemaPath)
	top, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Setup: %s has no properties object", expectSchemaPath)
	}
	// The fixture level may name which renderings exist, and does so in
	// exactly one field; the expectations beneath it may not name any.
	if _, ok := top["languages"]; !ok {
		t.Errorf("%s properties = %v, want a fixture-level languages field", expectSchemaPath, slices.Sorted(maps.Keys(top)))
	}
	if got := languageBindings(top["expect"], "/properties/expect"); len(got) != 0 {
		t.Errorf("languageBindings(%s expect) = %v, want no property, enum or const naming a language", expectSchemaPath, got)
	}

	// A copy of the schema with one language property planted under expect[]
	// must be caught, or the check above is not looking.
	planted := loadSchema(t, spec.Corpus, expectSchemaPath)
	rowProps := objectAt(planted, "properties/expect/items/properties")
	if rowProps == nil {
		t.Fatalf("Setup: %s expect.items.properties is not an object", expectSchemaPath)
	}
	rowProps["language"] = map[string]any{"type": "string"}
	got := languageBindings(objectAt(planted, "properties/expect"), "/properties/expect")
	want := []string{"/properties/expect/items/properties/language"}
	if !slices.Equal(got, want) {
		t.Errorf("languageBindings(planted schema) = %v, want %v", got, want)
	}
}

func TestExpectSchemaEnumsAreTheContractVocabularies(t *testing.T) {
	schema := loadSchema(t, spec.Corpus, expectSchemaPath)
	kinds := loadKinds(t)
	cases := []struct {
		name string
		at   string
		want []string
	}{
		{name: "languages", at: "properties/languages/items", want: kinds.Languages},
		{name: "confidence", at: "properties/expect/items/properties/confidence", want: kinds.ReachabilityClasses},
		{name: "reachability_class", at: "properties/expect/items/properties/reachability_class", want: kinds.ReachabilityClasses},
		{name: "fixability", at: "properties/expect/items/properties/fixability", want: kinds.Fixabilities},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := enumAt(schema, tc.at); !slices.Equal(got, tc.want) {
				t.Errorf("enumAt(%s, %q) = %v, want %v from %s", expectSchemaPath, tc.at, got, tc.want, kindsPath)
			}
		})
	}
	t.Run("report", func(t *testing.T) {
		found := patterns(schema, "")
		got := found["/properties/expect/items/properties/report/pattern"]
		want := "^(" + kinds.Prefix + "[0-9]{4}|none)$"
		if got != want {
			t.Errorf("report pattern = %q, want %q", got, want)
		}
	})
}

// TestExpectSchemaSymbolKindBindsToAReportedSubject pins both directions of the
// arm the subject-kind member sits under: a row that names a code may name the
// subject's kind, and a row that reports nothing may not, because there is no
// finding whose subject kind could be asserted. The member's own value resolves
// against contract/finding.schema.json in corpus_selftest_test.go; this is the
// schema's arm rather than the vocabulary.
func TestExpectSchemaSymbolKindBindsToAReportedSubject(t *testing.T) {
	cases := []struct {
		name string
		row  string
		want string
	}{
		{
			name: "a_reported_subject_may_name_its_kind",
			row:  `{"symbol":"UnusedOption","report":"DS1401","symbol_kind":"parameter","confidence":"certain"}`,
		},
		{
			name: "a_reported_subject_need_not_name_its_kind",
			row:  `{"symbol":"UnusedOption","report":"DS1401","confidence":"certain"}`,
		},
		{
			name: "an_unreported_subject_may_not_name_a_kind",
			row:  `{"symbol":"UsedByConsumer","report":"none","symbol_kind":"function"}`,
			want: "/expect/0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := `{"name":"planted","description":"One planted expectation.","languages":["go"],"target_kind":"application","expect":[` + tc.row + `]}`
			problems, err := validationProblems(expectSchemaPath, []byte(document))
			if err != nil {
				t.Fatalf("Setup: validationProblems(%q, %s): %v", expectSchemaPath, tc.name, err)
			}
			if tc.want == "" {
				if len(problems) != 0 {
					t.Errorf("validationProblems(%q, %s) = %v, want no violation", expectSchemaPath, tc.name, problems)
				}
				return
			}
			named := false
			for _, problem := range problems {
				if problem.InstanceLocation == tc.want && problem.names("not") {
					named = true
				}
			}
			if !named {
				t.Errorf("validationProblems(%q, %s) = %v, want the not constraint at %q", expectSchemaPath, tc.name, problems, tc.want)
			}
		})
	}
}

func TestCorpusSchemasAreClosedAndDescribed(t *testing.T) {
	for _, p := range []string{expectSchemaPath, fixtureSchemaPath} {
		t.Run(subtestName(p), func(t *testing.T) {
			schema := loadSchema(t, spec.Corpus, p)
			if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
				t.Errorf("%s $schema = %v, want JSON Schema 2020-12", p, schema["$schema"])
			}
			if got := openObjects(schema, ""); len(got) != 0 {
				t.Errorf("openObjects(%s) = %v, want every object schema to close its key set", p, got)
			}
			if got := undescribedProperties(schema, ""); len(got) != 0 {
				t.Errorf("undescribedProperties(%s) = %v, want a description on every property", p, got)
			}
			for at, pattern := range patterns(schema, "") {
				if _, err := regexp.Compile(pattern); err != nil {
					t.Errorf("regexp.Compile(%s %s = %q) = %v, want a pattern both dialects accept", p, at, pattern, err)
				}
			}
			if refs := references(schema, ""); len(refs) != 0 {
				t.Errorf("%s $ref at %v, want every shape inline", p, refs)
			}
		})
	}
}

// references lists every $ref the schema declares, by path.
func references(node any, at string) []string {
	var hits []string
	switch n := node.(type) {
	case map[string]any:
		if _, ok := n["$ref"]; ok {
			hits = append(hits, at+"/$ref")
		}
		for key, child := range n {
			hits = append(hits, references(child, at+"/"+key)...)
		}
	case []any:
		for i, child := range n {
			hits = append(hits, references(child, fmt.Sprintf("%s/%d", at, i))...)
		}
	}
	slices.Sort(hits)
	return hits
}

// TestExpectSchemaClosedWorldNamesItsThreeFacts pins both directions of the
// fixture-level declaration a narrowing or a never-built fixture rests on: a
// fixture may name any of the facts, and a value outside the three, a repeated
// value or an empty array is refused, so the member cannot declare a world the
// runner has no configuration key for.
func TestExpectSchemaClosedWorldNamesItsThreeFacts(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{name: "the_consumer_set_alone", value: `["consumers"]`},
		{name: "the_matrix_alone", value: `["matrix"]`},
		{name: "the_roots_alone", value: `["roots"]`},
		{name: "every_fact", value: `["consumers","matrix","roots"]`},
		{name: "a_fact_the_runner_cannot_declare", value: `["entries"]`, want: "/closed_world/0"},
		{name: "one_fact_named_twice", value: `["matrix","matrix"]`, want: "/closed_world"},
		{name: "no_fact_at_all", value: `[]`, want: "/closed_world"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := `{"name":"planted","description":"One planted fixture.","languages":["go"],` +
				`"target_kind":"library","closed_world":` + tc.value +
				`,"expect":[{"symbol":"Narrowable","report":"DS1501","confidence":"certain"}]}`
			problems, err := validationProblems(expectSchemaPath, []byte(document))
			if err != nil {
				t.Fatalf("Setup: validationProblems(%q, %s): %v", expectSchemaPath, tc.name, err)
			}
			if tc.want == "" {
				if len(problems) != 0 {
					t.Errorf("validationProblems(%q, %s) = %v, want no violation", expectSchemaPath, tc.name, problems)
				}
				return
			}
			named := false
			for _, problem := range problems {
				if problem.InstanceLocation == tc.want {
					named = true
				}
			}
			if !named {
				t.Errorf("validationProblems(%q, %s) = %v, want a violation at %q", expectSchemaPath, tc.name, problems, tc.want)
			}
		})
	}
}

// TestExpectSchemaConfiguredRootsBindALogicalNameToAnEntry pins the fixture-level
// member a DS9004 row rests on: a member binds one fixture-local logical name to
// one configured-root entry, and a member naming no root, a name no expectation
// row could carry, or a value that is not a non-empty string is refused, so the
// member cannot declare a root the runner has no entry for.
func TestExpectSchemaConfiguredRootsBindALogicalNameToAnEntry(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantAt  string
		refused bool
	}{
		{name: "one_root", value: `{"AbsentPattern":"go://example.com/app#absent*"}`},
		{name: "a_root_and_a_pattern_that_matches", value: `{"AbsentPattern":"go://example.com/app#absent*","RootedPattern":"go://example.com/app#root*"}`},
		{name: "no_root_at_all", value: `{}`, refused: true, wantAt: "/configured_roots"},
		// A member name is not a value, so the violation is reported against the
		// document rather than against a location inside it.
		{name: "a_name_no_expectation_row_could_carry", value: `{"absent pattern":"go://example.com/app#absent*"}`, refused: true},
		{name: "an_entry_that_is_not_a_string", value: `{"AbsentPattern":["go://example.com/app#absent*"]}`, refused: true, wantAt: "/configured_roots/AbsentPattern"},
		{name: "an_entry_with_nothing_in_it", value: `{"AbsentPattern":""}`, refused: true, wantAt: "/configured_roots/AbsentPattern"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := `{"name":"planted","description":"One planted fixture.","languages":["go"],` +
				`"target_kind":"application","configured_roots":` + tc.value +
				`,"expect":[{"symbol":"AbsentPattern","report":"DS9004","confidence":"certain"}]}`
			problems, err := validationProblems(expectSchemaPath, []byte(document))
			if err != nil {
				t.Fatalf("Setup: validationProblems(%q, %s): %v", expectSchemaPath, tc.name, err)
			}
			if !tc.refused {
				if len(problems) != 0 {
					t.Errorf("validationProblems(%q, %s) = %v, want no violation", expectSchemaPath, tc.name, problems)
				}
				return
			}
			if len(problems) == 0 {
				t.Fatalf("validationProblems(%q, %s) = no violation, want the member refused", expectSchemaPath, tc.name)
			}
			if tc.wantAt == "" {
				return
			}
			named := false
			for _, problem := range problems {
				if strings.HasPrefix(problem.InstanceLocation, tc.wantAt) {
					named = true
				}
			}
			if !named {
				t.Errorf("validationProblems(%q, %s) = %v, want a violation at %q", expectSchemaPath, tc.name, problems, tc.wantAt)
			}
		})
	}
}

// TestExpectSchemaStaleSuppressionRowNamesNothingTheRecordLacks pins the arm the
// stale-suppression row sits under: the row is answered by a record of the
// report's stale-suppression array, and a record carries no reachability class, no
// liveness relation and no configurations, so a row naming one of those three is
// refused rather than left unanswerable. The mechanism the row pins and the
// confidence every coded row carries stay admitted.
func TestExpectSchemaStaleSuppressionRowNamesNothingTheRecordLacks(t *testing.T) {
	cases := []struct {
		name string
		row  string
		want string
	}{
		{
			name: "a_record_row_pins_its_mechanism",
			row:  `{"symbol":"StaleEntry","report":"DS9003","confidence":"certain","symbol_kind":"suppression","details":{"mechanism":"ignore"}}`,
		},
		{
			name: "a_record_row_naming_a_reachability_class",
			row:  `{"symbol":"StaleEntry","report":"DS9003","confidence":"certain","reachability_class":"certain","details":{"mechanism":"ignore"}}`,
			want: "/expect/0",
		},
		{
			name: "a_record_row_naming_a_liveness_relation",
			row:  `{"symbol":"StaleEntry","report":"DS9003","confidence":"certain","liveness_relation":"reachability","details":{"mechanism":"ignore"}}`,
			want: "/expect/0",
		},
		{
			name: "a_record_row_naming_a_configuration",
			row:  `{"symbol":"StaleEntry","report":"DS9003","confidence":"certain","configurations":["linux-amd64"],"details":{"mechanism":"ignore"}}`,
			want: "/expect/0",
		},
		{
			name: "another_self_check_row_names_all_three",
			row:  `{"symbol":"UnscopedEntry","report":"DS9002","confidence":"certain","reachability_class":"certain","configurations":["linux-amd64"],"details":{"mechanism":"ignore"}}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := `{"name":"planted","description":"One planted fixture.","languages":["go"],` +
				`"target_kind":"application","expect":[` + tc.row + `]}`
			problems, err := validationProblems(expectSchemaPath, []byte(document))
			if err != nil {
				t.Fatalf("Setup: validationProblems(%q, %s): %v", expectSchemaPath, tc.name, err)
			}
			if tc.want == "" {
				if len(problems) != 0 {
					t.Errorf("validationProblems(%q, %s) = %v, want no violation", expectSchemaPath, tc.name, problems)
				}
				return
			}
			named := false
			for _, problem := range problems {
				if strings.HasPrefix(problem.InstanceLocation, tc.want) {
					named = true
				}
			}
			if !named {
				t.Errorf("validationProblems(%q, %s) = %v, want a violation at %q", expectSchemaPath, tc.name, problems, tc.want)
			}
		})
	}
}

// TestExpectSchemaConfiguredDeclarationsBindALogicalNameToAKeyAndAnEntry pins the
// fixture-level member a DS9006 row rests on: a member binds one logical name to a
// key of the closed set and one entry, and a member naming no entry, an entry of
// another key, or a member without its entry is refused.
func TestExpectSchemaConfiguredDeclarationsBindALogicalNameToAKeyAndAnEntry(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		wantAt  string
		refused bool
	}{
		{name: "one_entry", value: `{"AbsentEntry":{"key":"ts.serializers","entry":{"global":"JSON.absent"}}}`},
		{name: "an_injection_registration", value: `{"AbsentEntry":{"key":"ts.injection_registrations","entry":{"symbol":"ts://./main.ts#register"}}}`},
		{name: "no_entry_at_all", value: `{}`, refused: true, wantAt: "/configured_declarations"},
		{name: "a_key_outside_the_set", value: `{"AbsentEntry":{"key":"ts.lifecycle_contracts","entry":{"global":"JSON.absent"}}}`, refused: true, wantAt: "/configured_declarations/AbsentEntry/key"},
		{name: "a_member_without_its_entry", value: `{"AbsentEntry":{"key":"ts.serializers"}}`, refused: true, wantAt: "/configured_declarations/AbsentEntry"},
		{name: "an_empty_entry", value: `{"AbsentEntry":{"key":"ts.serializers","entry":{}}}`, refused: true, wantAt: "/configured_declarations/AbsentEntry/entry"},
		{name: "an_undeclared_member", value: `{"AbsentEntry":{"key":"ts.serializers","entry":{"global":"JSON.absent"},"line":1}}`, refused: true, wantAt: "/configured_declarations/AbsentEntry"},
		{name: "a_lifecycle_component", value: `{"AbsentEntry":{"key":"ts.lifecycle_contracts.components","entry":{"symbol":"ts://./main.ts#component"},"members":["connected"]}}`},
		{name: "a_lifecycle_base", value: `{"AbsentEntry":{"key":"ts.lifecycle_contracts.bases","entry":{"global":"HTMLElement"},"members":["connectedCallback"]}}`},
		{name: "a_lifecycle_entry_without_members", value: `{"AbsentEntry":{"key":"ts.lifecycle_contracts.bases","entry":{"global":"HTMLElement"}}}`, refused: true, wantAt: "/configured_declarations/AbsentEntry"},
		{name: "members_under_another_key", value: `{"AbsentEntry":{"key":"ts.serializers","entry":{"global":"JSON.absent"},"members":["connected"]}}`, refused: true, wantAt: "/configured_declarations/AbsentEntry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := `{"name":"planted","description":"One planted fixture.","languages":["ts"],` +
				`"target_kind":"application","configured_declarations":` + tc.value +
				`,"expect":[{"symbol":"AbsentEntry","report":"DS9006","confidence":"certain"}]}`
			problems, err := validationProblems(expectSchemaPath, []byte(document))
			if err != nil {
				t.Fatalf("Setup: validationProblems(%q, %s): %v", expectSchemaPath, tc.name, err)
			}
			if !tc.refused {
				if len(problems) != 0 {
					t.Errorf("validationProblems(%q, %s) = %v, want no violation", expectSchemaPath, tc.name, problems)
				}
				return
			}
			named := false
			for _, problem := range problems {
				if strings.HasPrefix(problem.InstanceLocation, tc.wantAt) {
					named = true
				}
			}
			if !named {
				t.Errorf("validationProblems(%q, %s) = %v, want a violation at %q", expectSchemaPath, tc.name, problems, tc.wantAt)
			}
		})
	}
}

// findingDetailsRefs are the definitions of contract/finding.schema.json whose
// value is a position or a stable symbol reference. A details member that reaches
// one of them spells one language's own file names or symbol names, which no
// field of a language-neutral expectation does, so such a member has no form on
// an expectation row.
var findingDetailsRefs = []string{"#/$defs/position", "#/$defs/positioned_symbol", "#/$defs/symbol_reference"}

// logicalPositionDetails are the details members whose value is one position that a
// row spells as a logical name of the rendering's manifest, which every rendering
// binds to its own file and line.
var logicalPositionDetails = []string{"name_literal"}

// logicalNamePattern is the shape of a fixture-local logical name, as the
// expectation schema's symbol member declares it.
const logicalNamePattern = `^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`

// refValues lists every $ref value the schema declares, at any depth.
func refValues(node any) []string {
	var out []string
	switch n := node.(type) {
	case map[string]any:
		if ref, ok := n["$ref"].(string); ok {
			out = append(out, ref)
		}
		for _, child := range n {
			out = append(out, refValues(child)...)
		}
	case []any:
		for _, child := range n {
			out = append(out, refValues(child)...)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// stringsAt reads a JSON array of strings out of a decoded schema node.
func stringsAt(node any) []string {
	values, ok := node.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// conditionValues reads the values one discriminating field is conditioned on in
// an if branch, whether the branch writes a const or an enum.
func conditionValues(branch any, field string) []string {
	condition := objectAt(map[string]any{"root": branch}, "root/properties/"+field)
	if condition == nil {
		return nil
	}
	if value, ok := condition["const"].(string); ok {
		return []string{value}
	}
	return stringsAt(condition["enum"])
}

// detailsRequired collects every details member a branch of a schema requires,
// skipping the negated and the alternative subtrees, so what it returns is what
// the branch asserts a document carries rather than what it forbids.
func detailsRequired(node any) []string {
	var out []string
	switch n := node.(type) {
	case map[string]any:
		if props, ok := n["properties"].(map[string]any); ok {
			if details, ok := props["details"].(map[string]any); ok {
				out = append(out, stringsAt(details["required"])...)
			}
		}
		for key, child := range n {
			if key == "not" || key == "else" {
				continue
			}
			out = append(out, detailsRequired(child)...)
		}
	case []any:
		for _, child := range n {
			out = append(out, detailsRequired(child)...)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// detailsForbidden collects every details member a branch of a schema forbids,
// which is how an expectation row's arms state that a member belongs to another
// code.
func detailsForbidden(node any) []string {
	var out []string
	switch n := node.(type) {
	case map[string]any:
		if props, ok := n["properties"].(map[string]any); ok {
			if details, ok := props["details"].(map[string]any); ok {
				out = append(out, detailsRequired(details["not"])...)
				out = append(out, stringsAt(objectAt(details, "not")["required"])...)
			}
		}
		for _, child := range n {
			out = append(out, detailsForbidden(child)...)
		}
	case []any:
		for _, child := range n {
			out = append(out, detailsForbidden(child)...)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// armCodes maps each details member to the issue-kind codes one schema's arms
// bind it to. requiredSide reads the member out of the branch a code matches,
// which is how the finding schema writes the binding; the other form reads it out
// of the branch every other code matches, which is how an expectation row does,
// because the member is optional there.
func armCodes(arms any, field string, requiredSide bool) map[string][]string {
	out := map[string][]string{}
	list, ok := arms.([]any)
	if !ok {
		return out
	}
	for _, arm := range list {
		branch, ok := arm.(map[string]any)
		if !ok {
			continue
		}
		codes := conditionValues(branch["if"], field)
		if len(codes) == 0 {
			continue
		}
		members := detailsForbidden(branch["else"])
		if requiredSide {
			members = detailsRequired(branch["then"])
		}
		for _, member := range members {
			out[member] = append(out[member], codes...)
		}
	}
	for member, codes := range out {
		slices.Sort(codes)
		out[member] = slices.Compact(codes)
	}
	return out
}

// withoutDescriptions is the schema with every description dropped, which is what
// makes two documents comparable on shape alone: each states its own member in
// its own words and the values they admit must be identical.
func withoutDescriptions(node any) any {
	switch n := node.(type) {
	case map[string]any:
		out := make(map[string]any, len(n))
		for key, child := range n {
			if key == "description" {
				continue
			}
			out[key] = withoutDescriptions(child)
		}
		return out
	case []any:
		out := make([]any, len(n))
		for i, child := range n {
			out[i] = withoutDescriptions(child)
		}
		return out
	default:
		return node
	}
}

// asJSON renders a decoded schema node for a failure message.
func asJSON(t *testing.T, node any) string {
	t.Helper()
	data, err := json.MarshalIndent(node, "", " ")
	if err != nil {
		t.Fatalf("Setup: json.MarshalIndent: %v", err)
	}
	return string(data)
}

// admittedDetailsMembers is the details member set an expectation row may name:
// every member the finding schema binds to an issue-kind code, less those whose
// value is a position or a stable symbol reference. It returns the codes each
// admitted member is bound to, so a caller compares the binding as well as the
// set.
func admittedDetailsMembers(t *testing.T, finding map[string]any) map[string][]string {
	t.Helper()
	bound := armCodes(finding["allOf"], "code", true)
	if len(bound) == 0 {
		t.Fatalf("Setup: %s binds no details member to a code", findingSchemaPath)
	}
	properties := objectAt(finding, "properties/details/properties")
	if properties == nil {
		t.Fatalf("Setup: %s properties/details/properties is not an object", findingSchemaPath)
	}
	admitted := map[string][]string{}
	for member, codes := range bound {
		shape, ok := properties[member]
		if !ok {
			t.Errorf("%s binds %q to %v and declares no property of that name", findingSchemaPath, member, codes)
			continue
		}
		spelled := false
		for _, ref := range refValues(shape) {
			if slices.Contains(findingDetailsRefs, ref) {
				spelled = true
			}
		}
		if !spelled {
			admitted[member] = codes
		}
	}
	if len(admitted) == 0 {
		t.Fatalf("Setup: no details member of %s is statable by a language-neutral expectation", findingSchemaPath)
	}
	return admitted
}

// TestExpectSchemaDetailsMirrorsTheFindingSchema pins the mirror the expectation
// row carries. The corpus schemas declare every shape inline, which is what
// TestCorpusSchemasAreClosedAndDescribed enforces, so the row cannot reference
// the finding schema's details arms and states them again; this is the check that
// makes the copy answer to the original. It compares three things: which members
// the row admits, which codes each is bound to, and the values each admits.
func TestExpectSchemaDetailsMirrorsTheFindingSchema(t *testing.T) {
	finding := loadFindingSchema(t)
	expect := loadSchema(t, spec.Corpus, expectSchemaPath)
	admitted := admittedDetailsMembers(t, finding)

	row := objectAt(expect, expectSchemaRowPath)
	if row == nil {
		t.Fatalf("Setup: %s %s is not an object", expectSchemaPath, expectSchemaRowPath)
	}
	mirrored := armCodes(row["allOf"], "report", false)
	bound := armCodes(finding["allOf"], "code", true)
	for _, member := range logicalPositionDetails {
		codes, ok := bound[member]
		if !ok {
			t.Fatalf("Setup: %s binds details.%s to no code, want the codes whose finding carries it", findingSchemaPath, member)
		}
		t.Run(subtestName(member)+"_is_a_logical_name", func(t *testing.T) {
			if got := mirrored[member]; !slices.Equal(got, codes) {
				t.Errorf("%s binds details.%s to %v and %s binds it to %v, want the same codes", expectSchemaPath, member, got, findingSchemaPath, codes)
			}
			got := withoutDescriptions(objectAt(expect, expectSchemaDetailsPath+"/properties/"+member))
			want := map[string]any{"type": "string", "pattern": logicalNamePattern}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s declares details.%s as %s, want %s, a logical name and nothing else", expectSchemaPath, member, asJSON(t, got), asJSON(t, want))
			}
		})
		delete(mirrored, member)
	}

	t.Run("members", func(t *testing.T) {
		got, want := slices.Sorted(maps.Keys(mirrored)), slices.Sorted(maps.Keys(admitted))
		if !slices.Equal(got, want) {
			t.Errorf("%s binds the details members %v and %s binds %v to a code, less those whose value is a position or a stable symbol reference: want the same set",
				expectSchemaPath, got, findingSchemaPath, want)
		}
	})

	for _, member := range slices.Sorted(maps.Keys(admitted)) {
		t.Run(subtestName(member), func(t *testing.T) {
			if got, want := mirrored[member], admitted[member]; !slices.Equal(got, want) {
				t.Errorf("%s binds details.%s to %v and %s binds it to %v, want the same codes",
					expectSchemaPath, member, got, findingSchemaPath, want)
			}
			got := withoutDescriptions(objectAt(expect, expectSchemaDetailsPath+"/properties/"+member))
			want := withoutDescriptions(objectAt(finding, "properties/details/properties/"+member))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s declares details.%s as\n%s\nand %s declares it as\n%s\nwant the same shape",
					expectSchemaPath, member, asJSON(t, got), findingSchemaPath, asJSON(t, want))
			}
		})
	}
}

// TestResultsSchemaDetailsEchoTheExpectationRow pins the other half of the round
// trip: a runner records the answer in the expectation's own vocabulary, so the
// details members it may write are the members a row may pin, with the same
// shapes.
func TestResultsSchemaDetailsEchoTheExpectationRow(t *testing.T) {
	expect := objectAt(loadSchema(t, spec.Corpus, expectSchemaPath), expectSchemaDetailsPath)
	results := objectAt(loadSchema(t, spec.Corpus, resultsSchemaPath), resultsSchemaActualPath+"/properties/details")
	if expect == nil || results == nil {
		t.Fatalf("Setup: %s or %s declares no details object", expectSchemaPath, resultsSchemaPath)
	}
	got, want := withoutDescriptions(results), withoutDescriptions(expect)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s declares the answer's details as\n%s\nand %s declares the row's as\n%s\nwant the same shape",
			resultsSchemaPath, asJSON(t, got), expectSchemaPath, asJSON(t, want))
	}
}

// TestExpectSchemaDetailsBindToTheCodesArm pins the arms the details member sits
// under, in both directions: a row may name the members its code's arm carries,
// may not name a member another code's arm carries, may not name a member the
// schema does not declare, may not pin an empty object, and may not carry details
// at all when it reports nothing.
func TestExpectSchemaDetailsBindToTheCodesArm(t *testing.T) {
	cases := []struct {
		name string
		row  string
		want string
	}{
		{
			name: "a_narrowing_row_names_its_visibility",
			row:  `{"symbol":"InFile","report":"DS1501","confidence":"certain","details":{"narrower_visibility":"file"}}`,
		},
		{
			name: "an_intra_function_row_names_its_overlap",
			row:  `{"symbol":"UnusedOption","report":"DS1401","confidence":"certain","details":{"overlap":["unparam"]}}`,
		},
		{
			name: "a_row_need_not_name_any_details",
			row:  `{"symbol":"InFile","report":"DS1501","confidence":"certain"}`,
		},
		{
			name: "a_narrowing_row_may_not_name_an_overlap",
			row:  `{"symbol":"InFile","report":"DS1501","confidence":"certain","details":{"narrower_visibility":"file","overlap":["unparam"]}}`,
			want: "/expect/0/details",
		},
		{
			name: "an_intra_function_row_may_not_name_a_visibility",
			row:  `{"symbol":"UnusedOption","report":"DS1401","confidence":"certain","details":{"narrower_visibility":"file"}}`,
			want: "/expect/0/details",
		},
		{
			name: "a_row_may_not_name_a_member_the_schema_does_not_declare",
			row:  `{"symbol":"InFile","report":"DS1501","confidence":"certain","details":{"write_positions":[]}}`,
			want: "/expect/0/details",
		},
		{
			name: "a_row_may_not_pin_an_empty_details",
			row:  `{"symbol":"InFile","report":"DS1501","confidence":"certain","details":{}}`,
			want: "/expect/0/details",
		},
		{
			name: "an_unreported_subject_may_not_pin_details",
			row:  `{"symbol":"UsedByConsumer","report":"none","details":{"narrower_visibility":"file"}}`,
			want: "/expect/0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := `{"name":"planted","description":"One planted expectation.","languages":["go"],"target_kind":"application","expect":[` + tc.row + `]}`
			problems, err := validationProblems(expectSchemaPath, []byte(document))
			if err != nil {
				t.Fatalf("Setup: validationProblems(%q, %s): %v", expectSchemaPath, tc.name, err)
			}
			if tc.want == "" {
				if len(problems) != 0 {
					t.Errorf("validationProblems(%q, %s) = %v, want no violation", expectSchemaPath, tc.name, problems)
				}
				return
			}
			named := false
			for _, problem := range problems {
				if problem.InstanceLocation == tc.want {
					named = true
				}
			}
			if !named {
				t.Errorf("validationProblems(%q, %s) = %v, want a violation at %q", expectSchemaPath, tc.name, problems, tc.want)
			}
		})
	}
}

// componentMembersNoRowStates are the component members of contract/finding.schema.json
// no expectation row names besides those whose value is a positioned symbol: the
// identifier, which the analyzer that reports the component mints and which carries
// no meaning a fixture could pin.
var componentMembersNoRowStates = []string{"id"}

// admittedComponentMembers is the component member set an expectation row may name:
// every member the finding schema's component declares, less the identifier and
// every member whose value is a position or a stable symbol reference.
func admittedComponentMembers(t *testing.T, finding map[string]any) []string {
	t.Helper()
	properties := objectAt(finding, "properties/component/properties")
	if properties == nil {
		t.Fatalf("Setup: %s properties/component/properties is not an object", findingSchemaPath)
	}
	var admitted []string
	for member, shape := range properties {
		spelled := slices.ContainsFunc(refValues(shape), func(ref string) bool { return slices.Contains(findingDetailsRefs, ref) })
		if !spelled && !slices.Contains(componentMembersNoRowStates, member) {
			admitted = append(admitted, member)
		}
	}
	if len(admitted) == 0 {
		t.Fatalf("Setup: no component member of %s is statable by an expectation row", findingSchemaPath)
	}
	slices.Sort(admitted)
	return admitted
}

// TestExpectSchemaComponentMirrorsTheFindingSchema pins the mirror the row's
// component member carries, as the details mirror is pinned: the members the row
// admits and the values each admits are the finding schema's own.
func TestExpectSchemaComponentMirrorsTheFindingSchema(t *testing.T) {
	finding := loadFindingSchema(t)
	expect := loadSchema(t, spec.Corpus, expectSchemaPath)
	admitted := admittedComponentMembers(t, finding)
	row := objectAt(expect, expectSchemaRowPath+"/properties/component/properties")
	if row == nil {
		t.Fatalf("Setup: %s %s/properties/component/properties is not an object", expectSchemaPath, expectSchemaRowPath)
	}
	if got := slices.Sorted(maps.Keys(row)); !slices.Equal(got, admitted) {
		t.Errorf("%s admits the component members %v and %s declares %v, less the identifier and the positioned symbols: want the same set",
			expectSchemaPath, got, findingSchemaPath, admitted)
	}
	for _, member := range admitted {
		t.Run(subtestName(member), func(t *testing.T) {
			got := withoutDescriptions(row[member])
			want := withoutDescriptions(objectAt(finding, "properties/component/properties/"+member))
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s declares component.%s as\n%s\nand %s declares it as\n%s\nwant the same shape",
					expectSchemaPath, member, asJSON(t, got), findingSchemaPath, asJSON(t, want))
			}
		})
	}
}

// TestResultsSchemaComponentEchoesTheExpectationRow pins the round trip's other half:
// the component members a runner may write are the members a row may pin, with the
// same shapes.
func TestResultsSchemaComponentEchoesTheExpectationRow(t *testing.T) {
	expect := objectAt(loadSchema(t, spec.Corpus, expectSchemaPath), expectSchemaRowPath+"/properties/component")
	results := objectAt(loadSchema(t, spec.Corpus, resultsSchemaPath), resultsSchemaActualPath+"/properties/component")
	if expect == nil || results == nil {
		t.Fatalf("Setup: %s or %s declares no component object", expectSchemaPath, resultsSchemaPath)
	}
	got, want := withoutDescriptions(results), withoutDescriptions(expect)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s declares the answer's component as\n%s\nand %s declares the row's as\n%s\nwant the same shape",
			resultsSchemaPath, asJSON(t, got), expectSchemaPath, asJSON(t, want))
	}
}

// TestExpectSchemaComponentBindsToAReportedFinding pins the arms the component member
// sits under: a reported row may pin any member, and a row that reports nothing or
// is answered by a stale-suppression record may not, because neither has a
// component to compare.
func TestExpectSchemaComponentBindsToAReportedFinding(t *testing.T) {
	cases := []struct {
		name string
		row  string
		want string
	}{
		{
			name: "a_reported_row_pins_its_component",
			row:  `{"symbol":"Ledger","report":"DS1002","confidence":"certain","component":{"root":true,"symbol_count":4,"deletable_lines":7}}`,
		},
		{
			name: "a_reported_row_pins_one_member",
			row:  `{"symbol":"Ledger","report":"DS1002","confidence":"certain","component":{"deletable_lines":0}}`,
		},
		{
			name: "a_row_may_not_pin_an_empty_component",
			row:  `{"symbol":"Ledger","report":"DS1002","confidence":"certain","component":{}}`,
			want: "/expect/0/component",
		},
		{
			name: "a_row_may_not_pin_the_minted_identifier",
			row:  `{"symbol":"Ledger","report":"DS1002","confidence":"certain","component":{"id":"deadset-go/c-1"}}`,
			want: "/expect/0/component",
		},
		{
			name: "an_unreported_subject_may_not_pin_a_component",
			row:  `{"symbol":"Ledger.total","report":"none","component":{"root":false}}`,
			want: "/expect/0",
		},
		{
			name: "a_stale_suppression_row_may_not_pin_a_component",
			row:  `{"symbol":"StaleEntry","report":"DS9003","confidence":"certain","details":{"mechanism":"ignore"},"component":{"root":true}}`,
			want: "/expect/0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := `{"name":"planted","description":"One planted expectation.","languages":["go"],"target_kind":"application","expect":[` + tc.row + `]}`
			problems, err := validationProblems(expectSchemaPath, []byte(document))
			if err != nil {
				t.Fatalf("Setup: validationProblems(%q, %s): %v", expectSchemaPath, tc.name, err)
			}
			if tc.want == "" {
				if len(problems) != 0 {
					t.Errorf("validationProblems(%q, %s) = %v, want no violation", expectSchemaPath, tc.name, problems)
				}
				return
			}
			named := false
			for _, problem := range problems {
				if problem.InstanceLocation == tc.want {
					named = true
				}
			}
			if !named {
				t.Errorf("validationProblems(%q, %s) = %v, want a violation at %q", expectSchemaPath, tc.name, problems, tc.want)
			}
		})
	}
}

// TestExpectSchemaEdgeEvaluationsCarryAPendingFindingOnlyForADeadSide pins the shape of
// one evaluation: a dead side names the declaration and the code of its pending
// finding, and a live or absent side names neither.
func TestExpectSchemaEdgeEvaluationsCarryAPendingFindingOnlyForADeadSide(t *testing.T) {
	cases := []struct {
		name    string
		value   string
		refused bool
	}{
		{name: "a_dead_side", value: `{"edge":"wire/a","side":"provides","state":"dead","symbol":"A","report":"DS1002"}`},
		{name: "a_live_side", value: `{"edge":"wire/a","side":"provides","state":"live"}`},
		{name: "a_dead_side_without_its_finding", value: `{"edge":"wire/a","side":"provides","state":"dead","symbol":"A"}`, refused: true},
		{name: "an_absent_side_with_a_finding", value: `{"edge":"wire/a","side":"provides","state":"absent","report":"DS1002"}`, refused: true},
		{name: "a_side_outside_the_two", value: `{"edge":"wire/a","side":"consumes","state":"live"}`, refused: true},
		{name: "an_undeclared_member", value: `{"edge":"wire/a","side":"provides","state":"live","line":1}`, refused: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := `{"name":"planted","description":"One planted fixture.","languages":["go"],` +
				`"target_kind":"application","edge_evaluations":[` + tc.value + `],` +
				`"expect":[{"symbol":"A","report":"none"}]}`
			problems, err := validationProblems(expectSchemaPath, []byte(document))
			if err != nil {
				t.Fatalf("Setup: validationProblems(%q, %s): %v", expectSchemaPath, tc.name, err)
			}
			named := slices.ContainsFunc(problems, func(p validationProblem) bool {
				return strings.HasPrefix(p.InstanceLocation, "/edge_evaluations/0")
			})
			if named != tc.refused {
				t.Errorf("validationProblems(%q, %s) = %v, want a violation at /edge_evaluations/0 %t", expectSchemaPath, tc.name, problems, tc.refused)
			}
		})
	}
}

func TestExpectSchemaFixabilityBindsToAReportedFinding(t *testing.T) {
	cases := []struct {
		name string
		row  string
		want string
	}{
		{
			name: "a_reported_row_pins_its_fixability",
			row:  `{"symbol":"Ledger","report":"DS1002","confidence":"certain","fixability":"manual"}`,
		},
		{
			name: "a_value_outside_the_vocabulary",
			row:  `{"symbol":"Ledger","report":"DS1002","confidence":"certain","fixability":"removable"}`,
			want: "/expect/0/fixability",
		},
		{
			name: "an_unreported_subject_may_not_pin_a_fixability",
			row:  `{"symbol":"Ledger","report":"none","fixability":"deletable"}`,
			want: "/expect/0",
		},
		{
			name: "a_stale_suppression_row_may_not_pin_a_fixability",
			row:  `{"symbol":"StaleEntry","report":"DS9003","confidence":"certain","details":{"mechanism":"ignore"},"fixability":"none"}`,
			want: "/expect/0",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := `{"name":"planted","description":"One planted expectation.","languages":["go"],"target_kind":"application","expect":[` + tc.row + `]}`
			problems, err := validationProblems(expectSchemaPath, []byte(document))
			if err != nil {
				t.Fatalf("Setup: validationProblems(%q, %s): %v", expectSchemaPath, tc.name, err)
			}
			if tc.want == "" {
				if len(problems) != 0 {
					t.Errorf("validationProblems(%q, %s) = %v, want no violation", expectSchemaPath, tc.name, problems)
				}
				return
			}
			named := false
			for _, problem := range problems {
				if problem.InstanceLocation == tc.want {
					named = true
				}
			}
			if !named {
				t.Errorf("validationProblems(%q, %s) = %v, want a violation at %q", expectSchemaPath, tc.name, problems, tc.want)
			}
		})
	}
}
