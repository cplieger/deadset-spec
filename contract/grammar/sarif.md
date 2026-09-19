# The SARIF 2.1.0 mapping

The SARIF format is the report shaped for upload to a code-scanning service: one [SARIF 2.1.0](https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/sarif-v2.1.0-errata01-os-complete.html) log, one run per analyzer, one result per finding. This page states the mapping object by object for an implementer who has no access to any existing implementation, names the two fingerprint keys and how each is computed, and states which SARIF properties are never emitted and why.

The mapping is written against [GitHub's supported-properties table](https://docs.github.com/en/code-security/code-scanning/integrating-with-code-scanning/sarif-support-for-code-scanning) as well as the OASIS schema, because a property the schema allows and the ingest ignores is a property that silently does nothing. GitHub's page closes its table with the sentence that the rest of the supported fields are ignored, so every property this page emits is either in that table or is emitted for a stated reason with the note that GitHub does not read it. The last section lists every emitted property beside the row that reads it.

## The document

```json
{
  "$schema": "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json",
  "version": "2.1.0",
  "runs": []
}
```

`$schema` is the OASIS URI of the 2.1.0 errata01 schema, fixed so that two producers write the same bytes; GitHub accepts any 2.1.0 schema URI. `version` is `2.1.0`. `runs` holds one run per analyzer report.

An analyzer writing its own SARIF produces a log with one run. The orchestrator writing the merged report produces one run per input report, in bytewise order of `analyzer.name`, each holding the findings and stale suppressions that report carried, and then one further run when the merged report holds a finding no input report carried. That happens for `DS1705`, which the merge emits (`merge.md`, step 5); the extra run's `tool.driver` is the orchestrator's own name and version, its `automationDetails.id` is `deadset/merge/`, and its `rules` list every live kind in `kinds.json`. The run a merged finding belongs to is the one whose `tool.driver.name` equals the analyzer that carried it, the field `report.schema.json` keeps on each record for the canonical key.

GitHub accepts at most 20 runs per file, 25,000 results per run (it displays the top 5,000) and 10 MB per gzip-compressed file. A configured maximum finding count is how a producer stays under those limits without silently truncating: the JSON report's `totals.omitted` names what the SARIF lacks.

## The run

| Property | Value |
| --- | --- |
| `tool.driver.name` | `analyzer.name` from the report, such as `deadset-go` or `deadset-ts`. |
| `tool.driver.version` | `analyzer.version`. |
| `tool.driver.semanticVersion` | `analyzer.version` again. GitHub prefers `semanticVersion` when both are present and other consumers read `version`. |
| `tool.driver.rules[]` | One rule per kind, [below](#the-rules). |
| `automationDetails.id` | `deadset/<language>/`, where `<language>` is the report's `analyzer.languages` entries joined with `+` in bytewise order, so `deadset/go/` for a Go analyzer and `deadset/ts/` for a TypeScript one. GitHub reads the text before the last `/` as the category and the empty remainder as no run identifier, and uses the category to tell one language's alerts from another's on the same commit. |
| `columnKind` | The SARIF name of the unit `finding.schema.json` fixes for `position.column`: `utf16CodeUnits` when the schema counts UTF-16 code units, `unicodeCodePoints` when it counts code points. SARIF admits no third value and requires the property on every run that holds a result (section 3.14.27), so the schema's unit is one of these two. GitHub does not read it. It is emitted because an absent `columnKind` defaults to `unicodeCodePoints`, and a producer counting UTF-16 code units would then mislabel every column after a character outside the Basic Multilingual Plane. |
| `originalUriBaseIds` | `{ "%SRCROOT%": { "description": { "text": "The target root, the directory the analyzer was run on." } } }`. The entry declares the base identifier every location uses and deliberately omits `uri`: SARIF section 3.14.14 permits the omission for exactly this case, producing deterministic output with no machine path in it. GitHub does not read it. |
| `results[]` | One result per finding, then one per stale suppression, [below](#the-results). |
| `properties.totals` | The report's `totals` object, unchanged. GitHub does not read it. It is the place a reader of the file sees `suppressions_in_effect`, `reasons_recorded` and `omitted`, the counts that describe what this document does not contain. |

Not emitted on the run: `invocations` (GitHub reads its working directory only to relativize absolute URIs, and every URI here is already relative; the array would otherwise carry the command line and the machine path), `artifacts`, `versionControlProvenance` (the analyzer reads no version control), `taxonomies`, `translations` and `policies`.

## The rules

`tool.driver.rules[]` holds one `reportingDescriptor` for every live kind in `kinds.json` whose `languages` includes the run's language, in bytewise order of `code`, whether or not the run holds a result for it and whether or not the configuration enabled it. A fixed rule list keeps `ruleIndex` stable across runs and configurations, which is the property GitHub's page asks of rules: information that changes when the tool changes, not when the code does.

| Property | Value |
| --- | --- |
| `id` | The kind's `code`, such as `DS1001`. The same string is the result's `ruleId`, the text line's code, an ignore entry's `code` and a configuration key. |
| `name` | The kind's `name`, such as `unused-exported`, which GitHub uses to filter alerts by rule. |
| `shortDescription.text` | The first sentence of the kind's `rule`: the text up to and including the first `.` that is followed by a space or ends the string. |
| `fullDescription.text` | The kind's `rule`, whole. Every rule text in `kinds.json` is under GitHub's 1024-character limit. |
| `help.text` | The kind's `rule`, then, when the kind carries a `precondition`, one blank line and the precondition. |
| `defaultConfiguration.level` | The kind's `default_severity` mapped by the [level table](#level-from-severity). This is the contract default; a result carries the run's configured severity in its own `level`. |
| `properties.precision` | The kind's `max_class` mapped: `certain` to `very-high`, `probable` to `high`, `possible` to `medium`. Every live kind in this Contract version is `certain`, so every rule reads `very-high`; the other two arms exist for the first kind that declares a lower ceiling. |
| `properties.problem.severity` | The kind's `default_severity` mapped: `deny` to `error`, `warn` to `warning`, `allow` to `recommendation`. GitHub combines it with `precision` to decide which alerts it shows by default. |

Not emitted on a rule: `helpUri` and `help.markdown` (GitHub shows `help.text` when the markdown is absent; a URI is not read), `properties.tags` (the family is recoverable from the code range), `properties.security-severity` (no kind is a security finding, and the property would make GitHub treat every alert as one), `defaultConfiguration.enabled`, `messageStrings` and `deprecatedNames`.

## The results

`results[]` holds one result per record of the report's `findings`, in report order, then one per record of `stale_suppressions`, in report order; the same sequence the text format prints. A stale suppression is a result under `ruleId` `DS1703` at level `error`, located at the suppression's own site (`text-line.md`, "Three cases with no special form"). No other record of the report becomes a result; the [omitted section](#what-is-never-emitted) says which and why.

| Property | Value |
| --- | --- |
| `ruleId` | The finding's `code`. The finding's `kind` is not repeated on the result; it is the `name` of the rule `ruleIndex` points at. |
| `ruleIndex` | The zero-based index of that code in this run's `rules[]`. |
| `level` | The finding's `severity` mapped by the [level table](#level-from-severity). |
| `message.text` | The finding's `message`, followed by the [related-location links](#related-locations) when the result has any. |
| `locations[]` | Exactly one location, the finding's `position`, [below](#the-location). GitHub reads only the first location of a result; SARIF allows more, and this format never has a second. |
| `relatedLocations[]` | The positions the finding names beyond its own, [below](#related-locations). Absent when the finding names none. |
| `partialFingerprints` | The two keys [below](#partial-fingerprints). |
| `properties` | Every field of the finding this page has not mapped above, under its schema name and with its value unchanged. At schema version 2.0.0 that is `language`, `symbol`, `reachability_class`, `confidence`, `liveness_relation`, `test_only`, `generated`, `component`, `retained_by`, `configurations`, `consumers_loaded`, `fixability` and `details`. A field the finding omits is absent from the bag, never written as a null or as an empty value. GitHub does not read the bag; it serves other SARIF consumers, a codemod and a reader of the file, and it changes nothing about the alert GitHub shows. |

Not emitted on a result: `kind` (the default `fail` is right for every result), `rule` (a `ruleId` with a `ruleIndex` locates the descriptor), `fixes` (the report is the interface for an edit, and no product edits source in this version), `codeFlows`, `stacks`, `taxa`, `baselineState`, `rank`, `occurrenceCount`, and `suppressions`, whose omission has [its own section](#the-omitted-suppression).

### The location

```json
{
  "physicalLocation": {
    "artifactLocation": { "uri": "catalog.go", "uriBaseId": "%SRCROOT%" },
    "region": { "startLine": 214, "startColumn": 6, "endLine": 231 }
  }
}
```

| Property | Value |
| --- | --- |
| `artifactLocation.uri` | The finding's `position.path`, a relative reference: the target-relative path with `/` separators, each segment percent-encoded as RFC 3986 requires for a path segment (a space becomes `%20`), and no leading `./`. |
| `artifactLocation.uriBaseId` | `%SRCROOT%`, the identifier declared in `originalUriBaseIds`. GitHub does not read it; it resolves a relative URI against the root of the repository being analyzed. |
| `region.startLine` | `position.line`. |
| `region.startColumn` | `position.column`, in the unit `columnKind` names. Every finding carries a column, so no clamp for a missing one is needed. |
| `region.endLine` | `position.end_line`, the last line of the declaration. |

`region.endColumn` is not emitted. The finding carries no end column, and SARIF section 3.30.8 defines an absent `endColumn` as one past the last character of `endLine`, which is the right extent for a declaration that is deletable whole. GitHub's table marks `endColumn` required, and its own examples on the same page omit it and omit `endLine`; the ingest displays a result from `startLine` alone.

One limit follows from the URI rule. GitHub resolves a relative URI against the repository root, so the SARIF resolves on GitHub only when the target root is the repository root. A target below the repository root, such as one package of a monorepo, produces paths GitHub cannot match to files; nothing in this mapping prefixes them, because the analyzer does not know where the repository root is.

### Related locations

A result carries a related location for every position the finding names beyond its own, in this order and numbered from 1 in this order:

1. Each entry of `details.implementations`, in report order, with `message.text` `implementation` (the `DS12xx` kinds).
2. Each entry of `details.write_positions`, in report order, with `message.text` `write` (`DS1301` and `DS1807`).
3. Each member of the finding's component other than the finding's own symbol, in the order the report lists them, with `message.text` `member`, when the finding carries the member list; it does so when the cascade output is set to full, under the field `finding.schema.json` names for it, and a finding without the list contributes no related location here.

Each related location is a `location` object with `id` (the 1-based number), `physicalLocation` in the shape above, and `message.text` as listed. At most 100 related locations are emitted, the first 100 in that order; GitHub rejects a result with more than 1,000 locations and includes 100 of them, and the JSON report stays complete whatever the cap removes.

GitHub shows a related location only when the result message links to it. So when a result has related locations, `message.text` is the finding's `message`, one space, and `"(see "` followed by one link per related location, `[<label> <path>:<line>:<col>](<id>)`, joined by `", "`, then `)`. The label is the location's `message.text`, the path is `position.path` unencoded, and the id is the related location's `id`. Rendered for a write-only member with two writes:

```text
private member is written and never read (see [write src/features/tabs/index.ts:1190:9](1), [write src/features/tabs/index.ts:1201:9](2))
```

A result with no related location carries `message` unchanged. A finding's `message` never ends with a period, and the appended clause is part of the same sentence, so GitHub's rule that it shows only the first sentence when space is short does not cut the links off.

### Partial fingerprints

```json
{
  "primaryLocationLineHash": "247cff8f02e0b919:1",
  "deadsetSymbolRef/v1": "d073714ada8cfcbee49bd5430446d6be7b837b6fd1fc34e6aa82be03b589c18d"
}
```

Two keys, each a versioned hierarchical string as SARIF section 3.27.17 asks, computed as follows.

**`primaryLocationLineHash`** is the key GitHub reads (its page: "Code scanning only uses the primaryLocationLineHash"), and the value is computed by the procedure in [`src/fingerprints.ts`](https://github.com/github/codeql-action/blob/main/src/fingerprints.ts) of GitHub's upload action, so that a file uploaded through the action or through the REST API fingerprints the same way. The action fills the key in when it is absent and leaves it alone when present, logging a warning when its own computation disagrees; a producer that follows the procedure exactly never triggers that warning. The procedure, over the file at `position.path`:

1. Decode the file as UTF-8 into a sequence of UTF-16 code units. An analyzer whose native unit is bytes or code points converts first. A file that is not valid UTF-8 has no defined value, because the two languages' decoders substitute for an invalid sequence differently; a Go source file with one does not compile, so no Go finding meets the case.
2. Walk the sequence and keep the significant units: drop every space (U+0020) and tab (U+0009); replace CR (U+000D) by LF (U+000A); drop an LF that immediately follows a CR, so CR LF counts once; keep everything else. Append one sentinel unit with the value 65535 after the last unit of the file.
3. A line starts at the first significant unit of the file and at every significant unit that follows an LF. Number the lines from 1 in that order. The sentinel starts one line of its own after a file that ends in LF; no finding refers to it.
4. The hash of a line is the polynomial hash of the 100 significant units beginning at its start, in unsigned 64-bit arithmetic with wraparound: `h = 0; for each of the 100 units u: h = h * 37 + u`. A position past the sentinel contributes 0. A rolling hash over a 100-unit window yields the same number.
5. Render `h` as lowercase hexadecimal with no leading zeros, then `:`, then the number of lines so far in this file, this one included, whose rendered hash is identical. The counter starts at 1 and disambiguates identical windows, such as repeated lines in a long run of identical lines.
6. The value for a result is the string computed for the line `position.line`.

A vector. The file below, stored with LF line endings and a trailing LF, is five lines long: the third holds a two-byte UTF-8 character, the fourth is indented, and the indentation may be a tab or spaces without changing any value below, because step 2 drops both.

```text
package fixture

func Ünused() {
    return
}
```

| Line | `primaryLocationLineHash` |
| --- | --- |
| 1 | `7a0e51a45e6d7320:1` |
| 2 | `3134adfd1bbad887:1` |
| 3 | `247cff8f02e0b919:1` |
| 4 | `58228fc5cbc49530:1` |
| 5 | `32dce9ccfdbc9d3e:1` |

A file of 200 lines each holding `y` renders line 1 as `43762f342805c306:1`, line 2 as `43762f342805c306:2` and line 151 as `43762f342805c306:151`; the last 49 lines differ because the sentinel and the zero padding enter their windows.

**`deadsetSymbolRef/v1`** is the key a baseline joins on and the one that survives a line move, which `primaryLocationLineHash` does not once the line's own text changes. Its value is the SHA-256 digest, as 64 lowercase hexadecimal digits, of the UTF-8 encoding of the finding's `code`, one LF (U+000A), and the finding's `symbol.ref` from `symbol-ref.md`. Neither a code nor a reference contains an LF, so the separator is unambiguous. For the code `DS1001` and the reference `go://example.com/fixture#Ünused`, the digest is `d073714ada8cfcbee49bd5430446d6be7b837b6fd1fc34e6aa82be03b589c18d`; for `DS1001` and `go://example.com/app#Catalog.ResolveAlias` it is `3969945e4504f5d8a52415a6f7b4f233d1ba4820a2fe24617a3611e2535d5e32`. The digest rather than the reference itself is the value so that every fingerprint has one length and one alphabet, and a consumer comparing fingerprints never parses a reference. GitHub does not read this key; it reads `primaryLocationLineHash` and no other.

Both keys are emitted on every result, a stale suppression included, whose `symbol.ref` is the reference the suppression names.

## Level from severity

| Finding `severity` | `results[].level` and `defaultConfiguration.level` | `properties.problem.severity` |
| --- | --- | --- |
| `deny` | `error` | `error` |
| `warn` | `warning` | `warning` |
| `allow` | `note` | `recommendation` |

The first column is the run's configured severity on a result and the kind's `default_severity` on a rule. A kind at `allow` produces no finding, so a result at `note` does not occur in the current configuration model; the arm is stated so that the mapping is total over the severity vocabulary.

## The omitted suppression

**A suppressed finding is absent from the SARIF document. The `suppressions` property is never emitted, on any result.**

SARIF 2.1.0 has a `suppressions` array (section 3.27.23) built for adjudicated results: a `suppression` object with `kind` `inSource` or `external` and a `justification`, which is what the mandatory `reason` on every ignore entry would fill. It is not used, for two reasons that hold together.

First, nothing exists to mark. A matched suppression marks its symbol live before the sweep (`suppression.md`), so the sweep produces no finding for that symbol and no finding for the symbols only it referenced. A suppressed finding does not exist in the report, and a SARIF result cannot be emitted for a finding that does not exist.

Second, the one consumer this format targets would publish it. GitHub's supported-properties table does not list `suppressions`, its page states that unlisted properties are ignored, and the upload action does not support the property. A result emitted with a `suppressions` entry would appear as an open code-scanning alert, which is the opposite of what an adjudication with a reason is for.

The omission is visible rather than silent. The report's `totals.suppressions_in_effect` and `totals.reasons_recorded` carry the counts, the summary line prints them, and this document carries the same `totals` object under `runs[].properties.totals`. A suppression that matched nothing is not omitted: it is a `DS1703` result like any other finding.

## What is never emitted

| Not in the document | Why |
| --- | --- |
| A suppressed finding, and the `suppressions` property | The section above. |
| A pending finding | It lives in an edge evaluation, is neither reported nor suppressed, and the run that holds one exits with code 4 to say the report is an input to a merge and not an answer. The SARIF a lone analyzer writes for such a report omits it, the same as the text format, and the exit code carries the warning. |
| A finding past the configured maximum count | `totals.omitted` in the report and in `runs[].properties.totals` names how many. |
| `region.endColumn` | The finding carries no end column; SARIF's default is the end of `endLine`, which is the declaration's extent. |
| `invocations`, `artifacts`, `versionControlProvenance` | Machine paths and command lines the report's determinism rule keeps out; nothing GitHub needs, since every URI is relative. |
| `fixes` | The report is the interface for an edit; no product edits source in this version. |
| `codeFlows`, `stacks`, `graphs`, `taxa`, `rank`, `baselineState` | No finding has a flow, a stack or a taxonomy, and the baseline is a document of the report, not a SARIF state. |
| `helpUri`, `help.markdown`, `properties.tags`, `properties.security-severity` | Not read, or read with a meaning that does not fit (`security-severity` marks a security alert). |

## Every emitted property, and the row that reads it

One row per property this mapping emits, with the row of GitHub's supported-properties table that reads it, the table's Required or Optional marker for that row, and `none` where GitHub reads no such property. A `none` row is emitted for the reason its section states, never because the ingest wants it.

| Property | GitHub reads it as |
| --- | --- |
| `$schema` | sarifLog `$schema`, Required |
| `version` | sarifLog `version`, Required |
| `runs[]` | sarifLog `runs[]`, Required |
| `runs[].tool.driver` | run `tool.driver`, Required |
| `runs[].tool.driver.name` | toolComponent `name`, Required |
| `runs[].tool.driver.version` | toolComponent `version`, Optional; not used when `semanticVersion` is present |
| `runs[].tool.driver.semanticVersion` | toolComponent `semanticVersion`, Optional |
| `runs[].tool.driver.rules[]` | toolComponent `rules[]`, Required |
| `rules[].id` | reportingDescriptor `id`, Required |
| `rules[].name` | reportingDescriptor `name`, Optional |
| `rules[].shortDescription.text` | reportingDescriptor `shortDescription.text`, Required |
| `rules[].fullDescription.text` | reportingDescriptor `fullDescription.text`, Required |
| `rules[].help.text` | reportingDescriptor `help.text`, Required |
| `rules[].defaultConfiguration.level` | reportingDescriptor `defaultConfiguration.level`, Optional |
| `rules[].properties.precision` | reportingDescriptor `properties.precision`, Optional |
| `rules[].properties.problem.severity` | reportingDescriptor `properties.problem.severity`, Optional |
| `runs[].automationDetails.id` | runAutomationDetails `id`, Optional |
| `runs[].columnKind` | none; required by SARIF section 3.14.27 on a run with results |
| `runs[].originalUriBaseIds` | none; declares `%SRCROOT%` for validators and other consumers |
| `runs[].properties.totals` | none; carries the report's totals for a reader of the file |
| `runs[].results[]` | run `results[]`, Required |
| `results[].ruleId` | result `ruleId`, Optional |
| `results[].ruleIndex` | result `ruleIndex`, Optional |
| `results[].level` | result `level`, Optional |
| `results[].message.text` | result `message.text`, Required |
| `results[].locations[]` | result `locations[]`, Required |
| `locations[].physicalLocation` | location `location.physicalLocation`, Required |
| `physicalLocation.artifactLocation.uri` | physicalLocation `artifactLocation.uri`, Required |
| `physicalLocation.artifactLocation.uriBaseId` | none; GitHub resolves a relative URI against the repository root |
| `physicalLocation.region.startLine` | physicalLocation `region.startLine`, Required |
| `physicalLocation.region.startColumn` | physicalLocation `region.startColumn`, Required |
| `physicalLocation.region.endLine` | physicalLocation `region.endLine`, Required |
| `results[].relatedLocations[]` | result `relatedLocations[]`, Optional; linked from the message |
| `relatedLocations[].id` | location `location.id`, Optional |
| `relatedLocations[].physicalLocation` | location `location.physicalLocation`, Required |
| `relatedLocations[].message.text` | location `location.message.text`, Optional |
| `results[].partialFingerprints` | result `partialFingerprints`, Required |
| `partialFingerprints.primaryLocationLineHash` | the one key the `partialFingerprints` row names as used |
| `partialFingerprints.deadsetSymbolRef/v1` | none; a baseline joins on it |
| `results[].properties` | none; the rest of the finding for other consumers |

## What other documents fix

- [`finding.schema.json`](../finding.schema.json): the finding fields this page maps, the unit of `position.column` that decides `columnKind`, and the field carrying a component's member list.
- [`report.schema.json`](../report.schema.json): the envelope, `analyzer`, `totals`, the stale-suppression record and the analyzer name kept on each merged record.
- [`kinds.json`](../kinds.json): every rule's `code`, `name`, `rule`, `precondition`, `default_severity` and `max_class`.
- `symbol-ref.md`: the reference `deadsetSymbolRef/v1` digests.
- [`merge.md`](merge.md): the order of results and the `DS1705` finding the merge run carries.
- [`text-line.md`](text-line.md): the same sequence of findings and stale suppressions, rendered one per line.
- [`exit-codes.json`](../exit-codes.json): the code that accompanies a document, including code 4 for a report the merge has not resolved.
