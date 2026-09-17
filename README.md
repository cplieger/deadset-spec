# deadset-spec

[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/PROJECT_ID/badge)](https://www.bestpractices.dev/projects/PROJECT_ID)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/cplieger/deadset-spec/badge)](https://scorecard.dev/viewer/?uri=github.com/cplieger/deadset-spec)

The contract every deadset dead-code analyzer implements, and the conformance corpus each one passes before it releases.

## What this is

deadset finds dead code in Go and TypeScript repositories: declarations nothing reaches, exports nothing outside the package uses, files nothing imports, dependencies nothing requires. It ships as three programs from three repositories. [deadset-go](https://github.com/cplieger/deadset-go) analyzes a Go module. [deadset-ts](https://github.com/cplieger/deadset-ts) analyzes a TypeScript or JavaScript package. [deadset](https://github.com/cplieger/deadset) runs both, resolves the references that cross the language boundary, and merges their reports into one.

This repository holds what those three programs agree on: data and documentation, plus a Go test harness that checks them. No analyzer executes anything here, so a third party can write a conforming analyzer for another language from this repository alone. The harness makes the repository the Go module `github.com/cplieger/deadset-spec`, and that module is also how a Go program pins the contract and the corpus at a version: it requires the module at a tag and reads the embedded files.

## Install

```sh
go get github.com/cplieger/deadset-spec@latest
```

The module is a test dependency for a Go analyzer that runs the corpus. A TypeScript analyzer clones the repository at a tag instead; no npm package is published.

## Usage

```go
import (
    "io/fs"
    "testing"

    "github.com/cplieger/deadset-spec"
)

func TestKindsAreCurrent(t *testing.T) {
    data, err := fs.ReadFile(spec.Contract, "contract/kinds.json")
    if err != nil {
        t.Fatalf("fs.ReadFile(Contract, kinds.json) = %v, want the document present", err)
    }
    // decode data and compare it with the kinds this analyzer implements
}
```

`fs.WalkDir(spec.Corpus, "corpus/fixtures", ...)` lists the fixtures; each rendering is extracted to a temporary directory before analysis.

## API

- `spec.Contract`, `spec.Corpus`, `spec.Vectors`, `spec.Examples`: four `embed.FS` values holding the `contract/`, `corpus/`, `vectors/` and `examples/` trees at the module version. Paths inside them start with the directory name.
- `examples/` holds one finding document per kind family, one report per envelope state, and refused documents with an index naming what each one violates, so an analyzer can test its own decoder and its own schema check against declared data.
- Nothing else is exported. A helper that interprets a document belongs to the analyzer that reads it.

## The contract

`contract/` holds JSON documents, JSON Schemas and grammar pages, versioned as one unit independently of every analyzer. `contract/contract.json` carries the contract version, the report schema versions an analyzer may emit and the platforms a released analyzer binary supports; an analyzer names the contract version it implements in its report and in its resolved configuration. The contract defines:

- The issue-kind vocabulary. Every kind has a code, the prefix `DS` and four digits, grouped by family into numbered ranges. A code renders identically in a text output line, an ignore entry, a configuration key and a SARIF rule identifier, and a retired code stays retired for the life of the code space.
- The exemption classes: the reasons an unreferenced symbol is still live, such as a method that satisfies an interface.
- The finding schema and the report schema, so every analyzer emits the same JSON object shape and the same SARIF mapping.
- The suppression grammar: an inline directive on the line above a declaration, an ignore-file entry scoped to one symbol in one file, and a baseline row of the same shape, each carrying a reason, plus the rule that a suppression matching nothing is itself reported.
- The exit-code table, `contract/exit-codes.json`: 0 clean, 1 findings at or above the failing severity or a stale suppression, 2 usage error, 3 load or type-check failure, 4 a report holding a finding whose cross-language reference is still unresolved.
- The text-line format, position first as `path:line:col`, so one grep expression matches the output of every analyzer.
- The merge of several reports into one, stated as an algorithm with a deterministic order, together with published input and output vectors so a merge implementation is tested against declared data rather than against another implementation.
- The configuration document, `contract/config.schema.json`: a closed key list, the precedence between the repository file, the central file and the invocation's flags, and the resolved configuration a run prints, with published vectors so a resolution is tested against declared data too.

Each of those formats is stated in full under [`contract/grammar/`](contract/grammar) or in the schema that carries it. The vocabularies also have reference pages, which is where a reader starts:

- [Issue kinds](docs/kinds.md): every kind with its code, rule, languages, default, severity and fixability, the confidence ceiling this contract version gives every kind, and every retired code.
- [Exemption classes](docs/exemptions.md): every class with its detection rule, what it retains, its per-language mechanism and its answer on TypeScript member visibility.
- [Exit codes](docs/exit-codes.md): the five codes, what decides each and which one wins when more than one applies.
- [Cross-language edges](docs/edges.md): the edges document, the three states a side takes, the pending finding and what a merge does with each pairing.
- [Migrating from punused](docs/migrating-from-punused.md): the pass from a `.punused-ignore` file to `deadset-ignore.json`, with the `EU1001` and `EU1002` mapping.

## What counts as an issue kind

A kind enters the vocabulary only when it passes both tests:

1. What it reports is dead code: a declaration that can be deleted, or a visibility that can be narrowed, with no change in behavior.
2. It is decided exactly from type information and the reference graph, never by searching text.

A finding that fails the first test is a bug, a missing declaration or a design question, and belongs to a linter or a reviewer. A kind that fails the second test is not shipped, not even disabled by default, because a kind whose author expects noise should not exist.

## The conformance corpus

`corpus/` holds fixture projects per language and, per fixture, a language-neutral expectation file naming what an analyzer must report and must not report for each issue kind and each exemption class. Every analyzer runs the corpus as a condition of its own release. Where an analyzer does not implement a capability an expectation covers, it records a declared gap in its conformance report; an expectation that is neither answered nor declared fails the analyzer. Where two analyzers answer the same expectation, the corpus requires them to agree on the code, the confidence and the suppression behavior.

A fixture ships one rendering per language. A Go rendering is one `go.txtar` archive holding the target module and its consumer modules as sections, so the archive never reads as a Go module of this repository. A TypeScript rendering is a directory holding the same projects as files, because a package needs a real `package.json` on disk to resolve modules. Both carry the same `fixture.json` manifest, which maps each symbol name the expectation file uses to a file and line in that rendering.

## Contributing

Issues and pull requests are welcome. The general guidelines live in [cplieger/.github](https://github.com/cplieger/.github/blob/main/CONTRIBUTING.md).

## Disclaimer

This project is built with care and follows security best practices, but it is intended for personal / self-hosted use. No guarantees of fitness for production environments. Use at your own risk.

This project was built with AI-assisted tooling using [Claude](https://claude.com), [GPT](https://openai.com), and [Kiro](https://kiro.dev). The human maintainer defines architecture, supervises implementation, and makes all final decisions.

## License

Apache-2.0. See [LICENSE](LICENSE).
