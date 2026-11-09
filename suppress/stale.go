package suppress

import (
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
// record there: the codes the site names and, where a declaration it bound
// would report another code, those codes, so the maintainer corrects the code
// rather than hunting for it (suppression.md "Staleness").
func StaleMessage(site []Record, would []Finding) string {
	named := make([]vocab.Code, 0, len(site))
	for i := range site {
		named = append(named, site[i].Code)
	}
	slices.Sort(named)
	named = slices.Compact(named)
	message := site[0].Mechanism.Written() + " for " + spellCodes(named) + " matches no current finding"
	var beside []vocab.Code
	for i := range would {
		found := &would[i]
		if found.Decl != "" && !slices.Contains(named, found.Code) &&
			slices.ContainsFunc(site, func(r Record) bool { return r.Bound == found.Decl }) {
			beside = append(beside, found.Code)
		}
	}
	if len(beside) > 0 {
		slices.Sort(beside)
		message += ", and its declaration reports " + spellCodes(slices.Compact(beside))
	}
	return message
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
