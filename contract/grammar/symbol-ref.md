# The stable symbol reference

A stable symbol reference names one declaration across runs, across reports and across the documents a maintainer writes, without naming a line. It is the value of a finding's `symbol.ref` and `symbol.parent`, the `symbol` of an ignore entry and of a baseline row, each side of a cross-language edge, the string a configured root names, the fifth component of the merge's canonical key, and the input of the `deadsetSymbolRef/v1` fingerprint. One grammar serves every one of those places, so a reference copied from a text line into an ignore entry matches by construction. This page states the grammar for an implementer who has no access to any existing implementation: one section per language, the rules that hold in every language, one regular expression per form, and what is deliberately not representable. Every string the corpus in [`symbol-ref-corpus.json`](symbol-ref-corpus.json) accepts matches its form's expression and every string it refuses matches none; the grammar self-test in this repository checks both.

The reference is a string compared as bytes. Every consumer, the merge included, compares two references with a plain byte comparison ([`merge.md`](merge.md), the canonical key), so the producer writes each reference in exactly one spelling, the canonical one this page fixes, and a hand-written reference matches only when it is byte-identical to that spelling. A reference that is not, whether misspelled or spelled in a form this page does not define, matches nothing, and the self-check kinds make that visible rather than inert: a suppression naming it is a `DS1703`, a root naming it is a `DS1704`, an edge naming it is a `DS1705` ([`kinds.json`](../kinds.json)).

## The shape

```text
<language>://<scope>#<fragment>
```

Rendered, one per language:

```text
go://example.com/app#Catalog.ResolveAlias
ts://@example/app/src/features/tabs/index.ts#TabStrip.cachedLayout
```

The reference borrows the shape of a URL so that a reader sees a language, a location and a fragment at a glance. It is not a URL: no part of it is percent-encoded or decoded, no Unicode normalization is applied, a non-ASCII identifier travels as its UTF-8 bytes, and characters a URL fragment forbids, such as `[`, `<` and `'`, appear as they are. The corpus refuses `go://example.com/fixture#%C3%9Cnused` and accepts `go://example.com/fixture#Ünused` for this reason.

Six rules hold in both languages.

1. **The language prefix is the namespace.** `go` covers Go; `ts` covers TypeScript and JavaScript, because one analyzer covers both and `kinds.json` names its language `ts`. The prefix is lowercase and is followed by `://`. A third language joins by adding a prefix and a section to this page, and nothing else changes: an edge pairs two references and names no language in its own shape.
2. **The first `#` splits the scope from the fragment.** The scope never contains `#`; the fragment may, in a TypeScript private name.
3. **An empty fragment names the scope itself**: the Go package, or the TypeScript module. The `#` is still written, so that a scope is never mistaken for a symbol: without it, the scope's last element reads as a symbol of that name.
4. **No CR and no LF, anywhere.** A reference is a field of a text line, one half of an LF-separated hash input ([`sarif.md`](sarif.md), `deadsetSymbolRef/v1`) and a JSON string, and each of those needs the guarantee. A TypeScript name that contains either is quoted and escaped.
5. **No whitespace outside a quoted TypeScript component.** Go has no quoted component, so a Go reference contains no whitespace at all.
6. **Case is significant** everywhere and compared bytewise: `Catalog` and `catalog` are two symbols, a module path keeps the case its `go.mod` spells, and a selector is lowercase.

## What makes it stable

The reference is built from names and containers only: the scope, the declared name, and the chain of containers between them. No line, no column, no byte offset and no index enters it. An edit above a declaration, a reformat, or a reordering of the fields of a struct or the members of a class leaves every reference in the file unchanged, so a suppression, a root or an edge written against it still matches. Three edits change a reference, and each is a change to the thing named: renaming the declaration, moving it to another package or module, and changing its container, such as moving a method to another type.

The position key `path:line:col` an analyzer uses inside one run is a different identifier with a different job, and the two never mix: the position locates the finding in the report, the reference identifies the symbol across reports.

A member is named, never numbered by its position in its container, so inserting a field or a method above another changes nothing. A reference carries no signature either, so adding a parameter changes nothing. The scope and the fragment are separated by `#` rather than by a dot, because a module path may itself contain dots and a dot would split such a reference ambiguously.

## Go

```text
go://<import-path>#<fragment>
```

### The Go scope

The scope is the import path of the package that declares the symbol: the module path from `go.mod`, then `/` and the package's directory below the module root, which is what `go list` reports as `ImportPath` and what `types.Package.Path()` returns. The package at the module root has the module path alone as its scope. A `main` package is scoped like any other. The corpus carries `go://example.com/app/cmd/tool#run`.

Two test packages exist per directory and they scope differently. A test file whose package clause names the production package (`package app`, in `catalog_test.go`) is a variant of the production package: the analyzer keys its declarations by position so each appears once, and its symbols share the production scope, `go://example.com/app#TestResolve`. A test file whose package clause carries the `_test` suffix (`package httpapi_test`) is a separate package whose path is the directory path with that suffix, `go://example.com/app/httpapi_test#TestSearch`. Neither carries the bracketed test-binary suffix that `go list -test` appends to the import path and `go/packages` keeps in a package's `ID`: the reference uses `PkgPath`, which is `example.com/app` for the variant and `example.com/app/httpapi_test` for the external package. The synthetic `main` package the toolchain builds for a test binary declares nothing of the target and is never a scope.

An import path element is ASCII letters, digits and `-`, `.`, `_`, `~`, and a path neither starts nor ends with `/` ([`module.CheckImportPath`](https://pkg.go.dev/golang.org/x/mod/module#CheckImportPath)). The set contains no `#`, which is what lets the first `#` split the reference. The expressions below check the character set and the slash placement; they do not check the finer rules, such as an element not ending in a dot, because the toolchain already refused any package that breaks them.

### Declarations

A package-level declaration is its name: a function, a type of any kind, a constant or a variable, exported or not, with the case it was declared in.

```text
go://example.com/app#Resolve
go://example.com/app#normalize
go://example.com/app#Exact
```

A member is its container's name, a dot and its own name, and a chain of members is read from the package-level declaration inward. Three kinds of member exist.

**A method** is `Type.Method`, with no receiver star and no type arguments: `Catalog.ResolveAlias` for `func (c *Catalog) ResolveAlias`, and `List.Push` for `func (l *List[T]) Push`. The star carries nothing, because the language forbids a value-receiver and a pointer-receiver method of one name on one base type ([Go specification, method declarations](https://go.dev/ref/spec#Method_declarations)). The display name `(*Catalog).ResolveAlias` keeps the star and is `symbol.name`, which [`text-line.md`](text-line.md) renders and this page never carries. A method declared on an alias of a defined type belongs to the defined type, so its container is that type's name. An interface method takes the same form, `Fetcher.Fetch`.

**A struct field** is `Type.Field`. An embedded field is spelled by its field name, which the language defines as the unqualified type name ([Go specification, struct types](https://go.dev/ref/spec#Struct_types)): `Buffered.Buffer` for `*bytes.Buffer` embedded in `Buffered`. A field of an anonymous struct type is reached through the field that carries the type, whatever pointer, slice, array, channel or map-value wrappers sit between: `Manifest.Tools.Version` for `Tools []struct{ Name, Version string }`. A field always carries its struct: a field name alone is a package-level reference, and two structs may each declare a field of one name, so the whole chain is what identifies the field rather than labelling it.

**A type parameter** of a function is the function name followed by the parameter name in square brackets, the language's own instantiation syntax: `Best[T]` for `func Best[T Tag](want T, have []T)`. The constraint never appears. Only a function's type parameters have a form, because `DS1303` reports on functions and methods only and a Go method declares no type parameters of its own; a type's parameters belong to the type and are not a subject ([`kinds.json`](../kinds.json), `DS1303`).

A Go enumerated member is a package-level constant and takes the package-level form. Type arguments never appear on a container, in either language: the type parameter list is part of the declaration, not of its identity.

### The package, a file and a module directive

The package itself is the empty fragment, `go://example.com/app#`. It is the `symbol.parent` of every package-level declaration.

A source file is its base name followed by the `:file` selector, scoped by the import path of the directory that holds it, whatever package clause the file carries: `go://example.com/app/internal/legacy#render_windows.go:file`. The selector exists because a file name is not an identifier chain and must not be read as one: `render_windows.go` without the selector parses as the member `go` of `render_windows`. The file name is not folded into the scope, because an import path element may legally end in `.go`, so `go://…/legacy/render_windows.go#` would be the spelling of a package rather than of a file. A selector is an operand, a colon and a lowercase word, and every subject that is not an identifier chain carries one.

A `require` directive is the required module path followed by `:require`, scoped by the module whose `go.mod` carries it, which is the module path itself: `go://example.com/app#github.com/example/left:require`. The required version is the directive's value, not its identity, so it never appears: a bump leaves the reference unchanged. A `replace` directive is the replaced module path, then `@` and the version when the directive names one, then `:replace`: `go://example.com/app#github.com/old/mod@v1.2.3:replace`, or `#github.com/old/mod:replace` for a directive with no version. The version stays on a `replace` because two `replace` lines for one module at two versions are two directives. A nested module inside the target is its own scope. No other directive has a form, because no other directive is a finding subject (`DS1605`'s precondition in `kinds.json`).

### Deliberately not representable in Go

Each of the following has no reference of its own. A finding whose subject is one of them carries the reference of the nearest enclosing symbol that has one, and its `position` and `symbol.name` say which part of that symbol it concerns.

- **A declaration inside a function body**: a local type, constant, variable or closure. A local name has no container chain from the package scope, and an ordinal within the body would change when a declaration is added above it. The enclosing function's reference stands for it.
- **A parameter, a named result, an unnamed result, a receiver, a statement, a case clause and a store** (`DS1801` to `DS1809`). These are parts of a declaration, not declarations the analysis enumerates as symbols, and a statement has no name at all. The enclosing function or method is the reference; the finding's position locates the site. Two `DS1801` findings on one function share a reference and differ in position and `symbol.name`.
- **A blank-identifier declaration**, `var _ Iface = (*T)(nil)`. A package may hold any number of them, all named `_`, so the name cannot identify one, and an ordinal would not survive an insertion. Such a declaration is a root and is never itself reported dead; the one kind that reports at one, `DS1204`, carries the reference of the interface the assertion names, at the assertion's position.
- **`init` functions.** A package may declare several, all named `init`, and all are roots. A reference `#init` denotes every one of them in its package; a finding inside one is told apart by position.
- **A field of an anonymous struct that is a map key, a function parameter or result type, or an element of an interface method's signature.** The chain steps into an anonymous struct only through the wrappers named above, because a map's key struct and value struct could otherwise produce one reference for two fields. The carrying field or declaration is the reference.
- **A promoted field or method, a cgo name, and a symbol of a package outside the target.** None is a declaration of the target.

Two spellings parse and match nothing, and the self-check kinds are the safety net for both: a field written without its struct is syntactically a package-level reference, and an object path such as `T.UM0.RA1.F0` is syntactically a member chain. A suppression carrying either is reported as `DS1703`.

## TypeScript and JavaScript

```text
ts://<package-name>/<source-path>#<fragment>
```

### The TypeScript scope

The scope is the module: the name of the package the file belongs to, then `/`, then the file's path inside that package, with `/` as the separator and the file's own extension. The package name is the `name` field of the nearest `package.json` at or above the file's directory, up to the target root, and the path is relative to that manifest's directory.

```text
ts://@example/app/src/wire.ts#
ts://@example/app/src/features/tabs/index.ts#TabStrip
```

When no manifest at or above the file carries a `name`, the package component is a single dot and the path is relative to the target root: `ts://./src/wire.ts#ServerEvent`. The two cannot collide, because an npm package name never starts with a dot ([validate-npm-package-name](https://github.com/npm/validate-npm-package-name)). An npm name also never contains `#`, `:`, whitespace or `*`, and contains `/` only after a leading `@scope`, which is what lets the package component be split from the path and lets a pattern's wildcard stay unambiguous.

The path names a source file, never an import specifier, so the extension is always present and is one of `.ts`, `.tsx`, `.mts`, `.cts`, `.d.ts`, `.d.mts`, `.d.cts`, `.js`, `.jsx`, `.mjs` or `.cjs`. The reference names the declaring file rather than a package entry point, because a module-local declaration that is not exported has no entry-point path at all, and `DS1002` reports exactly those. The `package.json` itself is a scope for one form only, the dependency form below.

A JavaScript file is scoped and spelled exactly as a TypeScript file, under the `ts` prefix: `ts://@example/app/scripts/build.mjs#bundle`.

### Components

A fragment is a chain of components joined by `.`, read from the module-level declaration inward. A component takes one of four spellings, and the spelling is decided by the name alone, never by what else the module declares, so it cannot change when a sibling is added.

- **An identifier name** is written bare: `TabStrip`, `default`, `static`, `größe`. The test is the ECMAScript `IdentifierName` production ([ECMA-262](https://tc39.es/ecma262/#prod-IdentifierName)), which admits reserved words, so `default` and `static` are bare.
- **A private name** keeps its leading `#`: `TabStrip.#frame` for `#frame = 0;`. The sign is part of the name in the language, `this.#frame` is how the source spells a use, and a class may declare `#frame` and `frame` side by side, so dropping the sign would merge two members. This is the one place a `#` appears after the scope separator, and rule 2 above already splits at the first `#`. A private name is only ever a member, so it never opens a fragment.
- **A string** is written in single quotes when the name is not an identifier name: `Headers.'content-type'`, `Kind.'tab-close'`, `Quirks.'it\'s'`. Inside the quotes a quote is `\'`, a backslash is `\\`, LF is `\n` and CR is `\r`; every other character stands for itself, non-ASCII included. The quotes are single rather than double, so that a reference sits inside a JSON string with no escaping, which is where every document carries one, and so that the reference matches the spelling `text-line.md` fixes for `symbol.name`. A numeric key such as `1.5` is a string in this sense and is spelled `'1.5'`.
- **A computed key** is the key's source text inside square brackets with every ASCII whitespace character removed: `TabStrip.[Symbol.iterator]` for `*[Symbol.iterator]()`. The text is compared as written, so `[Symbol.iterator]` and `[globalThis.Symbol.iterator]` are two references, which is right, because they are two declarations. A key whose text contains `]` has no spelling.

### Module-level declarations and exports

A module-level declaration is one component: a function, class, interface, type alias, enum, namespace or variable, exported or not. `ServerEvent`, `measure`, `GAP`. A value and a type of one name in one module are one symbol under declaration merging, so they are one reference; the two meanings are not separable here, because the analysis reports the symbol, not a meaning of it.

A default export is spelled `default`, its export name: `export default class App {}` is `#default`, and so is the anonymous `export default class {}`. The local name, `App`, is `symbol.name` where one exists. A module has at most one default export, so the spelling is unambiguous.

An export specifier creates an alias symbol distinct from the declaration it names, and the analysis reports the two separately: a re-export nothing imports is a finding on the alias while the declaration behind it may be live. The alias is spelled by its exported name followed by `:alias`: `TabStrip:alias` for `export { TabStrip } from './features/tabs'`, `Strip:alias` for `export { TabStrip as Strip }`, `default:alias` for `export default App;` written as a statement, and `ns:alias` for `export * as ns from './m'`. The selector is what keeps `const a = 1; export { a };` apart, where the module holds a local `a` and an export alias `a` with one name. The alias is marked rather than the local, because the unexported local is the more frequent subject and reads better unmarked. An exported name that is not an identifier, `export { x as 'my-name' }`, is a quoted component with the selector: `'my-name':alias`.

### Members

A member is its container's component, a dot and its own component. The containers and their members:

| Container | Member | Example |
| --- | --- | --- |
| class | instance property, method, accessor | `TabStrip.cachedLayout`, `TabStrip.render` |
| class | static property or method | `TabStrip.count:static` |
| class | private name | `TabStrip.#frame` |
| interface | property, method | `ServerEvent.seq` |
| type alias | member of the object type it declares | `Layout.gap` for `type Layout = { gap: number; width: number }` |
| enum | member | `Kind.Resize`, `Kind.'tab-close'` |
| namespace | exported declaration, recursively | `Wire.Server.Event` |

An instance member is bare and **a static member always carries `:static`**, because a class may declare a static and an instance member of one name and the spelling must not depend on whether it does. One side is marked, and always marked, so the reference is canonical. A getter and a setter of one name are one symbol and one reference. A private modifier (`private cachedLayout`) changes nothing in the spelling; only the `#` form is spelled differently, because only it is a different name.

A member of a type alias is a member of the object type literal the alias declares, or of a literal that is a direct constituent of a union or intersection under it. Where two constituents declare one member name, the member has no reference of its own and the alias stands for it. Nested classes do not exist as declarations in the language; a class inside a namespace is a namespace member, `N.C`, and a class expression assigned to a property has no name and no reference.

### Type parameters

A type parameter of a function or method is the function's or method's reference followed by the parameter name in angle brackets, the language's type-argument syntax: `decode<T>` for `export function decode<T>(raw: string): T`, `Codec.decode<T>` for a method, and `Codec.of:static<T>` for a static method, the selector first and the type parameter last. The constraint never appears, and a type's own parameters have no form, for the reason `DS1303` states. Go spells the same subject with square brackets; each language keeps its own bracket, and the two never meet in one reference.

### The module and a dependency

The module itself is the empty fragment, `ts://@example/app/src/wire.ts#`. It is the `symbol.parent` of every module-level declaration and the reference of a file-level finding (`DS1501`, `DS1502`).

A manifest dependency is scoped by the manifest, `<package-name>/package.json`, spelled by the dependency's own package name, and followed by the selector of the section that declares it: `:dependency` for `dependencies`, `:dev-dependency` for `devDependencies`, `:peer-dependency` for `peerDependencies`, the three sections `DS1601` names in `kinds.json`.

```text
ts://@example/app/package.json#lodash:dependency
ts://@example/app/package.json#@types/node:dev-dependency
```

The declared version range is the entry's value, not its identity, and never appears.

### Deliberately not representable in TypeScript

- **An overload signature.** A function with three overload signatures is one symbol with one reference. An index over the signatures would shift when a signature is inserted above it, which is the instability this grammar exists to avoid.
- **A constructor, a call signature, a construct signature and an index signature.** None has a name. The class or interface stands for them.
- **A declaration inside a function body, a parameter, a binding element of a destructuring pattern, a statement and a store**, for the reasons the Go section gives. The enclosing function or method is the reference.
- **A member of a class expression**, and a member whose computed key contains `]`.
- **An `export =` assignment**, whose export name the compiler spells `export=`.
- **One meaning of a merged symbol**: the type half of a value-and-type pair, the namespace half of an enum-and-namespace merge. The symbol is the unit of analysis.

As in Go, a spelling can parse and match nothing: `#Green` for an enum member written without its enum is syntactically a module-level reference, and `ts://src/wire.ts#X` is syntactically a module in a package named `src`. Both are reported by the self-check kinds when a document carries them.

## Patterns

A configured root may be a pattern (`roots.patterns` in [`config.schema.json`](../config.schema.json)). A pattern is a reference in which `*` stands for zero or more characters inside one element of the scope, where it never crosses `/` or `#`, or inside one component of the fragment, where it never crosses `.`, `:` or `#`; it never stands inside a quoted or computed component and never replaces any part of the language prefix. Everything else in the pattern is compared bytewise. A pattern that matches no symbol is a `DS1704`.

```text
go://example.com/app#Catalog.*
go://example.com/app/internal/*#*
ts://@example/app/src/generated/*.ts#*
```

`*` is the only wildcard: no character class, because `[` and `<` are literal in a type-parameter form, and no `**`, so a pattern names one directory level. Patterns are accepted for roots only. An ignore entry, a baseline row and an edge name exact references, because an adjudication broader than one symbol masks findings nobody adjudicated.

## The grammar

The expressions are written in the intersection of two dialects, RE2 as Go's `regexp` package implements it ([syntax](https://pkg.go.dev/regexp/syntax)) and ECMAScript `RegExp` with no flags, so one string compiles unchanged in a Go implementation and in a TypeScript one; `text-line.md` follows the same rule. Two consequences shape the identifier classes. Neither dialect offers a Unicode identifier property without a flag, so an identifier character outside ASCII is written `[^\x00-\x7F]`, which accepts every non-ASCII code point; the expressions are therefore form checkers, not identifier validators, and the compiler that already accepted the declaration is the validator of its name. And ECMAScript without the `u` flag matches that class one UTF-16 code unit at a time while RE2 matches one code point, which gives the same answer for any run of non-ASCII characters, astral ones included, because both halves of a surrogate pair are outside ASCII.

### Building blocks

| Name | Expression | Reads |
| --- | --- | --- |
| `GO_IDENT` | `(?:[A-Za-z_]\|[^\x00-\x7F])(?:[A-Za-z0-9_]\|[^\x00-\x7F])*` | a Go identifier |
| `GO_PATH` | `[A-Za-z0-9._~-]+(?:/[A-Za-z0-9._~-]+)*` | an import path or module path |
| `GO_VERSION` | `[A-Za-z0-9.+-]+` | a module version, as `go.mod` spells it |
| `GO_FILE` | `[^/\\:#\r\n]+\.go` | a Go file's base name |
| `TS_IDENT` | `(?:[A-Za-z_$]\|[^\x00-\x7F])(?:[A-Za-z0-9_$]\|[^\x00-\x7F])*` | an ECMAScript identifier name |
| `TS_QUOTED` | `'(?:[^'\\\r\n]\|\\['\\nr])*'` | a single-quoted component |
| `TS_COMPUTED` | `\[[^\]\r\n\t ]+\]` | a computed key |
| `TS_HEAD` | `(?:TS_IDENT\|TS_QUOTED)` | a component that may open a fragment |
| `TS_MEMBER` | `(?:TS_IDENT\|#TS_IDENT\|TS_QUOTED\|TS_COMPUTED)` | a component after a dot |
| `TS_PACKAGE` | `(?:@[A-Za-z0-9._-]+/)?[A-Za-z0-9._-]+` | an npm package name, or `.` |
| `TS_FILE` | `(?:[^/#\r\n]+/)*[^/#\r\n]+(?:(?:\.d)?\.[mc]?ts\|\.tsx\|\.[mc]?js\|\.jsx)` | a source path with its extension |

### One expression per form

Each row is anchored, `^` to `$`, and is the whole reference. The corpus names the form of each case with the `form` value in the first column; several forms share one expression because they differ in what the symbol is, not in how the reference is spelled.

| Language | Corpus `form` | Expression |
| --- | --- | --- |
| go | `package` | `^go://GO_PATH#$` |
| go | `package-level` | `^go://GO_PATH#GO_IDENT$` |
| go | `method`, `interface-method`, `field` | `^go://GO_PATH#GO_IDENT(?:\.GO_IDENT)+$` |
| go | `type-parameter` | `^go://GO_PATH#GO_IDENT\[GO_IDENT\]$` |
| go | `file` | `^go://GO_PATH#GO_FILE:file$` |
| go | `require` | `^go://GO_PATH#GO_PATH(?:@GO_VERSION)?:require$` |
| go | `replace` | `^go://GO_PATH#GO_PATH(?:@GO_VERSION)?:replace$` |
| ts | `module` | `^ts://TS_PACKAGE/TS_FILE#$` |
| ts | `module-level`, `default-export` | `^ts://TS_PACKAGE/TS_FILE#TS_HEAD$` |
| ts | `alias` | `^ts://TS_PACKAGE/TS_FILE#TS_HEAD:alias$` |
| ts | `class-member`, `private-member`, `interface-member`, `type-member`, `enum-member`, `namespace-member`, `computed-member` | `^ts://TS_PACKAGE/TS_FILE#TS_HEAD(?:\.TS_MEMBER)+$` |
| ts | `static-member` | `^ts://TS_PACKAGE/TS_FILE#TS_HEAD(?:\.TS_MEMBER)+:static$` |
| ts | `type-parameter` | `^ts://TS_PACKAGE/TS_FILE#TS_HEAD(?:\.TS_MEMBER)*(?::static)?<TS_IDENT>$` |
| ts | `dependency` | `^ts://TS_PACKAGE/package\.json#TS_PACKAGE:(?:dependency\|dev-dependency\|peer-dependency)$` |

### Expanded

The same fourteen expressions with every block substituted, one per line under its language and form, for copying into a test. Every line compiles under Go's `regexp` and under an ECMAScript `RegExp` with no flags.

```text
go package
^go://[A-Za-z0-9._~-]+(?:/[A-Za-z0-9._~-]+)*#$

go package-level
^go://[A-Za-z0-9._~-]+(?:/[A-Za-z0-9._~-]+)*#(?:[A-Za-z_]|[^\x00-\x7F])(?:[A-Za-z0-9_]|[^\x00-\x7F])*$

go method, interface-method, field
^go://[A-Za-z0-9._~-]+(?:/[A-Za-z0-9._~-]+)*#(?:[A-Za-z_]|[^\x00-\x7F])(?:[A-Za-z0-9_]|[^\x00-\x7F])*(?:\.(?:[A-Za-z_]|[^\x00-\x7F])(?:[A-Za-z0-9_]|[^\x00-\x7F])*)+$

go type-parameter
^go://[A-Za-z0-9._~-]+(?:/[A-Za-z0-9._~-]+)*#(?:[A-Za-z_]|[^\x00-\x7F])(?:[A-Za-z0-9_]|[^\x00-\x7F])*\[(?:[A-Za-z_]|[^\x00-\x7F])(?:[A-Za-z0-9_]|[^\x00-\x7F])*\]$

go file
^go://[A-Za-z0-9._~-]+(?:/[A-Za-z0-9._~-]+)*#[^/\\:#\r\n]+\.go:file$

go require
^go://[A-Za-z0-9._~-]+(?:/[A-Za-z0-9._~-]+)*#[A-Za-z0-9._~-]+(?:/[A-Za-z0-9._~-]+)*(?:@[A-Za-z0-9.+-]+)?:require$

go replace
^go://[A-Za-z0-9._~-]+(?:/[A-Za-z0-9._~-]+)*#[A-Za-z0-9._~-]+(?:/[A-Za-z0-9._~-]+)*(?:@[A-Za-z0-9.+-]+)?:replace$

ts module
^ts://(?:@[A-Za-z0-9._-]+/)?[A-Za-z0-9._-]+/(?:[^/#\r\n]+/)*[^/#\r\n]+(?:(?:\.d)?\.[mc]?ts|\.tsx|\.[mc]?js|\.jsx)#$

ts module-level, default-export
^ts://(?:@[A-Za-z0-9._-]+/)?[A-Za-z0-9._-]+/(?:[^/#\r\n]+/)*[^/#\r\n]+(?:(?:\.d)?\.[mc]?ts|\.tsx|\.[mc]?js|\.jsx)#(?:(?:[A-Za-z_$]|[^\x00-\x7F])(?:[A-Za-z0-9_$]|[^\x00-\x7F])*|'(?:[^'\\\r\n]|\\['\\nr])*')$

ts alias
^ts://(?:@[A-Za-z0-9._-]+/)?[A-Za-z0-9._-]+/(?:[^/#\r\n]+/)*[^/#\r\n]+(?:(?:\.d)?\.[mc]?ts|\.tsx|\.[mc]?js|\.jsx)#(?:(?:[A-Za-z_$]|[^\x00-\x7F])(?:[A-Za-z0-9_$]|[^\x00-\x7F])*|'(?:[^'\\\r\n]|\\['\\nr])*'):alias$

ts class-member, private-member, interface-member, type-member, enum-member, namespace-member, computed-member
^ts://(?:@[A-Za-z0-9._-]+/)?[A-Za-z0-9._-]+/(?:[^/#\r\n]+/)*[^/#\r\n]+(?:(?:\.d)?\.[mc]?ts|\.tsx|\.[mc]?js|\.jsx)#(?:(?:[A-Za-z_$]|[^\x00-\x7F])(?:[A-Za-z0-9_$]|[^\x00-\x7F])*|'(?:[^'\\\r\n]|\\['\\nr])*')(?:\.(?:(?:[A-Za-z_$]|[^\x00-\x7F])(?:[A-Za-z0-9_$]|[^\x00-\x7F])*|#(?:[A-Za-z_$]|[^\x00-\x7F])(?:[A-Za-z0-9_$]|[^\x00-\x7F])*|'(?:[^'\\\r\n]|\\['\\nr])*'|\[[^\]\r\n\t ]+\]))+$

ts static-member
^ts://(?:@[A-Za-z0-9._-]+/)?[A-Za-z0-9._-]+/(?:[^/#\r\n]+/)*[^/#\r\n]+(?:(?:\.d)?\.[mc]?ts|\.tsx|\.[mc]?js|\.jsx)#(?:(?:[A-Za-z_$]|[^\x00-\x7F])(?:[A-Za-z0-9_$]|[^\x00-\x7F])*|'(?:[^'\\\r\n]|\\['\\nr])*')(?:\.(?:(?:[A-Za-z_$]|[^\x00-\x7F])(?:[A-Za-z0-9_$]|[^\x00-\x7F])*|#(?:[A-Za-z_$]|[^\x00-\x7F])(?:[A-Za-z0-9_$]|[^\x00-\x7F])*|'(?:[^'\\\r\n]|\\['\\nr])*'|\[[^\]\r\n\t ]+\]))+:static$

ts type-parameter
^ts://(?:@[A-Za-z0-9._-]+/)?[A-Za-z0-9._-]+/(?:[^/#\r\n]+/)*[^/#\r\n]+(?:(?:\.d)?\.[mc]?ts|\.tsx|\.[mc]?js|\.jsx)#(?:(?:[A-Za-z_$]|[^\x00-\x7F])(?:[A-Za-z0-9_$]|[^\x00-\x7F])*|'(?:[^'\\\r\n]|\\['\\nr])*')(?:\.(?:(?:[A-Za-z_$]|[^\x00-\x7F])(?:[A-Za-z0-9_$]|[^\x00-\x7F])*|#(?:[A-Za-z_$]|[^\x00-\x7F])(?:[A-Za-z0-9_$]|[^\x00-\x7F])*|'(?:[^'\\\r\n]|\\['\\nr])*'|\[[^\]\r\n\t ]+\]))*(?::static)?<(?:[A-Za-z_$]|[^\x00-\x7F])(?:[A-Za-z0-9_$]|[^\x00-\x7F])*>$

ts dependency
^ts://(?:@[A-Za-z0-9._-]+/)?[A-Za-z0-9._-]+/package\.json#(?:@[A-Za-z0-9._-]+/)?[A-Za-z0-9._-]+:(?:dependency|dev-dependency|peer-dependency)$
```

A reference is valid when it matches one of its language's expressions. A refused string in the corpus matches none of them.

## The corpus

[`symbol-ref-corpus.json`](symbol-ref-corpus.json) is an array of cases. Each case carries the `input` string, its `language`, the `form` it takes or nearly takes, whether it is `accepted`, a `reason`, and for an accepted case an `example` of one or two source lines the reference denotes. Every form above has at least one accepted case, and every rule has a refused near-miss: the display name with its receiver star, a parameter in parentheses, a field written with a slash, a trailing dot, a parenthesized selector, a `#` or a `!` as a member separator, a double-quoted string, a Go bracket on a TypeScript type parameter, a percent-encoded identifier, a stray CR or LF, and a spelling in which the selector precedes its operand.

## What other documents fix

- [`finding.schema.json`](../finding.schema.json): the `symbol` object, its `ref`, `parent`, `name`, `kind` and `objectpath` fields, and what `symbol.ref` carries for a finding whose subject is a configured root (`DS1704`) or a declared edge (`DS1705`).
- [`report.schema.json`](../report.schema.json): the edge-evaluation record whose `symbol` is a reference, and the stale-suppression record whose `symbol.ref` is the reference the suppression named.
- `suppression.md`: the ignore entry and baseline row that carry a reference beside a path, and the inline directive that names none because its position is the declaration below it.
- [`config.schema.json`](../config.schema.json): `roots.patterns`, the one place a pattern is accepted.
- [`kinds.json`](../kinds.json): the kinds whose subjects fix which forms exist, and the self-check kinds that report a reference matching nothing.
- [`merge.md`](merge.md): the bytewise comparison of `symbol.ref` in the canonical key.
- [`sarif.md`](sarif.md): the `deadsetSymbolRef/v1` digest over the code, one LF and the reference.
- [`text-line.md`](text-line.md): `symbol.name`, the display form this page never carries.
