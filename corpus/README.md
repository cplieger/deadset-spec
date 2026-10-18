# Conformance corpus

Each directory under `fixtures/` is one fixture: an `expect.json` file and one rendering per language it lists. A Go rendering is one `go.txtar` archive, a Protocol Buffers rendering is one `proto.txtar` archive, and a TypeScript rendering is a `ts/` directory. The expectation file names, in language-neutral terms, what an analyzer must and must not report for each logical name, and each rendering's `fixture.json` binds those names to a file and a line.

## Where the rules are

- [`corpus.json`](corpus.json) states the layout, the closed vocabularies an expectation draws from, the subject shapes and the runner: how a rendering is loaded and configured, how a logical name resolves, how a report answers each row, the closed world, the setup failures, the suppression phase, the capabilities a gap may name and the results.
- [`expect.schema.json`](expect.schema.json) and [`fixture.schema.json`](fixture.schema.json) shape the two files of a fixture.
- [`conformance.schema.json`](conformance.schema.json) shapes the declared gaps an analyzer commits, and [`conformance-results.schema.json`](conformance-results.schema.json) the results document a run writes.
- A fixture may cite the analysis rules it pins in `rules`, each by its id on [the analysis pages](../contract/grammar/analysis.md). A rule every language follows, once a fixture outside `pending` cites it, is cited by at least one fixture rendered in every language.
- A fixture marked `pending` states a rule no analyzer implements yet. It is answered as any other and reported as pending until the rule lands.

## Running it

The `corpus` Go package of this module is the one runner and checker, and `cmd/deadset-corpus` puts them on the command line:

- `deadset-corpus run --language=LANG --gaps=conformance.json -- ANALYZER` answers every fixture rendered in the language through the analyzer's `analyze` and `print-retained` verbs, and the facts document `analyze --facts` writes where the fixture names `facts`, and writes the results document. A product linked into its caller, as deadset's schema reader is, is answered through `corpus.Run` with an in-process `Analyzer` instead.
- `deadset-corpus check` holds every fixture to the format, the vocabularies and the rule-coverage rule above.
- `deadset-corpus agree FIRST SECOND` compares two analyzers' results on the expectations both answer.
