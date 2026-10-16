# Configuration vectors

One case per directory, named for what it establishes. A case holds the documents a run reads and either the resolved configuration the run prints or the refusal it returns, so two implementations of configuration resolution are tested against declared data rather than against each other.

## The files of a case

| File | What it holds |
| --- | --- |
| `case.json` | the case's name, what it establishes, the aspects it covers, and the products it applies to where it does not apply to every product |
| `repository.json` | the repository configuration, the `deadset.json` at the target root; absent when the case has none |
| `central.json` | the central configuration the invocation names; absent when the case has none |
| `flags.json` | the settings a command-line flag supplied, each key one setting's dotted path; absent when the case passes none |
| `expected.json` | the resolved configuration the run prints, generated (below) |
| `expected-error.json` | the exit code the run returns and the key or field its message names |

A case carries exactly one of `expected.json` and `expected-error.json`.

To run a case, place `repository.json` at the target root as the repository configuration, pass `central.json` as the central configuration, supply each setting `flags.json` names through the flag that carries it, and ask for the resolved configuration. A case is compared as decoded values, so the indentation and the key order of a product's own output decide nothing.

## What a resolved configuration holds

`expected.json` names every key [`config.schema.json`](../../contract/config.schema.json) declares, in that schema's key order, each carrying the value resolution produced; the resolved `severity` object names its codes in ascending order. `contract_version` is the Contract version the product implements, which is the version a case that declares it declares. The closed key list admits no key of its own for commentary, so a case's documents carry none and `case.json` holds what the case establishes.

`deadset-gen` writes each `expected.json` from [`authoring/config-vectors/<case>.json`](../../authoring/config-vectors), which states only what the case decides: the value of each setting a source supplies and its `provenance` entry, in the resolved configuration's own shape. Every other key takes the default [`settings.json`](../../contract/settings.json) gives it and the provenance `default`, so a key the closed key list gains reaches every case when the files are generated again. A case whose `repository.json` names every key states `x-input` instead: an object is that input's own `provenance` member and every setting the case does not name comes from the repository at its default, and a case name reads that case's `expected.json` back as the input. Both files of such a case are generated.

`provenance` carries one entry per setting: one for each key that holds a value, and one per code the resolved `severity` object names, or the key `severity` alone when that object is empty. A section holds no value of its own and has no entry. An entry's value is `default`, or `repository`, `central` or `flag` followed by a colon, a space and the source that supplied the value, which in these cases is the case's own file name or the flag.

## Which products run a case

A case applies to every product unless its `case.json` lists `products`, the products whose run of the case has the outcome it states. The orchestrator checks the whole document; an analyzer checks and resolves only what its own language's analysis reads, the keys [`settings.json`](../../contract/settings.json) `read_by` names its language for and, of a key written per language, its own member, and accepts every other key unread. So a refusal of a key only another product reads lists the products that read it, and an analyzer compares a resolved case on the keys its language reads alone: those keys of `expected.json`, a key written per language narrowed to its own member, and their `provenance` entries.

## How the cases resolve

A flag outranks the repository configuration, which outranks the central configuration, which outranks the default the schema declares. A value replaces rather than merges, so an array from the higher-ranked source stands alone, while `severity` resolves per code, so a code only the central configuration names keeps its central value. `target.kind` is one setting in either of its shapes, so a string from the higher-ranked source replaces an object, an object replaces a string, and no member of an object comes from a lower-ranked source. The `provenance` key is accepted on input and ignored by resolution, so a resolved configuration read back as a repository configuration resolves to itself.

A document naming a key the closed key list does not declare, a document naming one key twice at the same level, a document naming one member of an object whose members are all required, a severity key naming no live kind, an integer setting written with a fraction or an exponent, a key written as null, a component extension that does not start with a full stop, a provider list naming one analyzer name twice, a target kind written per language that names a member the key does not declare, leaves a language out or gives a language a value other than `application` or `library`, and a run whose sources supply no target kind are each refused with the usage code [`exit-codes.json`](../../contract/exit-codes.json) names, before any analysis. The `names` field of `expected-error.json` is the key or field the message names, spelled as the document spells it: the dotted path to a nested key, and the key itself where a document writes a dot inside one key. A refusal of one entry of a list names the entry: the key followed by the entry's index in square brackets, counted from zero, and the path within the entry where the entry is an object, as `providers.analyzers[1].name` does. A target kind written per language names the member at fault, `target.kind.ts` for the TypeScript member, and where one object holds more than one fault, the first in the order `config.schema.json` states for that key.
