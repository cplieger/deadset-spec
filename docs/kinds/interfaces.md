# Interfaces: `DS1200` to `DS1299`

The interface or its method is never used through the interface. Interfaces, interface methods and satisfaction assertions nothing uses through the interface. Generated from `contract/kinds.json`; the [issue kinds](../kinds.md) page lists every family.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1201` | `unused-interface` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1202` | `uncalled-interface-method` | `go`, `ts` | `on` | `warn` | `manual` |
| `DS1203` | `unused-satisfaction-assertion` | `go` | `on` | `deny` | `deletable` |

## DS1201 unused-interface

An interface no value is ever used through: nothing converts to it, asserts it or calls a method through it.

Before:

```go
type Closer interface{ Close() error } // nothing converts a value to Closer
```

After:

```go
// Closer is deleted.
```

How to fix it: Delete the interface. The finding's fixability is `deletable`, or `manual` where a rule of the analysis gives it.

The rule: An interface no symbol uses as a type, counting uses from every loaded module. The finding names the concrete implementations and their positions, and the interface's members join its dead component rather than being reported as independent unused declarations.

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

## DS1202 uncalled-interface-method

An interface method nothing calls through the interface: implementations carry it for no caller.

Before:

```ts
interface Store {
  get(key: string): string;
  flush(): void; // no caller calls store.flush()
}
```

After:

```ts
interface Store {
  get(key: string): string;
}
```

How to fix it: Delete the method from the interface, then from each implementation that has no other use for it. The finding's fixability is `manual`.

The rule: An interface method that no call site invokes or selects through the interface, whatever the number of implementations. A call or selection in test code is no call for this rule, and an interface method only test code calls is DS1102's. The finding names the concrete implementations and their positions. Each method that implements it and that nothing else keeps is reported under the code its own references select. Such a method counts as referenced by the interface method alone, so it belongs to this finding's dead component, whose root member is the interface method.

Before reporting: In Go, every method of an exported interface whose method set holds an unexported method is exempt, a method of an interface it embeds included. An unexported interface restricts no implementor outside its package, so a method only it holds is judged. That is the sum-type shape, whose method set exists to restrict the implementors, and a TypeScript interface cannot take it because it declares no member less visible than itself. In both languages, every marker method is exempt, an interface method whose every implementation carries an empty body.

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

## DS1203 unused-satisfaction-assertion

A compile-time assertion that a type satisfies an interface, where nothing else needs the type to satisfy it.

Before:

```go
var _ io.Closer = (*conn)(nil) // nothing uses conn as an io.Closer
```

After:

```go
// The assertion is deleted.
```

How to fix it: Delete the assertion, or keep the interface use it was guarding. The finding's fixability is `deletable`.

The rule: A compile-time satisfaction assertion whose interface no symbol uses as a type.

Before reporting: In a library, an assertion whose interface and whose concrete type are both published API, each an exported declaration of a package outside programs can import, is not reported: it states to every importer a promise of the published API. An assertion whose interface alone is published API has the reachability class possible, whatever consumers the run declares and loads. The assertion checks a promise the published API makes, that the type satisfies the published interface, and that promise is made to every program that imports the package, not only to the consumers the run declares. A loaded consumer that names the interface as a type uses it, and the assertion is not reported.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `probable` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, while a declared consumer did not load.
- `possible` ([`confidence.2`](../../contract/grammar/analysis/confidence.md)): A subject of a library's published API, in a run that holds no consumer information.
- `possible` ([`roots.25`](../../contract/grammar/analysis/roots.md)): A declaration outside an internal tree of a Go package of an application that no build links, in a run that declares no consumer.
- `possible` ([`confidence.14`](../../contract/grammar/analysis/confidence.md)): A field the encoding-reflection class records only as filled by a callee outside the analyzed program.
- `possible` ([`confidence.19`](../../contract/grammar/analysis/confidence.md)): A member of a type the program only decodes into, where a callee outside the analysis, or an any or unknown value, fills it.
- `possible` ([`confidence.23`](../../contract/grammar/analysis/confidence.md)): A member that only records of exemption classes whose confidence is possible hold.
- A finding about a member of a dead component carries no higher confidence than the component's root members.
