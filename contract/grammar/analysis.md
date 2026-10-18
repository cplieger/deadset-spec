# The analysis

These pages state the rules every analyzer applies to decide what the program is, which of it is test code, what is live without a reference, what a type error withholds, which failures end a run and which hints a report carries. A rule on them holds for every analyzed language unless it names one, and the rules of the schemas page hold for the schema languages instead. The vocabulary the rules report in is [`kinds.json`](../kinds.json), the configuration keys they read are [`config.schema.json`](../config.schema.json), the report members they fill are [`report.schema.json`](../report.schema.json) and the exit codes they end with are [`exit-codes.json`](../exit-codes.json).

**analysis.1** Every rule on these pages that adds a root, a use or a retention only ever makes a subject live: no such rule turns a live subject dead, and none raises a finding's severity or confidence. A file such a rule makes live is judged member by member: the file is no longer reported, and each of its declarations is reported or held as its own references decide, so the one finding about the file may give way to findings about the declarations it holds. The rules that end a run end it before any finding list exists.

Each family of rules is one page, and each rule is a numbered paragraph cited by its id, `program.3` for the third rule of the program page. A rule's id is never reused for another rule, so a fixture, a test or a line of code that cites one keeps naming the same rule. A section another rule names by its heading is on the page this table lists.

| Page | Sections |
| --- | --- |
| [The program](analysis/program.md) | The target kind, The program, The Go modules of a target, Component files, Workspace packages, Nested packages, Type errors, Unanswered questions, JavaScript projects |
| [The build matrix](analysis/matrix.md) | The derived Go matrix |
| [Test code](analysis/tests.md) | Test code |
| [References in Go](analysis/references-go.md) | Uses through memory layout, Conversions an outside program writes, References in Go, Schema bindings in Go |
| [References in TypeScript](analysis/references-ts.md) | References in TypeScript, Module URLs, Module namespace objects, Module declarations, Overrides of outside classes, Overrides in the target's own classes, Members reached through types, Object spreads, Indexed-access types, Element access by a literal type, Schema bindings in TypeScript |
| [Roots](analysis/roots.md) | Roots, Configuration files by name, Strings in configuration, Script entries, Workflow steps, Manifest entries, Release tags, HTML entry files, Markdown and MDX documents, Computed loads, Type-query aliases, Go packages no build links, Go generators |
| [Convention rows](analysis/conventions.md) | Convention rows |
| [Confidence](analysis/confidence.md) | Confidence |
| [Fixability](analysis/fixability.md) | Fixability, Positional members |
| [Setup failures](analysis/setup-failures.md) | Setup failures, Run failures |
| [Notes](analysis/notes.md) | Notes |
| [Schema bindings](analysis/bindings.md) | The facts document, Records, States |
| [Schemas](analysis/schemas.md) | Schema declarations, Exemptions, Confidence of a schema finding, Generation configuration, Records |
