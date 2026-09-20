package spec_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/cplieger/deadset-spec/v2"
)

const (
	corpusPath        = "corpus/corpus.json"
	expectSchemaPath  = "corpus/expect.schema.json"
	fixtureSchemaPath = "corpus/fixture.schema.json"
	fixturesDir       = "corpus/fixtures"
	expectFile        = "expect.json"
	manifestFile      = "fixture.json"

	// corpusBaselinePath is the committed record of the fixture set and the
	// expectation-row members at the published corpus version, which is what
	// makes the version's own rule enforceable.
	corpusBaselinePath = "testdata/corpus-baseline.json"

	goRendering = "go.txtar"
	tsRendering = "ts"
	targetDir   = "target"
	goModFile   = "go.mod"
)

// corpusDocument mirrors corpus/corpus.json closely enough that an unknown
// key fails the decode.
type corpusDocument struct {
	Runner        map[string]string   `json:"runner"`
	Description   string              `json:"description"`
	CorpusVersion string              `json:"corpus_version"`
	Layout        corpusLayout        `json:"layout"`
	Schemas       corpusSchemas       `json:"schemas"`
	Vocabularies  corpusVocabularies  `json:"vocabularies"`
	SubjectShapes corpusSubjectShapes `json:"subject_shapes"`
}

// corpusSubjectShapes mirrors the subject_shapes block: the two shapes a subject
// is named under, with every other kind of the finding schema's vocabulary a
// declaration by omission.
type corpusSubjectShapes struct {
	Description string   `json:"description"`
	Part        []string `json:"part"`
	Row         []string `json:"row"`
}

type corpusLayout struct {
	Renderings          map[string]string `json:"renderings"`
	Description         string            `json:"description"`
	FixtureDirectory    string            `json:"fixture_directory"`
	ExpectationFile     string            `json:"expectation_file"`
	ManifestFile        string            `json:"manifest_file"`
	TargetDirectory     string            `json:"target_directory"`
	ConsumerDirectory   string            `json:"consumer_directory"`
	DependencyDirectory string            `json:"dependency_directory"`
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
	ClosedWorld []string         `json:"closed_world"`
	Expect      []expectationRow `json:"expect"`
}

type expectationRow struct {
	Details           *expectationDetails `json:"details"`
	Symbol            string              `json:"symbol"`
	Report            string              `json:"report"`
	SymbolKind        string              `json:"symbol_kind"`
	Confidence        string              `json:"confidence"`
	ReachabilityClass string              `json:"reachability_class"`
	LivenessRelation  string              `json:"liveness_relation"`
	Configurations    []string            `json:"configurations"`
	RetainedBy        []string            `json:"retained_by"`
}

// expectationDetails mirrors the details a row pins, one field per member the
// expectation schema admits, so a member the schema does not declare fails the
// decode as well as the validation.
type expectationDetails struct {
	Overlap            []string `json:"overlap"`
	NarrowerVisibility string   `json:"narrower_visibility"`
	ExcludedBy         string   `json:"excluded_by"`
	DependencyClass    string   `json:"dependency_class"`
	Replacement        string   `json:"replacement"`
	Mechanism          string   `json:"mechanism"`
	Edge               string   `json:"edge"`
}

// perLanguageDetails are the details members whose value one language alone
// spells: a build constraint, a dependency section, a replace directive's
// right-hand side and a linter list are each written in one language's own terms,
// so a fixture that lists more than one rendering cannot pin them.
var perLanguageDetails = []string{"dependency_class", "excluded_by", "overlap", "replacement"}

// namedDetails lists the details members one row pins, sorted.
func (row expectationRow) namedDetails() []string {
	if row.Details == nil {
		return nil
	}
	named := map[string]bool{
		"dependency_class":    row.Details.DependencyClass != "",
		"edge":                row.Details.Edge != "",
		"excluded_by":         row.Details.ExcludedBy != "",
		"mechanism":           row.Details.Mechanism != "",
		"narrower_visibility": row.Details.NarrowerVisibility != "",
		"overlap":             len(row.Details.Overlap) > 0,
		"replacement":         row.Details.Replacement != "",
	}
	var out []string
	for member, present := range named {
		if present {
			out = append(out, member)
		}
	}
	slices.Sort(out)
	return out
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

	errNoDependencyRequire = errors.New("go rendering target requires no dependency module")
	errNoDependencyReplace = errors.New("go rendering target replaces the dependency module with no relative path")

	errNoMatrix         = errors.New("expectation names a configuration and the rendering declares no build matrix")
	errUnknownConfig    = errors.New("expectation names a configuration outside the rendering's build matrix")
	errConfigsOutOrder  = errors.New("expectation names its configurations outside the matrix order")
	errMatrixUnordered  = errors.New("rendering declares its build matrix out of order")
	errMatrixUnused     = errors.New("rendering declares a build matrix no expectation names and no closed-world declaration configures")
	errMatrixUndeclared = errors.New("expectation file declares the build matrix complete and the rendering declares no build matrix")

	semverPattern = regexp.MustCompile(`^([0-9]+)\.[0-9]+\.[0-9]+$`)

	// languageWords are the tokens that would bind an expectation to one
	// language, in any casing, as a property name or as an enumerated value.
	languageWords = []string{"go", "golang", "ts", "typescript", "javascript", "language", "languages"}

	// vocabularyFields are the expectation-file fields corpus.json must name
	// a closed vocabulary for.
	vocabularyFields = []string{"confidence", "details", "languages", "reachability_class", "report", "retained_by", "symbol_kind"}
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
// closedWorldMatrix is the fact an expectation file names to have the runner
// configure the rendering's own matrix and declare it complete, which is the one
// declaration under which a file no configuration builds is a finding.
const closedWorldMatrix = "matrix"

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
	configured := slices.Contains(expect.ClosedWorld, closedWorldMatrix)
	if len(matrix) > 0 && named == 0 && !configured {
		errs = append(errs, fmt.Errorf("%w: rendering %q declares %v", errMatrixUnused, language, matrix))
	}
	if len(matrix) == 0 && configured {
		errs = append(errs, fmt.Errorf("%w: rendering %q", errMatrixUndeclared, language))
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
	if language == "go" {
		if len(expect.Consumers) > 0 {
			errs = append(errs, checkGoConsumerModules(expect.Consumers, files)...)
		}
		errs = append(errs, checkGoDependencyModules(expect.Consumers, files)...)
	}
	return errors.Join(errs...)
}

// topLevelDirectories is every directory at the rendering root, sorted, which is
// the target, the consumers the expectation file names and the dependencies it
// names by leaving them out.
func topLevelDirectories(files map[string][]byte) []string {
	held := map[string]bool{}
	for p := range files {
		if dir, _, nested := strings.Cut(p, "/"); nested {
			held[dir] = true
		}
	}
	return slices.Sorted(maps.Keys(held))
}

// checkGoDependencyModules pins the rule corpus/corpus.json states for a Go
// rendering's dependency directories: a directory at the rendering root that is
// neither the target nor a consumer holds a module the target's own module file
// requires and replaces with that directory's path relative to the target, so the
// target resolves the requirement from the rendering rather than from a network.
func checkGoDependencyModules(consumers []string, files map[string][]byte) []error {
	dependencies := slices.DeleteFunc(topLevelDirectories(files), func(dir string) bool {
		return dir == targetDir || slices.Contains(consumers, dir)
	})
	if len(dependencies) == 0 {
		return nil
	}
	targetMod := path.Join(targetDir, goModFile)
	_, requires, replaces := goModDirectives(files[targetMod])
	var errs []error
	for _, dependency := range dependencies {
		file := path.Join(dependency, goModFile)
		module, _, _ := goModDirectives(files[file])
		if module == "" {
			errs = append(errs, fmt.Errorf("%w: %s", errNoModule, file))
			continue
		}
		if !slices.Contains(requires, module) {
			errs = append(errs, fmt.Errorf("%w: %s names no require of %q", errNoDependencyRequire, targetMod, module))
		}
		want := "../" + dependency
		if got := replaces[module]; got != want {
			errs = append(errs, fmt.Errorf("%w: %s replaces %q with %q, want %q", errNoDependencyReplace, targetMod, module, got, want))
		}
	}
	return errs
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
		case verb == "replace":
			// The old module may carry a version, so the arrow is found
			// rather than assumed to be the second field.
			if at := slices.Index(fields, "=>"); at > 0 && at+1 < len(fields) {
				replaces[fields[0]] = fields[at+1]
			}
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
		gotPaths := []string{doc.Layout.FixtureDirectory, doc.Layout.ExpectationFile, doc.Layout.ManifestFile, doc.Layout.TargetDirectory, doc.Layout.ConsumerDirectory, doc.Layout.DependencyDirectory}
		wantPaths := []string{fixturesDir + "/<name>/", expectFile, manifestFile, targetDir + "/", "<consumer>/", "<dependency>/"}
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
			t.Errorf("subject_shapes.description = %q, want the three shapes stated", shapes.Description)
		}
		vocabulary := enumAt(loadFindingSchema(t), "properties/symbol/properties/kind")
		if len(vocabulary) == 0 {
			t.Fatalf("Setup: %s declares no symbol.kind vocabulary", findingSchemaPath)
		}
		named := map[string]string{}
		for shape, kinds := range map[string][]string{"part": shapes.Part, "row": shapes.Row} {
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
				t.Errorf("%s lists %q as carrying no liveness relation and subject_shapes names it under neither part nor row", findingSchemaPath, kind)
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
	if err = decodeStrict(data, &doc); err != nil {
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

// TestCorpusVersionMovesWithTheFixtureSetAndTheExpectationMembers enforces the
// rule corpus.json states for itself and could not check: the minor number moves
// when a fixture or a field is added. The baseline records the three sets at the
// version it names, so a fixture directory, a fixture-level member or an
// expectation-row member added while the published version stays where the
// baseline left it fails here, and moving the version and rewriting the baseline
// in one change is the green path.
//
// The three sets are the ones a version bump is decidable from. A change to a
// description or to a runner rule moves the version too and leaves this document
// alone, which is why the check is an equality over sets rather than a digest of
// the corpus.
func TestCorpusVersionMovesWithTheFixtureSetAndTheExpectationMembers(t *testing.T) {
	doc := loadCorpus(t)
	baseline := loadCorpusBaseline(t)

	if baseline.CorpusVersion != doc.CorpusVersion {
		t.Fatalf("%s records corpus_version %q and %s publishes %q: the two move in one change, so rewrite %s for the published version",
			corpusBaselinePath, baseline.CorpusVersion, corpusPath, doc.CorpusVersion, corpusBaselinePath)
	}
	if got := corpusFixtureNames(t); !slices.Equal(got, baseline.Fixtures) {
		t.Errorf("%s holds the fixtures %v and %s records %v at corpus_version %q: a fixture added or removed moves the version, so move it in %s and rewrite %s in one change",
			fixturesDir, got, corpusBaselinePath, baseline.Fixtures, doc.CorpusVersion, corpusPath, corpusBaselinePath)
	}
	schema := loadSchema(t, spec.Corpus, expectSchemaPath)
	fixtureMembers := objectAt(schema, "properties")
	if fixtureMembers == nil {
		t.Fatalf("Setup: %s properties is not an object", expectSchemaPath)
	}
	if got := slices.Sorted(maps.Keys(fixtureMembers)); !slices.Equal(got, baseline.FixtureMembers) {
		t.Errorf("%s declares the fixture members %v and %s records %v at corpus_version %q: a member added or removed moves the version, so move it in %s and rewrite %s in one change",
			expectSchemaPath, got, corpusBaselinePath, baseline.FixtureMembers, doc.CorpusVersion, corpusPath, corpusBaselinePath)
	}
	members := objectAt(schema, expectSchemaRowPath+"/properties")
	if members == nil {
		t.Fatalf("Setup: %s %s/properties is not an object", expectSchemaPath, expectSchemaRowPath)
	}
	if got := slices.Sorted(maps.Keys(members)); !slices.Equal(got, baseline.ExpectationMembers) {
		t.Errorf("%s declares the expectation members %v and %s records %v at corpus_version %q: a member added or removed moves the version, so move it in %s and rewrite %s in one change",
			expectSchemaPath, got, corpusBaselinePath, baseline.ExpectationMembers, doc.CorpusVersion, corpusPath, corpusBaselinePath)
	}
	details := objectAt(schema, expectSchemaDetailsPath+"/properties")
	if details == nil {
		t.Fatalf("Setup: %s %s/properties is not an object", expectSchemaPath, expectSchemaDetailsPath)
	}
	if got := slices.Sorted(maps.Keys(details)); !slices.Equal(got, baseline.DetailsMembers) {
		t.Errorf("%s declares the details members %v and %s records %v at corpus_version %q: a member added or removed moves the version, so move it in %s and rewrite %s in one change",
			expectSchemaPath, got, corpusBaselinePath, baseline.DetailsMembers, doc.CorpusVersion, corpusPath, corpusBaselinePath)
	}
	manifest := objectAt(loadSchema(t, spec.Corpus, fixtureSchemaPath), "properties")
	if manifest == nil {
		t.Fatalf("Setup: %s properties is not an object", fixtureSchemaPath)
	}
	if got := slices.Sorted(maps.Keys(manifest)); !slices.Equal(got, baseline.ManifestMembers) {
		t.Errorf("%s declares the manifest members %v and %s records %v at corpus_version %q: a member added or removed moves the version, so move it in %s and rewrite %s in one change",
			fixtureSchemaPath, got, corpusBaselinePath, baseline.ManifestMembers, doc.CorpusVersion, corpusPath, corpusBaselinePath)
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
			row:  `{"symbol":"UnusedOption","report":"DS1801","symbol_kind":"parameter","confidence":"certain"}`,
		},
		{
			name: "a_reported_subject_need_not_name_its_kind",
			row:  `{"symbol":"UnusedOption","report":"DS1801","confidence":"certain"}`,
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
		{
			name: "matrix_the_closed_world_declaration_configures",
			expect: func() expectationDocument {
				doc := rows(nil)
				doc.ClosedWorld = []string{closedWorldMatrix}
				return doc
			}(),
			matrix: matrix,
		},
		{
			name: "the_matrix_declared_complete_and_none_rendered",
			expect: func() expectationDocument {
				doc := rows(nil)
				doc.ClosedWorld = []string{closedWorldMatrix}
				return doc
			}(),
			wantErr: errMatrixUndeclared,
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
			name: "go_dependency_the_target_does_not_replace",
			mutate: func(m fstest.MapFS) {
				m["corpus/fixtures/planted/go.txtar"].Data = append(m["corpus/fixtures/planted/go.txtar"].Data,
					"-- dep/go.mod --\nmodule example.test/dep\n"...)
			},
			wantMsg: errNoDependencyReplace.Error() + `: target/go.mod replaces "example.test/dep" with ""`,
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
			wantMsg: errNoDependencyRequire.Error() + `: target/go.mod names no require of "example.test/dep"`,
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

// TestExpectSchemaClosedWorldNamesItsTwoFacts pins both directions of the
// fixture-level declaration a narrowing or a never-built fixture rests on: a
// fixture may name either fact or both, and a value outside the two, a repeated
// value or an empty array is refused, so the member cannot declare a world the
// runner has no configuration key for.
func TestExpectSchemaClosedWorldNamesItsTwoFacts(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  string
	}{
		{name: "the_consumer_set_alone", value: `["consumers"]`},
		{name: "the_matrix_alone", value: `["matrix"]`},
		{name: "both_facts", value: `["consumers","matrix"]`},
		{name: "a_fact_the_runner_cannot_declare", value: `["roots"]`, want: "/closed_world/0"},
		{name: "one_fact_named_twice", value: `["matrix","matrix"]`, want: "/closed_world"},
		{name: "no_fact_at_all", value: `[]`, want: "/closed_world"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			document := `{"name":"planted","description":"One planted fixture.","languages":["go"],` +
				`"target_kind":"library","closed_world":` + tc.value +
				`,"expect":[{"symbol":"Narrowable","report":"DS1101","confidence":"certain"}]}`
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

// findingDetailsRefs are the definitions of contract/finding.schema.json whose
// value is a position or a stable symbol reference. A details member that reaches
// one of them spells one language's own file names or symbol names, which no
// field of a language-neutral expectation does, so such a member has no form on
// an expectation row.
var findingDetailsRefs = []string{"#/$defs/position", "#/$defs/positioned_symbol", "#/$defs/symbol_reference"}

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

// detailsBinding is one branch of the finding schema's discrimination: the codes
// it holds for, the languages it narrows to, and the details members a finding
// under it carries. An empty language list is every language.
type detailsBinding struct {
	Codes     []string
	Languages []string
	Members   []string
}

// findingDetailsBindings reads every branch of the finding schema's allOf that
// binds details members to issue-kind codes, keeping the language a nested branch
// narrows to, so a caller knows both which codes carry a member and which
// language's findings do.
func findingDetailsBindings(schema map[string]any) []detailsBinding {
	var out []detailsBinding
	arms, ok := schema["allOf"].([]any)
	if !ok {
		return nil
	}
	for _, arm := range arms {
		branch, ok := arm.(map[string]any)
		if !ok {
			continue
		}
		codes := conditionValues(branch["if"], "code")
		if len(codes) == 0 {
			continue
		}
		then, _ := branch["then"].(map[string]any)
		if members := stringsAt(objectAt(then, "properties/details")["required"]); len(members) > 0 {
			out = append(out, detailsBinding{Codes: codes, Members: members})
		}
		nested, _ := then["allOf"].([]any)
		for _, inner := range nested {
			language, ok := inner.(map[string]any)
			if !ok {
				continue
			}
			innerThen, _ := language["then"].(map[string]any)
			if members := stringsAt(objectAt(innerThen, "properties/details")["required"]); len(members) > 0 {
				out = append(out, detailsBinding{Codes: codes, Languages: conditionValues(language["if"], "language"), Members: members})
			}
		}
	}
	return out
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

// checkDetailsLanguageScope reports every row that pins a details member one
// language alone spells in a fixture that lists more than one rendering. Every
// expectation applies to every rendering listed, so such a row asserts a value
// the other rendering's finding cannot carry; the expectation schema states the
// rule and JSON Schema cannot check it, because the fixture's language list and
// the row's details sit in two places the arms cannot relate.
func checkDetailsLanguageScope(doc *expectationDocument) []error {
	if len(doc.Languages) < 2 {
		return nil
	}
	var errs []error
	for _, row := range doc.Expect {
		for _, member := range row.namedDetails() {
			if slices.Contains(perLanguageDetails, member) {
				errs = append(errs, fmt.Errorf("%s %s pins details.%s and the fixture lists %v: a member one language alone spells is pinned only by a fixture that lists that one language",
					doc.Name, row.Symbol, member, doc.Languages))
			}
		}
	}
	return errs
}

func TestCorpusDetailsPinNoValueAnotherRenderingCannotCarry(t *testing.T) {
	for _, doc := range corpusExpectations(t) {
		t.Run("fixture_"+doc.Name, func(t *testing.T) {
			if got := checkDetailsLanguageScope(&doc); len(got) != 0 {
				t.Errorf("checkDetailsLanguageScope(%s) = %v, want no row pinning a value one language alone spells", doc.Name, got)
			}
		})
	}

	// One planted document per per-language member, in a two-language fixture,
	// so a member that stopped being scoped reports here.
	for _, member := range perLanguageDetails {
		t.Run("planted_"+member, func(t *testing.T) {
			details := &expectationDetails{}
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
			doc := expectationDocument{
				Name:       "planted",
				TargetKind: "library",
				Languages:  []string{"go", "ts"},
				Expect: []expectationRow{
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

// pinnableDetails is the details members a fixture listing languages must pin on
// a row reporting code, which is what makes the promotion of a kind carry the
// finding's own data rather than its position alone. A member is pinnable when
// the finding schema binds it to the code for every language the fixture lists,
// the expectation schema admits it, and its value is not one a single language
// spells while the fixture lists several.
func pinnableDetails(bindings []detailsBinding, admitted map[string][]string, code string, languages []string) []string {
	var out []string
	for _, binding := range bindings {
		if !slices.Contains(binding.Codes, code) {
			continue
		}
		if len(binding.Languages) > 0 {
			covered := true
			for _, language := range languages {
				covered = covered && slices.Contains(binding.Languages, language)
			}
			if !covered {
				continue
			}
		}
		for _, member := range binding.Members {
			if _, ok := admitted[member]; !ok {
				continue
			}
			if len(languages) > 1 && slices.Contains(perLanguageDetails, member) {
				continue
			}
			out = append(out, member)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// TestCorpusRowsPinEveryDetailsMemberTheirCodeCarries is the completeness half of
// the details member: a row whose code's finding carries a member the expectation
// can state pins it, so a promoted kind is answered on its data and not on its
// position alone. The finding schema is the source of what each code carries, so a
// code that gains a member turns every fixture reporting it red here until the
// fixture pins it.
func TestCorpusRowsPinEveryDetailsMemberTheirCodeCarries(t *testing.T) {
	finding := loadFindingSchema(t)
	bindings := findingDetailsBindings(finding)
	if len(bindings) == 0 {
		t.Fatalf("Setup: %s binds no details member to a code", findingSchemaPath)
	}
	admitted := admittedDetailsMembers(t, finding)

	for _, doc := range corpusExpectations(t) {
		t.Run("fixture_"+doc.Name, func(t *testing.T) {
			for _, row := range doc.Expect {
				if row.Report == reportedNone {
					continue
				}
				want := pinnableDetails(bindings, admitted, row.Report, doc.Languages)
				if len(want) == 0 {
					continue
				}
				got := row.namedDetails()
				for _, member := range want {
					if !slices.Contains(got, member) {
						t.Errorf("%s %s reports %s and pins details %v, want it to pin %q as well: %s binds that member to the code and %s admits it",
							doc.Name, row.Symbol, row.Report, got, member, findingSchemaPath, expectSchemaPath)
					}
				}
			}
		})
	}
}

// TestPinnableDetailsFollowsTheFindingSchemaAndTheLanguageList pins the rule the
// check above applies, on the two cases a fixture cannot state: a member the
// finding schema binds to one language the fixture does not list, and a member
// whose value one language spells in a fixture that lists several.
func TestPinnableDetailsFollowsTheFindingSchemaAndTheLanguageList(t *testing.T) {
	finding := loadFindingSchema(t)
	bindings := findingDetailsBindings(finding)
	admitted := admittedDetailsMembers(t, finding)
	cases := []struct {
		name      string
		code      string
		languages []string
		want      []string
	}{
		{name: "a_go_only_never_built_row_pins_the_constraint", code: "DS1501", languages: []string{"go"}, want: []string{"excluded_by"}},
		{name: "a_typescript_never_built_row_pins_nothing", code: "DS1501", languages: []string{"ts"}, want: nil},
		{name: "a_two_language_never_built_row_pins_nothing", code: "DS1501", languages: []string{"go", "ts"}, want: nil},
		{name: "a_narrowing_row_pins_its_visibility_in_either_language", code: "DS1101", languages: []string{"go", "ts"}, want: []string{"narrower_visibility"}},
		{name: "a_two_language_dependency_row_pins_nothing", code: "DS1601", languages: []string{"go", "ts"}, want: nil},
		{name: "a_go_only_dependency_row_pins_its_section", code: "DS1601", languages: []string{"go"}, want: []string{"dependency_class"}},
		{name: "an_interface_row_pins_nothing_the_row_can_state", code: "DS1201", languages: []string{"go"}, want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pinnableDetails(bindings, admitted, tc.code, tc.languages); !slices.Equal(got, tc.want) {
				t.Errorf("pinnableDetails(%s, %v) = %v, want %v", tc.code, tc.languages, got, tc.want)
			}
		})
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
			row:  `{"symbol":"InFile","report":"DS1101","confidence":"certain","details":{"narrower_visibility":"file"}}`,
		},
		{
			name: "an_intra_function_row_names_its_overlap",
			row:  `{"symbol":"UnusedOption","report":"DS1801","confidence":"certain","details":{"overlap":["unparam"]}}`,
		},
		{
			name: "a_row_need_not_name_any_details",
			row:  `{"symbol":"InFile","report":"DS1101","confidence":"certain"}`,
		},
		{
			name: "a_narrowing_row_may_not_name_an_overlap",
			row:  `{"symbol":"InFile","report":"DS1101","confidence":"certain","details":{"narrower_visibility":"file","overlap":["unparam"]}}`,
			want: "/expect/0/details",
		},
		{
			name: "an_intra_function_row_may_not_name_a_visibility",
			row:  `{"symbol":"UnusedOption","report":"DS1801","confidence":"certain","details":{"narrower_visibility":"file"}}`,
			want: "/expect/0/details",
		},
		{
			name: "a_row_may_not_name_a_member_the_schema_does_not_declare",
			row:  `{"symbol":"InFile","report":"DS1101","confidence":"certain","details":{"write_positions":[]}}`,
			want: "/expect/0/details",
		},
		{
			name: "a_row_may_not_pin_an_empty_details",
			row:  `{"symbol":"InFile","report":"DS1101","confidence":"certain","details":{}}`,
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
