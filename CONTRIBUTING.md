# Contributing to deadset-spec

The [shared rules](https://github.com/cplieger/.github/blob/main/CONTRIBUTING.md) for commits, releases, synced files and checks apply here.

## Releases

Mark a commit breaking exactly when it moves the major of `contract_version` in `contract/contract.json`. Any other breaking commit moves this Go module to a new `/vN` path. Every Go consumer then rewrites its imports for a contract it already implements.

A report schema major alone moves only the contract minor, so its commit is `feat:`.

## Generated files

`go generate` runs `deadset-gen`, which writes every file derived from the Contract's data, and a test fails when one is stale. Edit the source, never the output: `authoring/config.schema.json` for `contract/config.schema.json`, `contract/settings.json` and `vocab/config_gen.go`, and `authoring/config-vectors/` for the configuration vectors' resolved configurations. The configuration source states no list another document owns. `x-enum` names a value list, `x-per-language` writes one member per language of `contract/languages.json`, `x-vocabulary` names the vocabulary a key's values come from, and a `patternProperties` member named `{severity-keys}` stands for the pattern of the codes. `x-delivered` is left out, because the languages that receive a key follow from its `x-owner`; `false` keeps a key with the orchestrator.
