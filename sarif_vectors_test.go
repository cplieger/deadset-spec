package spec_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"unicode/utf16"
	"unicode/utf8"

	spec "github.com/cplieger/deadset-spec/v7"
)

// sarifVectorsDir holds one directory per published SARIF case.
const sarifVectorsDir = "vectors/sarif"

// The files of a SARIF case: the report rendered, the input reports a merged report
// was merged from, the target files the line fingerprints read, and either the
// expected document or the exit code the rendering ends with.
const (
	sarifReportFile   = "report.json"
	sarifInputsDir    = "inputs"
	sarifSourcesFile  = "sources.json"
	sarifExpectedFile = "expected.json"
	sarifExitFile     = "expected_exit"
)

// sarifFailedExit is the exit code of a rendering that fails after the report was written.
const sarifFailedExit = 3

// The fixed values grammar/sarif.md states for every document and every run.
const (
	sarifSchemaURI       = "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json"
	sarifRootDescription = "The target root."
	sarifMergeRunID      = "deadset/merge/"
	sarifMaxRelated      = 100
)

// sarifBagMembers are the finding members a result carries under properties.
var sarifBagMembers = []string{
	"language", "symbol", "reachability_class", "confidence", "liveness_relation", "test_only",
	"generated", "component", "retained_by", "configurations", "consumers_loaded", "fixability", "details",
}

// errSARIFRendering is a rendering grammar/sarif.md ends with exit code 3.
var errSARIFRendering = errors.New("the rendering fails")

// sarifCase is one published case, read from its directory.
type sarifCase struct {
	sources  map[string]string
	expected any
	report   []byte
	inputs   [][]byte
	exit     int
}

// sarifRecord is a finding or a stale-suppression record, read as the document holds it.
type sarifRecord = map[string]any

// sarifMergedEntry is one merged_from entry, the run it names.
type sarifMergedEntry struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// sarifReport is the part of a report the rendering reads.
type sarifReport struct {
	Totals   any `json:"totals"`
	Analyzer struct {
		Name      string   `json:"name"`
		Version   string   `json:"version"`
		Languages []string `json:"languages"`
	} `json:"analyzer"`
	MergedFrom        []sarifMergedEntry `json:"merged_from"`
	Findings          []sarifRecord      `json:"findings"`
	StaleSuppressions []sarifRecord      `json:"stale_suppressions"`
}

// sarifRenderer renders reports with one kinds vocabulary.
type sarifRenderer struct {
	sources map[string]string
	hashes  map[string][]string
	kinds   []kindRow
}

// sarifLevel maps a severity to a result level, and sarifProblem to a problem severity.
var (
	sarifLevel     = map[string]string{"deny": "error", "warn": "warning", "allow": "note"}
	sarifProblem   = map[string]string{"deny": "error", "warn": "warning", "allow": "recommendation"}
	sarifPrecision = map[string]string{"certain": "very-high", "probable": "high", "possible": "medium"}
)

// renderSARIF renders one report as grammar/sarif.md maps it: one run for an analyzer's
// report, one run per merged_from entry and the merge's own where a record names no
// analyzer for a merged one.
func (r *sarifRenderer) renderSARIF(document []byte, inputs [][]byte) (map[string]any, error) {
	var rep sarifReport
	if err := json.Unmarshal(document, &rep); err != nil {
		return nil, err
	}
	log := map[string]any{"$schema": sarifSchemaURI, "version": "2.1.0"}
	if len(rep.MergedFrom) == 0 {
		run, err := r.run(rep.Analyzer.Name, rep.Analyzer.Version, rep.Analyzer.Languages, rep.Totals, rep.Findings, rep.StaleSuppressions)
		if err != nil {
			return nil, err
		}
		log["runs"] = []any{run}
		return log, nil
	}
	runs, err := r.mergedRuns(&rep, inputs)
	if err != nil {
		return nil, err
	}
	log["runs"] = runs
	log["properties"] = sarifProperties(rep.Totals)
	return log, nil
}

// sarifProperties is the properties bag of a run or of a merged log: the totals
// unchanged, and the withheld line text-line.md defines where a count of
// totals.withheld is not 0.
func sarifProperties(totals any) map[string]any {
	properties := map[string]any{"totals": totals}
	if line := withheldLine(totals); line != "" {
		properties["withheld"] = line
	}
	return properties
}

// withheldLine renders the line text-line.md defines from a report's totals: each
// confidence whose withheld count is not 0, from probable to possible, and the
// setting that shows them all. It is empty where every count is 0.
func withheldLine(totals any) string {
	counts, _ := totals.(map[string]any)["withheld"].(map[string]any)
	var named []string
	lowest := ""
	for _, level := range withheldLevels {
		n, _ := counts[level].(float64)
		if n == 0 {
			continue
		}
		named = append(named, strconv.Itoa(int(n))+" "+level)
		lowest = level
	}
	if len(named) == 0 {
		return ""
	}
	return "withheld by analysis.min_confidence: " + strings.Join(named, ", ") + ", shown with analysis.min_confidence set to " + lowest
}

// mergedRuns is one run per merged_from entry in bytewise order of name, then the
// merge's own run when a record names no analyzer.
func (r *sarifRenderer) mergedRuns(rep *sarifReport, inputs [][]byte) ([]any, error) {
	byName := map[string]sarifReport{}
	for _, raw := range inputs {
		var in sarifReport
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, err
		}
		byName[in.Analyzer.Name] = in
	}
	names := map[string]bool{}
	entries := slices.Clone(rep.MergedFrom)
	slices.SortFunc(entries, func(a, b sarifMergedEntry) int { return strings.Compare(a.Name, b.Name) })
	var runs []any
	for _, entry := range entries {
		in, ok := byName[entry.Name]
		if !ok {
			return nil, fmt.Errorf("no input report of %s", entry.Name)
		}
		names[entry.Name] = true
		run, err := r.run(entry.Name, entry.Version, in.Analyzer.Languages, in.Totals,
			carriedBy(rep.Findings, entry.Name), carriedBy(rep.StaleSuppressions, entry.Name))
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	for _, record := range slices.Concat(rep.Findings, rep.StaleSuppressions) {
		if name, ok := record["analyzer"].(string); ok && !names[name] {
			return nil, fmt.Errorf("%w: a record names the analyzer %s, which merged_from does not", errSARIFRendering, name)
		}
	}
	own, stale := carriedBy(rep.Findings, ""), carriedBy(rep.StaleSuppressions, "")
	if len(own)+len(stale) > 0 {
		run, err := r.run(rep.Analyzer.Name, rep.Analyzer.Version, nil, rep.Totals, own, stale)
		if err != nil {
			return nil, err
		}
		run["automationDetails"] = map[string]any{"id": sarifMergeRunID}
		runs = append(runs, run)
	}
	return runs, nil
}

// carriedBy is the records whose analyzer member is name, the records with none for "".
func carriedBy(records []sarifRecord, name string) []sarifRecord {
	var held []sarifRecord
	for _, record := range records {
		if analyzer, _ := record["analyzer"].(string); analyzer == name {
			held = append(held, record)
		}
	}
	return held
}

// run is one run: its driver, its rules, its automation identifier, its results and
// its totals. A nil languages list is the merge's own run, which lists every live kind.
func (r *sarifRenderer) run(name, version string, languages []string, totals any, findings, stale []sarifRecord) (map[string]any, error) {
	rules, index := r.rules(languages)
	results := []any{}
	for _, found := range findings {
		result, err := r.findingResult(found, index)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	for _, record := range stale {
		result, err := r.staleResult(record, index)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	sorted := slices.Sorted(slices.Values(languages))
	return map[string]any{
		"tool": map[string]any{"driver": map[string]any{
			"name": name, "version": version, "semanticVersion": version, "rules": rules,
		}},
		"automationDetails":  map[string]any{"id": "deadset/" + strings.Join(sorted, "+") + "/"},
		"columnKind":         "utf16CodeUnits",
		"originalUriBaseIds": map[string]any{"%SRCROOT%": map[string]any{"description": map[string]any{"text": sarifRootDescription}}},
		"results":            results,
		"properties":         sarifProperties(totals),
	}, nil
}

// rules is one descriptor per live kind of the languages, in bytewise order of code,
// and the index of each code.
func (r *sarifRenderer) rules(languages []string) ([]any, map[string]int) {
	kinds := slices.Clone(r.kinds)
	slices.SortFunc(kinds, func(a, b kindRow) int { return strings.Compare(a.Code, b.Code) })
	var rules []any
	index := map[string]int{}
	for i := range kinds {
		kind := &kinds[i]
		if languages != nil && !slices.ContainsFunc(languages, func(l string) bool { return slices.Contains(kind.Languages, l) }) {
			continue
		}
		help := kind.Rule
		if kind.Precondition != "" {
			help += "\n\n" + kind.Precondition
		}
		index[kind.Code] = len(rules)
		rules = append(rules, map[string]any{
			"id":                   kind.Code,
			"name":                 kind.Name,
			"shortDescription":     map[string]any{"text": firstSentence(kind.Rule)},
			"fullDescription":      map[string]any{"text": kind.Rule},
			"help":                 map[string]any{"text": help},
			"defaultConfiguration": map[string]any{"level": sarifLevel[kind.DefaultSeverity]},
			"properties": map[string]any{
				"precision": sarifPrecision[kind.MaxClass],
				"problem":   map[string]any{"severity": sarifProblem[kind.DefaultSeverity]},
			},
		})
	}
	return rules, index
}

// firstSentence is the text up to and including the first full stop a space follows
// or that ends the text.
func firstSentence(text string) string {
	for i := range len(text) {
		if text[i] == '.' && (i+1 == len(text) || text[i+1] == ' ') {
			return text[:i+1]
		}
	}
	return text
}

// sarifPosition is a position as a record holds it.
type sarifPosition struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	EndLine int    `json:"end_line"`
}

// positionOf decodes one position-shaped value.
func positionOf(value any) (sarifPosition, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return sarifPosition{}, err
	}
	var at sarifPosition
	if err := json.Unmarshal(raw, &at); err != nil {
		return sarifPosition{}, err
	}
	if at.EndLine == 0 {
		at.EndLine = at.Line
	}
	return at, nil
}

// location is one position as a physical location against %SRCROOT%.
func location(at sarifPosition) map[string]any {
	return map[string]any{
		"artifactLocation": map[string]any{"uri": relativeReference(at.Path), "uriBaseId": "%SRCROOT%"},
		"region":           map[string]any{"startLine": at.Line, "startColumn": at.Column, "endLine": at.EndLine},
	}
}

// relativeReference encodes each segment byte by byte, keeping letters, digits and
// -._~$&+:=@, and a colon of the first segment encoded as well.
func relativeReference(target string) string {
	segments := strings.Split(target, "/")
	for i, segment := range segments {
		var out strings.Builder
		for _, b := range []byte(segment) {
			kept := b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || strings.IndexByte("-._~$&+:=@", b) >= 0
			switch {
			case kept && (i > 0 || b != ':'):
				out.WriteByte(b)
			default:
				fmt.Fprintf(&out, "%%%02X", b)
			}
		}
		segments[i] = out.String()
	}
	return strings.Join(segments, "/")
}

// findingResult is one finding as a result.
func (r *sarifRenderer) findingResult(found sarifRecord, index map[string]int) (map[string]any, error) {
	code, _ := found["code"].(string)
	at, err := positionOf(found["position"])
	if err != nil {
		return nil, err
	}
	hash, err := r.lineHash(at)
	if err != nil {
		return nil, err
	}
	symbol, _ := found["symbol"].(map[string]any)
	ref, _ := symbol["ref"].(string)
	related, err := relatedOf(found, ref, at.Path)
	if err != nil {
		return nil, err
	}
	message, _ := found["message"].(string)
	severity, _ := found["severity"].(string)
	bag := map[string]any{}
	for _, member := range sarifBagMembers {
		if value, ok := found[member]; ok {
			bag[member] = value
		}
	}
	result := map[string]any{
		"ruleId":    code,
		"ruleIndex": index[code],
		"level":     sarifLevel[severity],
		"message":   map[string]any{"text": withLinks(message, related)},
		"locations": []any{map[string]any{"physicalLocation": location(at)}},
		"partialFingerprints": map[string]any{
			"primaryLocationLineHash": hash,
			"deadsetSymbolRef/v1":     symbolDigest(code, ref),
		},
		"properties": bag,
	}
	if len(related) > 0 {
		var listed []any
		for i, one := range related {
			listed = append(listed, map[string]any{
				"id": i + 1, "physicalLocation": location(one.at), "message": map[string]any{"text": one.label},
			})
		}
		result["relatedLocations"] = listed
	}
	return result, nil
}

// staleResult is one stale-suppression record as a result at its own site.
func (r *sarifRenderer) staleResult(record sarifRecord, index map[string]int) (map[string]any, error) {
	at, err := positionOf(record["position"])
	if err != nil {
		return nil, err
	}
	at.EndLine = at.Line
	hash, err := r.lineHash(at)
	if err != nil {
		return nil, err
	}
	symbol, _ := record["symbol"].(string)
	message, _ := record["message"].(string)
	return map[string]any{
		"ruleId":    "DS1703",
		"ruleIndex": index["DS1703"],
		"level":     "error",
		"message":   map[string]any{"text": message},
		"locations": []any{map[string]any{"physicalLocation": location(at)}},
		"partialFingerprints": map[string]any{
			"primaryLocationLineHash": hash,
			"deadsetSymbolRef/v1":     symbolDigest("DS1703", symbol),
		},
	}, nil
}

// relatedLocation is one position a finding names beyond its own, with its label.
type relatedLocation struct {
	label string
	at    sarifPosition
}

// relatedOf is the implementations, the write positions and the other component
// members a finding names, in that order, the first hundred. The finding's own
// declaration is the member with its reference and its path.
func relatedOf(found sarifRecord, ownRef, ownPath string) ([]relatedLocation, error) {
	var related []relatedLocation
	details, _ := found["details"].(map[string]any)
	implementations, _ := details["implementations"].([]any)
	for _, one := range implementations {
		entry, _ := one.(map[string]any)
		at, err := positionOf(entry["position"])
		if err != nil {
			return nil, err
		}
		related = append(related, relatedLocation{"implementation", at})
	}
	writes, _ := details["write_positions"].([]any)
	for _, one := range writes {
		at, err := positionOf(one)
		if err != nil {
			return nil, err
		}
		related = append(related, relatedLocation{"write", at})
	}
	component, _ := found["component"].(map[string]any)
	members, _ := component["members"].([]any)
	for _, one := range members {
		entry, _ := one.(map[string]any)
		at, err := positionOf(entry["position"])
		if err != nil {
			return nil, err
		}
		if entry["ref"] == ownRef && at.Path == ownPath {
			continue
		}
		related = append(related, relatedLocation{"member", at})
	}
	if len(related) > sarifMaxRelated {
		related = related[:sarifMaxRelated]
	}
	return related, nil
}

// withLinks is a message followed by one link per related location.
func withLinks(message string, related []relatedLocation) string {
	if len(related) == 0 {
		return message
	}
	links := make([]string, len(related))
	for i, one := range related {
		links[i] = fmt.Sprintf("[%s %s:%d:%d](%d)", one.label, one.at.Path, one.at.Line, one.at.Column, i+1)
	}
	return message + " (see " + strings.Join(links, ", ") + ")"
}

// symbolDigest is deadsetSymbolRef/v1: the SHA-256 of the code, a line feed and the reference.
func symbolDigest(code, ref string) string {
	sum := sha256.Sum256([]byte(code + "\n" + ref))
	return hex.EncodeToString(sum[:])
}

// lineHash is primaryLocationLineHash for one position's line, failing for a file the
// case does not hold and a line past the last one.
func (r *sarifRenderer) lineHash(at sarifPosition) (string, error) {
	hashes, ok := r.hashes[at.Path]
	if !ok {
		content, held := r.sources[at.Path]
		if !held {
			return "", fmt.Errorf("%w: no file %s", errSARIFRendering, at.Path)
		}
		hashes = lineHashes(content)
		r.hashes[at.Path] = hashes
	}
	if at.Line < 1 || at.Line > len(hashes) {
		return "", fmt.Errorf("%w: %s holds %d lines and a record names line %d", errSARIFRendering, at.Path, len(hashes), at.Line)
	}
	return hashes[at.Line-1], nil
}

// lineHashes is the value of every line the procedure of grammar/sarif.md numbers.
func lineHashes(content string) []string {
	var units []uint16
	afterCR := false
	for _, unit := range utf16.Encode([]rune(content)) {
		switch {
		case unit == ' ' || unit == '\t' || unit == '\n' && afterCR:
			afterCR = false
		case unit == '\r':
			units = append(units, '\n')
			afterCR = true
		default:
			units = append(units, unit)
			afterCR = false
		}
	}
	units = append(units, 0xffff)
	starts := []int{0}
	for i := range len(units) - 1 {
		if units[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	seen := map[string]int{}
	hashes := make([]string, 0, len(starts))
	for _, start := range starts {
		var h uint64
		for i := start; i < start+100; i++ {
			var unit uint64
			if i < len(units) {
				unit = uint64(units[i])
			}
			h = h*37 + unit
		}
		rendered := strconv.FormatUint(h, 16)
		seen[rendered]++
		hashes = append(hashes, rendered+":"+strconv.Itoa(seen[rendered]))
	}
	return hashes
}

// sarifKinds is the live kinds of the embedded vocabulary.
func sarifKinds(t *testing.T) []kindRow {
	t.Helper()
	return loadKinds(t).Kinds
}

// sarifCaseDirs names every case directory under the SARIF vectors.
func sarifCaseDirs(t *testing.T, fsys fs.FS) []string {
	t.Helper()
	entries, err := fs.ReadDir(fsys, sarifVectorsDir)
	if err != nil {
		t.Fatalf("Setup: fs.ReadDir(%s): %v", sarifVectorsDir, err)
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(dirs) == 0 {
		t.Fatalf("Setup: %s holds no case", sarifVectorsDir)
	}
	return dirs
}

// loadSARIFCase reads one case, refusing a layout the vectors' README does not declare.
func loadSARIFCase(fsys fs.FS, dir string) (sarifCase, error) {
	at := path.Join(sarifVectorsDir, dir)
	entries, err := fs.ReadDir(fsys, at)
	if err != nil {
		return sarifCase{}, err
	}
	var c sarifCase
	outcomes := 0
	for _, entry := range entries {
		name := path.Join(at, entry.Name())
		switch entry.Name() {
		case sarifReportFile:
			c.report, err = fs.ReadFile(fsys, name)
		case sarifSourcesFile:
			c.sources, err = readSources(fsys, name)
		case sarifInputsDir:
			c.inputs, err = readInputs(fsys, name)
		case sarifExpectedFile:
			outcomes++
			var raw []byte
			if raw, err = fs.ReadFile(fsys, name); err == nil {
				err = decodeSARIF(raw, &c.expected)
			}
		case sarifExitFile:
			outcomes++
			var raw []byte
			if raw, err = fs.ReadFile(fsys, name); err == nil {
				c.exit, err = strconv.Atoi(strings.TrimSuffix(string(raw), "\n"))
			}
			if err == nil && c.exit != sarifFailedExit {
				err = fmt.Errorf("%s = %d, want %d", sarifExitFile, c.exit, sarifFailedExit)
			}
		default:
			err = fmt.Errorf("file %s is not one the case layout declares", entry.Name())
		}
		if err != nil {
			return sarifCase{}, err
		}
	}
	switch {
	case c.report == nil || c.sources == nil:
		return sarifCase{}, fmt.Errorf("want %s and %s", sarifReportFile, sarifSourcesFile)
	case outcomes != 1:
		return sarifCase{}, fmt.Errorf("want exactly one of %s and %s", sarifExpectedFile, sarifExitFile)
	}
	return c, nil
}

// decodeSARIF decodes a document strictly: UTF-8 with no byte order mark, one JSON text.
func decodeSARIF(raw []byte, into *any) error {
	if bytes.HasPrefix(raw, []byte("\xef\xbb\xbf")) || !utf8.Valid(raw) {
		return errors.New("the document is not UTF-8 without a byte order mark")
	}
	return json.Unmarshal(raw, into)
}

// readSources decodes a case's sources document into its path-to-content map.
func readSources(fsys fs.FS, name string) (map[string]string, error) {
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Files       map[string]string `json:"files"`
		Description string            `json:"description"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&doc); err != nil {
		return nil, fmt.Errorf("%s: %w", sarifSourcesFile, err)
	}
	if doc.Description == "" || len(doc.Files) == 0 {
		return nil, fmt.Errorf("%s: want a description and at least one file", sarifSourcesFile)
	}
	return doc.Files, nil
}

// readInputs reads every input report of a merged case, in file-name order.
func readInputs(fsys fs.FS, dir string) ([][]byte, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var inputs [][]byte
	for _, entry := range entries {
		raw, err := fs.ReadFile(fsys, path.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, raw)
	}
	return inputs, nil
}

// renderSARIFCase renders one case with the reference: the decoded document, or the
// exit code the rendering ends with.
func renderSARIFCase(kinds []kindRow, c *sarifCase) (any, int, error) {
	r := &sarifRenderer{sources: c.sources, hashes: map[string][]string{}, kinds: kinds}
	log, err := r.renderSARIF(c.report, c.inputs)
	switch {
	case errors.Is(err, errSARIFRendering):
		return nil, sarifFailedExit, nil
	case err != nil:
		return nil, 0, err
	}
	raw, err := json.Marshal(log)
	if err != nil {
		return nil, 0, err
	}
	var decoded any
	return decoded, 0, json.Unmarshal(raw, &decoded)
}

// sarifReproductionProblems renders one case and compares the outcome with the case's own.
func sarifReproductionProblems(kinds []kindRow, fsys fs.FS, dir string) []string {
	c, err := loadSARIFCase(fsys, dir)
	if err != nil {
		return []string{err.Error()}
	}
	got, exit, err := renderSARIFCase(kinds, &c)
	switch {
	case err != nil:
		return []string{err.Error()}
	case exit != c.exit:
		return []string{fmt.Sprintf("exit = %d, want %d", exit, c.exit)}
	case exit == 0 && !reflect.DeepEqual(got, c.expected):
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		return []string{fmt.Sprintf("document differs from %s; the reference renders:\n%s", sarifExpectedFile, gotJSON)}
	}
	return nil
}

// TestSARIFVectorCasesReproduceFromTheirReports renders every published case with the
// reference grammar/sarif.md defines and compares the decoded documents, so a value the
// report, its inputs and its sources do not determine is a failure here.
//
// The expected documents repeat the rule texts of kinds.json, so a change to a rule
// text regenerates them with UPDATE_SARIF_VECTORS=1, and the regenerated diff is
// reviewed as a change to the vectors.
func TestSARIFVectorCasesReproduceFromTheirReports(t *testing.T) {
	kinds := sarifKinds(t)
	update := os.Getenv("UPDATE_SARIF_VECTORS") == "1"
	for _, dir := range sarifCaseDirs(t, spec.Vectors) {
		t.Run(dir, func(t *testing.T) {
			if update {
				writeSARIFExpected(t, kinds, dir)
				return
			}
			for _, problem := range sarifReproductionProblems(kinds, spec.Vectors, dir) {
				t.Errorf("%s/%s: %s (regenerate with UPDATE_SARIF_VECTORS=1 go test -run TestSARIFVectorCasesReproduceFromTheirReports .)",
					sarifVectorsDir, dir, problem)
			}
		})
	}
}

// writeSARIFExpected rewrites one rendered case's expected document from the reference.
func writeSARIFExpected(t *testing.T, kinds []kindRow, dir string) {
	t.Helper()
	at := path.Join(sarifVectorsDir, dir)
	if _, err := os.Stat(path.Join(at, sarifExitFile)); err == nil {
		return
	}
	c, err := loadSARIFCase(os.DirFS("."), dir)
	if err != nil {
		t.Fatalf("loadSARIFCase(%s): %v", dir, err)
	}
	r := &sarifRenderer{sources: c.sources, hashes: map[string][]string{}, kinds: kinds}
	log, err := r.renderSARIF(c.report, c.inputs)
	if err != nil {
		t.Fatalf("renderSARIF(%s): %v", dir, err)
	}
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(log); err != nil {
		t.Fatalf("encode %s: %v", dir, err)
	}
	if err := os.WriteFile(path.Join(at, sarifExpectedFile), out.Bytes(), 0o644); err != nil {
		t.Fatalf("write %s: %v", dir, err)
	}
}

// TestSARIFVectorReportsAreInstancesOfTheReportSchema holds every case's report and
// input reports to the document a renderer reads.
func TestSARIFVectorReportsAreInstancesOfTheReportSchema(t *testing.T) {
	for _, dir := range sarifCaseDirs(t, spec.Vectors) {
		t.Run(dir, func(t *testing.T) {
			c, err := loadSARIFCase(spec.Vectors, dir)
			if err != nil {
				t.Fatalf("loadSARIFCase(%s): %v", dir, err)
			}
			for _, document := range append([][]byte{c.report}, c.inputs...) {
				validateAgainst(t, reportSchemaPath, document)
			}
		})
	}
}

// TestSARIFVectorsCoverTheMappingsCases holds the case set to what the page states:
// an analyzer's run, a merged log with the merge's own run and without it, a rendering
// that fails, a colon in a first segment and a capped related-location list.
func TestSARIFVectorsCoverTheMappingsCases(t *testing.T) {
	kinds := sarifKinds(t)
	seen := map[string]bool{}
	for _, dir := range sarifCaseDirs(t, spec.Vectors) {
		c, err := loadSARIFCase(spec.Vectors, dir)
		if err != nil {
			t.Fatalf("loadSARIFCase(%s): %v", dir, err)
		}
		if c.exit != 0 {
			seen["a failed rendering"] = true
			continue
		}
		raw, _ := json.Marshal(c.expected)
		document := string(raw)
		got, _, err := renderSARIFCase(kinds, &c)
		if err != nil {
			t.Fatalf("renderSARIFCase(%s): %v", dir, err)
		}
		log, _ := got.(map[string]any)
		runs, _ := log["runs"].([]any)
		switch {
		case len(c.inputs) == 0:
			seen["an analyzer's run"] = true
		case strings.Contains(document, `"`+sarifMergeRunID+`"`):
			seen["a merged log with the merge's own run"] = true
		case len(runs) == len(c.inputs):
			seen["a merged log without the merge's own run"] = true
		}
		if strings.Contains(document, `"uri":"a%3A`) {
			seen["a colon in a first segment"] = true
		}
		if strings.Contains(document, `"id":100,`) && !strings.Contains(document, `"id":101,`) {
			seen["a capped related-location list"] = true
		}
		if strings.Contains(document, `"withheld":"withheld by analysis.min_confidence: `) {
			seen["a withheld line"] = true
		}
	}
	wants := []string{
		"an analyzer's run", "a merged log with the merge's own run", "a merged log without the merge's own run",
		"a failed rendering", "a colon in a first segment", "a capped related-location list", "a withheld line",
	}
	for _, want := range wants {
		if !seen[want] {
			t.Errorf("cases holding %s = 0, want at least one", want)
		}
	}
}

// TestSARIFVectorsReadmeNamesEveryCase pins the table of the vectors' README to the
// case directories, one row each.
func TestSARIFVectorsReadmeNamesEveryCase(t *testing.T) {
	readme, err := fs.ReadFile(spec.Vectors, path.Join(sarifVectorsDir, "README.md"))
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	for _, dir := range sarifCaseDirs(t, spec.Vectors) {
		if n := strings.Count(string(readme), "| `"+dir+"` |"); n != 1 {
			t.Errorf("README rows naming %s = %d, want 1", dir, n)
		}
	}
}

// TestSARIFReferenceReproducesThePagesFingerprints holds the reference to the two
// fingerprint examples grammar/sarif.md publishes, so the vectors' fingerprints are the page's.
func TestSARIFReferenceReproducesThePagesFingerprints(t *testing.T) {
	five := lineHashes("package fixture\n\nfunc Ünused() {\n    return\n}\n")
	tabbed := lineHashes("package fixture\n\nfunc Ünused() {\n\treturn\n}\n")
	want := []string{"7a0e51a45e6d7320:1", "3134adfd1bbad887:1", "247cff8f02e0b919:1", "58228fc5cbc49530:1", "32dce9ccfdbc9d3e:1"}
	if !slices.Equal(five[:5], want) || !slices.Equal(tabbed[:5], want) {
		t.Errorf("lineHashes(the page's five-line file) = %v and %v with a tab, want %v", five[:5], tabbed[:5], want)
	}
	ys := lineHashes(strings.Repeat("y\n", 200))
	if got := []string{ys[0], ys[1], ys[150]}; !slices.Equal(got, []string{"43762f342805c306:1", "43762f342805c306:2", "43762f342805c306:151"}) {
		t.Errorf("lineHashes(200 lines of y) at lines 1, 2 and 151 = %v, want the page's values", got)
	}
	for _, tc := range []struct{ code, ref, want string }{
		{"DS1001", "go://example.com/fixture#Ünused", "d073714ada8cfcbee49bd5430446d6be7b837b6fd1fc34e6aa82be03b589c18d"},
		{"DS1001", "go://example.com/app#Catalog.ResolveAlias", "3969945e4504f5d8a52415a6f7b4f233d1ba4820a2fe24617a3611e2535d5e32"},
	} {
		if got := symbolDigest(tc.code, tc.ref); got != tc.want {
			t.Errorf("symbolDigest(%q, %q) = %s, want %s", tc.code, tc.ref, got, tc.want)
		}
	}
}

// TestSARIFReproductionRefuses plants one departure from the page per rule into a copy
// of a published case and requires the reference to see it.
func TestSARIFReproductionRefuses(t *testing.T) {
	tests := []struct {
		name, dir, file, old, replacement string
	}{
		{name: "the_first_segment_colon_written_raw", dir: "path-segments-encoded", file: sarifExpectedFile, old: `"a%3Ab.go"`, replacement: `"a:b.go"`},
		{name: "a_run_carrying_the_merged_totals", dir: "merged-runs-and-totals", file: "inputs/01-go.json", old: `"reasons_recorded": 1`, replacement: `"reasons_recorded": 0`},
		{name: "a_failed_rendering_written", dir: "record-naming-no-run", file: "report.json", old: `"deadset-rs"`, replacement: `"deadset-ts"`},
		{name: "a_line_fingerprint_changed", dir: "line-fingerprints", file: "sources.json", old: `"y\ny\n`, replacement: `"y\nz\n`},
		{name: "a_withheld_line_naming_a_zero_count", dir: "withheld-line", file: "report.json", old: `"probable": 2`, replacement: `"probable": 0`},
	}
	kinds := sarifKinds(t)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := fstest.MapFS{}
			at := path.Join(sarifVectorsDir, tc.dir)
			if err := fs.WalkDir(spec.Vectors, at, func(name string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return err
				}
				data, err := fs.ReadFile(spec.Vectors, name)
				m[name] = &fstest.MapFile{Data: data}
				return err
			}); err != nil {
				t.Fatalf("Setup: %v", err)
			}
			planted := m[path.Join(at, tc.file)]
			if planted == nil || !bytes.Contains(planted.Data, []byte(tc.old)) {
				t.Fatalf("Setup: %s holds no %q", tc.file, tc.old)
			}
			planted.Data = bytes.Replace(planted.Data, []byte(tc.old), []byte(tc.replacement), 1)
			if len(sarifReproductionProblems(kinds, m, tc.dir)) == 0 {
				t.Errorf("sarifReproductionProblems(%s with %s %q) = none, want the departure named", tc.dir, tc.file, tc.replacement)
			}
		})
	}
}
