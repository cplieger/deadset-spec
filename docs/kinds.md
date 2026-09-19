# Issue kinds

Every kind an analyzer reports carries a code, a name, the rule that produces it, a confidence
ceiling, a default enablement, a default severity and a fixability. This page states all of them,
one table per code range and one section per kind. Every value is read from
`contract/kinds.json`, and each section names the row it comes from.

A code is the prefix `DS` followed by four digits. Ranges group codes by family, and a code names
at most one kind for the life of the code space: a retired code stays retired and closes this page
as a table rather than naming a different kind later (from `contract/kinds.json`, `prefix`,
`ranges` and `retired`). One code renders identically in a text output line, an ignore entry, a
configuration key and a SARIF rule identifier.

The columns of every table below, each the field of the same name in the kind's row:

- **Languages** are the languages the kind applies to: `go` is Go, `ts` is TypeScript and
  JavaScript (from `contract/kinds.json`, `languages`).
- **Default** is `default_enabled`. `on` means an analyzer reports the kind unless a configuration
  disables it.
- **Severity** is `default_severity`, one of `allow`, `warn` and `deny` (from
  `contract/kinds.json`, `severities`). A finding at or above the failing severity exits with the
  findings code, and a `warn` finding never fails a run (from `contract/exit-codes.json`, codes 1
  and 0). A configuration names a code or a two-digit family prefix to change a severity (from
  `contract/config.schema.json`, `severity`).
- **Fixability** is `fixability`, one of `deletable`, `narrowable`, `manual` and `none` (from
  `contract/kinds.json`, `fixabilities`): what a mechanical edit may do with the finding.

Confidence is a ceiling on the reachability class, not a second axis. A finding carries a
`reachability_class`, which is what the analysis knows about the symbol's callers, and a
`confidence`, which is that class capped by the kind's `max_class`; both take one value from
`certain`, `probable` and `possible`, ordered from the strongest (from `contract/kinds.json`,
`reachability_classes`). Every kind in this contract version declares the ceiling `certain`, so a
kind whose ceiling is lower states it in its own section.

Three fields appear on a row only where they apply, and each section carries them where present.
`precondition` is a condition the analyzer checks before it reports the kind. `derived_from` names
the kinds a derived kind is computed from; such a finding is reported once, under the most specific
code. `overlap` names, per language, the external linters or rules that report the same kind, so a
project already running one silences whichever side it prefers; `none known` and `not applicable`
are values of that field rather than omissions.

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

From `contract/kinds.json`, row `DS1003`.

### DS1004 test-only-use

A symbol with zero production references and at least one test reference: an unused-exported or unused-unexported candidate whose test reference count is not zero, reported once under this code. A reference from a consumer's test files is a test reference unless the configuration counts consumer tests as production.

Derived from `DS1001` and `DS1002`.

From `contract/kinds.json`, row `DS1004`.

### DS1005 test-of-dead-code

A test symbol whose set of referenced target symbols is non-empty and every member of that set is reported dead. A test that references at least one live target symbol is never reported, no notion of a test's subject and no name matching enters the rule, and the message states the rule. The test joins the dead component of the symbols it references.

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

An exported symbol whose every reference is inside the symbol's own package or module, reported as a candidate for unexporting. The subject is a package-level declaration or a method, and three subjects are excluded: an interface method, whose exportedness is the contract of the interface that declares it; a struct field, which an encoder reads by name; and a method that satisfies an interface some symbol uses as a type, which cannot be unexported without its type ceasing to satisfy that interface. The finding names the narrower visibility the references support. A declared cross-language edge counts as an out-of-package reference; where the edge's other side is unknown to the analyzer the finding is emitted pending.

Precondition: Closed world only. Reported always for a main package and an internal/ directory tree, and for a published package only when the configuration declares the consumer set complete and every declared consumer loads. A library with no consumer loaded and no complete consumer set declared gets this finding on its internal/ tree and its main packages and never on its published API.

From `contract/kinds.json`, row `DS1101`.

### DS1102 unnecessary-exposure

An exported symbol of a non-internal package whose every reference is inside the target module, reported as a candidate for relocation behind an internal boundary. The subject is a package-level declaration or a method, and three subjects are excluded: an interface method, whose exportedness is the contract of the interface that declares it; a struct field, which an encoder reads by name; and a method that satisfies an interface some symbol uses as a type, which cannot be unexported without its type ceasing to satisfy that interface. The finding names the narrower visibility the references support. A declared cross-language edge counts as an out-of-package reference; where the edge's other side is unknown to the analyzer the finding is emitted pending.

Precondition: Closed world only. Reported always for a main package and an internal/ directory tree, and for a published package only when the configuration declares the consumer set complete and every declared consumer loads. A library with no consumer loaded and no complete consumer set declared gets this finding on its internal/ tree and its main packages and never on its published API.

From `contract/kinds.json`, row `DS1102`.

### DS1103 unreachable-export

An unused exported symbol in a package or file no external code can import: an unused-exported candidate whose enclosing package or file is unimportable from outside, reported once under this code at the certain class, because unimportability is a property of the package graph rather than of the consumer set.

Derived from `DS1001`.

From `contract/kinds.json`, row `DS1103`.

### DS1104 redundant-export-keyword

An exported declaration used only inside its own file, reported as a candidate for removing the export keyword. The finding names the narrower visibility the references support.

Precondition: Closed world only. Reported in a file that is not an entry file and that either belongs to a project whose consumer set the configuration declares complete or is reached by no manifest export. A declared cross-language edge counts as a reference from outside the file; where the edge's other side is unknown to the analyzer the finding is emitted pending.

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

An interface method that no call site invokes or selects through the interface, whatever the number of implementations. The finding names the concrete implementations and their positions.

Precondition: Exempt: every method of an interface that declares an unexported method, the sum-type shape whose method set exists to restrict the implementors; and every marker method, an interface method whose every implementation carries an empty body.

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

A package-level or module-level variable, struct field, class member or collection that production code writes and never reads. In production mode a read from a test file counts as no read. A comparison of struct values reads every field of the type: in Go, a value of a struct type used as an operand of `==` or `!=`, as a map key, or as a `switch` tag or `case` expression is a read of every field of that type, transitively through the fields of every struct type it holds, recorded at the comparison, because equality reads every field and a field that decides equality carries information. In TypeScript a comparison of two object references reads no member, because `===` compares identity and never a member. The finding names each write position, so the deletion set is visible.

From `contract/kinds.json`, row `DS1301`.

### DS1302 unused-enum-member

An enumerated member that no symbol names, at the reachability class derived for the member and with no confidence ceiling below it. Every member of a type that carries a string, text or binary conversion method, or that a conversion from an integer or a wire value produces, is retained by the enum-group exemption first, so a member reached only by value is never a candidate.

From `contract/kinds.json`, row `DS1302`.

### DS1303 unused-type-parameter

A type parameter of a function or method that no part of the declaration's signature and no part of the declaration's body names, at the certain class. The deletion is local to the declaration and the explicit instantiations the reference set already lists.

Precondition: Function and method type parameters only. A type parameter of a type declaration is never reported, because a phantom type parameter such as `type ID[T any] int` makes two instantiations distinct types while naming the parameter nowhere, so deleting it changes the program.

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

A source file no configuration in the build matrix builds. On the Go side the finding names the build constraint that excluded the file.

Precondition: Reported only when the configuration declares the build matrix complete, and never for a file the toolchain ignored solely because it imports "C" under a build with cgo disabled; such a file is recorded as excluded by cgo rather than as never built.

From `contract/kinds.json`, row `DS1501`.

### DS1502 file-never-imported

A source file that no import reaches and no root names.

From `contract/kinds.json`, row `DS1502`.

## DS1600 to DS1699: `dependencies-and-module-machinery`

Declared dependencies and module-file directives that are exactly a no-op. From `contract/kinds.json`, the `dependencies-and-module-machinery` range.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1601` | `unused-dependency` | `go`, `ts` | `on` | `deny` | `manual` |
| `DS1605` | `unused-module-directive` | `go` | `on` | `warn` | `deletable` |

### DS1601 unused-dependency

A directly declared dependency that no import in the target needs. On the Go side, a direct require whose module provides no package any target package or test variant imports. On the TypeScript side, a manifest dependency, development dependency or peer dependency the project's import closure does not need. A deletion finding whose fix would remove the last use of a dependency names that dependency.

Precondition: On the Go side the rule is the semantics of `go mod tidy -diff` exactly: a requirement marked indirect is never reported, because it exists to pin a transitive version and removing it changes the build list.

From `contract/kinds.json`, row `DS1601`.

### DS1605 unused-module-directive

A replace directive whose target module is absent from the build list, the one case in which the directive is exactly a no-op.

Precondition: Only a replace whose target is absent from the build list. No other module-file directive is reported: not an exclude directive, because not selected is a counterfactual about a resolution that did not happen; not a workspace use entry and not a tool directive, because each is an external entry point whose invocation lives outside anything the analysis reads.

From `contract/kinds.json`, row `DS1605`.

## DS1700 to DS1799: `self-check`

Suppressions, configured roots and declared edges that no longer match anything; the run checks its own inputs. From `contract/kinds.json`, the `self-check` range.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1701` | `suppression-without-reason` | `go`, `ts` | `on` | `deny` | `none` |
| `DS1702` | `unscoped-ignore-entry` | `go`, `ts` | `on` | `deny` | `none` |
| `DS1703` | `stale-suppression` | `go`, `ts` | `on` | `deny` | `none` |
| `DS1704` | `unmatched-root` | `go`, `ts` | `on` | `deny` | `none` |
| `DS1705` | `stale-cross-language-edge` | `go`, `ts` | `on` | `deny` | `none` |

### DS1701 suppression-without-reason

A suppression that carries no reason, in any of the three documents: an inline directive, an ignore-file entry or a baseline row. For the ignore file and the baseline, an entry or a row whose reason field is absent, empty or whitespace only is refused and reported rather than matched.

From `contract/kinds.json`, row `DS1701`.

### DS1702 unscoped-ignore-entry

An ignore-file entry or a baseline row that names a symbol and no file path. The entry is reported rather than matched, so a bare name cannot mask a match anywhere else in the project; the inline directive is scoped by its position and cannot be unscoped.

From `contract/kinds.json`, row `DS1702`.

### DS1703 stale-suppression

A suppression that matches no current finding, at either mechanism and in the baseline alike. The run exits with the findings code when at least one is reported. The kind is fixed on at deny: no flag, no severity setting and no per-mechanism exception reduces it below a finding, and a configuration naming this code under a severity key is an unimplemented key.

The row carries `"fixed": true`. No configuration changes this kind's enablement or its severity: a configuration that names this code under a severity key, or a family prefix whose range holds it, names a key the product does not implement and the run ends with the usage code (from `contract/config.schema.json`, `severity`, and `contract/exit-codes.json`, code 2).

From `contract/kinds.json`, row `DS1703`.

### DS1704 unmatched-root

A configured root that matches no symbol, or a configured root pattern that matches no symbol. A root set nobody checks silently changes every result, so a stale one is a finding. The kind is fixed on at deny: no flag, no severity setting and no exception reduces it below a finding, and a configuration naming this code under a severity key is an unimplemented key.

The row carries `"fixed": true`. No configuration changes this kind's enablement or its severity: a configuration that names this code under a severity key, or a family prefix whose range holds it, names a key the product does not implement and the run ends with the usage code (from `contract/config.schema.json`, `severity`, and `contract/exit-codes.json`, code 2).

From `contract/kinds.json`, row `DS1704`.

### DS1705 stale-cross-language-edge

A declared cross-language edge with a side that at least one analyzer evaluated and every evaluation of that side reports absent, so no analyzer that ran enumerates that side's symbol. Emitted by the merge, once per edge, never by an analyzer, so a stale edge is visible rather than silently inert. A dead side paired with an absent side drops its pending finding, and the edge is the reported defect: a misspelled reference must not turn a live pairing into a deletion.

From `contract/kinds.json`, row `DS1705`.

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

Precondition: The signature must be free, defined as follows: the function is not exported from a target whose consumer set is not declared complete, is not a method retained by interface satisfaction, is not used as a value, is not a go:linkname or cgo target, and is not a stub whose body is empty or only panics. A parameter is part of the function's type, and a caller the analysis cannot see may pass it.

Overlap: in Go `revive unused-parameter`, `gopls unusedparams`, `unparam`; in TypeScript `tsc --noUnusedParameters`, `@typescript-eslint/no-unused-vars`.

From `contract/kinds.json`, row `DS1801`.

### DS1802 unused-receiver

A named method receiver with no reference inside the method body, on a method whose signature is free to change. Go permits a method with no receiver name, so the fix deletes an identifier and changes no signature.

Precondition: The same free-signature rule as unused-parameter, applied to the receiver.

Overlap: in Go `revive unused-receiver`; in TypeScript `not applicable`.

From `contract/kinds.json`, row `DS1802`.

### DS1803 unused-result

A result value that no call site of the function uses, on a function whose signature is free to change.

Precondition: The same free-signature rule as unused-parameter, plus a closed-world condition: every call site of the function must be in the loaded graph, because an unknown caller may consume the result, so the kind reports only for a function whose callers are all visible.

Overlap: in Go `unparam`; in TypeScript `none known`.

From `contract/kinds.json`, row `DS1803`.

### DS1805 unreachable-statement

A statement that control flow cannot reach, at the precision the language's compiler applies to the same construct.

Overlap: in Go `go vet unreachable`; in TypeScript `tsc allowUnreachableCode`, `eslint no-unreachable`.

From `contract/kinds.json`, row `DS1805`.

### DS1807 dead-store

A write to a local variable with no read before the next write to it or the end of its scope, computed on the same read-and-write classification the write-only-symbol kind uses. The finding names the write position.

Overlap: in Go `ineffassign`, `wastedassign`, `staticcheck SA4006`; in TypeScript `eslint no-useless-assignment`.

From `contract/kinds.json`, row `DS1807`.

### DS1809 unreachable-case

A switch case whose type or value an earlier case in the same switch already covers.

Overlap: in Go `staticcheck SA4020`; in TypeScript `none known`.

From `contract/kinds.json`, row `DS1809`.

## Retired codes

Each code below named a kind that this contract no longer defines. No analyzer reports one, and no
code here is ever assigned to another kind (from `contract/kinds.json`, `retired`).

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
