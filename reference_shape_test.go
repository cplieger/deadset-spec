package spec_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"math/rand/v2"
	"regexp"
	"regexp/syntax"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/segment"
)

const (
	shapeSection         = "### The shape every document checks"
	segmentCorpusPath    = "contract/grammar/segment-corpus.json"
	referenceDefPointer  = "/$defs/reference"
	symbolReferenceRef   = "#/$defs/symbol_reference"
	configSpellingSymbol = "/properties/symbol/properties/ref"
)

// referenceShape is the page's one shape expression with every building block
// substituted, after checking the page's expanded fence publishes the same
// string.
func referenceShape(t *testing.T) string {
	t.Helper()
	page := grammarPage(t, symbolRefPagePath)
	section := pageSection(t, page, shapeSection)
	rows := pageTableRows(section)
	if len(rows) != 2 || len(rows[1]) != 3 || inlineCode(t, rows[1][0]) != "REF_SHAPE" {
		t.Fatalf("Setup: %s carries %q, want a header and the one REF_SHAPE row", shapeSection, rows)
	}
	expanded := substituteBlocks(t, inlineCode(t, rows[1][1]), symbolRefBuildingBlocks(t, page))
	if fenced := onlyFence(t, section, "the shape section"); len(fenced) != 1 || fenced[0] != expanded {
		t.Errorf("the shape section's fence is %q, want the REF_SHAPE row with its blocks substituted:\n%s", fenced, expanded)
	}
	return expanded
}

// pointerObject decodes one embedded schema and returns the node a JSON pointer names.
func pointerObject(t *testing.T, path, pointer string) map[string]any {
	t.Helper()
	data, err := fs.ReadFile(spec.Contract, path)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", path, err)
	}
	var node any
	if err := json.Unmarshal(data, &node); err != nil {
		t.Fatalf("Setup: decode %q: %v", path, err)
	}
	for token := range strings.SplitSeq(strings.TrimPrefix(pointer, "/"), "/") {
		object, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("%s%s does not resolve: %q is under no object", path, pointer, token)
		}
		node = object[token]
	}
	object, ok := node.(map[string]any)
	if !ok {
		t.Fatalf("%s%s is %v, want an object", path, pointer, node)
	}
	return object
}

func TestReferenceShapeIsTheGrammarsExpression(t *testing.T) {
	want := referenceShape(t)
	def := pointerObject(t, findingSchemaPath, referenceDefPointer)
	if got := def["pattern"]; got != want {
		t.Errorf("%s#%s pattern = %v, want symbol-ref.md's REF_SHAPE expanded:\n%s", findingSchemaPath, referenceDefPointer, got, want)
	}
	if got := def["type"]; got != "string" {
		t.Errorf("%s#%s type = %v, want string", findingSchemaPath, referenceDefPointer, got)
	}
}

// locatedObject is one object of a schema document with where it sits.
type locatedObject struct {
	schema, at string
	node       map[string]any
}

func locatedObjects(schema, at string, node any) []locatedObject {
	var out []locatedObject
	switch v := node.(type) {
	case map[string]any:
		out = append(out, locatedObject{schema: schema, at: at, node: v})
		for _, key := range slices.Sorted(maps.Keys(v)) {
			out = append(out, locatedObjects(schema, at+"/"+key, v[key])...)
		}
	case []any:
		for i, item := range v {
			out = append(out, locatedObjects(schema, fmt.Sprintf("%s/%d", at, i), item)...)
		}
	}
	return out
}

// schemePrefix is a pattern that only names the language a value is written in,
// which no grammar change can make drift.
var schemePrefix = regexp.MustCompile(`^\^[a-z][a-z0-9]*://$`)

// A schema checks a reference through the one definition, by $ref or by the
// copy x-inline writes of it: a pattern written by hand would drift from the
// grammar, as the edges document's own pattern once did. A bare scheme prefix
// may narrow a reference to one language beside the shape, never replace it.
func TestEveryReferenceInASchemaTakesTheOneDefinition(t *testing.T) {
	documents, err := embeddedSchemas()
	if err != nil {
		t.Fatalf("Setup: embeddedSchemas(): %v", err)
	}
	perForm := map[string]bool{}
	for _, expr := range symbolRefExpandedForms(t, grammarPage(t, symbolRefPagePath)) {
		perForm[expr] = true
	}
	shape := referenceShape(t)
	for _, name := range slices.Sorted(maps.Keys(documents)) {
		var root any
		if err := json.Unmarshal(documents[name], &root); err != nil {
			t.Fatalf("Setup: decode %s: %v", name, err)
		}
		for _, n := range locatedObjects(name, "", root) {
			where := n.schema + "#" + n.at
			if expr, ok := n.node["pattern"].(string); ok && strings.Contains(expr, "://") {
				if expr != shape && !perForm[expr] && !schemePrefix.MatchString(expr) {
					t.Errorf("%s carries the reference pattern %q, want $ref %s#%s or its copy, one of symbol-ref.md's per-form expressions, or a bare scheme prefix", where, expr, "finding.schema.json", referenceDefPointer)
				}
			}
			if ref, _ := n.node["$ref"].(string); strings.HasSuffix(ref, symbolReferenceRef) && (n.schema != findingSchemaPath || n.at != configSpellingSymbol) {
				t.Errorf("%s takes %s, which admits a configuration's spelling, want finding.schema.json#%s", where, symbolReferenceRef, referenceDefPointer)
			}
			description, _ := n.node["description"].(string)
			if !strings.HasPrefix(n.schema, "contract/") || n.node["type"] != "string" || n.node["enum"] != nil || !strings.Contains(strings.ToLower(description), "stable symbol reference") {
				continue
			}
			pattern, _ := n.node["pattern"].(string)
			if n.node["$ref"] == nil && pattern != shape && (n.schema != findingSchemaPath || n.at != "/$defs/symbol_reference") {
				t.Errorf("%s holds a stable symbol reference with neither $ref nor the shape, want finding.schema.json#%s or its copy", where, referenceDefPointer)
			}
		}
	}
}

// symbolKindAdmitsConfigurationSpelling is the kinds whose symbol.ref carries a
// configuration's spelling, which may be a pattern or a module entry and is no
// reference.
var symbolKindAdmitsConfigurationSpelling = []string{"configured-declaration", "root"}

func TestSymbolRefTakesTheShapeUnlessItCarriesAConfigurationsSpelling(t *testing.T) {
	symbol := pointerObject(t, findingSchemaPath, "/properties/symbol")
	data, err := json.Marshal(map[string]any{"if": symbol["if"], "else": symbol["else"], "then": symbol["then"]})
	if err != nil {
		t.Fatalf("Setup: marshal: %v", err)
	}
	var got struct {
		If struct {
			Properties struct {
				Kind struct {
					Enum []string `json:"enum"`
				} `json:"kind"`
			} `json:"properties"`
		} `json:"if"`
		Else struct {
			Properties struct {
				Ref struct {
					Ref string `json:"$ref"`
				} `json:"ref"`
			} `json:"properties"`
		} `json:"else"`
		Then any `json:"then"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Setup: decode the symbol object's condition: %v", err)
	}
	kinds := slices.Sorted(slices.Values(got.If.Properties.Kind.Enum))
	if !slices.Equal(kinds, symbolKindAdmitsConfigurationSpelling) {
		t.Errorf("the symbol object's if names the kinds %v, want %v", kinds, symbolKindAdmitsConfigurationSpelling)
	}
	if want := "#" + referenceDefPointer; got.Else.Properties.Ref.Ref != want {
		t.Errorf("the symbol object's else takes ref through %q, want %q", got.Else.Properties.Ref.Ref, want)
	}
	if got.Then != nil {
		t.Errorf("the symbol object carries then %v, want none: symbol_reference already admits the configuration's spelling", got.Then)
	}
}

// drawCandidates are the runes a character class is drawn from: every ASCII
// byte, and non-ASCII runes of two, three and four UTF-8 bytes, among them
// each whitespace and control past ASCII a class excludes and its neighbours.
var drawCandidates = func() []rune {
	out := make([]rune, 0, 0x80+len(boundaryRunes))
	for r := range rune(0x80) {
		out = append(out, r)
	}
	return append(out, boundaryRunes...)
}()

// boundaryRunes are the non-ASCII runes either side of each range the grammar's
// classes exclude past ASCII: the C1 controls, the no-break space and the
// whitespace past U+00FF.
var boundaryRunes = []rune{
	'\u0080', '\u0085', '\u009f', '\u00a0', '\u00a1', 'é', 'Ü', 'ß',
	'\u167f', '\u1680', '\u1681', '\u1fff', '\u2000', '\u200a', '\u200b',
	'\u2027', '\u2028', '\u2029', '\u202a', '\u202e', '\u202f', '\u2030',
	'\u205e', '\u205f', '\u2060', '\u2fff', '\u3000', '\u3001',
	'\ufefe', '\ufeff', '\uff00', '中', '€', '😀', '\U0010ffff',
}

const (
	drawsPerForm = 2000
	drawMaxExtra = 3
)

// draw writes one string the parsed expression matches. Every repetition is
// bounded, so a draw ends.
func draw(re *syntax.Regexp, r *rand.Rand, out *strings.Builder) {
	switch re.Op {
	case syntax.OpLiteral:
		out.WriteString(string(re.Rune))
	case syntax.OpCharClass:
		var in []rune
		for _, c := range drawCandidates {
			for i := 0; i+1 < len(re.Rune); i += 2 {
				if re.Rune[i] <= c && c <= re.Rune[i+1] {
					in = append(in, c)
					break
				}
			}
		}
		if len(in) == 0 {
			in = append(in, re.Rune[0])
		}
		out.WriteRune(in[r.IntN(len(in))])
	case syntax.OpCapture:
		draw(re.Sub[0], r, out)
	case syntax.OpConcat:
		for _, sub := range re.Sub {
			draw(sub, r, out)
		}
	case syntax.OpAlternate:
		draw(re.Sub[r.IntN(len(re.Sub))], r, out)
	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest, syntax.OpRepeat:
		low, high := repeatBounds(re)
		for range low + r.IntN(high-low+1) {
			draw(re.Sub[0], r, out)
		}
	case syntax.OpBeginText, syntax.OpEndText, syntax.OpBeginLine, syntax.OpEndLine, syntax.OpEmptyMatch:
	default:
		panic(fmt.Sprintf("draw: the operator %v is not drawn, want the generator to cover every operator a published expression uses", re.Op))
	}
}

func repeatBounds(re *syntax.Regexp) (low, high int) {
	switch re.Op {
	case syntax.OpStar:
		return 0, drawMaxExtra
	case syntax.OpPlus:
		return 1, 1 + drawMaxExtra
	case syntax.OpQuest:
		return 0, 1
	}
	if re.Max < 0 {
		return re.Min, re.Min + drawMaxExtra
	}
	return re.Min, min(re.Max, re.Min+drawMaxExtra)
}

// The shape admits every string every per-form expression admits, so a document
// checking the shape never refuses a reference its language defines.
func TestReferenceShapeAdmitsEveryProduction(t *testing.T) {
	shape := compileExpression(t, "REF_SHAPE", referenceShape(t))
	forms := symbolRefExpandedForms(t, grammarPage(t, symbolRefPagePath))
	for i, key := range sortedFormKeys(forms) {
		t.Run(caseName("draw", strconv.Itoa(i), key.language, key.form), func(t *testing.T) {
			expr := forms[key]
			parsed, err := syntax.Parse(expr, syntax.Perl)
			if err != nil {
				t.Fatalf("syntax.Parse(%s): %v", key, err)
			}
			form := compileExpression(t, key.String(), expr)
			seed := uint64(i) + 1
			r := rand.New(rand.NewPCG(seed, seed))
			for range drawsPerForm {
				var b strings.Builder
				draw(parsed, r, &b)
				drawn := b.String()
				if !form.MatchString(drawn) {
					t.Fatalf("seed %d: the draw %q does not match its own form %s, want the generator to draw only matches", seed, drawn, key)
				}
				if !shape.MatchString(drawn) {
					t.Fatalf("seed %d: REF_SHAPE refuses %q, which the %s form admits", seed, drawn, key)
				}
			}
		})
	}
}

func TestReferenceShapeRefusesWhatNoDocumentMayHold(t *testing.T) {
	shape := compileExpression(t, "REF_SHAPE", referenceShape(t))
	for i, c := range loadSymbolRefCorpus(t) {
		t.Run(caseName("shape", strconv.Itoa(i), c.Language, c.Form), func(t *testing.T) {
			got := shape.MatchString(c.Input)
			if c.Accepted {
				if c.Shape != nil {
					t.Errorf("accepted case %d (%q) carries shape, want it on refused cases only", i, c.Input)
				}
				if !got {
					t.Errorf("REF_SHAPE refuses accepted case %d (%q), want every reference in shape", i, c.Input)
				}
				return
			}
			if c.Shape == nil {
				t.Fatalf("refused case %d (%q) carries no shape, want whether REF_SHAPE admits it", i, c.Input)
			}
			if got != *c.Shape {
				t.Errorf("REF_SHAPE admits refused case %d (%q): %t, want %t", i, c.Input, got, *c.Shape)
			}
		})
	}
}

// segmentCase is one case of the path-segment corpus.
type segmentCase struct {
	Set       string `json:"set"`
	Spelled   string `json:"spelled"`
	Canonical bool   `json:"canonical"`
	RawHex    string `json:"raw_hex"`
	Reason    string `json:"reason"`
}

// segmentBlocks names the building block that spells one segment of each set.
var segmentBlocks = map[string]string{"go-element": "GO_ELEM", "source-path": "TS_SEG", "file-operand": "OP_SEG"}

// decodeSegment is the bytes a spelling's escapes decode to, every other byte
// as itself; ok is false where a % opens no escape with uppercase digits.
func decodeSegment(spelled string) ([]byte, bool) {
	var out []byte
	for i := 0; i < len(spelled); i++ {
		if spelled[i] != '%' {
			out = append(out, spelled[i])
			continue
		}
		if i+2 >= len(spelled) || strings.ToUpper(spelled[i+1:i+3]) != spelled[i+1:i+3] {
			return nil, false
		}
		b, err := hex.DecodeString(spelled[i+1 : i+3])
		if err != nil {
			return nil, false
		}
		out = append(out, b...)
		i += 2
	}
	return out, true
}

func TestSegmentCorpusSpellsEachSetByItsBlock(t *testing.T) {
	var cases []segmentCase
	grammarCorpus(t, segmentCorpusPath, &cases)
	blocks := map[string]*regexp.Regexp{}
	for _, block := range symbolRefBuildingBlocks(t, grammarPage(t, symbolRefPagePath)) {
		for set, name := range segmentBlocks {
			if block.name == name {
				blocks[set] = compileExpression(t, name, "^"+block.expr+"$")
			}
		}
	}
	if len(blocks) != len(segmentBlocks) {
		t.Fatalf("Setup: symbol-ref.md publishes %d of the segment blocks %v", len(blocks), slices.Sorted(maps.Values(segmentBlocks)))
	}
	seen := map[string][2]int{}
	for i, c := range cases {
		block, ok := blocks[c.Set]
		if !ok {
			t.Errorf("case %d names the set %q, want one of %v", i, c.Set, slices.Sorted(maps.Keys(segmentBlocks)))
			continue
		}
		if c.Reason == "" {
			t.Errorf("case %d (%q) carries no reason, want one", i, c.Spelled)
		}
		counts := seen[c.Set]
		if !c.Canonical {
			counts[1]++
			seen[c.Set] = counts
			if c.RawHex != "" {
				t.Errorf("non-canonical case %d (%q) carries raw_hex, want it on canonical cases only", i, c.Spelled)
			}
			continue
		}
		counts[0]++
		seen[c.Set] = counts
		raw, err := hex.DecodeString(c.RawHex)
		if err != nil || c.RawHex != strings.ToLower(c.RawHex) || len(raw) == 0 {
			t.Errorf("canonical case %d (%q) carries raw_hex %q, want lowercase hexadecimal of at least one byte", i, c.Spelled, c.RawHex)
			continue
		}
		if !block.MatchString(c.Spelled) {
			t.Errorf("the %s block refuses canonical case %d (%q), want a match", segmentBlocks[c.Set], i, c.Spelled)
		}
		if !utf8.ValidString(c.Spelled) {
			t.Errorf("canonical case %d (%q) is not well-formed UTF-8, want every reference well formed", i, c.Spelled)
		}
		if decoded, ok := decodeSegment(c.Spelled); !ok || string(decoded) != string(raw) {
			t.Errorf("canonical case %d (%q) decodes to %x, want raw_hex %s", i, c.Spelled, decoded, c.RawHex)
		}
	}
	for _, set := range slices.Sorted(maps.Keys(segmentBlocks)) {
		if counts := seen[set]; counts[0] == 0 || counts[1] == 0 {
			t.Errorf("the corpus carries %d canonical and %d non-canonical cases of %s, want at least one of each", counts[0], counts[1], set)
		}
	}
}

// segmentSets is each set of the segment package by the name the corpus gives it.
var segmentSets = map[string]segment.Set{"go-element": segment.GoElement, "source-path": segment.SourcePath, "file-operand": segment.FileOperand}

// The encoder keeps a character as itself exactly where its set's block admits
// it, so what a writer spells is what every document's expression reads.
func TestSpellKeepsExactlyWhatEachSegmentBlockAdmits(t *testing.T) {
	blocks := map[string]*regexp.Regexp{}
	for _, block := range symbolRefBuildingBlocks(t, grammarPage(t, symbolRefPagePath)) {
		for set, name := range segmentBlocks {
			if block.name == name {
				blocks[set] = compileExpression(t, name, "^"+block.expr+"$")
			}
		}
	}
	for _, name := range slices.Sorted(maps.Keys(segmentSets)) {
		set := segmentSets[name]
		for _, r := range drawCandidates {
			raw := string(r)
			kept := segment.Spell(set, raw) == raw
			if admitted := blocks[name].MatchString(raw); kept != admitted {
				t.Errorf("segment.Spell(%s, %U) keeps it as itself: %t, but the %s block admits it: %t", name, r, kept, segmentBlocks[name], admitted)
			}
		}
	}
}
