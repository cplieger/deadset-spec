package vocab

import (
	"path"
	"slices"
	"strings"
	"sync"
)

// Key is one key of the closed configuration key list, by its dotted path.
type Key string

// Choice is one value of a setting's closed choice that no other vocabulary
// owns.
type Choice string

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

// IdentitySource is the file whose declared name identifies a module of a
// language, and the member of it that declares the name.
type IdentitySource struct {
	File  string
	Field string
}

// LanguageRow is one language of the suite, its members those of its
// contract/languages.json row. Markers, Companions and SkippedDirectories are
// path.Match patterns over a name alone.
type LanguageRow struct {
	IdentitySource       IdentitySource
	Language             Language
	Name                 string
	Kind                 string
	Scheme               string
	Runner               string
	Analyzer             string
	Markers              []string
	Extensions           []string
	Companions           []string
	SkippedDirectories   []string
	WorkspaceFiles       []string
	ProgramWithoutMarker bool
	TakesTargetKind      bool
}

func (l *LanguageRow) clone() LanguageRow {
	held := *l
	held.Markers = slices.Clone(l.Markers)
	held.Extensions = slices.Clone(l.Extensions)
	held.Companions = slices.Clone(l.Companions)
	held.SkippedDirectories = slices.Clone(l.SkippedDirectories)
	held.WorkspaceFiles = slices.Clone(l.WorkspaceFiles)
	return held
}

// LanguageRows returns every language's row, in the registry's order, which is
// the order a module's identity source is chosen in.
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

// Skips reports whether the language's discovery leaves a directory of that
// name unread.
func (l *LanguageRow) Skips(dir string) bool { return matchesAny(l.SkippedDirectories, dir) }

// Marks reports whether a file of that name puts the language in scope alone.
func (l *LanguageRow) Marks(file string) bool { return matchesAny(l.Markers, file) }

// Accompanies reports whether a file of that name is one of the language's
// companions.
func (l *LanguageRow) Accompanies(file string) bool { return matchesAny(l.Companions, file) }

// Sources reports whether a file of that name is a source file of the
// language, by its extension.
func (l *LanguageRow) Sources(file string) bool {
	return slices.Contains(l.Extensions, path.Ext(file))
}

// matchesAny reports whether a name matches one of the patterns; a pattern
// path.Match cannot parse matches nothing.
func matchesAny(patterns []string, name string) bool {
	return slices.ContainsFunc(patterns, func(pattern string) bool {
		matched, err := path.Match(pattern, name)
		return err == nil && matched
	})
}

// GeneratorRow is one generator of contract/generators.json. VersionMin and
// VersionMax are empty where the row names no bound.
type GeneratorRow struct {
	ID            string
	Plugin        string
	Language      Language
	Header        string
	RuntimeImport string
	Cell          string
	VersionMin    string
	VersionMax    string
	Major         int
}

// GeneratorRows returns every generator row in document order, the order a
// generated file is matched in.
func GeneratorRows() []GeneratorRow { return slices.Clone(generatorRows) }

// ProtoPlugins returns every name proto.plugins admits.
func ProtoPlugins() []string { return slices.Clone(protoPlugins) }

// HTTPExposurePlugins returns the plugins whose presence in a generation
// configuration exposes a service over HTTP.
func HTTPExposurePlugins() []string { return slices.Clone(httpExposurePlugins) }

// ReadsRoot reports whether the analysis of languages reads the configured root
// string root, as merge.md's DS9004 rule states: one whose text before its first
// :// is absent, holds a wildcard, or names one of languages. Every analyzer is
// handed every configured string and answers only those it reads.
func ReadsRoot(languages []Language, root string) bool {
	prefix, _, found := strings.Cut(root, "://")
	return !found || strings.ContainsAny(prefix, "*?") || slices.Contains(languages, Language(prefix))
}
