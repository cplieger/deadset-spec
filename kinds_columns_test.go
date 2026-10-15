package spec_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"maps"
	"slices"

	spec "github.com/cplieger/deadset-spec/v8"
)

// The tables below are read from contract/kinds.json's columns, the one place
// each fact is stated, so a test compares a schema or a page against the
// vocabulary rather than against a second copy of it.
var (
	findingDetailsBranches   = kindsColumns().branches
	findingEmptyDetailsCodes = kindsColumns().empty
	findingNoRelationKinds   = kindsColumns().noRelationKinds
	findingLiveSubjectCodes  = kindsColumns().holdsLive
)

type kindsColumnTables struct {
	carrying        map[string][]string
	branches        []findingDetailsBranch
	empty           []string
	noRelationKinds []string
	holdsLive       []string
}

// kindsColumns groups the details members by the codes that carry them, as the
// finding schema's generated branches do: members carried by the same codes the
// same way share one branch, in the order the finding schema declares them.
func kindsColumns() kindsColumnTables {
	var doc struct {
		SymbolKinds map[string]string `json:"symbol_kinds"`
		Kinds       []struct {
			Details   map[string]any `json:"details"`
			Code      string         `json:"code"`
			HoldsLive bool           `json:"holds_live"`
		} `json:"kinds"`
	}
	var finding struct {
		Properties struct {
			Details struct {
				Properties json.RawMessage `json:"properties"`
			} `json:"details"`
		} `json:"properties"`
	}
	mustDecode(kindsPath, &doc)
	mustDecode(findingSchemaPath, &finding)
	order := memberOrder(finding.Properties.Details.Properties)
	out := kindsColumnTables{carrying: map[string][]string{}}
	how := map[string]string{}
	for _, k := range doc.Kinds {
		if len(k.Details) == 0 {
			out.empty = append(out.empty, k.Code)
		}
		if k.HoldsLive {
			out.holdsLive = append(out.holdsLive, k.Code)
		}
		for m, presence := range k.Details {
			out.carrying[m] = append(out.carrying[m], k.Code)
			raw, _ := json.Marshal(presence)
			how[m] = string(raw)
		}
	}
	for _, m := range order {
		codes := out.carrying[m]
		if codes == nil {
			continue
		}
		joined := false
		for i := range out.branches {
			b := &out.branches[i]
			if slices.Equal(b.codes, codes) && how[b.fields[0]] == how[m] {
				b.fields = append(b.fields, m)
				joined = true
			}
		}
		if !joined {
			out.branches = append(out.branches, findingDetailsBranch{codes: codes, fields: []string{m}})
		}
	}
	for _, kind := range slices.Sorted(maps.Keys(doc.SymbolKinds)) {
		if doc.SymbolKinds[kind] != "declaration" {
			out.noRelationKinds = append(out.noRelationKinds, kind)
		}
	}
	return out
}

func mustDecode(path string, into any) {
	data, err := fs.ReadFile(spec.Contract, path)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, into); err != nil {
		panic(path + ": " + err.Error())
	}
}

// memberOrder lists an object's member names in the order the document writes
// them.
func memberOrder(raw json.RawMessage) []string {
	dec := json.NewDecoder(bytes.NewReader(raw))
	var names []string
	if _, err := dec.Token(); err != nil {
		return nil
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return names
		}
		name, _ := tok.(string)
		names = append(names, name)
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return names
		}
	}
	return names
}
