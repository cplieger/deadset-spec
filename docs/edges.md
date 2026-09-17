# Cross-language edges

A cross-language edge declares that one symbol exists because a symbol in another language uses it.
The shape it exists for is a Go type whose only consumer is the TypeScript client generated from it:
nothing in the Go program references the type, so a Go analysis alone reports it as unused, and
nothing in the TypeScript program declares it, so a TypeScript analysis alone cannot say whether it
is still needed.

An edge is a pair of language-tagged symbol references, and the pair names no language in its own
shape, so a further language joins without a change of form. A maintainer or a code generator
declares an edge; nothing infers one (from `contract/grammar/merge.md`, the vocabulary).

Neither analyzer resolves an edge. An analyzer reads only the sides of an edge that carry its own
language and never resolves the paired symbol, because it holds no type information for the other
language and acquires none. Each one publishes its own side's verdict instead, and the merge, which
reads reports and no source at all, is where the pair is decided (from `contract/grammar/merge.md`,
the vocabulary and step 4). This page is for a maintainer declaring an edge.

## The edges document

`deadset-edges.json` at the target root, read by every analyzer the run invokes. The merge reads no
edges document: it works from the evaluations the reports carry, which is what makes it a function of
its inputs (from `contract/grammar/merge.md`, the opening paragraph).

```json
{
  "description": "Each edge pairs a Go wire type with the TypeScript generated from it.",
  "edges": [
    {
      "id": "wire/ServerEvent",
      "because": "generated",
      "provides": "go://example.com/app#ServerEvent",
      "used_by": "ts://@example/app/src/wire.ts#ServerEvent"
    }
  ]
}
```

| Member | Meaning |
| --- | --- |
| `description` | What a file header comment would have carried. |
| `edges` | The declared edges, in document order. May be empty. |
| `id` | The edge's identifier, one or more `/`-separated segments of ASCII letters, digits, `_`, `.` and `-`. It is the identifier every evaluation of this edge names, and the merge orders and groups records by it (from `contract/report.schema.json`, `edge_evaluations[].edge`). |
| `because` | Why the pair exists, for whoever reads the document next. |
| `provides` | The stable symbol reference of the symbol the pair's other side uses. |
| `used_by` | The stable symbol reference of the symbol that uses it. |

An edge has exactly two sides, `provides` and `used_by`; `provides` exists because `used_by` uses it
(from `contract/report.schema.json`, `edge_evaluations[].side`). Each side names one symbol in the
grammar `contract/grammar/symbol-ref.md` states, which is language-tagged and carries no line
number, so an edge survives every edit above the declarations it names.

Both references are exact. An edge admits no pattern, no glob and no bare name, for the same reason
an ignore entry does not: a declaration broader than one symbol would silence findings nobody
declared (from `contract/grammar/symbol-ref.md`, patterns).

## One evaluation per side, and three states

For each side of each declared edge that carries its own language, an analyzer publishes exactly
one record in its report's `edge_evaluations` array, naming the edge, the side, the symbol and that
side's state:

- `live`: the analyzer enumerates the symbol and holds no finding for it.
- `dead`: the analyzer enumerates the symbol and holds a finding for it that only a live paired
  symbol could cancel. The record carries that finding.
- `absent`: the analyzer does not enumerate the symbol at all. The record carries no finding.

A side that no analyzer evaluated has no record and is not `absent`, because nothing looked (from
`contract/report.schema.json`, `edge_evaluations`). A side holds more than one record when more
than one analyzer claims its language, and the array is not deduplicated.

## The pending finding

The finding a `dead` record carries is the pending finding. It sits inside the evaluation and
appears nowhere else in the report, so it is neither reported nor suppressed: the report says the
symbol is dead on this side and says nothing about whether it should be deleted, because that
answer is on the other side. A report that holds one is a report with a pending finding, and the
analyzer that wrote it exits with the pending code, 4 (from `contract/exit-codes.json`, code 4, and
`contract/report.schema.json`, `edge_evaluations[].finding`).

A pending finding is not a weaker finding. Its `confidence` and every other field are what the kind
declares; what is unresolved is the pairing, not the analysis.

A narrowing candidate is published the same way. A declared edge counts as a reference from outside
the symbol's own package, so when it is the only such reference the analyzer publishes the narrowing
finding inside the edge evaluation rather than reporting it, and a wire type consumed only through
its generated client is never reported as unnecessarily exported (from `contract/kinds.json`, rows
`DS1101` and `DS1102`).

## What the merge does with each pairing

The merge takes every `dead` evaluation in canonical order and reads the strongest state on the
other side, in the order `live`, then `dead`, then `absent`. The pairings, which read the same
whichever side is which:

| One side | The other side | What the merge does |
| --- | --- | --- |
| `live` | `live` | Nothing. No side holds a pending finding. |
| `dead` | `live` | Drops the pending finding. The pair is live, so the symbol stays. |
| `dead` | `dead` | Promotes each pending finding into the merged report's findings and unions the paired symbols' components, so one deletion covers both sides. |
| `dead` | `absent` | Drops the pending finding and reports the edge as `DS1705`. A misspelled reference must not turn a live pairing into a deletion, so the stale edge is the defect to fix first. |
| `live` | `absent` | Reports the edge as `DS1705`. |
| `absent` | `absent` | Reports the edge as `DS1705`, once for the edge however many sides are absent. |
| `dead` | no record at all | Ends the run with the failure code, 3, naming the unresolved edge, the pending side and its symbol. No report in the merge evaluated the paired side, so no answer exists. |
| `live` | no record at all | Nothing. Nothing looked at the paired side, so there is nothing to report. |

From `contract/grammar/merge.md`, steps 4 and 5. `DS1705` is `stale-cross-language-edge`, and only
the merge emits it: an analyzer never does, whatever it finds on its own side (from
`contract/kinds.json`, row `DS1705`).

The merged report carries every evaluation whose state is `live` or `absent` and none whose state is
`dead`, so it holds no pending finding and its verdict is an answer rather than an input (from
`contract/grammar/merge.md`, step 6).

## Reading a stale edge

`DS1705` says one thing: an edge names a symbol that no analyzer claiming its language enumerates.
Three edits produce it, and the finding names the edge, each side's symbol and each side's state so
a reader can tell which:

- The reference is misspelled, or it was written in a form the grammar does not define.
- The symbol was renamed or moved, which changes its reference.
- The symbol was deleted, and the edge is the record that outlived it.

`DS1705` carries the `deny` severity, so a run that emits one returns the findings code, 1 (from
`contract/kinds.json`, row `DS1705`, and `contract/exit-codes.json`, code 1).
