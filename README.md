# deadset-spec

[![Go Reference](https://pkg.go.dev/badge/github.com/cplieger/deadset-spec/v6.svg)](https://pkg.go.dev/github.com/cplieger/deadset-spec/v6) [![Go version](https://img.shields.io/github/go-mod/go-version/cplieger/deadset-spec)](https://github.com/cplieger/deadset-spec/blob/main/go.mod)

deadset-spec is the contract every deadset dead-code analyzer implements, and the conformance corpus each one passes before it releases.

It holds data and documentation, with a Go test suite that checks them, and no analyzer runs anything here. You can write a conforming analyzer for another language from this repository alone. The Go package imports only the standard library, needs Go 1.27.1 or later and is licensed under Apache-2.0.

## Why use it

deadset-spec is built for developers who write a deadset analyzer, in Go, TypeScript or another language. deadset finds declarations nothing reaches, exports nothing outside the package uses, files nothing imports and dependencies nothing requires. It runs only analyzers that pass this corpus.

- The contract defines 32 issue kinds. Each has a code, `DS` and four digits, and a retired code is never given to another kind.
- Every document an analyzer reads or writes has a JSON Schema, and `examples/` holds valid and refused documents for a decoder's tests.
- The merge, configuration, template, SARIF and baseline rules ship with input and expected-output vectors.
- The corpus has fixtures for both languages and fixtures for one, and two analyzers must agree wherever both answer an expectation.
- The contract has its own version. A rule change that can make a passing analyzer fail moves its major version, and every other change moves its minor.

## Install

```sh
go get github.com/cplieger/deadset-spec/v6@latest
```

## Usage

A Go analyzer requires the module from its tests and reads the embedded files with `io/fs`. The first test below fails when the vocabulary file is missing. The second walks the corpus, where each directory under `corpus/fixtures/` is one fixture. A runner reads the fixture's `expect.json` and writes the fixture's copy for its language, called a rendering, to a temporary directory. It then analyzes that directory as it would a real repository and compares the report with the expectations.

```go
import (
    "io/fs"
    "path"
    "testing"

    "github.com/cplieger/deadset-spec/v6"
)

func TestKindsAreCurrent(t *testing.T) {
    data, err := fs.ReadFile(spec.Contract, "contract/kinds.json")
    if err != nil {
        t.Fatalf("fs.ReadFile(Contract, kinds.json) = %v, want the document present", err)
    }
    _ = data // decode it and compare it with the kinds this analyzer implements
}

func TestCorpus(t *testing.T) {
    entries, err := fs.ReadDir(spec.Corpus, "corpus/fixtures")
    if err != nil {
        t.Fatalf("fs.ReadDir(Corpus, corpus/fixtures) = %v, want the fixture list", err)
    }
    for _, e := range entries {
        expect, err := fs.ReadFile(spec.Corpus, path.Join("corpus/fixtures", e.Name(), "expect.json"))
        if err != nil {
            t.Fatalf("fs.ReadFile(Corpus, %s/expect.json) = %v, want the expectations", e.Name(), err)
        }
        _ = expect // skip a fixture with no go.txtar, extract it, run the analyzer, compare
    }
}
```

A TypeScript analyzer clones this repository at a release tag instead, because no npm package is published. [The corpus format](corpus/README.md) states every rule a runner follows.

## API

- `spec.Contract`, `spec.Corpus`, `spec.Vectors` and `spec.Examples` are four `embed.FS` values holding the `contract/`, `corpus/`, `vectors/` and `examples/` trees at the module version. Paths inside them start with the directory name.
- `examples/` holds one finding per kind family, one report per report state, scope and describe documents, and refused documents with an index naming the constraint each one breaks.
- Nothing else is exported. Code that interprets a document belongs to the analyzer that reads it.

The generated reference is on [pkg.go.dev](https://pkg.go.dev/github.com/cplieger/deadset-spec/v6).

## The contract

`contract/` holds JSON documents, JSON Schemas and grammar pages. `contract/contract.json` carries the contract version, the report schema versions an analyzer may write and the platforms a released analyzer supports, which is Linux only. Every analyzer names the contract version it implements in its report.

The contract defines the issue kinds, the exemption classes, the exit codes and every document an analyzer reads or writes. It also states the analysis rules, the suppression grammar, the symbol references, the output formats and the merge of several reports into one. [What the contract defines](docs/contract.md) names the file that states each part.

A kind enters the vocabulary only when it passes two tests. What it reports must be dead code, a declaration that can be deleted or a visibility that can be narrowed with no change in behavior. It must also be decided exactly from type information and the reference graph, never by searching text. A finding that fails the first test belongs to a linter or a reviewer. A kind that fails the second is not shipped, not even disabled by default, because a kind expected to be noisy should not exist.

## The conformance corpus

`corpus/` holds 117 fixture projects. For each one, an `expect.json` file names in language-neutral terms what an analyzer must and must not report for each issue kind and each exemption class. Of these, 31 apply to both languages, 34 to Go only and 52 to TypeScript only. Every analyzer runs the corpus before each of its releases.

An analyzer records each capability it does not implement as a declared gap in its committed `conformance.json`, and its runner writes `conformance-results.json`. Both follow the schemas in `corpus/`. An expectation that is neither answered nor declared fails the analyzer. Where two analyzers answer the same expectation, they must agree on the code, the confidence and what a suppression does. Before any analysis, the product that runs the analyzers, such as deadset, reads the describe document each analyzer prints about itself. It runs an analyzer only when that document records a corpus result of `pass`.

A fixture ships one rendering per language. A Go rendering is one [`go.txtar`](https://pkg.go.dev/golang.org/x/tools/txtar) archive holding the target module and its consumer modules as sections, so no fixture file is ever compiled as part of this module. A TypeScript rendering is a directory of real files, because a package needs a real `package.json` on disk to resolve modules. Each rendering's `fixture.json` maps every name the expectation file uses to a file and a line in that rendering.

## Related projects

- [deadset](https://github.com/cplieger/deadset) runs both analyzers, resolves the references that cross the language boundary and merges their reports into one. It is the program to run to find dead code in a repository.
- [deadset-go](https://github.com/cplieger/deadset-go) analyzes a Go module.
- [deadset-ts](https://github.com/cplieger/deadset-ts) analyzes a TypeScript or JavaScript package.

## Documentation

- [What the contract defines](docs/contract.md) names each part of the contract and its file.
- [Issue kinds](docs/kinds.md) lists every kind with its rule, default and severity.
- [Exemption classes](docs/exemptions.md) states why an unreferenced symbol can stay live.
- [Exit codes](docs/exit-codes.md) states the five codes and which one wins.
- [Cross-language edges](docs/edges.md) pairs a Go symbol with a TypeScript one.
- [Migrating from punused](docs/migrating-from-punused.md) moves a `.punused-ignore` file to `deadset-ignore.json`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## Disclaimer

This project is built with care and follows security best practices, but it is intended for personal / self-hosted use. No guarantees of fitness for production environments. Use at your own risk.

This project was built with AI-assisted tooling using [Claude](https://claude.com), [GPT](https://openai.com), and [Kiro](https://kiro.dev). The human maintainer defines architecture, supervises implementation, and makes all final decisions.

## License

Apache-2.0. See [LICENSE](LICENSE).
