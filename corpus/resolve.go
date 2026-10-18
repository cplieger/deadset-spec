package corpus

import (
	"errors"
	"fmt"
	"maps"
	"slices"
)

// The defects of a fixture's logical names, each wrapped with the name it caught.
var (
	ErrDuplicateSymbol = errors.New("logical name expected twice")
	ErrUnboundSymbol   = errors.New("logical name absent from the manifest")
	ErrOrphanSymbol    = errors.New("manifest name no expectation uses")
	ErrOrphanRoot      = errors.New("configured root no expectation uses")
	ErrRootAlsoBound   = errors.New("configured root a manifest binds as well")
	ErrOrphanEntry     = errors.New("configured declaration no expectation uses")
	ErrEntryAlsoBound  = errors.New("configured declaration a manifest or configured_roots binds as well")
	ErrNoFile          = errors.New("manifest entry has no file")
	ErrNoLine          = errors.New("manifest entry has no line")
	ErrSharedPosition  = errors.New("manifest binds two names to one file and line")
	ErrNoRef           = errors.New("manifest entry a facts entry names has no ref")
	ErrStrayRef        = errors.New("manifest entry no facts entry names has a ref")
)

// Resolve binds every logical name the expectation file uses to its site in the
// manifest of language's rendering: each row's symbol, type_error_skips name,
// name_literal and edge evaluation's symbol, and the names of each facts entry
// that applies to language, which also need the manifest's ref. A configured
// root or declaration resolves to no site: its finding sits in the configuration
// document the runner writes. It fails on a name used twice or bound by neither
// or both documents, an entry without a file or a line, a manifest name or
// configured entry no expectation uses, and two names at one position.
func Resolve(expect *Expectation, language string, manifest *Manifest) (map[string]Site, error) {
	r := &resolver{expect: expect, language: language, manifest: manifest, sites: map[string]Site{}, used: map[string]bool{}, rows: map[string]bool{}, facts: map[string]bool{}}
	for i := range expect.Expect {
		r.rows[expect.Expect[i].Symbol] = true
	}
	r.evaluationSymbols()
	r.skips()
	r.literals()
	r.declaredRows()
	r.factNames()
	r.manifestNames()
	r.configuredEntries()
	if err := errors.Join(r.errs...); err != nil {
		return nil, err
	}
	return r.sites, nil
}

type resolver struct {
	expect   *Expectation
	language string
	manifest *Manifest
	sites    map[string]Site
	used     map[string]bool
	rows     map[string]bool
	facts    map[string]bool
	errs     []error
}

func (r *resolver) fail(err error, name string) {
	r.errs = append(r.errs, fmt.Errorf("%w: %q", err, name))
}

// bind resolves one logical name through the manifest, marking it used once it
// resolves.
func (r *resolver) bind(name string) {
	entry, ok := r.manifest.Symbols[name]
	switch {
	case !ok:
		r.fail(ErrUnboundSymbol, name)
	case entry.File == "":
		r.fail(ErrNoFile, name)
	case entry.Line == nil || *entry.Line < 1:
		r.fail(ErrNoLine, name)
	default:
		r.used[name] = true
		r.sites[name] = Site{File: entry.File, Line: *entry.Line}
	}
}

// evaluationSymbols names every edge evaluation symbol the manifest lacks.
func (r *resolver) evaluationSymbols() {
	for _, evaluation := range r.expect.EdgeEvaluations {
		if _, ok := r.manifest.Symbols[evaluation.Symbol]; evaluation.Symbol != "" && !ok {
			r.fail(ErrUnboundSymbol, evaluation.Symbol)
		}
	}
}

func (r *resolver) skips() {
	for _, name := range r.expect.TypeErrorSkips {
		if r.rows[name] {
			r.fail(ErrDuplicateSymbol, name)
			continue
		}
		r.bind(name)
	}
}

// literals binds each name_literal once, however many rows share it.
func (r *resolver) literals() {
	seen := map[string]bool{}
	for i := range r.expect.Expect {
		details := r.expect.Expect[i].Details
		if details == nil || details.NameLiteral == "" || seen[details.NameLiteral] {
			continue
		}
		name := details.NameLiteral
		seen[name] = true
		if r.used[name] {
			r.fail(ErrDuplicateSymbol, name)
			continue
		}
		r.bind(name)
	}
	for _, evaluation := range r.expect.EdgeEvaluations {
		if evaluation.Symbol != "" && !r.rows[evaluation.Symbol] {
			r.used[evaluation.Symbol] = true
		}
	}
}

// declaredRows binds each row's symbol, a configured entry's to no site.
func (r *resolver) declaredRows() {
	for i := range r.expect.Expect {
		name := r.expect.Expect[i].Symbol
		if r.used[name] {
			r.fail(ErrDuplicateSymbol, name)
			continue
		}
		r.used[name] = true
		_, bound := r.manifest.Symbols[name]
		_, root := r.expect.ConfiguredRoots[name]
		_, declared := r.expect.ConfiguredDeclarations[name]
		switch {
		case declared && (bound || root):
			r.fail(ErrEntryAlsoBound, name)
		case declared:
		case root && bound:
			r.fail(ErrRootAlsoBound, name)
		case root:
		default:
			r.bind(name)
		}
	}
}

// factNames binds the symbol and via names of each facts entry applying to the
// rendering, which may also be a row's, and refuses two entries applying to one
// rendering that name one role and symbol.
func (r *resolver) factNames() {
	seen := map[[3]string]bool{}
	for i := range r.expect.Facts {
		row := &r.expect.Facts[i]
		for _, language := range r.expect.Languages {
			key := [3]string{language, row.Role, row.Symbol}
			if row.appliesTo(language) && seen[key] {
				r.fail(ErrDuplicateSymbol, row.Symbol)
			}
			seen[key] = seen[key] || row.appliesTo(language)
		}
		if row.appliesTo(r.language) {
			r.factName(row)
		}
	}
}

// factName binds the symbol, via and carried names of one facts entry once
// each, and refuses one whose manifest entry carries no ref.
func (r *resolver) factName(row *FactRow) {
	names := append([]string{row.Symbol}, row.Via...)
	for _, c := range row.Carried {
		names = append(names, c.Symbol)
	}
	for _, name := range names {
		if r.facts[name] {
			continue
		}
		r.facts[name] = true
		if _, bound := r.sites[name]; !bound {
			r.bind(name)
		}
		if entry, ok := r.manifest.Symbols[name]; ok && entry.Ref == "" {
			r.fail(ErrNoRef, name)
		}
	}
}

// manifestNames names every manifest name nothing uses, every position two
// names share, and every ref on a name no facts entry names.
func (r *resolver) manifestNames() {
	at := map[Site]string{}
	for _, name := range slices.Sorted(maps.Keys(r.manifest.Symbols)) {
		if !r.used[name] {
			r.fail(ErrOrphanSymbol, name)
		}
		entry := r.manifest.Symbols[name]
		if entry.Ref != "" && !r.facts[name] {
			r.fail(ErrStrayRef, name)
		}
		if entry.File == "" || entry.Line == nil {
			continue
		}
		site := Site{File: entry.File, Line: *entry.Line}
		if other, taken := at[site]; taken {
			r.errs = append(r.errs, fmt.Errorf("%w: %q and %q at %s", ErrSharedPosition, other, name, site))
			continue
		}
		at[site] = name
	}
}

func (r *resolver) configuredEntries() {
	for _, name := range slices.Sorted(maps.Keys(r.expect.ConfiguredRoots)) {
		if !r.used[name] {
			r.fail(ErrOrphanRoot, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(r.expect.ConfiguredDeclarations)) {
		if !r.used[name] {
			r.fail(ErrOrphanEntry, name)
		}
	}
}
