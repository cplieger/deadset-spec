// Package spec carries the deadset contract, the conformance corpus, the
// published test vectors and the example documents as embedded file trees, so a
// Go program can pin them at a module version. It is a data carrier: no deadset analyzer or
// orchestrator executes anything in this module, and a helper that interprets
// a document belongs to the product that reads it.
package spec

import "embed"

// Contract holds the contract/ tree: the versioned JSON documents, JSON
// Schemas and grammar pages every analyzer implements.
//
//go:embed contract
var Contract embed.FS

// Corpus holds the corpus/ tree: the conformance fixtures and expectation
// files every analyzer passes before it releases.
//
//go:embed corpus
var Corpus embed.FS

// Vectors holds the vectors/ tree: the published merge and configuration
// vectors an implementation is tested against.
//
//go:embed vectors
var Vectors embed.FS

// Examples holds the examples/ tree: the finding and report documents that
// illustrate the schemas, and the negative documents a conforming validator
// refuses.
//
//go:embed examples
var Examples embed.FS
