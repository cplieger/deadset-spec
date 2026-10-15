# Too visible: `DS1500` to `DS1599`

It is used, but exposed more widely than its uses need. Live declarations whose export or visibility is wider than their references require. Generated from `contract/kinds.json`; the [issue kinds](../kinds.md) page lists every family.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1501` | `unnecessary-export` | `go`, `ts` | `on` | `warn` | `narrowable` |
| `DS1502` | `unnecessary-exposure` | `go` | `on` | `warn` | `narrowable` |
| `DS1503` | `redundant-export-keyword` | `ts` | `on` | `warn` | `narrowable` |

## DS1501 unnecessary-export

An exported declaration only its own package uses: the export widens the surface for no caller.

Before:

```go
func Parse(s string) Config { ... } // only this package calls Parse
```

After:

```go
func parse(s string) Config { ... }
```

How to fix it: Unexport the declaration. The finding's fixability is `narrowable`.

The rule: An exported symbol whose every reference is inside the symbol's own package or module, reported as a candidate for unexporting. The subject is a package-level declaration or a method. Three subjects are excluded. An interface method is excluded because its exportedness is the contract of the interface that declares it. A struct field is excluded because an encoder reads it by name. A method that satisfies an interface some symbol uses as a type is excluded because it cannot be unexported without its type ceasing to satisfy that interface, and so is a method whose name and signature are those of a method of an exported interface type declared by a package outside the target that the import closure holds, because that interface fixes them whether or not the program converts the method's type to it. The finding names the narrower visibility the references support. A declared cross-language edge counts as an out-of-package reference. Where the edge's other side is unknown to the analyzer, the finding is emitted pending. A reference from a Go external test package, a test file whose package clause names its directory's package with the suffix _test, is a reference from outside the package, because that test builds as another package and needs the export.

Before reporting: Closed world only. A Go main package, and an internal/ directory tree for importers outside its parent, are closed worlds whatever the configuration declares, because no other program can import them, so this finding is reported there always. A published package is a closed world only when the configuration declares the consumer set complete and every declared consumer loads. A library with no consumer loaded and no complete consumer set declared gets this finding on its internal/ tree and its main packages and never on its published API. A symbol the rule of DS1301 holds for is not reported under this code, whatever severity the configuration gives DS1301, because the write-only finding names the defect to fix first. A type that a parameter or a result of an exported function or method of its own package names is not reported when a reference from outside the package references that function or method, because the caller holds values of the type, and unexporting it would leave an exported signature naming a type its callers cannot name. A package-level declaration whose name a string literal spells carries the class grammar/analysis/confidence.md gives it.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `probable` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, while a declared consumer did not load.
- `possible` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, in a run that holds no consumer information.
- `possible` ([`roots.25`](../../contract/grammar/analysis/roots.md)): A declaration outside an internal tree of a Go package of an application that no build links, in a run that declares no consumer.
- `possible` ([`confidence.8`](../../contract/grammar/analysis/confidence.md)): A package-level Go declaration whose identifier a string literal of a production file spells.
- `possible` ([`confidence.13`](../../contract/grammar/analysis/confidence.md)): A TypeScript member whose name the left operand of an in expression of a production file spells.
- `possible` ([`confidence.14`](../../contract/grammar/analysis/confidence.md)): A field the encoding-reflection class records only as filled by a callee outside the analyzed program.
- `possible` ([`confidence.19`](../../contract/grammar/analysis/confidence.md)): A member of a type the program only decodes into, where a callee outside the analysis, or an any or unknown value, fills it.
- `possible` ([`confidence.20`](../../contract/grammar/analysis/confidence.md)): A declaration of a TypeScript file only tests reach that is no test-support file, and each dead declaration it reaches through dead declarations.
- `possible` ([`confidence.23`](../../contract/grammar/analysis/confidence.md)): A member that only records of exemption classes whose confidence is possible hold.
- `probable` ([`confidence.25`](../../contract/grammar/analysis/confidence.md)): A TypeScript declaration found dead under reachability alone, or a file only dead code imports, while ts.roots_complete is not true.
- `probable` ([`confidence.26`](../../contract/grammar/analysis/confidence.md)): A finding the roots cap applies to, in a configuration that compiles a load by a computed path.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1502 unnecessary-exposure

A declaration more visible than its uses need, such as an exported method of an unexported type nothing outside reaches.

Before:

```go
type parser struct{}

func (p *parser) Next() token { ... } // only this package calls it
```

After:

```go
func (p *parser) next() token { ... }
```

How to fix it: Narrow its visibility to what its uses require. The finding's fixability is `narrowable`.

The rule: An exported symbol of a non-internal package whose every reference is inside the target module, reported as a candidate for relocation behind an internal boundary. The subject is a package-level declaration or a method. Three subjects are excluded. An interface method is excluded because its exportedness is the contract of the interface that declares it. A struct field is excluded because an encoder reads it by name. A method that satisfies an interface some symbol uses as a type is excluded because it cannot be unexported without its type ceasing to satisfy that interface, and so is a method whose name and signature are those of a method of an exported interface type declared by a package outside the target that the import closure holds, because that interface fixes them whether or not the program converts the method's type to it. The finding names the narrower visibility the references support. A declared cross-language edge counts as an out-of-package reference. Where the edge's other side is unknown to the analyzer, the finding is emitted pending. A reference from a Go external test package, a test file whose package clause names its directory's package with the suffix _test, is a reference from outside the package, because that test builds as another package and needs the export.

Before reporting: Closed world only. Reported always for a main package and an internal/ directory tree, and for a published package only when the configuration declares the consumer set complete and every declared consumer loads. A library with no consumer loaded and no complete consumer set declared gets this finding on its internal/ tree and its main packages and never on its published API. A symbol the rule of DS1301 holds for is not reported under this code, whatever severity the configuration gives DS1301, because the write-only finding names the defect to fix first. A type that a parameter or a result of an exported function or method of its own package names is not reported when a reference from outside the target module references that function or method, because the caller holds values of the type, and moving it behind an internal boundary would leave an exported signature naming a type its callers cannot name.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `probable` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, while a declared consumer did not load.
- `possible` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, in a run that holds no consumer information.
- `possible` ([`roots.25`](../../contract/grammar/analysis/roots.md)): A declaration outside an internal tree of a Go package of an application that no build links, in a run that declares no consumer.
- `possible` ([`confidence.8`](../../contract/grammar/analysis/confidence.md)): A package-level Go declaration whose identifier a string literal of a production file spells.
- `possible` ([`confidence.14`](../../contract/grammar/analysis/confidence.md)): A field the encoding-reflection class records only as filled by a callee outside the analyzed program.
- `possible` ([`confidence.19`](../../contract/grammar/analysis/confidence.md)): A member of a type the program only decodes into, where a callee outside the analysis, or an any or unknown value, fills it.
- `possible` ([`confidence.23`](../../contract/grammar/analysis/confidence.md)): A member that only records of exemption classes whose confidence is possible hold.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1503 redundant-export-keyword

An `export` keyword on a declaration no other module imports, in a file whose exports nothing outside reads.

Before:

```ts
export function helper(): void {} // only this file calls helper
```

After:

```ts
function helper(): void {}
```

How to fix it: Drop the `export` keyword. The finding's fixability is `narrowable`.

The rule: An exported declaration used only inside its own file, reported as a candidate for removing the export keyword. The finding names the narrower visibility the references support.

Before reporting: Closed world only. Reported in a file that is not an entry file, a file only a configuration string or a script token names being none, because each runs the file and reads no name it exports, while a file a build script's bundler call names is one, because the bundle exposes what it exports, and a test file being one only for the exports its test runner calls, as grammar/analysis/tests.md "Test code" states, and a file a Markdown or MDX document imports one only for the exports the import reads, as grammar/analysis/roots.md "Markdown and MDX documents" states, and that either belongs to a project whose consumer set the configuration declares complete or is reached by no manifest export. A declared cross-language edge counts as a reference from outside the file. Where the edge's other side is unknown to the analyzer, the finding is emitted pending. A symbol the rule of DS1301 holds for is not reported under this code, whatever severity the configuration gives DS1301, because the write-only finding names the defect to fix first.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `probable` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, while a declared consumer did not load.
- `possible` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, in a run that holds no consumer information.
- `possible` ([`confidence.13`](../../contract/grammar/analysis/confidence.md)): A TypeScript member whose name the left operand of an in expression of a production file spells.
- `possible` ([`confidence.19`](../../contract/grammar/analysis/confidence.md)): A member of a type the program only decodes into, where a callee outside the analysis, or an any or unknown value, fills it.
- `possible` ([`confidence.20`](../../contract/grammar/analysis/confidence.md)): A declaration of a TypeScript file only tests reach that is no test-support file, and each dead declaration it reaches through dead declarations.
- `possible` ([`confidence.23`](../../contract/grammar/analysis/confidence.md)): A member that only records of exemption classes whose confidence is possible hold.
- `probable` ([`confidence.25`](../../contract/grammar/analysis/confidence.md)): A TypeScript declaration found dead under reachability alone, or a file only dead code imports, while ts.roots_complete is not true.
- `probable` ([`confidence.26`](../../contract/grammar/analysis/confidence.md)): A finding the roots cap applies to, in a configuration that compiles a load by a computed path.
- A finding about a member of a dead component carries no higher confidence than the component's root members.
