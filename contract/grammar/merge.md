# The merge

The merge is a function over analyzer reports. It takes one or more reports and the range of finding-schema versions the caller accepts, and it returns one merged report and one exit code. The merged report holds no pending finding. The function reads nothing but its arguments: no source tree, no edges document, no network and no clock. Two calls over the same reports in a different order return the same bytes and the same exit code.

This page states the algorithm for an implementer who has no access to any existing implementation. The published test vectors under `vectors/merge/` pin an implementation against declared inputs and outputs rather than against another implementation; each case holds its input reports, its accepted schema range, its byte-exact expected report and its expected exit code. The report envelope, the finding object and the edge-evaluation record are declared in `report.schema.json` and are not restated here. The exit codes are the table in [`exit-codes.json`](../exit-codes.json). The symbol references the key compares follow `symbol-ref.md`.

## Vocabulary

A **cross-language edge** is a declared pair of language-tagged symbol references, with no language named in the mechanism, so a further language joins without a change of form. An edge has an identifier and two sides, `provides` and `used_by`; `provides` exists because `used_by` uses it. A maintainer or a code generator declares edges in `deadset-edges.json` at the target root. Nothing infers an edge.

An analyzer reads only the sides of an edge that carry its own language, and it never resolves the paired symbol. For each such side it publishes exactly one **edge evaluation**, a record `{edge, side, symbol, state, finding?}` in the report's `edge_evaluations` array, with `state` one of three values:

- `live`: the analyzer enumerates the symbol and holds no finding for it.
- `dead`: the analyzer enumerates the symbol and holds a finding for it that only a live paired symbol could cancel. The record carries that finding. This is the **pending finding**: it appears nowhere else in the report, so it is neither reported nor suppressed.
- `absent`: the analyzer does not enumerate the symbol at all. The record carries no finding.

A report that holds an evaluation with a finding is a report with a pending finding, and an analyzer that writes one exits with code 4. The merge is where pending findings are resolved, and the merged report is the only report in which none remain.

## What the merge receives

The caller hands the function decoded reports and nothing else. A report that fails to decode against the accepted schema never reaches the merge: the caller exits with code 3 and names the analyzer and the path. An analyzer that exited with code 3 wrote no report, and the caller exits with code 3 without presenting any other analyzer's findings as the run's result. So the merge sees either every report the run produced or none, never a partial set.

Every input record keeps the analyzer that produced it. The canonical key below uses that analyzer's `analyzer.name` as its last component, so an implementation carries the name alongside each record it reads from an input report.

## The seven steps

### Step 1: Admission

For each input report, in any order:

1. If `schema_version` is outside the accepted range, exit with code 3 and name both the report's version and the accepted range.
2. If `analyzer.conformance` is missing, or its `result` is not `pass`, exit with code 3 and name the analyzer.

Admission runs over every report before any other step. A failure here produces no merged report. Exit code: 3.

### Step 2: Union

Carry every finding, every stale-suppression record and every declared gap from every input into the merged report, without deduplication. Two analyzers that claim one language and report the same finding produce two records, and step 6 orders them by the analyzer name. Carry every edge evaluation into a working set for steps 3 to 5. Exit code: none.

### Step 3: Index

Build an index from the working set: `index[edge][side]` is the list of every evaluation across every report that names that edge identifier and that side. A side holds more than one evaluation when more than one analyzer claims its language. A side holds none when no analyzer that ran claims its language; a side with no evaluation is not `absent`, because no analyzer looked. Exit code: none.

### Step 4: Resolve

Collect every evaluation whose state is `dead`, from every report, and order the collection by the canonical key of the finding each one carries. For each evaluation `e` in that order, let `other` be every evaluation in the index for the same edge whose side differs from `e.side`, and take the first rule that applies:

1. `other` is empty. Exit with code 3 and name the unresolved edge, the pending side and its symbol. No report in the merge evaluated the paired side, so no answer exists.
2. Any evaluation in `other` is `live`. Drop the finding. It appears nowhere in the merged report.
3. Any evaluation in `other` is `dead`. Promote the finding into `findings`. The paired symbols join one component under the rule below.
4. Every evaluation in `other` is `absent`. Drop the finding. The edge names a symbol no analyzer enumerates, and step 5 reports the edge. The finding is not promoted, because the stale edge is the defect to fix first: a misspelled paired reference would otherwise turn a live pairing into a reported deletion.

Rules 2 to 4 read the strongest state across `other`, in the order `live`, then `dead`, then `absent`. When one analyzer evaluates each side, `other` holds one evaluation and the three rules are the three states. When two analyzers evaluate one side and disagree, one `live` answer keeps the pair live, and one `dead` answer with no `live` answer promotes. The finding promoted is the one the evaluation carried, unchanged apart from its component.

**Component union.** After every `dead` evaluation has been decided, take the promoted findings in canonical order. For each one, union its component with the component of every promoted finding on the other sides of the same edge. The merged component keeps the identifier of the component the loop reached first; every finding carried from the same input report as a unioned component, whose `component.id` named it, takes that identifier; `symbol_count` and `deletable_lines` become the sums over the unioned components; each finding keeps its own `root` flag. A finding step 4 dropped takes part in no union. Iteration in canonical order rather than in report order is what makes the surviving identifier independent of which analyzer finished first.

Exit code: 3 under rule 1; otherwise none.

### Step 5: Stale edges

For each edge in the index, in bytewise order of the edge identifier: if at least one side of the edge holds at least one evaluation and every evaluation of that side is `absent`, emit one `DS1705` finding for the edge. The edge names a symbol that no analyzer claiming its language enumerates. One finding per edge, however many sides are absent.

The finding is built from the edge's evaluations alone: the edge identifier, and each side's symbol and state. The merge reads no edges document, so the finding's position is the target-relative path of `deadset-edges.json` with the line and column `report.schema.json` fixes for a document-level finding, and its `symbol` and `details` fields are what `kinds.json` and `report.schema.json` declare for the code. An `absent` evaluation carries no finding and only the merge emits `DS1705`, so no input report holds one for an edge and the merge never doubles one.

Three shapes reach this step: every side `absent`; a `dead` side whose paired side is `absent`, where step 4 dropped the pending finding; and a `live` side whose paired side is `absent`. Exit code: none. `DS1705` carries the `deny` severity, so step 7 returns 1 whenever this step emits.

### Step 6: Order

Sort `findings` (carried, promoted and emitted together), `stale_suppressions` and `declared_gaps` by the canonical key below. The merged report's `edge_evaluations` array carries every evaluation whose state is `live` or `absent`, ordered by edge identifier, then side, then the carrying analyzer's name, each compared bytewise; it carries no evaluation whose state is `dead`, because step 4 resolved each one into a promotion, a drop or a stale edge. No evaluation carries a finding, so the merged report holds no pending finding. Recompute `totals` over the merged arrays. The remaining envelope members, including the version and digest of each analyzer that ran, are declared by `report.schema.json` and are derived from the input reports alone. Exit code: none.

### Step 7: Verdict

Apply the exit-code table to the merged report. Return 1 when at least one finding carries the `deny` severity or `stale_suppressions` is non-empty. Return 0 otherwise. A `warn` finding never fails the run. Where the caller's configuration turns the exit code off, the verdict is 0 and the report is unchanged.

Codes 2, 3 and 4 cannot arise here. Code 3 is returned in steps 1 and 4 only; code 4 needs a pending finding, and step 6 rules one out; code 2 is a malformed invocation, which the caller reports before any report exists. Exit code: 0 or 1.

## The algorithm in one block

```text
merge(reports, accepted_schema_range) -> (report, exit_code)

1. Admission.   for each r in reports:
                  r.schema_version not in accepted_schema_range -> exit 3
                  r.analyzer.conformance.result != "pass"       -> exit 3
2. Union.       carry every finding, stale suppression and declared gap,
                without deduplication
3. Index.       index[edge][side] = [evaluation, ...] over every report
4. Resolve.     for each dead evaluation e, in canonical order of e.finding:
                  other = index[e.edge][side != e.side]
                  other empty            -> exit 3, name the edge
                  any(other) live        -> drop e.finding
                  any(other) dead        -> promote e.finding
                  all(other) absent      -> drop e.finding; step 5 reports
                union the components of promoted paired findings,
                in canonical order
5. Stale edges. for each edge with a side whose every evaluation is absent:
                  emit one DS1705
6. Order.       sort findings, stale_suppressions and declared_gaps by the
                canonical key; carry live and absent evaluations only
7. Verdict.     any deny finding or any stale suppression -> 1, else 0
```

## The canonical key

Every array in a merged report is a total order over its records, with no input from the environment. The key for a finding is the tuple `(path, line, column, code, symbol.ref, analyzer.name)`, compared component by component in that order; the first component that differs decides.

1. `path`: the finding's `position.path`, relative to the target root, with `/` as the separator, compared bytewise.
2. `line`: the finding's `position.line`, ascending.
3. `column`: the finding's `position.column`, ascending.
4. `code`: the issue-kind code, compared bytewise. Every code is `DS` followed by four digits, so the bytewise order is the numeric order.
5. `symbol.ref`: the stable symbol reference, compared bytewise.
6. `analyzer.name`: the `analyzer.name` of the input report that carried the finding, compared bytewise.

`analyzer.name` is the final tiebreak because two analyzers may claim one language and the merge keeps both their findings, so an identical finding from two analyzers appears twice, adjacent and in a fixed order. That consequence is checkable: configure two Go analyzers and every shared finding doubles.

Language is absent from the key. Two languages never share a path, so `path` already separates a Go finding from a TypeScript one, and a mixed repository interleaves a Go handler with the TypeScript client generated from it rather than splitting into two language blocks. Nothing is lost by the omission: each finding names its language in its own `language` field, and symbol references are namespaced by language, so two same-named symbols in two languages already differ on `symbol.ref`.

The same key orders `stale_suppressions` and `declared_gaps`. Each component is taken from the field `report.schema.json` maps to it for that record type; a component the record type does not carry is the empty value, which sorts before every present value. A finding step 5 emitted supplies the empty string as its `analyzer.name`, because no input report carried it. Two records equal on every component are ordered by the bytewise comparison of their compact JSON encodings in the schema's field order, so the order stays total whatever the record type.

## Step and exit code

| Step | Exit code it can produce |
| --- | --- |
| 1. Admission | 3 |
| 2. Union | none |
| 3. Index | none |
| 4. Resolve | 3 |
| 5. Stale edges | none |
| 6. Order | none |
| 7. Verdict | 0 or 1 |

## What other documents fix

- `report.schema.json`: the envelope, the finding object, the edge-evaluation record, the field each record type supplies to the key, and the position of a document-level finding.
- `kinds.json`: the `DS1705` row, its severity and the `details` shape the emitted finding carries.
- [`exit-codes.json`](../exit-codes.json): the meaning of each code step 7 returns.
- `symbol-ref.md`: the grammar of the references the key compares bytewise.
- `vectors/merge/`: one directory per case, each with its input reports, its accepted schema range, its byte-exact expected report and its expected exit code.
