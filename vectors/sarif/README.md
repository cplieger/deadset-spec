# SARIF vectors

One case per directory, named for what it establishes. A case holds a report, the target files its results name and the outcome: the SARIF document the report renders as, or the exit code the rendering ends with. The mapping the cases exercise is the one [`sarif.md`](../../contract/grammar/sarif.md) states.

## The files of a case

| File | What it holds |
| --- | --- |
| `report.json` | the report rendered, an instance of [`report.schema.json`](../../contract/report.schema.json): an analyzer's own report, or a merged report |
| `inputs/` | for a merged report, the report of every analyzer it names in `merged_from`, one file each; a run reads its languages and its totals from the input report whose `analyzer.name` is the run's |
| `sources.json` | the target files the line fingerprints read: `files` maps each target-relative path to the file's content, and `description` says what the files are |
| `expected.json` | the SARIF document, where the report renders |
| `expected_exit` | the exit code, one integer, where the rendering fails: 3 |

A case holds `report.json`, `sources.json` and exactly one of `expected.json` and `expected_exit`, and `inputs/` exactly when the report is a merged one.

## Running a case

Write every file of `sources.json` under an empty directory, at its path, as the target root. Render `report.json` as SARIF against that root, reading the input reports where the case has them. Where the rendering fails, the outcome is 3. Otherwise decode the document written and `expected.json`, and compare the two values: an object's members by name in any order, an array's elements in order, a string after its escapes are read. The rule texts of the expected documents are those of the `kinds.json` this repository holds.

## What the cases establish

| Case | What it establishes | Outcome |
| --- | --- | --- |
| `findings-and-a-stale-suppression` | An analyzer's run: its rules, a finding, a finding whose write positions become related locations linked from the message, and a stale suppression at its own site with no `properties` and a region ending on its own line. | rendered |
| `implementations-and-component-members` | Related locations in their order: an interface's implementations, then the members of a component listed in full, the finding's own symbol left out. | rendered |
| `line-fingerprints` | `primaryLocationLineHash` over the page's five-line file, over two hundred identical lines where the counter and the sentinel enter the windows, and over CR LF, CR then a space then LF, and CR CR LF line ends. | rendered |
| `line-past-the-end` | A finding on a line past the file's last fails the rendering. | 3 |
| `merged-runs-and-totals` | A merged log: one run per `merged_from` entry in bytewise order of name, each with its input report's languages and totals, the merge's own run for the record that names no analyzer, and the merged totals under the log's `properties.totals`. | rendered |
| `merged-without-a-record-of-its-own` | A merged log with no record outside the analyzers' runs holds no run of the merge's own, and an input report with no record still has its run, with an empty `results`. | rendered |
| `message-with-line-separators` | A message holding U+2028, U+2029, `<` and `&` keeps each character in the decoded value, however the document escapes it. | rendered |
| `path-segments-encoded` | Each path segment percent-encoded byte by byte: a colon in the first segment written `%3A` and in a later one as itself, a space, a two-byte character, `,`, `(`, `)` and `%` encoded, the characters the page keeps written as themselves, and a message link naming the path unencoded. | rendered |
| `record-naming-no-run` | A merged record whose `analyzer` names no `merged_from` entry fails the rendering. | 3 |
| `related-locations-capped` | A finding with 101 write positions carries the first 100 as related locations and links each of them. | rendered |
| `withheld-line` | A report whose minimum confidence withheld findings at two confidences carries the withheld line in `properties.withheld` beside the totals it is rendered from. | rendered |
