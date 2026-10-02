# Baseline vectors

One case per directory, named for what it establishes. A case holds the report of one round of a baseline write and the baseline document that round records, with no row recorded before it. The rule the cases exercise is the one the baseline section of [`suppression.md`](../../contract/grammar/suppression.md) states. A later round reads the recorded rows back and analyzes again, which no case can state without an analysis; the conformance corpus is where a run is pinned.

## The files of a case

| File | What it holds |
| --- | --- |
| `report.json` | the report of the round, an instance of [`report.schema.json`](../../contract/report.schema.json) that omits no finding |
| `expected.json` | the baseline document the round records |

## Running a case

Record the rows of one round from `report.json` and compare them with the `baseline` array of `expected.json`, row by row and in order, each row by its four members. The `description` of a baseline is not read by the analysis, and no case compares it.

## What the cases establish

| Case | What it establishes |
| --- | --- |
| `declarations-and-parts` | One row per finding in report order, each with the finding's code, its symbol reference and its path, and the reason naming the analyzer and its version: a declaration, a member, a narrowing finding, a write-only member and a part, the two parts of one declaration under one code recorded as one row. |
| `document-rows-left-out` | No row for a finding about a file, a dependency, a module directive, a suppression record or a configured root, and a row for the declaration beside them. |
| `nothing-to-record` | A round whose every finding is about a document row records an empty baseline. |
| `typescript-rows` | A TypeScript round: a row for a private class member, and none for a file or a configured declaration. |
