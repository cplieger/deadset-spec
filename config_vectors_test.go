package spec_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v2"
)

const (
	configVectorsDir     = "vectors/config"
	vectorCaseFile       = "case.json"
	vectorRepositoryFile = "repository.json"
	vectorCentralFile    = "central.json"
	vectorFlagsFile      = "flags.json"
	vectorExpectedFile   = "expected.json"
	vectorErrorFile      = "expected-error.json"
)

// configVectorAspects is the closed set of aspects the published case set covers. Every case
// names at least one of them and every one of them is named by at least one case.
var configVectorAspects = []string{
	"array-spanning-lines",
	"duplicated-key",
	"missing-target-kind",
	"provenance-on-input",
	"quoted-key-with-a-dot",
	"resolved-configuration-round-trip",
	"template-delimiters-configured",
	"template-delimiters-half",
	"unimplemented-key",
}

// vectorConfigFiles are the case files that hold a configuration document: the inputs and the
// resolved configuration, all instances of the closed key list.
var vectorConfigFiles = []string{vectorRepositoryFile, vectorCentralFile, vectorExpectedFile}

// vectorCaseFiles is every file name a case directory may hold.
var vectorCaseFiles = []string{
	vectorCaseFile,
	vectorRepositoryFile,
	vectorCentralFile,
	vectorFlagsFile,
	vectorExpectedFile,
	vectorErrorFile,
}

// configVectorCase mirrors a case.json closely enough that an unknown key fails the decode.
type configVectorCase struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Covers      []string `json:"covers"`
}

// configVectorError mirrors an expected-error.json.
type configVectorError struct {
	Names    string `json:"names"`
	ExitCode int    `json:"exit_code"`
}

// vectorSchemaIndex is every subschema of config.schema.json by the path walkSchema gives it.
type vectorSchemaIndex map[string]declaredKey

func newVectorSchemaIndex(t *testing.T) vectorSchemaIndex {
	t.Helper()
	index := vectorSchemaIndex{}
	for _, key := range walkSchema(loadConfigSchema(t)) {
		index[key.path] = key
	}
	return index
}

// configVectorDirs lists the case directories under vectors/config, in name order.
func configVectorDirs(t *testing.T) []string {
	t.Helper()
	entries, err := fs.ReadDir(spec.Vectors, configVectorsDir)
	if err != nil {
		t.Fatalf("Setup: fs.ReadDir(Vectors, %q): %v", configVectorsDir, err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(dirs) == 0 {
		t.Fatalf("Setup: fs.ReadDir(Vectors, %q) lists no case directory", configVectorsDir)
	}
	return dirs
}

// readVectorFile reads one file of a case and reports whether the case holds it. Any error
// other than absence fails the test.
func readVectorFile(t *testing.T, dir, name string) ([]byte, bool) {
	t.Helper()
	p := configVectorsDir + "/" + dir + "/" + name
	data, err := fs.ReadFile(spec.Vectors, p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, false
	case err != nil:
		t.Fatalf("Setup: fs.ReadFile(Vectors, %q): %v", p, err)
	}
	return data, true
}

// loadVectorCase decodes a case's case.json, failing the test on a missing or malformed one.
func loadVectorCase(t *testing.T, dir string) configVectorCase {
	t.Helper()
	data, ok := readVectorFile(t, dir, vectorCaseFile)
	if !ok {
		t.Fatalf("Setup: %s/%s/%s is absent, want every case to declare itself", configVectorsDir, dir, vectorCaseFile)
	}
	var c configVectorCase
	if err := decodeStrict(data, &c); err != nil {
		t.Fatalf("Setup: decoding %s/%s/%s: %v", configVectorsDir, dir, vectorCaseFile, err)
	}
	return c
}

// vectorDecodeConfig decodes one configuration document and refuses anything that follows it,
// so a file holding two concatenated documents fails here rather than resolving to the first.
func vectorDecodeConfig(data []byte) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	var doc map[string]any
	if err := dec.Decode(&doc); err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errors.New("want a JSON object, got null")
	}
	var rest json.RawMessage
	switch err := dec.Decode(&rest); {
	case errors.Is(err, io.EOF):
		return doc, nil
	case err != nil:
		return nil, fmt.Errorf("reading past the document: %w", err)
	default:
		return nil, fmt.Errorf("want one document, a second one follows it: %s", rest)
	}
}

// vectorTopLevelKeys lists the keys of a document's top-level object in the order the document
// writes them, a key written twice appearing twice.
func vectorTopLevelKeys(data []byte) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("want a JSON object, got %v", tok)
	}
	var keys []string
	for dec.More() {
		tok, err = dec.Token()
		if err != nil {
			return nil, err
		}
		key, ok := tok.(string)
		if !ok {
			return nil, fmt.Errorf("want an object key, got %v", tok)
		}
		keys = append(keys, key)
		var value json.RawMessage
		if err = dec.Decode(&value); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// vectorCanonicalJSON re-encodes a document so two documents are compared by the values they
// carry rather than by their indentation.
func vectorCanonicalJSON(t *testing.T, where string, data []byte) []byte {
	t.Helper()
	doc, err := vectorDecodeConfig(data)
	if err != nil {
		t.Fatalf("Setup: decoding %s: %v", where, err)
	}
	out, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Setup: json.Marshal(%s): %v", where, err)
	}
	return out
}

func vectorCompilePattern(t *testing.T, schemaPath, pattern string) *regexp.Regexp {
	t.Helper()
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Fatalf("Setup: regexp.Compile(schema[%s].pattern = %q): %v", schemaPath, pattern, err)
	}
	return re
}

// vectorPatternDeclares reports whether one of an object's patternProperties keys accepts name.
func vectorPatternDeclares(t *testing.T, schemaPath string, patternProps map[string]any, name string) bool {
	t.Helper()
	for _, pattern := range slices.Sorted(maps.Keys(patternProps)) {
		if vectorCompilePattern(t, schemaPath, pattern).MatchString(name) {
			return true
		}
	}
	return false
}

// vectorCheckAgainstSchema reports every problem vectorSchemaProblems finds in one document.
func vectorCheckAgainstSchema(t *testing.T, index vectorSchemaIndex, where, at, schemaPath string, value any) {
	t.Helper()
	for _, problem := range vectorSchemaProblems(t, index, where, at, schemaPath, value) {
		t.Error(problem)
	}
}

// vectorSchemaProblems walks one document value against the subschema walkSchema indexed at
// schemaPath, listing every key config.schema.json does not declare, every required key the
// document omits and every string a declared pattern or enumeration refuses. at is the path of
// value inside the document and where names the document. It returns the problems rather than
// reporting them, so a planted document drives the same walk the published vectors are checked
// with; a schema whose pattern does not compile is a setup failure of this suite instead.
func vectorSchemaProblems(t *testing.T, index vectorSchemaIndex, where, at, schemaPath string, value any) []string {
	t.Helper()
	entry, declared := index[schemaPath]
	if !declared || entry.node == nil {
		return []string{fmt.Sprintf("%s: %s reaches %s, which %s does not declare", where, at, schemaPath, configSchemaPath)}
	}
	var out []string
	if text, isText := value.(string); isText {
		if pattern, _ := entry.node["pattern"].(string); pattern != "" {
			if !vectorCompilePattern(t, schemaPath, pattern).MatchString(text) {
				out = append(out, fmt.Sprintf("%s: %s = %q, want a value matching schema[%s].pattern = %q", where, at, text, schemaPath, pattern))
			}
		}
		if enum := stringSlice(entry.node["enum"]); len(enum) > 0 && !slices.Contains(enum, text) {
			out = append(out, fmt.Sprintf("%s: %s = %q, want one of schema[%s].enum = %q", where, at, text, schemaPath, enum))
		}
	}
	switch v := value.(type) {
	case map[string]any:
		for _, name := range stringSlice(entry.node["required"]) {
			if _, present := v[name]; !present {
				out = append(out, fmt.Sprintf("%s: %s omits %q, want every key schema[%s].required names", where, at, name, schemaPath))
			}
		}
		props, _ := entry.node["properties"].(map[string]any)
		patternProps, _ := entry.node["patternProperties"].(map[string]any)
		for _, name := range slices.Sorted(maps.Keys(v)) {
			if _, isProperty := props[name]; isProperty {
				out = append(out, vectorSchemaProblems(t, index, where, joinPath(at, name), joinPath(schemaPath, name), v[name])...)
				continue
			}
			if !vectorPatternDeclares(t, schemaPath, patternProps, name) {
				out = append(out, fmt.Sprintf("%s: %s names %q, which %s does not declare under %s", where, at, name, configSchemaPath, schemaPath))
				continue
			}
			out = append(out, vectorSchemaProblems(t, index, where, joinPath(at, name), joinPath(schemaPath, "<pattern>"), v[name])...)
		}
	case []any:
		for i, item := range v {
			out = append(out, vectorSchemaProblems(t, index, where, fmt.Sprintf("%s[%d]", at, i), joinPath(schemaPath, "<items>"), item)...)
		}
	}
	return out
}

// caseCovering returns the one case that names an aspect, failing the test when no case or more
// than one case names it.
func caseCovering(t *testing.T, aspect string) string {
	t.Helper()
	var dirs []string
	for _, dir := range configVectorDirs(t) {
		if slices.Contains(loadVectorCase(t, dir).Covers, aspect) {
			dirs = append(dirs, dir)
		}
	}
	if len(dirs) != 1 {
		t.Fatalf("Setup: cases covering %q = %q, want exactly one", aspect, dirs)
	}
	return dirs[0]
}

func TestConfigVectorsCoverEveryAspectOnce(t *testing.T) {
	covered := map[string][]string{}
	for _, dir := range configVectorDirs(t) {
		c := loadVectorCase(t, dir)
		t.Run(dir, func(t *testing.T) {
			if strings.TrimSpace(c.Name) == "" {
				t.Errorf("%s/%s/%s: name = %q, want a non-empty name", configVectorsDir, dir, vectorCaseFile, c.Name)
			}
			if strings.TrimSpace(c.Description) == "" {
				t.Errorf("%s/%s/%s: description = %q, want a sentence stating what the case establishes", configVectorsDir, dir, vectorCaseFile, c.Description)
			}
			if len(c.Covers) == 0 {
				t.Errorf("%s/%s/%s: covers = %q, want at least one aspect", configVectorsDir, dir, vectorCaseFile, c.Covers)
			}
			for _, aspect := range c.Covers {
				if !slices.Contains(configVectorAspects, aspect) {
					t.Errorf("%s/%s/%s: covers names %q, want one of %q", configVectorsDir, dir, vectorCaseFile, aspect, configVectorAspects)
				}
			}
		})
		for _, aspect := range c.Covers {
			covered[aspect] = append(covered[aspect], dir)
		}
	}
	for _, aspect := range configVectorAspects {
		if len(covered[aspect]) == 0 {
			t.Errorf("cases covering %q = none, want at least one under %s", aspect, configVectorsDir)
		}
	}
}

func TestConfigVectorCasesCarryOneOutcome(t *testing.T) {
	for _, dir := range configVectorDirs(t) {
		t.Run(dir, func(t *testing.T) {
			entries, err := fs.ReadDir(spec.Vectors, configVectorsDir+"/"+dir)
			if err != nil {
				t.Fatalf("Setup: fs.ReadDir(Vectors, %q): %v", configVectorsDir+"/"+dir, err)
			}
			for _, entry := range entries {
				if !slices.Contains(vectorCaseFiles, entry.Name()) {
					t.Errorf("%s/%s holds %q, want only %q", configVectorsDir, dir, entry.Name(), vectorCaseFiles)
				}
			}
			_, resolved := readVectorFile(t, dir, vectorExpectedFile)
			_, refused := readVectorFile(t, dir, vectorErrorFile)
			if resolved == refused {
				t.Errorf("%s/%s holds %s = %t and %s = %t, want exactly one of them", configVectorsDir, dir, vectorExpectedFile, resolved, vectorErrorFile, refused)
			}
			if _, ok := readVectorFile(t, dir, vectorRepositoryFile); !ok {
				if _, central := readVectorFile(t, dir, vectorCentralFile); !central {
					t.Errorf("%s/%s holds neither %s nor %s, want at least one input document", configVectorsDir, dir, vectorRepositoryFile, vectorCentralFile)
				}
			}
		})
	}
}

func TestConfigVectorDocumentsDecodeAsOneObject(t *testing.T) {
	for _, dir := range configVectorDirs(t) {
		t.Run(dir, func(t *testing.T) {
			for _, name := range append(slices.Clone(vectorConfigFiles), vectorFlagsFile) {
				data, ok := readVectorFile(t, dir, name)
				if !ok {
					continue
				}
				if _, err := vectorDecodeConfig(data); err != nil {
					t.Errorf("decoding %s/%s/%s: %v, want one JSON object", configVectorsDir, dir, name, err)
				}
			}
		})
	}
}

func TestConfigVectorExpectationsNameOnlyDeclaredKeys(t *testing.T) {
	index := newVectorSchemaIndex(t)
	for _, dir := range configVectorDirs(t) {
		data, ok := readVectorFile(t, dir, vectorExpectedFile)
		if !ok {
			continue
		}
		t.Run(dir, func(t *testing.T) {
			where := fmt.Sprintf("%s/%s/%s", configVectorsDir, dir, vectorExpectedFile)
			doc, err := vectorDecodeConfig(data)
			if err != nil {
				t.Fatalf("Setup: decoding %s: %v", where, err)
			}
			vectorCheckAgainstSchema(t, index, where, "root", "root", doc)
		})
	}
}

func TestConfigVectorRefusalsNameATableCode(t *testing.T) {
	table := mustLoadExitCodes(t)
	codes := make([]int, 0, len(table.ExitCodes))
	for _, row := range table.ExitCodes {
		codes = append(codes, row.Code)
	}
	for _, dir := range configVectorDirs(t) {
		data, ok := readVectorFile(t, dir, vectorErrorFile)
		if !ok {
			continue
		}
		t.Run(dir, func(t *testing.T) {
			where := fmt.Sprintf("%s/%s/%s", configVectorsDir, dir, vectorErrorFile)
			var refusal configVectorError
			if err := decodeStrict(data, &refusal); err != nil {
				t.Fatalf("Setup: decoding %s: %v", where, err)
			}
			if !slices.Contains(codes, refusal.ExitCode) {
				t.Errorf("%s: exit_code = %d, want one of %v, the codes %s names", where, refusal.ExitCode, codes, exitCodesPath)
			}
			if strings.TrimSpace(refusal.Names) == "" {
				t.Errorf("%s: names = %q, want the key or field the message names", where, refusal.Names)
			}
		})
	}
}

// TestConfigVectorDuplicatedKeyCaseHoldsADuplicatedKey pins the input the case exists for: a
// document that names one key twice, which is the key the refusal names.
func TestConfigVectorDuplicatedKeyCaseHoldsADuplicatedKey(t *testing.T) {
	dir := caseCovering(t, "duplicated-key")
	where := fmt.Sprintf("%s/%s/%s", configVectorsDir, dir, vectorRepositoryFile)
	data, ok := readVectorFile(t, dir, vectorRepositoryFile)
	if !ok {
		t.Fatalf("Setup: %s is absent, want the duplicated key in the repository configuration", where)
	}
	keys, err := vectorTopLevelKeys(data)
	if err != nil {
		t.Fatalf("Setup: vectorTopLevelKeys(%s): %v", where, err)
	}
	counts := map[string]int{}
	for _, key := range keys {
		counts[key]++
	}
	var repeated []string
	for _, key := range slices.Sorted(maps.Keys(counts)) {
		if counts[key] > 1 {
			repeated = append(repeated, key)
		}
	}
	if len(repeated) != 1 {
		t.Fatalf("%s top-level keys = %q, want exactly one of them written twice", where, keys)
	}
	names := refusalNames(t, dir)
	if repeated[0] != names {
		t.Errorf("%s writes %q twice while the refusal names %q, want the refusal to name the duplicated key", where, repeated[0], names)
	}
}

// TestConfigVectorQuotedKeyCaseHoldsADottedKey pins the input the case exists for: a document
// whose top-level key is a dotted setting path, which is the key the refusal names.
func TestConfigVectorQuotedKeyCaseHoldsADottedKey(t *testing.T) {
	dir := caseCovering(t, "quoted-key-with-a-dot")
	where := fmt.Sprintf("%s/%s/%s", configVectorsDir, dir, vectorRepositoryFile)
	data, ok := readVectorFile(t, dir, vectorRepositoryFile)
	if !ok {
		t.Fatalf("Setup: %s is absent, want the dotted key in the repository configuration", where)
	}
	keys, err := vectorTopLevelKeys(data)
	if err != nil {
		t.Fatalf("Setup: vectorTopLevelKeys(%s): %v", where, err)
	}
	var dotted []string
	for _, key := range keys {
		if strings.Contains(key, ".") {
			dotted = append(dotted, key)
		}
	}
	if len(dotted) != 1 {
		t.Fatalf("%s top-level keys = %q, want exactly one of them spelled with a dot", where, keys)
	}
	if names := refusalNames(t, dir); dotted[0] != names {
		t.Errorf("%s spells %q at the top level while the refusal names %q, want the refusal to name that key", where, dotted[0], names)
	}
}

// refusalNames reads the key a case's refusal names.
func refusalNames(t *testing.T, dir string) string {
	t.Helper()
	where := fmt.Sprintf("%s/%s/%s", configVectorsDir, dir, vectorErrorFile)
	data, ok := readVectorFile(t, dir, vectorErrorFile)
	if !ok {
		t.Fatalf("Setup: %s is absent, want a refused case to name its exit code", where)
	}
	var refusal configVectorError
	if err := decodeStrict(data, &refusal); err != nil {
		t.Fatalf("Setup: decoding %s: %v", where, err)
	}
	return refusal.Names
}

// TestConfigVectorRoundTripResolvesToItself pins the round trip: the case reads the resolved
// configuration of the provenance case back as a repository configuration and resolves to that
// same document, so a resolved configuration is valid input under the closed key list.
func TestConfigVectorRoundTripResolvesToItself(t *testing.T) {
	source := caseCovering(t, "provenance-on-input")
	roundTrip := caseCovering(t, "resolved-configuration-round-trip")
	resolved, ok := readVectorFile(t, source, vectorExpectedFile)
	if !ok {
		t.Fatalf("Setup: %s/%s/%s is absent, want the resolved configuration the round trip reads", configVectorsDir, source, vectorExpectedFile)
	}
	want := vectorCanonicalJSON(t, source+"/"+vectorExpectedFile, resolved)
	for _, name := range []string{vectorRepositoryFile, vectorExpectedFile} {
		where := fmt.Sprintf("%s/%s/%s", configVectorsDir, roundTrip, name)
		data, present := readVectorFile(t, roundTrip, name)
		if !present {
			t.Fatalf("Setup: %s is absent, want the round trip to hold it", where)
		}
		got := vectorCanonicalJSON(t, roundTrip+"/"+name, data)
		if bytes.Equal(got, want) {
			continue
		}
		for _, key := range vectorDifferingKeys(t, "root", got, want) {
			t.Errorf("%s: %s = %s, want %s, the value %s/%s/%s carries", where, key.path, key.got, key.want, configVectorsDir, source, vectorExpectedFile)
		}
	}
}

// vectorKeyDifference is one key on which two documents disagree.
type vectorKeyDifference struct {
	path string
	got  string
	want string
}

// vectorDifferingKeys lists the keys on which two canonical documents disagree, descending into
// an object both sides carry, so a failure names the keys rather than printing both documents.
func vectorDifferingKeys(t *testing.T, at string, got, want []byte) []vectorKeyDifference {
	t.Helper()
	gotDoc, wantDoc := map[string]json.RawMessage{}, map[string]json.RawMessage{}
	for _, side := range []struct {
		data []byte
		into *map[string]json.RawMessage
	}{{got, &gotDoc}, {want, &wantDoc}} {
		if err := json.Unmarshal(side.data, side.into); err != nil {
			t.Fatalf("Setup: json.Unmarshal(%s): %v", side.data, err)
		}
	}
	names := slices.Sorted(maps.Keys(gotDoc))
	for _, name := range slices.Sorted(maps.Keys(wantDoc)) {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	var out []vectorKeyDifference
	for _, name := range names {
		gotValue, wantValue := gotDoc[name], wantDoc[name]
		if string(gotValue) == string(wantValue) {
			continue
		}
		path := joinPath(at, name)
		if vectorIsObject(gotValue) && vectorIsObject(wantValue) {
			out = append(out, vectorDifferingKeys(t, path, gotValue, wantValue)...)
			continue
		}
		out = append(out, vectorKeyDifference{
			path: path,
			got:  vectorValueOrAbsent(gotDoc, name),
			want: vectorValueOrAbsent(wantDoc, name),
		})
	}
	return out
}

// vectorIsObject reports whether a raw value is a JSON object.
func vectorIsObject(value json.RawMessage) bool {
	trimmed := bytes.TrimSpace(value)
	return len(trimmed) > 0 && trimmed[0] == '{'
}

func vectorValueOrAbsent(doc map[string]json.RawMessage, name string) string {
	value, present := doc[name]
	if !present {
		return "absent"
	}
	return string(value)
}

// plantedVectorConfiguration is a resolved configuration in memory, valid under the closed key
// list, so the walk over an expected resolved configuration is exercised in the accepting
// direction on a document this file owns and in the refusing direction on a planted copy of it.
const plantedVectorConfiguration = `{"contract_version": "1.0.0", "target": {"kind": "library"}, ` +
	`"analysis": {"languages": ["go"], "min_confidence": "probable", "generated_files": "exclude"}, ` +
	`"severity": {"DS1101": "warn", "DS18": "allow"}, ` +
	`"reporters": {"formats": ["text"], "sort": "size", "fail_on": "deny"}}`

func TestVectorSchemaProblemsAcceptsAResolvedConfiguration(t *testing.T) {
	index := newVectorSchemaIndex(t)
	doc, err := vectorDecodeConfig([]byte(plantedVectorConfiguration))
	if err != nil {
		t.Fatalf("Setup: decoding the planted configuration: %v", err)
	}
	for _, problem := range vectorSchemaProblems(t, index, "planted", "root", "root", doc) {
		t.Errorf("vectorSchemaProblems(a valid resolved configuration) reports %q, want no problem", problem)
	}
}

// TestVectorSchemaProblemsRefuses plants one violation per arm of the walk, so each arm is known to
// fire rather than only known to stay silent on documents that are already correct.
func TestVectorSchemaProblemsRefuses(t *testing.T) {
	cases := []struct {
		name    string
		old     string
		planted string
		wantMsg string
	}{
		{
			name:    "a_key_the_schema_does_not_declare",
			old:     `"target": {"kind": "library"}`,
			planted: `"target": {"kind": "library"}, "analyzers": ["deadset-go"]`,
			wantMsg: `root names "analyzers", which contract/config.schema.json does not declare under root`,
		},
		{
			name:    "a_nested_key_the_schema_does_not_declare",
			old:     `"min_confidence": "probable"`,
			planted: `"min_confidence": "probable", "sort": "size"`,
			wantMsg: `analysis names "sort", which contract/config.schema.json does not declare under analysis`,
		},
		{
			name:    "a_required_key_the_document_omits",
			old:     `"target": {"kind": "library"}`,
			planted: `"target": {}`,
			wantMsg: `target omits "kind", want every key schema[target].required names`,
		},
		{
			name:    "a_value_outside_the_enumeration",
			old:     `"target": {"kind": "library"}`,
			planted: `"target": {"kind": "framework"}`,
			wantMsg: `target.kind = "framework", want one of schema[target.kind].enum`,
		},
		{
			name:    "a_severity_key_the_code_pattern_refuses",
			old:     `"DS1101": "warn"`,
			planted: `"ds1101": "warn"`,
			wantMsg: `severity names "ds1101", which contract/config.schema.json does not declare under severity`,
		},
		{
			name:    "an_array_entry_the_enumeration_refuses",
			old:     `"languages": ["go"]`,
			planted: `"languages": ["gopher"]`,
			wantMsg: `analysis.languages[0] = "gopher", want one of schema[analysis.languages.<items>].enum`,
		},
	}
	index := newVectorSchemaIndex(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(plantedVectorConfiguration, tc.old) {
				t.Fatalf("Setup: the planted configuration does not hold %q", tc.old)
			}
			doc, err := vectorDecodeConfig([]byte(strings.Replace(plantedVectorConfiguration, tc.old, tc.planted, 1)))
			if err != nil {
				t.Fatalf("Setup: decoding the planted configuration: %v", err)
			}
			problems := vectorSchemaProblems(t, index, "planted", "root", "root", doc)
			if !slices.ContainsFunc(problems, func(problem string) bool { return strings.Contains(problem, tc.wantMsg) }) {
				t.Errorf("vectorSchemaProblems(planted with %s) = %q, want a problem containing %q", tc.name, problems, tc.wantMsg)
			}
		})
	}
}
