package corpus

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// The defects of a rendering's configurations and of its build matrix.
var (
	ErrNoMatrix          = errors.New("expectation names a configuration and the rendering declares no build matrix")
	ErrUnknownConfig     = errors.New("expectation names a configuration outside the rendering's build matrix")
	ErrConfigsOutOrder   = errors.New("expectation names its configurations outside the matrix order")
	ErrMatrixUnordered   = errors.New("rendering declares its build matrix out of order")
	ErrMatrixUnused      = errors.New("rendering declares a build matrix no expectation names and no closed-world declaration configures")
	ErrMatrixUndeclared  = errors.New("expectation file declares the build matrix complete and the rendering declares no build matrix")
	ErrNotBuiltOrder     = errors.New("rendering lists the configurations it drops out of bytewise order or twice")
	ErrNotBuiltBuilt     = errors.New("rendering lists a configuration as dropped and as built")
	ErrNotBuiltDeclared  = errors.New("rendering drops a configuration of a matrix the expectation file declares complete")
	ErrOutsideVocabulary = errors.New("expectation names a value outside its closed vocabulary")
)

// perLanguageDetails are the details members whose value one language alone
// spells, so a fixture listing more than one rendering cannot pin them.
var perLanguageDetails = []string{"dependency_class", "excluded_by", "overlap", "replacement"}

// missingConsumerClass is the setup-failure class whose fixture renders a
// declared consumer as absent.
const missingConsumerClass = "missing-consumer"

// edgesDocumentPath is where a rendering carries the edges document its target
// declares.
const edgesDocumentPath = TargetDir + "/deadset-edges.json"

// Check holds every fixture of the corpus in fsys to the corpus format and to the
// Contract's vocabularies, and the corpus to its coverage rules. It names every
// defect, prefixed with the fixture it is in.
func Check(fsys fs.FS) []error {
	doc, err := ReadDocument(fsys)
	if err != nil {
		return []error{err}
	}
	rules, err := ReadRules()
	if err != nil {
		return []error{err}
	}
	env, err := checkEnvironment(fsys)
	if err != nil {
		return []error{err}
	}
	entries, err := fs.ReadDir(fsys, FixturesDir)
	if err != nil {
		return []error{err}
	}
	var errs []error
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			errs = append(errs, fmt.Errorf("%s holds the file %q, want only fixture directories", FixturesDir, entry.Name()))
			continue
		}
		names = append(names, entry.Name())
	}
	var fixtures []Fixture
	for _, name := range names {
		fixture, err := checkFixture(fsys, doc, env, name)
		if err != nil {
			errs = append(errs, fmt.Errorf("fixture %s: %w", name, err))
		}
		if fixture != nil {
			fixtures = append(fixtures, *fixture)
		}
	}
	return append(errs, checkCoverage(fixtures, rules)...)
}

// environment is what the fixture checks read from the Contract and the
// expectation schema.
type environment struct {
	kindLanguages     map[string]string
	fixabilities      map[string][]string
	targetKinds       []string
	livenessRelations []string
	admittedDetails   []string
	languages         []string
	settable          []string
	classes           []string
	exemptions        []string
	symbolKinds       []string
}

func checkEnvironment(fsys fs.FS) (*environment, error) {
	schema, err := readSchema(fsys, ExpectSchemaPath)
	if err != nil {
		return nil, err
	}
	kindLanguages, err := subjectKindLanguages()
	if err != nil {
		return nil, err
	}
	env := &environment{
		kindLanguages:     kindLanguages,
		targetKinds:       enumAt(schema, "properties/target_kind/oneOf/0"),
		livenessRelations: enumAt(schema, "properties/expect/items/properties/liveness_relation"),
		admittedDetails:   slices.Sorted(maps.Keys(objectAt(schema, "properties/expect/items/properties/details/properties"))),
	}
	for _, language := range vocab.Languages() {
		env.languages = append(env.languages, string(language))
	}
	env.contractVocabularies()
	for name, set := range map[string][]string{"target_kind": env.targetKinds, "liveness_relation": env.livenessRelations, "details": env.admittedDetails} {
		if len(set) == 0 {
			return nil, fmt.Errorf("%s declares no %s vocabulary", ExpectSchemaPath, name)
		}
	}
	return env, nil
}

// checkFixture reads and checks one fixture, returning what it read whenever the
// expectation file decoded.
func checkFixture(fsys fs.FS, doc *Document, env *environment, name string) (*Fixture, error) {
	dir := path.Join(FixturesDir, name)
	data, err := fs.ReadFile(fsys, path.Join(dir, ExpectFile))
	if err != nil {
		return nil, err
	}
	fixture := &Fixture{Name: name, Renderings: map[string]*Rendering{}}
	if err := DecodeStrict(data, &fixture.Expect); err != nil {
		return nil, fmt.Errorf("%s: %w", ExpectFile, err)
	}
	expect := &fixture.Expect
	errs := checkExpectation(expect, env)
	if err := Validate(ExpectSchemaPath, data); err != nil {
		errs = append(errs, fmt.Errorf("%s does not meet %s: %w", ExpectFile, ExpectSchemaPath, err))
	}
	if expect.Name != name {
		errs = append(errs, fmt.Errorf("%s name = %q, want the directory name %q", ExpectFile, expect.Name, name))
	}
	for _, language := range slices.Sorted(maps.Keys(doc.Layout.Renderings)) {
		form := doc.Layout.Renderings[language]
		listed := slices.Contains(expect.Languages, language)
		files, err := renderingFiles(fsys, dir, form)
		switch {
		case listed && err != nil:
			errs = append(errs, fmt.Errorf("rendering %q listed but unreadable: %w", language, err))
		case !listed && err == nil:
			errs = append(errs, fmt.Errorf("rendering %q present but not listed in languages %v", language, expect.Languages))
		case listed:
			rendering := &Rendering{Language: language, Files: files}
			fixture.Renderings[language] = rendering
			errs = append(errs, checkRendering(expect, rendering))
		}
	}
	for _, language := range expect.Languages {
		if _, stated := doc.Layout.Renderings[language]; !stated {
			errs = append(errs, fmt.Errorf("languages lists %q, and %s states a rendering form for %v", language, DocumentPath, slices.Sorted(maps.Keys(doc.Layout.Renderings))))
		}
	}
	return fixture, errors.Join(errs...)
}

// checkExpectation applies the rules of one expectation file that need no
// rendering.
func checkExpectation(expect *Expectation, env *environment) []error {
	errs := checkSetupFailure(expect)
	errs = append(errs, checkVocabularies(expect, env)...)
	errs = append(errs, checkConventionsLanguage(expect)...)
	errs = append(errs, checkTypeScriptKeysLanguage(expect)...)
	errs = append(errs, checkConsumerTestsNamesAConsumer(expect)...)
	errs = append(errs, checkConfiguredRootsScope(expect)...)
	errs = append(errs, checkConfiguredDeclarationsScope(expect)...)
	errs = append(errs, checkSubjectKindLanguageScope(expect, env.kindLanguages)...)
	errs = append(errs, checkDetailsLanguageScope(expect)...)
	errs = append(errs, checkPinnedDetails(expect, env.admittedDetails)...)
	errs = append(errs, checkComponentLanguageScope(expect)...)
	errs = append(errs, checkLivenessRelation(expect)...)
	return errs
}

// checkLivenessRelation names every row naming a liveness relation for a subject
// whose finding carries none: one the analysis holds live, by its code, or one
// that is not a declaration, by its symbol kind.
func checkLivenessRelation(doc *Expectation) []error {
	var errs []error
	for i := range doc.Expect {
		row := &doc.Expect[i]
		if row.LivenessRelation == "" {
			continue
		}
		if kind, live := vocab.KindOf(vocab.Code(row.Report)); live && kind.HoldsLive {
			errs = append(errs, fmt.Errorf("%s names a liveness relation under %s, whose subject the analysis holds live", row.Symbol, row.Report))
		}
		if subject, known := vocab.SubjectOf(row.SymbolKind); known && subject != vocab.SubjectDeclaration {
			errs = append(errs, fmt.Errorf("%s names a liveness relation for a %s subject, which is no declaration", row.Symbol, row.SymbolKind))
		}
	}
	return errs
}

// checkRendering resolves every expectation through one rendering's manifest
// and checks each resolved position, the matrix, the edges document, the
// installed tree, the target and consumer directories and the Go module wiring.
func checkRendering(expect *Expectation, rendering *Rendering) error {
	language, files := rendering.Language, rendering.Files
	manifestData, ok := files[ManifestFile]
	if !ok {
		return fmt.Errorf("rendering %q has no %s", language, ManifestFile)
	}
	if err := DecodeStrict(manifestData, &rendering.Manifest); err != nil {
		return fmt.Errorf("rendering %q %s: %w", language, ManifestFile, err)
	}
	if err := Validate(ManifestSchemaPath, manifestData); err != nil {
		return fmt.Errorf("rendering %q %s does not meet %s: %w", language, ManifestFile, ManifestSchemaPath, err)
	}
	manifest := &rendering.Manifest
	sites, err := Resolve(expect, manifest)
	if err != nil {
		return fmt.Errorf("rendering %q: %w", language, err)
	}
	errs := checkConfigurations(expect, language, manifest.Configurations)
	errs = append(errs, checkConfigurationsNotBuilt(expect, language, manifest.Configurations, manifest.ConfigurationsNotBuilt)...)
	errs = append(errs, checkEdgeEvaluations(expect, language, files)...)
	errs = append(errs, checkInstalledPackages(language, files)...)
	for _, name := range slices.Sorted(maps.Keys(sites)) {
		site := sites[name]
		content, ok := files[site.File]
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("rendering %q symbol %q names file %q, which the rendering does not hold", language, name, site.File))
		case site.Line > lineCount(content):
			errs = append(errs, fmt.Errorf("rendering %q symbol %q names %s, want a line within the file's %d", language, name, site, lineCount(content)))
		}
	}
	errs = append(errs, checkDirectories(expect, language, files)...)
	return errors.Join(errs...)
}

// checkDirectories holds the rendering to a directory for the target and every
// consumer it holds, none for a consumer its setup failure names absent, and
// for Go to the module wiring of both.
func checkDirectories(expect *Expectation, language string, files map[string][]byte) []error {
	var errs []error
	absent := absentConsumers(expect)
	present := slices.DeleteFunc(slices.Clone(expect.Consumers), func(c string) bool { return slices.Contains(absent, c) })
	for _, d := range append([]string{TargetDir}, present...) {
		if !hasDirectory(files, d) {
			errs = append(errs, fmt.Errorf("rendering %q has no %s/ directory", language, d))
		}
	}
	for _, d := range absent {
		if hasDirectory(files, d) {
			errs = append(errs, fmt.Errorf("rendering %q holds %s/, which its missing-consumer setup failure names as absent", language, d))
		}
	}
	if language != string(vocab.Go) {
		return errs
	}
	if len(present) > 0 {
		errs = append(errs, checkGoConsumerModules(present, files)...)
	}
	return append(errs, checkGoDependencyModules(expect.Consumers, files)...)
}

// lineCount is the number of lines in a file, an unterminated last line one.
func lineCount(data []byte) int {
	n := bytes.Count(data, []byte("\n"))
	if len(data) > 0 && data[len(data)-1] != '\n' {
		n++
	}
	return n
}

func hasDirectory(files map[string][]byte, dir string) bool {
	for p := range files {
		if strings.HasPrefix(p, dir+"/") {
			return true
		}
	}
	return false
}

// checkConfigurations resolves every row's configuration list against the
// bytewise-ordered build matrix one rendering declares. A rendering with no
// matrix answers no row naming one and cannot carry the closed-world matrix
// fact, and a matrix no row names and no such fact configures has no reader.
func checkConfigurations(expect *Expectation, language string, matrix []string) []error {
	var errs []error
	if len(matrix) > 0 && !slices.IsSorted(matrix) {
		errs = append(errs, fmt.Errorf("%w: rendering %q declares %v", ErrMatrixUnordered, language, matrix))
	}
	named := 0
	for i := range expect.Expect {
		row := &expect.Expect[i]
		if len(row.Configurations) == 0 {
			continue
		}
		named++
		if len(matrix) == 0 {
			errs = append(errs, fmt.Errorf("%w: rendering %q symbol %q names %v", ErrNoMatrix, language, row.Symbol, row.Configurations))
			continue
		}
		errs = append(errs, matrixOrder(row, language, matrix)...)
	}
	configured := slices.Contains(expect.ClosedWorld, WorldMatrix)
	if len(matrix) > 0 && named == 0 && !configured {
		errs = append(errs, fmt.Errorf("%w: rendering %q declares %v", ErrMatrixUnused, language, matrix))
	}
	if len(matrix) == 0 && configured {
		errs = append(errs, fmt.Errorf("%w: rendering %q", ErrMatrixUndeclared, language))
	}
	return errs
}

// matrixOrder holds one row's configurations to the matrix and its order.
func matrixOrder(row *Row, language string, matrix []string) []error {
	var errs []error
	at := -1
	for _, id := range row.Configurations {
		switch i := slices.Index(matrix, id); {
		case i < 0:
			errs = append(errs, fmt.Errorf("%w: rendering %q symbol %q names %q, and the matrix is %v", ErrUnknownConfig, language, row.Symbol, id, matrix))
		case i <= at:
			errs = append(errs, fmt.Errorf("%w: rendering %q symbol %q names %v, want the matrix order %v", ErrConfigsOutOrder, language, row.Symbol, row.Configurations, matrix))
		default:
			at = i
		}
	}
	return errs
}

// checkConfigurationsNotBuilt holds configurations_not_built to each identifier
// once in bytewise order, none also built, and none where the expectation file
// declares the matrix complete, because only a derived configuration is dropped.
func checkConfigurationsNotBuilt(expect *Expectation, language string, built, dropped []string) []error {
	var errs []error
	for i := 1; i < len(dropped); i++ {
		if dropped[i-1] >= dropped[i] {
			errs = append(errs, fmt.Errorf("%w: rendering %q lists %q", ErrNotBuiltOrder, language, dropped))
			break
		}
	}
	for _, id := range dropped {
		if slices.Contains(built, id) {
			errs = append(errs, fmt.Errorf("%w: rendering %q lists %q", ErrNotBuiltBuilt, language, id))
		}
	}
	if len(dropped) > 0 && slices.Contains(expect.ClosedWorld, WorldMatrix) {
		errs = append(errs, fmt.Errorf("%w: rendering %q lists %q", ErrNotBuiltDeclared, language, dropped))
	}
	return errs
}

var installedPrefixes = []string{InstalledDir + "/", TargetDir + "/" + InstalledDir + "/"}

// checkInstalledPackages holds an installed tree to the layout corpus.json
// states: only a TypeScript rendering carries one, and every manifest below it
// names the package its path below node_modules gives.
func checkInstalledPackages(language string, files map[string][]byte) []error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(files)) {
		var rest string
		below := slices.ContainsFunc(installedPrefixes, func(prefix string) bool {
			var ok bool
			rest, ok = strings.CutPrefix(name, prefix)
			return ok
		})
		if !below {
			continue
		}
		if language != string(vocab.TS) {
			errs = append(errs, fmt.Errorf("rendering %q carries %s, which only a TypeScript rendering may", language, name))
			continue
		}
		dir, file := path.Split(rest)
		if file != "package.json" {
			continue
		}
		var manifest struct {
			Name    string `json:"name"`
			Private bool   `json:"private"`
		}
		if err := json.Unmarshal(files[name], &manifest); err != nil {
			errs = append(errs, fmt.Errorf("rendering %q %s: %w", language, name, err))
			continue
		}
		if want := strings.TrimSuffix(dir, "/"); manifest.Name != want || !manifest.Private {
			errs = append(errs, fmt.Errorf("rendering %q %s names %q private %t, want %q and private", language, name, manifest.Name, manifest.Private, want))
		}
	}
	return errs
}

// edgesDocument is a target's deadset-edges.json.
type edgesDocument struct {
	Description string `json:"description"`
	Edges       []struct {
		ID       string `json:"id"`
		Because  string `json:"because"`
		Provides string `json:"provides"`
		UsedBy   string `json:"used_by"`
	} `json:"edges"`
}

// checkEdgeEvaluations holds edge_evaluations to the edges document of the
// rendering: both or neither, only in a one-language fixture, every entry naming
// a declared side once, and every side the rendering's language spells named.
func checkEdgeEvaluations(expect *Expectation, language string, files map[string][]byte) []error {
	data, held := files[edgesDocumentPath]
	switch {
	case !held && len(expect.EdgeEvaluations) == 0:
		return nil
	case !held:
		return []error{fmt.Errorf("rendering %q names edge evaluations and carries no %s", language, edgesDocumentPath)}
	case len(expect.EdgeEvaluations) == 0:
		return []error{fmt.Errorf("rendering %q carries %s and the expectation file names no edge evaluation", language, edgesDocumentPath)}
	case len(expect.Languages) != 1:
		return []error{fmt.Errorf("edge_evaluations named by a fixture listing %v, want one language", expect.Languages)}
	}
	var edges edgesDocument
	if err := DecodeStrict(data, &edges); err != nil {
		return []error{fmt.Errorf("rendering %q %s: %w", language, edgesDocumentPath, err)}
	}
	prefix := language + "://"
	sides := map[string]bool{}
	for _, edge := range edges.Edges {
		for side, ref := range map[string]string{"provides": edge.Provides, "used_by": edge.UsedBy} {
			if strings.HasPrefix(ref, prefix) {
				sides[edge.ID+" "+side] = true
			}
		}
	}
	var errs []error
	named := map[string]bool{}
	for _, evaluation := range expect.EdgeEvaluations {
		key := evaluation.Edge + " " + evaluation.Side
		switch {
		case named[key]:
			errs = append(errs, fmt.Errorf("edge_evaluations names %s twice", key))
		case !sides[key]:
			errs = append(errs, fmt.Errorf("edge_evaluations names %s, which %s declares no %s side of", key, edgesDocumentPath, language))
		}
		named[key] = true
	}
	for _, key := range slices.Sorted(maps.Keys(sides)) {
		if !named[key] {
			errs = append(errs, fmt.Errorf("%s declares the %s side %s and edge_evaluations names no entry for it", edgesDocumentPath, language, key))
		}
	}
	return errs
}

// absentConsumers is the declared consumers a missing-consumer setup failure
// names, which its renderings hold no directory for.
func absentConsumers(expect *Expectation) []string {
	if expect.SetupFailure == nil || expect.SetupFailure.Class != missingConsumerClass {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(expect.Consumers), func(c string) bool {
		return !slices.Contains(expect.SetupFailure.Names, c)
	})
}

// checkSetupFailure holds a fixture whose run ends with a setup failure to no
// expectation and no member that configures or answers a report, and every
// other fixture to at least one expectation.
func checkSetupFailure(expect *Expectation) []error {
	if expect.SetupFailure == nil {
		if len(expect.Expect) == 0 {
			return []error{fmt.Errorf("%s expect is empty, want at least one expectation in a fixture with no setup_failure", ExpectFile)}
		}
		return nil
	}
	var errs []error
	if len(expect.Expect) != 0 {
		errs = append(errs, fmt.Errorf("%s holds setup_failure and %d expectations, want none", ExpectFile, len(expect.Expect)))
	}
	if expect.SetupFailure.Class == missingConsumerClass && len(absentConsumers(expect)) == 0 {
		errs = append(errs, fmt.Errorf("%s holds a %s setup failure whose names %v name none of its consumers %v, want the absent consumer named", ExpectFile, missingConsumerClass, expect.SetupFailure.Names, expect.Consumers))
	}
	reportMembers := map[string]bool{
		"closed_world":            len(expect.ClosedWorld) != 0,
		"configured_roots":        len(expect.ConfiguredRoots) != 0,
		"configured_declarations": len(expect.ConfiguredDeclarations) != 0,
		"edge_evaluations":        len(expect.EdgeEvaluations) != 0,
		"type_error_skips":        len(expect.TypeErrorSkips) != 0,
		"notes":                   len(expect.Notes) != 0,
		"conventions_applied":     len(expect.ConventionsApplied) != 0,
		"min_confidence":          expect.MinConfidence != "",
		"consumer_tests":          expect.ConsumerTests != "",
		"entry_files":             len(expect.EntryFiles) != 0,
		"disabled_conventions":    len(expect.DisabledConventions) != 0,
		"severity":                len(expect.Severity) != 0,
		"suppressions_in_effect":  expect.SuppressionsInEffect != nil,
	}
	for _, member := range slices.Sorted(maps.Keys(reportMembers)) {
		if reportMembers[member] {
			errs = append(errs, fmt.Errorf("%s holds setup_failure and %s, want no member that configures or answers a report", ExpectFile, member))
		}
	}
	return errs
}

// contractVocabularies reads the closed value sets the generated vocabulary
// holds: the codes a severity may name, those of the configuration's severity
// keys that name one kind, the classes, the exemption classes, the symbol kinds
// and the fixabilities of each code.
func (env *environment) contractVocabularies() {
	env.fixabilities = map[string][]string{}
	severity, _ := vocab.ConfigKeyOf(vocab.KeySeverity)
	for _, key := range severity.Values {
		if len(key) != vocab.FamilyKeyLength {
			env.settable = append(env.settable, key)
		}
	}
	kinds := vocab.Kinds()
	for i := range kinds {
		code := string(kinds[i].Code)
		env.fixabilities[code] = append(env.fixabilities[code], string(kinds[i].Fixability))
		for _, f := range kinds[i].FindingFixabilities {
			env.fixabilities[code] = append(env.fixabilities[code], string(f))
		}
	}
	for _, class := range vocab.Classes() {
		env.classes = append(env.classes, string(class))
	}
	classes := vocab.ExemptionClasses()
	for i := range classes {
		env.exemptions = append(env.exemptions, string(classes[i].Name))
	}
	env.symbolKinds = vocab.SymbolKinds()
}

// vocabularyCheck collects the values a fixture names outside their closed
// vocabulary.
type vocabularyCheck struct{ errs []error }

func (v *vocabularyCheck) value(subject, field, got string, want []string) {
	if !slices.Contains(want, got) {
		v.errs = append(v.errs, fmt.Errorf("%w: %s %s = %q, want one of %v", ErrOutsideVocabulary, subject, field, got, want))
	}
}

func (v *vocabularyCheck) optional(subject, field, got string, want []string) {
	if got != "" {
		v.value(subject, field, got, want)
	}
}

// checkVocabularies names every value the expectation file draws from a closed
// vocabulary and that the vocabulary does not hold.
func checkVocabularies(doc *Expectation, env *environment) []error {
	v := &vocabularyCheck{}
	if per := doc.TargetKind.PerLanguage; per != nil {
		for _, language := range slices.Sorted(maps.Keys(per)) {
			v.value("fixture", "target_kind language", language, env.languages)
		}
		for _, language := range env.languages {
			v.value("fixture", "target_kind."+language, per[language], env.targetKinds)
		}
	} else {
		v.value("fixture", "target_kind", doc.TargetKind.One, env.targetKinds)
	}
	for _, language := range doc.Languages {
		v.value("fixture", "languages", language, env.languages)
	}
	for _, code := range slices.Sorted(maps.Keys(doc.Severity)) {
		v.value("fixture", "severity", code, env.settable)
	}
	for i := range doc.Expect {
		v.row(&doc.Expect[i], env)
	}
	return v.errs
}

func (v *vocabularyCheck) row(row *Row, env *environment) {
	if row.Report != ReportNone && !vocab.Code(row.Report).Live() {
		v.errs = append(v.errs, fmt.Errorf("%w: %s report = %q, want a live code or %s", ErrOutsideVocabulary, row.Symbol, row.Report, ReportNone))
	}
	v.optional(row.Symbol, "symbol_kind", row.SymbolKind, env.symbolKinds)
	v.optional(row.Symbol, "confidence", row.Confidence, env.classes)
	v.optional(row.Symbol, "reachability_class", row.ReachabilityClass, env.classes)
	v.optional(row.Symbol, "liveness_relation", row.LivenessRelation, env.livenessRelations)
	v.optional(row.Symbol, "fixability under "+row.Report, row.Fixability, env.fixabilities[row.Report])
	for _, class := range row.RetainedBy {
		v.value(row.Symbol, "retained_by", class, env.exemptions)
	}
}

// checkConventionsLanguage holds conventions_applied to a one-language fixture,
// because a convention row is data of one language's analyzer.
func checkConventionsLanguage(expect *Expectation) []error {
	if len(expect.ConventionsApplied) == 0 || len(expect.Languages) == 1 {
		return nil
	}
	return []error{fmt.Errorf("%s holds conventions_applied and lists the languages %v, want exactly one language", ExpectFile, expect.Languages)}
}

// checkConsumerTestsNamesAConsumer holds consumer_tests to a fixture naming a
// consumer, because the setting decides only how a consumer's test file counts.
func checkConsumerTestsNamesAConsumer(expect *Expectation) []error {
	if expect.ConsumerTests == "" || len(expect.Consumers) != 0 {
		return nil
	}
	return []error{fmt.Errorf("%s holds consumer_tests and names no consumer, want a consumer whose test files the setting counts", ExpectFile)}
}

// checkTypeScriptKeysLanguage holds entry_files and disabled_conventions to the
// TypeScript rendering alone: a pattern spells that rendering's layout and a row
// is data of the TypeScript analyzer.
func checkTypeScriptKeysLanguage(expect *Expectation) []error {
	if slices.Equal(expect.Languages, []string{string(vocab.TS)}) {
		return nil
	}
	var errs []error
	if len(expect.EntryFiles) != 0 {
		errs = append(errs, fmt.Errorf("%s holds entry_files and lists the languages %v, want ts alone", ExpectFile, expect.Languages))
	}
	if len(expect.DisabledConventions) != 0 {
		errs = append(errs, fmt.Errorf("%s holds disabled_conventions and lists the languages %v, want ts alone", ExpectFile, expect.Languages))
	}
	return errs
}

// checkConfiguredRootsScope holds configured roots to a one-language fixture
// whose entries carry that language's prefix, because an entry spells one
// language's own scope.
func checkConfiguredRootsScope(doc *Expectation) []error {
	if len(doc.ConfiguredRoots) == 0 {
		return nil
	}
	if len(doc.Languages) != 1 {
		return []error{fmt.Errorf("the fixture names configured roots and lists %v: an entry spells one language's own scope, so the member is named by a fixture that lists one language", doc.Languages)}
	}
	var errs []error
	prefix := doc.Languages[0] + "://"
	for _, name := range slices.Sorted(maps.Keys(doc.ConfiguredRoots)) {
		if entry := doc.ConfiguredRoots[name]; !strings.HasPrefix(entry, prefix) {
			errs = append(errs, fmt.Errorf("configured root %s is %q, want it to carry the fixture's own language prefix %q", name, entry, prefix))
		}
	}
	return errs
}

// checkConfiguredDeclarationsScope holds every configured declaration to a
// fixture listing its key's language alone.
func checkConfiguredDeclarationsScope(doc *Expectation) []error {
	var errs []error
	for _, name := range slices.Sorted(maps.Keys(doc.ConfiguredDeclarations)) {
		language, _, _ := strings.Cut(doc.ConfiguredDeclarations[name].Key, ".")
		if !slices.Equal(doc.Languages, []string{language}) {
			errs = append(errs, fmt.Errorf("configured declaration %s is an entry of %s and the fixture lists %v, want the key's language alone",
				name, doc.ConfiguredDeclarations[name].Key, doc.Languages))
		}
	}
	return errs
}

// checkSubjectKindLanguageScope names every row naming a subject kind only one
// language declares in a fixture that does not list that language alone.
func checkSubjectKindLanguageScope(doc *Expectation, kindLanguages map[string]string) []error {
	var errs []error
	for i := range doc.Expect {
		row := &doc.Expect[i]
		language, single := kindLanguages[row.SymbolKind]
		if !single || slices.Equal(doc.Languages, []string{language}) {
			continue
		}
		errs = append(errs, fmt.Errorf("%s names the symbol_kind %q, a subject only %s declares, and the fixture lists %v: a row naming such a kind belongs to a fixture that lists that one language",
			row.Symbol, row.SymbolKind, language, doc.Languages))
	}
	return errs
}

// checkDetailsLanguageScope names every row pinning a member one language alone
// spells in a fixture listing more than one rendering.
func checkDetailsLanguageScope(doc *Expectation) []error {
	if len(doc.Languages) < 2 {
		return nil
	}
	var errs []error
	for i := range doc.Expect {
		row := &doc.Expect[i]
		for _, member := range row.Details.Named() {
			if slices.Contains(perLanguageDetails, member) {
				errs = append(errs, fmt.Errorf("%s pins details.%s and the fixture lists %v: a member one language alone spells is pinned only by a fixture that lists that one language",
					row.Symbol, member, doc.Languages))
			}
		}
	}
	return errs
}

// PinnableDetails is the details members a row reporting code must pin in a
// fixture listing languages: each member every finding of the kind carries in
// every listed language, that the expectation schema admits, and whose value no
// single language spells while the fixture lists several.
func PinnableDetails(code string, languages, admitted []string) []string {
	kind, live := vocab.KindOf(vocab.Code(code))
	if !live {
		return nil
	}
	var out []string
	for _, detail := range kind.Details {
		carried := detail.Presence.Required
		if len(detail.Presence.Languages) > 0 {
			carried = !slices.ContainsFunc(languages, func(language string) bool {
				return !slices.Contains(detail.Presence.Languages, vocab.Language(language))
			})
		}
		if !carried || !slices.Contains(admitted, detail.Member) {
			continue
		}
		if len(languages) > 1 && slices.Contains(perLanguageDetails, detail.Member) {
			continue
		}
		out = append(out, detail.Member)
	}
	slices.Sort(out)
	return out
}

// checkPinnedDetails names every row whose code's finding carries a member the
// expectation can state and the row does not pin, so a promoted kind is answered
// on its data and not on its position alone.
func checkPinnedDetails(doc *Expectation, admitted []string) []error {
	var errs []error
	for i := range doc.Expect {
		row := &doc.Expect[i]
		if row.Report == ReportNone {
			continue
		}
		got := row.Details.Named()
		for _, member := range PinnableDetails(row.Report, doc.Languages, admitted) {
			if !slices.Contains(got, member) {
				errs = append(errs, fmt.Errorf("%s reports %s and pins details %v, want it to pin %q as well: the kind carries that member and the expectation schema admits it",
					row.Symbol, row.Report, got, member))
			}
		}
	}
	return errs
}

// checkComponentLanguageScope names every row pinning a component count in a
// fixture listing more than one rendering: a count is a fact of one rendering's
// own declarations and lines.
func checkComponentLanguageScope(doc *Expectation) []error {
	if len(doc.Languages) < 2 {
		return nil
	}
	var errs []error
	for i := range doc.Expect {
		row := &doc.Expect[i]
		if row.Component == nil {
			continue
		}
		for _, count := range []struct {
			member string
			named  bool
		}{
			{"deletable_lines", row.Component.DeletableLines != nil},
			{"symbol_count", row.Component.SymbolCount != nil},
		} {
			if count.named {
				errs = append(errs, fmt.Errorf("%s pins component.%s and the fixture lists %v: a count of one rendering's declarations or lines is pinned only by a fixture that lists that one language",
					row.Symbol, count.member, doc.Languages))
			}
		}
	}
	return errs
}
