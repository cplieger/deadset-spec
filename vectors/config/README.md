# Configuration vectors

One case per directory, named for what it establishes. A case holds the documents a run reads and either the resolved configuration the run prints or the refusal it returns, so two implementations of configuration resolution are tested against declared data rather than against each other.

## The files of a case

| File | What it holds |
| --- | --- |
| `case.json` | the case's name, what it establishes, and the aspects it covers |
| `repository.json` | the repository configuration, the `deadset.json` at the target root; absent when the case has none |
| `central.json` | the central configuration the invocation names; absent when the case has none |
| `flags.json` | the settings a command-line flag supplied, each key one setting's dotted path; absent when the case passes none |
| `expected.json` | the resolved configuration the run prints |
| `expected-error.json` | the exit code the run returns and the key or field its message names |

A case carries exactly one of `expected.json` and `expected-error.json`.

To run a case, place `repository.json` at the target root as the repository configuration, pass `central.json` as the central configuration, supply each setting `flags.json` names through the flag that carries it, and ask for the resolved configuration. A case is compared as decoded values, so the indentation and the key order of a product's own output decide nothing.

## What a resolved configuration holds

`expected.json` names every key [`config.schema.json`](../../contract/config.schema.json) declares, in that schema's key order, each carrying the value resolution produced; the resolved `severity` object names its codes in ascending order. `contract_version` is the Contract version the product implements, which is the version a case that declares it declares. The closed key list admits no key of its own for commentary, so a case's documents carry none and `case.json` holds what the case establishes.

`provenance` carries one entry per setting: one for each key that holds a value, and one per code the resolved `severity` object names, or the key `severity` alone when that object is empty. A section holds no value of its own and has no entry. An entry's value is `default`, or `repository`, `central` or `flag` followed by a colon, a space and the source that supplied the value, which in these cases is the case's own file name or the flag.

## How the cases resolve

A flag outranks the repository configuration, which outranks the central configuration, which outranks the default the schema declares. A value replaces rather than merges, so an array from the higher-ranked source stands alone, while `severity` resolves per code, so a code only the central configuration names keeps its central value. The `provenance` key is accepted on input and ignored by resolution, so a resolved configuration read back as a repository configuration resolves to itself.

A document naming a key the closed key list does not declare, a document naming one key twice at the same level, a document naming one member of an object whose members are all required, and a run whose sources supply no target kind are each refused with the usage code [`exit-codes.json`](../../contract/exit-codes.json) names, before any analysis. The `names` field of `expected-error.json` is the key or field the message names, spelled as the document spells it: the dotted path to a nested key, and the key itself where a document writes a dot inside one key.
