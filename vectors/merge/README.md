# Merge vectors

One case per directory, named for what it establishes. A case holds the reports a merge reads, the schema versions the caller accepts, and either the merged report the merge returns or the exit code with which the run ends before any merged report exists, so two implementations of the merge are tested against declared data rather than against each other. The algorithm the cases exercise is the one [`merge.md`](../../contract/grammar/merge.md) states.

## The files of a case

| File | What it holds |
| --- | --- |
| `inputs/<nn>-<language>.json` | one input report, an instance of [`report.schema.json`](../../contract/report.schema.json) |
| `accepted.txt` | the schema versions the caller accepts, one per line in ascending order |
| `expected.json` | the merged report, byte for byte |
| `expected_exit` | the exit code, one integer from [`exit-codes.json`](../../contract/exit-codes.json) |

An input file's name is a two-digit ordinal, a hyphen and the language of the analysis the report carries. The ordinal orders the files of the directory and means nothing to the merge, which returns the same report and the same exit code whatever order it reads its inputs in; a case whose two reports carry one language numbers them both for that language.

Every case carries `expected_exit`. A case carries `expected.json` only where the merge produces a merged report: a report the admission step refuses, and a pending finding that meets no evaluation of its edge's other side, each end the run with the failure code before a merged report exists, and those cases hold `expected_exit` alone.

## Running a case

Decode every report under `inputs/`, take the accepted range from `accepted.txt`, and merge. Compare the exit code with `expected_exit`, and where the case carries one, compare the bytes of the merged report with `expected.json`.

`expected.json` is compared as bytes, so it is written in the encoding a merged report is written in: UTF-8, a line feed as the line ending, two-space indentation, one member or array element to a line, an empty array or object on one line, one trailing newline, and the members of every object in the order `report.schema.json` declares them. Every array is in the order that schema states for it, which for `findings`, `stale_suppressions` and `declared_gaps` is the canonical key.

## What the merged reports hold

`analyzer` names the merging product, `merged_from` names every input report, and `schema_versions_accepted` holds the range `accepted.txt` names. `configurations` and `test_file_rules` are the union of the inputs' entries with a repeated entry appearing once, each in the order the envelope states. A finding, a stale suppression and a declared gap carry the name of the analyzer whose report carried them; a finding the merge itself emits carries no analyzer name, because no input report carried it. `totals` are recomputed over the merged arrays, except `suppressions_in_effect` and `reasons_recorded`, which count records no merged array holds and are the sums over the input reports. `deletable_lines` counts each component of the merged report once, so a component two findings root contributes its lines once.

## What the cases establish

| Case | What it establishes | Exit |
| --- | --- | --- |
| `conformance-not-passed` | An input report whose conformance result is not a pass. Admission refuses it, and the findings it holds are never presented. | 3 |
| `edge-absent-on-every-side` | Both sides of a declared edge hold an evaluation and every one of them is `absent`. The merge emits one stale-edge finding for the edge, at the fixed position of a document-level finding, and carries both evaluations. | 1 |
| `one-report` | One report and no edge. The merged report carries the input's finding under the analyzer that reported it, names that analyzer in `merged_from`, and recomputes the totals. | 1 |
| `pending-pair-dead` | Both sides of an edge are `dead`. Both findings are promoted, their components are unioned under the identifier the canonical order reaches first, and the finding carried from the same report as a unioned component takes that identifier and the summed counts. | 1 |
| `pending-pair-live` | One side is `dead` and the paired side is `live`. The pending finding is dropped, and the merged report holds the live evaluation, no finding and no pending count. | 0 |
| `pending-pair-unevaluated` | A `dead` evaluation whose edge no other report evaluates. The merge ends the run on the unresolved edge. | 3 |
| `schema-version-out-of-range` | A report whose schema version is outside the accepted range. Admission refuses it before any other step. | 3 |
| `stale-suppression-carried` | A report holding one stale suppression and no finding. The merge carries the record, and the run fails on it. | 1 |
| `two-analyzers-one-language` | Two analyzers claim one language and report one finding in common. Both records survive, adjacent and ordered by the name of the analyzer that carried each. | 1 |
| `two-reports-no-edges` | Two reports of two languages and no edge. Every finding is carried, and the merged order interleaves the two languages by path. | 1 |
