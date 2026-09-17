# Exemption classes

An exemption is a named class of reason for which an analyzer keeps a symbol that the reference
graph alone would report. Each class has a detection rule the analyzer computes from its language's
type information and program graph, never from a text search except where the class says so. The
analyzer records the class name in the retained symbol's `retained_by` field, lists the symbols it
held back and their classes on request, and lets a maintainer switch a class off by name so an
exemption suspected of hiding a defect can be tested (from `contract/exemptions.json`, and
`contract/config.schema.json`, `exemptions.disabled`). A symbol an exemption retains is not
reported, so a finding carries `retained_by` present and empty (from
`contract/finding.schema.json`, `retained_by`).

The vocabulary is closed: a conformance expectation naming a class this file does not declare is a
defect in the expectation.

This page states every class, one section each. The fields:

- **Languages** are the languages the class runs on, `go` for Go and `ts` for TypeScript and
  JavaScript.
- **Confidence** uses the report's reachability vocabulary: `certain` for a class derived from a
  type relation, `possible` for a class derived from a text match (from `contract/kinds.json`,
  `reachability_classes`).
- **Names the site** is `names_site`: whether the recorded exemption names the source site that
  produced it.
- **Rule** is the detection rule stated for an implementer in any language, and **mechanism**
  restates it in each listed language's own terms.
- **Retains** names what the class keeps.
- **TypeScript visibility** is `typescript_visibility`, present on every class that runs on
  TypeScript. It answers two questions: whether the class can retain a class member declared with
  the `private` modifier, and whether it can retain one declared with a `#private` name. The two
  differ at run time. `private` is a compile-time constraint only, so a `private` member stays
  reachable by its name through a string index, a decorator, a container or a serializer, while a
  `#private` name cannot be named from outside its class body at all. Every class that reaches
  members by name therefore applies to `private` and not to `#private`, and a `#private` member
  with no reference in its own class body is reported whatever else the program does.

| Class | Languages | Confidence | Names the site |
| --- | --- | --- | --- |
| `interface-satisfaction` | `go`, `ts` | `certain` | no |
| `encoding-reflection` | `go` | `certain` | no |
| `format-verb-contract` | `go` | `certain` | no |
| `errors-duck-typing` | `go` | `certain` | no |
| `enum-group` | `go`, `ts` | `certain` | no |
| `generated-file` | `go` | `certain` | no |
| `linkname-cgo-asm-plugin` | `go` | `certain` | no |
| `template-field` | `go`, `ts` | `possible` | yes |
| `reflective-lookup` | `go`, `ts` | `possible` | yes |
| `decorator` | `ts` | `certain` | no |
| `injection-container` | `ts` | `certain` | no |
| `framework-lifecycle` | `ts` | `certain` | no |
| `serialization-contract` | `ts` | `certain` | no |

## The classes

### interface-satisfaction

A method is retained when a value of its receiver's type reaches a position typed as an interface the method helps satisfy. The analyzer records every site where a value of type T is converted or assigned to an interface type I (an explicit conversion or a satisfaction assertion, an assignment, an argument, a return value, an element stored in an I-typed container) and, for each recorded (T, I) pair, retains the methods of T that satisfy I. The assertion is not a use of I beyond itself: when nothing else uses I as a type, I is still reported as an unused satisfaction assertion while T's methods stay retained.

Retains: The methods of T that satisfy I, for each recorded (T, I) pair.

Mechanism in Go: Build the conversion set from the type-checked program: an assertion of the form `var _ I = (*T)(nil)` or `var _ I = T{}`, a value passed to a parameter of type I, a value returned into a result of type I, a value assigned to a variable or field of type I, and an element stored into an I-typed slice, map, channel or struct field. For each (T, I) pair, `types.Implements(T, I)` decides satisfaction and the method set of T that I requires is retained. A type registered with `flag.Var`, or used as an `io.Writer` or an `http.RoundTripper`, is an ordinary member of the conversion set, so those methods are retained here and by no other class.

Mechanism in TypeScript: Build the same conversion set over class instance types flowing into interface-typed positions: an `implements` clause, an assignment, an argument, a return value, an element stored in an interface-typed container. For each (class, interface) pair, the checker's assignability test decides satisfaction and the members the interface requires are retained.

TypeScript visibility: retains a member declared `private`, no; retains a member declared with a `#private` name, no.

From `contract/exemptions.json`, class `interface-satisfaction`.

### encoding-reflection

A type whose values reach a consumer that inspects them by name at runtime, namely reflection, a standard encoder or decoder, a template engine, a database scan target, a sort interface or a log-value interface, has its exported methods and its tagged fields retained, because that consumer names them by string and the reference graph holds no edge to them.

Retains: The exported methods and the struct fields carrying a tag, on every type that flows into such a consumer.

Mechanism in Go: A type T flows into the class when a value of T, or a pointer to one, is an argument of a function or method of `reflect`, `encoding/json`, `encoding/xml`, `encoding/gob`, `text/template` or `html/template`, is a `database/sql` `Scan` target, or is converted to `sort.Interface` or `slog.LogValuer`. Retain T's exported methods and every field of T that carries a struct tag.

From `contract/exemptions.json`, class `encoding-reflection`.

### format-verb-contract

A type whose values reach a formatting verb that calls its string or error method has that method retained: the formatting package invokes `String` or `Error` through an interface at runtime, so the method has no static reference.

Retains: The `String() string` and `Error() string` methods of the formatted type.

Mechanism in Go: A value of type T is an operand of a `fmt` print, format or error-construction function (`Print`, `Sprintf`, `Errorf` and their siblings) under a verb valid for a string operand (`%v`, `%s`, `%q`, `%x`, `%X`, and the verb-less `Print` and `Println` forms); retain T's `String` and `Error` methods. A method that satisfies `io.Writer` or `http.RoundTripper` is retained by `interface-satisfaction`, not by this class.

From `contract/exemptions.json`, class `format-verb-contract`.

### errors-duck-typing

The standard error helpers reach comparison and unwrapping methods by duck typing rather than through a declared interface, so those methods have no static reference and are retained on any type the program uses as an error.

Retains: Methods with signature `Is(error) bool`, `As(any) bool`, `Unwrap() error` or `Unwrap() []error` on a type reachable as an error.

Mechanism in Go: A type is reachable as an error when a value of it enters the conversion set with `error` as the interface. On such a type retain every method whose name and signature match one of the four forms `errors.Is`, `errors.As` and `errors.Unwrap` call.

From `contract/exemptions.json`, class `errors-duck-typing`.

### enum-group

A member of an enumerated type whose values can arrive by conversion rather than by name is never dead in isolation. When the type carries a string, text or binary conversion method, or a value of the type is produced from an integer or from a decoded wire value, every member of that type is retained, so that a member reached only by value is never reported.

Retains: Every member of the enumerated type.

Mechanism in Go: The enumerated type is a defined type whose constants are declared in an `iota` group. The class fires when the type has a `String`, `MarshalText`, `UnmarshalText`, `MarshalJSON` or `UnmarshalJSON` method, or when a value of the type is produced by a conversion from an integer type or from a decoded value (an `encoding/json` or `encoding/xml` target, a `database/sql` scan target, a `strconv` result). Retain every constant of the type.

Mechanism in TypeScript: The enumerated type is an `enum` declaration. The class fires when a value of the enum is produced by a type assertion from `number` or `string`, by the reverse mapping `E[n]`, or by a decoded value typed as `E`. Retain every member of the enum.

TypeScript visibility: retains a member declared `private`, no; retains a member declared with a `#private` name, no.

From `contract/exemptions.json`, class `enum-group`.

### generated-file

Every declaration in a generated file is retained by default, because the generator owns the file and a deletion there is undone at the next generation. When a project configures generated files as included, the analyzer reports findings in them instead and marks each such finding as one no mechanical edit may act on.

Retains: Every declaration in the file.

Mechanism in Go: A file is generated when it carries the standard Go generated-code header: a line matching `^// Code generated .* DO NOT EDIT\.$` that appears before the first non-comment, non-blank text of the file, which is the rule `go/ast.IsGenerated` implements. When generated files are configured as included, each finding in one carries `generated: true` and `fixability: none`.

From `contract/exemptions.json`, class `generated-file`.

### linkname-cgo-asm-plugin

A symbol that another compilation unit or the runtime reaches by name outside the type checker's view is retained: a linker-level alias, a symbol exported to C, a symbol an assembly file names, or a symbol looked up through the plugin package.

Retains: The named symbol.

Mechanism in Go: Retain a function or variable named on either side of a `//go:linkname` directive in any loaded package, a function carrying a `//export` directive in a cgo file, a symbol named by a `TEXT ·name` directive in an assembly file of the same package, and an exported symbol of a plugin's main package whose name is the string argument of a `plugin` `Lookup` call.

From `contract/exemptions.json`, class `linkname-cgo-asm-plugin`.

### template-field

Where the project configures template directories, a member whose name appears in a template as a field or method reference is retained at the lowest confidence, and the exemption names the template site that matched. The evidence is a text match, not a type relation, so this is a weak class and it applies only where the project asked for it.

Retains: The field or method whose name the template references, on any type.

Mechanism in Go: Scan every file under the configured template directories for `{{ .Name }}`, `{{ .Name arg }}` and `{{ $x.Name }}` action references, the `text/template` and `html/template` field and method syntax; retain every exported field and method named `Name`, recording the template file and line.

Mechanism in TypeScript: Scan every file under the configured template directories for the member's name in an interpolation or binding position; retain every member so named, recording the template file and line.

TypeScript visibility: retains a member declared `private`, yes; retains a member declared with a `#private` name, no.

From `contract/exemptions.json`, class `template-field`.

### reflective-lookup

A symbol whose name appears as a string literal that reaches a reflective lookup call is retained at the lowest confidence, and the exemption names the lookup site. The evidence is a string match beside a call, not a type relation, so this is a weak class.

Retains: The method, field or member whose name matches the string literal.

Mechanism in Go: A string literal equal to a method or field name is, or is a constant that flows into, an argument of `MethodByName` or `FieldByName` on a `reflect.Value` or `reflect.Type`, or of a `Lookup` method that resolves a name at runtime; retain the matching method or field, recording the call site.

Mechanism in TypeScript: A string literal equal to a member name is the index of an element access (`obj["name"]`) or an argument of a `Reflect` call (`Reflect.get`, `Reflect.set`, `Reflect.has`); retain the matching member, recording the site.

TypeScript visibility: retains a member declared `private`, yes; retains a member declared with a `#private` name, no.

From `contract/exemptions.json`, class `reflective-lookup`.

### decorator

A member that carries a decorator, or whose class carries a decorator, is retained: the decorator receives the member or the class at runtime and may reach the member by name, so the decorator's own symbol counts as a reference to what it decorates.

Retains: The decorated member, and every member of a decorated class that a decorator can name at runtime.

Mechanism in TypeScript: A member is retained when a decorator expression is attached to it, or when a decorator expression is attached to its class. The retention covers `private` members, which a decorator can reach by name, and never `#private` ones.

TypeScript visibility: retains a member declared `private`, yes; retains a member declared with a `#private` name, no.

From `contract/exemptions.json`, class `decorator`.

### injection-container

Where the project configures a dependency-injection container, a class registered with it has its constructor and the members the container injects retained, because the container instantiates the class and populates those members with no reference in the program's own code.

Retains: The class's constructor and every injected member.

Mechanism in TypeScript: The class is passed to a registration call of a configured container, or a constructor parameter or a property carries an injection decorator; retain the constructor and each parameter or property the container injects.

TypeScript visibility: retains a member declared `private`, yes; retains a member declared with a `#private` name, no.

From `contract/exemptions.json`, class `injection-container`.

### framework-lifecycle

Where the project declares a framework, a member whose name matches that framework's lifecycle contract, on a class reachable as that framework's component, is retained, because the framework calls it by name.

Retains: The lifecycle member.

Mechanism in TypeScript: The project declares a framework and the analyzer holds that framework's lifecycle member names as configuration; a member whose name is in that list is retained when its class is reachable as a component of that framework, by registration, by a decorator or by export as one.

TypeScript visibility: retains a member declared `private`, yes; retains a member declared with a `#private` name, no.

From `contract/exemptions.json`, class `framework-lifecycle`.

### serialization-contract

A class whose instances flow into a serializer or a schema validator has its data members retained, because the serializer reads them by name at runtime; its methods are not retained, because a serializer never calls them.

Retains: The class's properties, never its methods.

Mechanism in TypeScript: An instance of the class is an argument of `JSON.stringify`, of a configured serializer, or of a schema validator's parse or validate call; retain the class's properties and never its methods or accessors.

TypeScript visibility: retains a member declared `private`, yes; retains a member declared with a `#private` name, no.

From `contract/exemptions.json`, class `serialization-contract`.
