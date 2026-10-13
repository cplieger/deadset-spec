# Baseline vectors

One case per directory, named for what it establishes. A case holds the report of one round of a baseline write and the baseline document that round records, with no row recorded before it, and may hold the baseline document the write finds at the target root when it starts. The rule the cases exercise is the one the baseline section of [`suppression.md`](../../contract/grammar/suppression.md) states. A later round reads the recorded rows back and analyzes again, which no case can state without an analysis; the conformance corpus is where a run is pinned.

## The files of a case

| File | What it holds |
| --- | --- |
| `report.json` | the report of the round, an instance of [`report.schema.json`](../../contract/report.schema.json) that omits no finding |
| `existing.json` | the baseline document at the target root when the write starts; absent when the case starts from none |
| `expected.json` | the baseline document the round records |
| `expected-error.json` | the exit code the write ends the run with, in `exit_code`, where it refuses the document it finds |

A case carries exactly one of `expected.json` and `expected-error.json`, and a case carrying `expected-error.json` carries `existing.json`.

## Running a case

Place `existing.json`, where the case has one, at the target root as `deadset-baseline.json`. Record the rows of one round from `report.json` and write the document, then compare it with `expected.json`: its `baseline` array row by row and in order, each row by the members it holds, so a member a row leaves out is compared as left out, and then the written bytes with the bytes of `expected.json`, which take the form the baseline section fixes. The round's own scheme is the scheme of each language the report's `analyzer.languages` names, and a row of `existing.json` of another scheme is one the write keeps. The `description` of a baseline is not read by the analysis, and a case compares it only as part of the written bytes. A case carrying `expected-error.json` is refused: the run ends with its `exit_code` and leaves `deadset-baseline.json` holding the bytes of `existing.json`.

Two cases whose `existing.json` holds the same bytes, and whose reports come from analyzers of different languages, expect the same bytes, which is what lets the two analyzers of a target write its baseline in either order.

## What the cases establish

| Case | What it establishes |
| --- | --- |
| `backslash-in-a-file-name` | A TypeScript round whose finding sits in a file whose name holds a backslash: the row carries that path as the report spells it, the backslash a character of the name. |
| `declarations-and-parts` | One row per finding in report order, each with the finding's code, its symbol reference and its path, and the reason naming the analyzer and its version: a declaration, a member, a narrowing finding, a write-only member and a part, the two parts of one declaration under one code recorded as one row. |
| `document-rows-left-out` | No row for a finding about a file, a suppression record or a configured root, and a row for the declaration beside them and for each dependency row, a requirement and a replace directive. |
| `nothing-to-record` | A round whose every finding is about a document row records an empty baseline. |
| `typescript-parameters` | A TypeScript round with two unused parameters of one function on one line: each finding carries its parameter's own reference, so the round records two rows. |
| `typescript-rows` | A TypeScript round: a row for a private class member, and none for a file or a configured declaration. |
| `other-scheme-rows-kept` | A Go round over a document holding rows of both schemes: its own old row is replaced, and the TypeScript rows follow its own, in document order, each with exactly the members it held, one without a reason and one without a path. |
| `one-document-written-by-go` | A Go round over a document holding the TypeScript rows before the Go rows: the Go rows come first, and the document is the one `one-document-written-by-typescript` writes. |
| `one-document-written-by-typescript` | The TypeScript round over the same document: the Go rows are kept ahead of its own, so the document is the one the Go round writes. |
| `line-separator-written-by-go` | A Go round over a document whose TypeScript row holds U+2028 in its path, written as itself there: the written document escapes it, and is the one `line-separator-written-by-typescript` writes. |
| `line-separator-written-by-typescript` | The TypeScript round over the same document, whose own finding sits at that path: the document is the one the Go round writes. |
| `other-scheme-row-unreadable` | A Go round over a document whose TypeScript row carries a member no row declares: the write is refused with exit code 3 and the document stays as it is. |
| `own-scheme-row-unreadable` | A Go round over a document whose Go row carries a code outside the code form: the write is refused with exit code 3, although the write would replace that row. |
| `existing-document-not-a-baseline` | A TypeScript round over a document that holds no `baseline` array: the write is refused with exit code 3. |
