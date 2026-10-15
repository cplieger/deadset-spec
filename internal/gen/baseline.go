package gen

import (
	"fmt"
	"io/fs"
	"strconv"
)

// BaselinePath is the release snapshot of the closed vocabularies. It is
// rewritten only on request, at a release, never by Generate: a snapshot
// regenerated with every change would pin nothing.
const BaselinePath = "testdata/contract-baseline.json"

// Baseline returns the snapshot of the closed vocabularies on disk, keeping the
// committed snapshot's description.
func Baseline(fsys fs.FS) ([]byte, error) {
	old, err := readValue(fsys, BaselinePath)
	if err != nil {
		return nil, err
	}
	contract, err := readValue(fsys, "contract/contract.json")
	if err != nil {
		return nil, err
	}
	_, kinds, err := loadKinds(fsys)
	if err != nil {
		return nil, err
	}
	exemptions, err := readValue(fsys, "contract/exemptions.json")
	if err != nil {
		return nil, err
	}
	finding, err := readValue(fsys, findingPublished)
	if err != nil {
		return nil, err
	}
	exits, err := readValue(fsys, "contract/exit-codes.json")
	if err != nil {
		return nil, err
	}
	sets := map[string]*value{
		"kind_codes":        column(kinds.get("kinds"), "code"),
		"retired_codes":     column(kinds.get("retired"), "code"),
		"exemption_classes": column(exemptions.get("exemptions"), "class"),
		"symbol_kinds":      finding.get("properties").get("symbol").get("properties").get("kind").get("enum"),
		"exit_codes":        column(exits.get("exit_codes"), "code"),
	}
	out := objValue("description", old.get("description"),
		"contract_version", contract.get("contract_version"),
		"schema_versions", contract.get("schema_versions"))
	for _, key := range old.keys {
		if out.fields[key] != nil {
			continue
		}
		if set, ok := sets[key]; ok {
			out.set(key, set)
			continue
		}
		out.set(key, old.get(key))
	}
	for _, key := range out.keys {
		if out.fields[key] == nil {
			return nil, fmt.Errorf("%s: no value for %s", BaselinePath, strconv.Quote(key))
		}
	}
	return encode(out), nil
}

func column(rows *value, key string) *value {
	out := arrValue()
	if rows == nil {
		return out
	}
	for _, r := range rows.items {
		out.items = append(out.items, r.get(key))
	}
	return out
}
