package gen

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
)

const (
	vectorDeltas     = "authoring/config-vectors"
	configVectorsDir = "vectors/config"
	provenanceKey    = "provenance"
	inputKey         = "x-input"
	defaultSource    = "default"
	repositorySource = "repository: repository.json"
)

// vectorCases is what configVectors has resolved so far: each case's resolved
// configuration, and the cases whose input is another case's.
type vectorCases struct {
	resolved map[string]*value
	readBack map[string]string
}

// configVectors writes each configuration case's expected.json from the delta
// authoring/config-vectors/<case>.json states: what the case decides, in the
// resolved configuration's own shape, every other key taking its default. A
// delta's x-input makes the case's repository.json name every key: an object is
// that input's provenance member, and a case name reads that case's
// expected.json back as the input. Either way a key the table gains reaches
// every case without a hand edit.
func configVectors(fsys fs.FS, rows []configKey, out map[string][]byte) error {
	if err := onlyProvenanceReadPast(rows); err != nil {
		return err
	}
	cases, err := fs.ReadDir(fsys, configVectorsDir)
	if err != nil {
		return err
	}
	deltas, err := fs.Glob(fsys, vectorDeltas+"/*.json")
	if err != nil {
		return err
	}
	done := vectorCases{resolved: map[string]*value{}, readBack: map[string]string{}}
	for _, entry := range cases {
		if !entry.IsDir() {
			continue
		}
		dir, delta := path.Join(configVectorsDir, entry.Name()), path.Join(vectorDeltas, entry.Name()+".json")
		if !slices.Contains(deltas, delta) {
			if _, err := fs.Stat(fsys, path.Join(dir, "expected-error.json")); err != nil {
				return fmt.Errorf("%s: no expected-error.json and no %s to write expected.json from", dir, delta)
			}
			continue
		}
		deltas = slices.DeleteFunc(deltas, func(d string) bool { return d == delta })
		if err := done.resolve(fsys, rows, entry.Name(), out); err != nil {
			return fmt.Errorf("%s: %w", delta, err)
		}
	}
	if len(deltas) > 0 {
		return fmt.Errorf("%v: no case under %s", deltas, configVectorsDir)
	}
	return done.readInputsBack(rows, out)
}

// onlyProvenanceReadPast refuses a read-past key other than provenance:
// resolvedFrom writes the provenance entries under every such key.
func onlyProvenanceReadPast(rows []configKey) error {
	for i := range rows {
		if r := &rows[i]; r.role == roleIgnored && r.key != provenanceKey {
			return fmt.Errorf("%s is read past and is not %s; the vector generator writes no value for it", r.key, provenanceKey)
		}
	}
	return nil
}

// resolve writes the files one case's delta stands for, or keeps the case for
// readInputsBack where its input is another case's.
func (d *vectorCases) resolve(fsys fs.FS, rows []configKey, name string, out map[string][]byte) error {
	v, err := readValue(fsys, path.Join(vectorDeltas, name+".json"))
	if err != nil {
		return err
	}
	input := v.get(inputKey)
	if from := input.str(); from != "" {
		d.readBack[name] = from
		return nil
	}
	unnamed := defaultSource
	if input != nil {
		unnamed = repositorySource
	}
	doc, written, err := resolvedFrom(rows, v, unnamed)
	if err != nil {
		return err
	}
	d.resolved[name] = doc
	out[path.Join(configVectorsDir, name, "expected.json")] = encode(doc)
	if input != nil {
		written.set(provenanceKey, input)
		out[path.Join(configVectorsDir, name, "repository.json")] = encode(written)
	}
	return nil
}

// readInputsBack writes each case that reads another case's resolved
// configuration back as its repository.json, every setting then coming from it.
func (d *vectorCases) readInputsBack(rows []configKey, out map[string][]byte) error {
	for _, name := range sortedKeys(d.readBack) {
		from := d.resolved[d.readBack[name]]
		if from == nil {
			return fmt.Errorf("%s/%s.json: x-input names %q, no case resolved from a delta of its own", vectorDeltas, name, d.readBack[name])
		}
		values := clone(from)
		drop(values, provenanceKey)
		doc, _, err := resolvedFrom(rows, values, repositorySource)
		if err != nil {
			return fmt.Errorf("%s/%s.json: %w", vectorDeltas, name, err)
		}
		out[path.Join(configVectorsDir, name, "repository.json")] = encode(from)
		out[path.Join(configVectorsDir, name, "expected.json")] = encode(doc)
	}
	return nil
}

// resolvedFrom is the resolved configuration a delta stands for: every key of
// rows in their order, each the delta's value or the key's default, a keyed
// one's members ascending, and one provenance entry per setting, the delta's or
// unnamed, a keyed setting's one per member or one of its own when it has none.
// The input beside it holds the same values as the delta writes them, and no
// provenance.
func resolvedFrom(rows []configKey, delta *value, unnamed string) (doc, input *value, err error) {
	if err := knownPaths(rows, delta, ""); err != nil {
		return nil, nil, err
	}
	p := &provenanceWriter{named: delta.get(provenanceKey), unnamed: unnamed, entries: objValue(), used: map[string]bool{}}
	doc, input = objValue(), objValue()
	for i := range rows {
		r := &rows[i]
		parent, inParent, name := doc, input, r.key
		if at := strings.LastIndex(r.key, "."); at >= 0 {
			parent, inParent, name = atPath(doc, r.key[:at]), atPath(input, r.key[:at]), r.key[at+1:]
		}
		switch r.role {
		case roleSection:
			parent.set(name, objValue())
			inParent.set(name, objValue())
			continue
		case roleIgnored:
			parent.set(name, p.entries)
			continue
		}
		v, decided := atPath(delta, r.key), true
		if v == nil {
			v, decided = r.def, false
		}
		if v == nil {
			return nil, nil, fmt.Errorf("names no %s, which has no default", r.key)
		}
		inParent.set(name, v)
		resolved, err := p.setting(r, v, decided)
		if err != nil {
			return nil, nil, err
		}
		parent.set(name, resolved)
	}
	for _, entry := range fieldNames(p.named) {
		if !p.used[entry] {
			return nil, nil, fmt.Errorf("provenance names %s, which is no setting the case resolves", entry)
		}
	}
	return doc, input, nil
}

// provenanceWriter writes the provenance entries of one resolved configuration.
type provenanceWriter struct {
	named   *value // the delta's own entries
	entries *value
	used    map[string]bool
	unnamed string
}

// setting writes one setting's entries and returns its resolved value: for a
// keyed setting with members, one entry per member and the members ascending.
func (p *provenanceWriter) setting(r *configKey, v *value, decided bool) (*value, error) {
	entries := []string{r.key}
	if r.role == roleKeyed && len(fieldNames(v)) > 0 {
		ascending := objValue()
		entries = entries[:0]
		for _, member := range slices.Sorted(slices.Values(fieldNames(v))) {
			ascending.set(member, v.fields[member])
			entries = append(entries, r.key+"."+member)
		}
		v = ascending
	}
	for _, entry := range entries {
		source := p.named.get(entry)
		switch {
		case source != nil:
			p.used[entry] = true
		case decided && p.unnamed == defaultSource:
			return nil, fmt.Errorf("names %s and no provenance entry for it", entry)
		default:
			source = strValue(p.unnamed)
		}
		p.entries.set(entry, source)
	}
	return v, nil
}

// knownPaths refuses a member of a delta that names no key of rows.
func knownPaths(rows []configKey, delta *value, at string) error {
	for _, name := range fieldNames(delta) {
		key := name
		switch {
		case at != "":
			key = at + "." + name
		case name == provenanceKey || name == inputKey:
			continue
		}
		i := slices.IndexFunc(rows, func(r configKey) bool { return r.key == key })
		if i < 0 {
			return fmt.Errorf("names %s, which is no key of %s", key, configPublished)
		}
		if rows[i].role == roleSection {
			if err := knownPaths(rows, delta.fields[name], key); err != nil {
				return err
			}
		}
	}
	return nil
}

// atPath is the member of v a dotted path names, or nil.
func atPath(v *value, dotted string) *value {
	for part := range strings.SplitSeq(dotted, ".") {
		v = v.get(part)
	}
	return v
}
