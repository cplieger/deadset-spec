package spec_test

import (
	"maps"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The Contract's reference pages are documentation rather than contract data, so
// nothing embeds them and this suite reads them from the working tree. Two kinds
// of drift are the subject: a code, a class or an exit code the Contract
// declares and no page documents, and a page row that states a value the
// Contract's own document contradicts.
const (
	docsKindsPage      = "docs/kinds.md"
	docsExemptionsPage = "docs/exemptions.md"
	docsExitCodesPage  = "docs/exit-codes.md"
)

// docsRequiredKindColumns are the columns a kinds table must carry for a live
// code: the code that names the row, and the three values a reader decides a
// kind's treatment with. A page may carry more columns and this suite compares
// every column docsKindColumns names.
var docsRequiredKindColumns = []string{"code", "default", "severity", "fixability"}

// docsKindColumns maps a header cell of a kinds table to the contract/kinds.json
// field the column renders. A header this map does not name carries prose and is
// not compared. max_class is mapped and no published table renders it, which is
// deliberate: every kind of this contract version declares the same ceiling, so
// a page states it once in prose, and the day two ceilings coexist a page can
// grow the column and be compared row by row with no edit here.
var docsKindColumns = map[string]string{
	"code":       "code",
	"kind":       "name",
	"name":       "name",
	"lang":       "languages",
	"language":   "languages",
	"languages":  "languages",
	"default":    "default",
	"enabled":    "default",
	"severity":   "severity",
	"fix":        "fixability",
	"fixability": "fixability",
	"fixed":      "fixed",
	"confidence": "max_class",
	"max class":  "max_class",
}

// docsExitCodeColumns maps a header cell of the exit-code table to the
// contract/exit-codes.json field the column renders.
var docsExitCodeColumns = map[string]string{
	"code":      "code",
	"exit":      "code",
	"exit code": "code",
	"name":      "name",
}

var (
	// docsHeadingPattern matches an ATX heading and captures its level and text.
	docsHeadingPattern = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	// docsCodeInText finds every code the code space could hold, wherever a page
	// writes it, so a page naming an undeclared code fails.
	docsCodeInText = regexp.MustCompile(`DS[0-9]{4}`)
	// docsTokenPattern is the shape of a vocabulary token a heading may name: a
	// kind name, a range family or an exemption class, all lowercase words joined
	// by single hyphens.
	docsTokenPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)+$`)
	docsSpaceRun     = regexp.MustCompile(`\s+`)
	docsDigits       = regexp.MustCompile(`^[0-9]+$`)
	// docsMarkup are the markers a page adds around a value it quotes, and the
	// escapes markdown needs for the punctuation a contract string carries.
	docsMarkup = strings.NewReplacer("`", "", "*", "", "_", "", "\\", "", `"`, "", "'", "", ">", "")
)

// docsPage reads one reference page. An absent page is the state this suite
// exists to catch, so it fails here naming the page rather than skipping.
func docsPage(t *testing.T, p string) string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("Documentation(%s) = absent (%v), want the page that documents what the contract declares", p, err)
	}
	return string(data)
}

// docsProse folds a page's markdown and a contract string onto one comparable
// text: the markers a quotation adds go, every run of whitespace becomes one
// space, and the case folds.
func docsProse(s string) string {
	return strings.ToLower(strings.TrimSpace(docsSpaceRun.ReplaceAllString(docsMarkup.Replace(s), " ")))
}

// docsCarries reports whether a page's text states value verbatim.
func docsCarries(text, value string) bool {
	return strings.Contains(docsProse(text), docsProse(value))
}

// docsCell folds one table cell for comparison with a contract value.
func docsCell(cell string) string {
	return strings.TrimSuffix(docsProse(cell), ".")
}

// docsSection is one heading of a page with the text under it. body is the
// section's own lines and stops at the next heading of any level; scope is body
// plus every subsection nested under this one.
type docsSection struct {
	heading string
	body    string
	scope   string
	level   int
}

// docsHeading is one heading's position on the page.
type docsHeading struct {
	text  string
	line  int
	level int
}

// docsSections splits a page into its sections in document order, the text
// before the first heading first as a level-zero section with an empty heading,
// so every line of the page belongs to exactly one section body.
func docsSections(page string) []docsSection {
	lines := strings.Split(page, "\n")
	heads := []docsHeading{{line: -1, level: 0}}
	for i, line := range lines {
		if m := docsHeadingPattern.FindStringSubmatch(line); m != nil {
			heads = append(heads, docsHeading{line: i, level: len(m[1]), text: strings.TrimSpace(m[2])})
		}
	}
	out := make([]docsSection, 0, len(heads))
	for i, h := range heads {
		bodyEnd := len(lines)
		if i+1 < len(heads) {
			bodyEnd = heads[i+1].line
		}
		scopeEnd := len(lines)
		for _, next := range heads[i+1:] {
			if next.level <= h.level {
				scopeEnd = next.line
				break
			}
		}
		out = append(out, docsSection{
			heading: h.text,
			level:   h.level,
			body:    strings.Join(lines[h.line+1:bodyEnd], "\n"),
			scope:   strings.Join(lines[h.line+1:scopeEnd], "\n"),
		})
	}
	return out
}

// docsSectionsNaming returns every section whose heading names token, matched on
// the folded text so a heading may wrap the token in inline code.
func docsSectionsNaming(sections []docsSection, token string) []docsSection {
	var hits []docsSection
	for _, s := range sections {
		if s.level > 0 && strings.Contains(docsProse(s.heading), docsProse(token)) {
			hits = append(hits, s)
		}
	}
	return hits
}

// docsTable is one markdown table: its header cells and its body rows.
type docsTable struct {
	header []string
	rows   [][]string
}

// docsTables groups the markdown tables in text with their own header rows, so a
// row is always read through the header of the table it belongs to.
func docsTables(text string) []docsTable {
	var out []docsTable
	var run []string
	flush := func() {
		rows := pageTableRows(strings.Join(run, "\n"))
		run = nil
		if len(rows) > 1 {
			out = append(out, docsTable{header: rows[0], rows: rows[1:]})
		}
	}
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "|") {
			run = append(run, line)
			continue
		}
		flush()
	}
	flush()
	return out
}

// columns returns the index of every column whose header known names, keyed by
// the contract field the column renders.
func (tbl docsTable) columns(known map[string]string) map[string]int {
	at := make(map[string]int, len(tbl.header))
	for i, cell := range tbl.header {
		if field, ok := known[docsCell(cell)]; ok {
			at[field] = i
		}
	}
	return at
}

// docsCellAt returns one cell of a row, empty when the row is short of that
// column.
func docsCellAt(row []string, at int) string {
	if at < 0 || at >= len(row) {
		return ""
	}
	return row[at]
}

// docsRow is one row of one table, carried with the column index of the table it
// came from so a caller reads the row by field name.
type docsRow struct {
	columns map[string]int
	table   docsTable
	cells   []string
}

// value returns the folded cell one field of this row renders.
func (r docsRow) value(field string) string {
	at, ok := r.columns[field]
	if !ok {
		return ""
	}
	return docsCell(docsCellAt(r.cells, at))
}

// docsRowsFor returns every row of every table whose key column names want.
func docsRowsFor(tables []docsTable, known map[string]string, key, want string) []docsRow {
	var hits []docsRow
	for _, tbl := range tables {
		columns := tbl.columns(known)
		at, ok := columns[key]
		if !ok {
			continue
		}
		for _, row := range tbl.rows {
			if docsCell(docsCellAt(row, at)) == docsCell(want) {
				hits = append(hits, docsRow{table: tbl, columns: columns, cells: row})
			}
		}
	}
	return hits
}

// docsRowsKeyed returns every row of every table that carries a key column, so a
// page's own rows are the population when the page is what is being read.
func docsRowsKeyed(tables []docsTable, known map[string]string, key string) []docsRow {
	var hits []docsRow
	for _, tbl := range tables {
		columns := tbl.columns(known)
		if _, ok := columns[key]; !ok {
			continue
		}
		for _, row := range tbl.rows {
			hits = append(hits, docsRow{table: tbl, columns: columns, cells: row})
		}
	}
	return hits
}

// docsDeclaredCodes lists every code the vocabulary declares: the live rows, the
// retired rows and the bound of every range, since a page names a family by the
// two codes that bound it.
func docsDeclaredCodes(doc kindsDocument) []string {
	codes := make([]string, 0, len(doc.Kinds)+len(doc.Retired)+2*len(doc.Ranges))
	for _, k := range doc.Kinds {
		codes = append(codes, k.Code)
	}
	for _, r := range doc.Retired {
		codes = append(codes, r.Code)
	}
	for _, r := range doc.Ranges {
		codes = append(codes, r.Start, r.End)
	}
	return codes
}

// docsKindTables splits the tables of docs/kinds.md into the live range tables
// and the retired table, the retired one being the tables under the single
// section whose heading names retirement.
func docsKindTables(t *testing.T, page string) (live, retired []docsTable) {
	t.Helper()
	sections := docsSections(page)
	retiredAt := -1
	for i, s := range sections {
		if s.level > 0 && strings.Contains(strings.ToLower(s.heading), "retired") {
			if retiredAt >= 0 {
				t.Fatalf("Documentation(%s) sections naming retirement = %q and %q, want exactly 1", docsKindsPage, sections[retiredAt].heading, s.heading)
			}
			retiredAt = i
		}
	}
	if retiredAt < 0 {
		t.Fatalf("Documentation(%s) section naming retirement = absent, want one section listing the retired codes", docsKindsPage)
	}
	inRetired := map[int]bool{retiredAt: true}
	for i, s := range sections[retiredAt+1:] {
		if s.level <= sections[retiredAt].level {
			break
		}
		inRetired[retiredAt+1+i] = true
	}
	for i, s := range sections {
		if inRetired[i] {
			retired = append(retired, docsTables(s.body)...)
			continue
		}
		live = append(live, docsTables(s.body)...)
	}
	return live, retired
}

// docsCellLanguages reads a languages cell as the set of vocabulary words it
// names, so "both", "go, ts" and "Go and TypeScript" all resolve to the words
// contract/kinds.json uses.
func docsCellLanguages(vocabulary []string, raw string) []string {
	var got []string
	add := func(langs ...string) {
		for _, lang := range langs {
			if !slices.Contains(got, lang) {
				got = append(got, lang)
			}
		}
	}
	for _, word := range strings.FieldsFunc(docsCell(raw), func(r rune) bool {
		return r < 'a' || r > 'z'
	}) {
		switch word {
		case "and", "or", "only":
		case "both", "all", "either":
			add(vocabulary...)
		case "go", "golang":
			add("go")
		case "ts", "typescript":
			add("ts")
		default:
			add(word)
		}
	}
	slices.Sort(got)
	return got
}

// docsKindFieldAgrees reports whether a cell renders one field of one live row,
// and returns the spelling a failure message asks for. A cell states its own
// field: the enablement of the one fixed kind is `on` like every other row's,
// because whether a kind is fixed is a field of its own. A page that merges the
// two into one cell is accepted, and so is a fixability abbreviated to the stem
// of the vocabulary's own word.
func docsKindFieldAgrees(doc kindsDocument, k kindRow, field, raw string) (bool, string) {
	got := docsCell(raw)
	switch field {
	case "languages":
		want := slices.Clone(k.Languages)
		slices.Sort(want)
		return slices.Equal(docsCellLanguages(doc.Languages, raw), want), strings.Join(want, ", ")
	case "default":
		return slices.Contains(docsAllowedDefault(k), got), docsWantDefault(k)
	case "severity":
		return slices.Contains(docsAllowedSeverity(k), got), k.DefaultSeverity
	case "fixability":
		return slices.Contains(docsAllowedFixability(k), got), k.Fixability
	case "fixed":
		return slices.Contains(docsAllowedFixed(k), got), strconv.FormatBool(isSet(k.Fixed) && *k.Fixed)
	case "name":
		return got == docsCell(k.Name), k.Name
	case "max_class":
		return got == docsCell(k.MaxClass), k.MaxClass
	case "code":
		return got == docsCell(k.Code), k.Code
	}
	return true, ""
}

// docsWantDefault is the enablement contract/kinds.json states for a row.
func docsWantDefault(k kindRow) string {
	if isSet(k.DefaultEnabled) && *k.DefaultEnabled {
		return "on"
	}
	return "off"
}

// docsAllowedDefault lists the spellings a page may use for a row's enablement.
// A fixed row may carry its fixed state in the same cell.
func docsAllowedDefault(k kindRow) []string {
	want := docsWantDefault(k)
	if isSet(k.Fixed) && *k.Fixed {
		return []string{want, "fixed " + want, want + " (fixed)", want + ", fixed"}
	}
	return []string{want}
}

// docsAllowedSeverity lists the spellings a page may use for a row's severity. A
// fixed row may carry its fixed state in the same cell; a row the configuration
// may move may not, so a page calling that one fixed is wrong.
func docsAllowedSeverity(k kindRow) []string {
	sev := docsCell(k.DefaultSeverity)
	if isSet(k.Fixed) && *k.Fixed {
		return []string{sev, "fixed " + sev, sev + " (fixed)"}
	}
	return []string{sev}
}

// docsAllowedFixed lists the spellings a page may use in a column of its own for
// whether the configuration may move a row.
func docsAllowedFixed(k kindRow) []string {
	if isSet(k.Fixed) && *k.Fixed {
		return []string{"true", "yes", "fixed"}
	}
	return []string{"false", "no", "", "-"}
}

// docsFixabilityStems are the abbreviations a page may print for a fixability
// word, one per vocabulary value that has a shorter spelling.
var docsFixabilityStems = map[string]string{
	"deletable":  "del",
	"narrowable": "narrow",
}

// docsAllowedFixability lists the spellings a page may use for a row's
// fixability.
func docsAllowedFixability(k kindRow) []string {
	allowed := []string{docsCell(k.Fixability)}
	if stem, ok := docsFixabilityStems[k.Fixability]; ok {
		allowed = append(allowed, stem)
	}
	return allowed
}

// TestDocsKindsPageDocumentsEveryLiveCode pins the coverage half for the kinds
// page: every live code has a section naming it that quotes the kind's rule, and
// one range-table row whose cells are the values contract/kinds.json carries.
func TestDocsKindsPageDocumentsEveryLiveCode(t *testing.T) {
	doc := loadKinds(t)
	page := docsPage(t, docsKindsPage)
	sections := docsSections(page)
	live, _ := docsKindTables(t, page)
	for _, k := range doc.Kinds {
		t.Run(k.Code, func(t *testing.T) {
			section := docsKindSection(t, sections, k)
			if !docsCarries(section.heading+"\n"+section.scope, k.Rule) {
				t.Errorf("Documentation(%s) Kind(%s).rule = not stated under the heading %q, want the rule of %s: %q", docsKindsPage, k.Code, section.heading, kindsPath, k.Rule)
			}
			row := docsKindTableRow(t, live, k)
			for _, field := range slices.Sorted(maps.Keys(row.columns)) {
				at := row.columns[field]
				ok, want := docsKindFieldAgrees(doc, k, field, docsCellAt(row.cells, at))
				if !ok {
					t.Errorf("Documentation(%s) Kind(%s).%s = %q, want %q as %s carries it", docsKindsPage, k.Code, field, docsCellAt(row.cells, at), want, kindsPath)
				}
			}
			for _, field := range docsRequiredKindColumns {
				if _, ok := row.columns[field]; !ok {
					t.Errorf("Documentation(%s) Kind(%s) table columns = %q, want one naming %s", docsKindsPage, k.Code, row.table.header, field)
				}
			}
			if !docsCarries(section.heading+"\n"+section.scope+"\n"+strings.Join(row.cells, " | "), k.Name) {
				t.Errorf("Documentation(%s) Kind(%s).name = not stated in the row or the section, want %q", docsKindsPage, k.Code, k.Name)
			}
		})
	}
}

// docsKindSection returns the one section of the kinds page whose heading names
// the code.
func docsKindSection(t *testing.T, sections []docsSection, k kindRow) docsSection {
	t.Helper()
	hits := docsSectionsNaming(sections, k.Code)
	if len(hits) != 1 {
		t.Fatalf("Documentation(%s) Kind(%s) sections = %d, want exactly 1 heading naming the code", docsKindsPage, k.Code, len(hits))
	}
	return hits[0]
}

// docsKindTableRow returns the one range-table row that names the code.
func docsKindTableRow(t *testing.T, live []docsTable, k kindRow) docsRow {
	t.Helper()
	rows := docsRowsFor(live, docsKindColumns, "code", k.Code)
	if len(rows) != 1 {
		t.Fatalf("Documentation(%s) Kind(%s) range-table rows = %d, want exactly 1", docsKindsPage, k.Code, len(rows))
	}
	return rows[0]
}

// TestDocsKindsPageStatesWhichKindsAreFixed pins the one field of a live row a
// range table need not carry a column for: a kind no configuration moves is
// documented as fixed, so a reader is never told a severity setting will reach
// it. The population is the rows contract/kinds.json marks fixed.
func TestDocsKindsPageStatesWhichKindsAreFixed(t *testing.T) {
	sections := docsSections(docsPage(t, docsKindsPage))
	for _, k := range loadKinds(t).Kinds {
		if !isSet(k.Fixed) || !*k.Fixed {
			continue
		}
		t.Run(k.Code, func(t *testing.T) {
			section := docsKindSection(t, sections, k)
			if !docsCarries(section.heading+"\n"+section.scope, "fixed") {
				t.Errorf("Documentation(%s) Kind(%s).fixed = not stated under the heading %q, want the section to state that no configuration moves the kind, as %s marks the row fixed", docsKindsPage, k.Code, section.heading, kindsPath)
			}
		})
	}
}

// TestDocsKindsPageListsEveryRetiredCode pins that the retired list is complete
// and carries the name each retired code once held.
func TestDocsKindsPageListsEveryRetiredCode(t *testing.T) {
	page := docsPage(t, docsKindsPage)
	_, retired := docsKindTables(t, page)
	for _, r := range loadKinds(t).Retired {
		t.Run(r.Code, func(t *testing.T) {
			rows := docsRowsFor(retired, docsKindColumns, "code", r.Code)
			if len(rows) != 1 {
				t.Fatalf("Documentation(%s) Retired(%s) rows = %d, want exactly 1 row in the retired table", docsKindsPage, r.Code, len(rows))
			}
			if got := rows[0].value("name"); got != docsCell(r.Name) {
				t.Errorf("Documentation(%s) Retired(%s).name = %q, want %q as %s carries it", docsKindsPage, r.Code, got, r.Name, kindsPath)
			}
		})
	}
}

// TestDocsKindsPageKeepsLiveAndRetiredCodesApart pins the two directions of the
// one thing a reader must not be told: a live code listed as retired, and a
// retired code listed as a kind the analyzers report.
func TestDocsKindsPageKeepsLiveAndRetiredCodesApart(t *testing.T) {
	doc := loadKinds(t)
	page := docsPage(t, docsKindsPage)
	live, retired := docsKindTables(t, page)
	for _, k := range doc.Kinds {
		t.Run(k.Code, func(t *testing.T) {
			if rows := docsRowsFor(retired, docsKindColumns, "code", k.Code); len(rows) != 0 {
				t.Errorf("Documentation(%s) Kind(%s) retired-table rows = %d, want 0: %s carries the code as a live kind", docsKindsPage, k.Code, len(rows), kindsPath)
			}
		})
	}
	for _, r := range doc.Retired {
		t.Run(r.Code, func(t *testing.T) {
			if rows := docsRowsFor(live, docsKindColumns, "code", r.Code); len(rows) != 0 {
				t.Errorf("Documentation(%s) Retired(%s) range-table rows = %d, want 0: %s carries the code as retired", docsKindsPage, r.Code, len(rows), kindsPath)
			}
		})
	}
}

// TestDocsKindsPageNamesNoUndeclaredCode pins the drift in the other direction:
// a page naming a code the vocabulary does not declare.
func TestDocsKindsPageNamesNoUndeclaredCode(t *testing.T) {
	page := docsPage(t, docsKindsPage)
	declared := docsDeclaredCodes(loadKinds(t))
	for _, code := range slices.Sorted(slices.Values(docsCodeInText.FindAllString(page, -1))) {
		t.Run(code, func(t *testing.T) {
			if !slices.Contains(declared, code) {
				t.Errorf("Documentation(%s) names Kind(%s), want a code %s declares live or retired", docsKindsPage, code, kindsPath)
			}
		})
	}
}

// TestDocsKindsPageNamesNoUndeclaredKindName pins the same drift one level up: a
// heading that documents a kind or a family the vocabulary does not declare.
func TestDocsKindsPageNamesNoUndeclaredKindName(t *testing.T) {
	doc := loadKinds(t)
	page := docsPage(t, docsKindsPage)
	known := make([]string, 0, len(doc.Kinds)+len(doc.Retired)+len(doc.Ranges))
	for _, k := range doc.Kinds {
		known = append(known, k.Name)
	}
	for _, r := range doc.Retired {
		known = append(known, r.Name)
	}
	for _, r := range doc.Ranges {
		known = append(known, r.Family)
	}
	for _, s := range docsSections(page) {
		for _, token := range docsHeadingTokens(s.heading) {
			t.Run(caseName(token), func(t *testing.T) {
				if !slices.Contains(known, token) {
					t.Errorf("Documentation(%s) heading %q names %q, want a kind name or a range family %s declares", docsKindsPage, s.heading, token, kindsPath)
				}
			})
		}
	}
}

// docsHeadingTokens lists the vocabulary tokens a heading names: the words
// shaped like a kind name, a range family or an exemption class.
func docsHeadingTokens(heading string) []string {
	var tokens []string
	for _, word := range strings.FieldsFunc(docsProse(heading), func(r rune) bool {
		return r != '-' && (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		if docsTokenPattern.MatchString(word) && !slices.Contains(tokens, word) {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

// TestDocsExemptionsPageDocumentsEveryClass pins the coverage half for the
// exemption page: every class has a section naming it that states what the class
// retains and one mechanism per language the class declares.
func TestDocsExemptionsPageDocumentsEveryClass(t *testing.T) {
	page := docsPage(t, docsExemptionsPage)
	sections := docsSections(page)
	for _, class := range loadExemptions(t).Exemptions {
		t.Run(class.Class, func(t *testing.T) {
			hits := docsSectionsNaming(sections, class.Class)
			if len(hits) != 1 {
				t.Fatalf("Documentation(%s) Class(%s) sections = %d, want exactly 1 heading naming the class", docsExemptionsPage, class.Class, len(hits))
			}
			text := hits[0].heading + "\n" + hits[0].scope
			if !docsCarries(text, class.Retains) {
				t.Errorf("Documentation(%s) Class(%s).retains = not stated in the section, want what %s carries: %q", docsExemptionsPage, class.Class, exemptionsPath, class.Retains)
			}
			for _, lang := range class.Languages {
				if !docsCarries(text, class.Mechanism[lang]) {
					t.Errorf("Documentation(%s) Class(%s).mechanism[%q] = not stated in the section, want what %s carries: %q", docsExemptionsPage, class.Class, lang, exemptionsPath, class.Mechanism[lang])
				}
			}
		})
	}
}

// TestDocsExemptionsPageNamesNoUndeclaredClass pins the drift in the other
// direction: a section documenting a class the vocabulary does not declare.
func TestDocsExemptionsPageNamesNoUndeclaredClass(t *testing.T) {
	classes := classNames(loadExemptions(t))
	page := docsPage(t, docsExemptionsPage)
	for _, s := range docsSections(page) {
		for _, token := range docsHeadingTokens(s.heading) {
			t.Run(caseName(token), func(t *testing.T) {
				if !classes[token] {
					t.Errorf("Documentation(%s) heading %q names %q, want a class %s declares", docsExemptionsPage, s.heading, token, exemptionsPath)
				}
			})
		}
	}
}

// TestDocsExitCodesPageDocumentsEveryCode pins the coverage half for the
// exit-code page: every code the table declares has one row carrying its name.
func TestDocsExitCodesPageDocumentsEveryCode(t *testing.T) {
	table := mustLoadExitCodes(t)
	tables := docsTables(docsPage(t, docsExitCodesPage))
	for _, row := range table.ExitCodes {
		code := strconv.Itoa(row.Code)
		t.Run(code, func(t *testing.T) {
			rows := docsRowsFor(tables, docsExitCodeColumns, "code", code)
			if len(rows) != 1 {
				t.Fatalf("Documentation(%s) ExitCode(%s) rows = %d, want exactly 1", docsExitCodesPage, code, len(rows))
			}
			if got := rows[0].value("name"); got != docsCell(row.Name) {
				t.Errorf("Documentation(%s) ExitCode(%s).name = %q, want %q as %s carries it", docsExitCodesPage, code, got, row.Name, exitCodesPath)
			}
		})
	}
}

// TestDocsExitCodesPageNamesNoUndeclaredCode pins the drift in the other
// direction: a row documenting a code the table does not declare.
func TestDocsExitCodesPageNamesNoUndeclaredCode(t *testing.T) {
	table := mustLoadExitCodes(t)
	declared := make([]string, 0, len(table.ExitCodes))
	for _, row := range table.ExitCodes {
		declared = append(declared, strconv.Itoa(row.Code))
	}
	for _, code := range docsExitCodesNamed(docsTables(docsPage(t, docsExitCodesPage))) {
		t.Run(code, func(t *testing.T) {
			if !slices.Contains(declared, code) {
				t.Errorf("Documentation(%s) names ExitCode(%s), want a code %s declares", docsExitCodesPage, code, exitCodesPath)
			}
		})
	}
}

// docsExitCodesNamed lists every code a row of the exit-code page names, each
// once, in the order the page states them.
func docsExitCodesNamed(tables []docsTable) []string {
	var codes []string
	for _, tbl := range tables {
		at, ok := tbl.columns(docsExitCodeColumns)["code"]
		if !ok {
			continue
		}
		for _, row := range tbl.rows {
			got := docsCell(docsCellAt(row, at))
			if docsDigits.MatchString(got) && !slices.Contains(codes, got) {
				codes = append(codes, got)
			}
		}
	}
	return codes
}

// docsExemptionColumns maps a header cell of an exemption table to the
// contract/exemptions.json field the column renders.
var docsExemptionColumns = map[string]string{
	"class":          "class",
	"language":       "languages",
	"languages":      "languages",
	"confidence":     "confidence",
	"names site":     "names_site",
	"names the site": "names_site",
}

// docsExemptionFieldAgrees reports whether a cell renders one field of one class,
// and returns the spelling a failure message asks for.
func docsExemptionFieldAgrees(class exemptionClass, field, raw string) (bool, string) {
	got := docsCell(raw)
	switch field {
	case "languages":
		want := slices.Clone(class.Languages)
		slices.Sort(want)
		return slices.Equal(docsCellLanguages(knownLanguages, raw), want), strings.Join(want, ", ")
	case "confidence":
		return got == docsCell(class.Confidence), class.Confidence
	case "names_site":
		return slices.Contains(docsAllowedBool(class.NamesSite), got), strconv.FormatBool(class.NamesSite)
	case "class":
		return got == docsCell(class.Class), class.Class
	}
	return true, ""
}

// docsAllowedBool lists the spellings a page may use for a boolean field.
func docsAllowedBool(value bool) []string {
	if value {
		return []string{"true", "yes"}
	}
	return []string{"false", "no"}
}

// TestDocsExemptionsPageRowsStateTheVocabularysOwnValues reads the exemption
// page's own rows: every row that names a class names a declared one and states
// the values contract/exemptions.json carries for it, so a summary table cannot
// drift from the vocabulary it summarizes.
func TestDocsExemptionsPageRowsStateTheVocabularysOwnValues(t *testing.T) {
	doc := loadExemptions(t)
	declared := make(map[string]exemptionClass, len(doc.Exemptions))
	for _, class := range doc.Exemptions {
		declared[class.Class] = class
	}
	for _, row := range docsRowsKeyed(docsTables(docsPage(t, docsExemptionsPage)), docsExemptionColumns, "class") {
		name := row.value("class")
		t.Run(caseName(name), func(t *testing.T) {
			class, ok := declared[name]
			if !ok {
				t.Fatalf("Documentation(%s) states a row for Class(%s), want a class %s declares", docsExemptionsPage, name, exemptionsPath)
			}
			for _, field := range slices.Sorted(maps.Keys(row.columns)) {
				at := row.columns[field]
				agrees, want := docsExemptionFieldAgrees(class, field, docsCellAt(row.cells, at))
				if !agrees {
					t.Errorf("Documentation(%s) Class(%s).%s = %q, want %q as %s carries it", docsExemptionsPage, name, field, docsCellAt(row.cells, at), want, exemptionsPath)
				}
			}
		})
	}
}
