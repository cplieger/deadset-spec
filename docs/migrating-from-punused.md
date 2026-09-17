# Migrating from punused

punused reports unused exported Go symbols and takes its adjudications from a `.punused-ignore`
file, one literal substring per line with the reason in a `#` comment. deadset reports the same
symbols under the codes of its own code space and takes its adjudications from
`deadset-ignore.json`, one entry per adjudication with the reason in a field of the entry.

This page is the pass from one to the other. It is a pass by hand, per repository: no analyzer reads
another tool's ignore format, and none converts one (from `contract/grammar/suppression.md`, the
document). Most lines need no replacement, so do the pass in the order below rather than translating
line by line.

## The codes

Neither `EU1001` nor `EU1002` is a code in this space: every code here is the prefix `DS` followed
by four digits (from `contract/kinds.json`, `prefix`). Each maps into the space by what the symbol
is.

A line reading `is unused (EU1002)` maps by the kind of symbol it names:

| The symbol the line names | Code | Name |
| --- | --- | --- |
| An exported function, method, type, constant or variable | `DS1001` | `unused-exported` |
| An exported struct field or type member | `DS1003` | `unused-member` |
| An interface no symbol uses as a type | `DS1201` | `unused-interface` |
| A method of an interface that no call site invokes or selects through the interface | `DS1203` | `uncalled-interface-method` |

A line reading `is used in test only (EU1001)` maps to `DS1004`, `test-only-use`: a symbol with zero
production references and at least one test reference. It is not an adjudication in this contract but
a finding of its own, so a project sets its severity rather than writing an entry for it (from
`contract/kinds.json`, row `DS1004`, and `contract/config.schema.json`, `severity`).

Three codes take precedence over `DS1001` where they apply, and a finding is reported once, under the
most specific code (from `contract/kinds.json`, `derived_from`):

- `DS1103`, `unreachable-export`: nothing outside can import the enclosing package or file.
- `DS1006`, `deprecated-unused`: the symbol carries a deprecation marker.
- `DS1004`, `test-only-use`: every reference is a test reference.

Expect findings the old file has no line for, because two families have no `EU` counterpart and both
are on by default: `DS1002`, `unused-unexported`, reports an unexported symbol nothing references,
and the kinds in the `DS1800` to `DS1899` range report inside a function body (from
`contract/kinds.json`, row `DS1002` and the `intra-function` range).

## The pass

**1. Write the configuration first.** `deadset.json` at the target root names `target.kind`, which
has no default: a run whose sources all omit it exits with the usage code. For a library, this is
also where the consumer set is declared complete (from `contract/config.schema.json`, `target.kind`
and `consumers.complete`).

**2. Load the consumers.** A symbol whose callers live in another module is referenced once that
module is in the graph, so it produces no finding and needs no entry. The consumers come from an
already-populated workspace or from the scope the invocation passes, never from the configuration
file (from `contract/config.schema.json`, `consumers`). This step is what retires most lines in a
library's file.

**3. Run with the codes you are triaging at `warn`.** Name each code, or a two-digit family prefix,
under `severity` at `warn`, so a finding does not fail the run before it is adjudicated. `DS1703` is
fixed at `deny`: a key naming it, or the family prefix `DS17`, names a key the product does not
implement and the run ends with the usage code (from `contract/config.schema.json`, `severity`).

**4. Delete every line whose symbol an exemption class retains.** An exemption is a class of reason
for which the analyzer keeps a symbol the reference graph alone would report, and it needs no
adjudication at all: the symbol is not reported. The classes that cover the common shapes are
`interface-satisfaction` for a method that satisfies an interface a value of its type reaches,
`encoding-reflection` for a type an encoder or a reflective consumer inspects by name,
`format-verb-contract` for a string or error method a formatting verb calls, `errors-duck-typing` for
the comparison and unwrapping methods the standard error helpers reach, `enum-group` for a member of
an enumerated type whose values arrive by conversion, `generated-file` for every declaration in a
generated file, and `linkname-cgo-asm-plugin` for a symbol another compilation unit reaches by name.
[The exemption classes](exemptions.md) states each rule; `contract/exemptions.json` is the file it
reads.

**5. Write an entry for each finding that is left.** Carry the reason text from the line's `#`
comment into the entry's `reason` field.

**6. Read the run again.** An entry that matches nothing is reported as `DS1703`, so a file that
falls out of date is visible rather than silently inert (from `contract/kinds.json`, row `DS1703`).

## The entry

`deadset-ignore.json` at the target root, strict JSON with a closed key list:

```json
{
  "description": "Adjudications for example.com/app.",
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

Four keys, and no others (from `contract/grammar/suppression.md`, the entry):

| Key | What it names |
| --- | --- |
| `code` | The issue-kind code, `DS` and four digits. One code per entry. |
| `symbol` | The stable symbol reference of the declaration, in the grammar `contract/grammar/symbol-ref.md` states. Compared for equality with the finding's `symbol.ref`: no glob, no regular expression, no bare name. |
| `path` | The path of the file that holds the declaration, relative to the target root, with `/` as the separator. |
| `reason` | Why the finding is not to be reported. At least one character that is not whitespace. |

**Every entry names a path, and every entry carries a reason.** An entry with no `path` is reported
as `DS1702` rather than matched, so a bare name cannot mask a match anywhere else in the project. An
entry whose `reason` is absent, empty or whitespace only is reported as `DS1701`. Both are findings
at the entry's own position, which is the line and column of the `{` that opens it (from
`contract/grammar/suppression.md`, the entry).

An entry matches only the code, the symbol and the path it names, all three, so an adjudication
written for one symbol in one file masks nothing else.

## The other mechanism

An adjudication may sit in the source instead, as a comment on the line immediately above the
declaration, naming one or more codes and carrying the reason after `--`:

```go
//deadset:ignore DS1001 -- Reached only through the generated TypeScript client.
func (c *Catalog) ResolveAlias(name string) string {
```

The inline directive is scoped by its position, so it needs no path, and it carries the same reason
rule: a directive with no reason is reported as `DS1701` (from `contract/grammar/suppression.md`, the
inline directive). Put an adjudication inline when the reason belongs beside the declaration, and in
the file when a reader wants every adjudication of the target in one place.
