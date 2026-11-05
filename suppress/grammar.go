package suppress

import (
	"regexp"
	"strings"
	"sync"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// referenceForm is one form symbol-ref.md fixes: the language whose scheme
// opens it and its anchored expression.
type referenceForm struct{ language, expression string }

// compiledForms is every form's expression by the scheme that opens it.
var compiledForms = sync.OnceValue(func() map[string][]*regexp.Regexp {
	forms := map[string][]*regexp.Regexp{}
	for _, form := range referenceForms {
		scheme := form.language + "://"
		forms[scheme] = append(forms[scheme], regexp.MustCompile(form.expression))
	}
	return forms
})

// ValidReference reports whether ref takes one of the forms symbol-ref.md
// fixes for the language its scheme names; a scheme the grammar names no form
// for takes none.
func ValidReference(ref string) bool {
	scheme, _, found := strings.Cut(ref, "://")
	if !found {
		return false
	}
	for _, form := range compiledForms()[scheme+"://"] {
		if form.MatchString(ref) {
			return true
		}
	}
	return false
}

// DirectiveVerdict is the step of suppression.md's decision procedure one
// comment reaches.
type DirectiveVerdict uint8

// The verdicts, in the procedure's order.
const (
	// NotADirective is a comment outside the namespace: nothing happens,
	// whatever else it says.
	NotADirective DirectiveVerdict = iota

	// WellFormed is a directive: one record per code, each carrying the reason.
	WellFormed

	// Reasonless is a directive with no reason: one refusal per code.
	Reasonless

	// Malformed is a comment in the namespace the grammar refuses, which ends
	// the run.
	Malformed
)

// Directive is what the decision procedure makes of one comment.
type Directive struct {
	Reason  string
	Codes   []vocab.Code
	Verdict DirectiveVerdict
}

// directiveExpressions are the three expressions, compiled once.
var directiveExpressions = sync.OnceValue(func() [3]*regexp.Regexp {
	return [3]*regexp.Regexp{
		regexp.MustCompile(directiveCandidate),
		regexp.MustCompile(directiveWellFormed),
		regexp.MustCompile(directiveReasonless),
	}
})

// ReadDirective applies the procedure to the text of one line comment, from
// its first solidus to the end of its line with no line terminator.
func ReadDirective(comment string) Directive {
	expressions := directiveExpressions()
	if !expressions[0].MatchString(comment) {
		return Directive{Verdict: NotADirective}
	}
	if match := expressions[1].FindStringSubmatch(comment); match != nil {
		return Directive{
			Verdict: WellFormed,
			Codes:   codesOf(expressions[1], match),
			Reason:  match[expressions[1].SubexpIndex("reason")],
		}
	}
	if match := expressions[2].FindStringSubmatch(comment); match != nil {
		return Directive{Verdict: Reasonless, Codes: codesOf(expressions[2], match)}
	}
	return Directive{Verdict: Malformed}
}

// codesOf splits the code list an expression captured.
func codesOf(expression *regexp.Regexp, match []string) []vocab.Code {
	spelled := strings.Split(match[expression.SubexpIndex("codes")], ",")
	codes := make([]vocab.Code, len(spelled))
	for i, code := range spelled {
		codes[i] = vocab.Code(code)
	}
	return codes
}

// MalformedDirective is the error a malformed directive ends the run with.
func MalformedDirective(comment string, site Site) *MalformedError {
	return &MalformedError{Text: comment, Want: "//" + directiveShape, Mechanism: Inline, Site: site}
}
