# Unused: `DS1000` to `DS1099`

Nothing refers to it. Declarations, members, imports, enum members and type parameters that nothing in the program refers to. Generated from `contract/kinds.json`; the [issue kinds](../kinds.md) page lists every family.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1001` | `unused-exported` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1002` | `unused-unexported` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1003` | `unused-member` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1004` | `unused-import` | `ts` | `on` | `deny` | `deletable` |
| `DS1005` | `unused-enum-member` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1006` | `unused-type-parameter` | `go`, `ts` | `on` | `deny` | `deletable` |

## DS1001 unused-exported

An exported declaration that nothing in the target and no loaded consumer refers to can go: no caller the analysis can see would notice.

Before:

```go
package store

func Open(path string) (*DB, error) { ... }

// Nothing calls Purge.
func Purge(db *DB) error { ... }
```

After:

```go
package store

func Open(path string) (*DB, error) { ... }
```

How to fix it: Delete the declaration. In a library, check first that no program outside the run imports it, or declare the consumers that do. The finding's fixability is `deletable`, or `manual` where a rule of the analysis gives it.

The rule: An exported symbol with no reference in the target and no reference from any loaded consumer. A symbol referenced only from its own declaration site counts as unreferenced.

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

## DS1002 unused-unexported

An unexported declaration only its own package or file can name, and nothing there names it.

Before:

```ts
const retries = 3;
function backoff(n: number): number { return 2 ** n; }

export function fetchAll(): void { /* backoff is never called */ }
```

After:

```ts
export function fetchAll(): void {}
```

How to fix it: Delete the declaration. The finding's fixability is `deletable`, or `manual` where a rule of the analysis gives it.

The rule: An unexported symbol with no reference in the target. A symbol referenced only from its own declaration site counts as unreferenced. In Go a constant or a type a function body declares, a function literal's body included, that nothing in that body names outside its own specification is reported under this code, under the reference of the enclosing declaration, as `grammar/symbol-ref.md` states for a declaration inside a function body, at the local declaration's own position, with `symbol.kind` `constant` or `type` and the liveness relation `reference-counting`.

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

## DS1003 unused-member

A field or member that nothing reads or writes after its declaration: the type is used, the member is not.

Before:

```go
type Job struct {
	ID    string
	Owner string // never read or written
}
```

After:

```go
type Job struct {
	ID string
}
```

How to fix it: Delete the member and the writes of its initializer. The finding's fixability is `deletable`, or `manual` where a rule of the analysis gives it.

The rule: A struct field, class member or type member with no reference, a private member included. A member referenced only from its own declaration site counts as unreferenced.

Before reporting: A TypeScript member that no value writes and nothing reads, whose type names a type parameter of its declaring interface, class or type alias, is not reported when every part of that declaration that names the type parameter is a member no value writes and nothing reads. TypeScript compares object types by their members, so such members are what make two instantiations of their type distinct types, and deleting them leaves the type parameter phantom and lets one instantiation stand for another. A member a value writes is decided by the rule of DS1301 instead, whatever its type names.

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

## DS1004 unused-import

An import binding nothing in its file reads: the import only names a value the file never uses.

Before:

```ts
import { parse, stringify } from "./codec.ts";

export const read = (text: string) => parse(text);
```

After:

```ts
import { parse } from "./codec.ts";

export const read = (text: string) => parse(text);
```

How to fix it: Delete the binding, or the import declaration when it binds nothing else. The finding's fixability is `deletable`.

The rule: A name a TypeScript import declaration binds, or an `import x = require()` declaration that is not exported, a symbol of its own file beside the declaration it imports, reported at the binding when nothing in the file reads it: a use in a type position, in a local export or in a type, a link or a `@see` tag of a doc comment reads it, and so does JSX the compiler turns into calls of a factory it names. A binding of a declaration file or of a component file is not judged, nor one of a generated file while generated files are excluded.

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

## DS1005 unused-enum-member

A member of an enumerated type that nothing names: the set of values is wider than any use.

Before:

```ts
enum Level {
  Debug,
  Info,
  Trace, // never named
}
```

After:

```ts
enum Level {
  Debug,
  Info,
}
```

How to fix it: Delete the member, or give a later member its present value first where the values are positional. The finding's fixability is `deletable`, or `manual` where a rule of the analysis gives it.

The rule: An enumerated member that no symbol names, at the reachability class derived for the member and with no confidence ceiling below it. Every member of a type that carries a string, text or binary conversion method, or that a conversion from an integer or a wire value produces, is retained by the enum-group exemption first, so a member reached only by value is never a candidate.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `probable` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, while a declared consumer did not load.
- `possible` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, in a run that holds no consumer information.
- `possible` ([`roots.25`](../../contract/grammar/analysis/roots.md)): A declaration outside an internal tree of a Go package of an application that no build links, in a run that declares no consumer.
- `possible` ([`confidence.13`](../../contract/grammar/analysis/confidence.md)): A TypeScript member whose name the left operand of an in expression of a production file spells.
- `possible` ([`confidence.14`](../../contract/grammar/analysis/confidence.md)): A field the encoding-reflection class records only as filled by a callee outside the analyzed program.
- `possible` ([`confidence.19`](../../contract/grammar/analysis/confidence.md)): A member of a type the program only decodes into, where a callee outside the analysis, or an any or unknown value, fills it.
- `possible` ([`confidence.20`](../../contract/grammar/analysis/confidence.md)): A declaration of a TypeScript file only tests reach that is no test-support file, and each dead declaration it reaches through dead declarations.
- `possible` ([`confidence.23`](../../contract/grammar/analysis/confidence.md)): A member that only records of exemption classes whose confidence is possible hold.
- `probable` ([`confidence.25`](../../contract/grammar/analysis/confidence.md)): A TypeScript declaration found dead under reachability alone, or a file only dead code imports, while ts.roots_complete is not true.
- `probable` ([`confidence.26`](../../contract/grammar/analysis/confidence.md)): A finding the roots cap applies to, in a configuration that compiles a load by a computed path.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1006 unused-type-parameter

A type parameter that no parameter, result or body of its declaration uses.

Before:

```go
func Count[T any, K comparable](items []T) int { return len(items) }
```

After:

```go
func Count[T any](items []T) int { return len(items) }
```

How to fix it: Delete the type parameter and the type arguments callers pass for it. The finding's fixability is `deletable`.

The rule: A type parameter of a function, a method or a TypeScript type alias that no part of the declaration's signature, body or aliased type names, at the certain class. The deletion is local to the declaration and the explicit instantiations the reference set already lists. A type parameter of a declaration that is itself dead falls with it and is reported under no code of its own.

Before reporting: Function and method type parameters, and in TypeScript those of a type alias. A TypeScript alias names no type of its own, so two instantiations that differ only in a parameter its type never names are one type. A Go type declaration's type parameter is never reported, because a phantom type parameter such as `type ID[T any] int` makes two instantiations distinct types while naming the parameter nowhere, so deleting it changes the program. Nor is one of a TypeScript class or interface, which merges only with declarations of the same type parameters. A method signature an interface or an object type declares belongs to that type declaration, so its type parameters are never reported either.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `probable` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, while a declared consumer did not load.
- `possible` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, in a run that holds no consumer information.
- `possible` ([`roots.25`](../../contract/grammar/analysis/roots.md)): A declaration outside an internal tree of a Go package of an application that no build links, in a run that declares no consumer.
- `possible` ([`confidence.13`](../../contract/grammar/analysis/confidence.md)): A TypeScript member whose name the left operand of an in expression of a production file spells.
- `possible` ([`confidence.14`](../../contract/grammar/analysis/confidence.md)): A field the encoding-reflection class records only as filled by a callee outside the analyzed program.
- `possible` ([`confidence.19`](../../contract/grammar/analysis/confidence.md)): A member of a type the program only decodes into, where a callee outside the analysis, or an any or unknown value, fills it.
- `possible` ([`confidence.20`](../../contract/grammar/analysis/confidence.md)): A declaration of a TypeScript file only tests reach that is no test-support file, and each dead declaration it reaches through dead declarations.
- `possible` ([`confidence.23`](../../contract/grammar/analysis/confidence.md)): A member that only records of exemption classes whose confidence is possible hold.
- `probable` ([`confidence.25`](../../contract/grammar/analysis/confidence.md)): A TypeScript declaration found dead under reachability alone, or a file only dead code imports, while ts.roots_complete is not true.
- `probable` ([`confidence.26`](../../contract/grammar/analysis/confidence.md)): A finding the roots cap applies to, in a configuration that compiles a load by a computed path.
- A finding about a member of a dead component carries no higher confidence than the component's root members.
