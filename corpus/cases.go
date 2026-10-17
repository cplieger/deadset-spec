package corpus

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// casesDir is where the command-line vectors live in the spec module's vectors,
// and reportAbsent the outcome of a case that may write no report.
const (
	casesDir     = "vectors/cli"
	reportAbsent = "absent"
)

// Case is one command-line vector: an invocation of a product over a small tree
// and the outcome it must have.
type Case struct {
	ArgvByProduct map[string][]string `json:"argv_by_product"`
	Rename        map[string]string   `json:"rename"`
	Env           map[string]string   `json:"env"`
	Description   string              `json:"description"`
	Pending       string              `json:"pending"`
	Interrupt     string              `json:"interrupt"`
	Products      []string            `json:"products"`
	Argv          []string            `json:"argv"`
	Executables   []string            `json:"executables"`
	Expect        CaseOutcome         `json:"expect"`
}

// CaseOutcome is what a case's run must end with; a member it omits is not
// compared.
type CaseOutcome struct {
	Report         string   `json:"report"`
	StderrContains []string `json:"stderr_contains"`
	FindingPaths   []string `json:"finding_paths"`
	FindingSymbols []string `json:"finding_symbols"`
	Exit           int      `json:"exit"`
	Repeat         int      `json:"repeat"`
}

// CaseResult is one case's result for one product: pass, fail or pending, and
// what differed.
type CaseResult struct {
	Case    string
	Result  string
	Message string
}

// RunCases runs every command-line vector of the vectors tree that names the
// product through the product's command line, each in a fresh copy of its tree.
func RunCases(ctx context.Context, vectors fs.FS, product string, analyzer Analyzer) ([]CaseResult, error) {
	entries, err := fs.ReadDir(vectors, casesDir)
	if err != nil {
		return nil, err
	}
	var results []CaseResult
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		c, err := readCase(vectors, entry.Name())
		if err != nil {
			return nil, err
		}
		if !slices.Contains(c.Products, product) {
			continue
		}
		message, err := runCase(ctx, vectors, entry.Name(), c, product, analyzer)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		result := CaseResult{Case: entry.Name(), Result: Pass, Message: message}
		switch {
		case c.Pending != "":
			result.Result = Pending
		case message != "":
			result.Result = Fail
		}
		results = append(results, result)
	}
	return results, nil
}

// readCase decodes one case's case.json strictly.
func readCase(vectors fs.FS, name string) (*Case, error) {
	data, err := fs.ReadFile(vectors, path.Join(casesDir, name, "case.json"))
	if err != nil {
		return nil, err
	}
	var c Case
	if err := DecodeStrict(data, &c); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &c, nil
}

// runCase runs one case as many times as it repeats and returns the first way a
// run departs from the outcome, or "".
func runCase(ctx context.Context, vectors fs.FS, name string, c *Case, product string, analyzer Analyzer) (string, error) {
	argv := c.Argv
	if own, ok := c.ArgvByProduct[product]; ok {
		argv = own
	}
	for range max(c.Expect.Repeat, 1) {
		message, err := runOnce(ctx, vectors, name, c, argv, analyzer)
		if err != nil || message != "" {
			return message, err
		}
	}
	return "", nil
}

func runOnce(ctx context.Context, vectors fs.FS, name string, c *Case, argv []string, analyzer Analyzer) (string, error) {
	dir, err := os.MkdirTemp("", "deadset-case-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	target, out := filepath.Join(dir, "target"), filepath.Join(dir, "out")
	if mkErr := os.MkdirAll(out, 0o750); mkErr != nil {
		return "", mkErr
	}
	if copyErr := copyTree(vectors, path.Join(casesDir, name, "tree"), target, c); copyErr != nil {
		return "", copyErr
	}
	replace := strings.NewReplacer("@TARGET@", target, "@OUT@", out)
	args := make([]string, len(argv))
	report := ""
	for i, arg := range argv {
		args[i] = replace.Replace(arg)
		if file, ok := strings.CutPrefix(args[i], "--report="); ok {
			report = file
		}
	}
	analyzer.Env = slices.Clone(analyzer.Env)
	for _, name := range slices.Sorted(maps.Keys(c.Env)) {
		analyzer.Env = append(analyzer.Env, name+"="+replace.Replace(c.Env[name]))
	}
	_, stderr, code, err := invoke(ctx, analyzer, target, replace.Replace(c.Interrupt), args...)
	if err != nil {
		return "", err
	}
	return outcomeDifference(&c.Expect, code, stderr, report), nil
}

// copyTree copies a case's tree to dir, naming each file its rename gives it,
// writing @TARGET@ in its text as dir and making each of its executables one.
func copyTree(vectors fs.FS, root, dir string, c *Case) error {
	replace := strings.NewReplacer("@TARGET@", dir)
	return fs.WalkDir(vectors, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(vectors, p)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(p, root+"/")
		if renamed, ok := c.Rename[rel]; ok {
			rel = renamed
		}
		mode := os.FileMode(0o600)
		if slices.Contains(c.Executables, rel) {
			mode = 0o700
		}
		file := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
			return err
		}
		return os.WriteFile(file, []byte(replace.Replace(string(data))), mode)
	})
}

// outcomeDifference is every way one run departs from the outcome, in one line.
func outcomeDifference(want *CaseOutcome, code int, stderr, report string) string {
	var held []string
	if code != want.Exit {
		held = append(held, fmt.Sprintf("exit %d, want %d (stderr %q)", code, want.Exit, strings.TrimSpace(stderr)))
	}
	if n := classLines(stderr); code == vocab.ExitFailure && n != 1 {
		held = append(held, fmt.Sprintf("exit %d with %d class lines, want exactly one (stderr %q)", code, n, strings.TrimSpace(stderr)))
	}
	for _, text := range want.StderrContains {
		if !strings.Contains(stderr, text) {
			held = append(held, fmt.Sprintf("stderr %q holds no %q", strings.TrimSpace(stderr), text))
		}
	}
	if report != "" {
		held = append(held, reportDifference(want, report)...)
	}
	return strings.Join(held, "; ")
}

// reportDifference holds the report file the invocation names to the outcome:
// present and a report schema instance or absent, and its findings' paths and
// symbol names in order.
func reportDifference(want *CaseOutcome, report string) []string {
	data, err := os.ReadFile(report)
	switch {
	case want.Report == reportAbsent && err == nil:
		return []string{"a report was written, want none"}
	case want.Report == reportAbsent:
		return nil
	case err != nil:
		return []string{fmt.Sprintf("no report: %v", err)}
	}
	var held []string
	if err := ValidateReport(data); want.Report == "valid" && err != nil {
		held = append(held, fmt.Sprintf("the report does not meet the report schema: %v", err))
	}
	var findings struct {
		Findings []struct {
			Position struct {
				Path string `json:"path"`
			} `json:"position"`
			Symbol struct {
				Name string `json:"name"`
			} `json:"symbol"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(data, &findings); err != nil {
		return append(held, fmt.Sprintf("the report: %v", err))
	}
	paths, symbols := []string{}, []string{}
	for _, found := range findings.Findings {
		paths = append(paths, found.Position.Path)
		symbols = append(symbols, found.Symbol.Name)
	}
	if want.FindingPaths != nil && !slices.Equal(paths, want.FindingPaths) {
		held = append(held, fmt.Sprintf("finding paths %q, want %q", paths, want.FindingPaths))
	}
	if want.FindingSymbols != nil && !slices.Equal(symbols, want.FindingSymbols) {
		held = append(held, fmt.Sprintf("finding symbols %q, want %q", symbols, want.FindingSymbols))
	}
	return held
}
