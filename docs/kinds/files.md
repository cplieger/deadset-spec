# Files: `DS1600` to `DS1699`

The file is never built or never imported. Source files nothing builds or imports. Generated from `contract/kinds.json`; the [issue kinds](../kinds.md) page lists every family.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1601` | `file-never-built` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1602` | `file-never-imported` | `go`, `ts` | `on` | `deny` | `deletable` |

## DS1601 file-never-built

A source file no configuration of the run builds: its build constraints exclude it everywhere.

Before:

```go
//go:build plan9

package fs // no configuration of the run targets plan9
```

After:

```go
// The file is deleted, or plan9 joins analysis.configurations.
```

How to fix it: Delete the file, or add the configuration that builds it to the build matrix. The finding's fixability is `deletable`.

The rule: A source file no configuration in the build matrix builds. On the Go side the finding names the build constraint that excluded the file. A file whose build constraint holds only when the ignore tag is set is never reported, because grammar/analysis/matrix.md keeps such a file outside the analysis wherever no configuration builds it: the tag is the toolchain's convention for a file kept out of ordinary package builds unless a configuration explicitly satisfies the tag, and built by hand, the toolchain applying no build constraint to a file named on its own command line. A file any other custom tag excludes is reported under a matrix the configuration declares complete, because completeness is the maintainer's assertion that the listed configurations are every one the target builds, and a configuration the maintainer builds by hand belongs in that list.

Before reporting: Reported only when the configuration declares the build matrix complete, and never for a file the toolchain ignored solely because it imports "C" under a build with cgo disabled. Such a file is recorded as excluded by cgo rather than as never built.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1602 file-never-imported

A source file nothing imports and no root names: no entry point loads it.

Before:

```ts
// src/old-banner.ts: no import names it, no entry or convention roots it
export const banner = "v1";
```

After:

```ts
// src/old-banner.ts is deleted.
```

How to fix it: Delete the file, or root it if a tool loads it by name. The finding's fixability is `deletable`.

The rule: A source file that no import from live code reaches, that no root names, and in which no declared cross-language edge names a declaration: an import counts where it is written in a file a root, a live declaration or such an import keeps, or in a live declaration, so a file only dead code imports is reported, under reachability. An import written in a loaded consumer reaches a file of the target as an import written in the target does. An edge's side names a declaration as a root names one, so a file holding a declaration an edge names is evaluated with the edge rather than reported: whether that declaration is dead is the edge evaluation's to say, and the merge resolves it against the paired side.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `possible` ([`confidence.20`](../../contract/grammar/analysis/confidence.md)): A declaration of a TypeScript file only tests reach that is no test-support file, and each dead declaration it reaches through dead declarations.
- `probable` ([`confidence.25`](../../contract/grammar/analysis/confidence.md)): A TypeScript declaration found dead under reachability alone, or a file only dead code imports, while ts.roots_complete is not true.
- `probable` ([`confidence.26`](../../contract/grammar/analysis/confidence.md)): A finding the roots cap applies to, in a configuration that compiles a load by a computed path.
- A finding about a member of a dead component carries no higher confidence than the component's root members.
