# The merge

The merge is a function over analyzer reports. It takes one or more reports, the range of finding-schema versions the caller accepts and the caller's facts about the run, and it returns one merged report and one exit code. The merged report holds no pending finding. The function reads nothing but its arguments: no source tree, no edges document, no network and no clock. Two calls over the same reports in a different order return the same bytes and the same exit code.

This page states the algorithm for an implementer who has no access to any existing implementation. The published test vectors under `vectors/merge/` pin an implementation against declared inputs and outputs rather than against another implementation; each case holds its input reports, its accepted schema range, its caller's facts, its byte-exact expected report and its expected exit code. The report envelope and the edge-evaluation record are declared in `report.schema.json`, the finding object in `finding.schema.json`, and neither is restated here. The exit codes are the table in [`exit-codes.json`](../exit-codes.json). The symbol references the key compares follow `symbol-ref.md`.

## Vocabulary

A **cross-language edge** is a declared pair of language-tagged symbol references, with no language named in the mechanism, so a further language joins without a change of form. An edge has an identifier and two sides, `provides` and `used_by`; `provides` exists because `used_by` uses it. A side may name any declaration, a type or a member. An edge that names a type says nothing about that type's members: each member is judged on its own, or through an edge whose side names it. A maintainer or a code generator declares edges in `deadset-edges.json` at the target root. Nothing infers an edge.

An analyzer reads only the sides of an edge that carry its own language, and it never resolves the paired symbol. For each such side it publishes one **edge evaluation**, a record `{edge, side, symbol, state, finding?}` in the report's `edge_evaluations` array, with `state` one of three values:

- `live`: the analyzer enumerates the symbol and holds no finding for it.
- `dead`: the analyzer enumerates the symbol and holds a finding for it that only a live paired symbol could cancel. The record carries that finding. This is the **pending finding**: it appears nowhere else in the report, so it is neither reported nor suppressed. The record carries the finding about the symbol's declaration itself. A finding about a part of the declaration, a parameter, a receiver, a result, a statement, a case or a store, is decided inside the declaration whatever uses it, so no paired symbol can cancel it: it is never pending, and the analyzer reports it in `findings` whatever the state of the side.
- `absent`: the analyzer does not enumerate the symbol at all. The record carries no finding.

A report that holds an evaluation with a finding is a report with a pending finding, and an analyzer that writes one exits with code 4. The merge is where pending findings are resolved, and the merged report is the only report in which none remain.

## What the merge receives

The caller hands the function decoded reports and nothing else. A report that fails to decode against the accepted schema never reaches the merge: the caller exits with code 3 and names the analyzer and the path. An analyzer that exited with code 3 wrote no report, and the caller exits with code 3 without presenting any other analyzer's findings as the run's result. So the merge sees either every report the run produced or none, never a partial set.

Every input record keeps the analyzer that produced it. The canonical key below uses that analyzer's `analyzer.name` as its last component, so an implementation carries the name alongside each record it reads from an input report.

The caller also hands the function the facts no input report carries, because they describe the product that runs the merge and the artifacts it ran rather than any analysis:

- the merging product's identity: its `name`, its `version` and its `conformance` block, the three members of the merged report's `analyzer` the inputs cannot supply;
- the `schema_version` the merging product writes and the `contract_version` it implements;
- for each input report, the digest of the analyzer artifact that wrote it, which the merged report's `merged_from` entry for that report carries;
- `fail_on`, the lowest severity at which a finding fails the run, which is the merging product's resolved `reporters.fail_on` and which step 7 reads.

The accepted range is the list of finding-schema versions the caller admits, in ascending order. The merge reads it as given: step 1 admits a report whose `schema_version` the list names, and the merged report's `analyzer.schema_versions_accepted` is the list unchanged.

Every other member of the merged report is derived from the input reports and the accepted range by the steps below.

## The seven steps

### Step 1: Admission

For each input report, in any order:

1. If `schema_version` is outside the accepted range, exit with code 3 and name both the report's version and the accepted range.
2. If `analyzer.conformance` is missing, or its `result` is not `pass`, exit with code 3 and name the analyzer.
3. If `totals.omitted` is not 0, exit with code 3 and name the analyzer. An input report holds every finding its analysis produced: `reporters.max_findings` limits what a rendering prints, and the merging product applies it once, to the report the merge returns.
4. If its `target` differs from the `target` of another input report, exit with code 3 and name both reports and both targets. A merged report names one target.
5. If an entry of its `configurations`, `configurations_not_built`, `consumers.loaded` or `consumers.unavailable` shares its `id` with an entry of the same array in another input report, and the two entries differ in an identity member, exit with code 3 and name both reports and the `id`. The identity members of a configuration entry are its `id` and its shape: `os`, `arch` and `tags` for a platform, `project` for a project. The identity members of a consumer entry are its `id`, its `role` and, in `consumers.loaded`, its `path`. The `error` of a `configurations_not_built` entry and the `reason` of a `consumers.unavailable` entry are free text each analyzer words on its own, so two entries that differ only there name one configuration or one consumer, and step 6 carries one of them. One identifier names one configuration or one consumer of a run.
6. If an entry of its `configurations` shares its `id` with an entry of `configurations_not_built` in another input report, or an entry of its `consumers.loaded` shares its `id` with an entry of `consumers.unavailable` in another input report, exit with code 3 and name both reports and the `id`, whatever members the two entries hold. A configuration of a run is built or dropped, and a consumer is loaded or unavailable, never both.
7. If its `analyzer.name` equals the `analyzer.name` of another input report, compared bytewise, exit with code 3 and name both reports, whatever versions and digests the two carry. An analyzer's name prefixes every component identifier it mints and is the `analyzer` member of every record step 6 carries from its report, so the merged report cannot tell two reports of one name apart.

Two targets, or the identity members of two entries, are compared as JSON values: two objects are equal when they hold the same members with equal values, in any order, and two arrays when they hold equal elements in the same order. Admission runs over every report before any other step. A failure here produces no merged report. Exit code: 3.

### Step 2: Union

Carry every finding, every stale-suppression record and every declared gap from every input into the merged report, without deduplication. Two analyzers that claim one language and report the same finding produce two records, and step 6 orders them by the analyzer name. Carry every edge evaluation into a working set for steps 3 to 5. Exit code: none.

### Step 3: Index

Build an index from the working set: `index[edge][side]` is the list of every evaluation across every report that names that edge identifier and that side. A side holds more than one evaluation when more than one analyzer claims its language. A side holds none when no analyzer that ran claims its language; a side with no evaluation is not `absent`, because no analyzer looked. Exit code: none.

### Step 4: Resolve

Collect every evaluation whose state is `dead`, from every report, and order the collection by the canonical key of the finding each one carries. For an evaluation `e`, let `other` be every evaluation in the index for the same edge whose side differs from `e.side`, and let the **component of `e`** be the `component.id` the finding of `e` names: every finding the input reports carry, and every finding their dead evaluations carry, that names that identifier. An analyzer prefixes every identifier it mints with its own name and step 1 admits one report per name, so an identifier names a component of one input report. Four rules decide an evaluation:

1. `other` is empty. Exit with code 3 and name the unresolved edge, the pending side and its symbol. No report in the merge evaluated the paired side, so no answer exists.
2. Any evaluation in `other` reads as `live`. Drop the component of `e`, below. None of its findings appears in the merged report.
3. Any evaluation in `other` reads as `dead`. Promote the finding into `findings`. The paired symbols join one component under the rule below.
4. Every evaluation in `other` is `absent`. Drop the component of `e`, below. The edge names a symbol no analyzer enumerates, and step 5 reports the edge. The finding is not promoted, because the stale edge is the defect to fix first: a misspelled paired reference would otherwise turn a live pairing into a reported deletion.

Rule 1 runs first, over the whole collection in its order, and the first evaluation it applies to names the edge. Rules 2 to 4 read the strongest state across `other`, in the order `live`, then `dead`, then `absent`. When one analyzer evaluates each side, `other` holds one evaluation and the three rules are the three states. When two analyzers evaluate one side and disagree, one `live` answer keeps the pair live, and one `dead` answer with no `live` answer promotes. An evaluation reads as its own state, except that a `dead` evaluation whose component is dropped reads as `live`. The finding promoted is the one the evaluation carried, unchanged apart from its component.

**The fixpoint.** Rules 2 and 4 run to a fixpoint before rule 3 promotes anything, because a drop changes what the rules read on every other edge where the dropped component holds a dead evaluation:

1. Drop the component of every dead evaluation that rule 2 or rule 4 decides.
2. From then on, read every dead evaluation of a dropped component as `live`, on its own edge.
3. Repeat from 1 until a pass drops no component that an earlier pass had not dropped.

Then promote, under rule 3, the finding of every dead evaluation whose component is not dropped; rule 3 is the rule each of them meets once the fixpoint is reached. The set of dropped components only grows, so the outcome does not depend on the order in which the rules visit the evaluations. A dead evaluation of a dropped component reads as `live` whichever of rule 2 and rule 4 dropped its component, because the merged report reports no member of a dropped component for deletion, and a symbol paired with a member that stays is not reported for deletion either.

**What a drop removes.** Dropping a component removes every finding of it from the merged report: the finding of each of its dead evaluations, whichever edge carries the evaluation, and every finding step 2 carried whose `component.id` names the component. A component is one deletion, and a symbol that falls with a root is dead only through it, so once one finding of the component is kept out of the merged report, none of them is reported.

**Component union.** After every `dead` evaluation has been decided, take the promoted findings in canonical order. For each one, union its component with the component of every promoted finding on the other sides of the same edge. The merged component keeps the identifier of the component the loop reached first, the component of the earliest promoted finding in canonical order among those the union joins, however many edges the join spans; every finding step 2 carried whose `component.id` named a unioned component takes that identifier; `symbol_count` and `deletable_lines` become the sums over the unioned components, each component's two counts read from any one of its findings, because every finding of a component names the same counts; each finding keeps its own `root` flag. A finding step 4 dropped takes part in no union. Iteration in canonical order rather than in report order is what makes the surviving identifier independent of which analyzer finished first.

Exit code: 3 under rule 1; otherwise none.

### Step 5: Stale edges

For each edge in the index, in bytewise order of the edge identifier: if at least one side of the edge holds at least one evaluation and every evaluation of that side is `absent`, emit one `DS1705` finding for the edge. The edge names a symbol that no analyzer claiming its language enumerates. One finding per edge, however many sides are absent.

The finding is built from the edge's evaluations alone: the edge identifier, and each side's symbol and state. The merge reads no edges document. The **stale side** is the side whose every evaluation is `absent`, and `provides` where both sides are. The finding's members, in `finding.schema.json`'s order:

- `code` `DS1705`, and `kind`, `fixability` and `severity` the name, fixability and default severity [`kinds.json`](../kinds.json) gives that code;
- `language` the language the stale side's symbol reference names by its prefix ([`symbol-ref.md`](symbol-ref.md));
- `position` the target-relative path `deadset-edges.json`, line 1, column 1 and end line 1, the position `report.schema.json` fixes for a document-level finding;
- `symbol` with `ref` the stale side's symbol, `kind` `edge`, `name` the edge identifier and `size_lines` 1, and no other member;
- `reachability_class` and `confidence` both `certain`, and `test_only` and `generated` both `false`, with no `liveness_relation`;
- `component` a one-member root component with zero deletable lines, its identifier the merging product's name, a solidus, `c-` and the decimal ordinal of the edge among the edges this step emits for, counting from 1 with no leading zero: `deadset/c-1` for the first;
- `retained_by`, `configurations` and `consumers_loaded` empty, and no `analyzer` member, because no input report carried the finding;
- `message` `no analyzer enumerates the symbol either side of the edge names` where both sides are stale, and otherwise `no analyzer enumerates the symbol the provides side of the edge names` or `no analyzer enumerates the symbol the used_by side of the edge names`, naming the one stale side;
- `details` with `edge` the edge identifier and `sides` one entry per side that holds an evaluation, `provides` before `used_by`, each naming the side, its symbol and its strongest state in the order step 4 reads, `live`, then `dead`, then `absent`.

An `absent` evaluation carries no finding and only the merge emits `DS1705`, so no input report holds one for an edge and the merge never doubles one.

Three shapes reach this step: every side `absent`; a `dead` side whose paired side is `absent`, where step 4 dropped the pending finding; and a `live` side whose paired side is `absent`. Exit code: none. `DS1705` carries the `deny` severity, so step 7 returns 1 whenever this step emits.

### Step 6: Order

Sort `findings` (carried, promoted and emitted together), `stale_suppressions` and `declared_gaps` by the canonical key below. The merged report's `edge_evaluations` array carries every evaluation whose state is `live` or `absent`, ordered by edge identifier, then side, then the carrying analyzer's name, each compared bytewise, then by the bytewise comparison of the records' compact JSON encodings in the schema's field order; it carries no evaluation whose state is `dead`, because step 4 resolved each one into a promotion, a drop or a stale edge. No evaluation carries a finding, so the merged report holds no pending finding. Every carried record, an evaluation and a stale-suppression record included, gains an `analyzer` member naming the input report that carried it, where its schema declares one; a finding emitted in step 5 carries none, so every record of the merged report that names no analyzer is one the merge emitted, and every `analyzer` member names an entry of `merged_from`.

The rest of the envelope, member by member:

- `schema_version` and `contract_version` are the caller's.
- `analyzer` names the merging product: its `name`, `version` and `conformance` are the caller's, its `languages` is every language an input report's `analyzer.languages` names, each once and ordered bytewise, and its `schema_versions_accepted` is the accepted range as the caller gave it.
- `merged_from` holds one entry per input report, ordered by name compared bytewise: the report's `analyzer.name` and `analyzer.version`, and the digest the caller gave for that report. No two entries share a name, because step 1 admitted one report per name.
- `target` is the target every input report names, which step 1 made one target.
- `configurations`, `configurations_not_built`, `excluded_by_cgo` and `test_file_rules` are the union of the input reports' entries, an entry two reports carry appearing once, each in the order `report.schema.json` states for it. `test_file_rules` holds one entry per distinct entry, ordered by `rule` compared bytewise, then by the bytewise comparison of the entries' compact JSON encodings in the schema's field order, so two analyzers that count one rule differently contribute two entries under it. `configurations` and `configurations_not_built` hold one entry per `id`. Step 1 refused an `id` held in both arrays and two entries under one `id` whose identity members differ, so an `id` appears at most once across the two merged arrays, and the entries the inputs carry under it agree on every identity member. The merged entry is the entry of the input report that comes first in `merged_from`, its `error` included.
- `consumers.loaded` and `consumers.unavailable` are the unions of the input reports' entries in the same way: one entry per `id`, a `consumers.unavailable` entry carrying the `reason` of the report that comes first in `merged_from`, ordered by `id`, so an `id` appears at most once across the two lists, and `consumers.declared` is the sum of their two lengths.
- `totals` are recomputed over the merged arrays, except `suppressions_in_effect` and `reasons_recorded`, which count records no merged array holds and are the sums of the input reports' counts. `deletable_lines` is the sum of `deletable_lines` over the distinct components the root findings name, so a component two root findings name contributes its lines once. `omitted` is 0, because step 1 admitted no report that omitted a finding; a cap the merging product applies to the report it writes is applied after the merge returns.

Exit code: none.

### Step 7: Verdict

Apply the exit-code table to the merged report. Return 1 when at least one finding carries a severity at or above the caller's `fail_on`, `allow` ranking below `warn` and `warn` below `deny`, or when `stale_suppressions` is non-empty. Return 0 otherwise. Under the default `fail_on`, `deny`, a `warn` finding never fails the run. The verdict is always the table's: a switch that turns the exit code off belongs to the merging product, which applies it after the merge returns, as it applies `reporters.max_findings`.

Codes 2, 3 and 4 cannot arise here. Code 3 is returned in steps 1 and 4 only; code 4 needs a pending finding, and step 6 rules one out; code 2 is a malformed invocation, which the caller reports before any report exists. Exit code: 0 or 1.

## The algorithm in one block

```text
merge(reports, accepted_schema_range, caller) -> (report, exit_code)
    caller = {schema_version, contract_version, analyzer, digests, fail_on}

1. Admission.   for each r in reports:
                  r.schema_version not in accepted_schema_range -> exit 3
                  r.analyzer.conformance.result != "pass"       -> exit 3
                  r.totals.omitted != 0                         -> exit 3
                  r.target != another report's target           -> exit 3
                  an entry of r shares its id with an entry of
                  the same array in another report, and an
                  identity member of the two differs            -> exit 3
                  an entry of r shares its id with an entry of
                  the array paired with it in another report    -> exit 3
                  r.analyzer.name equals another report's       -> exit 3
2. Union.       carry every finding, stale suppression and declared gap,
                without deduplication
3. Index.       index[edge][side] = [evaluation, ...] over every report
4. Resolve.     dead = every dead evaluation, in canonical order of its finding
                other(e) = index[e.edge][side != e.side]
                comp(e)  = e.finding.component.id
                other(e) empty for an e in dead -> exit 3, name the first
                dropped = {}
                repeat until dropped stops growing:
                  for each e in dead with comp(e) not in dropped:
                    any(other(e)) live, or dead with
                    its comp in dropped  -> add comp(e) to dropped
                    all(other(e)) absent -> add comp(e) to dropped;
                                            step 5 reports
                promote e.finding for each e in dead, comp(e) not in dropped
                drop every finding of a dropped component: the finding of
                each of its dead evaluations, and every carried finding
                with its component.id
                union the components of promoted paired findings,
                in canonical order
5. Stale edges. for each edge with a side whose every evaluation is absent:
                  emit one DS1705
6. Order.       sort findings, stale_suppressions and declared_gaps by the
                canonical key; carry live and absent evaluations only;
                build the envelope from the inputs and the caller's facts,
                one entry per id in each id-keyed array, the entry of the
                first report in merged_from that carries the id
7. Verdict.     any finding at or above caller.fail_on
                or any stale suppression -> 1, else 0
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

- [`report.schema.json`](../report.schema.json): the envelope, the edge-evaluation record, the field each record type supplies to the key, and the position of a document-level finding.
- [`finding.schema.json`](../finding.schema.json): the finding object the page orders, its `details` branches, and the shape of the emitted `DS1705` finding.
- [`kinds.json`](../kinds.json): the `DS1705` row and its severity.
- [`exit-codes.json`](../exit-codes.json): the meaning of each code step 7 returns.
- `symbol-ref.md`: the grammar of the references the key compares bytewise.
- `vectors/merge/`: one directory per case, each with its input reports, its accepted schema range, its caller's facts, its byte-exact expected report and its expected exit code.
