# Dependencies and module files: `DS1700` to `DS1799`

The declared dependency or module directive changes nothing. Declared dependencies and module-file directives that are exactly a no-op. Generated from `contract/kinds.json`; the [issue kinds](../kinds.md) page lists every family.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1701` | `unused-dependency` | `go`, `ts` | `on` | `deny` | `manual` |
| `DS1702` | `unused-module-directive` | `go` | `on` | `warn` | `deletable` |

## DS1701 unused-dependency

A declared dependency nothing in the program imports or runs.

Before:

```ts
"dependencies": {
  "left-pad": "1.3.0",
  "zod": "4.1.0"
}
```

After:

```ts
"dependencies": {
  "zod": "4.1.0"
}
```

How to fix it: Remove it from the manifest or the module file. The finding's fixability is `manual`.

The rule: A directly declared dependency that no import in the target needs. On the Go side, a direct require whose module provides no package any target package or test variant imports. On the TypeScript side, a manifest dependency, development dependency or peer dependency the project's import closure does not need. A deletion finding whose fix would remove the last use of a dependency names that dependency. On the TypeScript side, a needed dependency whose every use is written inside declarations that are members of the run's dead components, or at the top level of a file reported under DS1602, that no command runs and that no needed dependency used otherwise names as a peer, is reported as well, dead only through that code: at the reachability class possible where one of those declarations belongs to a library's published API, and found under reachability alone where one of those declarations or files is, so at probable unless ts.roots_complete is true. A use the configuration makes, a use an unanswered specifier may make and an import none of whose bindings is used are uses by live code.

Before reporting: On the Go side the rule is the semantics of `go mod tidy -diff` exactly: a requirement marked indirect is never reported, because it exists to pin a transitive version and removing it changes the build list. On the TypeScript side the import closure is the files of the run's projects, and a call of `import.meta.resolve` or of `require.resolve` in them, whatever `require` names, whose first argument is a string literal or a template literal with no substitution needs each dependency an import of that specifier needs. A `@types/<name>` package in a type root of a compiler configuration that lists no `types`, a `node_modules/@types` directory at or above the configuration's directory or a `typeRoots` entry naming one, is needed by a use, in a file of that configuration, of a global name or an ambient module that one of the package's declaration files declares, which resolves to no declaration of the program: a compiler before version 6 includes every package of the type roots where `types` is not listed, and version 6 and later include none, so the program the analysis builds holds none of those declarations. A package's declaration files are the file its `types` or `typings` names, else its `index.d.ts`, and each file a `/// <reference path>` of one of them reaches inside the package. A global is a top-level declaration of such a file that is no module, or a declaration of a `declare global` block of one that is, and an ambient module is a `declare module` with a string name of a file that is no module. A name that names a member, a property or a label is no use of a global. Four more dependencies are needed: one whose command a script entry, a workflow step or a git hook runs, one that a string of a configuration file names, and one that a string names under a manifest key or a package key of an applied convention row, each as grammar/analysis/conventions.md states, and one that a needed dependency names as a peer, as stated below. A command no script entry, workflow step or git hook runs needs nothing. A declared dependency that the installed manifest of a needed declared dependency names in its peerDependencies is needed, and so on until no further dependency joins. A peer dependency is a package the needed dependency expects beside it at a version its manifest states, whether or not it imports the package, and the project's declaration is what keeps that package present at a version the project chooses, so removing the declaration can break the needed dependency where nothing the analysis reads imports the peer. A peer that peerDependenciesMeta marks optional is needed in the same way, because the needed dependency's manifest still names it as a package it works with, and a package manager may leave an optional peer out where the project does not declare it, so the project's declaration is what keeps it present for the needed dependency. A declared dependency that only dependencies nothing needs name as a peer is needed by nothing. A dependency of a manifest whose directory holds source only in configurations the run lists in configurations_not_built is reported at the reachability class possible, because the source that may import it was not analyzed. An installed manifest is the package's manifest in the nearest node_modules directory at or above the directory of the manifest that declares the dependency, and a dependency with none is decided by the import closure alone. On the TypeScript side, the dependencies judged are those of every manifest grammar/analysis/program.md says the analysis reads, a nested package's among them, each read against its own directory: a dependency is needed by a use written in a file at or below the directory of the manifest that declares it where no manifest between them declares a dependency of the same name, so a use below a nested manifest that declares the same dependency needs that nested manifest's declaration only, by a use no one file holds, and as a peer of a needed dependency of the same manifest or of a manifest below it. A dependency a nested package's manifest declares whose installed manifest the analysis cannot find is not judged, because no run requires a nested package to be installed and its commands and peers cannot be read, while a workspace member's and the target's own are judged as above.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1702 unused-module-directive

A module-file directive that changes nothing: a replacement or an exclusion no requirement it names reaches.

Before:

```go
replace example.com/old => ../old // no requirement names example.com/old
```

After:

```go
// The replace directive is deleted.
```

How to fix it: Delete the directive. The finding's fixability is `deletable`.

The rule: A replace directive whose target module is absent from the build list, the one case in which the directive is exactly a no-op.

Before reporting: Only a replace whose target is absent from the build list. No other module-file directive is reported. An exclude directive is not, because not selected is a counterfactual about a resolution that did not happen. A workspace use entry and a tool directive are not, because each is an external entry point whose invocation lives outside anything the analysis reads.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.
