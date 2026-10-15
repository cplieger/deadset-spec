// Package spec carries the deadset contract, the conformance corpus, the
// published test vectors and the example documents as embedded file trees, so a
// Go program can pin them at a module version. It decides no analysis rule: an
// analyzer's own rules belong to the analyzer, and code shared by the products
// lives in this module's other packages.
package spec

//go:generate go run ./cmd/deadset-gen

import "embed"

// Contract holds the contract/ tree: the versioned JSON documents, JSON
// Schemas and grammar pages every analyzer implements.
//
//go:embed contract
var Contract embed.FS

// Corpus holds the corpus/ documents and fixtures: the conformance fixtures and
// expectation files every analyzer passes before it releases.
//
//go:embed corpus/*.json corpus/README.md all:corpus/fixtures
var Corpus embed.FS

// Vectors holds the vectors/ tree: the published merge, configuration,
// template, SARIF and baseline vectors an implementation is tested against.
//
//go:embed vectors
var Vectors embed.FS

// Examples holds the examples/ tree: the finding and report documents that
// illustrate the schemas, and the negative documents a conforming validator
// refuses.
//
//go:embed examples
var Examples embed.FS
