# Contributing to deadset-spec

The [shared rules](https://github.com/cplieger/.github/blob/main/CONTRIBUTING.md) for commits, releases, synced files and checks apply here.

## Releases

Mark a commit breaking exactly when it moves the major of `contract_version` in `contract/contract.json`. Any other breaking commit moves this Go module to a new `/vN` path. Every Go consumer then rewrites its imports for a contract it already implements.

A report schema major alone moves only the contract minor, so its commit is `feat:`.
