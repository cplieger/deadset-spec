package suppress

import (
	"strconv"

	"github.com/cplieger/deadset-spec/v8/vocab"
)

// Finding is one finding a run would produce were no record there, as far as
// the ledger reads it.
type Finding struct {
	Code       vocab.Code
	SymbolKind string
	Ref        string

	// Holder is the reference of the declaration holding the part the finding
	// is about, where that part has a reference of its own, and empty otherwise.
	Holder string

	Path   string
	Line   int
	Column int
}

// Key is the finding's position, which a bound record names its declaration by.
func (f *Finding) Key() string {
	return f.Path + ":" + strconv.Itoa(f.Line) + ":" + strconv.Itoa(f.Column)
}

// claim is the finding's identity: its code and position.
func (f *Finding) claim() string { return string(f.Code) + "\x00" + f.Key() }

// Dials are the configuration's two dials as the ledger reads them: each
// decides whether a run reports a finding rather than whether one exists.
type Dials struct {
	// Allows reports whether the configuration sets one code's severity to
	// allow.
	Allows func(code vocab.Code) bool

	// Withholds reports whether the configuration withholds one finding, by its
	// severity, its confidence or its component's root.
	Withholds func(finding *Finding) bool
}

// Verdict is what a record did.
type Verdict string

// The verdicts.
const (
	InEffect Verdict = "in-effect"
	Dormant  Verdict = "dormant"
	Stale    Verdict = "stale"
)

// Ledger is what every record did, in reading order, and which findings the
// records withhold.
type Ledger struct {
	taken    map[string]bool
	Verdicts []Verdict
}

// Withheld reports whether a record withholds the finding.
func (l *Ledger) Withheld(finding *Finding) bool { return l.taken[finding.claim()] }

// Decide decides every record, in reading order (inline directives, then the
// ignore file's entries, then the baseline's rows, each in document order),
// against would, the findings the run would produce were no record there. The
// first record naming a finding claims it, so a later one naming it is stale; a
// part-kind record claims every finding of its code about a part of its
// declaration. A record is dormant when the dials withhold every finding it
// claims, or when it is bound, claims none, and its code's severity is allow.
func Decide(records []Record, would []Finding, dials *Dials) *Ledger {
	ledger := &Ledger{taken: map[string]bool{}, Verdicts: make([]Verdict, 0, len(records))}
	for i := range records {
		ledger.Verdicts = append(ledger.Verdicts, ledger.decide(&records[i], would, dials))
	}
	return ledger
}

// decide claims the findings one record names that no earlier record claimed,
// and is the record's verdict.
func (l *Ledger) decide(record *Record, would []Finding, dials *Dials) Verdict {
	claimed := l.claimable(record, would)
	if len(claimed) == 0 {
		if record.Bound != "" && dials.Allows(record.Code) {
			return Dormant
		}
		return Stale
	}
	withheld := true
	for _, found := range claimed {
		l.taken[found.claim()] = true
		withheld = withheld && dials.Withholds(found)
	}
	if withheld {
		return Dormant
	}
	return InEffect
}

// claimable is the first unclaimed finding the record names, or every one for
// a part-kind record.
func (l *Ledger) claimable(record *Record, would []Finding) []*Finding {
	var claimed []*Finding
	for j := range would {
		if found := &would[j]; !l.taken[found.claim()] && pairs(record, found) {
			claimed = append(claimed, found)
			if record.Code.Subject() != vocab.SubjectPart {
				break
			}
		}
	}
	return claimed
}

// pairs reports whether a record names a finding: it bound the declaration at
// the finding's position, or it names the finding's reference, or the reference
// of the declaration holding a part that has one of its own, and the finding's
// file, each exactly. A dependency row is reached only the second way and never
// by a directive, and no record reaches any other row of a document.
func pairs(record *Record, found *Finding) bool {
	if record.Code != found.Code {
		return false
	}
	subject, _ := vocab.SubjectOf(found.SymbolKind)
	switch subject {
	case vocab.SubjectDependency:
		return record.Mechanism != Inline && record.Symbol == found.Ref && record.Path == found.Path
	case vocab.SubjectFile, vocab.SubjectRecord:
		return false
	}
	if record.Bound != "" && record.Bound == found.Key() {
		return true
	}
	named := record.Symbol == found.Ref || (found.Holder != "" && record.Symbol == found.Holder)
	return named && record.Path == found.Path
}

// StaleRecords is one record per stale suppression, in reading order: a
// directive's code, an entry or a row is stale when every record it resolved to
// is stale, so one naming two declarations with one of them in effect is not.
func StaleRecords(records []Record, ledger *Ledger) []Record {
	live := map[string]bool{}
	for i := range records {
		if ledger.Verdicts[i] != Stale {
			live[records[i].suppression()] = true
		}
	}
	var stale []Record
	reported := map[string]bool{}
	for i := range records {
		key := records[i].suppression()
		if !live[key] && !reported[key] {
			reported[key] = true
			stale = append(stale, records[i])
		}
	}
	return stale
}

// suppression is the identity of what a record was written as: its mechanism,
// its site and its code.
func (r *Record) suppression() string {
	return string(r.Mechanism) + "\x00" + r.Site.key() + "\x00" + string(r.Code)
}

// Counts are the two suppression totals a report carries: the records in
// effect, and the directives, entries and rows carrying a reason, each once
// whatever its codes or the declarations it bound.
func Counts(records []Record, ledger *Ledger) (inEffect, reasonsRecorded int) {
	sites := map[string]bool{}
	for i := range records {
		sites[string(records[i].Mechanism)+"\x00"+records[i].Site.key()] = true
		if ledger.Verdicts[i] == InEffect {
			inEffect++
		}
	}
	return inEffect, len(sites)
}
