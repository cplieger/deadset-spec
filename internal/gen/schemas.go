package gen

import (
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
)

const (
	configSource      = "authoring/config.schema.json"
	configPublished   = "contract/config.schema.json"
	findingSource     = "authoring/finding.schema.json"
	findingPublished  = "contract/finding.schema.json"
	kindsPath         = "contract/kinds.json"
	defsPrefix        = "#/$defs/"
	deletableOnlyName = "removes_last_use_of"
	impliesKey        = "x-implies"
	presenceRequired  = "required"
)

// expandedConfig is the configuration schema's source with every key declared
// inline, because each product walks it by key path, from a source that states a
// repeated sub-schema once under $defs.
func expandedConfig(fsys fs.FS) (*value, error) {
	src, err := readValue(fsys, configSource)
	if err != nil {
		return nil, err
	}
	defs := src.get("$defs")
	if defs == nil {
		return nil, fmt.Errorf("%s: no $defs", configSource)
	}
	used := map[string]bool{}
	expanded, err := expand(src, defs, used, nil)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", configSource, err)
	}
	for _, name := range defs.keys {
		if !used[name] {
			return nil, fmt.Errorf("%s: $defs/%s is referenced nowhere", configSource, name)
		}
	}
	drop(expanded, "$defs")
	return expanded, nil
}

// expand returns v with every {"$ref": "#/$defs/x", ...} replaced by the
// definition's members spliced where the reference stands, beside the
// reference's own siblings. A sibling the definition also declares is refused,
// and so is a reference cycle.
func expand(v, defs *value, used map[string]bool, stack []string) (*value, error) {
	switch v.kind {
	case 'a':
		n := &value{kind: 'a', items: []*value{}}
		for _, it := range v.items {
			e, err := expand(it, defs, used, stack)
			if err != nil {
				return nil, err
			}
			n.items = append(n.items, e)
		}
		return n, nil
	case 'o':
		return expandObject(v, defs, used, stack)
	}
	return v, nil
}

func expandObject(v, defs *value, used map[string]bool, stack []string) (*value, error) {
	n := &value{kind: 'o', fields: map[string]*value{}}
	for _, k := range v.keys {
		var members *value
		var err error
		switch {
		case k == "$defs" && len(stack) == 0:
			n.set(k, v.fields[k])
			continue
		case k == "$ref":
			members, err = definition(v.fields[k].str(), defs, used, stack)
		default:
			var child *value
			child, err = expand(v.fields[k], defs, used, stack)
			members = objValue(k, child)
		}
		if err != nil {
			return nil, err
		}
		for _, mk := range members.keys {
			if _, dup := n.fields[mk]; dup || (k == "$ref" && v.fields[mk] != nil) {
				return nil, fmt.Errorf("member %q is both a sibling and a member of a definition", mk)
			}
			n.set(mk, members.fields[mk])
		}
	}
	return n, nil
}

// definition returns the expanded members of the definition a reference names.
func definition(ref string, defs *value, used map[string]bool, stack []string) (*value, error) {
	name, ok := strings.CutPrefix(ref, defsPrefix)
	def := defs.get(name)
	if !ok || def == nil {
		return nil, fmt.Errorf("$ref %q names no definition", ref)
	}
	if slices.Contains(stack, name) {
		return nil, fmt.Errorf("$defs/%s refers to itself", name)
	}
	used[name] = true
	return expand(def, defs, used, append(stack, name))
}

func drop(v *value, key string) {
	if _, ok := v.fields[key]; !ok {
		return
	}
	delete(v.fields, key)
	v.keys = slices.DeleteFunc(v.keys, func(k string) bool { return k == key })
}

// kindRow is what the finding schema's generated branches read of a kind.
type kindRow struct {
	details   *value
	code      string
	holdsLive bool
}

func loadKinds(fsys fs.FS) ([]kindRow, *value, error) {
	doc, err := readValue(fsys, kindsPath)
	if err != nil {
		return nil, nil, err
	}
	rows := doc.get("kinds")
	if rows == nil {
		return nil, nil, errors.New(kindsPath + ": no kinds")
	}
	out := make([]kindRow, 0, len(rows.items))
	for _, r := range rows.items {
		out = append(out, kindRow{
			code:      r.get("code").str(),
			holdsLive: string(r.get("holds_live").scalar) == "true",
			details:   r.get("details"),
		})
	}
	return out, doc, nil
}

// detailsBranch is one generated branch: the codes that carry a set of details
// members and how they carry them.
type detailsBranch struct {
	implies   *value // what a finding carrying an optional member must also hold
	members   []string
	codes     []string
	presence  string   // "required" or "optional"
	languages []string // set where only these languages' findings carry the members
}

// filledSchemas are the published schemas whose source differs from them only
// by the members x-enum and x-per-language stand for, by source.
var filledSchemas = map[string]string{
	"authoring/describe.schema.json":            "contract/describe.schema.json",
	"authoring/report.schema.json":              "contract/report.schema.json",
	"authoring/conformance-results.schema.json": "corpus/conformance-results.schema.json",
	"authoring/expect.schema.json":              "corpus/expect.schema.json",
}

// publishedSchemas writes each of filledSchemas from its source.
func publishedSchemas(fsys fs.FS, out map[string][]byte) error {
	ctx, err := loadConfigContext(fsys)
	if err != nil {
		return err
	}
	for _, source := range sortedKeys(filledSchemas) {
		src, err := readValue(fsys, source)
		if err != nil {
			return err
		}
		if _, err := ctx.fill(src); err != nil {
			return fmt.Errorf("%s: %w", source, err)
		}
		out[filledSchemas[source]] = encode(src)
	}
	return nil
}

// findingSchema writes the published finding schema: its source, with the
// members x-enum and x-per-language stand for written in, and the per-code
// branches of the details object and the liveness-relation rule generated from
// the kind rows ahead of the source's own allOf entries.
func findingSchema(fsys fs.FS, out map[string][]byte) error {
	src, err := readValue(fsys, findingSource)
	if err != nil {
		return err
	}
	ctx, err := loadConfigContext(fsys)
	if err != nil {
		return err
	}
	if _, err = ctx.fill(src); err != nil {
		return fmt.Errorf("%s: %w", findingSource, err)
	}
	kinds, kindsDoc, err := loadKinds(fsys)
	if err != nil {
		return err
	}
	symbolKinds := kindsDoc.get("symbol_kinds")
	members := src.get("properties").get("details").get("properties")
	if members == nil {
		return fmt.Errorf("%s: no details members", findingSource)
	}
	branches, err := detailsBranches(kinds, members.keys)
	if err != nil {
		return err
	}
	if err := attachImplies(branches, members); err != nil {
		return err
	}
	generated := make([]*value, 0, len(branches)+1)
	for i := range branches {
		generated = append(generated, branches[i].schema())
	}
	generated = append(generated, relationRule(kinds, symbolKinds))
	own := src.get("allOf")
	if own == nil {
		own = arrValue()
	}
	src.set("allOf", arrValue(append(generated, own.items...)...))
	order := []string{"$schema", "title", "description", "type", "additionalProperties", "required", "properties", "allOf", "$defs"}
	src.keys = slices.DeleteFunc(slices.Clone(order), func(k string) bool { return src.fields[k] == nil })
	out[findingPublished] = encode(src)
	return nil
}

// carriage is how the kinds carry one details member: the codes, and the
// presence they all give it.
type carriage struct {
	presence  string
	codes     []string
	languages []string
}

// attachImplies moves each optional member's x-implies annotation from the
// source onto the branch that carries the member, and out of the published
// schema.
func attachImplies(branches []detailsBranch, members *value) error {
	for i := range branches {
		if implies := members.get(branches[i].members[0]).get(impliesKey); implies != nil {
			if len(branches[i].members) != 1 || branches[i].presence != "optional" {
				return fmt.Errorf("%s: %s on details.%s, want it on an optional member a branch carries alone", findingSource, impliesKey, branches[i].members[0])
			}
			branches[i].implies = implies
		}
	}
	for _, m := range members.keys {
		drop(members.get(m), impliesKey)
	}
	return nil
}

func detailsBranches(kinds []kindRow, memberOrder []string) ([]detailsBranch, error) {
	by, err := carriages(kinds, memberOrder)
	if err != nil {
		return nil, err
	}
	var branches []detailsBranch
	for _, m := range memberOrder {
		c := by[m]
		if c == nil {
			continue
		}
		i := slices.IndexFunc(branches, func(b detailsBranch) bool {
			return slices.Equal(b.codes, c.codes) && b.presence == c.presence && slices.Equal(b.languages, c.languages)
		})
		if i >= 0 {
			branches[i].members = append(branches[i].members, m)
			continue
		}
		branches = append(branches, detailsBranch{members: []string{m}, codes: c.codes, presence: c.presence, languages: c.languages})
	}
	return branches, nil
}

func carriages(kinds []kindRow, memberOrder []string) (map[string]*carriage, error) {
	by := map[string]*carriage{}
	for i := range kinds {
		k := &kinds[i]
		if k.details == nil {
			continue
		}
		for _, m := range k.details.keys {
			if !slices.Contains(memberOrder, m) || m == deletableOnlyName {
				return nil, fmt.Errorf("%s: kind %s names details member %q the finding schema does not carry per code", kindsPath, k.code, m)
			}
			if err := carry(by, m, k.code, k.details.fields[m]); err != nil {
				return nil, err
			}
		}
	}
	return by, nil
}

// carry records that code carries member m as presence states, refusing a
// member two kinds carry two ways.
func carry(by map[string]*carriage, m, code string, presence *value) error {
	p, langs := presenceOf(presence)
	c := by[m]
	if c == nil {
		c = &carriage{presence: p, languages: langs}
		by[m] = c
	}
	if c.presence != p || !slices.Equal(c.languages, langs) {
		return fmt.Errorf("%s: details member %q is carried two ways", kindsPath, m)
	}
	c.codes = append(c.codes, code)
	return nil
}

// presenceOf reads a kind's details column: required, optional, or the
// languages whose findings carry the member, which then require it.
func presenceOf(v *value) (presence string, languages []string) {
	if v.kind == 'a' {
		return presenceRequired, v.strs()
	}
	return v.str(), nil
}

func codeIs(codes []string) *value {
	match := objValue("const", strValue(codes[0]))
	if len(codes) > 1 {
		match = objValue("enum", strsValue(codes))
	}
	return objValue("required", strsValue([]string{"code"}), "properties", objValue("code", match))
}

func detailsRequire(members []string) *value {
	return objValue("properties", objValue("details", objValue("required", strsValue(members))))
}

func detailsForbid(members []string) *value {
	alternatives := arrValue()
	for _, m := range members {
		alternatives.items = append(alternatives.items, objValue("required", strsValue([]string{m})))
	}
	return objValue("properties", objValue("details", objValue("not", objValue("anyOf", alternatives))))
}

func (b *detailsBranch) schema() *value {
	branch := objValue("if", codeIs(b.codes))
	switch {
	case len(b.languages) > 0:
		branch.set("then", objValue("allOf", arrValue(objValue(
			"if", objValue("required", strsValue([]string{"language"}), "properties", objValue("language", objValue("enum", strsValue(b.languages)))),
			"then", detailsRequire(b.members),
			"else", detailsForbid(b.members),
		))))
	case b.presence == presenceRequired:
		branch.set("then", detailsRequire(b.members))
	case b.implies != nil:
		branch.set("then", objValue(
			"if", objValue("required", strsValue([]string{"details"}), "properties", objValue("details", objValue("required", strsValue(b.members)))),
			"then", b.implies,
		))
	}
	branch.set("else", detailsForbid(b.members))
	return branch
}

// relationRule states that a finding carries a liveness relation exactly when
// its symbol kind's subject is a declaration and its kind does not hold the
// subject live.
func relationRule(kinds []kindRow, symbolKinds *value) *value {
	var parts, live []string
	for _, kind := range symbolKinds.keys {
		if symbolKinds.fields[kind].str() != "declaration" {
			parts = append(parts, kind)
		}
	}
	slices.Sort(parts)
	for i := range kinds {
		if kinds[i].holdsLive {
			live = append(live, kinds[i].code)
		}
	}
	symbol := objValue("required", strsValue([]string{"symbol"}), "properties", objValue("symbol",
		objValue("required", strsValue([]string{"kind"}), "properties", objValue("kind", objValue("enum", strsValue(parts))))))
	return objValue(
		"if", objValue("anyOf", arrValue(symbol, codeIs(live))),
		"then", objValue("not", objValue("required", strsValue([]string{"liveness_relation"}))),
		"else", objValue("required", strsValue([]string{"liveness_relation"})),
	)
}
