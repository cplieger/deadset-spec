# Migrating from punused

punused reports unused exported Go symbols and takes its adjudications from a `.punused-ignore` file, one literal substring per line with the reason in a `#` comment. deadset reports the same symbols under the codes of its own code space and takes its adjudications from `deadset-ignore.json`, one entry per adjudication with the reason in a field of the entry.

This page is the pass from one to the other. It is a pass by hand, per repository. No analyzer reads another tool's ignore format, and none converts one. The source is `contract/grammar/suppression.md`, the document. Most lines need no replacement, so do the pass in the order below rather than translating line by line.

## The codes

Neither `EU1001` nor `EU1002` is a code in this space, because every code here is the prefix `DS` followed by four digits. The source is `contract/kinds.json`, `prefix`. Each maps into the space by what the symbol is.

A line reading `is unused (EU1002)` maps by the kind of symbol it names:

| The symbol the line names | Code | Name |
| --- | --- | --- |
| An exported function, method, type, constant or variable | `DS1001` | `unused-exported` |
| An exported struct field or type member | `DS1003` | `unused-member` |
| An interface no symbol uses as a type | `DS1201` | `unused-interface` |
| A method of an interface that no call site invokes or selects through the interface | `DS1203` | `uncalled-interface-method` |

A line reading `is used in test only (EU1001)` maps to `DS1004`, `test-only-use`, a symbol with zero production references and at least one test reference. In this contract it is a finding of its own, not an adjudication, so a project sets its severity instead of writing an entry for it. The sources are `contract/kinds.json`, row `DS1004`, and `contract/config.schema.json`, `severity`.

Three codes take precedence over `DS1001` where they apply, and a finding is reported once, under the most specific code. The source is `contract/kinds.json`, `derived_from`. The three codes are these:

- `DS1103`, `unreachable-export`, when nothing outside can import the enclosing package or file.
- `DS1006`, `deprecated-unused`, when the symbol carries a deprecation marker.
- `DS1004`, `test-only-use`, when every reference is a test reference.

Expect findings the old file has no line for, because two families have no `EU` counterpart and both are on by default. `DS1002`, `unused-unexported`, reports an unexported symbol nothing references. The kinds in the `DS1800` to `DS1899` range report inside a function body. The source is `contract/kinds.json`, row `DS1002` and the `intra-function` range.

## The pass

### 1. Write the configuration first

`deadset.json` at the target root names `target.kind`, which has no default, so a run whose sources all omit it exits with the usage code. For a library, this is also where the consumer set is declared complete. The source is `contract/config.schema.json`, `target.kind` and `consumers.complete`.

### 2. Load the consumers

A symbol whose callers live in another module is referenced once that module is in the graph, so it produces no finding and needs no entry. The consumers come from an already-populated workspace or from the scope the invocation passes, never from the configuration file. The source is `contract/config.schema.json`, `consumers`. This step is what retires most lines in a library's file.

### 3. Run with the codes you are triaging at `warn`

Name each code, or a two-digit family prefix, under `severity` at `warn`, so a finding does not fail the run before it is adjudicated. `DS1703` is fixed at `deny`. A key naming it, or the family prefix `DS17`, names a key the product does not implement, and the run ends with the usage code. The source is `contract/config.schema.json`, `severity`.

### 4. Delete every line whose symbol an exemption class retains

An exemption is a class of reason for which the analyzer keeps a symbol the reference graph alone would report. It needs no adjudication, because the symbol is not reported. These classes cover the common shapes:

- `interface-satisfaction`, a method that satisfies an interface a value of its type reaches.
- `encoding-reflection`, a type an encoder or a reflective consumer inspects by name.
- `format-verb-contract`, a string or error method a formatting verb calls.
- `errors-duck-typing`, the comparison and unwrapping methods the standard error helpers reach.
- `enum-group`, a member of an enumerated type whose values arrive by conversion.
- `generated-file`, every declaration in a generated file.
- `linkname-cgo-asm-plugin`, a symbol another compilation unit reaches by name.

[The exemption classes](exemptions.md) states each rule, and `contract/exemptions.json` is the file it reads.

### 5. Write an entry for each finding that is left

Carry the reason text from the line's `#` comment into the entry's `reason` field.

### 6. Read the run again

An entry that matches nothing is reported as `DS1703`, so a file that falls out of date is visible rather than silently inert. The source is `contract/kinds.json`, row `DS1703`.

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

An entry has four keys and no others. The source is `contract/grammar/suppression.md`, the entry.

| Key | What it names |
| --- | --- |
| `code` | The issue-kind code, `DS` and four digits. One code per entry. |
| `symbol` | The stable symbol reference of the declaration, compared for equality with the finding's `symbol.ref`. |
| `path` | The path of the file that holds the declaration, relative to the target root, with `/` as the separator. |
| `reason` | Why the finding is not to be reported. At least one character that is not whitespace. |

The `symbol` value follows the grammar `contract/grammar/symbol-ref.md` states, and it admits no glob, no regular expression and no bare name.

Every entry names a path, and every entry carries a reason. An entry with no `path` is reported as `DS1702` rather than matched, so a bare name cannot mask a match anywhere else in the project. An entry whose `reason` is absent, empty or whitespace only is reported as `DS1701`. Both are findings at the entry's own position, which is the line and column of the `{` that opens it. The source is `contract/grammar/suppression.md`, the entry.

An entry matches only the code, the symbol and the path it names, all three, so an adjudication written for one symbol in one file masks nothing else.

## The other mechanism

An adjudication may sit in the source instead, as a comment on the line immediately above the declaration, naming one or more codes and carrying the reason after `--`:

```go
//deadset:ignore DS1001 -- Reached only through the generated TypeScript client.
func (c *Catalog) ResolveAlias(name string) string {
```

The inline directive is scoped by its position, so it needs no path, and it carries the same reason rule. A directive with no reason is reported as `DS1701`. The source is `contract/grammar/suppression.md`, the inline directive. Put an adjudication inline when the reason belongs beside the declaration, and in the file when a reader wants every adjudication of the target in one place.
