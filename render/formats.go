package render

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/cplieger/deadset-spec/v8/report"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// Options is what a rendering reads beside the report.
type Options struct {
	// Read returns the file at a target-relative path, which the SARIF
	// rendering's line fingerprints hash.
	Read func(path string) ([]byte, error)

	// Template is the parsed template the template rendering renders.
	Template *Template

	// BySize renders the findings by the lines their deletion removes, as
	// reporters.sort size asks; the JSON document stays in canonical order.
	BySize bool
}

// errOptions reports a rendering asked for without what it reads.
var errOptions = errors.New("render: the rendering lacks what it reads")

// Render writes r in format.
func Render(w io.Writer, format vocab.Format, r *report.Report, opts *Options) error {
	if format == vocab.FormatJSON {
		return report.Encode(w, r)
	}
	shown := r
	if opts.BySize {
		sorted := *r
		sorted.Findings = report.BySize(r.Findings)
		shown = &sorted
	}
	switch format {
	case vocab.FormatText:
		return Text(w, shown)
	case vocab.FormatGitHub:
		return Annotations(w, shown)
	case vocab.FormatSARIF:
		if opts.Read == nil {
			return fmt.Errorf("%w: the SARIF rendering reads the source lines it fingerprints", errOptions)
		}
		return SARIF(w, shown, opts.Read)
	case vocab.FormatTemplate:
		if opts.Template == nil {
			return fmt.Errorf("%w: the template rendering renders a template", errOptions)
		}
		return opts.Template.Render(w, shown)
	}
	return fmt.Errorf("render: %q is not a format", format)
}

// Verdict is the exit code the Contract's table gives a report whose totals
// are t under the failing severity failOn: a pending finding outranks
// everything, and a stale suppression or a failing finding yields the
// findings code. The totals describe the whole run, so a maximum finding
// count cannot change the verdict.
func Verdict(t *report.Totals, failOn vocab.Severity) int {
	switch {
	case t.Pending > 0:
		return vocab.ExitPending
	case t.StaleSuppressions > 0, Failing(&t.BySeverity, failOn):
		return vocab.ExitFindings
	default:
		return vocab.ExitClean
	}
}

// Failing reports whether a finding the counts hold carries a severity at or
// above failOn, in the vocabulary's order from allow to deny. failOn is one of
// the vocabulary's severities: both products refuse any other fail_on before a
// report exists.
func Failing(counts *report.BySeverity, failOn vocab.Severity) bool {
	order := vocab.Severities()
	at := slices.Index(order, failOn)
	held := map[vocab.Severity]int{vocab.Allow: counts.Allow, vocab.Warn: counts.Warn, vocab.Deny: counts.Deny}
	return slices.ContainsFunc(order[at:], func(severity vocab.Severity) bool { return held[severity] > 0 })
}
