# Inside functions: `DS1400` to `DS1499`

A part of a function does nothing. Parameters, receivers, results, stores, statements and cases inside a function that change nothing. Generated from `contract/kinds.json`; the [issue kinds](../kinds.md) page lists every family.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1401` | `unused-parameter` | `go`, `ts` | `on` | `warn` | `manual` |
| `DS1402` | `unused-receiver` | `go` | `on` | `warn` | `deletable` |
| `DS1403` | `unused-result` | `go`, `ts` | `on` | `warn` | `manual` |
| `DS1404` | `dead-store` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1405` | `unreachable-statement` | `go`, `ts` | `on` | `deny` | `deletable` |
| `DS1406` | `unreachable-case` | `go`, `ts` | `on` | `deny` | `deletable` |

## DS1401 unused-parameter

A parameter the function body never reads, where the function's signature is free to change.

Before:

```go
func render(w io.Writer, theme string) { fmt.Fprint(w, "ok") }
```

After:

```go
func render(w io.Writer) { fmt.Fprint(w, "ok") }
```

How to fix it: Delete the parameter and the argument every caller passes, or rename it to _ where the signature is fixed by a caller you keep. The finding's fixability is `manual`.

The rule: A parameter with no reference inside its function body, on a function whose signature is free to change.

Before reporting: The signature must be free, defined as follows: the function is not a method retained by interface satisfaction, is not a method whose name and signature are those of a method of an exported interface type declared by a package outside the target that the import closure holds, is not used as a value, is not a go:linkname or cgo target, is not a go-fuzz entry point, is not a function the Go test driver runs (a test, benchmark or fuzz test of a test file, or TestMain), and is not a stub whose body is empty or only panics. A function literal or function expression that initializes a variable the program only calls, never passing, storing, returning or assigning it, is called by name and is not used as a value. In Go a function literal that is the callee of the call it is written in, the call of a `go` or a `defer` statement included, is not used as a value, wherever the literal is written. A parameter its body never names is dead whatever the callers, so a published declaration of a library is reported too, with the fixability the vocabulary gives the kind: the signature change is a breaking change. In TypeScript a caller may pass a function used as a value more arguments than it declares parameters, so in such a function whose signature is free but for that use, a parameter its body never reads is reported for deletion when its body reads no parameter after it, and an unread parameter before the last read one is reported for a rename to its name with a leading underscore, which keeps every argument in place and which the compilers' unused-parameter rules read as declared unread, unless its name already starts with one. A parameter of a TypeScript function expression or arrow function that the compiler gives a contextual type, and that carries a type annotation, is not reported for deletion where the inference of the call holding the function may read the annotation, being where the callee is generic and its parameter type at the function's place names a type parameter at the parameter's place, or where the function is written inside an object literal such a call is passed, and it is reported for deletion at `probable` where it carries an annotation no such inference reads, because nothing in the program shows that an outside reader ignores it. A name an object binding pattern of a parameter of such a function binds, annotated or not, that the body never reads is reported for deletion at `probable`, because the pattern reads the property it names from the argument and the code that hands the argument may act on that read, which nothing in the program shows. Any other TypeScript parameter that is a binding pattern is judged by the names it binds: each name the body never reads is reported wherever the pattern stands, because removing it from the pattern moves no argument, except a name beside a rest element of the same object pattern, whose removal changes what that element holds, and the pattern counts as a read parameter when the body reads any name it binds. A rest parameter is judged as the last parameter. A TypeScript `this` parameter declares the type of `this` and receives no argument, so it is never reported. A TypeScript constructor is used as a value where its class is used as a value other than as the callee of a `new` expression or the class an `extends` clause names, and its signature is not free where its class carries a decorator or a parameter of the constructor carries one, because the code that decorator or a container runs may construct the class. A parameter property is read by the write to its member the constructor makes, so it is never reported under this kind, and its member is judged under DS1301. A TypeScript name a parameter binds, whole or through a binding pattern, that starts with an underscore is never reported, because a leading underscore is the spelling unused-parameter rules read as declared unread, and no other name exempts a parameter. A Go parameter named by the blank identifier declares no name a body could read and is never reported.

Other tools that report it:

- `go`: revive unused-parameter, gopls unusedparams, unparam
- `ts`: tsc --noUnusedParameters, @typescript-eslint/no-unused-vars

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1402 unused-receiver

A method receiver the body never uses.

Before:

```go
func (c *Client) version() string { return "1.0" }
```

After:

```go
func (*Client) version() string { return "1.0" }
```

How to fix it: Drop the receiver's name, or make the method a function. The finding's fixability is `deletable`.

The rule: A named method receiver with no reference inside the method body. Go permits a method with no receiver name, so the fix deletes an identifier and changes no signature.

Before reporting: None beyond the rule. Deleting a receiver name changes no signature, so a method retained by interface satisfaction, a method used as a value and a stub are reported as any other method is.

Other tools that report it:

- `go`: revive unused-receiver
- `ts`: not applicable

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1403 unused-result

A result every caller ignores, or one the function always returns with the same meaning nobody reads.

Before:

```ts
function save(doc: Doc): boolean { write(doc); return true; } // no caller reads the boolean
```

After:

```ts
function save(doc: Doc): void { write(doc); }
```

How to fix it: Delete the result and the value each return statement gives it. The finding's fixability is `manual`.

The rule: A result value that no call site of the function uses, on a function whose signature is free to change.

Before reporting: The same free-signature rule as unused-parameter, plus a closed-world condition: every call site of the function must be in the loaded graph, because an unknown caller may consume the result, so the kind reports only for a function whose callers are all visible. A function a program run by naming the file names, a file under the `ignore` tag or a script of a dropped module, has call sites outside the loaded graph. A call that is the operand of an await expression written as a statement of its own uses a result whose type has a member named then, a promise or another thenable, because the caller sequences on it, and discards any other result. A call written as a statement of its own, or as the operand of void, discards its result whatever its type.

Other tools that report it:

- `go`: unparam
- `ts`: none known

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1404 dead-store

A store to a local variable that no later read sees: the value is overwritten or the variable goes out of scope first.

Before:

```go
n := len(items)
n = 0 // the first store is never read
return n
```

After:

```go
n := 0
return n
```

How to fix it: Delete the store, or the variable when every store to it is dead. The finding's fixability is `deletable`.

The rule: A write to a local variable with no read before the next write to it or the end of its scope, computed on the same read-and-write classification the write-only-symbol kind uses. The finding names the write position.

Before reporting: In TypeScript the binding of a catch clause is a local variable the clause writes when it catches, so a binding that is a name and that the clause's block never reads is a dead store, positioned at the binding: the clause written with no binding behaves the same. A TypeScript local is a name a `let` or `const` declaration of a block binds, or a `var` declaration of a block of a function, a name an object or array binding pattern of one binds at any depth included, except a name with a default, whose removal skips the default's evaluation, and a name beside a rest element of its object pattern, whose removal changes what the rest holds. A function declaration of a block of a function is a TypeScript local too, written with its function when the block starts, wherever it stands in the block, so one that nothing outside its own body reads is a dead store positioned at its name, and a read anywhere else in the block reads it. A `let` or `var` declaration with no initializer writes `undefined`, so one that nothing reads and nothing writes is a dead store positioned at its name. A local whose declaration carries the type annotation `never` is no subject: a write to it is the compiler's exhaustiveness check, which fails to compile once a case goes unhandled, so deleting it changes what the program checks. A `var` of a block nested in its function's body is judged in that block, its last write kept because the function scope reads on past the block's end, and is no subject where a nested function of its function names it. A local a nested function reads is no subject, because the function may read it at any time, and every write to a local nothing reads is dead, one a nested function makes included, each positioned at the name it writes. The variable a `let` or `const` declaration of a `for…of` or `for…in` statement binds is written at each iteration, so one the loop body never reads is a dead store positioned at the binding, except a name that starts with an underscore, because a loop must bind a name and that spelling declares it unread. In Go a function literal a package-level variable's initializer holds is a function body of its own, and a finding about one of its stores carries the reference of the variable the literal initializes, which for a blank variable is its package's, as `grammar/symbol-ref.md` states for a blank-identifier declaration. In Go a range statement stores its key and its value at the start of every iteration, and an empty range stores neither. In Go a function literal reads each variable it names where it is written, and a store that a literal naming its variable may precede on some path of the control-flow graph, the literal of an earlier iteration of a loop included, is not dead, because a call of the literal the graph does not order may read it. A store no such literal precedes is judged as any other. A literal that is the callee of the call it is written in, outside a `go` or `defer` statement, runs there, so it reads its variables at that call and precedes no later store.

Other tools that report it:

- `go`: ineffassign, wastedassign, staticcheck SA4006
- `ts`: eslint no-useless-assignment, @typescript-eslint/no-unused-vars

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1405 unreachable-statement

A statement no execution reaches, after a return, a panic or an unconditional jump.

Before:

```ts
function stop(): never {
  throw new Error("stopped");
  cleanup(); // never runs
}
```

After:

```ts
function stop(): never {
  throw new Error("stopped");
}
```

How to fix it: Delete the statement. The finding's fixability is `deletable`.

The rule: A statement that control flow cannot reach, at the precision the language's compiler applies to the same construct.

Other tools that report it:

- `go`: go vet unreachable
- `ts`: tsc allowUnreachableCode, eslint no-unreachable

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1406 unreachable-case

A case of a switch no value can select, because an earlier case already takes every value it names.

Before:

```go
switch {
case n > 0:
	return "positive"
case n > 10: // n > 0 already took it
	return "large"
}
```

After:

```go
switch {
case n > 0:
	return "positive"
}
```

How to fix it: Delete the case. The finding's fixability is `deletable`.

The rule: A switch case whose type or value an earlier case in the same switch already covers.

Other tools that report it:

- `go`: staticcheck SA4020
- `ts`: none known

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.
