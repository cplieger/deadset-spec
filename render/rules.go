package render

import (
	"slices"
	"strings"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// rulesFor is one rule per live kind that applies to any of languages, ordered
// by code, whether or not a result names it: a fixed list keeps every rule's
// index stable across runs and configurations. Nil languages selects every
// live kind.
func rulesFor(languages []vocab.Language) []sarifRule {
	kinds := vocab.Kinds()
	slices.SortFunc(kinds, func(a, b vocab.Kind) int { return strings.Compare(string(a.Code), string(b.Code)) })
	rules := make([]sarifRule, 0, len(kinds))
	for i := range kinds {
		k := &kinds[i]
		if languages != nil && !slices.ContainsFunc(languages, k.AppliesTo) {
			continue
		}
		help := k.Rule
		if k.Precondition != "" {
			help += "\n\n" + k.Precondition
		}
		rules = append(rules, sarifRule{
			ID:                   k.Code,
			Name:                 k.Name,
			ShortDescription:     sarifMessage{Text: firstSentence(k.Rule)},
			FullDescription:      sarifMessage{Text: k.Rule},
			Help:                 sarifMessage{Text: help},
			DefaultConfiguration: sarifConfiguration{Level: levelOf(k.DefaultSeverity)},
			Properties: sarifRuleProperties{
				Precision: precisionOf(k.MaxClass),
				Problem:   sarifProblem{Severity: problemOf(k.DefaultSeverity)},
			},
		})
	}
	return rules
}

// firstSentence is text up to and including the first full stop that a space
// follows or that ends text, and all of text where none does.
func firstSentence(text string) string {
	for i := range len(text) {
		if text[i] == '.' && (i+1 == len(text) || text[i+1] == ' ') {
			return text[:i+1]
		}
	}
	return text
}

// The levels a severity maps to, and the problem severity and the precisions
// only a rule carries.
const (
	levelError   = "error"
	levelWarning = "warning"
	levelNote    = "note"

	problemRecommendation = "recommendation"

	precisionVeryHigh = "very-high"
	precisionHigh     = "high"
	precisionMedium   = "medium"
)

// levelOf is the level a severity maps to on a result and on a rule's default
// configuration.
func levelOf(severity vocab.Severity) string {
	switch severity {
	case vocab.Deny:
		return levelError
	case vocab.Warn:
		return levelWarning
	default:
		return levelNote
	}
}

// problemOf is the problem severity a kind's default severity maps to.
func problemOf(severity vocab.Severity) string {
	switch severity {
	case vocab.Deny:
		return levelError
	case vocab.Warn:
		return levelWarning
	default:
		return problemRecommendation
	}
}

// precisionOf is the precision a kind's confidence ceiling maps to.
func precisionOf(ceiling vocab.Class) string {
	switch ceiling {
	case vocab.Certain:
		return precisionVeryHigh
	case vocab.Probable:
		return precisionHigh
	default:
		return precisionMedium
	}
}
