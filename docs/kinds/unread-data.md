# Data nobody reads: `DS1300` to `DS1399`

It is written, but nothing reads what it stores. State the program writes and never reads. Generated from `contract/kinds.json`; the [issue kinds](../kinds.md) page lists every family.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1301` | `write-only-symbol` | `go`, `ts` | `on` | `deny` | `deletable` |

## DS1301 write-only-symbol

A declaration the program writes and never reads: every write stores a value nobody looks at.

Before:

```go
type Stats struct{ hits int }

func (s *Stats) Hit() { s.hits++ } // hits is never read
```

After:

```go
type Stats struct{}

func (s *Stats) Hit() {}
```

How to fix it: Delete the declaration and every write to it. The finding's fixability is `deletable`.

The rule: A package-level or module-level variable, struct field, class member, type member or collection that production code writes and never reads. In production mode a read from a test file counts as no read. A write from a loaded consumer's code counts as a read, because the deletion the finding names would break a consumer the target does not own. A write in a consumer's test file counts as a read in the run that counts test references. It counts in a production sweep only where the configuration counts consumer tests as production. In Go a read counts only from a declaration that is live under the finding's liveness relation, which is reference counting: a read from a declaration nothing references, whose own finding has the reachability class `certain`, is no read, so a declaration that only such declarations read is reported where a declaration something references writes it. A comparison of struct values reads every field of the type: in Go, a value of a struct type used as an operand of `==` or `!=`, as a map key, or as a `switch` tag or `case` expression is a read of every field of that type, transitively through the fields of every struct type it holds, recorded at the comparison, because equality reads every field and a field that decides equality carries information. In Go a type argument the program instantiates a type parameter with, where the parameter's constraint is `comparable` or embeds it, is compared as an operand of `==` of that type is, recorded at the instantiation, so the struct values `slices.Equal`, `slices.Index` or `maps.Equal` compares have every field read. `reflect.DeepEqual` is not a comparison under this rule. In Go, a conversion the compiler accepts between two struct types, or between two pointer types that are not named types and whose base types are struct types, reads every field of the two struct types where their struct type literals are distinct, recorded at the conversion, because the conversion is legal only while their fields are identical, ignoring tags, and so no field of either can be deleted alone. It reads in the same way every field of each pair of distinct struct type literals the identity of the two types compares in turn: through each field's type, and from there through the element, key, base, parameter and result types of the type literals it reaches, through the method signatures of an interface type literal, and through the type arguments of an instantiated generic type. A field reached there has no reference of its own, as grammar/symbol-ref.md states, so it is never a subject and those reads hold no finding back. It stops at a defined type, whose fields the conversion does not read, because a defined type is identical to itself whatever fields it declares. A pair whose two sides are one struct type literal, the two struct types themselves included, as where one type is declared from the other or the conversion is to the operand's own type, reads nothing, because a field deleted from that literal leaves both sides identical. Where the operand's type or the conversion's type is a type parameter, the struct type its constraint's core type is takes that side's place, and a type parameter operand reads in the same way the fields of each struct type argument an instantiation the program holds gives it, an instantiation made through another generic declaration's own type parameters included. A conversion to any other type, an interface type among them, reads no field under this rule. In TypeScript a comparison of two object references reads no member, because `===` compares identity and never a member. A TypeScript parameter property, a constructor parameter that carries a modifier, declares a member of its class that the constructor writes with its argument, so the member is judged under this rule, and under every rule that reads a class member, as a member the class body declares is. In TypeScript a property of an object literal writes the member of the same name that the literal's contextual type declares, whether or not the compiler checks the literal for excess properties. Where that contextual type is a union or an intersection, the property writes the member of that name on each constituent that declares one. So a literal that a declaration annotates, a function returns, a call passes or a `satisfies` expression checks writes the members of the type it is given. A literal whose type the compiler infers, as one a callback returns to a generic call, writes in the same way the member of each property's name that each object type a value of the literal's type reaches declares, through array elements, type arguments and the parameters and results of functions, so `ids.map((id) => ({ id }))` returned as `Row[]` writes `Row.id`. An object literal handed to a callee the target does not implement has its members read, as grammar/analysis/references-ts.md states. A store through the value a field or a member holds reads that field or member rather than writing it, because the store lands in storage every other holder of the value reads. In Go that store is an assignment, an increment, a compound assignment or a `delete` whose target is reached through a struct field holding a map, a slice or a pointer. Examples are `s.f[k] = v`, `s.f.x = v` and `delete(s.f, k)`. The same store through a field holding an array or a struct is a store into the field itself. TypeScript holds every object by reference, so an element or property store through a member's value reads the member. The finding names each write position, so the deletion set is visible. Deleting the subject and its writes also deletes every declaration whose only uses they hold. Such a declaration is a type parameter of the subject's declaring type that the subject's type alone names, an import binding the writes alone use, or a declaration whose every use is in the type annotation of the subject or of another subject this rule reports, or in the writes those findings name. The finding's message names each such declaration, and no finding is reported about the declaration itself.

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
