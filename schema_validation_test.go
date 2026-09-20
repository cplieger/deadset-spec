package spec_test

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
	"sync"
	"testing"

	"github.com/cplieger/deadset-spec/v2"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

const (
	examplesDir   = "examples"
	negativeIndex = "index.json"

	// schemaBase is the base URI every embedded schema is registered under.
	// A relative reference in one schema, which is how report.schema.json
	// reaches finding.schema.json, resolves against it to the sibling of the
	// same embedded tree rather than to the network.
	schemaBase = "mem:///"

	// schemaSuffix names the embedded documents that are JSON Schemas.
	schemaSuffix = ".schema.json"
)

// exampleFindingKinds is not a list: a finding example is named for the kind it
// carries and the family set comes from kinds.json, so this file names no code.
// reportEnvelopeStates is the closed set of envelope states the example set
// covers, one report each, because a state is a property of the envelope rather
// than of a vocabulary any document declares.
var reportEnvelopeStates = []string{
	"clean",
	"configuration-not-built",
	"declared-gaps",
	"findings",
	"merged",
	"pending",
	"stale-suppressions",
}

// compiledSchema is one entry of the schema cache: the schema compiled from an
// embedded path, or the reason it did not compile.
type compiledSchema struct {
	schema *jsonschema.Schema
	err    error
}

var (
	schemaCacheMu sync.Mutex
	schemaCache   = map[string]*compiledSchema{}
)

// contractSchema compiles the JSON Schema at an embedded path once per path and
// hands every later caller the same value, so a suite validating hundreds of
// documents in parallel compiles each schema once. The path is a path into the
// embedded trees, "contract/report.schema.json" or "corpus/expect.schema.json".
func contractSchema(schemaPath string) (*jsonschema.Schema, error) {
	schemaCacheMu.Lock()
	defer schemaCacheMu.Unlock()
	if entry, ok := schemaCache[schemaPath]; ok {
		return entry.schema, entry.err
	}
	schema, err := compileEmbeddedSchema(schemaPath)
	schemaCache[schemaPath] = &compiledSchema{schema: schema, err: err}
	return schema, err
}

// compileEmbeddedSchema registers every embedded schema under one base URI and
// compiles the one at schemaPath, so a cross-file reference resolves to the
// embedded sibling.
func compileEmbeddedSchema(schemaPath string) (*jsonschema.Schema, error) {
	documents, err := embeddedSchemas()
	if err != nil {
		return nil, err
	}
	if _, ok := documents[schemaPath]; !ok {
		return nil, fmt.Errorf("no schema is embedded at %q", schemaPath)
	}
	compiler := jsonschema.NewCompiler()
	for _, p := range slices.Sorted(maps.Keys(documents)) {
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(documents[p]))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if err = compiler.AddResource(schemaBase+p, document); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
	}
	return compiler.Compile(schemaBase + schemaPath)
}

// embeddedSchemas reads every JSON Schema of the contract and corpus trees,
// keyed by its embedded path. Both trees are read because the corpus schemas
// live in spec.Corpus and the contract schemas in spec.Contract, and a caller
// names a schema by one path whichever tree carries it.
func embeddedSchemas() (map[string][]byte, error) {
	documents := map[string][]byte{}
	for _, tree := range []fs.FS{spec.Contract, spec.Corpus} {
		err := fs.WalkDir(tree, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(p, schemaSuffix) {
				return err
			}
			documents[p], err = fs.ReadFile(tree, p)
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	return documents, nil
}

// validationProblem is one violation a validator reported: where in the
// instance it is, which keyword of which schema refused it, and what that
// keyword said.
type validationProblem struct {
	InstanceLocation string
	KeywordLocation  string
	Message          string
}

// names reports whether the problem was raised by the named JSON Schema
// keyword. A keyword usually appears as the last token of the keyword
// location; a failed "not" is reported at the location of the subschema it
// negates and names itself in the message instead.
func (p validationProblem) names(keyword string) bool {
	return strings.Contains(p.KeywordLocation, "/"+keyword) || strings.Contains(p.Message, keyword)
}

func (p validationProblem) String() string {
	at := p.InstanceLocation
	if at == "" {
		at = "the whole document"
	}
	return fmt.Sprintf("%s: %s (%s)", at, p.Message, p.KeywordLocation)
}

// validationProblems validates document against the embedded schema at
// schemaPath and returns one problem per violation. A schema that will not
// compile and a document that is not JSON are returned as an error, so a
// caller never reads a broken setup as a clean document.
func validationProblems(schemaPath string, document []byte) ([]validationProblem, error) {
	schema, err := contractSchema(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("compiling %s: %w", schemaPath, err)
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(document))
	if err != nil {
		return nil, fmt.Errorf("decoding the instance: %w", err)
	}
	err = schema.Validate(instance)
	if err == nil {
		return nil, nil
	}
	var invalid *jsonschema.ValidationError
	if !errors.As(err, &invalid) {
		return nil, err
	}
	return leafProblems(invalid.DetailedOutput()), nil
}

// leafProblems walks a detailed output into one problem per unit that carries
// an error, which is the unit that names the keyword the instance failed and
// the location it failed at.
func leafProblems(unit *jsonschema.OutputUnit) []validationProblem {
	var problems []validationProblem
	if unit.Error != nil {
		problems = append(problems, validationProblem{
			InstanceLocation: unit.InstanceLocation,
			KeywordLocation:  unit.KeywordLocation,
			Message:          unit.Error.String(),
		})
	}
	for i := range unit.Errors {
		problems = append(problems, leafProblems(&unit.Errors[i])...)
	}
	return problems
}

// validationErrors returns one line per violation of the embedded schema at
// schemaPath by document, each naming the instance location, what the
// validator said and the keyword that refused it. A valid document yields
// nothing, and a schema that will not compile or a document that is not JSON
// yields one line naming that failure rather than nothing.
func validationErrors(schemaPath string, document []byte) []string {
	problems, err := validationProblems(schemaPath, document)
	if err != nil {
		return []string{err.Error()}
	}
	lines := make([]string, len(problems))
	for i, problem := range problems {
		lines[i] = problem.String()
	}
	return lines
}

// validateAgainst fails the test once per violation when document is not an
// instance of the embedded schema at schemaPath.
func validateAgainst(t *testing.T, schemaPath string, document []byte) {
	t.Helper()
	for _, line := range validationErrors(schemaPath, document) {
		t.Errorf("validateAgainst(%q) reports a violation: %s", schemaPath, line)
	}
}

// exampleFiles lists the JSON documents of one examples subdirectory, in name
// order, with the negatives index left out because it is the index rather than
// an example.
func exampleFiles(t *testing.T, dir string) []string {
	t.Helper()
	at := path.Join(examplesDir, dir)
	entries, err := fs.ReadDir(spec.Examples, at)
	if err != nil {
		t.Fatalf("Setup: fs.ReadDir(%q): %v", at, err)
	}
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") || name == negativeIndex {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		t.Fatalf("Setup: fs.ReadDir(%q) found no example, want at least one", at)
	}
	slices.Sort(names)
	return names
}

// readExample reads one example document.
func readExample(t *testing.T, dir, name string) []byte {
	t.Helper()
	at := path.Join(examplesDir, dir, name)
	data, err := fs.ReadFile(spec.Examples, at)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(%q): %v", at, err)
	}
	return data
}

// exampleName is the example's name without its extension, which is the kind
// or the envelope state the document is named for and a name -run selects.
func exampleName(file string) string {
	return strings.TrimSuffix(file, ".json")
}

// exampleFinding mirrors the fields of a finding example that must agree with
// the file that carries it and with the code space.
type exampleFinding struct {
	Code string `json:"code"`
	Kind string `json:"kind"`
}

// exampleReport mirrors the parts of a report envelope that its totals count,
// so a count and the array it counts are compared, and carries every finding
// the envelope holds so each is validated on its own as well.
type exampleReport struct {
	Findings        []json.RawMessage `json:"findings"`
	EdgeEvaluations []struct {
		Finding json.RawMessage `json:"finding"`
		State   string          `json:"state"`
	} `json:"edge_evaluations"`
	StaleSuppressions []json.RawMessage `json:"stale_suppressions"`
	Consumers         struct {
		Loaded      []json.RawMessage `json:"loaded"`
		Unavailable []json.RawMessage `json:"unavailable"`
		Declared    int               `json:"declared"`
	} `json:"consumers"`
	Totals struct {
		Findings          int `json:"findings"`
		StaleSuppressions int `json:"stale_suppressions"`
		Pending           int `json:"pending"`
		Omitted           int `json:"omitted"`
	} `json:"totals"`
	Configurations []struct {
		ID string `json:"id"`
	} `json:"configurations"`
	ConfigurationsNotBuilt []struct {
		ID string `json:"id"`
	} `json:"configurations_not_built"`
}

// negativeRow is one row of examples/negatives/index.json.
type negativeRow struct {
	File         string `json:"file"`
	Schema       string `json:"schema"`
	Constraint   string `json:"constraint"`
	InstancePath string `json:"instance_path"`
	Violates     string `json:"violates"`
}

// loadNegativeIndex decodes the negatives index; an unknown key fails the
// decode, so a row cannot carry a field this suite does not read.
func loadNegativeIndex(t *testing.T) []negativeRow {
	t.Helper()
	data := readExample(t, "negatives", negativeIndex)
	var doc struct {
		Description string        `json:"description"`
		Negatives   []negativeRow `json:"negatives"`
	}
	if err := decodeStrict(data, &doc); err != nil {
		t.Fatalf("Setup: decodeStrict(%q): %v", negativeIndex, err)
	}
	if len(doc.Negatives) == 0 {
		t.Fatalf("Setup: %s names no negative, want at least one", negativeIndex)
	}
	return doc.Negatives
}

func TestFindingExamplesAreInstancesOfTheFindingSchema(t *testing.T) {
	for _, file := range exampleFiles(t, "findings") {
		t.Run(exampleName(file), func(t *testing.T) {
			validateAgainst(t, findingSchemaPath, readExample(t, "findings", file))
		})
	}
}

func TestFindingExamplesAreNamedForTheKindTheyCarry(t *testing.T) {
	for _, file := range exampleFiles(t, "findings") {
		t.Run(exampleName(file), func(t *testing.T) {
			var finding exampleFinding
			if err := json.Unmarshal(readExample(t, "findings", file), &finding); err != nil {
				t.Fatalf("Setup: json.Unmarshal(%q): %v", file, err)
			}
			if finding.Kind != exampleName(file) {
				t.Errorf("%s carries kind %q, want the file's own name %q", file, finding.Kind, exampleName(file))
			}
		})
	}
}

func TestEveryLiveIssueKindFamilyHasOneFindingExample(t *testing.T) {
	doc := loadKinds(t)
	byFamily := map[string][]string{}
	for _, file := range exampleFiles(t, "findings") {
		var finding exampleFinding
		if err := json.Unmarshal(readExample(t, "findings", file), &finding); err != nil {
			t.Fatalf("Setup: json.Unmarshal(%q): %v", file, err)
		}
		families := exampleFamilies(doc, finding.Code)
		if len(families) != 1 {
			t.Errorf("%s carries code %q, which sits in %d live range(s) %v, want exactly one",
				file, finding.Code, len(families), families)
			continue
		}
		byFamily[families[0]] = append(byFamily[families[0]], file)
	}
	for _, r := range doc.Ranges {
		if isRetired(r) {
			continue
		}
		if got := byFamily[r.Family]; len(got) != 1 {
			t.Errorf("the %q family has %d finding example(s) %v, want exactly one", r.Family, len(got), got)
		}
	}
}

// exampleFamilies names the live ranges of the code space that contain code.
func exampleFamilies(doc kindsDocument, code string) []string {
	var families []string
	for _, r := range rangesContaining(doc.Ranges, codeNumber(code)) {
		if !isRetired(r) {
			families = append(families, r.Family)
		}
	}
	return families
}

func TestReportExamplesAreInstancesOfTheReportSchema(t *testing.T) {
	for _, file := range exampleFiles(t, "reports") {
		t.Run(exampleName(file), func(t *testing.T) {
			validateAgainst(t, reportSchemaPath, readExample(t, "reports", file))
		})
	}
}

func TestEveryFindingInsideAReportExampleIsAnInstanceOfTheFindingSchema(t *testing.T) {
	for _, file := range exampleFiles(t, "reports") {
		t.Run(exampleName(file), func(t *testing.T) {
			report := loadReportExample(t, file)
			for i, finding := range report.Findings {
				t.Run(fmt.Sprintf("findings_%d", i), func(t *testing.T) {
					validateAgainst(t, findingSchemaPath, finding)
				})
			}
			for i, evaluation := range report.EdgeEvaluations {
				if len(evaluation.Finding) == 0 {
					continue
				}
				t.Run(fmt.Sprintf("edge_evaluations_%d", i), func(t *testing.T) {
					validateAgainst(t, findingSchemaPath, evaluation.Finding)
				})
			}
		})
	}
}

func TestEveryReportEnvelopeStateHasOneExample(t *testing.T) {
	present := map[string]bool{}
	for _, file := range exampleFiles(t, "reports") {
		present[exampleName(file)] = true
	}
	for _, state := range reportEnvelopeStates {
		if !present[state] {
			t.Errorf("no report example for the %q envelope state, want %s.json", state, state)
		}
	}
}

func TestReportExampleTotalsCountTheirOwnArrays(t *testing.T) {
	for _, file := range exampleFiles(t, "reports") {
		t.Run(exampleName(file), func(t *testing.T) {
			report := loadReportExample(t, file)
			if got, want := report.Totals.Findings, len(report.Findings)+report.Totals.Omitted; got != want {
				t.Errorf("%s totals.findings = %d, want len(findings) plus omitted, %d", file, got, want)
			}
			if got, want := report.Totals.StaleSuppressions, len(report.StaleSuppressions); got != want {
				t.Errorf("%s totals.stale_suppressions = %d, want len(stale_suppressions), %d", file, got, want)
			}
			if got, want := report.Totals.Pending, countDeadEvaluations(report); got != want {
				t.Errorf("%s totals.pending = %d, want the number of dead edge evaluations, %d", file, got, want)
			}
			consumers := report.Consumers
			if got, want := consumers.Declared, len(consumers.Loaded)+len(consumers.Unavailable); got != want {
				t.Errorf("%s consumers.declared = %d, want len(loaded) plus len(unavailable), %d", file, got, want)
			}
		})
	}
}

// TestReportExampleConfigurationsNotBuiltNameNoBuiltConfiguration pins the one
// rule the report schema states about the two configuration arrays and JSON
// Schema cannot express: a configuration is either in the matrix the analysis
// ran or dropped from it, so an identifier in one array is in neither the other
// nor a finding's configuration list.
func TestReportExampleConfigurationsNotBuiltNameNoBuiltConfiguration(t *testing.T) {
	for _, file := range exampleFiles(t, "reports") {
		t.Run(exampleName(file), func(t *testing.T) {
			report := loadReportExample(t, file)
			built := map[string]bool{}
			for _, c := range report.Configurations {
				built[c.ID] = true
			}
			for _, c := range report.ConfigurationsNotBuilt {
				if built[c.ID] {
					t.Errorf("%s names configuration %q as built and as not built, want the identifier in one array only", file, c.ID)
				}
			}
		})
	}
}

// countDeadEvaluations is the number of edge evaluations whose state is dead,
// which is the number of pending findings a report holds.
func countDeadEvaluations(report exampleReport) int {
	dead := 0
	for _, evaluation := range report.EdgeEvaluations {
		if evaluation.State == "dead" {
			dead++
		}
	}
	return dead
}

// loadReportExample decodes one report example into the fields this suite
// compares.
func loadReportExample(t *testing.T, file string) exampleReport {
	t.Helper()
	var report exampleReport
	if err := json.Unmarshal(readExample(t, "reports", file), &report); err != nil {
		t.Fatalf("Setup: json.Unmarshal(%q): %v", file, err)
	}
	return report
}

func TestNegativeExamplesAreRefusedAtTheirNamedConstraint(t *testing.T) {
	for _, row := range loadNegativeIndex(t) {
		t.Run(exampleName(row.File), func(t *testing.T) {
			problems, err := validationProblems(row.Schema, readExample(t, "negatives", row.File))
			if err != nil {
				t.Fatalf("Setup: validationProblems(%q, %q): %v", row.Schema, row.File, err)
			}
			if len(problems) == 0 {
				t.Fatalf("validationProblems(%q, %q) reports no violation, want the %s constraint at %q",
					row.Schema, row.File, row.Constraint, row.InstancePath)
			}
			locations := map[string]bool{}
			named := false
			for _, problem := range problems {
				locations[problem.InstanceLocation] = true
				if problem.InstanceLocation == row.InstancePath && problem.names(row.Constraint) {
					named = true
				}
			}
			if got := slices.Sorted(maps.Keys(locations)); len(got) != 1 || got[0] != row.InstancePath {
				t.Errorf("validationProblems(%q, %q) reports violations at %v, want exactly one location, %q",
					row.Schema, row.File, got, row.InstancePath)
			}
			if !named {
				t.Errorf("validationProblems(%q, %q) reports %v, want the %s constraint at %q",
					row.Schema, row.File, problems, row.Constraint, row.InstancePath)
			}
		})
	}
}

func TestEveryNegativeExampleHasOneIndexRow(t *testing.T) {
	rows := loadNegativeIndex(t)
	named := map[string]int{}
	for _, row := range rows {
		named[row.File]++
	}
	for _, file := range exampleFiles(t, "negatives") {
		if named[file] != 1 {
			t.Errorf("%s has %d row(s) in %s, want exactly one", file, named[file], negativeIndex)
		}
	}
	present := map[string]bool{}
	for _, file := range exampleFiles(t, "negatives") {
		present[file] = true
	}
	for _, row := range rows {
		if !present[row.File] {
			t.Errorf("%s names %q, which is not a document of the negatives directory", negativeIndex, row.File)
		}
		if row.Violates == "" {
			t.Errorf("%s row for %q states no violation, want one sentence", negativeIndex, row.File)
		}
	}
}

func TestCorpusFixtureDocumentsAreInstancesOfTheCorpusSchemas(t *testing.T) {
	names, err := fixtureNames(spec.Corpus)
	if err != nil {
		t.Fatalf("Setup: fixtureNames(Corpus): %v", err)
	}
	if len(names) == 0 {
		t.Fatal("Setup: the corpus holds no fixture, want at least one")
	}
	for _, name := range slices.Sorted(maps.Keys(names)) {
		dir := path.Join(fixturesDir, name)
		t.Run(subtestName(name), func(t *testing.T) {
			expect, err := fs.ReadFile(spec.Corpus, path.Join(dir, expectFile))
			if err != nil {
				t.Fatalf("Setup: fs.ReadFile(Corpus, %q): %v", path.Join(dir, expectFile), err)
			}
			validateAgainst(t, expectSchemaPath, expect)
			for _, language := range []string{"go", "ts"} {
				files, err := renderingFiles(spec.Corpus, dir, language)
				if err != nil {
					continue
				}
				manifest, ok := files[manifestFile]
				if !ok {
					t.Errorf("the %s rendering of %s holds no %s, want the rendering manifest", language, name, manifestFile)
					continue
				}
				t.Run(language, func(t *testing.T) {
					validateAgainst(t, fixtureSchemaPath, manifest)
				})
			}
		})
	}
}

func TestConfigVectorExpectationsAreInstancesOfTheConfigSchema(t *testing.T) {
	validated := 0
	for _, name := range configVectorDirs(t) {
		at := path.Join(configVectorsDir, name, vectorExpectedFile)
		data, err := fs.ReadFile(spec.Vectors, at)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatalf("Setup: fs.ReadFile(Vectors, %q): %v", at, err)
		}
		validated++
		t.Run(subtestName(name), func(t *testing.T) {
			validateAgainst(t, configSchemaPath, data)
		})
	}
	if validated == 0 {
		t.Errorf("no case under %s carries a %s, want at least one resolved configuration to validate",
			configVectorsDir, vectorExpectedFile)
	}
}

func TestValidationErrorsNamesASchemaThatIsNotEmbedded(t *testing.T) {
	const absent = "contract/absent.schema.json"
	lines := validationErrors(absent, []byte(`{}`))
	if len(lines) != 1 || !strings.Contains(lines[0], absent) {
		t.Errorf("validationErrors(%q, `{}`) = %v, want one line naming the absent schema", absent, lines)
	}
}
