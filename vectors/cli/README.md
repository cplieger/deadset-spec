# Command-line vectors

Each case is one invocation of a product's command line over a small tree, and the outcome it must have. A case holds `case.json` and `tree/`, the files of the target. The runner copies `tree/` to a fresh directory, replaces `@TARGET@` in the arguments and in the text of the tree's files with that directory and `@OUT@` with a fresh output directory, runs each product `products` names with `argv` (or its own entry of `argv_by_product`) from the target directory, and compares:

- `exit`: the exit code.
- `stderr_contains`: strings standard error holds.
- `report`: `valid` when the report file must exist and be an instance of `contract/report.schema.json`, `absent` when no report may be written.
- `finding_paths`: the paths of the report's findings, in order.
- `finding_symbols`: the names of the report's findings' symbols, in order.
- `rename`: tree files the runner renames before the run, for a file this module cannot embed under its own name: a `go.mod`, which makes its directory another module, a Go source file, which would be a package of this module, a name Go does not embed, or a document the module's own checks would refuse.
- `repeat`: how many times the invocation runs, each run giving the same outcome.

A case with `pending` states a rule no product implements yet, named by the reason; the runner reports its result as pending, as the corpus does for a pending fixture. `deadset-corpus cases --product=NAME -- PRODUCT` runs every case naming a product.

| Case | What it pins |
| --- | --- |
| `cascade-full-edge-evaluation` | An analyzer's report under reporters.cascade full is an instance of the report schema: a finding an edge evaluation carries lists no component members, and its component is listed in the report's components. |
| `findings-ordered-bytewise-above-the-basic-plane` | A report lists findings in the bytewise order of their paths' UTF-8 encoding: a path holding U+FF5E sorts before one holding U+1F600, whose encoding starts with a higher byte, though the latter's UTF-16 code unit is lower. |
| `go-work-reader-under-a-scope-document` | A module of the target's workspace is a reader of the target whatever source names the target: a function the workspace's other module calls is live with or without a scope document, and a function nothing calls is reported. |
| `malformed-ignore-document` | An ignore file that is not a JSON document is a refused document: the run ends with the usage code 2, as suppression.md's refusals table states, in every analyzer. |
| `template-refusal-names-the-first-definition` | A template refused for its definitions names the first definition in the order the text writes them, whatever order a product holds them in, so two runs of one template print one refusal. |
| `third-scheme-entry-read-unchecked` | An ignore entry of a scheme no analyzer of the run implements is read unchecked: it binds nothing, is reported under no code, and the run ends as it would without it. |
| `typescript-configuration-under-a-dot-directory` | A compiler configuration in a directory whose name starts with a full stop puts TypeScript in scope, because the TypeScript row of languages.json skips only node_modules and .git, the directories its analyzer's discovery leaves unread: the run then needs an analyzer claiming ts, and a provider list naming only the Go analyzer is refused with the usage code 2. |
| `typescript-configuration-under-testdata` | A compiler configuration under a testdata directory puts TypeScript in scope, because only the Go row of languages.json skips testdata: the run then needs an analyzer claiming ts, and a provider list naming only the Go analyzer is refused with the usage code 2. |
| `typescript-sources-without-a-compiler-configuration` | A tree holding TypeScript sources and a manifest but no compiler configuration puts no language in scope, because TypeScript is in scope by a marker alone: the run holds no program and ends with exit code 3 and the setup failure no-program. |
| `unknown-exemption-class-refused-at-configuration` | exemptions.disabled admits only the classes of contract/exemptions.json, and the check is a configuration check: print-config refuses a class the vocabulary does not hold with the usage code 2, as analyze does. |
| `unreadable-named-configuration` | A configuration file the invocation names that cannot be read ends the run with the usage code 2, the file named, before any analysis. |
| `unreadable-named-scope-document` | A scope document the invocation names that cannot be read ends the run with the usage code 2, the file named, before any analysis. |
| `usage-names-the-verbs-of-the-command-line` | A verb the orchestrator's row of cli.json does not name is refused with the usage code 2, and the usage text names every verb of that row with its summary. |
