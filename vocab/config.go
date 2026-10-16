package vocab

import (
	"slices"
	"sync"
)

// Key is one key of the closed configuration key list, by its dotted path.
type Key string

// Role is what one key of the configuration is.
type Role string

// The roles a key plays.
const (
	// RoleSection is a key whose own keys a higher-ranked source replaces one
	// by one.
	RoleSection Role = "section"
	// RoleSetting is a key a higher-ranked source replaces whole.
	RoleSetting Role = "setting"
	// RoleKeyed is a section whose keys a pattern admits, each one setting.
	RoleKeyed Role = "keyed"
	// RoleIgnored is a key resolution reads past.
	RoleIgnored Role = "ignored"
)

// ConfigKey is one key of the closed configuration key list, its members those
// of its contract/settings.json row. Default and Value are JSON text, "" where
// the row has none; Value is the JSON Schema each value is an instance of, for a
// keyed or an ignored key each member's value, whose name matches KeyPattern.
type ConfigKey struct {
	Key         Key
	Role        Role
	Owner       string
	Default     string
	Value       string
	KeyPattern  string
	Option      string
	Vocabulary  string
	ReadBy      []Language
	Delivered   []Language
	Values      []string
	PerLanguage []Language
	Required    bool
}

// ReadIn reports whether the analysis of one language reads the key.
func (k *ConfigKey) ReadIn(language Language) bool {
	return slices.Contains(k.ReadBy, language)
}

// DeliveredTo reports whether an orchestrator writes the key into the document
// it hands the analyzer of one language.
func (k *ConfigKey) DeliveredTo(language Language) bool {
	return slices.Contains(k.Delivered, language)
}

func (k *ConfigKey) clone() ConfigKey {
	held := *k
	held.ReadBy = slices.Clone(k.ReadBy)
	held.Delivered = slices.Clone(k.Delivered)
	held.Values = slices.Clone(k.Values)
	held.PerLanguage = slices.Clone(k.PerLanguage)
	return held
}

// ConfigKeys returns every key of the closed key list, sections included, in
// the schema's key order, the order a resolved configuration prints in.
func ConfigKeys() []ConfigKey {
	held := make([]ConfigKey, len(configKeys))
	for i := range configKeys {
		held[i] = configKeys[i].clone()
	}
	return held
}

var byKey = sync.OnceValue(func() map[Key]*ConfigKey {
	index := make(map[Key]*ConfigKey, len(configKeys))
	for i := range configKeys {
		index[configKeys[i].Key] = &configKeys[i]
	}
	return index
})

// ConfigKeyOf returns the row of one key, and false for a key the list does not
// declare.
func ConfigKeyOf(key Key) (ConfigKey, bool) {
	row, declared := byKey()[key]
	if !declared {
		return ConfigKey{}, false
	}
	return row.clone(), true
}

// LanguageRow is one language of the suite, its members those of its
// contract/languages.json row; IdentityFile and IdentityField are its
// identity_source.
type LanguageRow struct {
	Language             Language
	Name                 string
	Kind                 string
	Scheme               string
	Runner               string
	Analyzer             string
	IdentityFile         string
	IdentityField        string
	Markers              []string
	Extensions           []string
	Companions           []string
	WorkspaceFiles       []string
	ProgramWithoutMarker bool
	SkipsUnderscored     bool
	TakesTargetKind      bool
}

func (l *LanguageRow) clone() LanguageRow {
	held := *l
	held.Markers = slices.Clone(l.Markers)
	held.Extensions = slices.Clone(l.Extensions)
	held.Companions = slices.Clone(l.Companions)
	held.WorkspaceFiles = slices.Clone(l.WorkspaceFiles)
	return held
}

// LanguageRows returns every language's row, in the registry's order.
func LanguageRows() []LanguageRow {
	held := make([]LanguageRow, len(languageRows))
	for i := range languageRows {
		held[i] = languageRows[i].clone()
	}
	return held
}

// LanguageRowOf returns the row of one language, and false for a language the
// registry does not hold.
func LanguageRowOf(language Language) (LanguageRow, bool) {
	for i := range languageRows {
		if languageRows[i].Language == language {
			return languageRows[i].clone(), true
		}
	}
	return LanguageRow{}, false
}

// SkippedDirectories returns the directory names under which no file puts a
// language in scope.
func SkippedDirectories() []string { return slices.Clone(skippedDirectories) }
