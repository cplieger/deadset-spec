package suppress

import (
	"cmp"
	"slices"
	"strings"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// Written is the mechanism as a message names one suppression of it.
func (m Mechanism) Written() string {
	switch m {
	case Inline:
		return "inline directive"
	case Ignore:
		return "ignore entry"
	case Baseline:
		return "baseline row"
	}
	return string(m)
}

// StaleSites is the records [StaleRecords] returns grouped by the mechanism and
// the site they were written at, in reading order: one stale-suppression
// finding each, since a maintainer edits one line.
func StaleSites(stale []Record) [][]Record {
	var sites [][]Record
	at := map[string]int{}
	for i := range stale {
		key := string(stale[i].Mechanism) + "\x00" + stale[i].Site.key()
		j, seen := at[key]
		if !seen {
			j = len(sites)
			at[key] = j
			sites = append(sites, nil)
		}
		sites[j] = append(sites[j], stale[i])
	}
	return sites
}

// StaleMessage is the message of the stale-suppression finding of the records
// one site holds, against would, the findings the run would produce were no
// record there, which ledger decided: the codes the site names, then [Beside].
func StaleMessage(site []Record, would []Finding, ledger *Ledger) string {
	return site[0].Mechanism.Written() + " for " + spellCodes(namedAt(site)) + " matches no current finding" + Beside(site, would, ledger)
}

// Beside is how the message of the stale-suppression finding of the records one
// site holds ends: every other code a finding in would about a declaration one
// of them bound carries, with that finding's confidence and what withholds it,
// the record ledger says matched it among them, so the maintainer corrects the
// code rather than hunting for it, and empty where there is none
// (suppression.md "Staleness").
func Beside(site []Record, would []Finding, ledger *Ledger) string {
	named := namedAt(site)
	var entries []beside
	for i := range would {
		found := &would[i]
		if found.Decl != "" && !slices.Contains(named, found.Code) &&
			slices.ContainsFunc(site, func(r Record) bool { return r.Bound == found.Decl }) {
			entries = append(entries, beside{code: found.Code, confidence: found.Confidence, withheld: ledger.withholding(found)})
		}
	}
	if len(entries) == 0 {
		return ""
	}
	slices.SortFunc(entries, compareBeside)
	entries = slices.Compact(entries)
	spelled := make([]string, len(entries))
	for i, e := range entries {
		spelled[i] = string(e.code) + " at " + string(e.confidence)
		if e.withheld != "" {
			spelled[i] += ", withheld by " + string(e.withheld)
		}
	}
	if n := len(spelled); n > 1 {
		spelled[n-1] = "and " + spelled[n-1]
	}
	return ", and its declaration reports " + strings.Join(spelled, "; ")
}

// beside is one entry [Beside] names.
type beside struct {
	code       vocab.Code
	confidence vocab.Class
	withheld   Withholding
}

// withholdings is every withholding a finding can carry, nothing first.
var withholdings = []Withholding{
	"", WithheldByTypeError, WithheldBySeverity, WithheldByMinConfidence,
	WithheldByInline, WithheldByIgnore, WithheldByBaseline,
}

// compareBeside orders entries by code, then from the most to the least certain,
// then by withholding.
func compareBeside(a, b beside) int {
	return cmp.Or(
		cmp.Compare(a.code, b.code),
		cmp.Compare(slices.Index(vocab.Classes(), a.confidence), slices.Index(vocab.Classes(), b.confidence)),
		cmp.Compare(slices.Index(withholdings, a.withheld), slices.Index(withholdings, b.withheld)),
	)
}

// namedAt is every code the records of one site name, in order and each once.
func namedAt(site []Record) []vocab.Code {
	named := make([]vocab.Code, 0, len(site))
	for i := range site {
		named = append(named, site[i].Code)
	}
	slices.Sort(named)
	return slices.Compact(named)
}

// spellCodes is codes as one message lists them: one on its own, and several
// separated by commas with the last joined by "and".
func spellCodes(codes []vocab.Code) string {
	spelled := make([]string, len(codes))
	for i, code := range codes {
		spelled[i] = string(code)
	}
	if n := len(spelled); n > 1 {
		return strings.Join(spelled[:n-1], ", ") + " and " + spelled[n-1]
	}
	return strings.Join(spelled, "")
}
