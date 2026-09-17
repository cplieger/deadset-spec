package spec_test

import (
	"encoding/json"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec"
)

const (
	symbolRefPagePath     = "contract/grammar/symbol-ref.md"
	symbolRefCorpusPath   = "contract/grammar/symbol-ref-corpus.json"
	suppressionPagePath   = "contract/grammar/suppression.md"
	suppressionCorpusPath = "contract/grammar/suppression-corpus.json"
	textLinePagePath      = "contract/grammar/text-line.md"
)

// grammarPage reads one published grammar page out of the embedded contract tree.
func grammarPage(t *testing.T, p string) string {
	t.Helper()
	data, err := fs.ReadFile(spec.Contract, p)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", p, err)
	}
	return string(data)
}

// grammarCorpus decodes one token corpus, refusing a case that carries a field the corpus
// does not declare.
func grammarCorpus(t *testing.T, p string, cases any) {
	t.Helper()
	data, err := fs.ReadFile(spec.Contract, p)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", p, err)
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(cases); err != nil {
		t.Fatalf("Setup: decode %q: %v", p, err)
	}
}

// pageSection returns the body of one heading: every line after it up to the next heading
// of any level, so a section's own text comes back without its subsections. The heading
// must appear exactly once on the page.
func pageSection(t *testing.T, page, heading string) string {
	t.Helper()
	lines := strings.Split(page, "\n")
	var starts []int
	for i, line := range lines {
		if line == heading {
			starts = append(starts, i)
		}
	}
	if len(starts) != 1 {
		t.Fatalf("Setup: the page carries %d headings %q, want exactly 1", len(starts), heading)
	}
	body := lines[starts[0]+1:]
	for i, line := range body {
		if strings.HasPrefix(line, "#") {
			return strings.Join(body[:i], "\n")
		}
	}
	return strings.Join(body, "\n")
}

// pageFences returns the lines of every fenced block in text, in document order.
func pageFences(text string) [][]string {
	var out [][]string
	var open []string
	inside := false
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "```") {
			if inside {
				out = append(out, open)
				open, inside = nil, false
				continue
			}
			inside = true
			continue
		}
		if inside {
			open = append(open, line)
		}
	}
	return out
}

// firstFenceLine returns the single line of the first fenced block of text.
func firstFenceLine(t *testing.T, text, what string) string {
	t.Helper()
	fences := pageFences(text)
	if len(fences) == 0 {
		t.Fatalf("Setup: %s carries no fenced block, want one", what)
	}
	if len(fences[0]) != 1 {
		t.Fatalf("Setup: the first fenced block of %s spans %d lines, want 1", what, len(fences[0]))
	}
	return fences[0][0]
}

// onlyFence returns the one fenced block of text, which must hold exactly one.
func onlyFence(t *testing.T, text, what string) []string {
	t.Helper()
	fences := pageFences(text)
	if len(fences) != 1 {
		t.Fatalf("Setup: %s carries %d fenced blocks, want exactly 1", what, len(fences))
	}
	return fences[0]
}

// pageTableRows returns the cells of every markdown table row in text, the header row
// first and the delimiter row dropped, with each escaped pipe restored to a pipe.
func pageTableRows(text string) [][]string {
	var out [][]string
	for line := range strings.SplitSeq(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue
		}
		fields := strings.Split(strings.Trim(strings.ReplaceAll(trimmed, `\|`, "\x00"), "|"), "|")
		cells := make([]string, 0, len(fields))
		for _, field := range fields {
			cells = append(cells, strings.ReplaceAll(strings.TrimSpace(field), "\x00", "|"))
		}
		if !isTableDelimiter(cells) {
			out = append(out, cells)
		}
	}
	return out
}

func isTableDelimiter(cells []string) bool {
	for _, cell := range cells {
		if cell == "" || strings.Trim(cell, "-:") != "" {
			return false
		}
	}
	return true
}

var inlineCodePattern = regexp.MustCompile("^`(.+)`$")

// inlineCode returns the text of a table cell holding one inline code span.
func inlineCode(t *testing.T, cell string) string {
	t.Helper()
	m := inlineCodePattern.FindStringSubmatch(cell)
	if m == nil {
		t.Fatalf("the table cell %q is not one inline code span, want `...`", cell)
	}
	return m[1]
}

// compileExpression compiles one published expression, so a page whose expression stopped
// compiling is a failure rather than a panic.
func compileExpression(t *testing.T, what, expr string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(expr)
	if err != nil {
		t.Fatalf("regexp.Compile(%s = %q): %v", what, expr, err)
	}
	return re
}

// caseName is a subtest name: an identifier with no slash, dot or space, since a reference
// and a path are full of them and the runner reads a slash as a subtest path separator.
func caseName(parts ...string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '/', '.', ':':
			return '_'
		}
		return r
	}, strings.Join(parts, "_"))
}

// symbolFormKey names one row of the symbol-reference grammar: a language and the form the
// corpus spells in its form field.
type symbolFormKey struct {
	language string
	form     string
}

func (k symbolFormKey) String() string { return k.language + " " + k.form }

func sortedFormKeys[V any](m map[symbolFormKey]V) []symbolFormKey {
	out := make([]symbolFormKey, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	slices.SortFunc(out, func(a, b symbolFormKey) int { return strings.Compare(a.String(), b.String()) })
	return out
}

type namedBlock struct {
	name string
	expr string
}

// symbolRefBuildingBlocks parses the building-block table, then expands each block's own
// references to other blocks, so every block is a standalone expression.
func symbolRefBuildingBlocks(t *testing.T, page string) []namedBlock {
	t.Helper()
	rows := pageTableRows(pageSection(t, page, "### Building blocks"))
	if len(rows) < 2 {
		t.Fatalf("Setup: the building-block table carries %d rows, want a header and at least one block", len(rows))
	}
	blocks := make([]namedBlock, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) != 3 {
			t.Fatalf("Setup: the building-block row %q holds %d cells, want 3", row, len(row))
		}
		blocks = append(blocks, namedBlock{name: inlineCode(t, row[0]), expr: inlineCode(t, row[1])})
	}
	expanded := make([]namedBlock, 0, len(blocks))
	for _, block := range blocks {
		others := slices.DeleteFunc(slices.Clone(blocks), func(b namedBlock) bool { return b.name == block.name })
		expanded = append(expanded, namedBlock{name: block.name, expr: substituteBlocks(t, block.expr, others)})
	}
	return expanded
}

// substituteBlocks replaces every building-block name in expr with the block's expression,
// to a fixed point.
func substituteBlocks(t *testing.T, expr string, blocks []namedBlock) string {
	t.Helper()
	for range len(blocks) + 1 {
		before := expr
		for _, block := range blocks {
			expr = strings.ReplaceAll(expr, block.name, block.expr)
		}
		if expr == before {
			return expr
		}
	}
	t.Fatalf("substituting the building blocks into %q does not settle", expr)
	return ""
}

// symbolRefFormTable parses the one-expression-per-form table and substitutes the building
// blocks into every row, which is what the page's expanded block publishes.
func symbolRefFormTable(t *testing.T, page string, blocks []namedBlock) map[symbolFormKey]string {
	t.Helper()
	rows := pageTableRows(pageSection(t, page, "### One expression per form"))
	if len(rows) < 2 {
		t.Fatalf("Setup: the per-form table carries %d rows, want a header and at least one form", len(rows))
	}
	out := make(map[symbolFormKey]string)
	for _, row := range rows[1:] {
		if len(row) != 3 {
			t.Fatalf("Setup: the per-form row %q holds %d cells, want 3", row, len(row))
		}
		expr := substituteBlocks(t, inlineCode(t, row[2]), blocks)
		for cell := range strings.SplitSeq(row[1], ",") {
			key := symbolFormKey{language: row[0], form: inlineCode(t, strings.TrimSpace(cell))}
			if _, seen := out[key]; seen {
				t.Errorf("the per-form table names %s twice, want one expression per form", key)
			}
			out[key] = expr
		}
	}
	return out
}

// symbolRefExpandedForms parses the expanded block, where each pair of lines is a language
// with its forms and the expression those forms take.
func symbolRefExpandedForms(t *testing.T, page string) map[symbolFormKey]string {
	t.Helper()
	out := make(map[symbolFormKey]string)
	rest := onlyFence(t, pageSection(t, page, "### Expanded"), "the expanded section")
	for len(rest) > 0 {
		header := strings.TrimSpace(rest[0])
		if header == "" {
			rest = rest[1:]
			continue
		}
		if len(rest) < 2 {
			t.Fatalf("Setup: the expanded block ends on the header %q, want an expression under it", header)
		}
		language, forms, ok := strings.Cut(header, " ")
		if !ok {
			t.Fatalf("Setup: the expanded header %q names no form, want a language and its forms", header)
		}
		for form := range strings.SplitSeq(forms, ",") {
			out[symbolFormKey{language: language, form: strings.TrimSpace(form)}] = rest[1]
		}
		rest = rest[2:]
	}
	if len(out) == 0 {
		t.Fatalf("Setup: the expanded block publishes no form, want one expression per form")
	}
	return out
}

// compileSymbolRefForms returns the expression of every published form, compiled.
func compileSymbolRefForms(t *testing.T) map[symbolFormKey]*regexp.Regexp {
	t.Helper()
	forms := symbolRefExpandedForms(t, grammarPage(t, symbolRefPagePath))
	out := make(map[symbolFormKey]*regexp.Regexp, len(forms))
	for key, expr := range forms {
		out[key] = compileExpression(t, key.String(), expr)
	}
	return out
}

// matchingSymbolForms returns every published form whose expression matches s.
func matchingSymbolForms(forms map[symbolFormKey]*regexp.Regexp, s string) []symbolFormKey {
	var matched []symbolFormKey
	for _, key := range sortedFormKeys(forms) {
		if forms[key].MatchString(s) {
			matched = append(matched, key)
		}
	}
	return matched
}

type symbolRefCase struct {
	Input    string `json:"input"`
	Language string `json:"language"`
	Form     string `json:"form"`
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason"`
	Example  string `json:"example"`
}

func loadSymbolRefCorpus(t *testing.T) []symbolRefCase {
	t.Helper()
	var cases []symbolRefCase
	grammarCorpus(t, symbolRefCorpusPath, &cases)
	if len(cases) == 0 {
		t.Fatalf("Setup: %q holds no case, want the published corpus", symbolRefCorpusPath)
	}
	return cases
}

func TestSymbolRefPageSubstitutesItsBuildingBlocks(t *testing.T) {
	page := grammarPage(t, symbolRefPagePath)
	blocks := symbolRefBuildingBlocks(t, page)
	fromTable := symbolRefFormTable(t, page, blocks)
	expanded := symbolRefExpandedForms(t, page)

	tableForms, expandedForms := formNames(fromTable), formNames(expanded)
	if !slices.Equal(tableForms, expandedForms) {
		t.Fatalf("the per-form table names %v, the expanded block names %v, want the same forms", tableForms, expandedForms)
	}
	for _, key := range sortedFormKeys(fromTable) {
		if got, want := expanded[key], fromTable[key]; got != want {
			t.Errorf("the expanded expression of %s is\n%s\nwant the per-form row with its blocks substituted:\n%s", key, got, want)
		}
	}
}

func formNames[V any](m map[symbolFormKey]V) []string {
	out := make([]string, 0, len(m))
	for _, key := range sortedFormKeys(m) {
		out = append(out, key.String())
	}
	return out
}

func TestSymbolRefCorpusMatchesItsFormAndNothingElse(t *testing.T) {
	forms := compileSymbolRefForms(t)
	for i, c := range loadSymbolRefCorpus(t) {
		t.Run(caseName("ref", strconv.Itoa(i), c.Language, c.Form), func(t *testing.T) {
			key := symbolFormKey{language: c.Language, form: c.Form}
			re, ok := forms[key]
			if !ok {
				t.Fatalf("case %d (%q) names form %s, want a form the page publishes", i, c.Input, key)
			}
			if c.Reason == "" {
				t.Errorf("case %d (%q) carries no reason, want one", i, c.Input)
			}
			if !c.Accepted {
				if matched := matchingSymbolForms(forms, c.Input); matched != nil {
					t.Errorf("refused case %d (%q) matches %v, want no form's expression", i, c.Input, matched)
				}
				return
			}
			if !re.MatchString(c.Input) {
				t.Errorf("the %s expression does not match accepted case %d (%q), want a match", key, i, c.Input)
			}
			if strings.ContainsAny(c.Input, "\r\n") {
				t.Errorf("accepted case %d (%q) carries CR or LF, want neither", i, c.Input)
			}
			if c.Example == "" {
				t.Errorf("accepted case %d (%q) carries no example, want the source it denotes", i, c.Input)
			}
		})
	}
}

func TestSymbolRefCorpusCoversEveryPublishedForm(t *testing.T) {
	accepted := make(map[symbolFormKey]int)
	for _, c := range loadSymbolRefCorpus(t) {
		if c.Accepted {
			accepted[symbolFormKey{language: c.Language, form: c.Form}]++
		}
	}
	forms := symbolRefExpandedForms(t, grammarPage(t, symbolRefPagePath))
	for _, key := range sortedFormKeys(forms) {
		if accepted[key] == 0 {
			t.Errorf("the corpus carries no accepted case for form %s, want at least 1", key)
		}
	}
}

// inlineExpressions are the three expressions the suppression page applies in order: the
// candidate, the well-formed directive, and the directive that lacks only its reason.
type inlineExpressions struct {
	candidate  *regexp.Regexp
	wellFormed *regexp.Regexp
	reasonless *regexp.Regexp
}

func compileInlineExpressions(t *testing.T) inlineExpressions {
	t.Helper()
	var published []string
	for _, fence := range pageFences(pageSection(t, grammarPage(t, suppressionPagePath), "### The shape")) {
		if len(fence) == 1 && strings.HasPrefix(fence[0], "^//") {
			published = append(published, fence[0])
		}
	}
	if len(published) != 3 {
		t.Fatalf("Setup: the inline shape section publishes %d expressions, want the 3 the decision procedure applies", len(published))
	}
	return inlineExpressions{
		candidate:  compileExpression(t, "the candidate expression", published[0]),
		wellFormed: compileExpression(t, "the well-formed expression", published[1]),
		reasonless: compileExpression(t, "the reasonless expression", published[2]),
	}
}

// inlineVerdict is the step of the page's decision procedure a comment reaches.
type inlineVerdict string

const (
	notADirective inlineVerdict = "step 1, not a directive"
	wellFormed    inlineVerdict = "step 2, a directive"
	reasonMissing inlineVerdict = "step 3, DS1701"
	malformed     inlineVerdict = "step 4, exit code 2"
)

func (e inlineExpressions) classify(comment string) inlineVerdict {
	switch {
	case !e.candidate.MatchString(comment):
		return notADirective
	case e.wellFormed.MatchString(comment):
		return wellFormed
	case e.reasonless.MatchString(comment):
		return reasonMissing
	default:
		return malformed
	}
}

// refusedInlineVerdict is the step the refused cases of each inline rule reach, and the
// text the page's refusal column carries for that step.
var (
	refusedInlineVerdict = map[string]inlineVerdict{
		"namespace":         notADirective,
		"first-token":       notADirective,
		"line-comment-only": notADirective,
		"reason-required":   reasonMissing,
		"directive-name":    malformed,
		"reason-separator":  malformed,
		"code-list":         malformed,
		"line-above":        wellFormed,
	}
	statedOutcome = map[inlineVerdict]string{
		notADirective: "not a directive",
		wellFormed:    "DS1703",
		reasonMissing: "DS1701",
		malformed:     "exit code 2",
	}
)

// boundLineOffset is the only offset from which an inline directive binds: the line
// immediately above the declaration it names.
const boundLineOffset = -1

type suppressionCase struct {
	Input    json.RawMessage `json:"input"`
	Kind     string          `json:"kind"`
	Accepted bool            `json:"accepted"`
	Rule     string          `json:"rule"`
	Reason   string          `json:"reason"`
}

type inlineInput struct {
	Comment    string `json:"comment"`
	LineOffset *int   `json:"line_offset"`
}

func loadSuppressionCorpus(t *testing.T) []suppressionCase {
	t.Helper()
	var cases []suppressionCase
	grammarCorpus(t, suppressionCorpusPath, &cases)
	if len(cases) == 0 {
		t.Fatalf("Setup: %q holds no case, want the published corpus", suppressionCorpusPath)
	}
	return cases
}

func decodeInlineInput(t *testing.T, raw json.RawMessage) inlineInput {
	t.Helper()
	var in inlineInput
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		t.Fatalf("decode the inline input %s: %v", raw, err)
	}
	if in.LineOffset == nil {
		t.Fatalf("the inline input %s carries no line_offset, want one", raw)
	}
	return in
}

var directiveCodePattern = regexp.MustCompile(`^DS[0-9]{4}$`)

func TestSuppressionInlineCorpusFollowsTheDecisionProcedure(t *testing.T) {
	exprs := compileInlineExpressions(t)
	vocabulary := suppressionRuleVocabulary(t)
	codesGroup := exprs.wellFormed.SubexpIndex("codes")
	reasonGroup := exprs.wellFormed.SubexpIndex("reason")
	if codesGroup < 0 || reasonGroup < 0 {
		t.Fatalf("the well-formed expression captures %v, want the groups codes and reason", exprs.wellFormed.SubexpNames())
	}

	for i, c := range loadSuppressionCorpus(t) {
		if c.Kind != "inline" {
			continue
		}
		t.Run(caseName("inline", strconv.Itoa(i), c.Rule), func(t *testing.T) {
			in := decodeInlineInput(t, c.Input)
			got := exprs.classify(in.Comment)

			if c.Accepted {
				if got != wellFormed {
					t.Fatalf("the procedure reaches %q on the accepted comment %q, want %q", got, in.Comment, wellFormed)
				}
				if *in.LineOffset != boundLineOffset {
					t.Errorf("accepted case %d has line_offset = %d, want %d", i, *in.LineOffset, boundLineOffset)
				}
				groups := exprs.wellFormed.FindStringSubmatch(in.Comment)
				for code := range strings.SplitSeq(groups[codesGroup], ",") {
					if !directiveCodePattern.MatchString(code) {
						t.Errorf("accepted case %d names the code %q, want DS and four digits", i, code)
					}
				}
				if reason := groups[reasonGroup]; strings.TrimSpace(reason) == "" {
					t.Errorf("accepted case %d captures the reason %q, want a reason that is not whitespace", i, reason)
				}
				return
			}

			want, ok := refusedInlineVerdict[c.Rule]
			if !ok {
				t.Fatalf("refused case %d names the rule %q, want a rule whose refusal this suite reads", i, c.Rule)
			}
			if got != want {
				t.Errorf("the procedure reaches %q on the refused comment %q, want %q under rule %q", got, in.Comment, want, c.Rule)
			}
			if outcome := vocabulary[c.Rule].onRefusal; !strings.Contains(outcome, statedOutcome[want]) {
				t.Errorf("the page states %q on refusal under rule %q, want it to name %q", outcome, c.Rule, statedOutcome[want])
			}
			if boundHere := *in.LineOffset == boundLineOffset; boundHere == (c.Rule == "line-above") {
				t.Errorf("refused case %d under %q has line_offset = %d, want the placement rule to be the one defect it exercises", i, c.Rule, *in.LineOffset)
			}
		})
	}
}

// entryKeys is the closed key list an ignore entry and a baseline row share.
var entryKeys = []string{"code", "symbol", "path", "reason"}

// isRenderedPath reports whether p takes the path form text-line.md fixes for
// position.path: target-relative, forward slashes, no leading "./" and no trailing "/".
func isRenderedPath(p string) bool {
	if strings.ContainsAny(p, "\\\r\n") || strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") {
		return false
	}
	for segment := range strings.SplitSeq(p, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// failedEntryChecks returns the checks an ignore entry or a baseline row fails, named as
// the corpus names its rules. A value of the wrong type is the decoder's refusal, so the
// checks over a value's own form apply only where the value is the string the page declares.
func failedEntryChecks(t *testing.T, raw json.RawMessage, forms map[symbolFormKey]*regexp.Regexp) []string {
	t.Helper()
	var entry map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("decode the entry %s: %v", raw, err)
	}

	var failed []string
	fail := func(check string) {
		if !slices.Contains(failed, check) {
			failed = append(failed, check)
		}
	}
	values := make(map[string]string)
	mistyped := make(map[string]bool)
	for key, value := range entry {
		if !slices.Contains(entryKeys, key) {
			fail("closed-keys")
			continue
		}
		var s string
		if err := json.Unmarshal(value, &s); err != nil {
			fail("value-type")
			mistyped[key] = true
			continue
		}
		values[key] = s
	}

	for _, key := range []string{"code", "symbol"} {
		if _, present := entry[key]; !present {
			fail("required-keys")
		}
	}
	if code, ok := values["code"]; ok && !directiveCodePattern.MatchString(code) {
		fail("code-form")
	}
	if symbol, ok := values["symbol"]; ok && matchingSymbolForms(forms, symbol) == nil {
		fail("symbol-form")
	}
	if path, ok := values["path"]; !mistyped["path"] {
		switch {
		case !ok || path == "":
			fail("path-required")
		case !isRenderedPath(path):
			fail("path-form")
		}
	}
	if reason, ok := values["reason"]; !mistyped["reason"] && (!ok || strings.TrimSpace(reason) == "") {
		fail("reason-required")
	}
	slices.Sort(failed)
	return failed
}

// statedEntryOutcome is the outcome the page states for a refused ignore entry or
// baseline row under each rule.
var statedEntryOutcome = map[string]string{
	"reason-required": "DS1701",
	"path-required":   "DS1702",
	"path-form":       "exit code 2",
	"symbol-form":     "exit code 2",
	"code-form":       "exit code 2",
	"closed-keys":     "exit code 2",
	"value-type":      "exit code 2",
}

func TestSuppressionDocumentCorpusFailsExactlyTheCheckItsRuleNames(t *testing.T) {
	forms := compileSymbolRefForms(t)
	vocabulary := suppressionRuleVocabulary(t)
	for i, c := range loadSuppressionCorpus(t) {
		if c.Kind == "inline" {
			continue
		}
		t.Run(caseName(c.Kind, strconv.Itoa(i), c.Rule), func(t *testing.T) {
			failed := failedEntryChecks(t, c.Input, forms)
			if c.Accepted {
				if len(failed) != 0 {
					t.Errorf("accepted case %d fails %v, want every check passed", i, failed)
				}
				return
			}
			if want := []string{c.Rule}; !slices.Equal(failed, want) {
				t.Fatalf("refused case %d fails %v, want exactly %v", i, failed, want)
			}
			if outcome := vocabulary[c.Rule].onRefusal; !strings.Contains(outcome, statedEntryOutcome[c.Rule]) {
				t.Errorf("the page states %q on refusal under rule %q, want it to name %q", outcome, c.Rule, statedEntryOutcome[c.Rule])
			}
		})
	}
}

// suppressionRule is one row of the page's rule vocabulary: the mechanisms the rule
// applies to and what the page states a refused case under it produces.
type suppressionRule struct {
	kinds     []string
	onRefusal string
}

// appliesToKinds reads the mechanism column of the rule vocabulary as corpus kinds.
var appliesToKinds = map[string][]string{
	"inline":                     {"inline"},
	"ignore entry":               {"ignore-entry"},
	"baseline row":               {"baseline-row"},
	"ignore entry, baseline row": {"ignore-entry", "baseline-row"},
	"all three":                  {"inline", "ignore-entry", "baseline-row"},
}

func suppressionRuleVocabulary(t *testing.T) map[string]suppressionRule {
	t.Helper()
	rows := pageTableRows(pageSection(t, grammarPage(t, suppressionPagePath), "## The token corpus"))
	if len(rows) < 2 {
		t.Fatalf("Setup: the rule table carries %d rows, want a header and the rules", len(rows))
	}
	out := make(map[string]suppressionRule)
	for _, row := range rows[1:] {
		if len(row) != 3 {
			t.Fatalf("Setup: the rule row %q holds %d cells, want 3", row, len(row))
		}
		kinds, ok := appliesToKinds[row[1]]
		if !ok {
			t.Fatalf("Setup: the rule row %q applies to %q, want a mechanism this suite reads", row[0], row[1])
		}
		for cell := range strings.SplitSeq(row[0], ",") {
			out[inlineCode(t, strings.TrimSpace(cell))] = suppressionRule{kinds: kinds, onRefusal: row[2]}
		}
	}
	return out
}

func TestSuppressionCorpusRulesAreThePageVocabulary(t *testing.T) {
	vocabulary := suppressionRuleVocabulary(t)
	exercised := make(map[string]bool)
	for i, c := range loadSuppressionCorpus(t) {
		rule, ok := vocabulary[c.Rule]
		if !ok {
			t.Errorf("case %d names the rule %q, want a rule the page's vocabulary carries", i, c.Rule)
			continue
		}
		if !slices.Contains(rule.kinds, c.Kind) {
			t.Errorf("case %d exercises the rule %q at kind %q, want one of %v", i, c.Rule, c.Kind, rule.kinds)
		}
		if c.Reason == "" {
			t.Errorf("case %d carries no reason, want one", i)
		}
		exercised[c.Rule] = true
	}
	rules := make([]string, 0, len(vocabulary))
	for rule := range vocabulary {
		rules = append(rules, rule)
	}
	slices.Sort(rules)
	for _, rule := range rules {
		if !exercised[rule] {
			t.Errorf("the corpus carries no case under the rule %q, want at least one", rule)
		}
	}
}

// textLineExpression returns the expression the text-line page publishes and the shape
// line it renders.
func textLineExpression(t *testing.T, page string) (expression *regexp.Regexp, shape string) {
	t.Helper()
	published := firstFenceLine(t, pageSection(t, page, "## The expression"), "the expression section")
	return compileExpression(t, "the text-line expression", published), firstFenceLine(t, pageSection(t, page, "## The shape"), "the shape section")
}

// textLineSamples returns the non-empty lines of the sample block under one heading.
func textLineSamples(t *testing.T, page, heading string) []string {
	t.Helper()
	var out []string
	for _, line := range onlyFence(t, pageSection(t, page, heading), heading) {
		if line != "" {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		t.Fatalf("Setup: the %s block holds no sample line, want the published set", heading)
	}
	return out
}

var textLineFieldPattern = regexp.MustCompile(`\b(path|line|col|kind|name|message|confidence|CODE)\b`)

// renderTextLine substitutes a match's groups into the page's shape line, in one
// left-to-right pass, so a field value that reads like a field name is never rescanned.
func renderTextLine(t *testing.T, re *regexp.Regexp, shape string, groups []string) string {
	t.Helper()
	field := func(name string) string {
		i := re.SubexpIndex(name)
		if i < 0 {
			t.Fatalf("the expression captures %v, want a group %q", re.SubexpNames(), name)
		}
		return groups[i]
	}
	return textLineFieldPattern.ReplaceAllStringFunc(shape, func(placeholder string) string {
		if placeholder == "CODE" {
			return field("code")
		}
		return field(placeholder)
	})
}

var (
	describedLinePattern = regexp.MustCompile(`(?m)^The ([a-z]+) line `)
	statedPathPattern    = regexp.MustCompile("`path` `([^`]*)`")
	statedNamePattern    = regexp.MustCompile("`name` `([^`]*)`")
	statedMessagePattern = regexp.MustCompile("the message `([^`]*)`")
)

var ordinals = []string{"first", "second", "third", "fourth", "fifth", "sixth", "seventh", "eighth", "ninth", "tenth"}

// statedParse returns the accepted sample the page reads field by field, with the field
// values it states for that sample.
func statedParse(t *testing.T, page string) (index int, path, name, message string) {
	t.Helper()
	body := pageSection(t, page, "### Accepted")
	ordinal := describedLinePattern.FindStringSubmatch(body)
	if ordinal == nil {
		t.Fatalf("Setup: the accepted section names no sample, want a sentence reading one of them field by field")
	}
	index = slices.Index(ordinals, ordinal[1])
	if index < 0 {
		t.Fatalf("Setup: the accepted section names the %q sample, want one of %v", ordinal[1], ordinals)
	}
	for _, stated := range []struct {
		field   string
		pattern *regexp.Regexp
		into    *string
	}{
		{"path", statedPathPattern, &path},
		{"name", statedNamePattern, &name},
		{"message", statedMessagePattern, &message},
	} {
		m := stated.pattern.FindStringSubmatch(body)
		if m == nil {
			t.Fatalf("Setup: the accepted section states no %s for the %s sample, want the value it parses to", stated.field, ordinal[1])
		}
		*stated.into = m[1]
	}
	return index, path, name, message
}

func TestTextLineAcceptedSamplesMatchAndRenderBack(t *testing.T) {
	page := grammarPage(t, textLinePagePath)
	re, shape := textLineExpression(t, page)
	samples := textLineSamples(t, page, "### Accepted")

	for i, line := range samples {
		t.Run(caseName("accepted", strconv.Itoa(i)), func(t *testing.T) {
			groups := re.FindStringSubmatch(line)
			if groups == nil {
				t.Fatalf("the expression does not match the accepted sample %q, want a match", line)
			}
			if got := renderTextLine(t, re, shape, groups); got != line {
				t.Errorf("the captured fields render %q, want the sample %q", got, line)
			}
		})
	}

	index, path, name, message := statedParse(t, page)
	if index >= len(samples) {
		t.Fatalf("the accepted section reads sample %d field by field, want one of the %d published samples", index+1, len(samples))
	}
	groups := re.FindStringSubmatch(samples[index])
	if groups == nil {
		t.Fatalf("the expression does not match %q, the sample the page reads field by field", samples[index])
	}
	for _, want := range []struct {
		group string
		value string
	}{
		{"path", path},
		{"name", name},
		{"message", message},
	} {
		if got := groups[re.SubexpIndex(want.group)]; got != want.value {
			t.Errorf("group %s of %q = %q, want the stated %q", want.group, samples[index], got, want.value)
		}
	}
}

func TestTextLineRefusedSamplesDoNotMatch(t *testing.T) {
	page := grammarPage(t, textLinePagePath)
	re, _ := textLineExpression(t, page)
	for i, line := range textLineSamples(t, page, "### Refused") {
		t.Run(caseName("refused", strconv.Itoa(i)), func(t *testing.T) {
			if re.MatchString(line) {
				t.Errorf("the expression matches the refused sample %q, want no match", line)
			}
		})
	}
}

func TestTextLineShellFilterAgreesWithTheExpression(t *testing.T) {
	page := grammarPage(t, textLinePagePath)
	filter := compileExpression(t, "the shell filter", firstFenceLine(t, pageSection(t, page, "## A filter for a shell"), "the filter section"))
	for _, line := range textLineSamples(t, page, "### Accepted") {
		if !filter.MatchString(line) {
			t.Errorf("the filter does not select the accepted sample %q, want every accepted line selected", line)
		}
	}
	for _, line := range textLineSamples(t, page, "### Refused") {
		if filter.MatchString(line) {
			t.Errorf("the filter selects the refused sample %q, want no refused line selected", line)
		}
	}
}

// constructsOutsideBothDialects compile in one of RE2 and ECMAScript without flags and not
// in the other, or read differently in the two, so no published expression carries one.
var constructsOutsideBothDialects = []string{
	`\A`, `\z`, `\Z`, `\Q`, `\E`, `\p{`, `\P{`, `(?P<`, `(?>`, `(?=`, `(?!`, `(?<=`, `(?<!`, `(?#`, `[:`,
}

var (
	inlineFlagGroup      = regexp.MustCompile(`\(\?[a-zA-Z-]+[:)]`)
	possessiveQuantifier = regexp.MustCompile(`[*+?}]\+`)
	numericEscape        = regexp.MustCompile(`\\[0-9]`)
	oneLetterClass       = regexp.MustCompile(`\\p[A-Za-z]`)
)

// publishedExpressions returns every expression the three grammar pages publish, keyed by
// where the page publishes it.
func publishedExpressions(t *testing.T) map[string]string {
	t.Helper()
	out := make(map[string]string)
	symbolRef := grammarPage(t, symbolRefPagePath)
	for _, block := range symbolRefBuildingBlocks(t, symbolRef) {
		out["symbol-ref block "+block.name] = block.expr
	}
	for key, expr := range symbolRefExpandedForms(t, symbolRef) {
		out["symbol-ref form "+key.String()] = expr
	}
	inline := compileInlineExpressions(t)
	out["suppression candidate"] = inline.candidate.String()
	out["suppression well-formed"] = inline.wellFormed.String()
	out["suppression reasonless"] = inline.reasonless.String()
	textLine := grammarPage(t, textLinePagePath)
	expression, _ := textLineExpression(t, textLine)
	out["text-line expression"] = expression.String()
	out["text-line shell filter"] = firstFenceLine(t, pageSection(t, textLine, "## A filter for a shell"), "the filter section")
	return out
}

func TestPublishedExpressionsStayInsideBothDialects(t *testing.T) {
	published := publishedExpressions(t)
	names := make([]string, 0, len(published))
	for name := range published {
		names = append(names, name)
	}
	slices.Sort(names)

	for i, name := range names {
		expr := published[name]
		t.Run(caseName("expr", strconv.Itoa(i)), func(t *testing.T) {
			compileExpression(t, name, expr)
			for _, construct := range constructsOutsideBothDialects {
				if strings.Contains(expr, construct) {
					t.Errorf("%s carries %q, want only what both dialects read: %s", name, construct, expr)
				}
			}
			for _, outside := range []struct {
				what    string
				pattern *regexp.Regexp
			}{
				{"an inline flag group", inlineFlagGroup},
				{"a possessive quantifier", possessiveQuantifier},
				{"a numeric escape", numericEscape},
				{"a one-letter Unicode class", oneLetterClass},
			} {
				if m := outside.pattern.FindString(expr); m != "" {
					t.Errorf("%s carries %s, %q, want only what both dialects read: %s", name, outside.what, m, expr)
				}
			}
		})
	}
}
