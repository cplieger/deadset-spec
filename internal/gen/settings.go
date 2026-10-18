package gen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/format"
	"io/fs"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	languagesPath      = "contract/languages.json"
	cliPath            = "contract/cli.json"
	exemptionsPath     = "contract/exemptions.json"
	settingsPath       = "contract/settings.json"
	configVocabPath    = "vocab/config_gen.go"
	contractVersionKey = "contract_version"
	namePlaceholder    = "{name}"
	ownerSuffix        = "-analyzer"
)

// The roles a key of the closed key list plays, as contract/settings.json
// spells them.
const (
	roleSection = "section"
	roleSetting = "setting"
	roleKeyed   = "keyed"
	roleIgnored = "ignored"
)

type languageDoc struct {
	IdentitySource struct {
		File  string `json:"file"`
		Field string `json:"field"`
	} `json:"identity_source"`
	Language             string   `json:"language"`
	Name                 string   `json:"name"`
	Kind                 string   `json:"kind"`
	Scheme               string   `json:"scheme"`
	Runner               string   `json:"runner"`
	Analyzer             string   `json:"analyzer"`
	Markers              []string `json:"markers"`
	Extensions           []string `json:"extensions"`
	Companions           []string `json:"companions"`
	SkippedDirectories   []string `json:"skipped_directories"`
	WorkspaceFiles       []string `json:"workspace_files"`
	ProgramWithoutMarker bool     `json:"program_without_marker"`
	TakesTargetKind      bool     `json:"takes_target_kind"`
}

// flags is a row's boolean columns by name, the ones an x-per-language selects
// languages by.
func (l *languageDoc) flags() map[string]bool {
	return map[string]bool{
		"program_without_marker": l.ProgramWithoutMarker,
		"takes_target_kind":      l.TakesTargetKind,
	}
}

type languagesDoc struct {
	Description string        `json:"description"`
	Languages   []languageDoc `json:"languages"`
}

// configContext is every Contract document the published schemas' generated
// members are derived from beside their authoring sources.
type configContext struct {
	options     map[string]string // a setting's path to the option that supplies it
	contract    contractDoc
	generators  generatorsDoc
	exemptions  exemptionsDoc
	languages   languagesDoc
	kinds       kindsDoc
	symbolKinds []string
}

func loadConfigContext(fsys fs.FS) (*configContext, error) {
	c := &configContext{options: map[string]string{}}
	var cli struct {
		Options []struct {
			Option  string `json:"option"`
			Setting string `json:"setting"`
		} `json:"options"`
	}
	for name, into := range map[string]any{
		kindsPath: &c.kinds, exemptionsPath: &c.exemptions, "contract/contract.json": &c.contract, cliPath: &cli,
		generatorsPath: &c.generators,
	} {
		if err := decodeFile(fsys, name, into); err != nil {
			return nil, err
		}
	}
	// Strict, so a column languages.json gains fails generation until the
	// generated registry carries it.
	data, err := fs.ReadFile(fsys, languagesPath)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&c.languages); err != nil {
		return nil, fmt.Errorf("%s: %w", languagesPath, err)
	}
	symbolKinds, err := parse(c.kinds.SymbolKinds)
	if err != nil {
		return nil, fmt.Errorf("%s: symbol_kinds: %w", kindsPath, err)
	}
	c.symbolKinds = fieldNames(symbolKinds)
	for _, o := range cli.Options {
		if o.Setting == "" {
			continue
		}
		if held, dup := c.options[o.Setting]; dup {
			return nil, fmt.Errorf("%s: options --%s and --%s both supply %q", cliPath, held, o.Option, o.Setting)
		}
		c.options[o.Setting] = o.Option
	}
	return c, nil
}

// languageIDs lists the languages keep admits, every language for nil, in
// languages.json order.
func (c *configContext) languageIDs(keep func(*languageDoc) bool) []string {
	ids := []string{}
	for i := range c.languages.Languages {
		if keep == nil || keep(&c.languages.Languages[i]) {
			ids = append(ids, c.languages.Languages[i].Language)
		}
	}
	return ids
}

// readers is the languages whose analysis reads a key of one x-owner.
func (c *configContext) readers(owner string) ([]string, error) {
	analyzed := func(l *languageDoc) bool { return l.Kind == "analyzed" && l.Analyzer != "" }
	switch owner {
	case "product", "analyzer":
		return c.languageIDs(analyzed), nil
	case "analysis":
		return c.languageIDs(func(l *languageDoc) bool { return l.Analyzer != "" }), nil
	case "orchestrator":
		return []string{}, nil
	}
	if id, ok := strings.CutSuffix(owner, ownerSuffix); ok && slices.Contains(c.languageIDs(nil), id) {
		return []string{id}, nil
	}
	return nil, fmt.Errorf("x-owner %q is none of product, analyzer, analysis, orchestrator and <language>%s for a language of %s", owner, ownerSuffix, languagesPath)
}

// valueList is the closed value list one x-enum names, from the Contract
// document that owns it.
func (c *configContext) valueList(name string) ([]string, error) {
	switch name {
	case "languages":
		return c.languageIDs(nil), nil
	case "severities":
		return c.kinds.Severities, nil
	case "reachability-classes":
		return c.kinds.ReachabilityClasses, nil
	case "fixabilities":
		return c.kinds.Fixabilities, nil
	case "symbol-kinds":
		return c.symbolKinds, nil
	case "target-kinds":
		return c.kinds.TargetKinds, nil
	case "exemption-classes":
		classes := make([]string, 0, len(c.exemptions.Exemptions))
		for i := range c.exemptions.Exemptions {
			classes = append(classes, c.exemptions.Exemptions[i].Class)
		}
		return classes, nil
	case "severity-keys":
		return c.severityKeys(), nil
	}
	if held, ok := c.schemaValueList(name); ok {
		return held, nil
	}
	return nil, fmt.Errorf("x-enum %q names no value list: languages, severities, reachability-classes, fixabilities, symbol-kinds, target-kinds, exemption-classes, severity-keys, facts-versions, generator-rows, proto-plugins, schema-languages, process-languages or in-process-analyzers", name)
}

// severityKeys is every key the severity object admits, ascending: the code of
// each live kind whose severity the Contract does not fix, and the family key
// of each range holding a live kind and no fixed one.
func (c *configContext) severityKeys() []string {
	var keys []string
	for i := range c.kinds.Ranges {
		key := familyKey(&c.kinds.Ranges[i])
		live, fixed := false, false
		for j := range c.kinds.Kinds {
			if strings.HasPrefix(c.kinds.Kinds[j].Code, key) {
				live = true
				fixed = fixed || c.kinds.Kinds[j].Fixed
			}
		}
		if live && !fixed {
			keys = append(keys, key)
		}
	}
	for i := range c.kinds.Kinds {
		if !c.kinds.Kinds[i].Fixed {
			keys = append(keys, c.kinds.Kinds[i].Code)
		}
	}
	slices.Sort(keys)
	return keys
}

// namePatterns writes, in the place of each patternProperties member named for
// a vocabulary in braces, the pattern of the names that vocabulary admits.
func (c *configContext) namePatterns(patterns *value) error {
	for _, name := range fieldNames(patterns) {
		if vocabulary, braced := strings.CutPrefix(name, "{"); braced {
			pattern, err := c.namePattern(strings.TrimSuffix(vocabulary, "}"))
			if err != nil {
				return err
			}
			replaceMember(patterns, name, pattern, patterns.fields[name])
		}
	}
	return nil
}

// namePattern is the pattern of the names one vocabulary's shape admits: for
// severity-keys the prefix and a family's digits, then optionally a kind's.
func (c *configContext) namePattern(vocabulary string) (string, error) {
	if vocabulary != "severity-keys" {
		return "", fmt.Errorf("patternProperties names {%s}, which is no vocabulary with a name pattern: severity-keys", vocabulary)
	}
	family, code := 0, 0
	for i := range c.kinds.Ranges {
		r := &c.kinds.Ranges[i]
		f, k := len(familyKey(r))-len(c.kinds.Prefix), len(r.Start)-len(familyKey(r))
		if i > 0 && (f != family || k != code) {
			return "", fmt.Errorf("%s: the range %s to %s has codes of another shape", kindsPath, r.Start, r.End)
		}
		family, code = f, k
	}
	if family <= 0 || code <= 0 {
		return "", fmt.Errorf("%s: no range gives a code a family and a kind part", kindsPath)
	}
	return fmt.Sprintf("^%s[0-9]{%d}([0-9]{%d})?$", regexp.QuoteMeta(c.kinds.Prefix), family, code), nil
}

// configKey is one row of contract/settings.json.
type configKey struct {
	def         *value // nil where the key has none
	val         *value // nil for a section
	key         string
	role        string
	owner       string
	keyPattern  string
	option      string
	vocabulary  string
	readBy      []string
	delivered   []string
	values      []string
	choices     []string // the enum of a value schema that lists its own
	perLanguage []string
	required    bool
}

// configuration writes the published configuration schema, contract/settings.json,
// the vocab package's configuration data and the configuration vectors'
// resolved configurations from one walk of the expanded schema, so the four never
// disagree. The authoring source states no list another document owns: x-enum
// names a value list, x-per-language one member per selected language,
// x-delivered is derived from x-owner unless false keeps a key with the
// orchestrator, and a patternProperties member named {severity-keys} stands for
// its name pattern. A key's vocabulary is the list its items or its member names
// draw from.
func configuration(fsys fs.FS, out map[string][]byte) error {
	ctx, err := loadConfigContext(fsys)
	if err != nil {
		return err
	}
	published, err := expandedConfig(fsys)
	if err != nil {
		return err
	}
	maxBytes := published.get("x-max-bytes")
	if maxBytes == nil {
		return fmt.Errorf("%s: no x-max-bytes", configSource)
	}
	var rows []configKey
	if _, walkErr := ctx.walkSection(published, "", &rows); walkErr != nil {
		return fmt.Errorf("%s: %w", configSource, walkErr)
	}
	for _, setting := range sortedKeys(ctx.options) {
		if !slices.ContainsFunc(rows, func(r configKey) bool { return r.key == setting && r.role == roleSetting }) {
			return fmt.Errorf("%s: option --%s supplies %q, which is no setting of %s", cliPath, ctx.options[setting], setting, configPublished)
		}
	}
	src, err := ctx.configVocab(rows, maxBytes)
	if err != nil {
		return err
	}
	if err := configVectors(fsys, rows, out); err != nil {
		return err
	}
	out[configPublished] = encode(published)
	out[settingsPath] = encode(settingsDocument(rows, maxBytes))
	out[configVocabPath] = src
	return nil
}

// walkSection derives every key of one section into rows, in the schema's key
// order, writing each key's x-delivered, and returns the languages that receive
// any key of the section.
func (c *configContext) walkSection(section *value, path string, rows *[]configKey) ([]string, error) {
	props := section.get("properties")
	required := section.get("required").strs()
	receivers := []string{}
	for _, name := range fieldNames(props) {
		key := name
		if path != "" {
			key = path + "." + name
		}
		delivered, err := c.walkKey(props.fields[name], key, slices.Contains(required, name), rows)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		receivers = union(c.languageIDs(nil), receivers, delivered)
	}
	return receivers, nil
}

// walkKey derives one key's row, and a section's keys after it, and returns the
// languages that receive the key.
func (c *configContext) walkKey(node *value, key string, required bool, rows *[]configKey) ([]string, error) {
	row := configKey{key: key, owner: node.get("x-owner").str(), required: required, role: roleOf(node)}
	readBy, err := c.readers(row.owner)
	if err != nil {
		return nil, err
	}
	row.readBy = readBy
	kept, err := keptByOrchestrator(node, row.role)
	if err != nil {
		return nil, err
	}
	row.delivered = []string{}
	if !kept {
		row.delivered = c.languageIDs(func(l *languageDoc) bool { return slices.Contains(readBy, l.Language) && l.Runner == "process" })
	}
	at := len(*rows)
	*rows = append(*rows, row)
	if row.role == roleSection {
		inner, err := c.walkSection(node, key, rows)
		if err != nil {
			return nil, err
		}
		row.delivered = union(c.languageIDs(nil), row.delivered, inner)
	} else if err := c.describeValue(node, &row); err != nil {
		return nil, err
	}
	setAfter(node, "x-owner", "x-delivered", strsValue(row.delivered))
	(*rows)[at] = row
	return row.delivered, nil
}

// keptByOrchestrator reads a key's authored x-delivered: absent, the key goes
// to every analyzer that reads it; false, the orchestrator keeps it.
func keptByOrchestrator(node *value, role string) (bool, error) {
	mark := node.get("x-delivered")
	switch {
	case mark == nil:
		return false, nil
	case role == roleSection:
		return false, errors.New("x-delivered on a section, want it on each key the orchestrator keeps")
	case mark.kind != 's' || string(mark.scalar) != "false":
		return false, fmt.Errorf("x-delivered is %s, want false or absent: the languages are derived from x-owner", bytes.TrimSpace(encode(mark)))
	}
	return true, nil
}

// schemaValueList is the value lists of the facts document and the schema
// languages, and false for a name that is none of them.
func (c *configContext) schemaValueList(name string) ([]string, bool) {
	switch name {
	case "facts-versions":
		return c.contract.FactsVersions, true
	case "generator-rows":
		return c.generators.ids(), true
	case "proto-plugins":
		return c.generators.plugins(), true
	case "schema-languages":
		return c.languageIDs(func(l *languageDoc) bool { return l.Kind == "schema" }), true
	case "process-languages":
		return c.languageIDs(func(l *languageDoc) bool { return l.Runner == "process" }), true
	case "in-process-analyzers":
		var names []string
		for i := range c.languages.Languages {
			if l := &c.languages.Languages[i]; l.Runner == "in-process" {
				names = append(names, l.Analyzer)
			}
		}
		return names, true
	}
	return nil, false
}

// roleOf is what one key of the closed key list is.
func roleOf(node *value) string {
	switch {
	case isTrue(node.get("x-ignored-by-resolution")):
		return roleIgnored
	case node.get("type").str() != jsonObject || isTrue(node.get("x-setting")):
		return roleSetting
	case node.get("patternProperties") != nil:
		return roleKeyed
	}
	return roleSection
}

func isTrue(v *value) bool {
	return v != nil && v.kind == 's' && string(v.scalar) == "true"
}

// describeValue fills the members of a row that is no section: its value schema
// with the generated members written in, its default, vocabulary and option.
func (c *configContext) describeValue(node *value, row *configKey) error {
	row.vocabulary = node.get("items").get("x-enum").str()
	if row.vocabulary == "" {
		row.vocabulary = node.get("propertyNames").get("x-enum").str()
	}
	if node.get("x-enum") == nil {
		row.choices = node.get("enum").strs()
	}
	perLanguage, err := c.fill(node)
	if err != nil {
		return err
	}
	row.perLanguage = perLanguage
	row.def = node.get("default")
	if row.key == contractVersionKey {
		row.def = strValue(c.contract.ContractVersion)
	}
	row.option = c.options[row.key]
	if err := c.namePatterns(node.get("patternProperties")); err != nil {
		return err
	}
	if row.vocabulary != "" {
		values, err := c.valueList(row.vocabulary)
		if err != nil {
			return err
		}
		row.values = values
	}
	row.val = clone(node)
	for _, k := range []string{"default", "x-setting", "x-ignored-by-resolution", "x-delivered"} {
		drop(row.val, k)
	}
	stripAnnotations(row.val)
	if row.role == roleKeyed || row.role == roleIgnored {
		patterns := row.val.get("patternProperties")
		if len(fieldNames(patterns)) != 1 {
			return errors.New("patternProperties holds other than one pattern")
		}
		row.keyPattern = patterns.keys[0]
		row.val = patterns.fields[row.keyPattern]
	}
	return nil
}

// fill writes the members an x-enum or an x-per-language stands for anywhere
// under v, and returns the languages an x-per-language object names.
func (c *configContext) fill(v *value) ([]string, error) {
	var named []string
	for _, child := range append(slices.Clone(v.items), fieldValues(v)...) {
		inner, err := c.fill(child)
		if err != nil {
			return nil, err
		}
		named = append(named, inner...)
	}
	if source := v.get("x-enum"); source != nil {
		values, err := c.valueList(source.str())
		if err != nil {
			return nil, err
		}
		// An enum beside the x-enum lists the values the schema admits after the list's.
		added := v.get("enum").strs()
		if i := slices.IndexFunc(added, func(a string) bool { return slices.Contains(values, a) }); i >= 0 {
			return nil, fmt.Errorf("the enum beside x-enum %q repeats %q", source.str(), added[i])
		}
		drop(v, "enum")
		replaceMember(v, "x-enum", "enum", strsValue(slices.Concat(values, added)))
	}
	if spec := v.get("x-per-language"); spec != nil {
		languages, err := c.perLanguage(v, spec)
		if err != nil {
			return nil, err
		}
		named = append(named, languages...)
	}
	return named, nil
}

// perLanguage writes, in the place of an x-per-language, one member per
// language whose languages.json column the spec's when names, each the spec's
// member with the language's name written into its description, every one
// required unless the spec is optional.
func (c *configContext) perLanguage(v, spec *value) ([]string, error) {
	when, member := spec.get("when").str(), spec.get("member")
	if member == nil {
		return nil, errors.New("x-per-language names no member")
	}
	props := objValue()
	ids := []string{}
	for i := range c.languages.Languages {
		l := &c.languages.Languages[i]
		selected, ok := l.flags()[when]
		if !ok {
			return nil, fmt.Errorf("x-per-language when %q names no boolean column of %s", when, languagesPath)
		}
		if !selected {
			continue
		}
		m := clone(member)
		if d := m.get("description"); d != nil {
			m.set("description", strValue(strings.ReplaceAll(d.str(), namePlaceholder, l.Name)))
		}
		props.set(l.Language, m)
		ids = append(ids, l.Language)
	}
	if isTrue(spec.get("optional")) {
		replaceMember(v, "x-per-language", "properties", props)
		return ids, nil
	}
	replaceMember(v, "x-per-language", "required", strsValue(ids))
	setAfter(v, "required", "properties", props)
	return ids, nil
}

// settingsDocument is contract/settings.json.
func settingsDocument(rows []configKey, maxBytes *value) *value {
	keys := arrValue()
	for i := range rows {
		r := &rows[i]
		row := objValue("key", strValue(r.key), "role", strValue(r.role), "owner", strValue(r.owner),
			"read_by", strsValue(r.readBy), "delivered", strsValue(r.delivered))
		if r.required {
			row.set("required", &value{kind: 's', scalar: json.RawMessage("true")})
		}
		for _, m := range []struct {
			v    *value
			name string
		}{
			{name: "default", v: r.def},
			{name: "option", v: optional(r.option)},
			{name: "per_language", v: optionalList(r.perLanguage)},
			{name: "vocabulary", v: optional(r.vocabulary)},
			{name: "values", v: optionalList(r.values)},
			{name: "key_pattern", v: optional(r.keyPattern)},
			{name: "value", v: r.val},
		} {
			if m.v != nil {
				row.set(m.name, m.v)
			}
		}
		keys.items = append(keys.items, row)
	}
	return objValue("description", strValue(settingsDescription), "max_bytes", maxBytes, "keys", keys)
}

const settingsDescription = "Generated by deadset-gen from authoring/config.schema.json, contract/languages.json, contract/kinds.json, contract/exemptions.json, contract/contract.json and contract/cli.json; an edit belongs in those. One row per key of the closed key list contract/config.schema.json declares, in its key order, the order a resolved configuration prints in. role is section for a key whose own keys a higher-ranked source replaces one by one, setting for a key a higher-ranked source replaces whole, keyed for a section whose keys a pattern admits, each of them one setting, and ignored for the key resolution reads past. owner is the key's x-owner; read_by names the languages whose analysis reads the key, and delivered the languages whose analyzers receive it in the document an orchestrator writes for each. default is the value a run uses where no source names the key; a key with none is a section, or is marked required because the resolved configuration must take it from a source. value is the schema each value of the key is an instance of, and for a keyed or an ignored key the schema of each member's value, whose name matches key_pattern. per_language names the languages a value written per language holds one member for. vocabulary names the Contract vocabulary the entries or member names of the key come from, and values is every value it admits. option is the command-line option of contract/cli.json that supplies the setting. max_bytes bounds, in bytes, a configuration document, a scope document, and the ignore file, the baseline and the edges document at the target root."

// configVocab writes vocab/config_gen.go: the language registry and the key
// table, each value the one contract/settings.json gives.
func (c *configContext) configVocab(rows []configKey, maxBytes *value) ([]byte, error) {
	w := &writer{}
	w.f("// Code generated by deadset-gen from the Contract's documents. DO NOT EDIT.\n\npackage vocab\n\n")
	w.f("// DocumentMaxBytes bounds a configuration document, a scope document, and the\n// ignore file, the baseline and the edges document at the target root: one\n// larger is refused without being read whole.\nconst DocumentMaxBytes = %s\n\n", maxBytes.scalar)
	w.f("var languageRows = []LanguageRow{\n")
	for i := range c.languages.Languages {
		l := &c.languages.Languages[i]
		w.f("{\nLanguage: %s,\nName: %q,\nKind: %q,\nScheme: %q,\nRunner: %q,\nAnalyzer: %q,\n", languageIdent(l.Language), l.Name, l.Kind, l.Scheme, l.Runner, l.Analyzer)
		w.f("Markers: %s,\nExtensions: %s,\nCompanions: %s,\nSkippedDirectories: %s,\nWorkspaceFiles: %s,\n",
			goStrings(l.Markers), goStrings(l.Extensions), goStrings(l.Companions), goStrings(l.SkippedDirectories), goStrings(l.WorkspaceFiles))
		w.f("IdentitySource: IdentitySource{File: %q, Field: %q},\nProgramWithoutMarker: %t,\nTakesTargetKind: %t,\n},\n",
			l.IdentitySource.File, l.IdentitySource.Field, l.ProgramWithoutMarker, l.TakesTargetKind)
	}
	w.f("}\n\nvar generatorRows = []GeneratorRow{\n")
	for i := range c.generators.Generators {
		g := &c.generators.Generators[i]
		var low, high string
		if g.Versions != nil {
			low, high = g.Versions.Min, g.Versions.Max
		}
		w.f("{ID: %q, Plugin: %q, Language: %s, Header: %q, RuntimeImport: %q, Cell: %q, VersionMin: %q, VersionMax: %q, Major: %d},\n",
			g.ID, g.Plugin, languageIdent(g.Language), g.Header, g.RuntimeImport, g.Cell, low, high, g.Major)
	}
	w.f("}\n\nvar protoPlugins = %s\n\nvar httpExposurePlugins = %s\n", goStrings(c.generators.plugins()), goStrings(c.generators.HTTPExposurePlugins))
	w.f("\n// The keys of the closed configuration key list, by their dotted paths.\nconst (\n")
	for i := range rows {
		w.f("%s Key = %q\n", c.keyIdent(rows[i].key), rows[i].key)
	}
	w.f(")\n\n// The choices of each setting whose value schema lists them itself.\nconst (\n")
	for i := range rows {
		for _, choice := range rows[i].choices {
			w.f("%s%s Choice = %q\n", strings.TrimPrefix(c.keyIdent(rows[i].key), "Key"), ident(choice), choice)
		}
	}
	w.f(")\n\nvar configKeys = []ConfigKey{\n")
	for i := range rows {
		c.configKeyRow(w, &rows[i])
	}
	w.f("}\n")
	src, err := format.Source([]byte(w.b.String()))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", configVocabPath, err)
	}
	return src, nil
}

func (c *configContext) configKeyRow(w *writer, r *configKey) {
	w.f("{\nKey: %s,\nRole: Role%s,\nOwner: %q,\nReadBy: %s,\nDelivered: %s,\n", c.keyIdent(r.key), ident(r.role), r.owner,
		goConsts("Language", r.readBy, languageIdent), goConsts("Language", r.delivered, languageIdent))
	for _, m := range []struct{ name, text string }{
		{"Default", compact(r.def)}, {"Value", compact(r.val)}, {"KeyPattern", r.keyPattern},
	} {
		if m.text != "" {
			w.f("%s: %s,\n", m.name, goText(m.text))
		}
	}
	for _, m := range []struct{ name, text string }{{"Option", r.option}, {"Vocabulary", r.vocabulary}} {
		if m.text != "" {
			w.f("%s: %q,\n", m.name, m.text)
		}
	}
	if len(r.values) > 0 {
		w.f("Values: %s,\n", goStrings(r.values))
	}
	if len(r.perLanguage) > 0 {
		w.f("PerLanguage: %s,\n", goConsts("Language", r.perLanguage, languageIdent))
	}
	if r.required {
		w.f("Required: true,\n")
	}
	w.f("},\n")
}

// keyIdent is the constant naming one key: Key, then each part of its path
// spelled as an identifier, a language as the vocabulary's constant spells it.
func (c *configContext) keyIdent(key string) string {
	var b strings.Builder
	b.WriteString("Key")
	for part := range strings.SplitSeq(key, ".") {
		if slices.Contains(c.languageIDs(nil), part) {
			b.WriteString(languageIdent(part))
			continue
		}
		b.WriteString(ident(strings.ReplaceAll(part, "_", "-")))
	}
	return b.String()
}

// goText is a Go string literal for s, raw where s holds no backquote, so a
// pattern reads as the schema writes it.
func goText(s string) string {
	if strings.Contains(s, "`") {
		return strconv.Quote(s)
	}
	return "`" + s + "`"
}

// compact is v as one line of JSON, and "" for nil.
func compact(v *value) string {
	if v == nil {
		return ""
	}
	var b bytes.Buffer
	if err := json.Compact(&b, encode(v)); err != nil {
		panic(err) // encode writes valid JSON of every value parse and the builders make
	}
	return b.String()
}

func optional(s string) *value {
	if s == "" {
		return nil
	}
	return strValue(s)
}

func optionalList(ss []string) *value {
	if len(ss) == 0 {
		return nil
	}
	return strsValue(ss)
}

// union is the members of all any of lists names, in the order of all.
func union(all []string, lists ...[]string) []string {
	out := []string{}
	for _, l := range all {
		if slices.ContainsFunc(lists, func(list []string) bool { return slices.Contains(list, l) }) {
			out = append(out, l)
		}
	}
	return out
}

func fieldNames(v *value) []string {
	if v == nil || v.kind != 'o' {
		return nil
	}
	return v.keys
}

func fieldValues(v *value) []*value {
	out := make([]*value, 0, len(fieldNames(v)))
	for _, k := range fieldNames(v) {
		out = append(out, v.fields[k])
	}
	return out
}

// replaceMember puts a member in the place of another, keeping the order.
func replaceMember(v *value, old, name string, child *value) {
	i := slices.Index(v.keys, old)
	delete(v.fields, old)
	v.keys[i] = name
	v.fields[name] = child
}

// setAfter sets a member right after another, or last where the other is
// absent; a member already set keeps its place.
func setAfter(v *value, after, name string, child *value) {
	if _, held := v.fields[name]; held {
		v.fields[name] = child
		return
	}
	i := slices.Index(v.keys, after) + 1
	if i == 0 {
		i = len(v.keys)
	}
	v.keys = slices.Insert(v.keys, i, name)
	v.fields[name] = child
}

// stripAnnotations drops every description and x-owner from a schema and the
// schemas it applies, leaving the values its keywords hold, a default among
// them, as they are.
func stripAnnotations(s *value) {
	drop(s, "description")
	drop(s, "x-owner")
	for _, k := range fieldNames(s) {
		child := s.fields[k]
		switch k {
		case "properties", "patternProperties":
			for _, member := range fieldValues(child) {
				stripAnnotations(member)
			}
		case "items", "not":
			stripAnnotations(child)
		case "oneOf", "anyOf", "allOf":
			for _, branch := range child.items {
				stripAnnotations(branch)
			}
		}
	}
}

func clone(v *value) *value {
	n := &value{kind: v.kind, scalar: v.scalar, keys: slices.Clone(v.keys)}
	for _, it := range v.items {
		n.items = append(n.items, clone(it))
	}
	if v.fields != nil {
		n.fields = make(map[string]*value, len(v.fields))
		for k, f := range v.fields {
			n.fields[k] = clone(f)
		}
	}
	return n
}
