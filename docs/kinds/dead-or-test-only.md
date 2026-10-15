# Only dead code or tests use it: `DS1100` to `DS1199`

Something refers to it, but only dead code or tests. Declarations that only dead code or only tests reach, exports of a package nothing outside can import, and tests of dead code. Generated from `contract/kinds.json`; the [issue kinds](../kinds.md) page lists every family.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1101` | `unreachable-export` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1102` | `test-only-use` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1103` | `test-of-dead-code` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1104` | `deprecated-unused` | `go`, `ts` | `on` | `deny` | `deletable` |

## DS1101 unreachable-export

An exported declaration in a package or file nothing outside can import, so its export marks no real surface and nothing inside uses it.

Before:

```go
// internal/cache, which only this module can import
package cache

func Warm() { ... } // nothing in the module calls it
```

After:

```go
package cache
```

How to fix it: Delete the declaration. The finding's fixability is `deletable`, or `manual` where a rule of the analysis gives it.

The rule: An unused exported symbol in a package or file no external code can import: an unused-exported candidate whose enclosing package or file is unimportable from outside, reported once under this code at the certain class, because unimportability is a property of the package graph rather than of the consumer set, except where the confidence rule of grammar/analysis/confidence.md gives a declaration a string literal spells the class possible.

Derived from `DS1001`: a finding is reported once, under the most specific code.

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

## DS1102 test-only-use

A declaration only tests use: production code never refers to it, so it exists for its tests alone.

Before:

```go
func normalize(s string) string { ... } // only normalize_test.go calls it
```

After:

```go
// normalize and its test are deleted, or the helper moves to a _test.go file.
```

How to fix it: Delete the declaration and the tests of it, or move it into the test files when the tests need it as a helper. The finding's fixability is `deletable`, or `manual` where a rule of the analysis gives it.

The rule: A symbol with zero production references and at least one test reference: an unused-exported, unused-unexported or unused-member candidate whose test reference count is not zero, reported once under this code. A reference from a consumer's test files is a test reference unless the configuration counts consumer tests as production. A reference from test-support code, a package or file that only tests reach and that carries test evidence of its own language, is a test reference. A declaration of test-support code is test code and is no subject of this code, as grammar/analysis/tests.md states. An interface method whose only calls and selections through the interface are in test code, and that DS1202's precondition does not exempt, is reported under this code. The methods implementing it are not reported with it, because the test reaches them through it. A candidate found dead under reachability, every production reference to which is made by a declaration itself dead, is reported under this code as well where test code that is no test of dead code, and that the run counting test references reaches, references it, because that test keeps it, and the finding's message names both kinds of reference.

Derived from `DS1001`, `DS1002`, `DS1003`: a finding is reported once, under the most specific code.

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
- `possible` ([`confidence.21`](../../contract/grammar/analysis/confidence.md)): A declaration of a Go package only tests reach that is no test-support package and that test code references, and each dead declaration it reaches or holds.
- `possible` ([`confidence.23`](../../contract/grammar/analysis/confidence.md)): A member that only records of exemption classes whose confidence is possible hold.
- `probable` ([`confidence.25`](../../contract/grammar/analysis/confidence.md)): A TypeScript declaration found dead under reachability alone, or a file only dead code imports, while ts.roots_complete is not true.
- `probable` ([`confidence.26`](../../contract/grammar/analysis/confidence.md)): A finding the roots cap applies to, in a configuration that compiles a load by a computed path.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1103 test-of-dead-code

A test whose every subject is dead: it keeps testing code nothing in production uses.

Before:

```ts
// legacy.ts: nothing imports formatLegacy
export function formatLegacy(n: number): string { return n.toFixed(2); }

// legacy.test.ts
test("formats", () => expect(formatLegacy(1)).toBe("1.00"));
```

After:

```ts
// legacy.ts and legacy.test.ts are both deleted.
```

How to fix it: Delete the test with the code it tests. The finding's fixability is `deletable`, or `manual` where a rule of the analysis gives it.

The rule: A test symbol whose set of referenced target symbols is non-empty and every member of that set is reported dead. In TypeScript an ambient declaration, one a declaration file holds or one a `declare` modifier carries or a declaration carrying one holds, describes code the target does not implement, so it is no target symbol of that set. A test that references at least one live target symbol is never reported, no notion of a test's subject and no name matching enters the rule, and the message states the rule. A production declaration whose reachability class is `possible` because programs outside the analysis may use it, being published API of a library whose run declares no consumer, and in Go a declaration of a package no build links or an export of a unit a dropped module requires, counts as a live target symbol of every test that references it, so such a test is no test of dead code. The declaration is still reported at `possible`. The test joins the dead component of the symbols it references, and it and each of them count as referencing each other, so the test is a member of their cycle and never a dead symbol outside it.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `probable` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, while a declared consumer did not load.
- `possible` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, in a run that holds no consumer information.
- `possible` ([`roots.25`](../../contract/grammar/analysis/roots.md)): A declaration outside an internal tree of a Go package of an application that no build links, in a run that declares no consumer.
- `possible` ([`confidence.13`](../../contract/grammar/analysis/confidence.md)): A TypeScript member whose name the left operand of an in expression of a production file spells.
- `possible` ([`confidence.14`](../../contract/grammar/analysis/confidence.md)): A field the encoding-reflection class records only as filled by a callee outside the analyzed program.
- `possible` ([`confidence.19`](../../contract/grammar/analysis/confidence.md)): A member of a type the program only decodes into, where a callee outside the analysis, or an any or unknown value, fills it.
- `possible` ([`confidence.20`](../../contract/grammar/analysis/confidence.md)): A declaration of a TypeScript file only tests reach that is no test-support file, and each dead declaration it reaches through dead declarations.
- `possible` ([`confidence.21`](../../contract/grammar/analysis/confidence.md)): A declaration of a Go package only tests reach that is no test-support package and that test code references, and each dead declaration it reaches or holds.
- `possible` ([`confidence.23`](../../contract/grammar/analysis/confidence.md)): A member that only records of exemption classes whose confidence is possible hold.
- `probable` ([`confidence.25`](../../contract/grammar/analysis/confidence.md)): A TypeScript declaration found dead under reachability alone, or a file only dead code imports, while ts.roots_complete is not true.
- `probable` ([`confidence.26`](../../contract/grammar/analysis/confidence.md)): A finding the roots cap applies to, in a configuration that compiles a load by a computed path.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1104 deprecated-unused

A declaration marked deprecated that no production code uses any more.

Before:

```go
// Deprecated: use OpenContext.
func Open(path string) (*DB, error) { ... }
```

After:

```go
// Open is deleted; OpenContext stays.
```

How to fix it: Delete the declaration: its replacement already carries every use. The finding's fixability is `deletable`, or `manual` where a rule of the analysis gives it.

The rule: A symbol carrying a deprecation marker and no production reference: an unused-exported, unused-unexported or unused-member candidate whose symbol is deprecated, reported once under this code. Go identifies the marker by the convention the standard tools recognize, TypeScript by the documentation tag its tools recognize.

Derived from `DS1001`, `DS1002`, `DS1003`: a finding is reported once, under the most specific code.

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
- `possible` ([`confidence.21`](../../contract/grammar/analysis/confidence.md)): A declaration of a Go package only tests reach that is no test-support package and that test code references, and each dead declaration it reaches or holds.
- `possible` ([`confidence.23`](../../contract/grammar/analysis/confidence.md)): A member that only records of exemption classes whose confidence is possible hold.
- `probable` ([`confidence.25`](../../contract/grammar/analysis/confidence.md)): A TypeScript declaration found dead under reachability alone, or a file only dead code imports, while ts.roots_complete is not true.
- `probable` ([`confidence.26`](../../contract/grammar/analysis/confidence.md)): A finding the roots cap applies to, in a configuration that compiles a load by a computed path.
- A finding about a member of a dead component carries no higher confidence than the component's root members.
