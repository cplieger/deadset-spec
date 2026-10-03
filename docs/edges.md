# Cross-language edges

A cross-language edge declares that one symbol exists because a symbol in another language uses it. The shape it exists for is a Go type whose only consumer is the TypeScript client generated from it. Nothing in the Go program references the type, so a Go analysis alone reports it as unused. Nothing in the TypeScript program declares it, so a TypeScript analysis alone cannot say whether it is still needed. This page is for a maintainer declaring an edge.

An edge is a pair of language-tagged symbol references. The pair names no language in its own shape, so a further language joins without a change of form. A maintainer or a code generator declares an edge, and nothing infers one. The source is `contract/grammar/merge.md`, the vocabulary.

Neither analyzer resolves an edge. An analyzer reads only the sides of an edge that carry its own language and never resolves the paired symbol, because it holds no type information for the other language and acquires none. Each one publishes its own side's verdict instead. The merge reads reports and no source at all, and it is where the pair is decided. The source is `contract/grammar/merge.md`, the vocabulary and step 4.

## The edges document

The edges document is `deadset-edges.json` at the target root, and every analyzer the run invokes reads it. The merge reads no edges document. It works from the evaluations the reports carry, which is what makes it a function of its inputs. The source is `contract/grammar/merge.md`, the opening paragraph.

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
| `id` | The edge's identifier, one or more `/`-separated segments of ASCII letters, digits, `_`, `.` and `-`. |
| `because` | Why the pair exists, for whoever reads the document next. |
| `provides` | The stable symbol reference of the symbol the pair's other side uses. |
| `used_by` | The stable symbol reference of the symbol that uses it. |

Every evaluation of an edge names its `id`, and the merge orders and groups records by it. The source is `contract/report.schema.json`, `edge_evaluations[].edge`.

An edge has exactly two sides, `provides` and `used_by`, and `provides` exists because `used_by` uses it. The source is `contract/report.schema.json`, `edge_evaluations[].side`. Each side names one symbol in the grammar `contract/grammar/symbol-ref.md` states. A reference in that grammar is language-tagged and carries no line number, so an edge survives every edit above the declarations it names.

A side may name any declaration, a type or a member. An edge that names a type says nothing about that type's members. Each member is judged on its own, or through an edge whose side names it, so an edge on a type holds none of the type's members live. The source is `contract/grammar/merge.md`, the vocabulary.

Both references are exact. An edge admits no pattern, no glob and no bare name, for the same reason an ignore entry does not. A declaration broader than one symbol would silence findings nobody declared. The source is `contract/grammar/symbol-ref.md`, patterns.

## One evaluation per side, and three states

For each side of each declared edge that carries its own language, an analyzer publishes exactly one record in its report's `edge_evaluations` array, naming the edge, the side, the symbol and that side's state:

- `live`: the analyzer enumerates the symbol and holds no finding for it.
- `dead`: the analyzer enumerates the symbol and holds a finding for it that only a live paired symbol could cancel. The record carries that finding.
- `absent`: the analyzer does not enumerate the symbol at all. The record carries no finding.

A side that no analyzer evaluated has no record and is not `absent`, because nothing looked. The source is `contract/report.schema.json`, `edge_evaluations`. A side holds more than one record when more than one analyzer claims its language, and the array is not deduplicated.

## The pending finding

The finding a `dead` record carries is the pending finding. It sits inside the evaluation and appears nowhere else in the report, so it is neither reported nor suppressed. The report says the symbol is dead on this side and says nothing about whether it should be deleted, because that answer is on the other side. A report that holds one is a report with a pending finding, and the analyzer that wrote it exits with the pending code, 4. The sources are `contract/exit-codes.json`, code 4, and `contract/report.schema.json`, `edge_evaluations[].finding`.

A pending finding is not a weaker finding. Its `confidence` and every other field are what the kind declares. What is unresolved is the pairing, not the analysis.

A narrowing candidate is published the same way. A declared edge counts as a reference from outside the symbol's own package. When it is the only such reference, the analyzer publishes the narrowing finding inside the edge evaluation rather than reporting it. A wire type consumed only through its generated client is therefore never reported as unnecessarily exported. The source is `contract/kinds.json`, rows `DS1101` and `DS1102`.

A file is evaluated with the edge too. A declared edge names a declaration of the file that holds it, as a root does. While the edge names one of them, a file whose only declarations are edge-paired is not reported as never imported. The declaration's own evaluation carries the answer, and a pair the merge finds dead is reported through the declaration's finding. The source is `contract/kinds.json`, row `DS1502`.

## What the merge does with each pairing

The merge takes every `dead` evaluation in canonical order and reads the strongest state on the other side, in the order `live`, then `dead`, then `absent`. The pairings, which read the same whichever side is which:

| One side | The other side | What the merge does |
| --- | --- | --- |
| `live` | `live` | Nothing. No side holds a pending finding. |
| `dead` | `live` | Drops the pending finding's component. The pair is live, so the symbol stays, and so does everything it references. |
| `dead` | `dead` | Promotes each pending finding into the merged report's findings and unions the paired symbols' components, so one deletion covers both sides. |
| `dead` | `absent` | Drops the pending finding's component and reports the edge as `DS1705`. |
| `live` | `absent` | Reports the edge as `DS1705`. |
| `absent` | `absent` | Reports the edge as `DS1705`, once for the edge however many sides are absent. |
| `dead` | no record at all | Ends the run with the failure code, 3, naming the unresolved edge, the pending side and its symbol. |
| `live` | no record at all | Nothing. Nothing looked at the paired side, so there is nothing to report. |

Dropping a component drops every finding its report carries in that component, pending or not. A `dead` and `dead` pairing promotes nothing when the component of either finding is dropped on another edge. A `dead` and `absent` pairing drops the component rather than promoting it, because a misspelled reference must not turn a live pairing into a deletion. The stale edge is the defect to fix first. A `dead` side with no record on the other side ends the run, because no report in the merge evaluated the paired side and no answer exists.

A drop reaches every edge the dropped component holds a pending finding on. Once a component is dropped, each of its `dead` evaluations reads as `live` on its own edge. A member of a component dropped on one edge therefore keeps the symbol paired with it on another edge from being promoted, whichever rule dropped the component. The merge repeats the drops until no further component is dropped, and only then promotes what remains, so the outcome does not depend on the order it visits the edges in.

The source for the pairings and the drops is `contract/grammar/merge.md`, steps 4 and 5. `DS1705` is `stale-cross-language-edge`, and only the merge emits it. An analyzer never does, whatever it finds on its own side. The source is `contract/kinds.json`, row `DS1705`.

The merged report carries every evaluation whose state is `live` or `absent` and none whose state is `dead`. It holds no pending finding, so its verdict is an answer rather than an input. The source is `contract/grammar/merge.md`, step 6.

## Reading a stale edge

`DS1705` says that an edge names a symbol no analyzer claiming its language enumerates. Three edits produce it, and the finding names the edge, each side's symbol and each side's state so a reader can tell which:

- The reference is misspelled, or it was written in a form the grammar does not define.
- The symbol was renamed or moved, which changes its reference.
- The symbol was deleted, and the edge is the record that outlived it.

`DS1705` carries the `deny` severity, so a run that emits one returns the findings code, 1. The sources are `contract/kinds.json`, row `DS1705`, and `contract/exit-codes.json`, code 1.
