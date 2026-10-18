# The merge

The merge is a function over analyzer reports. It takes one or more reports, each with the facts document its analyzer wrote beside it where the run asked for one, the range of finding-schema versions the caller accepts and the caller's facts about the run, and it returns one merged report and one exit code. The merged report holds no pending finding. The function reads nothing but its arguments: no source tree, no edges document, no network and no clock. Two calls over the same reports in a different order return the same bytes and the same exit code.

This page states the algorithm for an implementer who has no access to any existing implementation. The published test vectors under `vectors/merge/` pin an implementation against declared inputs and outputs rather than against another implementation; each case holds its input reports, its accepted schema range, its caller's facts, its byte-exact expected report and its expected exit code. The report envelope and the edge-evaluation record are declared in `report.schema.json`, the finding object in `finding.schema.json`, the facts document and its binding record in [`facts.schema.json`](../facts.schema.json), and none is restated here. The exit codes are the table in [`exit-codes.json`](../exit-codes.json). The symbol references the key compares follow `symbol-ref.md`.

## Vocabulary

A **cross-language edge** is a declared pair of language-tagged symbol references, with no language named in the mechanism, so a further language joins without a change of form. An edge has an identifier and two sides, `provides` and `used_by`; `provides` exists because `used_by` uses it. A side may name any declaration, a type or a member. An edge that names a type says nothing about that type's members: each member is judged on its own, or through an edge whose side names it. A maintainer or a code generator declares edges in `deadset-edges.json` at the target root, each under its identifier, so one identifier names one edge. Nothing infers an edge.

An analyzer reads only the sides of an edge that carry its own language, and it never resolves the paired symbol. For each such side it publishes one **edge evaluation**, a record `{edge, side, symbol, state, finding?}` in the report's `edge_evaluations` array, with `state` one of three values:

- `live`: the analyzer enumerates the symbol and holds no finding for it.
- `dead`: the analyzer enumerates the symbol and holds a finding for it that only a live paired symbol could cancel. The record carries that finding. This is the **pending finding**: it appears nowhere else in the report, so it is neither reported nor suppressed. The record carries the finding about the symbol's declaration itself. A finding about a part of the declaration, a parameter, a receiver, a result, a statement, a case or a store, is decided inside the declaration whatever uses it, so no paired symbol can cancel it: it is never pending, and the analyzer reports it in `findings` whatever the state of the side.
- `absent`: the analyzer does not enumerate the symbol at all. The record carries no finding.

A report that holds an evaluation with a finding is a report with a pending finding, and an analyzer that writes one exits with code 4. The merge is where pending findings are resolved, and the merged report is the only report in which none remain.

A **binding record** is a record of a facts document: what one language's analysis, or the schema reader, knows about one schema declaration. A `schema` record is a schema declaration the reader publishes, `live` or `dead` with a pending finding; a `consumer` record is a generated declaration that stands for a schema declaration, `live` or `unreferenced`; an `implementer` record is a hand-written method implementing a schema RPC, `live` or `dead` with a pending finding; a `fallback` record is a generated file whose declarations bind only as a whole. [`analysis/schemas.md`](analysis/schemas.md) states what the reader publishes and [`analysis/bindings.md`](analysis/bindings.md) what an analyzer publishes. A binding record **matches** a schema record when their `schema` members are equal, compared bytewise, and their `schema_file` members are equal or one is the other preceded by one or more whole path segments, so `v1/service.proto` matches `proto/v1/service.proto` and not `xv1/service.proto`. A pending finding a dead binding record carries is a pending finding as an edge evaluation's is.

## What the merge receives

The caller hands the function decoded reports and nothing else. A report that fails to decode against the accepted schema never reaches the merge: the caller exits with code 3 and names the analyzer and the path. An analyzer that exited with code 3 wrote no report, and the caller exits with code 3 without presenting any other analyzer's findings as the run's result. So the merge sees either every report the run produced or none, never a partial set. A facts document is read the same way: one the run asked for that is absent, that fails to decode against [`facts.schema.json`](../facts.schema.json) or that breaks one of the document rules that schema names makes the caller exit with code 3, and the merge sees none.

Every input record keeps the analyzer that produced it. The canonical key below uses that analyzer's `analyzer.name` as its last component, so an implementation carries the name alongside each record it reads from an input report.

The caller also hands the function the facts no input report carries, because they describe the product that runs the merge and the artifacts it ran rather than any analysis:

- the merging product's identity: its `name`, its `version` and its `conformance` block, the three members of the merged report's `analyzer` the inputs cannot supply;
- the `schema_version` the merging product writes and the `contract_version` it implements;
- for each input report, the digest of the analyzer artifact that wrote it, which the merged report's `merged_from` entry for that report carries;
- `fail_on`, the lowest severity at which a finding fails the run, which is the merging product's resolved `reporters.fail_on` and which step 7 reads;
- `min_confidence`, the merging product's resolved `analysis.min_confidence`, which binding rule 3 reads; absent, the merge withholds nothing.

The accepted range is the list of finding-schema versions the caller admits, in ascending order. The merge reads it as given: step 1 admits a report whose `schema_version` the list names, and the merged report's `analyzer.schema_versions_accepted` is the list unchanged.

Every other member of the merged report is derived from the input reports and the accepted range by the steps below.

## The seven steps

### Step 1: Admission

For each input report, in any order:

1. If `schema_version` is outside the accepted range, exit with code 3 and name both the report's version and the accepted range.
2. If `analyzer.conformance` is missing, or its `result` is not `pass`, exit with code 3 and name the analyzer.
3. If `totals.omitted` is not 0, exit with code 3 and name the analyzer. An input report holds every finding its analysis produced: `reporters.max_findings` limits what a rendering prints, and the merging product applies it once, to the report the merge returns.
4. If its `target.root` or its `target.identity` differs from that of another input report, exit with code 3 and name both reports and both values. A merged report names one target. Each input report then claims a kind for each language its `analyzer.languages` names whose row of [`languages.json`](../languages.json) has `takes_target_kind`: that language's member of `target.language_kinds` where `target.kind` is `mixed`, and `target.kind` otherwise. A report whose languages take no target kind, a schema reader's, claims none. If its `target.kind` is `mixed` and its `target.language_kinds` names a language its kind-taking languages do not, leaves out one of them, or gives every language it names one kind, exit with code 3 and name the report. If it claims a kind for a language another input report claims another kind for, exit with code 3 and name both reports, the language and both kinds. Reports of one target agree on each language's kind, and two reports of two languages may claim two kinds.
5. If an entry of its `configurations`, `configurations_not_built`, `consumers.loaded` or `consumers.unavailable` shares its `id` with an entry of the same array in another input report, and the two entries differ in an identity member, exit with code 3 and name both reports and the `id`. The identity members of a configuration entry are its `id` and its shape: `os`, `arch` and `tags` for a platform, `project` for a project. The identity members of a consumer entry are its `id`, its `role` and, in `consumers.loaded`, its `path`. The `error` of a `configurations_not_built` entry and the `reason` of a `consumers.unavailable` entry are free text each analyzer words on its own, so two entries that differ only there name one configuration or one consumer, and step 6 carries one of them. One identifier names one configuration or one consumer of a run.
6. If an entry of its `configurations` shares its `id` with an entry of `configurations_not_built` in another input report, or an entry of its `consumers.loaded` shares its `id` with an entry of `consumers.unavailable` in another input report, exit with code 3 and name both reports and the `id`, whatever members the two entries hold. A configuration of a run is built or dropped, and a consumer is loaded or unavailable, never both.
7. If its `analyzer.name` equals the `analyzer.name` of another input report, compared bytewise, exit with code 3 and name both reports, whatever versions and digests the two carry. An analyzer's name prefixes every component identifier it mints and is the `analyzer` member of every record step 6 carries from its report, so the merged report cannot tell two reports of one name apart.
8. If it comes with a facts document whose `analyzer.name` or `analyzer.version` differs from its own, exit with code 3 and name the report and both values.
9. If its facts document's `language` is not a language its `analyzer.languages` names, exit with code 3 and name the report and the language.
10. If a finding its facts document carries, as a record's `finding` or in its `carried`, names a `component.id` or a `symbol.ref` that a finding of the report names, exit with code 3 and name the report, the identifier or the reference. A facts finding is by construction one the report does not hold ([`analysis/bindings.md`](analysis/bindings.md), `bindings.18`), so a document that restates, alters or duplicates a reported finding is refused rather than read.

The identity members of two entries are compared as JSON values: two objects are equal when they hold the same members with equal values, in any order, and two arrays when they hold equal elements in the same order. Admission runs over every report before any other step. A failure here produces no merged report. Exit code: 3.

### Step 2: Union

Carry every finding, every stale-suppression record and every declared gap from every input into the merged report, without deduplication. Two analyzers that claim one language and report the same finding produce two records, and step 6 orders them by the analyzer name. Carry every edge evaluation into a working set for steps 3 to 5, and every binding record of every facts document into it for step 4, each keeping the analyzer of the report it came with.

A `DS9004` finding is the one exception. Its `symbol.ref` is a configured root string the reporting analyzer matched with no symbol, and every analyzer is handed every configured string, its own language's or not. A report **reads** a configured string when the text before the string's first `://` holds no `*` and no `?` and is a language its `analyzer.languages` names; every report reads a string with no such text. Where a report that reads a configured string holds no `DS9004` finding for it, an analyzer matched the string, and the merge carries no `DS9004` finding for that string from any report. Every other `DS9004` finding is carried as any finding is, so a string no analyzer matched is reported by every analyzer that reported it. Exit code: none.

### Step 3: Index

Build an index from the working set: `index[edge][side]` is the list of every evaluation across every report that names that edge identifier and that side. A side holds more than one evaluation when more than one analyzer claims its language. A side holds none when no analyzer that ran claims its language; a side with no evaluation is not `absent`, because no analyzer looked. Exit code: none.

### Step 4: Resolve

Collect every evaluation whose state is `dead`, from every report, and order the collection by the canonical key of the finding each one carries. For an evaluation `e`, let `other` be every evaluation in the index for the same edge whose side differs from `e.side`, and let the **component of `e`** be the `component.id` the finding of `e` names: every finding the input reports carry, and every finding their dead evaluations carry, that names that identifier. An analyzer prefixes every identifier it mints with its own name and step 1 admits one report per name, so an identifier names a component of one input report. Four rules decide an evaluation:

1. `other` is empty. Exit with code 3 and name the unresolved edge, the pending side and its symbol. No report in the merge evaluated the paired side, so no answer exists.
2. Any evaluation in `other` reads as `live`. Drop the component of `e`, below. None of its findings appears in the merged report.
3. Any evaluation in `other` reads as `dead`. Promote the finding into `findings`. The paired symbols join one component under the rule below.
4. Every evaluation in `other` is `absent`. Drop the component of `e`, below. The edge names a symbol no analyzer enumerates, and step 5 reports the edge. The finding is not promoted, because the stale edge is the defect to fix first: a misspelled paired reference would otherwise turn a live pairing into a reported deletion.

Rule 1 runs first, over the whole collection in its order, and the first evaluation it applies to names the edge. Rules 2 to 4 read the strongest state across `other`, in the order `live`, then `dead`, then `absent`. When one analyzer evaluates each side, `other` holds one evaluation and the three rules are the three states. When two analyzers evaluate one side and disagree, one `live` answer keeps the pair live, and one `dead` answer with no `live` answer promotes. An evaluation reads as its own state, except that a `dead` evaluation whose component is dropped reads as `live`. The finding promoted is the one the evaluation carried, unchanged apart from its component and its confidence. Two dead evaluations of one report carry one finding where two edges name one symbol: a finding is promoted once, however many of its report's dead evaluations carry it, and it takes part in the component union on every edge whose evaluation carried it. Two carried findings are one finding when they are equal on the canonical key.

**The fixpoint.** Rules 2 and 4 run to a fixpoint before rule 3 promotes anything, because a drop changes what the rules read on every other edge where the dropped component holds a dead evaluation:

1. Drop the component of every dead evaluation that rule 2 or rule 4 decides.
2. From then on, read every dead evaluation of a dropped component as `live`, on its own edge.
3. Repeat from 1 until a pass drops no component that an earlier pass had not dropped.

Then promote, under rule 3, the finding of every dead evaluation whose component is not dropped; rule 3 is the rule each of them meets once the fixpoint is reached. The set of dropped components only grows, so the outcome does not depend on the order in which the rules visit the evaluations. A dead evaluation of a dropped component reads as `live` whichever of rule 2 and rule 4 dropped its component, because the merged report reports no member of a dropped component for deletion, and a symbol paired with a member that stays is not reported for deletion either.

**What a drop removes.** Dropping a component removes every finding of it from the merged report: the finding of each of its dead evaluations, whichever edge carries the evaluation, and every finding step 2 carried whose `component.id` names the component. A component is one deletion, and a symbol that falls with a root is dead only through it, so once one finding of the component is kept out of the merged report, none of them is reported.

**Bindings.** The binding records decide alongside the edge evaluations, in the same fixpoint. For a dead `schema` record `s`, its **bindings** are the `consumer` records that match it and the `fallback` records whose `schema_file` matches its `schema_file` by the file rule above and whose `scope` covers the kind of `s`'s finding: `all` covers every kind, and `types` covers every `symbol.kind` but `service` and `rpc`. An `implementer` record is never a binding: a server implementing an RPC is no evidence that anything calls it. Three rules decide `s`, and the **component of `s`** is the `component.id` its finding names:

1. `s` has no binding. Drop the component of `s`: no analyzed language binds the declaration, so nothing the run analyzes can say it is unused.
2. A binding of `s` reads `live`; or a `schema` record reads `live` whose `uses` names `s`'s `schema`; or a `schema` record reads `live` that matches a reference `s`'s `members` names, a field, a value, an RPC or a nested declaration of `s`, because a container cannot be deleted while a member stays. Drop the component of `s`. A live container keeps no member.
3. Otherwise every binding of `s` reads `unreferenced`. Promote the finding of `s`, its confidence lowered to `possible` where a binding of `s` carries `cap` and the finding is not `certain`, and where a binding of `s` is of a language its report's target kind makes a `library` ([`analysis/schemas.md`](analysis/schemas.md), `schemas.12`, `schemas.13` and `schemas.16`). The schema reader publishes a finding `certain` only under `proto.closed`, which states that no reader outside the target exists, so no binding's `cap` lowers it; a library binding lowers it all the same. A dead `schema` record whose finding this rule would promote at a confidence below the caller's `analysis.min_confidence` is **withheld**, as the reader withholds a record below it (`schemas.20`): before the first pass its component is dropped, so it reads `live`. The merged `withheld` count of each confidence below the minimum, taken from the highest, gains one for each finding that a merge whose minimum is that confidence promotes and that no merge whose minimum is a higher confidence promotes, so lowering the minimum to a confidence shows exactly the findings counted at it and above it: a withheld record, and every finding it keeps from promotion by reading `live`, such as the container of a withheld member, the type a withheld record uses and the implementer of a withheld RPC with the findings it carries.

For a dead `implementer` record `i`, its schema records are the `schema` records that match it, and the **component of `i`** is the `component.id` its finding names:

1. No schema record matches `i`. Drop the component of `i`: no schema reader ran for its declaration, and the method stays live through its registration.
2. A schema record of `i` reads `live`, or an `implementer` record of the same analyzer that `i`'s `via` names reads `live`. Drop the component of `i`.
3. Otherwise promote the finding of `i` and every finding its `carried` holds.

A binding record reads as its own state, with four exceptions. A `consumer` or an `implementer` record that carries no `state` reads `live`, so identity facts keep every declaration they bind and promote nothing. A `dead` record of a dropped component reads `live`, whichever relation dropped it. A `consumer` record with `via` reads `live` exactly when at least one `implementer` record its `via` names, of the same analyzer, reads `live`, and `unreferenced` otherwise. A `via` entry that names no `implementer` record of that analyzer reads `live`. A `fallback` record reads `live`.

The fixpoint above runs over both relations: each pass drops the component of every dead evaluation edge rule 2 or 4 decides and of every dead `schema` or `implementer` record binding rule 1 or 2 decides, and the passes repeat until one drops no new component. The promotion then runs over both: a finding is promoted once, whichever dead evaluations and dead binding records of its report carry it, and a finding that several components carry is promoted only when none of them is dropped, whichever relation carries it, so a finding a dropped edge side carries is promoted by no binding record, and one a dropped binding record carries by no edge side. Dropping a component removes the findings of its dead binding records as it removes those of its dead evaluations.

The binding joins take part in the component union below, before the edge joins, in the canonical order of the promoted findings: a promoted `implementer` finding joins the component of each promoted `schema` finding whose record matches the implementer's; a promoted `schema` finding of which a binding with `via` read `unreferenced` joins the component of each promoted `implementer` finding that `via` names; and a promoted `schema` finding whose record a promoted `schema` record's `members` names joins that container's component, and is marked `root` `false`. A carried finding joins every promoted component that carries it.

**Component union.** After every `dead` evaluation and every dead binding record has been decided, take the promoted findings in canonical order. For each one, make the binding joins above, then union its component with the component of every promoted finding on the other sides of the same edge. The merged component keeps the identifier of the component the loop reached first, the component of the earliest promoted finding in canonical order among those the union joins, however many edges the join spans; every finding step 2 carried whose `component.id` named a unioned component takes that identifier; `symbol_count` and `deletable_lines` become the sums over the unioned components, each component's two counts read from any one of its findings, because every finding of a component names the same counts, less, for the findings several of the unioned components carry, one symbol for each carrying component beyond the first, and each source line those findings span, from `position.line` to `position.end_line`, once for every component beyond the first among those whose carried findings span it, so each finding and each of its lines is counted once, and two records of one component carrying a finding take nothing from that component; and less, from each component that is not dropped, one symbol for each finding it carries that no relation promotes because a dropped component carries it too, and the distinct source lines those findings span, each from its `position.line` to its `position.end_line`, that no other finding of the component spans, a finding step 2 carried or a dead evaluation or dead binding record carries, because those findings stay live and deleting what the merge reports does not remove them; each finding keeps its own `root` flag. A finding step 4 dropped takes part in no union. Every finding of a unioned component then takes one confidence, by rule confidence.29 of [`analysis/confidence.md`](analysis/confidence.md): the lowest `confidence` among the unioned component's root findings whose `test_only` is `false`, or its own where its own is lower, so a finding production code only tests use keeps its `possible`. Where every root finding of the unioned component has `test_only` `true`, each finding keeps its own confidence. Iteration in canonical order rather than in report order is what makes the surviving identifier independent of which analyzer finished first.

Exit code: 3 under rule 1; otherwise none.

### Step 5: Stale edges

For each edge in the index, in bytewise order of the edge identifier: if at least one side of the edge holds at least one evaluation and every evaluation of that side is `absent`, emit one `DS9005` finding for the edge. The edge names a symbol that no analyzer claiming its language enumerates. One finding per edge, however many sides are absent.

The finding is built from the edge's evaluations alone: the edge identifier, and each side's symbol and state. The merge reads no edges document. The **stale side** is the side whose every evaluation is `absent`, and `provides` where both sides are. The finding's members, in `finding.schema.json`'s order:

- `code` `DS9005`, and `kind`, `fixability` and `severity` the name, fixability and default severity [`kinds.json`](../kinds.json) gives that code;
- `language` the language the stale side's symbol reference names by its prefix ([`symbol-ref.md`](symbol-ref.md));
- `position` the target-relative path `deadset-edges.json`, line 1, column 1 and end line 1, the position `report.schema.json` fixes for a document-level finding;
- `symbol` with `ref` the stale side's symbol, `kind` `edge`, `name` the edge identifier and `size_lines` 1, and no other member;
- `reachability_class` and `confidence` both `certain`, and `test_only` and `generated` both `false`, with no `liveness_relation`;
- `component` a one-member root component with zero deletable lines, its identifier the merging product's name, a solidus, `c-` and the decimal ordinal of the edge among the edges this step emits for, counting from 1 with no leading zero: `deadset/c-1` for the first;
- `retained_by`, `configurations` and `consumers_loaded` empty, and no `analyzer` member, because no input report carried the finding;
- `message` `no analyzer enumerates the symbol either side of the edge names` where both sides are stale, and otherwise `no analyzer enumerates the symbol the provides side of the edge names` or `no analyzer enumerates the symbol the used_by side of the edge names`, naming the one stale side;
- `details` with `edge` the edge identifier and `sides` one entry per side that holds an evaluation, `provides` before `used_by`, each naming the side, its symbol and its strongest state in the order step 4 reads, `live`, then `dead`, then `absent`.

An `absent` evaluation carries no finding and only the merge emits `DS9005`, so no input report holds one for an edge and the merge never doubles one.

Three shapes reach this step: every side `absent`; a `dead` side whose paired side is `absent`, where step 4 dropped the pending finding; and a `live` side whose paired side is `absent`. Exit code: none. `DS9005` carries the `deny` severity, so step 7 returns 1 whenever this step emits.

### Step 6: Order

Sort `findings` (carried, promoted and emitted together), `stale_suppressions` and `declared_gaps` by the canonical key below. The merged report's `edge_evaluations` array carries every evaluation whose state is `live` or `absent`, ordered by edge identifier, then side, then the carrying analyzer's name, each compared bytewise, then by the bytewise comparison of the records' compact JSON encodings in the schema's field order; it carries no evaluation whose state is `dead`, because step 4 resolved each one into a promotion, a drop or a stale edge. No evaluation carries a finding, so the merged report holds no pending finding. Every carried record, an evaluation and a stale-suppression record included, gains an `analyzer` member naming the input report that carried it, where its schema declares one; a finding emitted in step 5 carries none, so every record of the merged report that names no analyzer is one the merge emitted, and every `analyzer` member names an entry of `merged_from`.

The rest of the envelope, member by member:

- `schema_version` and `contract_version` are the caller's.
- `analyzer` names the merging product: its `name`, `version` and `conformance` are the caller's, its `languages` is every language an input report's `analyzer.languages` names, each once and ordered bytewise, and its `schema_versions_accepted` is the accepted range as the caller gave it.
- `merged_from` holds one entry per input report, ordered by name compared bytewise: the report's `analyzer.name` and `analyzer.version`, the digest the caller gave for that report, the report's `analyzer.languages`, and the report's `totals` as it wrote them. No two entries share a name, because step 1 admitted one report per name.
- `components` is present where an input report carries it, with one entry per distinct `component.id` the merged `findings` name, in the order of the first merged finding naming it. An entry holds every member an input report lists under that identifier, and the entry of a component the component union joined holds the members of every component it joined; each member appears once, and the members are ordered by the canonical key below, read from each member's `position` and `ref`. The one-member component of a stale-edge finding the merger makes, which no input lists, holds that finding's `symbol.ref`, `symbol.name` and `position` as its member, and a finding a facts document carried, which no report lists, is a member of its component the same way. A component no merged finding names, a dropped one among them, has no entry.
- `target` names the `root` and the `identity` every input report names, which step 1 made one target. Its `kind` is the kind the input reports claim for every kind-taking language the merged `analyzer.languages` names where they claim one kind for all of them, and `mixed` otherwise, with `language_kinds` then naming the kind claimed for each of those languages, in the order `report.schema.json` lists them. A merge of reports whose languages share one kind therefore writes that kind, as each input report does, and a merge whose languages take no kind writes neither member.
- `configurations`, `configurations_not_built`, `excluded_by_cgo`, `test_file_rules`, `type_error_skips`, `notes`, `unanswered_questions` and `conventions_applied` are the union of the input reports' entries, an entry two reports carry appearing once, each in the order `report.schema.json` states for it. `test_file_rules` holds one entry per distinct entry, ordered by `rule` compared bytewise, then by the bytewise comparison of the entries' compact JSON encodings in the schema's field order, so two analyzers that count one rule differently contribute two entries under it. The last four arrays hold one entry per distinct entry in the same way: `type_error_skips` ordered by `path` compared bytewise, then by `line`; `notes` by `kind`, then by `path`; `unanswered_questions` by `configuration`, so two analyzers that count one configuration differently contribute two entries under it; and `conventions_applied` by `name`, then by `manifest`; each then by the bytewise comparison of the entries' compact JSON encodings. `configurations` and `configurations_not_built` hold one entry per `id`. Step 1 refused an `id` held in both arrays and two entries under one `id` whose identity members differ, so an `id` appears at most once across the two merged arrays, and the entries the inputs carry under it agree on every identity member. The merged entry is the entry of the input report that comes first in `merged_from`, its `error` included.
- `notes` gains the merge's own notes, beside the input reports' entries. One `unbound-schema` note per schema file every dead `schema` record of which has no binding, so binding rule 1 drops each: its `path` the `position.path` of those records' findings, its `key` `proto.plugins`, and its `message` `no analyzed language binds a declaration of this schema file; proto.plugins names the generators that do`. One `binding-unsupported` note per `generated` entry of a facts document whose `generator.row` names a row of [`generators.json`](../generators.json) whose `cell` is `unsupported`: its `path` the entry's `path`, no `key`, and its `message` `the generator of this file is outside every supported version or option set, so no declaration of it binds`.
- No binding record is carried into the merged report, which holds no member for one.
- `consumers.loaded` and `consumers.unavailable` are the unions of the input reports' entries in the same way: one entry per `id`, a `consumers.unavailable` entry carrying the `reason` of the report that comes first in `merged_from`, ordered by `id`, so an `id` appears at most once across the two lists, and `consumers.declared` is the sum of their two lengths.
- `totals` are recomputed over the merged arrays, except `suppressions_in_effect`, `reasons_recorded` and each count of `withheld`, which count records no merged array holds and are the sums of the input reports' counts, each `withheld` count with the findings binding rule 3 withholds at that confidence. `deletable_lines` is the sum of `deletable_lines` over the distinct components the root findings name, so a component two root findings name contributes its lines once. `omitted` is 0, because step 1 admitted no report that omitted a finding; a cap the merging product applies to the report it writes is applied after the merge returns.

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
                  r.target.root or r.target.identity differs
                  from another report's                         -> exit 3
                  r.target.kind mixed and language_kinds not
                  exactly r's kind-taking languages, or of one
                  kind                                          -> exit 3
                  r claims a language's kind another report
                  claims otherwise                              -> exit 3
                  an entry of r shares its id with an entry of
                  the same array in another report, and an
                  identity member of the two differs            -> exit 3
                  an entry of r shares its id with an entry of
                  the array paired with it in another report    -> exit 3
                  r.analyzer.name equals another report's       -> exit 3
                  r's facts name another analyzer or version,
                  or a language r does not name                 -> exit 3
                  a finding r's facts carry names a component
                  or a symbol a finding of r names              -> exit 3
2. Union.       carry every finding, stale suppression and declared gap,
                without deduplication, but no DS9004 for a configured
                string a report that reads it holds no DS9004 for
3. Index.       index[edge][side] = [evaluation, ...] over every report;
                bindings(s) = matching consumers and covering fallbacks
4. Resolve.     dead = every dead evaluation, in canonical order of its finding
                other(e) = index[e.edge][side != e.side]
                comp(e)  = e.finding.component.id
                other(e) empty for an e in dead -> exit 3, name the first
                dropped = {comp(s) for each dead schema record s
                           promoted below caller.min_confidence}
                repeat until dropped stops growing:
                  for each e in dead with comp(e) not in dropped:
                    any(other(e)) live, or dead with
                    its comp in dropped  -> add comp(e) to dropped
                    all(other(e)) absent -> add comp(e) to dropped;
                                            step 5 reports
                  for each dead schema record s, comp(s) not in dropped:
                    bindings(s) empty    -> add comp(s) to dropped
                    a binding, a user or a member of s reads live
                                         -> add comp(s) to dropped
                  for each dead implementer i, comp(i) not in dropped:
                    no schema record of i, or one reads live, or
                    an implementer its via names reads live
                                         -> add comp(i) to dropped
                count in withheld, at each confidence c below the
                minimum from the highest, every finding a resolution
                with minimum c promotes and none with a higher one does
                promote, once, the finding of each e in dead and of
                each dead binding record, lowered to possible under a
                cap unless certain and under a library binding
                language, that no dead evaluation or binding record
                of a component in dropped carries
                drop every finding of a dropped component: the finding of
                each of its dead evaluations, and every carried finding
                with its component.id
                union the components of promoted paired findings and
                the binding joins, in canonical order; each finding of
                a unioned component takes the lowest confidence of its
                non-test-only roots; a component not dropped counts
                no finding a dropped component also carries
5. Stale edges. for each edge with a side whose every evaluation is absent:
                  emit one DS9005
6. Order.       sort findings, stale_suppressions and declared_gaps by the
                canonical key; carry live and absent evaluations only;
                build the envelope from the inputs and the caller's facts,
                one entry per id in each id-keyed array, the entry of the
                first report in merged_from that carries the id
                list the members of each component a merged finding names
                once, where an input report lists components
                add one unbound-schema note per schema file whose
                every dead schema record has no binding, one
                binding-unsupported note per generated entry of an
                unsupported row
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
- [`finding.schema.json`](../finding.schema.json): the finding object the page orders, its `details` branches, and the shape of the emitted `DS9005` finding.
- [`kinds.json`](../kinds.json): the `DS9005` row and its severity.
- [`facts.schema.json`](../facts.schema.json): the facts document and the binding record the binding pass reads.
- [`exit-codes.json`](../exit-codes.json): the meaning of each code step 7 returns.
- `symbol-ref.md`: the grammar of the references the key compares bytewise.
- `vectors/merge/`: one directory per case, each with its input reports, its accepted schema range, its caller's facts, its byte-exact expected report and its expected exit code.
