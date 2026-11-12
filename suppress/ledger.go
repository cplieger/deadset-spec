package suppress

import (
	"slices"

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

	Path string

	// File is the file the finding's subject is written in, whatever path a
	// line directive renders it under, and empty for a row of a document.
	File string

	// Decl is the identity of the declaration the finding is about, or of the
	// one holding the part it is about, as a binder's Lookup spells it, and
	// empty where the finding is about no declaration of the inventory.
	Decl string

	// ID is the finding's identity, distinct among the findings of its code
	// even where two of them render at one position.
	ID string

	// Confidence is the confidence the finding carries, and Withheld the hold or
	// the dial that keeps it out of the report, empty where neither does; a record
	// that matches it is the ledger's to say.
	Confidence vocab.Class
	Withheld   Withholding

	// Named is every declaration whose reference the finding carries where that
	// is not Decl's: an assertion's interface, once per platform alternative the
	// configurations compiling the assertion name.
	Named []string
}

// Withholding is what keeps a finding the run computed out of its report, as a
// stale record's message names it (suppression.md "Staleness").
type Withholding string

// The withholdings, in the order a finding two of them withhold is named by the
// first: a hold, the dials, then the record that matches it.
const (
	WithheldByTypeError     Withholding = "a type error"
	WithheldBySeverity      Withholding = "severity allow"
	WithheldByMinConfidence Withholding = "analysis.min_confidence"
	WithheldByInline        Withholding = "an inline directive"
	WithheldByIgnore        Withholding = "an ignore entry"
	WithheldByBaseline      Withholding = "a baseline row"
)

// withheldBy is the withholding a record of mechanism m is.
var withheldBy = map[Mechanism]Withholding{Inline: WithheldByInline, Ignore: WithheldByIgnore, Baseline: WithheldByBaseline}

// claim is the finding's identity across codes.
func (f *Finding) claim() string { return string(f.Code) + "\x00" + f.ID }

// Dials is the severity dial as the ledger reads it for a record that claims no
// finding; what withholds a finding it claims is the finding's Withheld.
type Dials struct {
	// Allows reports whether the configuration sets one code's severity to
	// allow.
	Allows func(code vocab.Code) bool
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
	taken    map[string]Mechanism // the mechanism of the record that matched each claimed finding
	Verdicts []Verdict
}

// Withheld reports whether a record withholds the finding.
func (l *Ledger) Withheld(finding *Finding) bool {
	_, taken := l.taken[finding.claim()]
	return taken
}

// withholding is what keeps the finding out of the report: its own Withheld,
// else the record that matched it, empty where nothing does.
func (l *Ledger) withholding(finding *Finding) Withholding {
	if finding.Withheld != "" {
		return finding.Withheld
	}
	return withheldBy[l.taken[finding.claim()]]
}

// Decide decides every record, in reading order (inline directives, then the
// ignore file's entries, then the baseline's rows, each in document order),
// against would, the findings the run would produce were no record there. A
// record claims every finding it names that no earlier record claimed, so a
// later one naming only claimed findings is stale. A record is dormant when
// something withholds every finding it claims, or when it is bound, claims none,
// and its code's severity is allow.
func Decide(records []Record, would []Finding, dials *Dials) *Ledger {
	ledger := &Ledger{taken: map[string]Mechanism{}, Verdicts: make([]Verdict, 0, len(records))}
	bound := map[string][]string{}
	for i := range records {
		if records[i].Bound != "" {
			written := records[i].suppression()
			bound[written] = append(bound[written], records[i].Bound)
		}
	}
	for i := range records {
		record := &records[i]
		ledger.Verdicts = append(ledger.Verdicts, ledger.decide(record, bound[record.suppression()], would, dials))
	}
	return ledger
}

// decide claims the findings one record names that no earlier record claimed,
// and is the record's verdict. bound is every declaration the record's entry or
// row resolved to, its own included.
func (l *Ledger) decide(record *Record, bound []string, would []Finding, dials *Dials) Verdict {
	var claimed []*Finding
	for j := range would {
		if found := &would[j]; !l.Withheld(found) && pairs(record, bound, found) {
			claimed = append(claimed, found)
		}
	}
	if len(claimed) == 0 {
		if record.Bound != "" && dials.Allows(record.Code) {
			return Dormant
		}
		return Stale
	}
	withheld := true
	for _, found := range claimed {
		l.taken[found.claim()] = record.Mechanism
		withheld = withheld && found.Withheld != ""
	}
	if withheld {
		return Dormant
	}
	return InEffect
}

// pairs reports whether a record names a finding under a code its mechanism
// can name. A directive names what is written in its own file: a finding about
// the declaration it bound or a part of it, or one carrying that declaration's
// reference. An entry or a row names what a finding reports, its reference, or
// a part's holder's, and its path; where it resolved to several declarations,
// a finding about one of them is that declaration's record's alone.
func pairs(record *Record, bound []string, found *Finding) bool {
	if record.Code != found.Code || !Names(record.Code.Subject(), record.Mechanism) {
		return false
	}
	owners := found.owners()
	if record.Mechanism == Inline {
		return record.Bound != "" && slices.Contains(owners, record.Bound) && record.Site.File == found.File
	}
	if record.Path != found.Path || record.Symbol != found.Ref && (found.Holder == "" || record.Symbol != found.Holder) {
		return false
	}
	ownedHere := slices.ContainsFunc(owners, func(owner string) bool { return slices.Contains(bound, owner) })
	return !ownedHere || slices.Contains(owners, record.Bound)
}

// owners is every declaration a finding is about or carries the reference of.
func (f *Finding) owners() []string {
	if f.Decl == "" {
		return f.Named
	}
	return append([]string{f.Decl}, f.Named...)
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
