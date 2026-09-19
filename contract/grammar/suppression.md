# Suppression

One suppression grammar, implemented identically by every analyzer, in two mechanisms and no third: an inline directive in the source, and a committed ignore file at the target root. A third document, the baseline, records a finding set so a run fails only on what is new; it uses the same row shape but adjudicates nothing. Every suppression carries a reason, is scoped to one code at one site, marks its symbol live before the sweep rather than filtering the report after it, and is itself a finding when it stops matching. This page states the grammar formally enough to implement from, then the semantics, for an implementer who has no access to any existing implementation. The token corpus in [`suppression-corpus.json`](suppression-corpus.json) holds every accepted and refused string with the rule it exercises; the grammar self-test in this repository reads it, and a product's conformance suite reads it too.

Nothing here is applied by the merge. An analyzer reads the three documents and applies them before it writes its report; the merge sees findings, stale-suppression records and totals, and reads no document of its own ([`merge.md`](merge.md)).

## Vocabulary

A **suppression record** is one code bound to one site: an inline directive naming two codes is two records with one reason. A record is **bound** when the site it names resolves to a symbol; **in effect** when the symbol would otherwise have produced a finding under the record's code; **dormant** when it is bound and the configuration sets the severity of the code it names to `allow`; **stale** when it is bound to nothing or in effect for nothing; and **refused** when it fails a rule below, in which case it binds nothing and the rule's outcome applies. A refused string has one of three outcomes: it is not a directive at all and nothing happens; the run ends with exit code 2 before any finding exists; or the run continues and the string is reported under a `DS17xx` code.

A dormant record is neither in effect nor stale: it produces no finding, and `totals.suppressions_in_effect` does not count it. It is counted under `totals.reasons_recorded` like every record that carries a reason, because that total counts the records that carry one and not the records that hold a finding back. Dormancy is what makes the severity dial safe to turn: setting a kind to `allow` never fails a run over the adjudications that setting it back would need.

## The namespace

The namespace is `deadset:`, lowercase, followed by a directive name. A comment in any other namespace is not a directive of this contract: it suppresses nothing, it is reported as nothing, and the finding it sits above stays reported. The same holds for a near miss, `//deadset-ignore`, `// Deadset:ignore` or `deadset:ignore` inside a block comment: the analyzer does not read it, and the finding it was meant to cover is what tells the maintainer.

The spelling is the [Go directive-comment form](https://go.dev/doc/comment#directives): a line comment of the form `//toolname:directive arguments`, lowercase tool name, no whitespace before the colon. `//deadset:ignore …` is therefore a directive comment under Go's own definition, so `gofmt` keeps it out of the rendered documentation and moves it to the end of the doc comment, directly above the declaration, which is what the placement rule below relies on.

This contract defines one directive name, `ignore`. A comment in the namespace with any other name is malformed: the run ends with exit code 2 and names the file and line, in the same way an undeclared key in a JSON document does (the closed key lists below). The namespace is closed so that a later contract version can add a name without a product having silently ignored it first.

## The inline directive

### The shape

```text
deadset:ignore <CODE>[,<CODE>…] -- <reason>
```

The directive is the first token of a `//` line comment, with optional whitespace between `//` and the token, and it names one or more codes and then a reason after the separator `--`. Both spellings, `//deadset:ignore` and `// deadset:ignore`, parse to the same directive. The no-space form is the Go directive convention above, which `gofmt` preserves; the space form is what a TypeScript formatter keeps. After the whitespace between `//` and the token is removed, the two are one string.

The grammar, in EBNF:

```text
directive  = "//" , { ws } , "deadset:ignore" , ws , { ws } , codes , ws , { ws } , "--" , ws , { ws } , reason , { ws } ;
codes      = code , { "," , code } ;
code       = "DS" , digit , digit , digit , digit ;
reason     = non-ws , { any } ;            (* at least one character that is not whitespace *)
ws         = " " | "\t" ;
non-ws     = any character other than " ", "\t", CR and LF ;
any        = any character other than CR and LF ;
```

A directive is one line. The code list is codes joined by a comma with no whitespace inside the list. The separator is exactly `--`, one token, with whitespace on both sides. The reason is everything after the separator up to the end of the line, with trailing whitespace trimmed, and it is not parsed: it may contain a colon, a code, a path or a further `--`.

An implementer applies three expressions in order, written in the intersection of RE2 as Go's `regexp` package implements it and ECMAScript `RegExp` with no flags, so one string compiles unchanged in a Go implementation and in a TypeScript one ([`text-line.md`](text-line.md) states the dialect rules). The first recognizes a candidate, the second the well-formed directive, the third a directive that lacks only its reason:

```text
^//[ \t]*deadset:
```

```text
^//[ \t]*deadset:ignore[ \t]+(?<codes>DS[0-9]{4}(?:,DS[0-9]{4})*)[ \t]+--[ \t]+(?<reason>[^ \t\r\n][^\r\n]*?)[ \t]*$
```

```text
^//[ \t]*deadset:ignore[ \t]+(?<codes>DS[0-9]{4}(?:,DS[0-9]{4})*)(?:[ \t]+--)?[ \t]*$
```

The decision procedure, over the text of each `//` comment from its first slash to the end of its line:

1. The comment does not match the first expression. It is not a directive. Nothing happens, whatever else it says.
2. The comment matches the second expression. It is a directive: one record per code in `codes`, all carrying `reason`.
3. The comment matches the third expression. It is a directive with no reason: one record per code in `codes`, each refused, each reported as `DS1701` (`suppression-without-reason`) at the comment's position, and each binding nothing. A reasonless directive naming two codes is therefore two refusals.
4. Otherwise the comment is in the namespace and malformed: the run ends with exit code 2, naming the file, the line and the expected form.

Step 4 covers a directive name other than `ignore`, a missing or misspelled code, whitespace inside the code list, a code outside the `DS` shape, and text after the code list that does not begin with the separator. The malformed case ends the run rather than becoming a finding because it is a maintainer's instruction the analyzer cannot carry out: read as prose the instruction would do nothing, and if the finding it was meant to cover has already gone, nothing would say so.

The reason is required: it records for the next reader why the finding is not to be reported. No setting makes it optional, and a suppression that carries none is a finding at either mechanism.

### Where it sits

The directive sits on the line immediately above the declaration it targets, and it binds to every symbol whose declaration begins on the next line. Nothing else binds it: not a trailing comment on the declaration's own line, not a line two above with a blank line or a doc-comment line between, not a line below. A directive on any other line is well formed and bound to nothing, so it is reported as `DS1703` (`stale-suppression`) at its own position, and the finding it was meant to cover stays reported.

The declaration begins at its first token. In Go that is the keyword of a declaration (`func`, `type`, `var`, `const`), the name of a specification inside a grouped declaration (`var (` … `)`), a field name inside a struct, or a method name inside an interface. In TypeScript and JavaScript it is the first token of the declaration node, a decorator, a modifier (`export`, `private`, `static`, `readonly`) or the keyword (`class`, `function`, `const`, `enum`, `interface`, `type`), whichever comes first. A doc comment is not a token of the declaration, so a directive placed above a doc comment is not immediately above the declaration; in Go, `gofmt` moves a directive comment to the end of the doc comment for exactly this reason, so a formatted file always has the directive on the right line. A line that declares several symbols, `var a, b int` or `const x = 1, y = 2`, binds the directive to each of them; a `var (` or `const (` line declares no symbol and binds nothing. A function's parameters and results declared on the function's first line bind alongside the function, which is where a `DS1801` or `DS1803` directive goes; a parameter on a later line of a wrapped signature takes a directive on the line above it, inside the parameter list.

A trailing directive on the declaration's own line, a directive covering a range of lines, and a file-wide directive are not admitted: a suppression here names one site so that its staleness can be decided at that site, and a range or a file-wide directive has no single symbol to be stale against.

### Rendered

Go, with a doc comment, in the position `gofmt` produces:

```go
// ResolveAlias returns the catalog entry an alias stands for.
//
//deadset:ignore DS1001 -- Reached only through the generated client; see client_gen.go:88.
func (c *Catalog) ResolveAlias(name string) string {
```

TypeScript, on a class member:

```ts
// deadset:ignore DS1003 -- Read by the Vitest snapshot serializer through a string index.
private cachedLayout: Layout | null = null;
```

Go, two codes on one line, two records with one reason:

```go
//deadset:ignore DS1001,DS1101 -- Public by the v3 API promise; narrowing it is a major bump.
func (c *Catalog) Aliases() []string {
```

## The ignore file

### The document

`deadset-ignore.json` at the target root, one file per target, name and location fixed. An absent file is an empty one. The document is strict JSON (RFC 8259), decoded by each language's standard library with a closed key list: on the Go side `encoding/json` with `DisallowUnknownFields`, on the TypeScript side `JSON.parse` followed by a structural check against the same key list. No comment syntax, no trailing comma, no other file name and no converter from another tool's format.

```json
{
  "description": "Adjudications for example.com/app. Every entry names a path: a bare symbol name is refused (DS1702).",
  "ignore": [
    {
      "code": "DS1001",
      "symbol": "go://example.com/app#Catalog.ResolveAlias",
      "path": "catalog.go",
      "reason": "Kept for the v3 API promise; removing it is a major bump. Revisit at v4."
    }
  ]
}
```

| Key | Type | Required | Meaning |
| --- | --- | --- | --- |
| `description` | string | no | What a file header comment would have carried. Not read by the analysis. |
| `ignore` | array of entries | yes | The adjudications, in document order. May be empty. |

Any other top-level key is a decode error: the run ends with exit code 2 and names the key.

### The entry

| Key | Type | Meaning |
| --- | --- | --- |
| `code` | string | The issue-kind code, `DS` followed by four digits, rendered exactly as a text line, a configuration key and a SARIF rule identifier render it. One code per entry. |
| `symbol` | string | The stable symbol reference of the declaration, in the grammar `symbol-ref.md` states: `go://<module-path>[/<package-dir>]#<Name>[.<Member>]` or `ts://<package-name>/<source-path>#<Container>[.<Member>]`. Compared for equality with the finding's `symbol.ref`; no glob, no regular expression, no bare name. |
| `path` | string | The path of the file that holds the declaration, relative to the target root, with `/` as the separator, no leading `./`, no trailing `/`, never absolute: the same form the finding's `position.path` takes ([`text-line.md`](text-line.md), `path`). |
| `reason` | string | Why the finding is not to be reported. At least one character that is not whitespace. |

The four keys are the whole key list. An entry with a key outside it, `line` included, is a decode error and ends the run with exit code 2, as does a value of the wrong type. `code` and `symbol` are required by the decoder in the same way: without them the entry can name nothing. `path` and `reason` are required by a finding rather than by the decoder, because each has its own rule in the contract and each is the field a maintainer drops when carrying an entry over from a format that did not have it:

- An entry whose `reason` is absent, empty or whitespace only is refused and reported as `DS1701` (`suppression-without-reason`) at the entry's position.
- An entry whose `path` is absent or empty is refused and reported as `DS1702` (`unscoped-ignore-entry`) at the entry's position rather than matched, so a bare name cannot mask a match anywhere else in the project.

The two are separate checks and both run. An entry lacking both its `reason` and its `path` is two findings at one position, `DS1701` and `DS1702`, so a maintainer who supplies the missing reason still sees the missing path.

A `code` outside the `DS` shape, a `symbol` that does not parse under `symbol-ref.md`, and a `path` outside the form above are malformed values: the run ends with exit code 2 and names the entry and the expected form. A `code` in the right shape that names no live kind in [`kinds.json`](../kinds.json), a retired code for example, is well formed and can match nothing; it is reported as `DS1703`.

An entry's position, for the findings above, is the line and column of the `{` that opens the entry object in `deadset-ignore.json`.

### Matching

An entry matches a symbol when its `code`, `symbol` and `path` all equal the finding the symbol would produce: the finding's `code`, its `symbol.ref` and its `position.path`. It never matches a symbol of the same name in another file or another package. For a TypeScript entry the reference already carries the source path and `path` repeats it; the two must agree, and an entry whose `path` names a different file from its `symbol` matches nothing and is `DS1703`.

An entry names the code, the symbol and the file exactly, and admits no glob, no regular expression, no bare name and no substring match in any of them, so an adjudication written for one symbol in one file masks nothing else.

## The baseline

`deadset-baseline.json` at the target root, name and location fixed, written by an analyzer and read back by a later run of the same analyzer. It records a finding set so that only a finding absent from it fails the run, a ratchet on the finding total rather than an adjudication of any one finding.

```json
{
  "description": "Recorded before the migration; shrink it, never grow it.",
  "baseline": [
    {
      "code": "DS1001",
      "symbol": "go://example.com/app#Catalog.ResolveAlias",
      "path": "catalog.go",
      "reason": "recorded by deadset-go 1.0.0"
    }
  ]
}
```

| Key | Type | Required | Meaning |
| --- | --- | --- | --- |
| `description` | string | no | Not read by the analysis. |
| `baseline` | array of rows | yes | One row per recorded finding, in the report order of the run that wrote it. |

A row has the same four keys as an ignore entry, with the same types, the same closed key list, the same value forms, the same two findings when `reason` or `path` is missing (`DS1701`, `DS1702`) and the same matching rule. The shape is shared so that one decoder and one matcher serve both documents and so that a row can be moved into the ignore file by adding a maintainer's reason. What differs is what `reason` holds. The analyzer writes it as the two words `recorded by`, a space, its `analyzer.name`, a space and its `analyzer.version`, the same two strings its report carries; it is the provenance of the row and never a maintainer's judgement. A reader must not take a baseline row as an adjudication: the finding it records is real, and the row says only that it was already there when the baseline was written.

When asked to write a baseline, an analyzer writes one row per record of its report's `findings`, in report order, and nothing else: no row for a stale suppression, no row for a pending finding (which lives in an edge evaluation and is neither reported nor suppressed), no row for a finding a directive or an entry already suppressed. A later run that reads the document back suppresses exactly the recorded findings. No timestamp and no host detail is written, so the document is the same bytes for the same report.

A stale row is a finding like any other stale suppression: a row bound to nothing, or to a symbol that would not have produced the recorded code, is reported as `DS1703` at the row's position, the line and column of the `{` that opens it. The ratchet turns on this: the total goes down when a maintainer deletes dead code and rewrites the baseline, and a stale row that is not rewritten fails the run, so the baseline cannot drift into a list of claims about code that has gone.

## Before the sweep

A bound suppression record has two effects, and both happen before mark and sweep run.

First, it marks its symbol live, under both liveness relations the analysis uses, reference counting and reachability. The symbol is then live for every kind that asks whether a symbol is used, and every symbol the marked one references is live through it, so the marked symbol's dependents never cascade into findings. Second, it withholds the finding the record's code would have produced for that symbol, which is what a record for a kind that is not about liveness needs: a write-only member (`DS1301`) or an unused parameter (`DS1801`) is referenced, so the mark alone would change nothing.

The consequence is plain and it is deliberate: **a suppressed symbol keeps every symbol it references alive.** If `ResolveAlias` is suppressed and it is the only caller of `normalizeName`, then `normalizeName` is not reported, and neither is anything only `normalizeName` reaches. Delete the directive and the whole chain is reported together, as one dead component with `ResolveAlias` at its root. A maintainer who suppresses a symbol is asserting that the symbol is wanted, and a wanted symbol's callees are wanted with it.

Filtering the report after the sweep does not implement this rule. Under a filter the suppressed symbol stays dead in the graph, so every symbol only it references is still reported, and because a dead component reports its root and not its members, an adjudication on a root would remove the only line on which the rest of the component was visible.

The ordering of the run, then: resolve every directive, entry and row to a record; bind each record to its symbol; mark; sweep; compute each record's `matched`; report. The mark step is step 0 of the mark-and-sweep in the design, and the marked set is the union of every bound record's symbol.

### Staleness

`matched` is the finding a record's symbol would have produced under the record's code had the mark not been there, computed by running that kind's candidate test on the marked symbol alone, with the kind evaluated under the run's resolved configuration. A record with a match is in effect. A record with none is stale and is reported as `DS1703` (`stale-suppression`) at the record's own position, whatever its mechanism: the directive's line for an inline directive, the entry's opening brace in `deadset-ignore.json`, the row's opening brace in `deadset-baseline.json`. The `details` object of a `DS1703` finding carries `mechanism` (`inline`, `ignore` or `baseline`) and the record's `entry`; `finding.schema.json` fixes their shape.

A record is stale when:

- it is bound to nothing: a directive on the wrong line, or above a line that declares no symbol, or an entry whose `symbol` and `path` resolve to no declaration in the target;
- its symbol is used, so no kind reports it; this is the stale directive left behind after the code changed, and it is the case that fails the run;
- its symbol is held back by an exemption class, so no suppression was needed; this is the expected fate of most existing adjudications, because the exemption model resolves the type-dependent classes (interface satisfaction, encoding and reflection, the `String` and `io.Writer` contracts, iota groups) without a suppression;
- its symbol would produce a finding, but under a code the record does not name; a directive for `DS1001` on a symbol that reports `DS1002` is stale. The mark still keeps the symbol live, so a liveness kind's finding for it is withheld and the `DS1703` is what the run reports; its message names the code the symbol would have produced, so the maintainer corrects the code rather than hunting for it. A kind the mark does not touch (`DS1301`, `DS1801`) is reported beside the stale record;
- the finding its code would produce falls below the minimum confidence the resolved configuration sets, so nothing reports it; a record for a finding nobody sees is a claim nobody checks. A record whose code the configuration sets to `allow` is dormant rather than stale;
- its code names one of the self-check kinds `DS1701` to `DS1705`: those findings report a directive, an entry, a root or an edge and no declaration, so nothing exists for a record to bind to;
- another record already matched the same finding. A finding is matched by at most one record. Records are resolved in the order inline directives, ignore entries, baseline rows, and within a document in document order, and the first record to bind a given symbol and code is the one in effect; a later one is stale. Two identical entries are one entry written twice, and the second is reported.

Staleness has no severity dial. `DS1703` is fixed on at `deny` (`"fixed": true` in [`kinds.json`](../kinds.json)): no flag, no severity setting and no per-mechanism exception reduces it below a finding, and a configuration naming it under a severity key is an unimplemented key. A run that reports at least one stale suppression exits with the findings code, 1, whatever else it holds ([`exit-codes.json`](../exit-codes.json)).

### The counts

The report's `totals.suppressions_in_effect` counts records in effect and `totals.reasons_recorded` counts the directives, entries and rows that carry a reason; `report.schema.json` fixes both. The summary line prints them, which is where a suppressed finding is visible, because no finding exists for it in `findings` or in any output rendered from `findings` ([`text-line.md`](text-line.md); [`sarif.md`](sarif.md) omits suppressed findings and never emits the SARIF `suppressions` property).

## The refusals, in one table

| Defect | Mechanism | Outcome |
| --- | --- | --- |
| Comment in another namespace, a near miss of this one, `deadset:ignore` not the first token, or a block comment | inline | Not a directive. Nothing happens; the finding stays reported. |
| Directive name other than `ignore`; no code; whitespace inside the code list; a code outside `DS` and four digits; text after the codes without the `--` separator | inline | Exit code 2, the file and line named, before any finding is produced. |
| Separator absent and no reason, or separator present and the reason empty or whitespace | inline | One `DS1701` per code the directive names, at the directive's line. Binds nothing. |
| Directive on any line other than the one immediately above a declaration | inline | Well formed, bound to nothing: `DS1703` at the directive's line. |
| Document not strict JSON; a key outside the closed list; a value of the wrong type; `code` or `symbol` absent; a `code`, `symbol` or `path` value outside its form | ignore entry, baseline row | Exit code 2, the document and the entry named. |
| `reason` absent, empty or whitespace | ignore entry, baseline row | `DS1701` at the entry's position. Binds nothing. |
| `path` absent or empty | ignore entry, baseline row | `DS1702` at the entry's position. Binds nothing. |
| `reason` and `path` both absent or empty | ignore entry, baseline row | `DS1701` and `DS1702` at the entry's position. Binds nothing. |
| Well formed, bound to nothing or in effect for nothing | all three | `DS1703` at the record's position. |

## The token corpus

[`suppression-corpus.json`](suppression-corpus.json) is an array of cases. Each case is `{ "input", "kind", "accepted", "rule", "reason" }` and may carry a sixth member, `reports`: `kind` is `inline`, `ignore-entry` or `baseline-row`; `accepted` says whether the input is a well-formed, bound suppression under this page; `rule` names the rule the case exercises from the vocabulary below; `reason` says why, and for a refused case names the outcome. For an `inline` case the input is `{ "comment", "line_offset" }`, where `comment` is the text of the comment from its first `/` to the end of the line, with no leading indentation, and `line_offset` is the comment's line number minus the first line of the declaration it targets: `-1` is the line immediately above and the only accepted value. For an entry or a row the input is the JSON object itself.

A refused case has exactly one defect unless it carries `reports`, an array naming one code per finding the input produces, in the order the rules below apply. Such a case names under `rule` the rule of its first defect, and `reports` is what says how many findings the input is. The two inputs that carry it are the two the count is not otherwise readable from: a reasonless directive naming two codes, which is one refusal per code, and an entry lacking both its `reason` and its `path`, which fails two separate checks.

The rule vocabulary, with the outcome a refused case under it has:

| Rule | Applies to | On refusal |
| --- | --- | --- |
| `spelling-no-space`, `spelling-space`, `reason-text` | inline | accepted forms only |
| `code-list` | inline | accepted when two codes are comma-joined; exit code 2 when the list is absent, spaced or outside the `DS` shape |
| `namespace`, `first-token`, `line-comment-only` | inline | not a directive |
| `directive-name`, `reason-separator` | inline | exit code 2 |
| `reason-required` | all three | `DS1701` |
| `line-above` | inline | `DS1703` |
| `entry-shape`, `row-shape` | ignore entry, baseline row | accepted forms only |
| `path-required` | ignore entry, baseline row | `DS1702` |
| `path-form`, `symbol-form`, `code-form`, `closed-keys`, `value-type` | ignore entry, baseline row | exit code 2 |

### Accepted

Inline, each at `line_offset` `-1`:

```text
//deadset:ignore DS1001 -- Reached only through the generated client; see client_gen.go:88.
// deadset:ignore DS1003 -- Read by the Vitest snapshot serializer through a string index.
//<TAB>deadset:ignore DS1002 -- Called by name from the generated dispatcher in dispatch_gen.go.
//deadset:ignore DS1001,DS1101 -- Public by the v3 API promise; narrowing it is a major bump.
//deadset:ignore DS1801 -- kept -- the upstream callback signature fixes the arity.
// deadset:ignore DS1301 -- see DS1301 in docs/adjudications.md: the binder writes it by reflection.
//deadset:ignore DS1001 -- Kept for the plugin loader, which resolves it by name.   
```

The third has one tab character between `//` and the token, written `<TAB>` above because a rendered tab is invisible; the corpus file holds the real character. The fourth yields two records; the fifth has a `--` inside its reason, which is not parsed; the seventh has trailing whitespace, which is trimmed. The accepted entries are the three under `entry-shape` in the corpus (a Go method, a TypeScript class member, a symbol in a nested Go package) and the accepted rows are the two under `row-shape`.

### Refused

Inline, at `line_offset` `-1` unless stated:

```text
//deadset:ignore DS1001
//deadset:ignore DS1001 --
//deadset:ignore DS1001 --    
//deadset:ignore DS1001,DS1101
//deadset:ignore DS1001 -- Reached only through the generated client.        (line_offset 0)
//deadset:ignore DS1001 -- Reached only through the generated client.        (line_offset -2)
//deadset:ignore DS1001 -- Reached only through the generated client.        (line_offset 1)
//nolint:unused // legacy hook kept for the plugin loader
//lint:ignore U1000 legacy hook kept for the plugin loader
// eslint-disable-next-line @typescript-eslint/no-unused-vars -- kept for the snapshot serializer
//revive:disable-next-line:unused-receiver
//deadset-ignore DS1001 -- Reached only through the generated client.
// Deadset:ignore DS1001 -- Reached only through the generated client.
// TODO deadset:ignore DS1001 -- Reached only through the generated client.
/* deadset:ignore DS1001 -- Reached only through the generated client. */
/** @public */
//deadset:disable DS1001 -- Reached only through the generated client.
//deadset:ignoreDS1001 -- Reached only through the generated client.
//deadset:ignore -- Reached only through the generated client.
//deadset:ignore DS1001, DS1002 -- Reached only through the generated client.
//deadset:ignore U1000 -- Reached only through the generated client.
//deadset:ignore ds1001 -- Reached only through the generated client.
//deadset:ignore DS1001 Reached only through the generated client.
//deadset:ignore DS1001 --Reached only through the generated client.
//deadset:ignore DS1001 --- Reached only through the generated client.
```

In order: four with no reason (`DS1701`), the fourth naming two codes and so reported twice; three on the wrong line (`DS1703`); six in another namespace or a near miss of this one, one with the token not first, and two block comments (not directives); two with a directive name the contract does not define; four with a malformed code list; three with a malformed separator (each exit code 2).

The refused entries and rows are in the corpus under `reason-required` (no `reason`, an empty one, a whitespace one, and one lacking both its `reason` and its `path` and so reported under `DS1701` and `DS1702` both), `path-required` (no `path`, an empty one; `DS1702`), `path-form` (absolute, `./`-prefixed, backslash separators), `symbol-form` (a bare name, a glob), `code-form` (`EU1002`, `ds1001`), `closed-keys` (a `line` key) and `value-type` (an array for `code`, a number for `reason`), the last five exit code 2.

## What other documents fix

- `symbol-ref.md`: the grammar of the `symbol` value, per language.
- [`finding.schema.json`](../finding.schema.json): the `DS1701`, `DS1702` and `DS1703` finding shapes and their `details` (`mechanism`, `entry`), and `symbol.ref` and `position.path`, the two fields an entry is matched against.
- [`report.schema.json`](../report.schema.json): the `stale_suppressions` array and the `totals` fields `suppressions_in_effect`, `reasons_recorded` and `stale_suppressions`.
- [`kinds.json`](../kinds.json): the three codes, their `deny` severity, and `DS1703`'s `"fixed": true`.
- [`exit-codes.json`](../exit-codes.json): code 1 for a run with a stale suppression, code 2 for a malformed directive or document.
- [`text-line.md`](text-line.md): how a stale suppression renders, and the path form the `path` key shares.
- [`sarif.md`](sarif.md): the omitted suppressed finding and the never-emitted `suppressions` property.
- [`merge.md`](merge.md): the merge reads no suppression document; it carries the stale-suppression records the analyzers wrote.
