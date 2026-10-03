# The analysis

This page states the rules every analyzer applies to decide what the program is, which of it is test code, what is live without a reference, what a type error withholds, which failures end a run and which hints a report carries. A rule here holds for both languages unless it names one. The vocabulary the rules report in is [`kinds.json`](../kinds.json), the configuration keys they read are [`config.schema.json`](../config.schema.json), the report members they fill are [`report.schema.json`](../report.schema.json) and the exit codes they end with are [`exit-codes.json`](../exit-codes.json).

Every rule on this page that adds a root, a use or a retention only ever withholds a finding: it never produces one, never changes a finding's code and never raises a finding's severity or confidence. The rules that end a run end it before any finding list exists.

## The program

The program is the source the analysis holds for the target and its declared consumers under each configuration of the run. A run that holds no program, a target with no source file of its language or a configuration that names no file, ends with exit code 3.

### Component files

A file whose name ends with an extension `ts.component_extensions` lists is a component file. A component file is in a project's program when the project's file list names it or an import of the program resolves to it, and an import of a component file resolves to that file. The analysis reads a component file as markup holding script blocks, and it parses no markup:

- A `<script>` element whose start tag begins a line at the top level of the file is a block. Its content, up to the matching `</script>` end tag, is source of the language its `lang` attribute names: `ts` and `tsx` are TypeScript, and a block with no `lang`, or with `js` or `jsx`, is JavaScript.
- A file whose first line is `---` holds a frontmatter block: the lines after it up to the next line that is `---` are a TypeScript block.
- A `<script>` element with a `src` attribute, in its self-closing form or with an end tag, is a namespace import of the file `src` names, positioned at the attribute.
- The blocks of one file are one module, read in file order, of the strongest language any block names, in the order TSX, TypeScript, JSX, JavaScript. A file that holds no block is an empty module.
- A `<script` start tag that begins a line and is not read as a block, or that begins a line inside a block, is printed as a warning on standard error naming the file and the line, and the file is analyzed without it.

Every top-level binding of a component file's module is retained while the file is live, because the markup may use it: a declaration, and an import binding, which is resolved to the declaration it names, a namespace or default import to the module or the export it names. No finding is reported about a retained binding, `DS1104` included. Everything the module references counts as a use. The markup of a live component file is template text for the `template-field` exemption of [`exemptions.json`](../exemptions.json), so a member whose name the markup holds is retained by that class. A type error in a block is a type error under the rule below.

### Workspace packages

A TypeScript workspace is declared by the `packages` sequence of a `pnpm-workspace.yaml` file, or by the `workspaces` member of a `package.json`, an array of globs or an object whose `packages` member is one. The analysis searches for the declaration from the target root upward to the first directory holding one, and no further than the root of the repository that holds the target. A member is a directory the globs name, minus the globs a `!` negates and never under a `node_modules` directory, that holds a `package.json` with a `name`; the directory holding the declaration is a member too.

An import whose package name is a member's resolves to the member's source, so a workspace is analyzed with no build and no path mapping:

1. Where the package manager linked the importing directory to another package of that name, and that package is not the member, the import keeps the resolution the compiler gives it, and so does an import from outside the member that the member's name does not link.
2. Otherwise, where the compiler's own resolution reaches a source file inside the member, that file stands. A declaration file stands only when no configuration of the member writes its output where the file is.
3. Otherwise the member's manifest decides: the `exports` entry the import's subpath selects, the exact key first and then the longest pattern, its targets tried in condition order; with no `exports`, `types`, `typings`, `main` and `module` for the package itself, or the subpath below the member's directory. Each target is read back to its source through the member's emit mappings: each compiler configuration of the member maps its output directory and its declaration directory to its root directory, or, with no root directory, to the configuration's own directory and then to the common directory of its input files, and an output file maps to the source file of the same name with the source extension the output extension stands for.
4. An import no candidate reads back to a source file is the setup failure `workspace-member-without-source` below.

A reference from every program that holds a member's file counts, so a declaration a sibling member uses is live. A finding about a member's file is reported only from a configuration that owns the file, one whose own file list names it; a configuration that holds the file only because an import reached it contributes references to it and no verdict about it. A member's manifest entries are roots as the target's own are, published unless the member's manifest marks it private.

## Test code

Test code is the test files and the test-support code. A test file is a Go file whose name ends with `_test.go`, and a TypeScript file `ts.test_files` names. Test-support code is a Go package, or a TypeScript file, that is not test code by name and that only test code imports: at least one test file imports it, directly or through other test-support code, and nothing else imports it. A Go package or a TypeScript file a library's published API reaches is not test-support code, because outside programs may import it.

Test code is judged with the tests:

- A reference from test code is a test reference, and a production sweep, which counts no test reference, judges no declaration of test code.
- A declaration of a test file is reported only under `DS1005`.
- A declaration of test-support code is reported under `DS1005` where a declaration of a test file would be. Otherwise it is live while test code references it, and it is reported under the unused code its visibility selects when nothing references it. `DS1004` never reports a declaration of test-support code.
- A part of a declaration of test code, a parameter, a receiver, a result, a statement, a store or a case, is reported only while the declaration is live in the run that counts test references. A declaration of test code nothing live references is not judged, so none of its parts is reported, and a test reported under `DS1005` is reported whole.

A file the analysis classified as test-support code is counted in the report's `test_file_rules` under the rule `test-support`.

## Roots

A root is live without a reference. Beside the roots `roots.patterns` and `ts.entry_files` configure, the entry points a manifest names and a library's published API, the TypeScript analysis marks the roots below. A root that names a file roots the file's top level and everything its export table names, a re-export walked to the declaration it carries, and is live under both liveness relations, because a program the project runs loads it.

### Configuration files by name

A file in a directory that holds a `package.json` is a configuration file when its name is `<stem>.config.<ext>` or `<stem>.<qualifier>.config.<ext>`, with `<ext>` one of `js`, `mjs`, `cjs`, `ts`, `mts` and `cts`, or `.<stem>rc.<ext>`, with `<ext>` one of `js`, `mjs` and `cjs`. `<stem>` and `<qualifier>` are each a run of letters, digits, hyphens and underscores. A configuration file in the program is a root.

A JSON configuration file is a file in a directory that holds a `package.json` whose name is `<stem>.config.json`, `<stem>.<qualifier>.config.json`, `.<stem>rc` or `.<stem>rc.json`. It holds no declaration and is read for its strings alone.

### Strings in configuration

In a configuration file and in a JSON configuration file, every string literal and every template literal with no substitution, at any depth, is read against the `package.json` of the file's directory and against the file's own directory:

- A string equal to the name of a dependency that manifest declares, or to that name followed by `/` and a subpath, is a use of the dependency.
- A string that starts with `./` or `../` and names, read against the file's directory, a source file of the program, directly or with one of the extensions the program's module resolution tries, roots that file.

A JSON object member's name is not read as a string.

### Script entries

Each value of the `scripts` member of a `package.json` the analysis reads, the target's and each workspace member's, is split at white space and at the shell operators `&&`, `||`, `;` and `|`, and each token, with one pair of enclosing quotes removed, is read against the manifest's directory. A token that names a file of the program, directly or through the emit mappings that read an output file back to its source, roots that file.

### Type-query aliases

An ambient global declaration in a declaration file whose type is `typeof import(<string>)[<string>]`, or `typeof import(<string>).<name>`, is an alias of the declaration that names the module the first string names exports under the second string or the name. A reference to the global is a reference to that declaration and to every link of the alias chain that reaches it, and the type query in the global's own declaration is not a reference. No finding is reported about the global declaration itself.

### Convention rows

An analyzer may carry a table of convention rows, each the data of one framework's file-system conventions. A row has a name, an enabling package, a version range in the semantic-versioning range syntax, the globs of its entry files, and the configuration properties that move a directory those globs name, each with the configuration files the property is read from. The table is part of the analyzer, versioned with it, and no configuration key adds a row.

A row applies when a `package.json` the analysis reads declares the enabling package as a dependency, a development dependency, a peer dependency or an optional dependency, and the version the installed package's own `package.json` names, found from that manifest's directory as the compiler resolves a package, is in the row's range. A declared enabling package whose installed manifest the analysis cannot find is the setup failure `missing-module`. An applied row's globs are matched against the directory of the manifest that declared the package, and every file they match is a root, as an entry of `ts.entry_files` is.

A property that moves a directory is read from the row's configuration file without running it: where the property's value in the file is a string literal or a template literal with no substitution, the globs read the moved directory; where the file does not set the property, the globs read the default directory; and where the value is anything else, the row is the setup failure `convention-not-literal`.

The configuration's own keys win. A row `ts.disabled_conventions` names does not apply, adds no root and raises no setup failure, and the entries of `ts.entry_files` are roots whether a row applies or not. A row only adds roots. The report lists each row it applied in `conventions_applied`.

## Confidence

A finding's reachability class states what the analysis can see of its subject's users. A subject of a library's published API, a public member of a type the published API names included, is `certain` when every declared consumer loads, `probable` when a declared consumer did not load, and `possible` when the run holds no consumer information. A library's internals, which outside programs cannot name, and every subject of an application are `certain`. `analysis.min_confidence` defaults to `probable`, so a run over a library with no declared consumer withholds the findings about its published API and reports every other finding; the dial is the one mechanism that withholds a published API, and no kind's default depends on the target kind.

## Type errors

A type error the compiler reports in source the program holds does not end the run. The function, method or file-level statement whose source holds the error's position is not evaluated: nothing is reported about it or about any part of it, every reference in it the compiler resolves still counts, and every declaration an expression of it that does not resolve could reach is retained, so the skip can only withhold a finding. A diagnostic the compiler marks as reporting unnecessary code is not a type error for this rule, in any file. Each type error that skipped something is listed in the report's `type_error_skips` with its file, its line and the compiler's message, printed on standard error, and rendered as a warning annotation by the `github` format; the exit code follows the findings.

A type error in a dependency's declarations, outside the program, is not the program's and skips nothing.

## Unanswered questions

A question the analysis asks its type checker that the checker fails to answer is never read as an answer. Every declaration the answer could have made live is held: no finding is reported about it, every liveness relation treats it as live, and a declaration that reads a held binding by name is held too. Each configuration in which at least one question went unanswered is listed in the report's `unanswered_questions` with the number of questions and of declarations held.

## Setup failures

A setup failure is a file or a component the analysis needs and the project does not provide. In a configuration the configuration document declares, it ends the run with exit code 3 before any finding list exists, because every missing piece hides real uses and a partial run would report live code as dead. A configuration the analysis derived is dropped instead, as a derived configuration it cannot build is: it is listed in the report's `configurations_not_built` with the failure's line as its `error`, and the run analyzes the configurations left. When the analysis drops every configuration it derived, no matrix is left to analyze, and a setup failure among them ends the run as a declared configuration's does. For each failure that ends the run, the run prints one line on standard error that starts with `setup failure:`, a space, the class, a colon and a space, and names what is missing and the fix:

- `setup failure: missing-module`: an import names a module the project expects to exist and nothing provides it. A module the project expects is a module of the target's own module or a path its configuration maps, a declared dependency, a workspace member, and a module under a directory the project's ignore rules leave to a generator or a build. The fix names the import, the file that writes it, and the generator, the build or the install to run.
- `setup failure: incomplete-module-sum`: the Go module sum file lacks a checksum the build needs. The fix is `go mod tidy` in the module that requires it.
- `setup failure: test-build-tag`: Go test files build under no configuration of the run, because a build constraint they carry is satisfied by none. The fix names the exact entry of `analysis.configurations` that builds them.
- `setup failure: missing-consumer`: a declared consumer is absent from the path the scope names, or its own dependencies are not installed. The fix names the consumer, the path and the install.
- `setup failure: workspace-member-without-source`: a workspace member import reads back to no source file. The fix names the member, the subpath, the importing file and the targets the manifest names, and asks for a build of the member or an output and root directory that map the target to its source.
- `setup failure: convention-not-literal`: an applied convention row's directory property is not a literal. The fix names the configuration file and the property, and the alternative of disabling the row in `ts.disabled_conventions` and naming the files in `ts.entry_files`.

A configuration the invocation or the configuration document names that cannot be read ends the run with exit code 3 as well, and so does a run that needs more memory than the machine makes available, which prints the line `memory_exhaustion` in [`exit-codes.json`](../exit-codes.json) states.

## Notes

A note is a hint about the run's setup that the analysis cannot decide. It is no finding: the findings beside it are reported as they would be without it, and it fails no run.

- `published-package`: a Go package of an application, not a `main` package and not under an `internal` directory, that no file of another package of the target imports. Outside programs may import such a package, and only the user knows whether they do; the note names the package's directory and `roots.patterns`, the key that roots its published declarations.
