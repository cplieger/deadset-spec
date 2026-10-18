package spec_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path"
	"slices"
	"strings"
	"testing"

	spec "github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/corpus"
	"github.com/cplieger/deadset-spec/v8/facts"
)

// updateVectors is the gate that rewrites every family's expected outputs from
// its reference, and updateCommand the invocation a failure names.
const (
	updateVectors = "UPDATE_VECTORS"
	updateCommand = "UPDATE_VECTORS=1 go test -run TestVectorCasesReproduceFromTheirInputs ."
)

// vectorFamily is one published vector family: the report documents a case
// holds, the check that holds a case to the reference that reproduces it, and
// that reference's rendering of the expected output a regeneration writes. A
// family a product alone reproduces has no reference here.
type vectorFamily struct {
	reports  func(fsys fs.FS, dir string) ([][]byte, error)
	problems func(t *testing.T, fsys fs.FS, dir string) []string
	render   func(t *testing.T, dir string) (file string, data []byte, renders bool)
	name     string
	readme   bool
}

// vectorFamilies is every directory of vectors/, in name order.
func vectorFamilies() []vectorFamily {
	return []vectorFamily{
		baselineFamily(),
		{name: "cli", readme: true, problems: cliCaseProblems},
		{name: "config"},
		mergeFamily(),
		sarifFamily(),
		templateFamily(),
	}
}

// vectorCaseDirs is every case directory of one family, in name order.
func vectorCaseDirs(t *testing.T, fsys fs.FS, family string) []string {
	t.Helper()
	root := path.Join("vectors", family)
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		t.Fatalf("Setup: fs.ReadDir(%s): %v", root, err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(dirs) == 0 {
		t.Fatalf("Setup: %s holds no case", root)
	}
	return dirs
}

func TestVectorFamiliesAreEveryDirectoryOfTheVectors(t *testing.T) {
	entries, err := fs.ReadDir(spec.Vectors, "vectors")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	var dirs, families []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	for _, family := range vectorFamilies() {
		families = append(families, family.name)
	}
	if !slices.Equal(dirs, families) {
		t.Errorf("vectors/ holds the families %v, and this suite runs %v", dirs, families)
	}
}

func TestVectorReadmesNameEveryCase(t *testing.T) {
	for _, family := range vectorFamilies() {
		if !family.readme {
			continue
		}
		readme, err := fs.ReadFile(spec.Vectors, path.Join("vectors", family.name, "README.md"))
		if err != nil {
			t.Fatalf("Setup: %v", err)
		}
		for _, dir := range vectorCaseDirs(t, spec.Vectors, family.name) {
			if n := strings.Count(string(readme), "| `"+dir+"` |"); n != 1 {
				t.Errorf("vectors/%s/README.md rows naming %s = %d, want 1", family.name, dir, n)
			}
		}
	}
}

func TestVectorReportsAreInstancesOfTheReportSchema(t *testing.T) {
	for _, family := range vectorFamilies() {
		if family.reports == nil {
			continue
		}
		for _, dir := range vectorCaseDirs(t, spec.Vectors, family.name) {
			t.Run(family.name+"/"+dir, func(t *testing.T) {
				documents, err := family.reports(spec.Vectors, dir)
				if err != nil {
					t.Fatalf("Setup: %v", err)
				}
				if len(documents) == 0 {
					t.Fatalf("vectors/%s/%s holds no report document", family.name, dir)
				}
				for _, document := range documents {
					validateAgainst(t, reportSchemaPath, document)
				}
			})
		}
	}
}

// TestVectorCasesReproduceFromTheirInputs holds every case of every family with a
// reference here to what the reference makes of its inputs, and under
// UPDATE_VECTORS=1 rewrites each rendered case's expected output instead, so a
// change to a rule text or a reference regenerates the vectors as one reviewed
// diff.
func TestVectorCasesReproduceFromTheirInputs(t *testing.T) {
	update := os.Getenv(updateVectors) == "1"
	for _, family := range vectorFamilies() {
		if family.problems == nil {
			continue
		}
		for _, dir := range vectorCaseDirs(t, spec.Vectors, family.name) {
			t.Run(family.name+"/"+dir, func(t *testing.T) {
				if update && family.render != nil {
					if file, data, renders := family.render(t, dir); renders {
						if err := os.WriteFile(path.Join("vectors", family.name, dir, file), data, 0o644); err != nil {
							t.Fatalf("write %s: %v", file, err)
						}
					}
					return
				}
				for _, problem := range family.problems(t, spec.Vectors, dir) {
					t.Errorf("vectors/%s/%s: %s (regenerate with %s)", family.name, dir, problem, updateCommand)
				}
			})
		}
	}
}

// cliCaseProblems holds one command-line case to its layout: a case.json that
// decodes strictly, names products the Contract's command-line surface lists and
// an exit code of the table, and a tree holding every file it renames or makes
// executable, each describe document of which a run of this contract admits,
// and each report and facts document of which the Contract's schemas admit.
func cliCaseProblems(t *testing.T, fsys fs.FS, dir string) []string {
	t.Helper()
	at := path.Join("vectors/cli", dir)
	data, err := fs.ReadFile(fsys, path.Join(at, "case.json"))
	if err != nil {
		return []string{err.Error()}
	}
	var c corpus.Case
	if err := corpus.DecodeStrict(data, &c); err != nil {
		return []string{"case.json: " + err.Error()}
	}
	var problems []string
	products := cliProducts(t)
	for _, product := range append(slices.Clone(c.Products), slices.Collect(maps.Keys(c.ArgvByProduct))...) {
		if !slices.Contains(products, product) {
			problems = append(problems, "names the product "+product+", which contract/cli.json does not list")
		}
	}
	if c.Description == "" || len(c.Products) == 0 || len(c.Argv) == 0 {
		problems = append(problems, "case.json names no description, product or argv")
	}
	if !slices.Contains(mergeExitCodes(t), c.Expect.Exit) {
		problems = append(problems, "expects an exit code outside contract/exit-codes.json")
	}
	if c.Expect.Report != "" && c.Expect.Report != "valid" && c.Expect.Report != "absent" {
		problems = append(problems, "expect.report is "+c.Expect.Report+", want valid or absent")
	}
	for from := range c.Rename {
		if _, err := fs.Stat(fsys, path.Join(at, "tree", from)); err != nil {
			problems = append(problems, "renames "+from+", which the tree does not hold")
		}
	}
	tree := path.Join(at, "tree")
	held := map[string]bool{}
	walkErr := fs.WalkDir(fsys, tree, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(p, tree+"/")
		if to, ok := c.Rename[rel]; ok {
			rel = to
		}
		held[rel] = true
		switch {
		case path.Base(rel) == "describe.json":
			problems = append(problems, describedProblems(t, fsys, p)...)
		case strings.HasPrefix(rel, "bin/") && path.Base(rel) == "report.json":
			problems = append(problems, writtenProblems(fsys, p, func(data []byte) []string { return validationErrors(reportSchemaPath, data) })...)
		case strings.HasSuffix(rel, ".facts.json"):
			problems = append(problems, writtenProblems(fsys, p, func(data []byte) []string {
				if _, err := facts.Decode(data); err != nil {
					return []string{err.Error()}
				}
				return nil
			})...)
		}
		return nil
	})
	if walkErr != nil {
		problems = append(problems, walkErr.Error())
	}
	for _, file := range c.Executables {
		if !held[file] {
			problems = append(problems, "makes "+file+" executable, which the tree does not hold")
		}
	}
	return problems
}

// describedProblems holds a describe document a case's stub analyzer prints to
// the describe schema and to the report schema versions this contract accepts,
// so a stub stays admitted when the contract moves.
func describedProblems(t *testing.T, fsys fs.FS, file string) []string {
	t.Helper()
	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		return []string{err.Error()}
	}
	var problems []string
	for _, line := range validationErrors(describeSchemaPath, data) {
		problems = append(problems, file+": "+line)
	}
	var described struct {
		SchemaVersionsAccepted []string `json:"schema_versions_accepted"`
	}
	if err := json.Unmarshal(data, &described); err != nil {
		return append(problems, file+": "+err.Error())
	}
	accepted := loadContract(t).SchemaVersions
	for _, version := range described.SchemaVersionsAccepted {
		if !slices.Contains(accepted, version) {
			problems = append(problems, file+": reads the schema version "+version+", which contract/contract.json does not list")
		}
	}
	return problems
}

// writtenProblems is every refusal of a document a case's stub analyzer writes,
// each naming the file.
func writtenProblems(fsys fs.FS, file string, refusals func([]byte) []string) []string {
	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		return []string{err.Error()}
	}
	var problems []string
	for _, line := range refusals(data) {
		problems = append(problems, file+": "+line)
	}
	return problems
}

// cliProducts is every product contract/cli.json lists.
func cliProducts(t *testing.T) []string {
	t.Helper()
	data, err := fs.ReadFile(spec.Contract, "contract/cli.json")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	var doc struct {
		Products []struct {
			Product string `json:"product"`
		} `json:"products"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	var names []string
	for _, product := range doc.Products {
		names = append(names, product.Product)
	}
	return names
}

// indented is a value encoded as the published vectors are: two-space
// indentation, HTML left unescaped, and a final newline.
func indented(t *testing.T, value any) []byte {
	t.Helper()
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return out.Bytes()
}
