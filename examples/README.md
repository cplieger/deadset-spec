# Examples

Documents that are instances of the contract's JSON Schemas, and documents that are not. This
repository's own test suite validates every one of them, so a schema and the shapes it admits are
checked together.

## Layout

```text
examples/
  findings/<kind>.json    one finding per issue-kind family, an instance of contract/finding.schema.json
  reports/<state>.json    one report per envelope state, an instance of contract/report.schema.json
  negatives/<name>.json   a refused document, each violating exactly one constraint
  negatives/index.json    one row per refused document: its schema, its constraint, its instance path
```

## findings/

One document per family of the code space, named for the `kind` field it carries. The eight
families are the live ranges of [`../contract/kinds.json`](../contract/kinds.json), so a family
gains an example when it gains a kind, and every example carries the `details` its own code
requires.

A finding here is a complete finding: every field
[`../contract/finding.schema.json`](../contract/finding.schema.json) requires, the optional fields
a real analyzer writes where the subject has them, and values drawn from the contract's
vocabularies. The module and package paths are neutral: `example.com/app` for Go, `@example/app`
for TypeScript.

## reports/

One document per state a report envelope can be in: `clean.json` for a run with no findings,
`findings.json` for a run that reports some, `pending.json` for a run holding an edge evaluation
whose state is `dead`, `stale-suppressions.json` for a run whose suppressions no longer match,
`declared-gaps.json` for a run that declines capabilities the corpus covers,
`configuration-not-built.json` for a run that derived a configuration from the tree, could not
build it and dropped it from the matrix, and `merged.json` for the report a merge writes over two
analyzers' reports.

Every finding inside a report here is also an instance of the finding schema on its own, and every
count in `totals` is the count of the array it describes.

A report a product commits as a fixture belongs in this directory too, so it is validated where
the schema lives rather than in the product that wrote it.

## negatives/

Each document is refused by the schema `index.json` names for it, at the instance location and by
the constraint that row names, and by nothing else. A document that stops being refused, or that
is refused somewhere else, is a document whose defect has drifted away from the constraint it
exists to exercise.

Two refusals of the contract are outside a schema's reach and are not here. A report whose
`analyzer.conformance.result` is `fail` is a valid instance that a merge refuses before it runs, and
a report whose `schema_version` falls outside the range a caller accepts is a valid instance a merge
refuses on admission. Both are decisions of the merge, which
[`../contract/grammar/merge.md`](../contract/grammar/merge.md) states.
