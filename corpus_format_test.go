package spec_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cplieger/deadset-spec"
)

const (
	corpusPath        = "corpus/corpus.json"
	expectSchemaPath  = "corpus/expect.schema.json"
	fixtureSchemaPath = "corpus/fixture.schema.json"
	fixturesDir       = "corpus/fixtures"
	expectFile        = "expect.json"
	manifestFile      = "fixture.json"
	goRendering       = "go.txtar"
	tsRendering       = "ts"
	targetDir         = "target"
	goModFile         = "go.mod"
)

// corpusDocument mirrors corpus/corpus.json closely enough that an unknown
// key fails the decode.
type corpusDocument struct {
	Runner        map[string]string  `json:"runner"`
	Description   string             `json:"description"`
	CorpusVersion string             `json:"corpus_version"`
	Layout        corpusLayout       `json:"layout"`
	Schemas       corpusSchemas      `json:"schemas"`
	Vocabularies  corpusVocabularies `json:"vocabularies"`
}

type corpusLayout struct {
	Renderings        map[string]string `json:"renderings"`
	Description       string            `json:"description"`
	FixtureDirectory  string            `json:"fixture_directory"`
	ExpectationFile   string            `json:"expectation_file"`
	ManifestFile      string            `json:"manifest_file"`
	TargetDirectory   string            `json:"target_directory"`
	ConsumerDirectory string            `json:"consumer_directory"`
}

type corpusSchemas struct {
	Description     string `json:"description"`
	ExpectationFile string `json:"expectation_file"`
	ManifestFile    string `json:"manifest_file"`
	DeclaredGaps    string `json:"declared_gaps"`
	Results         string `json:"results"`
}

type corpusVocabularies struct {
	Fields      map[string]vocabulary `json:"fields"`
	Description string                `json:"description"`
}

type vocabulary struct {
	File        string   `json:"file"`
	Field       string   `json:"field"`
	Description string   `json:"description"`
	Also        []string `json:"also"`
}

// expectationDocument mirrors a fixture's expect.json; an unknown key fails
// the decode, so a fixture cannot carry a field the schema does not declare.
type expectationDocument struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	TargetKind  string           `json:"target_kind"`
	Languages   []string         `json:"languages"`
	Consumers   []string         `json:"consumers"`
	Expect      []expectationRow `json:"expect"`
}

type expectationRow struct {
	Symbol            string   `json:"symbol"`
	Report            string   `json:"report"`
	Confidence        string   `json:"confidence"`
	ReachabilityClass string   `json:"reachability_class"`
	LivenessRelation  string   `json:"liveness_relation"`
	Configurations    []string `json:"configurations"`
	RetainedBy        []string `json:"retained_by"`
}

// manifestDocument mirrors a rendering's fixture.json. Line is a pointer so
// an absent line is distinguishable from line zero.
type manifestDocument struct {
	Symbols        map[string]manifestPosition `json:"symbols"`
	Configurations []string                    `json:"configurations"`
}

type manifestPosition struct {
	Line *int   `json:"line"`
	File string `json:"file"`
}

// symbolPosition is what a logical name resolves to in one rendering.
type symbolPosition struct {
	File string
	Line int
}

var (
	errDuplicateSymbol = errors.New("logical name expected twice")
	errUnboundSymbol   = errors.New("logical name absent from the manifest")
	errOrphanSymbol    = errors.New("manifest name no expectation uses")
	errNoFile          = errors.New("manifest entry has no file")
	errNoLine          = errors.New("manifest entry has no line")
	errDuplicateTxtar  = errors.New("txtar section declared twice")

	errNoModule  = errors.New("go rendering module file declares no module path")
	errNoRequire = errors.New("go rendering consumer requires no target module")
	errNoReplace = errors.New("go rendering consumer replaces the target module with no relative path")

	errNoMatrix        = errors.New("expectation names a configuration and the rendering declares no build matrix")
	errUnknownConfig   = errors.New("expectation names a configuration outside the rendering's build matrix")
	errConfigsOutOrder = errors.New("expectation names its configurations outside the matrix order")
	errMatrixUnordered = errors.New("rendering declares its build matrix out of order")
	errMatrixUnused    = errors.New("rendering declares a build matrix no expectation names")

	semverPattern = regexp.MustCompile(`^([0-9]+)\.[0-9]+\.[0-9]+$`)

	// languageWords are the tokens that would bind an expectation to one
	// language, in any casing, as a property name or as an enumerated value.
	languageWords = []string{"go", "golang", "ts", "typescript", "javascript", "language", "languages"}

	// vocabularyFields are the expectation-file fields corpus.json must name
	// a closed vocabulary for.
	vocabularyFields = []string{"confidence", "languages", "reachability_class", "report", "retained_by"}
)

// decodeStrict decodes one JSON object, failing on any key the target type
// does not declare.
func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// loadCorpus decodes corpus/corpus.json, failing the test on any setup error.
func loadCorpus(t *testing.T) corpusDocument {
	t.Helper()
	data, err := fs.ReadFile(spec.Corpus, corpusPath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Corpus, %q): %v", corpusPath, err)
	}
	var doc corpusDocument
	if err = decodeStrict(data, &doc); err != nil {
		t.Fatalf("Setup: decoding %s: %v", corpusPath, err)
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
// when the path leaves the tree or ends on a non-object.
func objectAt(schema map[string]any, p string) map[string]any {
	var node any = schema
	for key := range strings.SplitSeq(p, "/") {
		m, ok := node.(map[string]any)
		if !ok {
			return nil
		}
		node = m[key]
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

// resolveExpectations maps every logical name the expectation file uses
// through the manifest to a file and a line. It fails on a name used twice,
// a name the manifest lacks, an entry without a file or a line, and a
// manifest name no expectation uses; every error names the logical name.
func resolveExpectations(expect *expectationDocument, manifest manifestDocument) (map[string]symbolPosition, error) {
	positions := make(map[string]symbolPosition, len(expect.Expect))
	expected := make(map[string]bool, len(expect.Expect))
	var errs []error
	for _, row := range expect.Expect {
		if expected[row.Symbol] {
			errs = append(errs, fmt.Errorf("%w: %q", errDuplicateSymbol, row.Symbol))
			continue
		}
		expected[row.Symbol] = true
		entry, ok := manifest.Symbols[row.Symbol]
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("%w: %q", errUnboundSymbol, row.Symbol))
		case entry.File == "":
			errs = append(errs, fmt.Errorf("%w: %q", errNoFile, row.Symbol))
		case entry.Line == nil || *entry.Line < 1:
			errs = append(errs, fmt.Errorf("%w: %q", errNoLine, row.Symbol))
		default:
			positions[row.Symbol] = symbolPosition{File: entry.File, Line: *entry.Line}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(manifest.Symbols)) {
		if !expected[name] {
			errs = append(errs, fmt.Errorf("%w: %q", errOrphanSymbol, name))
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return positions, nil
}

// checkConfigurations resolves every expectation's configuration list against
// the build matrix one rendering declares. A rendering declares the matrix a
// run derives from it and an expectation names configurations from that list,
// in the matrix's own order, so the two documents cannot drift into naming
// different configurations for one finding; a rendering that declares no matrix
// answers no expectation that names one, and a matrix no expectation names is a
// declaration with no reader.
func checkConfigurations(expect *expectationDocument, language string, matrix []string) []error {
	var errs []error
	if len(matrix) > 0 && !slices.IsSorted(matrix) {
		errs = append(errs, fmt.Errorf("%w: rendering %q declares %v", errMatrixUnordered, language, matrix))
	}
	named := 0
	for _, row := range expect.Expect {
		if len(row.Configurations) == 0 {
			continue
		}
		named++
		if len(matrix) == 0 {
			errs = append(errs, fmt.Errorf("%w: rendering %q symbol %q names %v", errNoMatrix, language, row.Symbol, row.Configurations))
			continue
		}
		at := -1
		for _, id := range row.Configurations {
			switch i := slices.Index(matrix, id); {
			case i < 0:
				errs = append(errs, fmt.Errorf("%w: rendering %q symbol %q names %q, and the matrix is %v", errUnknownConfig, language, row.Symbol, id, matrix))
			case i <= at:
				errs = append(errs, fmt.Errorf("%w: rendering %q symbol %q names %v, want the matrix order %v", errConfigsOutOrder, language, row.Symbol, row.Configurations, matrix))
			default:
				at = i
			}
		}
	}
	if len(matrix) > 0 && named == 0 {
		errs = append(errs, fmt.Errorf("%w: rendering %q declares %v", errMatrixUnused, language, matrix))
	}
	return errs
}

// parseTxtar reads a txtar archive into its sections by name, dropping the
// leading comment. The format: a marker line "-- NAME --" opens each section,
// surrounding white space in NAME is stripped, content runs to the next
// marker, and a missing final newline is treated as present.
func parseTxtar(data []byte) (map[string][]byte, error) {
	files := map[string][]byte{}
	_, name, rest := findMarker(data)
	for name != "" {
		var body []byte
		var next string
		body, next, rest = findMarker(rest)
		if _, dup := files[name]; dup {
			return nil, fmt.Errorf("%w: %q", errDuplicateTxtar, name)
		}
		files[name] = body
		name = next
	}
	return files, nil
}

// findMarker splits data at its first marker line into the bytes before it,
// the section name it opens, and the bytes after it. With no marker, before
// is all of data with a final newline supplied and name is empty.
func findMarker(data []byte) (before []byte, name string, after []byte) {
	const open, closing = "-- ", " --"
	for i := 0; ; {
		line, rest := data[i:], []byte(nil)
		if j := bytes.IndexByte(line, '\n'); j >= 0 {
			line, rest = line[:j], line[j+1:]
		}
		if bytes.HasPrefix(line, []byte(open)) && bytes.HasSuffix(line, []byte(closing)) && len(line) >= len(open)+len(closing) {
			name = strings.TrimSpace(string(line[len(open) : len(line)-len(closing)]))
			if name != "" {
				return data[:i], name, rest
			}
		}
		if rest == nil {
			return withFinalNewline(data), "", nil
		}
		i = len(data) - len(rest)
	}
}

func withFinalNewline(data []byte) []byte {
	if len(data) == 0 || data[len(data)-1] == '\n' {
		return data
	}
	return append(slices.Clone(data), '\n')
}

// lineCount is the number of lines in a file, counting an unterminated last
// line as one.
func lineCount(data []byte) int {
	n := bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		n++
	}
	return n
}

// renderingFiles reads one language rendering of the fixture at dir as a map
// from rendering-relative path to content: the sections of go.txtar for Go,
// the files under ts/ for TypeScript.
func renderingFiles(fsys fs.FS, dir, language string) (map[string][]byte, error) {
	switch language {
	case "go":
		data, err := fs.ReadFile(fsys, path.Join(dir, goRendering))
		if err != nil {
			return nil, err
		}
		return parseTxtar(data)
	case "ts":
		root := path.Join(dir, tsRendering)
		files := map[string][]byte{}
		err := fs.WalkDir(fsys, root, func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			files[strings.TrimPrefix(p, root+"/")], err = fs.ReadFile(fsys, p)
			return err
		})
		return files, err
	default:
		return nil, fmt.Errorf("no rendering layout for language %q", language)
	}
}

// checkFixture verifies the fixture at dir against the corpus format: its
// expectation file decodes and is named for its directory, only the listed
// languages have renderings, and every logical name resolves through every
// rendering's manifest to a file the rendering holds, on a line it has, with
// the target and every consumer present as a directory.
func checkFixture(fsys fs.FS, dir string) error {
	data, err := fs.ReadFile(fsys, path.Join(dir, expectFile))
	if err != nil {
		return err
	}
	var expect expectationDocument
	if err = decodeStrict(data, &expect); err != nil {
		return fmt.Errorf("%s: %w", expectFile, err)
	}
	var errs []error
	if expect.Name != path.Base(dir) {
		errs = append(errs, fmt.Errorf("%s name = %q, want the directory name %q", expectFile, expect.Name, path.Base(dir)))
	}
	for _, language := range []string{"go", "ts"} {
		listed := slices.Contains(expect.Languages, language)
		files, err := renderingFiles(fsys, dir, language)
		switch {
		case listed && err != nil:
			errs = append(errs, fmt.Errorf("rendering %q listed but unreadable: %w", language, err))
		case !listed && err == nil:
			errs = append(errs, fmt.Errorf("rendering %q present but not listed in languages %v", language, expect.Languages))
		case listed:
			errs = append(errs, checkRendering(&expect, language, files))
		}
	}
	return errors.Join(errs...)
}

// checkRendering resolves every expectation through one rendering's manifest
// and checks each resolved position against the rendering's files.
func checkRendering(expect *expectationDocument, language string, files map[string][]byte) error {
	manifestData, ok := files[manifestFile]
	if !ok {
		return fmt.Errorf("rendering %q has no %s", language, manifestFile)
	}
	var manifest manifestDocument
	if err := decodeStrict(manifestData, &manifest); err != nil {
		return fmt.Errorf("rendering %q %s: %w", language, manifestFile, err)
	}
	positions, err := resolveExpectations(expect, manifest)
	if err != nil {
		return fmt.Errorf("rendering %q: %w", language, err)
	}
	errs := checkConfigurations(expect, language, manifest.Configurations)
	for _, name := range slices.Sorted(maps.Keys(positions)) {
		pos := positions[name]
		content, ok := files[pos.File]
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("rendering %q symbol %q names file %q, which the rendering does not hold", language, name, pos.File))
		case pos.Line > lineCount(content):
			errs = append(errs, fmt.Errorf("rendering %q symbol %q names %s:%d, want a line within the file's %d", language, name, pos.File, pos.Line, lineCount(content)))
		}
	}
	for _, d := range append([]string{targetDir}, expect.Consumers...) {
		if !hasDirectory(files, d) {
			errs = append(errs, fmt.Errorf("rendering %q has no %s/ directory", language, d))
		}
	}
	if language == "go" && len(expect.Consumers) > 0 {
		errs = append(errs, checkGoConsumerModules(expect.Consumers, files)...)
	}
	return errors.Join(errs...)
}

// checkGoConsumerModules pins the rule corpus/corpus.json states for a Go
// rendering: each consumer module requires the target module and replaces it
// with the target's path relative to the consumer directory, so the rendering
// loads with no network access.
func checkGoConsumerModules(consumers []string, files map[string][]byte) []error {
	targetMod := path.Join(targetDir, goModFile)
	module, _, _ := goModDirectives(files[targetMod])
	if module == "" {
		return []error{fmt.Errorf("%w: %s", errNoModule, targetMod)}
	}
	var errs []error
	for _, consumer := range consumers {
		file := path.Join(consumer, goModFile)
		data, held := files[file]
		if !held {
			errs = append(errs, fmt.Errorf("%w: %s", errNoModule, file))
			continue
		}
		_, requires, replaces := goModDirectives(data)
		if !slices.Contains(requires, module) {
			errs = append(errs, fmt.Errorf("%w: %s names no require of %q", errNoRequire, file, module))
		}
		want := "../" + targetDir
		if got := replaces[module]; got != want {
			errs = append(errs, fmt.Errorf("%w: %s replaces %q with %q, want %q", errNoReplace, file, module, got, want))
		}
	}
	return errs
}

// goModDirectives reads a module file's own module path, the module paths it
// requires and the path each replace directive names, in either the one-line or
// the parenthesised block form.
func goModDirectives(data []byte) (module string, requires []string, replaces map[string]string) {
	replaces = map[string]string{}
	block := ""
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) == 0 {
			continue
		}
		if fields[0] == ")" {
			block = ""
			continue
		}
		verb := block
		if fields[0] == "module" || fields[0] == "require" || fields[0] == "replace" {
			verb, fields = fields[0], fields[1:]
			if len(fields) == 1 && fields[0] == "(" {
				block = verb
				continue
			}
		}
		switch {
		case verb == "module" && len(fields) > 0:
			module = fields[0]
		case verb == "require" && len(fields) > 0:
			requires = append(requires, fields[0])
		case verb == "replace" && len(fields) > 2 && fields[1] == "=>":
			replaces[fields[0]] = fields[2]
		}
	}
	return module, requires, replaces
}

func hasDirectory(files map[string][]byte, dir string) bool {
	for p := range files {
		if strings.HasPrefix(p, dir+"/") {
			return true
		}
	}
	return false
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
		want := map[string]string{"go": goRendering, "ts": tsRendering + "/"}
		if !maps.Equal(doc.Layout.Renderings, want) {
			t.Errorf("layout.renderings = %v, want %v, the layout this suite walks", doc.Layout.Renderings, want)
		}
		if got := slices.Sorted(maps.Keys(doc.Layout.Renderings)); !slices.Equal(got, kinds.Languages) {
			t.Errorf("layout.renderings languages = %v, want %s languages %v", got, kindsPath, kinds.Languages)
		}
		gotPaths := []string{doc.Layout.FixtureDirectory, doc.Layout.ExpectationFile, doc.Layout.ManifestFile, doc.Layout.TargetDirectory, doc.Layout.ConsumerDirectory}
		wantPaths := []string{fixturesDir + "/<name>/", expectFile, manifestFile, targetDir + "/", "<consumer>/"}
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

	t.Run("runner", func(t *testing.T) {
		for _, step := range []string{"select", "load", "resolve", "report_phase", "closed_world", "suppression_phase", "capabilities", "results"} {
			if doc.Runner[step] == "" {
				t.Errorf("runner[%q] = %q, want the step stated", step, doc.Runner[step])
			}
		}
	})
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

func TestResolveExpectationsBinds(t *testing.T) {
	line := func(n int) *int { return &n }
	cases := []struct {
		name     string
		expect   expectationDocument
		manifest manifestDocument
		want     map[string]symbolPosition
	}{
		{
			name: "one_reported_one_retained",
			expect: expectationDocument{Expect: []expectationRow{
				{Symbol: "DeadExport", Report: "DS1001", Confidence: "certain"},
				{Symbol: "SatisfiesWriter", Report: "none", RetainedBy: []string{"interface-satisfaction"}},
			}},
			manifest: manifestDocument{Symbols: map[string]manifestPosition{
				"DeadExport":      {File: "target/dead.go", Line: line(5)},
				"SatisfiesWriter": {File: "target/writer.go", Line: line(12)},
			}},
			want: map[string]symbolPosition{
				"DeadExport":      {File: "target/dead.go", Line: 5},
				"SatisfiesWriter": {File: "target/writer.go", Line: 12},
			},
		},
		{
			name:     "dotted_member",
			expect:   expectationDocument{Expect: []expectationRow{{Symbol: "Sink.Write", Report: "none"}}},
			manifest: manifestDocument{Symbols: map[string]manifestPosition{"Sink.Write": {File: "target/sink.ts", Line: line(3)}}},
			want:     map[string]symbolPosition{"Sink.Write": {File: "target/sink.ts", Line: 3}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveExpectations(&tc.expect, tc.manifest)
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
	twoRows := expectationDocument{Expect: []expectationRow{
		{Symbol: "DeadExport", Report: "DS1001", Confidence: "certain"},
		{Symbol: "UsedByConsumer", Report: "none"},
	}}
	cases := []struct {
		name       string
		wantErr    error
		wantSymbol string
		expect     expectationDocument
		manifest   manifestDocument
	}{
		{
			name:       "name_absent_from_manifest",
			expect:     twoRows,
			manifest:   manifestDocument{Symbols: map[string]manifestPosition{"DeadExport": {File: "target/dead.go", Line: line(5)}}},
			wantErr:    errUnboundSymbol,
			wantSymbol: "UsedByConsumer",
		},
		{
			name:   "entry_without_line",
			expect: twoRows,
			manifest: manifestDocument{Symbols: map[string]manifestPosition{
				"DeadExport":     {File: "target/dead.go", Line: line(5)},
				"UsedByConsumer": {File: "target/used.go"},
			}},
			wantErr:    errNoLine,
			wantSymbol: "UsedByConsumer",
		},
		{
			name:   "entry_with_line_zero",
			expect: twoRows,
			manifest: manifestDocument{Symbols: map[string]manifestPosition{
				"DeadExport":     {File: "target/dead.go", Line: line(5)},
				"UsedByConsumer": {File: "target/used.go", Line: line(0)},
			}},
			wantErr:    errNoLine,
			wantSymbol: "UsedByConsumer",
		},
		{
			name:   "entry_without_file",
			expect: twoRows,
			manifest: manifestDocument{Symbols: map[string]manifestPosition{
				"DeadExport":     {File: "target/dead.go", Line: line(5)},
				"UsedByConsumer": {Line: line(9)},
			}},
			wantErr:    errNoFile,
			wantSymbol: "UsedByConsumer",
		},
		{
			name:   "manifest_name_no_expectation_uses",
			expect: twoRows,
			manifest: manifestDocument{Symbols: map[string]manifestPosition{
				"DeadExport":     {File: "target/dead.go", Line: line(5)},
				"UsedByConsumer": {File: "target/used.go", Line: line(9)},
				"Forgotten":      {File: "target/old.go", Line: line(1)},
			}},
			wantErr:    errOrphanSymbol,
			wantSymbol: "Forgotten",
		},
		{
			name: "logical_name_expected_twice",
			expect: expectationDocument{Expect: []expectationRow{
				{Symbol: "DeadExport", Report: "DS1001", Confidence: "certain"},
				{Symbol: "DeadExport", Report: "none"},
			}},
			manifest:   manifestDocument{Symbols: map[string]manifestPosition{"DeadExport": {File: "target/dead.go", Line: line(5)}}},
			wantErr:    errDuplicateSymbol,
			wantSymbol: "DeadExport",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveExpectations(&tc.expect, tc.manifest)
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
	rows := func(configurations ...[]string) expectationDocument {
		doc := expectationDocument{}
		for i, ids := range configurations {
			doc.Expect = append(doc.Expect, expectationRow{
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
		expect  expectationDocument
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
			wantErr: errUnknownConfig,
		},
		{
			name:    "configurations_out_of_the_matrix_order",
			expect:  rows([]string{"linux-arm64", "linux-amd64"}),
			matrix:  matrix,
			wantErr: errConfigsOutOrder,
		},
		{
			name:    "one_configuration_named_twice",
			expect:  rows([]string{"linux-amd64", "linux-amd64"}),
			matrix:  matrix,
			wantErr: errConfigsOutOrder,
		},
		{
			name:    "no_matrix_declared",
			expect:  rows([]string{"linux-amd64"}),
			wantErr: errNoMatrix,
		},
		{
			name:    "matrix_declared_out_of_order",
			expect:  rows([]string{"linux-arm64"}),
			matrix:  []string{"linux-arm64", "linux-amd64"},
			wantErr: errMatrixUnordered,
		},
		{
			name:    "matrix_no_expectation_names",
			expect:  rows(nil),
			matrix:  matrix,
			wantErr: errMatrixUnused,
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

func TestParseTxtar(t *testing.T) {
	cases := []struct {
		name    string
		archive string
		want    map[string]string
	}{
		{
			name:    "comment_then_two_sections",
			archive: "a comment\n-- fixture.json --\n{}\n-- target/go.mod --\nmodule example.test/target\n",
			want:    map[string]string{"fixture.json": "{}\n", "target/go.mod": "module example.test/target\n"},
		},
		{
			name:    "padded_name_and_missing_final_newline",
			archive: "--   target/a.go   --\npackage target",
			want:    map[string]string{"target/a.go": "package target\n"},
		},
		{
			name:    "empty_section",
			archive: "-- empty --\n-- next --\nx\n",
			want:    map[string]string{"empty": "", "next": "x\n"},
		},
		{
			name:    "no_sections",
			archive: "only a comment\n",
			want:    map[string]string{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files, err := parseTxtar([]byte(tc.archive))
			if err != nil {
				t.Fatalf("parseTxtar(%q) error = %v, want nil", tc.archive, err)
			}
			got := make(map[string]string, len(files))
			for name, body := range files {
				got[name] = string(body)
			}
			if !maps.Equal(got, tc.want) {
				t.Errorf("parseTxtar(%q) = %q, want %q", tc.archive, got, tc.want)
			}
		})
	}
}

func TestParseTxtarRefusesDuplicateSection(t *testing.T) {
	archive := "-- a --\n1\n-- a --\n2\n"
	if _, err := parseTxtar([]byte(archive)); !errors.Is(err, errDuplicateTxtar) {
		t.Errorf("parseTxtar(%q) error = %v, want %v", archive, err, errDuplicateTxtar)
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
	if err := checkFixture(plantedFixture(), "corpus/fixtures/planted"); err != nil {
		t.Errorf("checkFixture(planted) = %v, want nil", err)
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
			wantMsg: errUnboundSymbol.Error() + `: "UsedByConsumer"`,
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
			wantMsg: errNoReplace.Error() + `: consumer/go.mod replaces "example.test/target" with ""`,
		},
		{
			name: "go_consumer_does_not_require_the_target",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/go.txtar"].Data = bytes.Replace(m["corpus/fixtures/planted/go.txtar"].Data,
					[]byte("\nrequire example.test/target v0.0.0\n"), []byte("\n"), 1)
			},
			wantMsg: errNoRequire.Error() + `: consumer/go.mod names no require of "example.test/target"`,
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
			err := checkFixture(fsys, "corpus/fixtures/planted")
			if err == nil {
				t.Fatalf("checkFixture(%s) = nil, want an error containing %q", tc.name, tc.wantMsg)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Errorf("checkFixture(%s) = %q, want it to contain %q", tc.name, err, tc.wantMsg)
			}
		})
	}
}

func TestFixturesOnDiskResolveThroughEveryRendering(t *testing.T) {
	entries, err := fs.ReadDir(spec.Corpus, fixturesDir)
	if errors.Is(err, fs.ErrNotExist) {
		// No fixture has landed yet; the planted fixture above keeps the walk
		// under test until one does.
		return
	}
	if err != nil {
		t.Fatalf("Setup: fs.ReadDir(Corpus, %q): %v", fixturesDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			t.Errorf("%s holds file %q, want only fixture directories", fixturesDir, e.Name())
			continue
		}
		t.Run(e.Name(), func(t *testing.T) {
			dir := path.Join(fixturesDir, e.Name())
			if err := checkFixture(spec.Corpus, dir); err != nil {
				t.Errorf("checkFixture(%q) = %v, want every expectation to resolve through every rendering", dir, err)
			}
		})
	}
}
