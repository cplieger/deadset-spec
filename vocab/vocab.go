// Package vocab is the Contract's closed vocabularies as Go values: the issue
// kinds and their families, the symbol kinds and the subject each names, the
// exemption classes, the evidence that lowers a finding's class, the exit codes,
// the setup-failure classes, the versions, the language registry and the keys of
// the configuration. A product compiles them in rather than reading the
// documents at run time. vocab_gen.go and config_gen.go are written by
// deadset-gen from contract/*.json, authoring/config.schema.json and
// corpus/corpus.json, and a freshness test fails when a document moves and
// nobody regenerated.
package vocab

import (
	"slices"
	"strings"
	"sync"
)

// Code is an issue-kind code: the prefix, two digits naming the family and two
// naming the kind within it.
type Code string

// Language is a language the Contract's vocabularies name.
type Language string

// Subject is what a finding of a kind is about.
type Subject string

// Class is a reachability class, ordered from the most to the least certain.
type Class string

// Severity is the severity a configuration may give a kind.
type Severity string

// Fixability is what a mechanical edit may do with a finding.
type Fixability string

// Format is one rendering of a report the configuration's reporters.formats
// names.
type Format string

// Presence is how a kind carries one member of a finding's details: always,
// sometimes, or only in some languages.
type Presence struct {
	Languages []Language
	Required  bool
	Optional  bool
}

// Detail is one member of a finding's details a kind declares.
type Detail struct {
	Member   string
	Presence Presence
}

// Kind is one live issue kind, its members those of its kinds.json row.
type Kind struct {
	Overlap             map[Language][]string
	Precondition        string
	Subject             Subject
	Code                Code
	Rule                string
	Name                string
	DefaultSeverity     Severity
	MaxClass            Class
	Fixability          Fixability
	Languages           []Language
	Details             []Detail
	DerivedFrom         []Code
	FindingFixabilities []Fixability
	HoldsLive           bool
	DefaultEnabled      bool
	Fixed               bool
}

// AppliesTo reports whether the kind applies to one language.
func (k *Kind) AppliesTo(language Language) bool {
	return slices.Contains(k.Languages, language)
}

// Detail returns how the kind carries one member of a finding's details, and
// false when it declares none.
func (k *Kind) Detail(member string) (Presence, bool) {
	for _, d := range k.Details {
		if d.Member == member {
			return d.Presence, true
		}
	}
	return Presence{}, false
}

func (k *Kind) clone() Kind {
	held := *k
	held.Languages = slices.Clone(k.Languages)
	held.Details = slices.Clone(k.Details)
	for i := range held.Details {
		held.Details[i].Presence.Languages = slices.Clone(held.Details[i].Presence.Languages)
	}
	held.FindingFixabilities = slices.Clone(k.FindingFixabilities)
	held.DerivedFrom = slices.Clone(k.DerivedFrom)
	if k.Overlap != nil {
		held.Overlap = make(map[Language][]string, len(k.Overlap))
		for language, rules := range k.Overlap {
			held.Overlap[language] = slices.Clone(rules)
		}
	}
	return held
}

// Kinds returns every live kind, in the vocabulary's order. The rows share
// nothing with the vocabulary, so a caller may keep and change them.
func Kinds() []Kind {
	held := make([]Kind, len(kinds))
	for i := range kinds {
		held[i] = kinds[i].clone()
	}
	return held
}

var byCode = sync.OnceValue(func() map[Code]*Kind {
	index := make(map[Code]*Kind, len(kinds))
	for i := range kinds {
		index[kinds[i].Code] = &kinds[i]
	}
	return index
})

// KindOf returns the live kind one code names, and false for a retired code or
// one the vocabulary never held.
func KindOf(code Code) (Kind, bool) {
	kind, live := byCode()[code]
	if !live {
		return Kind{}, false
	}
	return kind.clone(), true
}

// Live reports whether a live kind carries the code.
func (c Code) Live() bool {
	_, live := byCode()[c]
	return live
}

// Subject returns what a finding under the code is about, and "" for a code no
// live kind carries.
func (c Code) Subject() Subject {
	if kind, live := byCode()[c]; live {
		return kind.Subject
	}
	return ""
}

// Family is one range of a hundred codes, its members those of its kinds.json
// range.
type Family struct {
	Key      string
	Name     string
	Title    string
	Question string
	Status   string
}

// Families returns every family, in the vocabulary's order.
func Families() []Family {
	return slices.Clone(families)
}

// FamilyOf returns the family key one code belongs to: its first
// [FamilyKeyLength] characters.
func FamilyOf(code Code) string {
	if len(code) < FamilyKeyLength {
		return string(code)
	}
	return string(code[:FamilyKeyLength])
}

// CodesOf returns every live code one severity key names, in the vocabulary's
// order: the code itself, or each code of the family a family key names.
func CodesOf(key string) []Code {
	var named []Code
	for i := range kinds {
		code := kinds[i].Code
		if string(code) == key || (len(key) == FamilyKeyLength && strings.HasPrefix(string(code), key)) {
			named = append(named, code)
		}
	}
	return named
}

// SubjectOf returns the subject a finding about one symbol kind is about, and
// false for a symbol kind the vocabulary does not declare.
func SubjectOf(symbolKind string) (Subject, bool) {
	subject, ok := symbolKinds[symbolKind]
	return subject, ok
}

// Exemption is the name of an exemption class.
type Exemption string

// ExemptionClass is one class of exemption, its members those of its
// exemptions.json row.
type ExemptionClass struct {
	RetainsKinds map[Language][]string
	Name         Exemption
	Confidence   Class
	Languages    []Language
}

// AppliesTo reports whether the class runs on one language.
func (c *ExemptionClass) AppliesTo(language Language) bool {
	return slices.Contains(c.Languages, language)
}

// ExemptionClasses returns every exemption class, in the vocabulary's order,
// the classes of every language included.
func ExemptionClasses() []ExemptionClass {
	held := make([]ExemptionClass, len(exemptionClasses))
	for i, c := range exemptionClasses {
		c.Languages = slices.Clone(c.Languages)
		retains := make(map[Language][]string, len(c.RetainsKinds))
		for language, symbolKinds := range c.RetainsKinds {
			retains[language] = slices.Clone(symbolKinds)
		}
		c.RetainsKinds = retains
		held[i] = c
	}
	return held
}

// Evidence is one row of the confidence evidence table: a kind of evidence that
// lowers a finding's class in some languages under some codes.
type Evidence struct {
	Name      string
	Rule      string
	Class     Class
	Languages []Language
	Codes     []Code
}

// Lowers reports whether the evidence lowers the class of a finding of one
// code in one language.
func (e *Evidence) Lowers(language Language, code Code) bool {
	return slices.Contains(e.Languages, language) && slices.Contains(e.Codes, code)
}

// EvidenceFor returns the row of the evidence table one name gives in one
// language, and false where the table holds none.
func EvidenceFor(name string, language Language) (Evidence, bool) {
	for _, e := range evidence {
		if e.Name == name && slices.Contains(e.Languages, language) {
			e.Languages = slices.Clone(e.Languages)
			e.Codes = slices.Clone(e.Codes)
			return e, true
		}
	}
	return Evidence{}, false
}

// SetupClass is one class of setup failure: what the project lacks that the
// analysis needs.
type SetupClass struct {
	Name      string
	Languages []Language
}

// SetupClasses returns every setup-failure class, in the vocabulary's order.
func SetupClasses() []SetupClass {
	held := slices.Clone(setupClasses)
	for i := range held {
		held[i].Languages = slices.Clone(held[i].Languages)
	}
	return held
}

// SchemaVersions returns every report schema version the Contract admits, in
// ascending order; [SchemaVersion] is the last.
func SchemaVersions() []string {
	return slices.Clone(schemaVersions)
}

// Languages returns every language the vocabularies name, in their order.
func Languages() []Language { return slices.Clone(languages) }

// Classes returns the reachability classes, from the most to the least certain.
func Classes() []Class { return slices.Clone(classValues) }

// Severities returns the severities a configuration may give a kind.
func Severities() []Severity { return slices.Clone(severityValues) }

// Fixabilities returns the fixabilities a finding may carry.
func Fixabilities() []Fixability { return slices.Clone(fixabilityValues) }

// Formats returns the renderings reporters.formats may name.
func Formats() []Format { return slices.Clone(formatValues) }

// SymbolKinds returns every symbol kind the vocabulary declares, in its order.
func SymbolKinds() []string { return slices.Clone(symbolKindOrder) }
