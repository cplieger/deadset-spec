# Issue kinds

Every kind an analyzer reports carries a code, a name, the rule that produces it, a confidence ceiling, a default enablement, a default severity and a fixability. This page states all of them, one table per code range and one section per kind. Every value is read from `contract/kinds.json`, and each section names the row it comes from.

A code is the prefix `DS` followed by four digits. Ranges group codes by family. A code names at most one kind for the life of the code space. A retired code stays retired and is listed in the table that closes this page, rather than naming a different kind later. The source is `contract/kinds.json`, `prefix`, `ranges` and `retired`. One code renders identically in a text output line, an ignore entry, a configuration key and a SARIF rule identifier.

The columns of every table below, each the field of the same name in the kind's row:

- Languages are the languages the kind applies to, where `go` is Go and `ts` is TypeScript and JavaScript. The source is `contract/kinds.json`, `languages`.
- Default is `default_enabled`. `on` means an analyzer reports the kind unless a configuration disables it.
- Severity is `default_severity`, one of `allow`, `warn` and `deny`, as `contract/kinds.json`, `severities`, lists them. A finding at or above the failing severity, `reporters.fail_on`, exits with the findings code. Under the default, `deny`, a `warn` finding never fails a run. The sources are `contract/exit-codes.json`, codes 1 and 0, and the document's `description`. A configuration names a code or a two-digit family prefix under `severity` to change a severity, as `contract/config.schema.json` states.
- Fixability is `fixability`, one of `deletable`, `narrowable`, `manual` and `none`, as `contract/kinds.json`, `fixabilities`, lists them. It says what a mechanical edit may do with the finding.

Confidence is a ceiling on the reachability class, not a second axis. A finding carries a `reachability_class`, which is what the analysis knows about the symbol's callers, and a `confidence`, which is that class capped by the kind's `max_class`. Both take one value from `certain`, `probable` and `possible`, ordered from the strongest. The source is `contract/kinds.json`, `reachability_classes`.

Every kind in this contract version declares the ceiling `certain`, so a kind whose ceiling is lower states it in its own section. A finding about a library's published API, a public member of a published type included, is `possible` when the run holds no consumer information. So is any `DS1004` or `DS1201` finding about test-support code that test code references. The default `analysis.min_confidence`, `probable`, withholds both and reports the rest. The report counts the findings it withheld at each level. The text and SARIF outputs name those counts in one line, with the setting that shows them.

A finding about a member of a dead component is also capped by the lowest confidence among the component's root members. So every finding of one component carries one confidence. The sources are `contract/grammar/analysis.md`, "Confidence", and `contract/config.schema.json`, `analysis.min_confidence`.

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

From `contract/kinds.json`, row `DS1001`.

### DS1002 unused-unexported

An unexported symbol with no reference in the target. A symbol referenced only from its own declaration site counts as unreferenced.

From `contract/kinds.json`, row `DS1002`.

### DS1003 unused-member

A struct field, class member or type member with no reference, a private member included. A member referenced only from its own declaration site counts as unreferenced.

Precondition: A TypeScript member that no value writes and nothing reads may name a type parameter of its declaring interface, class or type alias. It is not reported when every part of that declaration naming the type parameter is such a member. TypeScript compares object types by their members, so those members keep two instantiations of the type apart. Deleting them leaves the type parameter phantom and lets one instantiation stand for another. A member a value writes is decided by the rule of `DS1301` instead, whatever its type names.

From `contract/kinds.json`, row `DS1003`.

### DS1004 test-only-use

A symbol with zero production references and at least one test reference: an unused-exported, unused-unexported or unused-member candidate whose test reference count is not zero, reported once under this code. A reference from a consumer's test files is a test reference unless the configuration counts consumer tests as production. A reference from test-support code, a package or file that only tests reach, is a test reference. A declaration of test-support code that test code references is reported under this code at the reachability class possible, as grammar/analysis.md states. Such a declaration that is an interface is reported under DS1201 instead.

Derived from `DS1001`, `DS1002` and `DS1003`.

From `contract/kinds.json`, row `DS1004`.

### DS1005 test-of-dead-code

A test symbol whose set of referenced target symbols is non-empty and every member of that set is reported dead. A test that references at least one live target symbol is never reported, no notion of a test's subject and no name matching enters the rule, and the message states the rule. The test joins the dead component of the symbols it references, and it and each of them count as referencing each other, so the test is a member of their cycle and never a dead symbol outside it.

From `contract/kinds.json`, row `DS1005`.

### DS1006 deprecated-unused

A symbol carrying a deprecation marker and no production reference: an unused-exported, unused-unexported or unused-member candidate whose symbol is deprecated, reported once under this code. Go identifies the marker by the convention the standard tools recognize, TypeScript by the documentation tag its tools recognize.

Derived from `DS1001`, `DS1002` and `DS1003`.

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

An exported symbol whose every reference is inside the symbol's own package or module, reported as a candidate for unexporting. The subject is a package-level declaration or a method. Three subjects are excluded. An interface method is excluded because its exportedness is the contract of the interface that declares it. A struct field is excluded because an encoder reads it by name. A method that satisfies an interface some symbol uses as a type is excluded because it cannot be unexported without its type ceasing to satisfy that interface. The finding names the narrower visibility the references support. A declared cross-language edge counts as an out-of-package reference. Where the edge's other side is unknown to the analyzer, the finding is emitted pending.

Precondition: Closed world only. A Go main package, and an internal/ directory tree for importers outside its parent, are closed worlds whatever the configuration declares, because no other program can import them. So this finding is reported there always. A published package is a closed world only when the configuration declares the consumer set complete and every declared consumer loads. A library with no consumer loaded and no complete consumer set declared gets this finding on its internal/ tree and its main packages and never on its published API.

A symbol the rule of DS1301 holds for is not reported under this code, whatever severity the configuration gives DS1301, because the write-only finding names the defect to fix first.

An exported function or method may name a type of its own package in a parameter or a result. When code outside the package references that function or method, the type is not reported. The caller holds values of the type, and unexporting it would leave an exported signature naming a type its callers cannot name.

From `contract/kinds.json`, row `DS1101`.

### DS1102 unnecessary-exposure

An exported symbol of a non-internal package whose every reference is inside the target module, reported as a candidate for relocation behind an internal boundary. The subject is a package-level declaration or a method. Three subjects are excluded. An interface method is excluded because its exportedness is the contract of the interface that declares it. A struct field is excluded because an encoder reads it by name. A method that satisfies an interface some symbol uses as a type is excluded because it cannot be unexported without its type ceasing to satisfy that interface. The finding names the narrower visibility the references support. A declared cross-language edge counts as an out-of-package reference. Where the edge's other side is unknown to the analyzer, the finding is emitted pending.

Precondition: Closed world only. Reported always for a main package and an internal/ directory tree, and for a published package only when the configuration declares the consumer set complete and every declared consumer loads. A library with no consumer loaded and no complete consumer set declared gets this finding on its internal/ tree and its main packages and never on its published API. A symbol the rule of DS1301 holds for is not reported under this code, whatever severity the configuration gives DS1301, because the write-only finding names the defect to fix first.

An exported function or method may name a type of its own package in a parameter or a result. When code outside the target module references that function or method, the type is not reported. The caller holds values of the type. Moving the type behind an internal boundary would leave an exported signature naming a type its callers cannot name.

From `contract/kinds.json`, row `DS1102`.

### DS1103 unreachable-export

An unused exported symbol in a package or file no external code can import: an unused-exported candidate whose enclosing package or file is unimportable from outside, reported once under this code at the certain class, because unimportability is a property of the package graph rather than of the consumer set.

Derived from `DS1001`.

From `contract/kinds.json`, row `DS1103`.

### DS1104 redundant-export-keyword

An exported declaration used only inside its own file, reported as a candidate for removing the export keyword. The finding names the narrower visibility the references support.

Precondition: Closed world only. Reported in a file that is not an entry file and that either belongs to a project whose consumer set the configuration declares complete or is reached by no manifest export. A declared cross-language edge counts as a reference from outside the file. Where the edge's other side is unknown to the analyzer, the finding is emitted pending. A symbol the rule of DS1301 holds for is not reported under this code, whatever severity the configuration gives DS1301, because the write-only finding names the defect to fix first.

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

From `contract/kinds.json`, row `DS1201`.

### DS1203 uncalled-interface-method

An interface method that no call site invokes or selects through the interface, whatever the number of implementations. The finding names the concrete implementations and their positions. Each method that implements it and that nothing else keeps is reported under the code its own references select. Such a method counts as referenced by the interface method alone, so it belongs to this finding's dead component, whose root member is the interface method.

Precondition: In Go, every method of an interface that declares an unexported method is exempt. That is the sum-type shape, whose method set exists to restrict the implementors, and a TypeScript interface cannot take it because it declares no member less visible than itself. Also exempt, in both languages, is every marker method, an interface method whose every implementation carries an empty body.

From `contract/kinds.json`, row `DS1203`.

### DS1204 unused-satisfaction-assertion

A compile-time satisfaction assertion whose interface no symbol uses as a type.

From `contract/kinds.json`, row `DS1204`.

## DS1300 to DS1399: `reads-and-writes`

State that carries no information: write-only symbols, enumerated members nothing names, type parameters nothing uses. From `contract/kinds.json`, the `reads-and-writes` range.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1301` | `write-only-symbol` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1302` | `unused-enum-member` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1303` | `unused-type-parameter` | `go`, `ts` | `on` | `deny` | `deletable` |

### DS1301 write-only-symbol

A package-level or module-level variable, struct field, class member, type member or collection that production code writes and never reads. In production mode a read from a test file counts as no read. A write from a loaded consumer's code counts as a read, because the deletion the finding names would break a consumer the target does not own. A write in a consumer's test file counts as a read in the run that counts test references. It counts in a production sweep only where the configuration counts consumer tests as production. A comparison of struct values reads every field of the type: in Go, a value of a struct type used as an operand of `==` or `!=`, as a map key, or as a `switch` tag or `case` expression is a read of every field of that type, transitively through the fields of every struct type it holds, recorded at the comparison, because equality reads every field and a field that decides equality carries information. In Go a type argument the program instantiates a type parameter with, where the parameter's constraint is `comparable` or embeds it, is compared as an operand of `==` of that type is, recorded at the instantiation, so the struct values `slices.Equal`, `slices.Index` or `maps.Equal` compares have every field read. `reflect.DeepEqual` is not a comparison under this rule. In TypeScript a comparison of two object references reads no member, because `===` compares identity and never a member.

In TypeScript a property of an object literal writes the member of the same name that the literal's contextual type declares, whether or not the compiler checks the literal for excess properties. Where that contextual type is a union or an intersection, the property writes the member of that name on each constituent that declares one. So a literal that a declaration annotates, a function returns, a call passes or a `satisfies` expression checks writes the members of the type it is given.

A store through the value a field or a member holds reads that field or member rather than writing it, because the store lands in storage every other holder of the value reads. In Go that store is an assignment, an increment, a compound assignment or a `delete` whose target is reached through a struct field holding a map, a slice or a pointer. Examples are `s.f[k] = v`, `s.f.x = v` and `delete(s.f, k)`. The same store through a field holding an array or a struct is a store into the field itself. TypeScript holds every object by reference, so an element or property store through a member's value reads the member.

The finding names each write position, so the deletion set is visible. Deleting the subject and its writes also deletes every declaration whose only uses they hold. Such a declaration is a type parameter of the subject's declaring type that the subject's type alone names, an import binding the writes alone use, or a declaration whose every use is in the type annotation of the subject or of another subject this rule reports, or in the writes those findings name. The finding's message names each such declaration, and no finding is reported about the declaration itself.

From `contract/kinds.json`, row `DS1301`.

### DS1302 unused-enum-member

An enumerated member that no symbol names, at the reachability class derived for the member and with no confidence ceiling below it. Every member of a type that carries a string, text or binary conversion method, or that a conversion from an integer or a wire value produces, is retained by the enum-group exemption first, so a member reached only by value is never a candidate.

From `contract/kinds.json`, row `DS1302`.

### DS1303 unused-type-parameter

A type parameter of a function or method that no part of the declaration's signature and no part of the declaration's body names, at the certain class. The deletion is local to the declaration and the explicit instantiations the reference set already lists. A type parameter of a function or method that is itself dead falls with it and is reported under no code of its own.

Precondition: Function and method type parameters only. A type parameter of a type declaration is never reported. A phantom type parameter such as `type ID[T any] int` makes two instantiations distinct types while naming the parameter nowhere, so deleting it changes the program. A method signature an interface or an object type declares belongs to that type declaration, so its type parameters are never reported either.

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

A source file no configuration in the build matrix builds. On the Go side the finding names the build constraint that excluded the file. A file whose build constraint is the ignore tag is never reported: the toolchain applies no build constraint to a file named on its own command line, so that tag is the toolchain's convention for a file built by hand. A file any other custom tag excludes is reported under a matrix the configuration declares complete, because completeness is the maintainer's assertion that the listed configurations are every one the target builds, and a configuration the maintainer builds by hand belongs in that list.

Precondition: Reported only when the configuration declares the build matrix complete, and never for a file the toolchain ignored solely because it imports "C" under a build with cgo disabled. Such a file is recorded as excluded by cgo rather than as never built.

From `contract/kinds.json`, row `DS1501`.

### DS1502 file-never-imported

A source file that no import reaches, that no root names, and in which no declared cross-language edge names a declaration. An import written in a loaded consumer reaches a file of the target as an import written in the target does. An edge's side names a declaration as a root names one, so a file holding a declaration an edge names is evaluated with the edge rather than reported: whether that declaration is dead is the edge evaluation's to say, and the merge resolves it against the paired side.

From `contract/kinds.json`, row `DS1502`.

## DS1600 to DS1699: `dependencies-and-module-machinery`

Declared dependencies and module-file directives that are exactly a no-op. From `contract/kinds.json`, the `dependencies-and-module-machinery` range.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1601` | `unused-dependency` | `go`, `ts` | `on` | `deny` | `manual` |
| `DS1605` | `unused-module-directive` | `go` | `on` | `warn` | `deletable` |

### DS1601 unused-dependency

A directly declared dependency that no import in the target needs. On the Go side, a direct require whose module provides no package any target package or test variant imports. On the TypeScript side, a manifest dependency, development dependency or peer dependency the project's import closure does not need. A deletion finding whose fix would remove the last use of a dependency names that dependency.

Precondition: On the Go side the rule is exactly the semantics of `go mod tidy -diff`. A requirement marked indirect is never reported, because it exists to pin a transitive version and removing it changes the build list.

On the TypeScript side the import closure is the files of the run's projects, and four more dependencies are needed. The first is a dependency whose installed manifest declares a command, because a command is run by name rather than imported. The second is a dependency that the installed manifest of a needed dependency declares as a peer and does not mark optional. The third is, to a fixpoint, the required peers of every dependency so held. The fourth is a dependency a string names under a `package.json` member that an applied convention row reads its tool's configuration from.

An installed manifest is the package's manifest in the nearest node_modules directory at or above the target, and a dependency with none is decided by the import closure alone.

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

A suppression that matches no current finding, at either mechanism and in the baseline alike. The run exits with the findings code when at least one is reported. The kind is fixed on at deny: no flag, no severity setting and no per-mechanism exception reduces it below a finding, and a configuration naming this code under a severity key is an unimplemented key.

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

Precondition: The signature must be free. A free signature belongs to a function that is not a method retained by interface satisfaction, is not used as a value and is not a go:linkname or cgo target. It is also not a function the Go test driver runs, which is a test, benchmark or fuzz test of a test file, or TestMain, and not a stub whose body is empty or only panics.

A parameter its body never names is dead whatever the callers. A published declaration of a library is therefore reported too, with the fixability the vocabulary gives the kind, because the signature change is a breaking change.

In TypeScript a caller may pass a function used as a value more arguments than it declares. In such a function, an unread parameter is reported only when the body reads no parameter after it. No name exempts a parameter, so a TypeScript parameter whose name starts with `_` is judged as any other. A Go parameter named `_` declares no name and is never reported.

A TypeScript parameter that destructures its argument is judged name by name. Each name the body never reads is reported wherever the parameter stands, because removing it moves no argument. A name beside a rest element of an object pattern is kept, because removing it changes what the rest element holds. The pattern counts as read when the body reads any name it binds. A rest parameter is judged as the last parameter, and a `this` parameter receives no argument and is never reported.

Overlap in Go: `revive unused-parameter`, `gopls unusedparams`, `unparam`. Overlap in TypeScript: `tsc --noUnusedParameters`, `@typescript-eslint/no-unused-vars`.

From `contract/kinds.json`, row `DS1801`.

### DS1802 unused-receiver

A named method receiver with no reference inside the method body, on a method whose signature is free to change. Go permits a method with no receiver name, so the fix deletes an identifier and changes no signature.

Precondition: The same free-signature rule as unused-parameter, applied to the receiver.

Overlap in Go: `revive unused-receiver`. Overlap in TypeScript: `not applicable`.

From `contract/kinds.json`, row `DS1802`.

### DS1803 unused-result

A result value that no call site of the function uses, on a function whose signature is free to change.

Precondition: The same free-signature rule as unused-parameter, plus a closed-world condition. Every call site of the function must be in the loaded graph, because an unknown caller may consume the result. The kind therefore reports only for a function whose callers are all visible.

Overlap in Go: `unparam`. Overlap in TypeScript: `none known`.

From `contract/kinds.json`, row `DS1803`.

### DS1805 unreachable-statement

A statement that control flow cannot reach, at the precision the language's compiler applies to the same construct.

Overlap in Go: `go vet unreachable`. Overlap in TypeScript: `tsc allowUnreachableCode`, `eslint no-unreachable`.

From `contract/kinds.json`, row `DS1805`.

### DS1807 dead-store

A write to a local variable with no read before the next write to it or the end of its scope, computed on the same read-and-write classification the write-only-symbol kind uses. The finding names the write position.

Precondition: In TypeScript the binding of a catch clause is a local variable the clause writes when it catches. A binding the clause's block never reads is a dead store, positioned at the binding, because the clause behaves the same with no binding.

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
