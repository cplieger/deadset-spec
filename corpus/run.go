package corpus

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	spec "github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// Analyzer is the command line of the product a run answers the corpus through.
// Command is the program and any leading arguments; a verb and its options follow.
type Analyzer struct {
	Language string
	Env      []string
	Command  []string
}

// Options narrow and shape one run.
type Options struct {
	// Scratch is the directory each fixture's scratch copy is made in; the
	// system's temporary directory when empty.
	Scratch string
	// Gaps are the capabilities the product declares it declines.
	Gaps []GapRow
	// Fixtures names the fixtures to answer; every selected one when empty.
	Fixtures []string
	// Jobs is how many fixtures run at once; one when below one.
	Jobs int
}

// The file names the run writes and reads in a fixture's scratch directory.
const (
	ignoreFile    = "deadset-ignore.json"
	nodeModules   = "node_modules"
	renderingDir  = "rendering"
	setupPrefix   = "setup failure: "
	runnerReason  = "Written by the conformance run to check that the finding it names is suppressed."
	ignoreSummary = "The adjudications the conformance run writes for its suppression phase."
	typeErrorCap  = "type-error-skips"
)

// StaleGap is the message of a gap row whose expectation the analyzer answers as
// written, which a product removes the declared gap for.
const StaleGap = "the analyzer answers this expectation as written, and a declared gap covers it"

// errFixture is a fixture the runner could not complete, recorded as its failure.
var errFixture = errors.New("fixture")

func fixtureError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errFixture, fmt.Sprintf(format, args...))
}

// Run answers every fixture of the corpus in fsys that lists the analyzer's
// language through the analyzer's command line, in two phases, and returns the
// results document. The product's name and version are what its describe verb
// prints. An error means the run itself could not proceed; a fixture the runner
// could not complete is a failed row.
func Run(ctx context.Context, fsys fs.FS, analyzer Analyzer, options Options) (*Results, error) {
	doc, err := ReadDocument(fsys)
	if err != nil {
		return nil, err
	}
	product, err := describe(ctx, analyzer)
	if err != nil {
		return nil, err
	}
	selected, err := selectFixtures(fsys, doc, analyzer.Language, options.Fixtures)
	if err != nil {
		return nil, err
	}
	r := &runner{analyzer: analyzer, options: options, rows: map[string]bool{}}
	for _, kind := range doc.SubjectShapes.Row {
		r.rows[kind] = true
	}
	rows := make([]FixtureResult, len(selected))
	work := make(chan int)
	var wg sync.WaitGroup
	for range max(options.Jobs, 1) {
		wg.Go(func() {
			for i := range work {
				rows[i] = r.answerFixture(ctx, selected[i])
			}
		})
	}
	for i := range selected {
		work <- i
	}
	close(work)
	wg.Wait()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	results := &Results{
		CorpusVersion: doc.CorpusVersion,
		Product:       Product{Name: product.Name, Version: product.Version, Language: analyzer.Language},
		Fixtures:      rows,
	}
	results.tally()
	return results, nil
}

// selectFixtures loads every fixture whose expectation file lists the language,
// or of those only the ones named.
func selectFixtures(fsys fs.FS, doc *Document, language string, named []string) ([]*Fixture, error) {
	names, err := Names(fsys)
	if err != nil {
		return nil, err
	}
	var selected []*Fixture
	for _, name := range names {
		if len(named) > 0 && !slices.Contains(named, name) {
			continue
		}
		fixture, err := LoadFixture(fsys, doc, name)
		if err != nil {
			return nil, fmt.Errorf("fixture %s: %w", name, err)
		}
		if slices.Contains(fixture.Expect.Languages, language) {
			selected = append(selected, fixture)
		}
	}
	return selected, nil
}

// describe reads the product's name and version from its describe verb.
func describe(ctx context.Context, analyzer Analyzer) (*GapProduct, error) {
	out, stderr, code, err := invoke(ctx, analyzer, "", "describe")
	if err != nil {
		return nil, err
	}
	if code != vocab.ExitClean {
		return nil, fmt.Errorf("describe exited %d: %s", code, strings.TrimSpace(stderr))
	}
	var described struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(out, &described); err != nil {
		return nil, fmt.Errorf("describe: %w", err)
	}
	return &GapProduct{Name: described.Name, Version: described.Version}, nil
}

// invoke runs one verb of the analyzer in dir and returns its standard output,
// its standard error and its exit code; err is a failure to run it at all.
func invoke(ctx context.Context, analyzer Analyzer, dir string, args ...string) (stdout []byte, stderr string, code int, err error) {
	argv := append(slices.Clone(analyzer.Command[1:]), args...)
	cmd := exec.CommandContext(ctx, analyzer.Command[0], argv...) //nolint:gosec // the analyzer under test, named by its caller
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), analyzer.Env...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	runErr := cmd.Run()
	if exit, exited := errors.AsType[*exec.ExitError](runErr); exited {
		return out.Bytes(), errOut.String(), exit.ExitCode(), nil
	}
	if runErr != nil {
		return nil, "", 0, fmt.Errorf("run %s: %w", analyzer.Command[0], runErr)
	}
	return out.Bytes(), errOut.String(), 0, nil
}

type runner struct {
	rows     map[string]bool
	analyzer Analyzer
	options  Options
}

// scratch is one fixture's run: where it is copied and the documents both
// analyses read.
type scratch struct {
	dir       string
	rendering string
	scope     bool
}

func (r *runner) answerFixture(ctx context.Context, fixture *Fixture) FixtureResult {
	row, err := r.answer(ctx, fixture)
	if err != nil {
		row = FixtureResult{
			Fixture: fixture.Name, Result: Fail, Expectations: []ExpectationResult{}, Unexpected: []Unexpected{},
			Message: strings.TrimPrefix(err.Error(), errFixture.Error()+": "),
		}
	}
	if fixture.Expect.Pending != "" {
		row.Result = Pending
	}
	return row
}

func (r *runner) answer(ctx context.Context, fixture *Fixture) (FixtureResult, error) {
	expect := &fixture.Expect
	rendering := fixture.Renderings[r.analyzer.Language]
	dir, err := os.MkdirTemp(r.options.Scratch, "deadset-corpus-")
	if err != nil {
		return FixtureResult{}, err
	}
	defer os.RemoveAll(dir)
	sites, err := Resolve(expect, &rendering.Manifest)
	if err != nil {
		return FixtureResult{}, fixtureError("%v", err)
	}
	at := &scratch{dir: dir, rendering: filepath.Join(dir, renderingDir), scope: len(expect.Consumers) > 0}
	before, err := at.prepare(expect, rendering, r.analyzer.Language)
	if err != nil {
		return FixtureResult{}, err
	}
	if expect.SetupFailure != nil {
		return r.setupFixture(ctx, at, fixture.Name, expect.SetupFailure)
	}
	first, err := r.analyze(ctx, at, "first", slices.ContainsFunc(expect.Expect, func(row Row) bool { return row.Report == ReportNone }))
	if err != nil {
		return FixtureResult{}, err
	}
	rows := slices.SortedFunc(slices.Values(expect.Expect), func(a, b Row) int { return strings.Compare(a.Symbol, b.Symbol) })
	held := make([]ExpectationResult, len(rows))
	for i := range rows {
		actual, message := answerOf(expect, &rows[i], sites, first)
		held[i] = ExpectationResult{Symbol: rows[i].Symbol, Result: Fail, Actual: actual, Message: message}
	}
	written, err := r.suppressionPhase(ctx, at, expect, sites, first, rows, held)
	if err != nil {
		return FixtureResult{}, err
	}
	if written != "" {
		before[path.Join(TargetDir, ignoreFile)] = written
	}
	after, err := digests(at.rendering)
	if err != nil {
		return FixtureResult{}, err
	}
	if !maps.Equal(after, before) {
		return FixtureResult{}, fixtureError("the analyses changed a file of the rendering")
	}
	for i := range rows {
		decide(&rows[i], &held[i], r.options.Gaps, fixture.Name)
	}
	return r.fixtureResult(fixture, sites, first, held), nil
}

// prepare writes the rendering, the configuration document and the scope
// document of one fixture, and returns the digests of the rendering's files.
func (at *scratch) prepare(expect *Expectation, rendering *Rendering, language string) (map[string]string, error) {
	before, err := writeRendering(at.rendering, rendering)
	if err != nil {
		return nil, err
	}
	config, err := configOf(expect, &rendering.Manifest, language)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(at.dir, "deadset.json"), config, 0o600); err != nil {
		return nil, err
	}
	if at.scope {
		if err := os.WriteFile(filepath.Join(at.dir, "scope.json"), scopeOf(expect, at.rendering), 0o600); err != nil {
			return nil, err
		}
	}
	return before, nil
}

// fixtureResult is the fixture's row from its decided expectations and the
// fixture-level members its first report answers.
func (r *runner) fixtureResult(fixture *Fixture, sites map[string]Site, first *analysis, held []ExpectationResult) FixtureResult {
	expect, manifest, gaps := &fixture.Expect, &fixture.Renderings[r.analyzer.Language].Manifest, r.options.Gaps
	unexpected := unexpectedOf(first.report, expect, sites)
	rowsDeclined := slices.ContainsFunc(slices.Concat(conventionNames(expect), expect.DisabledConventions), func(name string) bool {
		return coversFixture(gaps, fixture.Name, name)
	})
	var messages []string
	for _, message := range []string{
		edgeDifferences(expect, manifest, first.report),
		notBuiltDifference(manifest, first.report),
		skipDifferences(expect, manifest, first.report, coversFixture(gaps, fixture.Name, typeErrorCap)),
		noteDifferences(expect, first.report),
		conventionDifferences(expect, first.report, rowsDeclined),
		inEffectDifference(expect, first.report),
	} {
		if message != "" {
			messages = append(messages, message)
		}
	}
	result := Pass
	switch {
	case len(unexpected) > 0 || len(messages) > 0 || slices.ContainsFunc(held, func(row ExpectationResult) bool { return row.Result == Fail }):
		result = Fail
	case rowsDeclined || slices.ContainsFunc(held, func(row ExpectationResult) bool { return row.Result == Gap }):
		result = Gap
	}
	return FixtureResult{Fixture: fixture.Name, Result: result, Expectations: held, Unexpected: unexpected, Message: strings.Join(messages, "; ")}
}

func conventionNames(expect *Expectation) []string {
	names := make([]string, 0, len(expect.ConventionsApplied))
	for _, row := range expect.ConventionsApplied {
		names = append(names, row.Name)
	}
	return names
}

// writeRendering writes the rendering's files below dir, an installed tree under
// the name node_modules, and returns each file's digest by its rendered path.
func writeRendering(dir string, rendering *Rendering) (map[string]string, error) {
	for _, name := range rendering.Sorted() {
		target := filepath.Join(dir, filepath.FromSlash(installedAs(name)))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, rendering.Files[name], 0o600); err != nil {
			return nil, err
		}
	}
	return digests(dir)
}

// installedAs is a rendering path with an installed directory at the rendering
// root or in the target named node_modules, as the corpus layout states.
func installedAs(name string) string {
	for _, prefix := range installedPrefixes {
		if rest, below := strings.CutPrefix(name, prefix); below {
			return strings.TrimSuffix(prefix, InstalledDir+"/") + nodeModules + "/" + rest
		}
	}
	return name
}

// digests is the SHA-256 of every file below dir, by its slash path from dir.
func digests(dir string) (map[string]string, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	tree := root.FS()
	held := map[string]string{}
	err = fs.WalkDir(tree, ".", func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		data, readErr := fs.ReadFile(tree, p)
		if readErr != nil {
			return readErr
		}
		held[p] = fmt.Sprintf("%x", sha256.Sum256(data))
		return nil
	})
	return held, err
}

// configOf is the configuration document the fixture is answered under: its
// target kind and every key its members name, each other key left at its
// default.
func configOf(expect *Expectation, manifest *Manifest, language string) ([]byte, error) {
	doc := map[string]any{"target": map[string]any{"kind": expect.TargetKind}}
	analysis := map[string]any{}
	section := map[string]any{}
	if expect.MinConfidence != "" {
		analysis["min_confidence"] = expect.MinConfidence
	}
	if expect.ConsumerTests != "" {
		analysis["consumer_tests"] = expect.ConsumerTests
	}
	if err := closedWorld(expect, manifest, language, doc, analysis, section); err != nil {
		return nil, err
	}
	if len(analysis) > 0 {
		doc["analysis"] = analysis
	}
	if len(expect.Severity) > 0 {
		doc["severity"] = expect.Severity
	}
	if len(expect.ConfiguredRoots) > 0 {
		patterns := make([]string, 0, len(expect.ConfiguredRoots))
		for _, name := range slices.Sorted(maps.Keys(expect.ConfiguredRoots)) {
			patterns = append(patterns, expect.ConfiguredRoots[name])
		}
		doc["roots"] = map[string]any{"patterns": patterns}
	}
	configuredDeclarations(expect, section)
	if len(expect.EntryFiles) > 0 {
		section["entry_files"] = expect.EntryFiles
	}
	if len(expect.DisabledConventions) > 0 {
		section["disabled_conventions"] = expect.DisabledConventions
	}
	if len(section) > 0 {
		doc[language] = section
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// closedWorld sets the keys each fact of the closed_world member names: the
// consumers complete, the matrix complete over the manifest's configurations,
// and the language's roots_complete where its section has the key.
func closedWorld(expect *Expectation, manifest *Manifest, language string, doc, analysis, section map[string]any) error {
	for _, fact := range expect.ClosedWorld {
		switch fact {
		case WorldConsumers:
			doc["consumers"] = map[string]any{"complete": true}
		case WorldMatrix:
			configurations, err := declaredMatrix(manifest.Configurations)
			if err != nil {
				return err
			}
			analysis["matrix"] = map[string]any{"complete": true}
			analysis["configurations"] = configurations
		case WorldRoots:
			if hasRootsComplete(language) {
				section["roots_complete"] = true
			}
		default:
			return fixtureError("the expectation file declares the closed-world fact %q", fact)
		}
	}
	return nil
}

// configuredDeclarations writes each configured declaration under its key in
// ascending order of logical name, an entry of a lifecycle contract's list as
// one contract of its own holding that entry and its members.
func configuredDeclarations(expect *Expectation, section map[string]any) {
	for _, name := range slices.Sorted(maps.Keys(expect.ConfiguredDeclarations)) {
		declared := expect.ConfiguredDeclarations[name]
		_, key, _ := strings.Cut(declared.Key, ".")
		var entry any = declared.Entry
		if contract, list, lifecycle := strings.Cut(key, "."); lifecycle {
			key = contract
			entry = map[string]any{list: []map[string]string{declared.Entry}, "members": slices.Concat([]string{}, declared.Members)}
		}
		held, _ := section[key].([]any)
		section[key] = append(held, entry)
	}
}

// hasRootsComplete reports whether the configuration schema gives the language a
// roots_complete key, the key a closed-world roots fact sets.
var hasRootsComplete = func() func(string) bool {
	keys := sync.OnceValue(func() map[string]bool {
		held := map[string]bool{}
		schema, err := readSchema(spec.Contract, "contract/config.schema.json")
		if err != nil {
			return held
		}
		properties, _ := schema["properties"].(map[string]any)
		for language, section := range properties {
			if object, ok := section.(map[string]any); ok && objectAt(object, "properties/roots_complete") != nil {
				held[language] = true
			}
		}
		return held
	})
	return func(language string) bool { return keys()[language] }
}()

// declaredMatrix is the platform configurations a manifest's identifiers name:
// the operating system, the architecture and any tags joined by hyphens.
func declaredMatrix(identifiers []string) ([]map[string]any, error) {
	declared := make([]map[string]any, 0, len(identifiers))
	for _, id := range identifiers {
		parts := strings.Split(id, "-")
		if len(parts) < 2 {
			return nil, fixtureError("the manifest declares the configuration %q, and an identifier names an operating system and an architecture", id)
		}
		entry := map[string]any{"id": id, "os": parts[0], "arch": parts[1]}
		if len(parts) > 2 {
			entry["tags"] = parts[2:]
		}
		declared = append(declared, entry)
	}
	return declared, nil
}

// scopeOf is the scope document naming each consumer of the fixture beside the
// target, by absolute path, so the rendering holds no file the runner added.
func scopeOf(expect *Expectation, rendering string) []byte {
	type entry struct {
		Path string `json:"path"`
		Role string `json:"role"`
	}
	doc := struct {
		Target    entry   `json:"target"`
		Consumers []entry `json:"consumers"`
	}{Target: entry{Path: filepath.Join(rendering, TargetDir), Role: "target"}}
	for _, name := range expect.Consumers {
		doc.Consumers = append(doc.Consumers, entry{Path: filepath.Join(rendering, name), Role: "consumer"})
	}
	data, _ := json.Marshal(doc) //nolint:errchkjson // strings only
	return append(data, '\n')
}

// options is the analyzer options every verb of a fixture's run takes.
func (at *scratch) options() []string {
	args := []string{"--target=" + path.Join(renderingDir, TargetDir), "--config=" + filepath.Join(at.dir, "deadset.json")}
	if at.scope {
		args = append(args, "--scope="+filepath.Join(at.dir, "scope.json"))
	}
	return args
}

// analysis is what one analysis of the rendering answered: its report and the
// classes the retained-symbol listing names at each site.
type analysis struct {
	report   *wireReport
	retained map[Site][]string
}

// analyze runs the analyze verb into a report held to the report schema, and the
// print-retained verb where withRetained asks for the listing.
func (r *runner) analyze(ctx context.Context, at *scratch, phase string, withRetained bool) (*analysis, error) {
	held := &analysis{report: &wireReport{}, retained: map[Site][]string{}}
	if err := r.report(ctx, at, phase, held.report); err != nil {
		return nil, err
	}
	if !withRetained {
		return held, nil
	}
	out, stderr, code, err := invoke(ctx, r.analyzer, at.dir, append([]string{"print-retained"}, at.options()...)...)
	if err != nil {
		return nil, err
	}
	if code != vocab.ExitClean && code != vocab.ExitFindings {
		return nil, fixtureError("print-retained exited %d: %s", code, strings.TrimSpace(stderr))
	}
	if held.retained, err = retainedListing(out); err != nil {
		return nil, fixtureError("print-retained: %v", err)
	}
	return held, nil
}

// report runs the analyze verb and decodes the report it wrote, held to the
// report schema.
func (r *runner) report(ctx context.Context, at *scratch, phase string, into *wireReport) error {
	name := filepath.Join(at.dir, phase+".report.json")
	_, stderr, code, err := invoke(ctx, r.analyzer, at.dir, append([]string{"analyze"}, append(at.options(), "--report="+name)...)...)
	if err != nil {
		return err
	}
	if code != vocab.ExitClean && code != vocab.ExitFindings && code != vocab.ExitPending {
		return fixtureError("the %s analysis exited %d: %s", phase, code, strings.TrimSpace(stderr))
	}
	data, err := os.ReadFile(name)
	if err != nil {
		return fixtureError("the %s analysis wrote no report: %v", phase, err)
	}
	if err := ValidateReport(data); err != nil {
		return fixtureError("the %s report does not meet the report schema: %v", phase, err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		return fixtureError("the %s report: %v", phase, err)
	}
	return nil
}

// retainedListing reads print-retained's lines: a reference, the declaration's
// position, then each record's class, site and detail, tab-separated; it returns
// the classes at each declaration's site, sorted.
func retainedListing(out []byte) (map[Site][]string, error) {
	held := map[Site][]string{}
	for line := range strings.SplitSeq(strings.TrimSuffix(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 || (len(fields)-2)%3 != 0 {
			return nil, fmt.Errorf("line %q names no reference, position and record", line)
		}
		site, err := positionSite(fields[1])
		if err != nil {
			return nil, err
		}
		for i := 2; i < len(fields); i += 3 {
			if !slices.Contains(held[site], fields[i]) {
				held[site] = append(held[site], fields[i])
			}
		}
		slices.Sort(held[site])
	}
	return held, nil
}

// positionSite reads a path:line:column position relative to the target root as
// the site it names from the rendering root.
func positionSite(position string) (Site, error) {
	rest, _, withColumn := strings.CutLast(position, ":")
	file, line, withLine := strings.CutLast(rest, ":")
	n, err := strconv.Atoi(line)
	if !withColumn || !withLine || err != nil {
		return Site{}, fmt.Errorf("position %q is no path, line and column", position)
	}
	return Site{File: path.Join(TargetDir, file), Line: n}, nil
}

// setupFixture answers a fixture whose run must end with a setup failure.
func (r *runner) setupFixture(ctx context.Context, at *scratch, name string, want *SetupFailure) (FixtureResult, error) {
	report := filepath.Join(at.dir, "first.report.json")
	_, stderr, code, err := invoke(ctx, r.analyzer, at.dir, append([]string{"analyze"}, append(at.options(), "--report="+report)...)...)
	if err != nil {
		return FixtureResult{}, err
	}
	findings := 0
	if data, err := os.ReadFile(report); err == nil {
		var written wireReport
		if json.Unmarshal(data, &written) == nil {
			findings = len(written.Findings)
		}
	}
	row := FixtureResult{Fixture: name, Result: Pass, Expectations: []ExpectationResult{}, Unexpected: []Unexpected{}}
	if message := setupFailureDifference(want, code, stderr, findings); message != "" {
		row.Result, row.Message = Fail, message
		if coversFixture(r.options.Gaps, name, want.Class) {
			row.Result = Gap
		}
	}
	return row, nil
}

// setupFailureDifference is why one run's end does not answer the fixture's
// setup failure, or "".
func setupFailureDifference(want *SetupFailure, code int, stderr string, findings int) string {
	prefix := setupPrefix + want.Class + ": "
	named := false
	for line := range strings.SplitSeq(stderr, "\n") {
		if strings.HasPrefix(line, prefix) && !slices.ContainsFunc(want.Names, func(name string) bool { return !strings.Contains(line, name) }) {
			named = true
		}
	}
	var held []string
	if code != vocab.ExitFailure {
		held = append(held, fmt.Sprintf("want exit %d, got %d", vocab.ExitFailure, code))
	}
	if findings > 0 {
		held = append(held, fmt.Sprintf("want no finding, got %d", findings))
	}
	if !named {
		quoted, _ := json.Marshal(strings.TrimSpace(stderr)) //nolint:errchkjson // a string
		prefixed, _ := json.Marshal(prefix)                  //nolint:errchkjson // a string
		held = append(held, fmt.Sprintf("want a line starting %s naming %s, got %s", prefixed, strings.Join(want.Names, ", "), quoted))
	}
	return strings.Join(held, "; ")
}

// decide sets one row's outcome: a gap wherever a declared gap covers a
// capability the row exercises, otherwise a pass when the answer matches and the
// suppression phase held, and a fail when either does not.
func decide(row *Row, held *ExpectationResult, gaps []GapRow, fixture string) {
	if held.Message == "" && held.Suppression != nil && (!held.Suppression.Suppressed || held.Suppression.Stale) {
		held.Message = fmt.Sprintf("the report phase agrees and the suppression phase does not: suppressed is %t and stale is %t",
			held.Suppression.Suppressed, held.Suppression.Stale)
	}
	held.Result = Pass
	if held.Message != "" {
		held.Result = Fail
	}
	if capability := covering(gaps, fixture, row); capability != "" {
		held.Result, held.Capability = Gap, capability
		if held.Message == "" {
			held.Message = StaleGap
		}
	}
}
