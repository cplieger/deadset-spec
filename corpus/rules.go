package corpus

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"regexp"
	"slices"
	"strings"

	spec "github.com/cplieger/deadset-spec/v8"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

const rulePagesDir = "contract/grammar/analysis"

// The coverage defects of the corpus's rule citations.
var (
	ErrUnknownRule         = errors.New("fixture cites a rule id no analysis page states")
	ErrRuleLanguageMissing = errors.New("fixture cites a rule of another language's page")
	ErrRuleOneLanguage     = errors.New("a rule every language follows is cited by no fixture rendered in every language")
)

var ruleIDPattern = regexp.MustCompile(`\*\*([a-z][a-z-]*\.\d+)\*\*`)

// Rules maps each rule id the analysis pages state to the languages that follow
// it: the language of a references-<language> page, the schema languages for
// schemas.md, and the analyzed languages for every other page.
type Rules map[string][]string

// schemaPage is the page whose rules the readers of the schema languages follow.
const schemaPage = "schemas"

// ReadRules reads the rule ids of every analysis page of the Contract.
func ReadRules() (Rules, error) {
	fsys := spec.Contract
	entries, err := fs.ReadDir(fsys, rulePagesDir)
	if err != nil {
		return nil, err
	}
	byKind := map[string][]string{}
	rows := vocab.LanguageRows()
	for i := range rows {
		byKind[rows[i].Kind] = append(byKind[rows[i].Kind], string(rows[i].Language))
	}
	rules := Rules{}
	for _, entry := range entries {
		page, isPage := strings.CutSuffix(entry.Name(), ".md")
		if !isPage {
			continue
		}
		data, err := fs.ReadFile(fsys, path.Join(rulePagesDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		languages := pageLanguages(page, byKind)
		for _, match := range ruleIDPattern.FindAllStringSubmatch(string(data), -1) {
			if id := match[1]; strings.HasPrefix(id, page+".") {
				rules[id] = languages
			}
		}
	}
	return rules, nil
}

// checkCoverage holds the rule citations of the fixtures to the pages: every
// cited id exists and is followed by every language its fixture lists, and a rule
// every language follows that a fixture outside pending cites is cited by one
// such fixture listing every language, so its parity is pinned in both.
func checkCoverage(fixtures []Fixture, rules Rules) []error {
	var errs []error
	citing := map[string][]string{}
	both := map[string]bool{}
	for i := range fixtures {
		expect := &fixtures[i].Expect
		for _, id := range expect.Rules {
			languages, known := rules[id]
			if !known {
				errs = append(errs, fmt.Errorf("fixture %s: %w: %s", expect.Name, ErrUnknownRule, id))
				continue
			}
			errs = append(errs, followedBy(expect, id, languages)...)
			if expect.Pending != "" || len(languages) < 2 {
				continue
			}
			citing[id] = append(citing[id], expect.Name)
			both[id] = both[id] || !slices.ContainsFunc(languages, func(language string) bool { return !slices.Contains(expect.Languages, language) })
		}
	}
	for _, id := range slices.Sorted(maps.Keys(citing)) {
		if !both[id] {
			errs = append(errs, fmt.Errorf("%w: %s, cited by %v", ErrRuleOneLanguage, id, citing[id]))
		}
	}
	return errs
}

// followedBy names each language the fixture lists that the cited rule's page
// does not cover.
func followedBy(expect *Expectation, id string, languages []string) []error {
	var errs []error
	for _, language := range expect.Languages {
		if !slices.Contains(languages, language) {
			errs = append(errs, fmt.Errorf("fixture %s: %w: %s is followed by %v and the fixture lists %s", expect.Name, ErrRuleLanguageMissing, id, languages, language))
		}
	}
	return errs
}

// pageLanguages is the languages that follow the rules of one page, given the
// languages of each row kind.
func pageLanguages(page string, byKind map[string][]string) []string {
	if language, own := strings.CutPrefix(page, "references-"); own {
		return []string{language}
	}
	if page == schemaPage {
		return byKind["schema"]
	}
	return byKind["analyzed"]
}
