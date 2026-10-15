package corpus

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"sync"

	spec "github.com/cplieger/deadset-spec/v8"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

// schemaBase is the base URI every embedded schema is registered under, so a
// cross-file reference resolves to its embedded sibling.
const schemaBase = "mem:///"

var schemas sync.Map

// Schema compiles the JSON Schema at an embedded path of the spec module once per
// path, "contract/report.schema.json" or "corpus/expect.schema.json".
func Schema(name string) (*jsonschema.Schema, error) {
	if held, ok := schemas.Load(name); ok {
		schema, _ := held.(*jsonschema.Schema)
		return schema, nil
	}
	compiler := jsonschema.NewCompiler()
	found := false
	for _, tree := range []fs.FS{spec.Contract, spec.Corpus} {
		added, err := addSchemas(compiler, tree, name)
		if err != nil {
			return nil, err
		}
		found = found || added
	}
	if !found {
		return nil, fmt.Errorf("no schema is embedded at %q", name)
	}
	schema, err := compiler.Compile(schemaBase + name)
	if err != nil {
		return nil, err
	}
	held, _ := schemas.LoadOrStore(name, schema)
	schema, _ = held.(*jsonschema.Schema)
	return schema, nil
}

// addSchemas registers every schema of one tree under the base URI and reports
// whether the tree holds the one named.
func addSchemas(compiler *jsonschema.Compiler, tree fs.FS, name string) (bool, error) {
	found := false
	err := fs.WalkDir(tree, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".schema.json") {
			return err
		}
		data, err := fs.ReadFile(tree, p)
		if err != nil {
			return err
		}
		document, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		found = found || p == name
		return compiler.AddResource(schemaBase+p, document)
	})
	return found, err
}

// Validate holds one document to the embedded schema at name.
func Validate(name string, document []byte) error {
	schema, err := Schema(name)
	if err != nil {
		return err
	}
	instance, err := jsonschema.UnmarshalJSON(bytes.NewReader(document))
	if err != nil {
		return err
	}
	return schema.Validate(instance)
}

// ValidateReport holds one report to contract/report.schema.json.
func ValidateReport(document []byte) error {
	return Validate("contract/report.schema.json", document)
}
