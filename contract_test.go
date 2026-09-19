package spec_test

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"maps"
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
