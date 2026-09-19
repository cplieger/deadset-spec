package spec_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/cplieger/deadset-spec"
)

const contractPath = "contract/contract.json"

// contractVersionExempt is the one committed document that names a version this
// contract does not admit: its whole job is to be the report a caller refuses on
// admission, so it stays at the contract version and the schema version it was
// written against.
const contractVersionExempt = "vectors/merge/schema-version-out-of-range/inputs/00-go.json"

// contractDocument mirrors contract/contract.json; an unknown key fails the
// decode.
type contractDocument struct {
	Description     string   `json:"description"`
	ContractVersion string   `json:"contract_version"`
	SchemaVersions  []string `json:"schema_versions"`
	Platforms       []string `json:"platforms"`
}

// versionedDocument is the part of any committed instance document this suite
// reads: the two versions it names and the schema versions its writer accepts.
type versionedDocument struct {
	ContractVersion string   `json:"contract_version"`
	SchemaVersion   string   `json:"schema_version"`
	Accepted        []string `json:"schema_versions_accepted"`
}

// loadContract decodes contract/contract.json, failing the test on any setup
// error so every caller reads a document that exists and parses.
func loadContract(t *testing.T) contractDocument {
	t.Helper()
	data, err := fs.ReadFile(spec.Contract, contractPath)
	if err != nil {
		t.Fatalf("Setup: fs.ReadFile(Contract, %q): %v", contractPath, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var doc contractDocument
	if err = dec.Decode(&doc); err != nil {
		t.Fatalf("Setup: decoding %s: %v", contractPath, err)
	}
	return doc
}

// contractVersionedDocuments reads every JSON document of one embedded tree that
// names a version, keyed by its path in that tree.
func contractVersionedDocuments(t *testing.T, tree fs.FS, root string) map[string]versionedDocument {
	t.Helper()
	found := map[string]versionedDocument{}
	err := fs.WalkDir(tree, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".json") {
			return nil
		}
		data, err := fs.ReadFile(tree, p)
		if err != nil {
			return err
		}
		var doc versionedDocument
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil
		}
		if doc.ContractVersion == "" && doc.SchemaVersion == "" && len(doc.Accepted) == 0 {
			return nil
		}
		found[p] = doc
		return nil
	})
	if err != nil {
		t.Fatalf("Setup: fs.WalkDir(%s): %v", root, err)
	}
	if len(found) == 0 {
		t.Fatalf("Setup: %s holds no versioned document, want the committed instances", root)
	}
	return found
}

// TestContractVersionsAreTheOnesEveryCommittedDocumentNames pins the version
// pass a contract amendment forces: this repository publishes one contract
// version and one report shape, so every committed instance document names that
// contract version, carries a schema version the contract admits, and accepts
// exactly the versions the contract admits. The one document that exists to be
// refused on admission is named above and skipped.
func TestContractVersionsAreTheOnesEveryCommittedDocumentNames(t *testing.T) {
	contract := loadContract(t)
	if len(contract.SchemaVersions) == 0 {
		t.Fatalf("Setup: %s schema_versions = %v, want at least one admitted report shape", contractPath, contract.SchemaVersions)
	}
	documents := map[string]versionedDocument{}
	maps.Copy(documents, contractVersionedDocuments(t, spec.Examples, "examples"))
	maps.Copy(documents, contractVersionedDocuments(t, spec.Vectors, "vectors"))
	for _, p := range slices.Sorted(maps.Keys(documents)) {
		if p == contractVersionExempt {
			continue
		}
		doc := documents[p]
		t.Run(caseName(strings.TrimSuffix(p, path.Ext(p))), func(t *testing.T) {
			if doc.ContractVersion != "" && doc.ContractVersion != contract.ContractVersion {
				t.Errorf("Document(%s).contract_version = %q, want the one %s names, %q", p, doc.ContractVersion, contractPath, contract.ContractVersion)
			}
			if doc.SchemaVersion != "" && !slices.Contains(contract.SchemaVersions, doc.SchemaVersion) {
				t.Errorf("Document(%s).schema_version = %q, want one of the versions %s admits, %v", p, doc.SchemaVersion, contractPath, contract.SchemaVersions)
			}
			if len(doc.Accepted) != 0 && !slices.Equal(doc.Accepted, contract.SchemaVersions) {
				t.Errorf("Document(%s).schema_versions_accepted = %v, want the versions %s admits, %v", p, doc.Accepted, contractPath, contract.SchemaVersions)
			}
		})
	}
}

// contractSchemaVersionInProse reads a schema version a published page names in
// a sentence, where the anchored shape of a version cannot be used.
var contractSchemaVersionInProse = regexp.MustCompile(`schema version ([0-9]+\.[0-9]+\.[0-9]+)`)

// TestContractPagesNameNoSchemaVersionTheContractRefuses pins the other half of
// the version pass: a page that states the report shape by naming its schema
// version states one this contract admits, so a page cannot describe a shape the
// contract no longer publishes.
func TestContractPagesNameNoSchemaVersionTheContractRefuses(t *testing.T) {
	contract := loadContract(t)
	pages := map[string]string{}
	err := fs.WalkDir(spec.Contract, "contract", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		data, err := fs.ReadFile(spec.Contract, p)
		if err != nil {
			return err
		}
		pages[p] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("Setup: fs.WalkDir(contract): %v", err)
	}
	for _, page := range []string{docsKindsPage, docsExemptionsPage, docsExitCodesPage} {
		pages[page] = docsPage(t, page)
	}
	if len(pages) == 0 {
		t.Fatalf("Setup: pages naming a schema version = 0, want the published pages")
	}
	for _, p := range slices.Sorted(maps.Keys(pages)) {
		t.Run(caseName(strings.TrimSuffix(p, path.Ext(p))), func(t *testing.T) {
			for _, match := range contractSchemaVersionInProse.FindAllStringSubmatch(pages[p], -1) {
				if !slices.Contains(contract.SchemaVersions, match[1]) {
					t.Errorf("Documentation(%s) names schema version %q, want one of the versions %s admits, %v", p, match[1], contractPath, contract.SchemaVersions)
				}
			}
		})
	}
}

// contractBaselinePath is the committed record of the closed vocabularies at the
// published contract version, which is what makes that version's own rule
// enforceable for the part of a contract change that is a set.
const contractBaselinePath = "testdata/contract-baseline.json"

// contractBaselineDocument mirrors testdata/contract-baseline.json; an unknown
// key fails the decode.
type contractBaselineDocument struct {
	Description      string   `json:"description"`
	ContractVersion  string   `json:"contract_version"`
	SchemaVersions   []string `json:"schema_versions"`
	KindCodes        []string `json:"kind_codes"`
	RetiredCodes     []string `json:"retired_codes"`
	ExemptionClasses []string `json:"exemption_classes"`
	SymbolKinds      []string `json:"symbol_kinds"`
	ExitCodes        []int    `json:"exit_codes"`
}

// loadContractBaseline decodes the committed baseline, failing the test on any
// setup error and on an empty set, because an empty set pins nothing.
func loadContractBaseline(t *testing.T) contractBaselineDocument {
	t.Helper()
	data, err := os.ReadFile(contractBaselinePath)
	if err != nil {
		t.Fatalf("Setup: os.ReadFile(%q): %v", contractBaselinePath, err)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var doc contractBaselineDocument
	if err = dec.Decode(&doc); err != nil {
		t.Fatalf("Setup: decoding %s: %v", contractBaselinePath, err)
	}
	sets := map[string]int{
		"schema_versions":   len(doc.SchemaVersions),
		"kind_codes":        len(doc.KindCodes),
		"retired_codes":     len(doc.RetiredCodes),
		"exemption_classes": len(doc.ExemptionClasses),
		"symbol_kinds":      len(doc.SymbolKinds),
		"exit_codes":        len(doc.ExitCodes),
	}
	for _, name := range slices.Sorted(maps.Keys(sets)) {
		if sets[name] == 0 {
			t.Fatalf("Setup: %s records no %s, want the published set", contractBaselinePath, name)
		}
	}
	return doc
}

// TestContractVersionMovesWithEveryClosedVocabulary enforces the rule
// contract.json states for itself and could not check: the version moves when a
// document under contract/ changes in a way an implementation can observe. The
// baseline records every closed vocabulary at the version it names, so a code, a
// class, a subject kind, an exit code or an admitted report shape added or retired
// while the published version stays where the baseline left it fails here, and
// moving the version and rewriting the baseline in one change is the green path.
//
// The sets are the observable part of a contract change that a comparison can
// decide. An observable change no set holds, a mechanism sentence for example,
// moves the version and leaves the baseline alone, which is why this is an
// equality over vocabularies rather than a digest of the tree: a digest would make
// a description a version bump.
func TestContractVersionMovesWithEveryClosedVocabulary(t *testing.T) {
	contract := loadContract(t)
	baseline := loadContractBaseline(t)

	if baseline.ContractVersion != contract.ContractVersion {
		t.Fatalf("%s records contract_version %q and %s publishes %q: the two move in one change, so rewrite %s for the published version",
			contractBaselinePath, baseline.ContractVersion, contractPath, contract.ContractVersion, contractBaselinePath)
	}

	kinds := loadKinds(t)
	live := make([]string, len(kinds.Kinds))
	for i, row := range kinds.Kinds {
		live[i] = row.Code
	}
	retired := make([]string, len(kinds.Retired))
	for i, row := range kinds.Retired {
		retired[i] = row.Code
	}
	classes := make([]string, len(loadExemptions(t).Exemptions))
	for i, class := range loadExemptions(t).Exemptions {
		classes[i] = class.Class
	}
	table := mustLoadExitCodes(t)
	codes := make([]int, len(table.ExitCodes))
	for i, row := range table.ExitCodes {
		codes[i] = row.Code
	}

	for _, tc := range []struct {
		file string
		name string
		got  []string
		want []string
	}{
		{name: "schema_versions", file: contractPath, got: contract.SchemaVersions, want: baseline.SchemaVersions},
		{name: "kind_codes", file: kindsPath, got: live, want: baseline.KindCodes},
		{name: "retired_codes", file: kindsPath, got: retired, want: baseline.RetiredCodes},
		{name: "exemption_classes", file: exemptionsPath, got: classes, want: baseline.ExemptionClasses},
		{name: "symbol_kinds", file: findingSchemaPath, got: enumAt(loadFindingSchema(t), "properties/symbol/properties/kind"), want: baseline.SymbolKinds},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !slices.Equal(tc.got, tc.want) {
				t.Errorf("%s declares %s %v and %s records %v at contract_version %q: a value added or retired moves the version, so move it in %s and rewrite %s in one change",
					tc.file, tc.name, tc.got, contractBaselinePath, tc.want, contract.ContractVersion, contractPath, contractBaselinePath)
			}
		})
	}
	t.Run("exit_codes", func(t *testing.T) {
		if !slices.Equal(codes, baseline.ExitCodes) {
			t.Errorf("%s declares exit_codes %v and %s records %v at contract_version %q: a value added or retired moves the version, so move it in %s and rewrite %s in one change",
				exitCodesPath, codes, contractBaselinePath, baseline.ExitCodes, contract.ContractVersion, contractPath, contractBaselinePath)
		}
	})
}
