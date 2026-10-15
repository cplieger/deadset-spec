package render

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/cplieger/deadset-spec/v8/report"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// The kind token and the confidence a stale suppression's text line carries:
// the record is about a suppression whatever subject it named, and its kind is
// fixed on, so the line claims what the analysis knows.
const (
	staleSuppressionToken      = "suppression"
	staleSuppressionConfidence = vocab.Certain
)

// staleSuppression is the kind a stale-suppression record's code names.
var staleSuppression, _ = vocab.KindOf(vocab.StaleSuppression)

// Text writes one line per finding of r and then one per stale suppression,
// in the shape contract/grammar/text-line.md states, then the withheld line
// where the totals count a withheld finding.
func Text(w io.Writer, r *report.Report) error {
	var text strings.Builder
	for i := range r.Findings {
		found := &r.Findings[i]
		fmt.Fprintf(&text, "%s:%d:%d: %s %s: %s [%s] (%s)\n",
			found.Position.Path, found.Position.Line, found.Position.Column,
			found.Symbol.Kind, found.Symbol.Name, found.Message, found.Confidence, found.Code)
	}
	for i := range r.StaleSuppressions {
		stale := &r.StaleSuppressions[i]
		fmt.Fprintf(&text, "%s:%d:%d: %s %s: %s [%s] (%s)\n",
			stale.Position.Path, stale.Position.Line, stale.Position.Column,
			staleSuppressionToken, stale.Symbol, stale.Message, staleSuppressionConfidence, stale.Code)
	}
	if line := r.Totals.Withheld.Line(); line != "" {
		text.WriteString(line + "\n")
	}
	_, err := io.WriteString(w, text.String())
	return err
}

// Summary writes the summary of r, which is no finding line: on a merged
// report one line per analyzer it was merged from, naming its version and its
// artifact's digest, then the summary line over every count but the withheld
// ones, which on a merged report ends with the languages it covers.
func Summary(w io.Writer, r *report.Report) error {
	var text strings.Builder
	for i := range r.MergedFrom {
		ran := &r.MergedFrom[i]
		fmt.Fprintf(&text, "analyzer %s %s %s\n", ran.Name, ran.Version, ran.Digest)
	}
	totals := &r.Totals
	fmt.Fprintf(&text, "summary: %s (%d allow, %d warn, %d deny), %s, %s, %s, %s, %d pending, %d omitted",
		counted(totals.Findings, "finding", "findings"),
		totals.BySeverity.Allow, totals.BySeverity.Warn, totals.BySeverity.Deny,
		counted(totals.DeletableLines, "deletable line", "deletable lines"),
		counted(totals.SuppressionsInEffect, "suppression in effect", "suppressions in effect"),
		counted(totals.ReasonsRecorded, "reason recorded", "reasons recorded"),
		counted(totals.StaleSuppressions, "stale suppression", "stale suppressions"),
		totals.Pending, totals.Omitted)
	if r.MergedFrom != nil {
		text.WriteString(", across " + sentence(r.Analyzer.Languages))
	}
	text.WriteString("\n")
	_, err := io.WriteString(w, text.String())
	return err
}

// counted renders a count with the noun for it.
func counted(n int, one, many string) string {
	if n == 1 {
		return strconv.Itoa(n) + " " + one
	}
	return strconv.Itoa(n) + " " + many
}

// sentence joins languages as a sentence lists them: "go", "go and ts".
func sentence(languages []vocab.Language) string {
	names := make([]string, len(languages))
	for i, language := range languages {
		names[i] = string(language)
	}
	if len(names) < 2 {
		return strings.Join(names, "")
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// Annotations writes the workflow commands contract/grammar/annotations.md
// states for r: one per finding at the command its severity alone names, then
// one error per stale suppression, one warning per type-error skip and one
// warning per note.
func Annotations(w io.Writer, r *report.Report) error {
	var text strings.Builder
	for i := range r.Findings {
		found := &r.Findings[i]
		annotate(&text, commandOf(found.Severity), &found.Position, string(found.Code)+" "+found.Kind, found.Message)
	}
	for i := range r.StaleSuppressions {
		stale := &r.StaleSuppressions[i]
		at := report.Position{
			Path: stale.Position.Path, Line: stale.Position.Line,
			Column: stale.Position.Column, EndLine: stale.Position.Line,
		}
		annotate(&text, commandError, &at, string(stale.Code)+" "+staleSuppression.Name, stale.Message)
	}
	for _, skip := range r.TypeErrorSkips {
		fmt.Fprintf(&text, "::%s file=%s,line=%d,title=%s::%s\n", commandWarning,
			propertyEscapes.Replace(skip.Path), skip.Line, propertyEscapes.Replace(typeErrorSkipTitle),
			dataEscapes.Replace(typeErrorSkipMessage+skip.Message))
	}
	for _, note := range r.Notes {
		fmt.Fprintf(&text, "::%s file=%s,title=%s::%s\n", commandWarning,
			propertyEscapes.Replace(note.Path), propertyEscapes.Replace(string(note.Kind)), dataEscapes.Replace(note.Message))
	}
	_, err := io.WriteString(w, text.String())
	return err
}

// The workflow commands, and the title and message prefix of a type-error
// skip's annotation.
const (
	commandError   = "error"
	commandWarning = "warning"
	commandNotice  = "notice"

	typeErrorSkipTitle   = "type error skipped"
	typeErrorSkipMessage = "the analysis did not evaluate the function or statement holding this type error: "
)

// commandOf is the workflow command a finding of the given severity is
// annotated with.
func commandOf(severity vocab.Severity) string {
	switch severity {
	case vocab.Deny:
		return commandError
	case vocab.Warn:
		return commandWarning
	default:
		return commandNotice
	}
}

// annotate writes one workflow command, each value escaped as the command
// grammar requires so a separator inside it cannot end a property or the line.
func annotate(text *strings.Builder, command string, at *report.Position, title, message string) {
	fmt.Fprintf(text, "::%s file=%s,line=%d,col=%d,endLine=%d,title=%s::%s\n",
		command, propertyEscapes.Replace(at.Path), at.Line, at.Column, at.EndLine,
		propertyEscapes.Replace(title), dataEscapes.Replace(message))
}

// The escapes of a workflow command: its message escapes the escape character
// and both line breaks, and a property value escapes those and the two
// separators of the property list as well.
var (
	dataEscapes     = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A")
	propertyEscapes = strings.NewReplacer("%", "%25", "\r", "%0D", "\n", "%0A", ":", "%3A", ",", "%2C")
)
