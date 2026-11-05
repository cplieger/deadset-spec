package suppress

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// The pairing world describes a run in suppression.md's words: declarations,
// each written in a file and rendered under a path, and the findings about
// them. The oracle reads only this world, never a [Finding] or a [Record].
type (
	worldDecl struct{ id, ref, file, path string }

	worldFinding struct {
		code vocab.Code
		kind string
		// about is the declaration the finding is about, or the one holding the
		// part it is about; nil for a row of a document.
		about *worldDecl
		// param is the part's own name for a parameter, empty otherwise.
		param string
		// at is the path a line directive moves a part to, empty where the part
		// renders under its holder's path.
		at string
		// iface is the interface a satisfaction assertion names, and twins its
		// other platform alternatives the assertion's configurations name.
		iface     *worldDecl
		twins     []*worldDecl
		ref, path string // a row's reference and file
		id        string
	}

	// worldRecord is a directive written in file above the declaration above
	// (nil: above none), or an entry or row naming symbol at path.
	worldRecord struct {
		code         vocab.Code
		mechanism    Mechanism
		above        *worldDecl
		file         string
		symbol, path string
	}

	world struct {
		name     string
		decls    []*worldDecl
		findings []*worldFinding
	}
)

// reported is the symbol.ref and position.path the finding reports.
func (f *worldFinding) reported() (ref, path string) {
	switch {
	case f.about == nil:
		return f.ref, f.path
	case f.iface != nil:
		return f.iface.ref, f.about.path
	case f.param != "":
		return f.about.ref + "(" + f.param + ")", cmp.Or(f.at, f.about.path)
	}
	return f.about.ref, cmp.Or(f.at, f.about.path)
}

// names is the oracle. A directive names what is written in its own file: the
// declaration below it, a part of it, and an assertion of the interface below
// it written in that file ("Where it sits"). An entry names what a finding
// reports, its code, symbol.ref and position.path, and a part-kind entry the
// parts of the declaration its symbol names ("Matching", "Before the sweep").
func (r *worldRecord) names(f *worldFinding) bool {
	if r.code != f.code || !Names(r.code.Subject(), r.mechanism) {
		return false
	}
	if r.mechanism == Inline {
		switch {
		case r.above == nil:
			return false
		case f.about == r.above:
			return true
		}
		named := f.iface != nil && (f.iface == r.above || slices.Contains(f.twins, r.above))
		return named && f.about.file == r.file
	}
	ref, path := f.reported()
	if path != r.path {
		return false
	}
	return ref == r.symbol || f.param != "" && f.about.ref == r.symbol
}

func (f *worldFinding) finding() Finding {
	found := Finding{Code: f.code, SymbolKind: f.kind, ID: f.id}
	found.Ref, found.Path = f.reported()
	if f.about != nil {
		found.Decl, found.File = f.about.id, f.about.file
	}
	if f.param != "" {
		found.Holder = f.about.ref
	}
	if f.iface != nil {
		found.Named = []string{f.iface.id}
		for _, twin := range f.twins {
			found.Named = append(found.Named, twin.id)
		}
	}
	return found
}

// records is what an analyzer resolves the world record to: a directive
// bound to the declaration below it, or an entry bound through the world's
// inventory.
func (w *world) records(t *testing.T, r *worldRecord, line int) []Record {
	t.Helper()
	if r.mechanism == Inline {
		site := Site{Path: r.file, File: r.file, Written: fmt.Sprintf("%s@%d", r.file, line), Line: line, Column: 1}
		record := Record{Code: r.code, Path: r.file, Reason: "kept", Mechanism: Inline, Site: site}
		if r.above != nil {
			record.Symbol, record.Path, record.Bound = r.above.ref, r.above.path, r.above.id
			record.Site.Path = r.above.path
		}
		return []Record{record}
	}
	binder := &Binder{
		Scheme: "go://",
		Lookup: func(symbol, path string) []string {
			var ids []string
			for _, d := range w.decls {
				if d.ref == symbol && d.path == path {
					ids = append(ids, d.id)
				}
			}
			return ids
		},
		Holder: func(symbol string) (string, bool) {
			held, _, part := strings.Cut(symbol, "(")
			return held, part
		},
	}
	reason, path := "kept", r.path
	records, refusals, err := binder.Bind([]Entry{{Code: r.code, Symbol: r.symbol, Path: &path, Reason: &reason, Site: Site{Path: "deadset-ignore.json", Line: line, Column: 3}}}, r.mechanism)
	if err != nil || len(refusals) != 0 {
		t.Fatalf("%s: Bind(%+v) = %v and %v", w.name, r, refusals, err)
	}
	return records
}

// candidates is every record the world can hold under its findings' code: a
// directive above each declaration and one above none, and an entry and a row
// for each symbol and path a finding reports, a part's holder, a declaration
// where it renders, and a path no finding reports.
func (w *world) candidates() []*worldRecord {
	code := w.findings[0].code
	var records []*worldRecord
	for _, d := range w.decls {
		records = append(records, &worldRecord{code: code, mechanism: Inline, above: d, file: d.file})
	}
	records = append(records, &worldRecord{code: code, mechanism: Inline, file: "a.go"})
	type named struct{ symbol, path string }
	var entries []named
	for _, f := range w.findings {
		ref, path := f.reported()
		entries = append(entries, named{ref, path}, named{ref, "elsewhere.go"})
		if f.param != "" {
			entries = append(entries, named{f.about.ref, path})
		}
	}
	for _, d := range w.decls {
		if !strings.HasSuffix(d.ref, "#_") {
			entries = append(entries, named{d.ref, d.path})
		}
	}
	slices.SortFunc(entries, func(a, b named) int { return strings.Compare(a.symbol+"\x00"+a.path, b.symbol+"\x00"+b.path) })
	for _, e := range slices.Compact(entries) {
		for _, mechanism := range []Mechanism{Ignore, Baseline} {
			records = append(records, &worldRecord{code: code, mechanism: mechanism, symbol: e.symbol, path: e.path})
		}
	}
	return records
}

// pairingWorlds is every subject kind times every placement: in the record's
// file, two there, one in another file, two written files a line directive
// folds onto one position, and one a line directive moves to another path; a
// part also moved off its holder's path, onto another alternative's, and two
// alternatives' parts folded onto one path. The reviewers' probes are the
// rows their names carry.
func pairingWorlds() []world {
	decl := func(id, ref, path string) *worldDecl {
		file, _, _ := strings.Cut(id, ":")
		return &worldDecl{id: id, ref: ref, file: file, path: path}
	}
	var worlds []world
	add := func(name string, decls []*worldDecl, findings ...*worldFinding) {
		for i, f := range findings {
			f.id = fmt.Sprintf("%s#%d", name, i)
		}
		worlds = append(worlds, world{name: name, decls: decls, findings: findings})
	}

	// A declaration: two in one file share a reference as two inits do, and
	// platform alternatives share one across files. at is where each one's
	// part renders, where a line directive moved it.
	type placement struct {
		name     string
		decls    []*worldDecl
		at       []string
		partOnly bool
	}
	alternatives := func() []*worldDecl {
		return []*worldDecl{decl("a.go:3:6", "go://x#f", "a.go"), decl("z.go:3:6", "go://x#f", "z.go")}
	}
	for _, p := range []placement{
		{name: "one", decls: []*worldDecl{decl("a.go:3:6", "go://x#f", "a.go")}},
		{name: "two", decls: []*worldDecl{decl("a.go:3:6", "go://x#f", "a.go"), decl("a.go:9:6", "go://x#f", "a.go")}},
		{name: "other-file", decls: alternatives()},
		{name: "folded (p6)", decls: []*worldDecl{decl("a_linux.go:6:6", "go://x#f", "ping.proto"), decl("z_windows.go:7:6", "go://x#f", "ping.proto")}},
		{name: "moved", decls: []*worldDecl{decl("a.go:3:6", "go://x#f", "gen.proto")}},
		{name: "part moved (adv-grok-r6-1)", decls: []*worldDecl{decl("a.go:3:6", "go://x#f", "a.go")}, at: []string{"gen.go"}, partOnly: true},
		{name: "part moved onto an alternative", decls: alternatives(), at: []string{"z.go", ""}, partOnly: true},
		{name: "parts folded", decls: alternatives(), at: []string{"gen.go", "gen.go"}, partOnly: true},
	} {
		var declarations, bindings, stores, parameters, both []*worldFinding
		for i, d := range p.decls {
			at := ""
			if i < len(p.at) {
				at = p.at[i]
			}
			declarations = append(declarations, &worldFinding{code: vocab.UnreachableExport, kind: "function", about: d})
			bindings = append(bindings, &worldFinding{code: vocab.UnusedImport, kind: "import", about: d})
			stores = append(stores, &worldFinding{code: vocab.DeadStore, kind: "store", about: d, at: at})
			parameters = append(parameters, &worldFinding{code: vocab.UnusedParameter, kind: "parameter", about: d, param: "a", at: at})
			both = append(both,
				&worldFinding{code: vocab.UnusedParameter, kind: "parameter", about: d, param: "a", at: at},
				&worldFinding{code: vocab.UnusedParameter, kind: "parameter", about: d, param: "b"})
		}
		if !p.partOnly {
			add("declaration "+p.name, p.decls, declarations...)
			add("binding "+p.name, p.decls, bindings...)
		}
		add("store "+p.name, p.decls, stores...)
		add("parameter "+p.name, p.decls, parameters...)
		add("two parameters "+p.name, p.decls, both...)
	}

	// A satisfaction assertion carries the reference of the interface it names
	// at its own position.
	writer := func() *worldDecl { return decl("z.go:5:6", "go://x#writer", "z.go") }
	for _, p := range []struct {
		name       string
		iface      *worldDecl
		twins      []*worldDecl
		others     []*worldDecl
		assertions []*worldDecl
	}{
		{name: "one", iface: writer(), assertions: []*worldDecl{decl("z.go:13:5", "go://x#_", "z.go")}},
		{name: "two (two-assert)", iface: writer(), assertions: []*worldDecl{decl("z.go:13:5", "go://x#_", "z.go"), decl("z.go:14:5", "go://x#_", "z.go")}},
		{name: "other-file (p1 p2 p3 p4 p5 two-dir)", iface: writer(), assertions: []*worldDecl{decl("z.go:13:5", "go://x#_", "z.go"), decl("a.go:7:5", "go://x#_", "a.go")}},
		{name: "folded (q1 q2a)", iface: decl("z.go:5:6", "go://x#writer", "gen.proto"), assertions: []*worldDecl{decl("z.go:13:5", "go://x#_", "gen.proto"), decl("a.go:7:5", "go://x#_", "gen.proto")}},
		{name: "renamed (q3)", iface: decl("z.go:5:6", "go://x#writer", "gen.y"), assertions: []*worldDecl{decl("z.go:16:5", "go://x#_", "other.y")}},
		{name: "moved onto the interface's path", iface: writer(), assertions: []*worldDecl{decl("a.go:7:5", "go://x#_", "z.go")}},
		{
			name: "folded onto an alternative it does not name", iface: writer(),
			others:     []*worldDecl{decl("y.go:5:6", "go://x#writer", "a.go")},
			assertions: []*worldDecl{decl("a.go:7:5", "go://x#_", "a.go")},
		},
		{
			name: "of an interface per platform (r6b)", iface: decl("iface_linux.go:3:6", "go://x#writer", "iface_linux.go"),
			twins:      []*worldDecl{decl("iface_windows.go:3:6", "go://x#writer", "iface_windows.go")},
			assertions: []*worldDecl{decl("a.go:3:5", "go://x#_", "a.go")},
		},
		{
			name: "of an interface per platform, folded (r6a)", iface: decl("iface_linux.go:4:6", "go://x#writer", "gen.proto"),
			twins:      []*worldDecl{decl("iface_windows.go:3:6", "go://x#writer", "iface_windows.go")},
			assertions: []*worldDecl{decl("a.go:4:5", "go://x#_", "gen.proto")},
		},
	} {
		var findings []*worldFinding
		for _, a := range p.assertions {
			findings = append(findings, &worldFinding{code: vocab.UnusedSatisfactionAssertion, kind: "satisfaction-assertion", about: a, iface: p.iface, twins: p.twins})
		}
		decls := append(append(append([]*worldDecl{p.iface}, p.twins...), p.others...), p.assertions...)
		add("assertion "+p.name, decls, findings...)
	}

	// A row of a document is no declaration: a dependency row, a file and a
	// record of deadset's own configuration.
	add("dependency one", nil, &worldFinding{code: vocab.UnusedDependency, kind: "dependency", ref: "go://x#y:require", path: "go.mod"})
	add("dependency other-file", nil,
		&worldFinding{code: vocab.UnusedDependency, kind: "dependency", ref: "go://x#y:require", path: "go.mod"},
		&worldFinding{code: vocab.UnusedDependency, kind: "dependency", ref: "go://x#y:require", path: "sub/go.mod"})
	add("file one", nil, &worldFinding{code: vocab.FileNeverBuilt, kind: "file", ref: "go://x#a.go:file", path: "a.go"})
	add("file other-file", nil,
		&worldFinding{code: vocab.FileNeverBuilt, kind: "file", ref: "go://x#a.go:file", path: "a.go"},
		&worldFinding{code: vocab.FileNeverBuilt, kind: "file", ref: "go://x#z.go:file", path: "z.go"})
	add("record one", nil, &worldFinding{code: vocab.StaleSuppression, kind: "suppression", ref: "go://x#f", path: "a.go"})
	return worlds
}

// Every record the pairing world can hold, alone, twice, and all of them in
// reading order, withholds what the oracle says it names and nothing else: one
// record claims every finding it names, and a later record naming only claimed
// findings is stale.
func TestDecidePairsAsTheWorldSays(t *testing.T) {
	t.Parallel()

	for _, w := range pairingWorlds() {
		candidates := w.candidates()
		var sets [][]*worldRecord
		for _, r := range candidates {
			sets = append(sets, []*worldRecord{r}, []*worldRecord{r, r})
		}
		sets = append(sets, candidates)
		for _, set := range sets {
			checkPairing(t, &w, set)
		}
	}
}

func checkPairing(t *testing.T, w *world, set []*worldRecord) {
	t.Helper()
	var records []Record
	var of []int
	for i, r := range set {
		resolved := w.records(t, r, i+1)
		records = append(records, resolved...)
		for range resolved {
			of = append(of, i)
		}
	}
	would := make([]Finding, len(w.findings))
	for i, f := range w.findings {
		would[i] = f.finding()
	}
	ledger := Decide(records, would, &noDials)

	claimed := map[*worldFinding]bool{}
	for i, r := range set {
		inEffect := false
		for j := range records {
			inEffect = inEffect || of[j] == i && ledger.Verdicts[j] == InEffect
		}
		want := false
		for _, f := range w.findings {
			if r.names(f) && !claimed[f] {
				claimed[f], want = true, true
			}
		}
		if inEffect != want {
			t.Errorf("%s, record %d of %s: in effect %t, want %t", w.name, i+1, describe(set), inEffect, want)
		}
	}
	for i, f := range w.findings {
		if got := ledger.Withheld(&would[i]); got != claimed[f] {
			t.Errorf("%s, finding %d under %s: withheld %t, want %t", w.name, i, describe(set), got, claimed[f])
		}
	}
}

func describe(set []*worldRecord) string {
	spelled := make([]string, len(set))
	for i, r := range set {
		if r.mechanism == Inline {
			above := "nothing"
			if r.above != nil {
				above = r.above.id
			}
			spelled[i] = fmt.Sprintf("inline %s above %s in %s", r.code, above, r.file)
			continue
		}
		spelled[i] = fmt.Sprintf("%s %s %s at %s", r.mechanism, r.code, r.symbol, r.path)
	}
	return "[" + strings.Join(spelled, "; ") + "]"
}
