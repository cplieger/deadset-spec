package spec_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"text/template"
	"text/template/parse"
	"unicode/utf8"

	spec "github.com/cplieger/deadset-spec/v7"
)

// templateVectorsDir holds one directory per published template case.
const templateVectorsDir = "vectors/template"

// The files of a template case: the template, the report it renders, and either the
// output it renders or the exit code it ends the run with.
const (
	templateFile         = "template.tmpl"
	templateReportFile   = "report.json"
	templateExpectedFile = "expected.txt"
	templateExitFile     = "expected_exit"
)

// The two exit codes a template case can end with: a template refused before any
// analysis, and a rendering that failed after the report was written.
const (
	templateRefusedExit = 2
	templateFailedExit  = 3
)

// templateFunctions are the functions of the subset grammar/template.md admits.
var templateFunctions = []string{
	"and", "or", "not", "len", "index", "eq", "ne", "lt", "le", "gt", "ge",
	"print", "printf", "println",
}

// errTemplateSubset is a template text/template parses and the subset refuses.
var errTemplateSubset = errors.New("outside the subset")

// templateCase is one published case, read from its directory.
type templateCase struct {
	source   string
	report   []byte
	expected []byte
	exit     int
}

// referenceTemplate parses one template as grammar/template.md states, refusing every
// form outside the subset with errTemplateSubset.
func referenceTemplate(source string) (*template.Template, error) {
	parsed, err := template.New("case").Option("missingkey=error").Funcs(template.FuncMap{
		"index":  templateIndex,
		"printf": templatePrintf,
	}).Parse(source)
	if err != nil {
		return nil, err
	}
	if len(parsed.Templates()) != 1 {
		return nil, fmt.Errorf("%w: a template definition", errTemplateSubset)
	}
	if err := templateSubset(parsed.Root); err != nil {
		return nil, err
	}
	return parsed, nil
}

// templateSubset walks a parsed tree and refuses the forms the subset leaves out.
func templateSubset(node parse.Node) error {
	switch n := node.(type) {
	case *parse.ListNode:
		if n == nil {
			return nil
		}
		for _, child := range n.Nodes {
			if err := templateSubset(child); err != nil {
				return err
			}
		}
	case *parse.ActionNode:
		return templateSubset(n.Pipe)
	case *parse.IfNode:
		return templateBranch(&n.BranchNode)
	case *parse.WithNode:
		return templateBranch(&n.BranchNode)
	case *parse.RangeNode:
		return templateBranch(&n.BranchNode)
	case *parse.PipeNode:
		if n == nil {
			return nil
		}
		for _, command := range n.Cmds {
			for _, arg := range command.Args {
				if err := templateSubset(arg); err != nil {
					return err
				}
			}
		}
	case *parse.ChainNode:
		return templateSubset(n.Node)
	case *parse.IdentifierNode:
		if !slices.Contains(templateFunctions, n.Ident) {
			return fmt.Errorf("%w: the function %s", errTemplateSubset, n.Ident)
		}
	case *parse.NumberNode:
		return templateNumber(n.Text)
	case *parse.StringNode:
		return templateString(n.Quoted)
	case *parse.TemplateNode:
		return fmt.Errorf("%w: a template action", errTemplateSubset)
	}
	return nil
}

// templateBranch walks the pipeline and both lists of an if, a with or a range.
func templateBranch(n *parse.BranchNode) error {
	for _, part := range []parse.Node{n.Pipe, n.List, n.ElseList} {
		if err := templateSubset(part); err != nil {
			return err
		}
	}
	return nil
}

// templateNumber refuses every number constant but a decimal integer with an optional
// sign and no leading zero, a character constant included.
func templateNumber(text string) error {
	digits := strings.TrimLeft(text, "+-")
	if digits == "" || strings.Trim(digits, "0123456789") != "" || (len(digits) > 1 && digits[0] == '0') {
		return fmt.Errorf("%w: the number %s", errTemplateSubset, text)
	}
	return nil
}

// templateString refuses an interpreted string holding an octal escape.
func templateString(quoted string) error {
	if strings.HasPrefix(quoted, "`") {
		return nil
	}
	for i := 0; i < len(quoted)-1; i++ {
		if quoted[i] != '\\' {
			continue
		}
		if next := quoted[i+1]; next >= '0' && next <= '7' {
			return fmt.Errorf("%w: an octal escape in %s", errTemplateSubset, quoted)
		}
		i++
	}
	return nil
}

// templateData decodes a report as the template reads it: every number an integer.
func templateData(document []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(document))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return templateIntegers(value)
}

// templateIntegers replaces every decoded number with the integer it spells.
func templateIntegers(value any) (any, error) {
	switch v := value.(type) {
	case json.Number:
		return strconv.ParseInt(v.String(), 10, 64)
	case []any:
		for i, element := range v {
			converted, err := templateIntegers(element)
			if err != nil {
				return nil, err
			}
			v[i] = converted
		}
	case map[string]any:
		for name, member := range v {
			converted, err := templateIntegers(member)
			if err != nil {
				return nil, err
			}
			v[name] = converted
		}
	}
	return value, nil
}

// templateIndex is index as the subset states it: a key outside the array or a member
// the object does not carry fails.
func templateIndex(value any, keys ...any) (any, error) {
	for _, key := range keys {
		switch held := value.(type) {
		case []any:
			at, ok := templateInteger(key)
			if !ok || at < 0 || at >= int64(len(held)) {
				return nil, fmt.Errorf("index %v out of range", key)
			}
			value = held[at]
		case map[string]any:
			name, ok := key.(string)
			member, present := held[name]
			if !ok || !present {
				return nil, fmt.Errorf("no member %v", key)
			}
			value = member
		default:
			return nil, fmt.Errorf("index of %v into %T", key, value)
		}
	}
	return value, nil
}

// templateInteger is an integer operand, a document's and a constant's alike.
func templateInteger(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	}
	return 0, false
}

// templatePrintf is printf as the subset states it: six verbs, no flag, width or
// precision, and every operand taken.
func templatePrintf(format string, operands ...any) (string, error) {
	var out strings.Builder
	next := 0
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			out.WriteByte(format[i])
			continue
		}
		if i+1 >= len(format) {
			return "", errors.New("printf: a verb with no character")
		}
		i++
		verb := format[i]
		if verb == '%' {
			out.WriteByte('%')
			continue
		}
		if next >= len(operands) {
			return "", fmt.Errorf("printf: %%%c has no operand", verb)
		}
		operand := operands[next]
		next++
		switch verb {
		case 'v', 's':
			fmt.Fprint(&out, operand)
		case 'd':
			n, ok := templateInteger(operand)
			if !ok {
				return "", fmt.Errorf("printf: %%d over %T", operand)
			}
			out.WriteString(strconv.FormatInt(n, 10))
		case 't':
			b, ok := operand.(bool)
			if !ok {
				return "", fmt.Errorf("printf: %%t over %T", operand)
			}
			out.WriteString(strconv.FormatBool(b))
		case 'q':
			out.WriteString(templateQuote(fmt.Sprint(operand)))
		default:
			return "", fmt.Errorf("printf: the verb %%%c is not in the subset", verb)
		}
	}
	if next < len(operands) {
		return "", fmt.Errorf("printf: %d operand(s) left over", len(operands)-next)
	}
	return out.String(), nil
}

// templateQuote is the %q quoting of grammar/template.md: ASCII escapes only, every
// character from U+0080 up written as itself.
func templateQuote(text string) string {
	named := map[rune]string{'"': `\"`, '\\': `\\`, '\a': `\a`, '\b': `\b`, '\f': `\f`, '\n': `\n`, '\r': `\r`, '\t': `\t`, '\v': `\v`}
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range text {
		switch escaped, ok := named[r]; {
		case ok:
			out.WriteString(escaped)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&out, `\x%02x`, r)
		default:
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
	return out.String()
}

// renderTemplateCase runs one case and returns the output or the exit code it ends with.
func renderTemplateCase(c templateCase) ([]byte, int) {
	parsed, err := referenceTemplate(c.source)
	if err != nil {
		return nil, templateRefusedExit
	}
	data, err := templateData(c.report)
	if err != nil {
		return nil, templateFailedExit
	}
	var out bytes.Buffer
	if err := parsed.Execute(&out, data); err != nil {
		return nil, templateFailedExit
	}
	return out.Bytes(), 0
}

// templateCaseDirs names every case directory under the template vectors.
func templateCaseDirs(t *testing.T, fsys fs.FS) []string {
	t.Helper()
	entries, err := fs.ReadDir(fsys, templateVectorsDir)
	if err != nil {
		t.Fatalf("Setup: fs.ReadDir(%s): %v", templateVectorsDir, err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(dirs) == 0 {
		t.Fatalf("Setup: %s holds no case", templateVectorsDir)
	}
	return dirs
}

// loadTemplateCase reads one case, refusing a layout grammar/template.md's vector
// README does not declare.
func loadTemplateCase(fsys fs.FS, dir string) (templateCase, error) {
	at := path.Join(templateVectorsDir, dir)
	entries, err := fs.ReadDir(fsys, at)
	if err != nil {
		return templateCase{}, err
	}
	var names []string
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	var c templateCase
	source, err := fs.ReadFile(fsys, path.Join(at, templateFile))
	if err != nil {
		return templateCase{}, err
	}
	if !utf8.Valid(source) {
		return templateCase{}, fmt.Errorf("%s is not UTF-8", templateFile)
	}
	c.source = string(source)
	if c.report, err = fs.ReadFile(fsys, path.Join(at, templateReportFile)); err != nil {
		return templateCase{}, err
	}
	expected, hasExpected := slices.Contains(names, templateExpectedFile), slices.Contains(names, templateExitFile)
	switch {
	case expected == hasExpected:
		return templateCase{}, fmt.Errorf("files %v, want exactly one of %s and %s", names, templateExpectedFile, templateExitFile)
	case expected:
		c.expected, err = fs.ReadFile(fsys, path.Join(at, templateExpectedFile))
	default:
		var raw []byte
		if raw, err = fs.ReadFile(fsys, path.Join(at, templateExitFile)); err == nil {
			c.exit, err = strconv.Atoi(strings.TrimSuffix(string(raw), "\n"))
		}
		if err == nil && c.exit != templateRefusedExit && c.exit != templateFailedExit {
			err = fmt.Errorf("%s = %d, want %d or %d", templateExitFile, c.exit, templateRefusedExit, templateFailedExit)
		}
	}
	if err != nil {
		return templateCase{}, err
	}
	if len(names) != 3 {
		return templateCase{}, fmt.Errorf("files %v, want %s, %s and one outcome file", names, templateFile, templateReportFile)
	}
	return c, nil
}

// templateReproductionProblems renders one case with the reference and compares the
// outcome with the case's own.
func templateReproductionProblems(fsys fs.FS, dir string) []string {
	c, err := loadTemplateCase(fsys, dir)
	if err != nil {
		return []string{err.Error()}
	}
	got, exit := renderTemplateCase(c)
	switch {
	case exit != c.exit:
		return []string{fmt.Sprintf("exit = %d, want %d", exit, c.exit)}
	case exit == 0 && !bytes.Equal(got, c.expected):
		return []string{fmt.Sprintf("rendering = %q, want %q", got, c.expected)}
	}
	return nil
}

// TestTemplateVectorCasesReproduceFromTheirTemplatesAndReports renders every published
// case with the reference the subset defines, so an output byte or an exit code the
// template and the report do not determine is a failure here.
func TestTemplateVectorCasesReproduceFromTheirTemplatesAndReports(t *testing.T) {
	for _, dir := range templateCaseDirs(t, spec.Vectors) {
		t.Run(dir, func(t *testing.T) {
			for _, problem := range templateReproductionProblems(spec.Vectors, dir) {
				t.Errorf("%s/%s: %s", templateVectorsDir, dir, problem)
			}
		})
	}
}

// TestTemplateVectorReportsAreInstancesOfTheReportSchema holds every case's input to
// the document a template reads.
func TestTemplateVectorReportsAreInstancesOfTheReportSchema(t *testing.T) {
	for _, dir := range templateCaseDirs(t, spec.Vectors) {
		t.Run(dir, func(t *testing.T) {
			report, err := fs.ReadFile(spec.Vectors, path.Join(templateVectorsDir, dir, templateReportFile))
			if err != nil {
				t.Fatalf("Setup: %v", err)
			}
			validateAgainst(t, reportSchemaPath, report)
		})
	}
}

// TestTemplateVectorsCoverEveryOutcome holds the case set to the three outcomes the
// page states: a rendering, a refusal before analysis and a failed rendering.
func TestTemplateVectorsCoverEveryOutcome(t *testing.T) {
	seen := map[int]int{}
	for _, dir := range templateCaseDirs(t, spec.Vectors) {
		c, err := loadTemplateCase(spec.Vectors, dir)
		if err != nil {
			t.Fatalf("loadTemplateCase(%s): %v", dir, err)
		}
		seen[c.exit]++
	}
	for _, exit := range []int{0, templateRefusedExit, templateFailedExit} {
		if seen[exit] == 0 {
			t.Errorf("cases ending with %d = 0, want at least one", exit)
		}
	}
}

// TestTemplateVectorsReadmeNamesEveryCase pins the table of the vectors' README to the
// case directories, one row each.
func TestTemplateVectorsReadmeNamesEveryCase(t *testing.T) {
	readme, err := fs.ReadFile(spec.Vectors, path.Join(templateVectorsDir, "README.md"))
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	for _, dir := range templateCaseDirs(t, spec.Vectors) {
		if n := strings.Count(string(readme), "| `"+dir+"` |"); n != 1 {
			t.Errorf("README rows naming %s = %d, want 1", dir, n)
		}
	}
}

// TestTemplateReproductionRefuses plants one departure from the page per rule into a
// copy of a published case and requires the reference to see it.
func TestTemplateReproductionRefuses(t *testing.T) {
	tests := []struct {
		name, dir, file, old, replacement string
	}{
		{name: "a_missing_member_rendered", dir: "member-the-document-lacks", file: templateExitFile, old: "3", replacement: "0"},
		{name: "a_function_the_subset_omits_rendered", dir: "function-outside-the-subset", file: templateExitFile, old: "2", replacement: "3"},
		{name: "an_output_byte_changed", dir: "findings-by-member-name", file: templateExpectedFile, old: "41 deletable", replacement: "42 deletable"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := fstest.MapFS{}
			at := path.Join(templateVectorsDir, tc.dir)
			for _, name := range []string{templateFile, templateReportFile, tc.file} {
				data, err := fs.ReadFile(spec.Vectors, path.Join(at, name))
				if err != nil {
					t.Fatalf("Setup: %v", err)
				}
				m[path.Join(at, name)] = &fstest.MapFile{Data: data}
			}
			planted := m[path.Join(at, tc.file)]
			if !bytes.Contains(planted.Data, []byte(tc.old)) {
				t.Fatalf("Setup: %s holds no %q", tc.file, tc.old)
			}
			planted.Data = bytes.Replace(planted.Data, []byte(tc.old), []byte(tc.replacement), 1)
			if len(templateReproductionProblems(m, tc.dir)) == 0 {
				t.Errorf("templateReproductionProblems(%s with %s %q) = none, want the departure named", tc.dir, tc.file, tc.replacement)
			}
		})
	}
}
