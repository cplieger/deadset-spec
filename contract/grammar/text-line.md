# The text line

The text format writes one finding per line, position first, in one shape for every analyzer. One search expression therefore finds every finding in a single analyzer's output and in merged output alike, and an editor or a log viewer that links `path:line:col` links these lines too. This page states the format field by field for an implementer who has no access to any existing implementation, and it publishes the regular expression that defines it. Every string under [Accepted](#accepted) matches the expression and every string under [Refused](#refused) does not; the grammar self-test in this repository checks both sets.

## The shape

```text
path:line:col: kind name: message [confidence] (CODE)
```

Two findings rendered, one from each language:

```text
catalog.go:214:6: method (*Catalog).ResolveAlias: exported method has no reference in the target and none from any loaded consumer [probable] (DS1001)
src/features/tabs/index.ts:1182:11: class-member TabStrip.cachedLayout: private member is written and never read [certain] (DS1301)
```

Each line is one finding object from the JSON report, rendered from eight of its fields and nothing else. The mapping is fixed; a reporter adds no field and drops none.

## Field by field

| Field | Source in the finding | Form |
| --- | --- | --- |
| `path` | `position.path` | The path relative to the target root, with `/` as the separator on every platform, no leading `./`, no trailing `/`. Beyond that separator normalization the producer encodes nothing: each element carries the bytes the tree holds it under, so a space, a `%` or a non-ASCII character in a name appears as itself. Never an absolute path and never the target root itself, because a host detail has no place in the default text output. |
| `line` | `position.line` | A positive decimal integer, 1-based, no sign, no leading zero. |
| `col` | `position.column` | A positive decimal integer, 1-based, no sign, no leading zero. It counts the unit `finding.schema.json` fixes for `position.column`; this page does not restate the unit. |
| `kind` | `symbol.kind` | One value from the closed vocabulary `finding.schema.json` declares for `symbol.kind`: lowercase ASCII letters and hyphens, such as `method`, `class-member` or `file`. |
| `name` | `symbol.name` | The symbol's display name, verbatim. For a Go method that is the `(*Catalog).ResolveAlias` form; for a TypeScript member it is `Container.member`. `symbol-ref.md` and `finding.schema.json` fix the name; this page only carries it. |
| `message` | `message` | The finding's message, verbatim. |
| `confidence` | `confidence` | One of `certain`, `probable` or `possible`. It is the `confidence` field, the one `--min-confidence` filters on, and never `reachability_class`. |
| `CODE` | `code` | The issue-kind code, `DS` followed by four digits, rendered exactly as it appears in an ignore entry, a configuration key and a SARIF rule identifier. |

## Separators, and the order of the fields

The position comes first, so that editors, terminals and log viewers turn `path:line:col` into a link and one expression anchored at the start of the line matches every analyzer's output. The code comes last, in parentheses: a reader scanning a long report finds the code at a fixed distance from the right margin, and a filter for one code is one expression on the tail.

The separators, in order:

1. `:` between `path` and `line`, and between `line` and `col`.
2. `": "` (a colon and one space) after `col`.
3. One space between `kind` and `name`.
4. `": "` (a colon and one space) after `name`.
5. One space, then `[`, the confidence, `]`.
6. One space, then `(`, the code, `)`.
7. Nothing after `)`. The line ends with one LF (`\n`), never CR LF; every released analyzer targets the platforms `contract.json` names, and all of them use LF.

A parser reads the line left to right with two rules. The position is the leftmost split: the shortest prefix that is followed by `":digits:digits: "` and a valid remainder is the `path`, so a path that contains `:` still parses. The `name` ends at the first `": "` after `kind`, and the `message` is everything from there to the fixed tail `" [confidence] (CODE)"`. The tail is unambiguous because the confidence vocabulary is closed and the code shape is fixed, so a message may contain `[`, `]`, `(`, `)` and `:` freely.

## Escaping

The format has no escape mechanism. It carries every field verbatim, and it stays parseable through three constraints the producer keeps rather than through an escape mechanism:

- No field contains CR or LF. A path, a name or a message holding either cannot be rendered on one line, and the expression refuses the line.
- `name` does not contain the two-character sequence `": "`. A name that did would be cut at that point, and the rest would be read as message text.
- `kind` is one token from the vocabulary above, so it never contains a space.

Nothing else is constrained. A path with a space, a colon or a non-ASCII character renders as it is; a message with a colon renders as it is; a TypeScript member declared with a string-literal key renders with the quotes its `symbol.name` carries.

## Three cases with no special form

**A finding with no column does not exist.** Every finding carries a line and a column, the document-level ones included: a file nothing builds or imports (`DS1501`, `DS1502`), a dependency or module directive (`DS1601`, `DS1605`), an unmatched root (`DS1704`) and a stale edge (`DS1705`) each render at the line and column `report.schema.json` fixes for that record, which is the declaring line where one exists (the `require` line in `go.mod`, the entry in `package.json`) and the document's fixed position otherwise. So the format has no `path:line:` form, and a reporter never emits one. A reporter renders `line` and `col` from the finding's `position.line` and `position.column`, never from a position type that omits a zero column. Rendered:

```text
go.mod:12:2: dependency github.com/example/left: required by no package in the build list [certain] (DS1601)
internal/legacy/render_windows.go:1:1: file internal/legacy/render_windows.go: built under no declared configuration [certain] (DS1501)
```

**A stale suppression renders as a finding under `DS1703`.** A suppression that matches no current finding is a finding of its own, so it takes the same line shape as every other finding. Its position is the suppression's own site: the line of the directive comment for an inline directive, the entry's line in `deadset-ignore.json` for an ignore entry, and the row's line in `deadset-baseline.json` for a baseline row. Its `kind`, `name` and `message` are the fields `report.schema.json` maps to them for a stale-suppression record. Its confidence is `certain` and its code is always `DS1703`, because the kind is fixed on at `deny` and no configuration lowers it. Rendered, for an inline directive one line above the declaration it named:

```text
catalog.go:213:1: suppression go://example.com/app#Catalog.ResolveAlias: inline directive for DS1001 matches no current finding [certain] (DS1703)
```

**A pending finding is not rendered.** A finding that waits on a cross-language edge lives inside an edge evaluation, is neither reported nor suppressed, and appears in no `findings` array, so the text reporter writes no line for it. The run names the count instead: an analyzer holding at least one exits with code 4 and says how many, and the merged report of the orchestrator holds none. A suppressed finding is not rendered either; a matched suppression marks its symbol live before the sweep, so no finding exists to render, and the summary's suppression counts are where that is visible.

## Order, and the lines around the findings

The text reporter writes the report's `findings` in their report order, which is the canonical key `merge.md` defines, `(path, line, column, code, symbol.ref, analyzer.name)`; then the report's `stale_suppressions` in their report order. Where a sort by size is configured, the reporter orders findings by `component.deletable_lines` descending, then `symbol.size_lines` descending, then the canonical key, and stale suppressions follow unchanged. Where a maximum finding count is configured, the reporter writes the first lines in that order up to the maximum and then reports the number omitted.

Finding lines go to standard output. The summary, the omitted count, the deletable-line total, the remediation text and every load error are not finding lines, and none of them matches the expression below, so a filter on the expression yields exactly the finding lines. No timestamp, duration or host detail appears in the output, and two runs over an unchanged tree write the same bytes.

## The expression

The expression is written in the intersection of two dialects, RE2 as Go's `regexp` package implements it ([syntax](https://pkg.go.dev/regexp/syntax)) and ECMAScript `RegExp` with no flags, so one string compiles unchanged in a Go implementation and in a TypeScript one. Named groups use the `(?<name>...)` spelling both dialects accept (Go since 1.22). Character classes are spelled `[^\r\n]` rather than `.`, because `.` excludes different characters in the two dialects (LF only in RE2, four line terminators in ECMAScript) and the class makes them agree. Lazy quantifiers (`+?`) and the anchors `^` and `$` behave the same in both with no flags set: `$` matches only at the end of the input, so a line is matched without its terminator.

```text
^(?<path>[^\r\n]+?):(?<line>[1-9][0-9]*):(?<col>[1-9][0-9]*): (?<kind>[a-z][a-z-]*) (?<name>[^\r\n]+?): (?<message>[^\r\n]+?) \[(?<confidence>certain|probable|possible)\] \((?<code>DS[0-9]{4})\)$
```

| Group | Captures |
| --- | --- |
| `path` | `position.path`, by the leftmost split described above |
| `line` | `position.line` |
| `col` | `position.column` |
| `kind` | `symbol.kind` |
| `name` | `symbol.name`, up to the first `": "` |
| `message` | `message`, up to the fixed tail |
| `confidence` | `confidence` |
| `code` | `code` |

The expression is the format's definition: a product's text reporter conforms when every line it writes for a finding matches it, and when the groups equal the finding's fields. It is not a validator of the values inside the groups. That `kind` is in the vocabulary is the finding schema's check, made on the JSON report; that `code` is a live code and that `confidence` is not above the kind's ceiling are the analyzer's, because both read `kinds.json`, which the schema does not restate.

### Accepted

```text
catalog.go:214:6: method (*Catalog).ResolveAlias: exported method has no reference in the target and none from any loaded consumer [probable] (DS1001)
src/features/tabs/index.ts:1182:11: class-member TabStrip.cachedLayout: private member is written and never read [certain] (DS1301)
go.mod:12:2: dependency github.com/example/left: required by no package in the build list [certain] (DS1601)
internal/legacy/render_windows.go:1:1: file internal/legacy/render_windows.go: built under no declared configuration [certain] (DS1501)
catalog.go:213:1: suppression go://example.com/app#Catalog.ResolveAlias: inline directive for DS1001 matches no current finding [certain] (DS1703)
odd:name.go:3:1: function weird: message with [brackets] and (parens): still one line [possible] (DS1002)
src/x.ts:9:3: class-member Foo.'my key': private member is written and never read [certain] (DS1301)
```

The sixth line has a colon in the path and brackets, parentheses and a colon in the message; the leftmost split and the fixed tail parse it into `path` `odd:name.go`, `name` `weird` and the message `message with [brackets] and (parens): still one line`. The seventh has a space inside `name`.

### Refused

```text
catalog.go:214: method (*Catalog).ResolveAlias: exported method has no reference [probable] (DS1001)
catalog.go:214:6 method (*Catalog).ResolveAlias is unused (DS1001)
catalog.go:214:6: method (*Catalog).ResolveAlias: exported method has no reference (DS1001)
catalog.go:214:6: method (*Catalog).ResolveAlias: exported method has no reference [probable] DS1001
catalog.go:214:6: method (*Catalog).ResolveAlias: exported method has no reference [high] (DS1001)
catalog.go:214:6: method (*Catalog).ResolveAlias: exported method has no reference [probable] (EU1002)
catalog.go:0:6: method (*Catalog).ResolveAlias: exported method has no reference [probable] (DS1001)
DS1001: catalog.go:214:6: method (*Catalog).ResolveAlias: exported method has no reference [probable]
catalog.go:214:6: method (*Catalog).ResolveAlias: exported method has no reference [probable] (DS1001) 
```

In order: no column; no colon after the column and no confidence; no confidence; the code outside parentheses; a confidence outside the vocabulary; a code outside the `DS` code space; a zero line number; the code before the position; a trailing space after the closing parenthesis.

## A filter for a shell

POSIX extended regular expressions have neither named groups nor lazy quantifiers, so the expression above does not paste into `grep -E`. This one does, and it accepts every line the expression accepts:

```text
^.+:[1-9][0-9]*:[1-9][0-9]*: .+ \[(certain|probable|possible)\] \(DS[0-9]{4}\)$
```

It is a filter, not a parser: it selects the finding lines out of a run's output and refuses every line in the refused set above, but it captures no field. To count findings per code from a captured run:

```sh
grep -E '^.+:[1-9][0-9]*:[1-9][0-9]*: .+ \[(certain|probable|possible)\] \(DS[0-9]{4}\)$' run.txt |
  sed -E 's/^.* \((DS[0-9]{4})\)$/\1/' | sort | uniq -c
```

## What other documents fix

- [`finding.schema.json`](../finding.schema.json): the `position` object and the unit `column` counts, the `symbol.kind` vocabulary, `symbol.name` and `symbol.size_lines`, and `confidence`.
- [`report.schema.json`](../report.schema.json): the stale-suppression record and the fields it supplies to `kind`, `name`, `message` and the position; the fixed position of a document-level finding.
- [`kinds.json`](../kinds.json): the live codes and each kind's confidence ceiling.
- [`merge.md`](merge.md): the canonical key that orders the lines.
- [`exit-codes.json`](../exit-codes.json): the code a run returns alongside the lines, including code 4 for a report that holds a pending finding.
- [`contract.json`](../contract.json): the platform set the LF terminator rests on.
