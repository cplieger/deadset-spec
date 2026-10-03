# What the contract defines

This page names every part of the deadset contract and the file under `contract/` that states it. It is for a developer implementing an analyzer, a merge or a tool that reads deadset reports.

The contract is versioned as one unit, apart from every analyzer's version and from this repository's release tags. `contract/contract.json` carries the contract version, the report schema versions an analyzer may write and the platforms a released analyzer supports. The minor version moves for every change to a document under `contract/` that an implementation can observe. The major version moves only when a mechanism text changes so that an implementation that conformed before the change can fail a conformance run. Every analyzer names the contract version it implements in its report and in its resolved configuration.

## Vocabularies

- The issue kinds, in `contract/kinds.json`. Every kind has a code, the prefix `DS` and four digits, grouped by family into numbered ranges. A code renders identically in a text output line, an ignore entry, a configuration key and a SARIF rule identifier, and a retired code stays retired for the life of the code space. [Issue kinds](kinds.md) is the reference page.
- The exemption classes, in `contract/exemptions.json`. Each is a reason an unreferenced symbol is still live, such as a method that satisfies an interface. [Exemption classes](exemptions.md) is the reference page.
- The exit codes, in `contract/exit-codes.json`. Code 0 is a clean report, and 1 is findings at or above the failing severity or a stale suppression. Code 2 is a usage error, and 3 is a failure before any verdict. Code 4 is a report holding a finding whose cross-language reference is still unresolved. [Exit codes](exit-codes.md) is the reference page.

## Documents

- The finding schema, `contract/finding.schema.json`, and the report schema, `contract/report.schema.json`, so every analyzer writes the same JSON object shape.
- The configuration document, `contract/config.schema.json`. It holds a closed key list, the precedence between the repository file, the central file and the invocation's flags, and the resolved configuration a run prints. The vectors under `vectors/config/` test a resolution against declared data.
- The scope document, `contract/scope.schema.json`. It names the target an analysis reports on and the consumers whose references count against it. The product that invokes the analyzer writes it, and the analyzer reads it.
- The describe document, `contract/describe.schema.json`. It is what an analyzer states about itself before any analysis runs, and the invoking product reads it to admit or refuse the analyzer.

## Grammars and rules

- The analysis rules, `contract/grammar/analysis.md`. They state what the program is, component files and workspace packages included, and which of it is test code. They also state the roots an analyzer marks beside the configured ones, what a type error withholds, the setup failures that end a run with the fix named, and the notes a report carries.
- The suppression grammar, `contract/grammar/suppression.md`. It has two mechanisms, an inline directive on the line above a declaration and an ignore-file entry scoped to one symbol in one file. A baseline uses the same row shape to record existing findings, so a run fails only on new ones, and it adjudicates nothing. Each suppression carries a reason, and a suppression that matches nothing is itself reported. The vectors under `vectors/baseline/` test the baseline.
- The symbol references, `contract/grammar/symbol-ref.md`. A reference names one declaration in one language and carries no line number, so it stays valid across edits above the declaration.
- The text-line format, `contract/grammar/text-line.md`. Each line starts with the position as `path:line:col`, so one search expression matches the output of every analyzer.
- The SARIF mapping, `contract/grammar/sarif.md`, with vectors under `vectors/sarif/`.
- The template rendering, `contract/grammar/template.md`. It states the template language and the data model a user's template renders a report through, with vectors under `vectors/template/`.
- The merge of several reports into one, `contract/grammar/merge.md`, stated as an algorithm with a deterministic order. The vectors under `vectors/merge/` test a merge implementation against declared data rather than against another implementation. [Cross-language edges](edges.md) explains the part of the merge that pairs symbols across languages.

Three case files in `contract/grammar/` hold accepted and refused inputs, each with its reason. `suppression-corpus.json` covers the suppression grammar, `symbol-ref-corpus.json` the symbol references, and `pattern-corpus.json` the patterns a configured root may use.
