# Issue kinds

Every kind an analyzer reports carries a code, a name, the rule that produces it, a confidence ceiling, a default enablement, a default severity and a fixability. This page states all of them, one table per code range and one section per kind. Every value is read from `contract/kinds.json`, and each section names the row it comes from.

A code is the prefix `DS` followed by four digits. Ranges group codes by family. A code names at most one kind for the life of the code space. A retired code stays retired and is listed in the table that closes this page, rather than naming a different kind later. The source is `contract/kinds.json`, `prefix`, `ranges` and `retired`. One code renders identically in a text output line, an ignore entry, a configuration key and a SARIF rule identifier.

The columns of every table below, each the field of the same name in the kind's row:

- Languages are the languages the kind applies to, where `go` is Go and `ts` is TypeScript and JavaScript. The source is `contract/kinds.json`, `languages`.
- Default is `default_enabled`. `on` means an analyzer reports the kind unless a configuration disables it.
- Severity is `default_severity`, one of `allow`, `warn` and `deny`, as `contract/kinds.json`, `severities`, lists them. A finding at or above the failing severity, `reporters.fail_on`, exits with the findings code. Under the default, `deny`, a `warn` finding never fails a run. The sources are `contract/exit-codes.json`, codes 1 and 0, and the document's `description`. A configuration names a code or a two-digit family prefix under `severity` to change a severity, as `contract/config.schema.json` states.
- Fixability is `fixability`, one of `deletable`, `narrowable`, `manual` and `none`, as `contract/kinds.json`, `fixabilities`, lists them. It says what a mechanical edit may do with the finding. A finding about a declaration whose deletion changes the value of a live declaration carries `manual` instead. So does a finding about a declaration such a one depends on. This holds under the kinds whose row lists `manual` in `finding_fixabilities`, as `contract/grammar/analysis.md`, "Fixability", states.

Confidence is a ceiling on the reachability class, not a second axis. A finding carries a `reachability_class`, which is what the analysis knows about the symbol's callers, and a `confidence`, which is that class capped by the kind's `max_class`. Both take one value from `certain`, `probable` and `possible`, ordered from the strongest. The source is `contract/kinds.json`, `reachability_classes`.

Every kind in this contract version declares the ceiling `certain`, so a kind whose ceiling is lower states it in its own section. A finding about a library's published API, a public member of a published type included, is `possible` when the run holds no consumer information. So is a declaration of a Go package no build links in a run that declares no consumer, and a Go field only an outside filling callee fills. The default `analysis.min_confidence`, `probable`, withholds them and reports the rest. The report counts the findings it withheld at each level. The text and SARIF outputs name those counts in one line, with the setting that shows them.

A TypeScript finding decided under reachability alone, about a declaration only dead declarations reference or a file only dead code imports, is capped at `probable` unless `ts.roots_complete` is `true`, because a root the analysis cannot see would make it live. A finding about a member of a dead component is also capped by the lowest confidence among the component's root members. So every finding of one component carries one confidence, except where production code only tests use lowers a finding to `possible` on its own. The sources are `contract/grammar/analysis.md`, "Confidence", and `contract/config.schema.json`, `analysis.min_confidence`.

Three fields appear on a row only where they apply, and each section carries them where present. `precondition` is a condition the analyzer checks before it reports the kind. `derived_from` names the kinds a derived kind is computed from, and such a finding is reported once, under the most specific code. `overlap` names, per language, the external linters or rules that report the same kind, so a project already running one silences whichever side it prefers. The entries `none known` and `not applicable` are values of that field rather than omissions.

## DS1000 to DS1099: `unused-declarations`

Declarations nothing references, and the test-only and deprecated variants of them. From `contract/kinds.json`, the `unused-declarations` range.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1001` | `unused-exported` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1002` | `unused-unexported` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1003` | `unused-member` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1004` | `test-only-use` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1005` | `test-of-dead-code` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1006` | `deprecated-unused` | `go`, `ts` | `on` | `deny` | `deletable` |

### DS1001 unused-exported

An exported symbol with no reference in the target and no reference from any loaded consumer. A symbol referenced only from its own declaration site counts as unreferenced.

A finding of this kind may carry `manual` instead, as the fixability bullet above states.

From `contract/kinds.json`, row `DS1001`.

### DS1002 unused-unexported

An unexported symbol with no reference in the target. A symbol referenced only from its own declaration site counts as unreferenced. In TypeScript a name an import declaration binds, or an `import x = require()` declaration that is not exported, is such a symbol of its file, reported at the binding when nothing in the file reads it: a use in a type position, in a local export or in a type, a link or a `@see` tag of a doc comment reads it, and so does JSX the compiler turns into calls of a factory it names. A binding of a declaration file or of a component file is not judged, nor one of a generated file while generated files are excluded. In Go a constant or a type a function body declares, a function literal's body included, that nothing in that body names outside its own specification is reported under this code, under the reference of the enclosing declaration, as `grammar/symbol-ref.md` states for a declaration inside a function body, at the local declaration's own position, with `symbol.kind` `constant` or `type` and the liveness relation `reference-counting`.

A finding of this kind may carry `manual` instead, as the fixability bullet above states.

From `contract/kinds.json`, row `DS1002`.

### DS1003 unused-member

A struct field, class member or type member with no reference, a private member included. A member referenced only from its own declaration site counts as unreferenced.

Precondition: A TypeScript member that no value writes and nothing reads may name a type parameter of its declaring interface, class or type alias. It is not reported when every part of that declaration naming the type parameter is such a member. TypeScript compares object types by their members, so those members keep two instantiations of the type apart. Deleting them leaves the type parameter phantom and lets one instantiation stand for another. A member a value writes is decided by the rule of `DS1301` instead, whatever its type names.

A finding of this kind may carry `manual` instead, as the fixability bullet above states.

From `contract/kinds.json`, row `DS1003`.

### DS1004 test-only-use

A symbol with zero production references and at least one test reference: an unused-exported, unused-unexported or unused-member candidate whose test reference count is not zero, reported once under this code. A reference from a consumer's test files is a test reference unless the configuration counts consumer tests as production. A reference from test-support code, a package or file that only tests reach and that carries test evidence of its own language, is a test reference. A declaration of test-support code is test code and is no subject of this code, as grammar/analysis.md states. An interface method whose only calls and selections through the interface are in test code, and that DS1203's precondition does not exempt, is reported under this code. The methods implementing it are not reported with it, because the test reaches them through it. A candidate found dead under reachability, every production reference to which is made by a declaration itself dead, is reported under this code as well where test code that is no test of dead code, and that the run counting test references reaches, references it, because that test keeps it, and the finding's message names both kinds of reference.

Derived from `DS1001`, `DS1002` and `DS1003`.

A finding of this kind may carry `manual` instead, as the fixability bullet above states.

From `contract/kinds.json`, row `DS1004`.

### DS1005 test-of-dead-code

A test symbol whose set of referenced target symbols is non-empty and every member of that set is reported dead. In TypeScript an ambient declaration, one a declaration file holds or one a `declare` modifier carries or a declaration carrying one holds, describes code the target does not implement, so it is no target symbol of that set. A test that references at least one live target symbol is never reported, no notion of a test's subject and no name matching enters the rule, and the message states the rule. A production declaration whose reachability class is `possible` because programs outside the analysis may use it, being published API of a library whose run declares no consumer, and in Go a declaration of a package no build links or an export of a unit a dropped module requires, counts as a live target symbol of every test that references it, so such a test is no test of dead code. The declaration is still reported at `possible`. The test joins the dead component of the symbols it references, and it and each of them count as referencing each other, so the test is a member of their cycle and never a dead symbol outside it.

A finding of this kind may carry `manual` instead, as the fixability bullet above states.

From `contract/kinds.json`, row `DS1005`.

### DS1006 deprecated-unused

A symbol carrying a deprecation marker and no production reference: an unused-exported, unused-unexported or unused-member candidate whose symbol is deprecated, reported once under this code. Go identifies the marker by the convention the standard tools recognize, TypeScript by the documentation tag its tools recognize.

Derived from `DS1001`, `DS1002` and `DS1003`.

A finding of this kind may carry `manual` instead, as the fixability bullet above states.

From `contract/kinds.json`, row `DS1006`.

## DS1100 to DS1199: `visibility-narrowing`

Symbols that are alive but more visible than their references require. From `contract/kinds.json`, the `visibility-narrowing` range.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1101` | `unnecessary-export` | `go`, `ts` | `on` | `warn` | `narrowable` |
| `DS1102` | `unnecessary-exposure` | `go` | `on` | `warn` | `narrowable` |
| `DS1103` | `unreachable-export` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1104` | `redundant-export-keyword` | `ts` | `on` | `warn` | `narrowable` |

### DS1101 unnecessary-export

An exported symbol whose every reference is inside the symbol's own package or module, reported as a candidate for unexporting. The subject is a package-level declaration or a method. Three subjects are excluded. An interface method is excluded because its exportedness is the contract of the interface that declares it. A struct field is excluded because an encoder reads it by name. A method that satisfies an interface some symbol uses as a type is excluded because it cannot be unexported without its type ceasing to satisfy that interface, and so is a method whose name and signature are those of a method of an exported interface type declared by a package outside the target that the import closure holds, because that interface fixes them whether or not the program converts the method's type to it. The finding names the narrower visibility the references support. A declared cross-language edge counts as an out-of-package reference. Where the edge's other side is unknown to the analyzer, the finding is emitted pending. A reference from a Go external test package, a test file whose package clause names its directory's package with the suffix _test, is a reference from outside the package, because that test builds as another package and needs the export.

Precondition: Closed world only. A Go main package, and an internal/ directory tree for importers outside its parent, are closed worlds whatever the configuration declares, because no other program can import them. So this finding is reported there always. A published package is a closed world only when the configuration declares the consumer set complete and every declared consumer loads. A library with no consumer loaded and no complete consumer set declared gets this finding on its internal/ tree and its main packages and never on its published API.

A symbol the rule of DS1301 holds for is not reported under this code, whatever severity the configuration gives DS1301, because the write-only finding names the defect to fix first.

An exported function or method may name a type of its own package in a parameter or a result. When code outside the package references that function or method, the type is not reported. The caller holds values of the type, and unexporting it would leave an exported signature naming a type its callers cannot name.

A package-level declaration whose name a string literal spells carries the class `contract/grammar/analysis.md` gives it, which is `possible`.

From `contract/kinds.json`, row `DS1101`.

### DS1102 unnecessary-exposure

An exported symbol of a non-internal package whose every reference is inside the target module, reported as a candidate for relocation behind an internal boundary. The subject is a package-level declaration or a method. Three subjects are excluded. An interface method is excluded because its exportedness is the contract of the interface that declares it. A struct field is excluded because an encoder reads it by name. A method that satisfies an interface some symbol uses as a type is excluded because it cannot be unexported without its type ceasing to satisfy that interface, and so is a method whose name and signature are those of a method of an exported interface type declared by a package outside the target that the import closure holds, because that interface fixes them whether or not the program converts the method's type to it. The finding names the narrower visibility the references support. A declared cross-language edge counts as an out-of-package reference. Where the edge's other side is unknown to the analyzer, the finding is emitted pending. A reference from a Go external test package, a test file whose package clause names its directory's package with the suffix _test, is a reference from outside the package, because that test builds as another package and needs the export.

Precondition: Closed world only. Reported always for a main package and an internal/ directory tree, and for a published package only when the configuration declares the consumer set complete and every declared consumer loads. A library with no consumer loaded and no complete consumer set declared gets this finding on its internal/ tree and its main packages and never on its published API. A symbol the rule of DS1301 holds for is not reported under this code, whatever severity the configuration gives DS1301, because the write-only finding names the defect to fix first.

An exported function or method may name a type of its own package in a parameter or a result. When code outside the target module references that function or method, the type is not reported. The caller holds values of the type. Moving the type behind an internal boundary would leave an exported signature naming a type its callers cannot name.

From `contract/kinds.json`, row `DS1102`.

### DS1103 unreachable-export

An unused exported symbol in a package or file no external code can import: an unused-exported candidate whose enclosing package or file is unimportable from outside, reported once under this code at the certain class, because unimportability is a property of the package graph rather than of the consumer set, except where the confidence rule of grammar/analysis.md gives a declaration a string literal spells the class possible.

Derived from `DS1001`.

A finding of this kind may carry `manual` instead, as the fixability bullet above states.

From `contract/kinds.json`, row `DS1103`.

### DS1104 redundant-export-keyword

An exported declaration used only inside its own file, reported as a candidate for removing the export keyword. The finding names the narrower visibility the references support.

Precondition: Closed world only. Reported in a file that is not an entry file, a file only a configuration string or a script token names being none, because each runs the file and reads no name it exports, while a file a build script's bundler call names is one, because the bundle exposes what it exports, and a test file being one only for the exports its test runner calls, as grammar/analysis.md "Test code" states, and a file a Markdown or MDX document imports one only for the exports the import reads, as grammar/analysis.md "Markdown and MDX documents" states, and that either belongs to a project whose consumer set the configuration declares complete or is reached by no manifest export. A declared cross-language edge counts as a reference from outside the file. Where the edge's other side is unknown to the analyzer, the finding is emitted pending. A symbol the rule of DS1301 holds for is not reported under this code, whatever severity the configuration gives DS1301, because the write-only finding names the defect to fix first.

From `contract/kinds.json`, row `DS1104`.

## DS1200 to DS1299: `interfaces`

Interfaces and interface members nothing uses through the interface. From `contract/kinds.json`, the `interfaces` range.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1201` | `unused-interface` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1203` | `uncalled-interface-method` | `go`, `ts` | `on` | `warn` | `manual` |
| `DS1204` | `unused-satisfaction-assertion` | `go` | `on` | `deny` | `deletable` |

### DS1201 unused-interface

An interface no symbol uses as a type, counting uses from every loaded module. The finding names the concrete implementations and their positions, and the interface's members join its dead component rather than being reported as independent unused declarations.

A finding of this kind may carry `manual` instead, as the fixability bullet above states.

From `contract/kinds.json`, row `DS1201`.

### DS1203 uncalled-interface-method

An interface method that no call site invokes or selects through the interface, whatever the number of implementations. A call or selection in test code is no call for this rule, and an interface method only test code calls is DS1004's. The finding names the concrete implementations and their positions. Each method that implements it and that nothing else keeps is reported under the code its own references select. Such a method counts as referenced by the interface method alone, so it belongs to this finding's dead component, whose root member is the interface method.

Precondition: In Go, every method of an exported interface whose method set holds an unexported method is exempt, embedded methods included. An unexported interface restricts no implementor outside its package, so a method only it holds is judged. That is the sum-type shape, whose method set exists to restrict the implementors, and a TypeScript interface cannot take it because it declares no member less visible than itself. Also exempt, in both languages, is every marker method, an interface method whose every implementation carries an empty body.

From `contract/kinds.json`, row `DS1203`.

### DS1204 unused-satisfaction-assertion

A compile-time satisfaction assertion whose interface no symbol uses as a type.

Precondition: In a library, an assertion whose interface and concrete type are both published API is not reported, because it states a promise to every importer. An assertion on a published interface alone has the reachability class `possible`. A published interface is an exported interface of a package that outside programs can import. The class holds whatever consumers the run declares and loads, because the assertion checks a promise the published API makes to every program that imports the package, not only to the declared consumers: that the type satisfies the interface. A loaded consumer that names the interface as a type uses it, and the assertion is not reported.

From `contract/kinds.json`, row `DS1204`.

## DS1300 to DS1399: `reads-and-writes`

State that carries no information: write-only symbols, enumerated members nothing names, type parameters nothing uses. From `contract/kinds.json`, the `reads-and-writes` range.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1301` | `write-only-symbol` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1302` | `unused-enum-member` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1303` | `unused-type-parameter` | `go`, `ts` | `on` | `deny` | `deletable` |

### DS1301 write-only-symbol

A package-level or module-level variable, struct field, class member, type member or collection that production code writes and never reads. In production mode a read from a test file counts as no read. A write from a loaded consumer's code counts as a read, because the deletion the finding names would break a consumer the target does not own. A write in a consumer's test file counts as a read in the run that counts test references. It counts in a production sweep only where the configuration counts consumer tests as production. In Go a read counts only from a declaration that is live under the finding's liveness relation, which is reference counting: a read from a declaration nothing references, whose own finding has the reachability class `certain`, is no read, so a declaration that only such declarations read is reported where a declaration something references writes it. A comparison of struct values reads every field of the type: in Go, a value of a struct type used as an operand of `==` or `!=`, as a map key, or as a `switch` tag or `case` expression is a read of every field of that type, transitively through the fields of every struct type it holds, recorded at the comparison, because equality reads every field and a field that decides equality carries information. In Go a type argument the program instantiates a type parameter with, where the parameter's constraint is `comparable` or embeds it, is compared as an operand of `==` of that type is, recorded at the instantiation, so the struct values `slices.Equal`, `slices.Index` or `maps.Equal` compares have every field read. `reflect.DeepEqual` is not a comparison under this rule.

In Go, a conversion the compiler accepts between two struct types, or between two pointer types that are not named types and whose base types are struct types, reads every field of the two struct types where their struct type literals are distinct, recorded at the conversion, because the conversion is legal only while their fields are identical, ignoring tags, and so no field of either can be deleted alone. It reads in the same way every field of each pair of distinct struct type literals the identity of the two types compares in turn: through each field's type, and from there through the element, key, base, parameter and result types of the type literals it reaches, through the method signatures of an interface type literal, and through the type arguments of an instantiated generic type. A field reached there has no reference of its own, as grammar/symbol-ref.md states, so it is never a subject and those reads hold no finding back. It stops at a defined type, whose fields the conversion does not read, because a defined type is identical to itself whatever fields it declares. A pair whose two sides are one struct type literal, the two struct types themselves included, as where one type is declared from the other or the conversion is to the operand's own type, reads nothing, because a field deleted from that literal leaves both sides identical. Where the operand's type or the conversion's type is a type parameter, the struct type its constraint's core type is takes that side's place, and a type parameter operand reads in the same way the fields of each struct type argument an instantiation the program holds gives it, an instantiation made through another generic declaration's own type parameters included. A conversion to any other type, an interface type among them, reads no field under this rule.

In TypeScript a comparison of two object references reads no member, because `===` compares identity and never a member.

A TypeScript parameter property, a constructor parameter that carries a modifier, declares a member of its class that the constructor writes with its argument, so the member is judged under this rule, and under every rule that reads a class member, as a member the class body declares is.

In TypeScript a property of an object literal writes the member of the same name that the literal's contextual type declares, whether or not the compiler checks the literal for excess properties. Where that contextual type is a union or an intersection, the property writes the member of that name on each constituent that declares one. So a literal that a declaration annotates, a function returns, a call passes or a `satisfies` expression checks writes the members of the type it is given. A literal whose type the compiler infers, as one a callback returns to a generic call, writes in the same way the member of each property's name that each object type a value of the literal's type reaches declares, through array elements, type arguments and the parameters and results of functions, so `ids.map((id) => ({ id }))` returned as `Row[]` writes `Row.id`. An object literal handed to a callee the target does not implement has its members read, as grammar/analysis.md states.

A store through the value a field or a member holds reads that field or member rather than writing it, because the store lands in storage every other holder of the value reads. In Go that store is an assignment, an increment, a compound assignment or a `delete` whose target is reached through a struct field holding a map, a slice or a pointer. Examples are `s.f[k] = v`, `s.f.x = v` and `delete(s.f, k)`. The same store through a field holding an array or a struct is a store into the field itself. TypeScript holds every object by reference, so an element or property store through a member's value reads the member.

The finding names each write position, so the deletion set is visible. Deleting the subject and its writes also deletes every declaration whose only uses they hold. Such a declaration is a type parameter of the subject's declaring type that the subject's type alone names, an import binding the writes alone use, or a declaration whose every use is in the type annotation of the subject or of another subject this rule reports, or in the writes those findings name. The finding's message names each such declaration, and no finding is reported about the declaration itself.

From `contract/kinds.json`, row `DS1301`.

### DS1302 unused-enum-member

An enumerated member that no symbol names, at the reachability class derived for the member and with no confidence ceiling below it. Every member of a type that carries a string, text or binary conversion method, or that a conversion from an integer or a wire value produces, is retained by the enum-group exemption first, so a member reached only by value is never a candidate.

A finding of this kind may carry `manual` instead, as the fixability bullet above states.

From `contract/kinds.json`, row `DS1302`.

### DS1303 unused-type-parameter

A type parameter of a function, a method or a TypeScript type alias that no part of the declaration's signature, body or aliased type names, at the certain class. The deletion is local to the declaration and the explicit instantiations the reference set already lists. A type parameter of a declaration that is itself dead falls with it and is reported under no code of its own.

Precondition: Function and method type parameters, and in TypeScript those of a type alias, which names no type of its own. A Go type declaration's type parameter is never reported, and neither is one of a TypeScript class or interface, which merges only with declarations of the same type parameters. A Go phantom type parameter such as `type ID[T any] int` makes two instantiations distinct types while naming the parameter nowhere, so deleting it changes the program. A method signature an interface or an object type declares belongs to that type declaration, so its type parameters are never reported either.

From `contract/kinds.json`, row `DS1303`.

## DS1400 to DS1499: `redundant-surface`

The range is retired and holds no live kind. From `contract/kinds.json`, the `redundant-surface` range.

## DS1500 to DS1599: `non-code-artifacts`

Source files nothing builds or imports. From `contract/kinds.json`, the `non-code-artifacts` range.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1501` | `file-never-built` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1502` | `file-never-imported` | `go`, `ts` | `on` | `deny` | `deletable` |

### DS1501 file-never-built

A source file no configuration in the build matrix builds. On the Go side the finding names the build constraint that excluded the file. A file whose build constraint holds only when the ignore tag is set is never reported, because grammar/analysis.md keeps such a file outside the analysis wherever no configuration builds it: the tag is the toolchain's convention for a file kept out of ordinary package builds unless a configuration explicitly satisfies the tag, and built by hand, the toolchain applying no build constraint to a file named on its own command line. A file any other custom tag excludes is reported under a matrix the configuration declares complete, because completeness is the maintainer's assertion that the listed configurations are every one the target builds, and a configuration the maintainer builds by hand belongs in that list.

Precondition: Reported only when the configuration declares the build matrix complete, and never for a file the toolchain ignored solely because it imports "C" under a build with cgo disabled. Such a file is recorded as excluded by cgo rather than as never built.

From `contract/kinds.json`, row `DS1501`.

### DS1502 file-never-imported

A source file that no import from live code reaches, that no root names, and in which no declared cross-language edge names a declaration: an import counts where it is written in a file a root, a live declaration or such an import keeps, or in a live declaration, so a file only dead code imports is reported, under reachability. An import written in a loaded consumer reaches a file of the target as an import written in the target does. An edge's side names a declaration as a root names one, so a file holding a declaration an edge names is evaluated with the edge rather than reported: whether that declaration is dead is the edge evaluation's to say, and the merge resolves it against the paired side.

From `contract/kinds.json`, row `DS1502`.

## DS1600 to DS1699: `dependencies-and-module-machinery`

Declared dependencies and module-file directives that are exactly a no-op. From `contract/kinds.json`, the `dependencies-and-module-machinery` range.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1601` | `unused-dependency` | `go`, `ts` | `on` | `deny` | `manual` |
| `DS1605` | `unused-module-directive` | `go` | `on` | `warn` | `deletable` |

### DS1601 unused-dependency

A directly declared dependency that no import in the target needs. On the Go side, a direct require whose module provides no package any target package or test variant imports. On the TypeScript side, a manifest dependency, development dependency or peer dependency the project's import closure does not need. A deletion finding whose fix would remove the last use of a dependency names that dependency. On the TypeScript side, a needed dependency whose every use is written inside declarations that are members of the run's dead components, or at the top level of a file reported under DS1502, that no command runs and that no needed dependency used otherwise names as a peer, is reported as well, dead only through that code: at the reachability class possible where one of those declarations belongs to a library's published API, and found under reachability alone where one of those declarations or files is, so at probable unless ts.roots_complete is true. A use the configuration makes, a use an unanswered specifier may make and an import none of whose bindings is used are uses by live code.

Precondition: On the Go side the rule is exactly the semantics of `go mod tidy -diff`. A requirement marked indirect is never reported, because it exists to pin a transitive version and removing it changes the build list.

On the TypeScript side the import closure is the files of the run's projects, and four more dependencies are needed. The first is a dependency whose command a script entry, a workflow step or a git hook runs. The second is a dependency a string of a configuration file names. The third is a dependency a string names under a `package.json` member that an applied convention row reads its tool's configuration from. The fourth is a dependency a string names under a configuration file member that an applied row lists as naming the packages its tool loads.

A call of `import.meta.resolve` or `require.resolve` whose argument is a literal specifier needs the dependency the specifier names, as an import of it does.

A `@types` package of a type root is needed by a use of a global or an ambient module it declares, where the compiler configuration lists no `types`: compilers before version 6 include such packages automatically, so the program the analysis builds holds none of their declarations.

A command nothing runs keeps no dependency. A declared dependency that a needed dependency's installed manifest names as a peer, optional or not, is needed. So is each declared peer of that peer, in turn. The declaration is what keeps the package the needed dependency expects beside it present. A dependency of a manifest whose source sits only in configurations that were not built is reported at `possible`.

An installed manifest is the package's manifest in the nearest node_modules directory at or above the directory of the manifest that declares the dependency, and a dependency with none is decided by the import closure alone.

On the TypeScript side the manifests judged are every manifest the analysis reads. A nested package's manifest is one of them, so a run at the repository root judges the dependencies of a package below it. Each is read against its own directory. A dependency is needed by a use written in a file at or below that directory where no manifest between them declares the same name, by a use no one file holds, and as a peer of a needed dependency of the same manifest or of one below it. A nested package's dependency that nothing installs is not judged, because no run requires a nested package to be installed.

From `contract/kinds.json`, row `DS1601`.

### DS1605 unused-module-directive

A replace directive whose target module is absent from the build list, the one case in which the directive is exactly a no-op.

Precondition: Only a replace whose target is absent from the build list. No other module-file directive is reported. An exclude directive is not, because not selected is a counterfactual about a resolution that did not happen. A workspace use entry and a tool directive are not, because each is an external entry point whose invocation lives outside anything the analysis reads.

From `contract/kinds.json`, row `DS1605`.

## DS1700 to DS1799: `self-check`

Suppressions, configured roots, configured declarations and declared edges that no longer match anything, so the run checks its own inputs. From `contract/kinds.json`, the `self-check` range.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1701` | `suppression-without-reason` | `go`, `ts` | `on` | `deny` | `none` |
| `DS1702` | `unscoped-ignore-entry` | `go`, `ts` | `on` | `deny` | `none` |
| `DS1703` | `stale-suppression` | `go`, `ts` | `on` | `deny` | `none` |
| `DS1704` | `unmatched-root` | `go`, `ts` | `on` | `deny` | `none` |
| `DS1705` | `stale-cross-language-edge` | `go`, `ts` | `on` | `deny` | `none` |
| `DS1706` | `unmatched-configured-declaration` | `ts` | `on` | `deny` | `none` |

### DS1701 suppression-without-reason

A suppression that carries no reason, in any of the three documents: an inline directive, an ignore-file entry or a baseline row. For the ignore file and the baseline, an entry or a row whose reason field is absent, empty or whitespace only is refused and reported rather than matched.

From `contract/kinds.json`, row `DS1701`.

### DS1702 unscoped-ignore-entry

An ignore-file entry or a baseline row that names a symbol and no file path. The entry is reported rather than matched, so a bare name cannot mask a match anywhere else in the project. The inline directive is scoped by its position and cannot be unscoped.

From `contract/kinds.json`, row `DS1702`.

### DS1703 stale-suppression

A suppression record that is stale, as grammar/suppression.md defines it, at either mechanism and in the baseline alike. The run exits with the findings code when at least one is reported. The kind is fixed on at deny: no flag, no severity setting and no per-mechanism exception reduces it below a finding, and a configuration naming this code under a severity key is an unimplemented key.

The row carries `"fixed": true`. No configuration changes this kind's enablement or its severity. A configuration that names this code under a severity key, or a family prefix whose range holds it, names a key the product does not implement, and the run ends with the usage code. The sources are `contract/config.schema.json`, `severity`, and `contract/exit-codes.json`, code 2.

From `contract/kinds.json`, row `DS1703`.

### DS1704 unmatched-root

A configured root that matches no symbol, or a configured root pattern that matches no symbol. A root set nobody checks silently changes every result, so a stale one is a finding. The kind is fixed on at deny: no flag, no severity setting and no exception reduces it below a finding, and a configuration naming this code under a severity key is an unimplemented key.

The row carries `"fixed": true`. No configuration changes this kind's enablement or its severity. A configuration that names this code under a severity key, or a family prefix whose range holds it, names a key the product does not implement, and the run ends with the usage code. The sources are `contract/config.schema.json`, `severity`, and `contract/exit-codes.json`, code 2.

From `contract/kinds.json`, row `DS1704`.

### DS1705 stale-cross-language-edge

A declared cross-language edge with a side that at least one analyzer evaluated and every evaluation of that side reports absent, so no analyzer that ran enumerates that side's symbol. Emitted by the merge, once per edge, never by an analyzer, so a stale edge is visible rather than silently inert. A dead side paired with an absent side drops its pending finding, and the edge is the reported defect: a misspelled reference must not turn a live pairing into a deletion.

From `contract/kinds.json`, row `DS1705`.

### DS1706 unmatched-configured-declaration

An entry of a configuration key that names a declaration, being an entry of ts.injection_registrations, of the components or the bases of an entry of ts.lifecycle_contracts, or of ts.serializers, that names no declaration in any project of the run, by the resolution the ts section of contract/config.schema.json states for its shape. An entry that names a declaration no call reaches matches. An exemption whose configured declarations nobody checks silently retains nothing, so a stale entry is a finding. The kind is fixed on at deny: no flag, no severity setting and no exception reduces it below a finding, and a configuration naming this code under a severity key is an unimplemented key.

The row carries `"fixed": true`. No configuration changes this kind's enablement or its severity. A configuration that names this code under a severity key, or a family prefix whose range holds it, names a key the product does not implement, and the run ends with the usage code. The sources are `contract/config.schema.json`, `severity`, and `contract/exit-codes.json`, code 2.

From `contract/kinds.json`, row `DS1706`.

## DS1800 to DS1899: `intra-function`

Dead code inside a function body. The six kinds form one issue group with one enable switch, enabled by default: a severity key naming the range prefix addresses every kind in it at once. Each row names the external linter or rule in each language that reports the same kind, so a repository already running one silences whichever side it prefers. From `contract/kinds.json`, the `intra-function` range.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1801` | `unused-parameter` | `go`, `ts` | `on` | `warn` | `manual` |
| `DS1802` | `unused-receiver` | `go` | `on` | `warn` | `deletable` |
| `DS1803` | `unused-result` | `go`, `ts` | `on` | `warn` | `manual` |
| `DS1805` | `unreachable-statement` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1807` | `dead-store` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1809` | `unreachable-case` | `go`, `ts` | `on` | `deny` | `deletable` |

### DS1801 unused-parameter

A parameter with no reference inside its function body, on a function whose signature is free to change.

Precondition: The signature must be free. A free signature belongs to a function that is not a method retained by interface satisfaction, is not used as a value and is not a go:linkname or cgo target or a go-fuzz entry point. It is also not a function the Go test driver runs, which is a test, benchmark or fuzz test of a test file, or TestMain, and not a stub whose body is empty or only panics.

A method whose name and signature match an exported interface of a package outside the target is not free either, because that interface fixes them. A function literal held in a variable the program only calls is called by name, so its signature is free, and so is a Go function literal that is the callee of the call it is written in.

A parameter its body never names is dead whatever the callers. A published declaration of a library is therefore reported too, with the fixability the vocabulary gives the kind, because the signature change is a breaking change.

In TypeScript a caller may pass a function used as a value more arguments than it declares. In such a function, an unread parameter is reported for deletion only when the body reads no parameter after it, and an unread parameter before a read one is reported for a rename to an underscore name, unless it has one. A TypeScript name a parameter binds, whole or through a binding pattern, that starts with `_` is never reported, because unused-parameter rules read a leading underscore as declared unread. A Go parameter named `_` declares no name and is never reported.

A parameter of a TypeScript callback the compiler types from its context and that carries a type annotation is not reported where the call's inference may read the annotation, a generic callee whose parameter type names a type parameter at its place, and is reported at `probable` where no inference reads it. Nor is it reported when it is an object pattern, which reads properties of the argument.

Any other TypeScript parameter that destructures its argument is judged name by name. Each name the body never reads is reported wherever the parameter stands, because removing it moves no argument. A name beside a rest element of an object pattern is kept, because removing it changes what the rest element holds. The pattern counts as read when the body reads any name it binds. A rest parameter is judged as the last parameter, and a `this` parameter receives no argument and is never reported.

A TypeScript constructor is used as a value where its class is, other than as the callee of `new` or the class an `extends` clause names. Its signature is not free either where the class or one of the constructor's parameters carries a decorator, because a decorator or a container may construct the class. A parameter property is read by the write to its member the constructor makes, so it is never reported here, and its member is judged as a write-only symbol.

Overlap in Go: `revive unused-parameter`, `gopls unusedparams`, `unparam`. Overlap in TypeScript: `tsc --noUnusedParameters`, `@typescript-eslint/no-unused-vars`.

From `contract/kinds.json`, row `DS1801`.

### DS1802 unused-receiver

A named method receiver with no reference inside the method body. Go permits a method with no receiver name, so the fix deletes an identifier and changes no signature.

Precondition: None beyond the rule. Deleting a receiver name changes no signature, so a method retained by interface satisfaction, a method used as a value and a stub are reported as any other method is.

Overlap in Go: `revive unused-receiver`. Overlap in TypeScript: `not applicable`.

From `contract/kinds.json`, row `DS1802`.

### DS1803 unused-result

A result value that no call site of the function uses, on a function whose signature is free to change.

Precondition: The same free-signature rule as unused-parameter, plus a closed-world condition. Every call site of the function must be in the loaded graph, because an unknown caller may consume the result. The kind therefore reports only for a function whose callers are all visible. A function a program run by naming the file names, a file under the `ignore` tag or a script of a dropped module, has call sites outside the loaded graph. A call awaited as a statement of its own uses a result that has a `then` member, a promise or another thenable, because the caller sequences on it, and discards any other result. A call written as a statement of its own, or as the operand of `void`, discards its result whatever its type.

Overlap in Go: `unparam`. Overlap in TypeScript: `none known`.

From `contract/kinds.json`, row `DS1803`.

### DS1805 unreachable-statement

A statement that control flow cannot reach, at the precision the language's compiler applies to the same construct.

Overlap in Go: `go vet unreachable`. Overlap in TypeScript: `tsc allowUnreachableCode`, `eslint no-unreachable`.

From `contract/kinds.json`, row `DS1805`.

### DS1807 dead-store

A write to a local variable with no read before the next write to it or the end of its scope, computed on the same read-and-write classification the write-only-symbol kind uses. The finding names the write position.

Precondition: In TypeScript the binding of a catch clause is a local variable the clause writes when it catches. A binding the clause's block never reads is a dead store, positioned at the binding, because the clause behaves the same with no binding. A function declaration of a block of a function is a local too, so one nothing outside its own body reads is a dead store, and so is a `let` or `var` declaration with no initializer that nothing reads or writes. A local annotated `never` is no subject, because a write to it is the compiler's exhaustiveness check.

A TypeScript local is a name a `let` or `const` of a block, or a `var` of a block of a function, binds, a name of a binding pattern included, except one with a default or beside a rest element. A `var` of a nested block is judged in that block, its last write kept, and is no subject where a nested function names it. A local a nested function reads is no subject. A `for…of` or `for…in` variable the body never reads is a dead store, unless its name starts with `_`.

In Go a function literal a package-level initializer holds is a body of its own, a range statement stores its key and value at the start of every iteration, and a store a function literal naming its variable may precede on some path is not dead, unless the literal is called where it is written outside a `go` or `defer` statement.

Overlap in Go: `ineffassign`, `wastedassign`, `staticcheck SA4006`. Overlap in TypeScript: `eslint no-useless-assignment`, `@typescript-eslint/no-unused-vars`.

From `contract/kinds.json`, row `DS1807`.

### DS1809 unreachable-case

A switch case whose type or value an earlier case in the same switch already covers.

Overlap in Go: `staticcheck SA4020`. Overlap in TypeScript: `none known`.

From `contract/kinds.json`, row `DS1809`.

## Retired codes

Each code below named a kind that this contract no longer defines. No analyzer reports one, and no code here is ever assigned to another kind. The source is `contract/kinds.json`, `retired`.

| Code | Name |
| --- | --- |
| `DS1202` | `single-implementation-interface` |
| `DS1401` | `alias-only-declaration` |
| `DS1402` | `forwarding-only-symbol` |
| `DS1503` | `duplicate-export` |
| `DS1504` | `import-cycle` |
| `DS1505` | `orphan-test-data` |
| `DS1506` | `unread-embed-pattern` |
| `DS1507` | `unused-message-key` |
| `DS1602` | `undeclared-dependency` |
| `DS1603` | `test-only-dependency` |
| `DS1604` | `unresolved-import` |
| `DS1606` | `unused-tool-directive` |
| `DS1607` | `unused-workspace-entry` |
| `DS1608` | `unused-binary` |
| `DS1804` | `constant-result` |
| `DS1806` | `discarded-pure-result` |
| `DS1808` | `write-to-copy` |
| `DS1810` | `empty-branch` |
| `DS1811` | `redundant-conversion` |
