# Contributing to deadset-spec

The [shared rules](https://github.com/cplieger/.github/blob/main/CONTRIBUTING.md) for commits, releases, synced files and checks apply here.

## Releases

Mark a commit breaking exactly when it moves the major of `contract_version` in `contract/contract.json`. Any other breaking commit moves this Go module to a new `/vN` path. Every Go consumer then rewrites its imports for a contract it already implements.

A report schema major alone moves only the contract minor, so its commit is `feat:`.

## Generated files

`go generate` runs `deadset-gen`, which writes every file derived from the Contract's data, and a test fails when one is stale. Edit the source, never the output: `authoring/config.schema.json` for `contract/config.schema.json`, `contract/settings.json` and `vocab/config_gen.go`; `authoring/finding.schema.json`, `authoring/report.schema.json` and `authoring/describe.schema.json` for the schemas of the same name under `contract/`; `authoring/expect.schema.json` and `authoring/conformance-results.schema.json` for the schemas of the same name under `corpus/`; and `authoring/config-vectors/` for the configuration vectors' resolved configurations. The configuration source states no list another document owns, and every other source draws the lists its `x-enum` members name from their owner. `x-enum` names a value list, and an `enum` beside it adds the values the schema admits after the list's. `x-per-language` writes one member per language of `contract/languages.json` whose column `when` names, each one required unless `optional` is true. `x-inline` names an object definition of another source as `<path>#/$defs/<name>` and stands for its members less its description, so a corpus schema states inline a shape the Contract defines once. Generation refuses a definition that holds a `$ref` or an `x-inline` of its own. In the configuration source, a `patternProperties` member named `{severity-keys}` stands for the pattern of the codes, and a key's vocabulary is the list its `items` or its `propertyNames` name. `x-delivered` is left out, because the languages that receive a key follow from its `x-owner`; `false` keeps a key with the orchestrator.
