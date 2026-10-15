package gen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"regexp"
	"slices"
	"strings"
	"text/template"
)

const docsTemplates = "authoring/docs"

// docsPages writes the reference pages under docs/ from the Contract's data and
// the hand-written templates that frame it.
func docsPages(fsys fs.FS, out map[string][]byte) error {
	data, err := loadDocsData(fsys)
	if err != nil {
		return err
	}
	tmpl, err := template.New("docs").Funcs(template.FuncMap{
		"codes": codeList,
		"ticks": ticks,
		"onoff": func(b bool) string { return map[bool]string{true: "on", false: "off"}[b] },
		"fence": fence,
		"rulepage": func(id string) string {
			page, _, _ := strings.Cut(id, ".")
			return "contract/grammar/analysis/" + page + ".md"
		},
	}).ParseFS(fsys, docsTemplates+"/*.tmpl")
	if err != nil {
		return err
	}
	render := func(path, name string, v any) error {
		var b bytes.Buffer
		if err := tmpl.ExecuteTemplate(&b, name, v); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		out[path] = tidy(b.Bytes())
		return nil
	}
	if err := render("docs/kinds.md", "kinds.md.tmpl", data); err != nil {
		return err
	}
	for i := range data.Ranges {
		r := &data.Ranges[i]
		if err := render("docs/kinds/"+r.Family+".md", "family.md.tmpl", familyPage{Range: r}); err != nil {
			return err
		}
	}
	for _, page := range []string{"exemptions", "exit-codes", "confidence"} {
		if err := render("docs/"+page+".md", page+".md.tmpl", data); err != nil {
			return err
		}
	}
	return nil
}

type docsData struct {
	Kinds      kindsDoc
	Ranges     []rangeRow
	Guide      map[string]guideRow
	Exemptions struct {
		Exemptions []map[string]any `json:"exemptions"`
		Sinks      []map[string]any `json:"sinks"`
	}
	Exits      map[string]any
	Confidence struct {
		Evidence []evidenceRow `json:"evidence"`
	}
}

type kindsDoc struct {
	Prefix              string          `json:"prefix"`
	Languages           []string        `json:"languages"`
	Severities          []string        `json:"severities"`
	ReachabilityClasses []string        `json:"reachability_classes"`
	Fixabilities        []string        `json:"fixabilities"`
	Subjects            []string        `json:"subjects"`
	SymbolKinds         json.RawMessage `json:"symbol_kinds"`
	Ranges              []rangeRow      `json:"ranges"`
	Kinds               []kindDoc       `json:"kinds"`
	Retired             []struct {
		Code string `json:"code"`
		Name string `json:"name"`
	} `json:"retired"`
}

type kindDoc struct {
	Details           map[string]any      `json:"details"`
	Overlap           map[string][]string `json:"overlap"`
	Guide             guideRow            `json:"-"`
	Code              string              `json:"code"`
	Rule              string              `json:"rule"`
	Precondition      string              `json:"precondition"`
	DefaultSeverity   string              `json:"default_severity"`
	MaxClass          string              `json:"max_class"`
	Fixability        string              `json:"fixability"`
	Subject           string              `json:"subject"`
	Name              string              `json:"name"`
	Lowered           []evidenceRow       `json:"-"`
	OverlapByLanguage [][2]string         `json:"-"`
	FindingFixability []string            `json:"finding_fixabilities"`
	DerivedFrom       []string            `json:"derived_from"`
	Languages         []string            `json:"languages"`
	HoldsLive         bool                `json:"holds_live"`
	Fixed             bool                `json:"fixed"`
	DefaultEnabled    bool                `json:"default_enabled"`
}

type rangeRow struct {
	Start       string `json:"start"`
	End         string `json:"end"`
	Family      string `json:"family"`
	Title       string `json:"title"`
	Question    string `json:"question"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Kinds       []kindDoc
}

type guideRow struct {
	Code    string `json:"code"`
	Why     string `json:"why"`
	Fix     string `json:"fix"`
	Example struct {
		Language string `json:"language"`
		Before   string `json:"before"`
		After    string `json:"after"`
	} `json:"example"`
}

type evidenceRow struct {
	Evidence    string   `json:"evidence"`
	Rule        string   `json:"rule"`
	Class       string   `json:"class"`
	Description string   `json:"description"`
	Languages   []string `json:"languages"`
	Codes       []string `json:"codes"`
}

type familyPage struct {
	Range *rangeRow
}

func loadDocsData(fsys fs.FS) (*docsData, error) {
	d := &docsData{Guide: map[string]guideRow{}}
	var ranges struct {
		Ranges []rangeRow `json:"ranges"`
	}
	var guide struct {
		Kinds []guideRow `json:"kinds"`
	}
	sources := []struct {
		into any
		path string
	}{
		{into: &d.Kinds, path: kindsPath},
		{into: &ranges, path: kindsPath},
		{into: &guide, path: "authoring/kinds-guide.json"},
		{into: &d.Exemptions, path: "contract/exemptions.json"},
		{into: &d.Exits, path: "contract/exit-codes.json"},
		{into: &d.Confidence, path: "contract/confidence.json"},
	}
	for _, src := range sources {
		if err := decodeFile(fsys, src.path, src.into); err != nil {
			return nil, err
		}
	}
	for i := range guide.Kinds {
		d.Guide[guide.Kinds[i].Code] = guide.Kinds[i]
	}
	if len(guide.Kinds) != len(d.Kinds.Kinds) {
		return nil, fmt.Errorf("authoring/kinds-guide.json: %d entries for %d kinds", len(guide.Kinds), len(d.Kinds.Kinds))
	}
	for i := range d.Kinds.Kinds {
		if err := d.complete(&d.Kinds.Kinds[i]); err != nil {
			return nil, err
		}
	}
	for i := range ranges.Ranges {
		r := ranges.Ranges[i]
		for j := range d.Kinds.Kinds {
			if code := d.Kinds.Kinds[j].Code; code >= r.Start && code <= r.End {
				r.Kinds = append(r.Kinds, d.Kinds.Kinds[j])
			}
		}
		d.Ranges = append(d.Ranges, r)
	}
	return d, nil
}

func decodeFile(fsys fs.FS, path string, into any) error {
	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, into); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// complete attaches to one kind its guide entry, the evidence rows that lower
// its confidence and its overlap lists in language order.
func (d *docsData) complete(k *kindDoc) error {
	g, ok := d.Guide[k.Code]
	if !ok {
		return fmt.Errorf("authoring/kinds-guide.json: no entry for %s", k.Code)
	}
	k.Guide = g
	for i := range d.Confidence.Evidence {
		e := &d.Confidence.Evidence[i]
		if slices.Contains(e.Codes, k.Code) && overlaps(e.Languages, k.Languages) {
			k.Lowered = append(k.Lowered, *e)
		}
	}
	for _, lang := range slices.Sorted(maps.Keys(k.Overlap)) {
		k.OverlapByLanguage = append(k.OverlapByLanguage, [2]string{lang, strings.Join(k.Overlap[lang], ", ")})
	}
	return nil
}

func overlaps(a, b []string) bool {
	return slices.ContainsFunc(a, func(x string) bool { return slices.Contains(b, x) })
}

func ticks(ss []string) string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = "`" + s + "`"
	}
	return strings.Join(out, ", ")
}

func codeList(ks []kindDoc) []string {
	out := make([]string, len(ks))
	for i := range ks {
		out[i] = ks[i].Code
	}
	return out
}

func fence(lang, body string) string {
	return "```" + lang + "\n" + body + "\n```"
}

var blankRuns = regexp.MustCompile(`\n{3,}`)

// tidy collapses the blank lines a template's control actions leave, so a page
// is the same whatever whitespace its template carries around an action.
func tidy(b []byte) []byte {
	s := strings.TrimSpace(blankRuns.ReplaceAllString(string(b), "\n\n"))
	return []byte(s + "\n")
}
