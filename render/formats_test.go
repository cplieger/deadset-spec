package render

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec/v8/report"
	"github.com/cplieger/deadset-spec/v8/vocab"
)

// A pending finding outranks a stale suppression, which yields the findings
// code whatever the failing severity, as a finding at or above it does.
func TestVerdict(t *testing.T) {
	t.Parallel()

	for name, tt := range map[string]struct {
		totals report.Totals
		failOn vocab.Severity
		want   int
	}{
		"nothing":                      {failOn: vocab.Deny, want: vocab.ExitClean},
		"a pending finding":            {totals: report.Totals{Pending: 1, StaleSuppressions: 1}, failOn: vocab.Deny, want: vocab.ExitPending},
		"a stale suppression":          {totals: report.Totals{StaleSuppressions: 1}, failOn: vocab.Deny, want: vocab.ExitFindings},
		"a warn under deny":            {totals: report.Totals{BySeverity: report.BySeverity{Warn: 1}}, failOn: vocab.Deny, want: vocab.ExitClean},
		"a warn under warn":            {totals: report.Totals{BySeverity: report.BySeverity{Warn: 1}}, failOn: vocab.Warn, want: vocab.ExitFindings},
		"a deny under warn":            {totals: report.Totals{BySeverity: report.BySeverity{Deny: 1}}, failOn: vocab.Warn, want: vocab.ExitFindings},
		"an allow under allow":         {totals: report.Totals{BySeverity: report.BySeverity{Allow: 1}}, failOn: vocab.Allow, want: vocab.ExitFindings},
		"an omitted deny still counts": {totals: report.Totals{BySeverity: report.BySeverity{Deny: 1}, Omitted: 1}, failOn: vocab.Deny, want: vocab.ExitFindings},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := Verdict(&tt.totals, tt.failOn); got != tt.want {
				t.Errorf("Verdict(%+v, %s) = %d, want %d", tt.totals, tt.failOn, got, tt.want)
			}
		})
	}
}

// A size order renders findings by the lines their deletion removes, and the
// JSON document stays in canonical order.
func TestRenderOrdersRenderingsBySizeAndTheDocumentCanonically(t *testing.T) {
	t.Parallel()

	r := &report.Report{Findings: []report.Finding{
		{Code: "DS1001", Position: report.Position{Path: "a.go", Line: 1, Column: 1}, Component: report.Component{DeletableLines: 1}},
		{Code: "DS1001", Position: report.Position{Path: "b.go", Line: 1, Column: 1}, Component: report.Component{DeletableLines: 9}},
	}}
	var text bytes.Buffer
	if err := Render(&text, vocab.FormatText, r, &Options{BySize: true}); err != nil {
		t.Fatalf("Render(text, by size) = %v", err)
	}
	if lines := strings.Split(text.String(), "\n"); !strings.HasPrefix(lines[0], "b.go:") || !strings.HasPrefix(lines[1], "a.go:") {
		t.Errorf("Render(text, by size) = %q, want b.go's finding first", text.String())
	}
	if r.Findings[0].Position.Path != "a.go" {
		t.Errorf("Render(text, by size) reordered the report's findings, want them left canonical")
	}
}

// A rendering asked for without what it reads is refused before writing.
func TestRenderRefusesARenderingWithoutWhatItReads(t *testing.T) {
	t.Parallel()

	for _, format := range []vocab.Format{vocab.FormatSARIF, vocab.FormatTemplate} {
		var out bytes.Buffer
		if err := Render(&out, format, &report.Report{}, &Options{}); !errors.Is(err, errOptions) || out.Len() != 0 {
			t.Errorf("Render(%s, no options) = %v and wrote %d bytes, want %v and nothing written", format, err, out.Len(), errOptions)
		}
	}
}

// A template refused for its definitions names the first one the text writes,
// on every parse, whatever order the parser holds them in.
func TestParseTemplateNamesTheFirstDefinition(t *testing.T) {
	t.Parallel()

	const text = `{{define "alpha"}}a{{end}}{{define "beta"}}b{{end}}{{define "gamma"}}c{{end}}{{define "delta"}}d{{end}}x`
	for range 20 {
		_, err := ParseTemplate(text)
		if err == nil || !strings.Contains(err.Error(), `"alpha"`) {
			t.Fatalf("ParseTemplate(four definitions) = %v, want a refusal naming \"alpha\"", err)
		}
	}
}
