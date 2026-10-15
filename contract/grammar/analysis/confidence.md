# Confidence

Rules `confidence.1` to `confidence.34` of [the analysis](../analysis.md). A rule is cited by its id, which is never reused for another rule.

## The reachability class

**confidence.1** A finding's reachability class states what the analysis can see of its subject's users.

**confidence.2** A subject of a library's published API, a published workspace member's included, is `certain` when every declared consumer loads, `probable` when a declared consumer did not load, and `possible` when the run holds no consumer information.

**confidence.3** So is each public or protected member of a type the published API reaches, because an outside program holding a value of that type can name the member: the published API reaches the type of each published declaration, of each of its parameters, results and properties and of each constraint of its type parameters, and in turn the type of each member, base type and type argument of a type it reaches, a getter and a setter of one name being one declaration whose types are both reached, the getter's result and the setter's parameter.

**confidence.4** In TypeScript it reaches as well the type of each value the body of a published function or method returns, through the type assertions around the value, and each property of that type and of its union or intersection constituents, because an outside program receives the value whatever type the signature names. Where such a value is an object literal, written in the return or as the initializer of a variable the body declares, the body of each of its methods and of each function its properties are written as is read the same way, through four such literals at most, because the outside program calls them.

**confidence.5** A published value whose type is a module's namespace reaches each export of that module, and not the module's file.

**confidence.6** A library's internals, which outside programs cannot name, and every other subject of an application are `certain`, except a declaration of a Go package no build links, as Roots states.

## Evidence that lowers the class

**confidence.7** Each row of [`confidence.json`](../../confidence.json) is one kind of evidence that lowers a finding's reachability class: the class it lowers the finding to, the codes and the languages it applies to, and the rule below that states when it holds. A finding takes the lowest class any row that holds for it gives, and no row raises a class.

### A string literal that spells the name

**confidence.8** A package-level Go declaration whose identifier a string literal spells exactly, a literal whose value equals the identifier, written outside the declaration itself in a production Go file of the target that the run builds, has the reachability class `possible` under `DS1001`, `DS1002`, `DS1102`, `DS1104`, `DS1501`, `DS1502` and `DS1101`.

**confidence.9** A string literal here is one of an expression, a constant's value included; the path of an import declaration and the tag of a struct field are not counted. Outside the declaration itself means outside the source range of its own declaration node, so a literal in the body of a method declared on a type counts for the type, because the method is a declaration of its own. The own declaration node of a name a grouped `const`, `var` or `type` declaration declares is the specification that declares it, so a literal in one specification counts for a sibling the group declares.

**confidence.10** The reason is that a tool reading the source, a code generator among them, may find the declaration by that name, and only a text search can tell whether one does. The finding's `details.name_literal` is the position of the first such literal in the order positions sort.

**confidence.11** A struct field, a method and an interface method are not capped this way, and a literal in a test file caps nothing, as an exemption's evidence in a test file holds nothing in a production sweep.

**confidence.12** A package-level constant whose type is a defined type of the target that such a literal spells has the same class under the same codes, its `details.name_literal` the literal that spells the type, because a tool that finds a type by its name reads that type's constants.

### A name an `in` expression tests

**confidence.13** A TypeScript member of a class, an interface or a type literal whose name equals a string literal, or a template literal with no substitution, written as the left operand of an `in` expression in a production file of the target, has the reachability class `possible` under every code. A value narrowed by such an expression is read back by that name, often through `unknown`, where no type links the read to the member, so only a text match can tell which member it reads. Such a finding carries no `details.name_literal`.

### Members of sent and received types

**confidence.14** A finding about a field that the `encoding-reflection` class records only as filled by a callee outside the analyzed program, one whose name says it fills its argument, one handed a pointer to the value, or a generic function that makes the value, has the reachability class `possible`, as [`exemptions.json`](../../exemptions.json) states.

**confidence.15** One rule holds in both languages for a data member of a type the program sends or receives.

**confidence.16** A member of a type whose values the program sends or returns to code outside the analysis, through an encoder, a serializer or an outside callee, is live where production code writes it, because the receiver reads what the value carries, and a member nobody writes or reads is dead though the type is sent: a write is an assignment, a field or property a composite or object literal writes, a constructor's parameter property, a class property's initializer, and in TypeScript a member an object literal's spread or an `Object.assign` call copies from a value whose type has a member of that name.

**confidence.17** A decode production code makes writes each member it fills, so a type the program decodes and sends again keeps the members it relays. One is a standard decoder filling the value. One is a callee outside the analysis filling it: a Go function or method handed a pointer to the value at an empty-interface parameter, whatever its name, or a generic function whose type parameter its results name and none of its parameters does. In TypeScript, one is a value a `JSON.parse` call returns, or one any other call or a property types as `any` or `unknown`, read as the type: through an `as` or angle-bracket assertion around it, or by the position it is written in, the annotation of the variable it initializes, the declared result of the function it is returned from, the target of an assignment or the parameter of a call, or that of the call a callback returning it is passed to. A `satisfies` check types no value, so a value it checks is read as no type by it.

**confidence.18** A member of a type the program only decodes into, a standard decoder filling it or a `JSON.parse` result read as the type, that production code never reads is dead at the class `certain`, because the decoder reads none of it.

**confidence.19** Where the value comes from a function or method outside the analysis, a Go decoding callee or a TypeScript generic declared outside the analysis or only ambiently whose type parameter its result names and none of its parameters does, such as `get<T>()`, or from a TypeScript call or property other than `JSON.parse` that types it `any` or `unknown`, the finding has the class `possible`, because that callee fills the member and may read it where the analysis cannot see.

### Production code only tests use

**confidence.20** A declaration of a TypeScript file only tests reach that is no test-support file, production code only tests use as Test code states, has the reachability class `possible` where the run that counts test references reaches it, and so does each dead declaration such a declaration reaches through dead declarations, because each is dead only while code that tests use is production code: every finding about each of them carries `possible`, a test of dead code every target of which is such a declaration carries the confidence `possible`, a `DS1602` finding about such a file has the class `possible` too, and a declaration of that file nothing references keeps `certain`. A test reaches what its references name: a star re-export, a namespace import or a module read the test's references pass through names the module, not each declaration it exports, so a declaration of such a file that no reference names keeps its class.

**confidence.21** A declaration of a Go package only tests reach that is no test-support package, production code only tests use as Test code states, that test code references has the reachability class `possible`, and so does each dead declaration such a declaration reaches through references or holds as its container, because each is dead only while code that tests use is production code: every finding about each of them carries `possible`, a test of dead code every target of which is such a declaration carries the confidence `possible`, each dead declaration such a test references or holds as its container, a declaration of a test file among them, carries it too, and so does each dead declaration one of those reaches the same way, and a declaration of that package nothing references keeps `certain`. A `DS1301` finding about a declaration of such a package that only test files read has the class `possible` too, because only tests use what its writes store.

**confidence.22** In both languages that `possible` is the finding's own: the component cap below does not carry it to another member, and a root finding it alone puts below the minimum withholds no other finding. Every dead declaration such a declaration references is such a declaration too, so a reported finding is never about a declaration a withheld one needs. A package a production file imports under one configuration of the run is no such package under any.

### A member only a `possible` class holds

**confidence.23** A member that only records of exemption classes whose `confidence` in [`exemptions.json`](../../exemptions.json) is `possible` hold is reported, at the reachability class `possible`, rather than retained: such a class reads the member's use from a text match, which can name a member nothing uses. A record of a `certain` class retains the member whatever records of `possible` classes also name it. The minimum's default withholds such a finding, so the default view shows the member neither reported nor held.

## Caps

**confidence.24** A finding's confidence is its reachability class capped by its kind's `max_class`.

### Roots the analysis cannot see

**confidence.25** A TypeScript finding about a declaration found dead under reachability alone, one that only declarations themselves dead reference, and a `DS1602` finding about a file only dead code imports, is capped as well at `probable` unless `ts.roots_complete` is `true`: a root the analysis does not see, a framework's convention or a computed load, would make the subject live, and only the maintainer can declare that none exists.

**confidence.26** A TypeScript `import()` or `require()` call in a production file whose argument is no string literal, and whose path starts with no relative literal ending in `/` that names a directory below the calling file's, and with no literal naming a package, loads a file the analysis cannot resolve, so in each configuration that compiles that file `ts.roots_complete` lifts no cap: a finding this cap applies to whose subject such a configuration holds is `probable` at most, and the run states a `computed-load` note per call. A `require` the target declares loads nothing, as Computed loads states.

**confidence.27** A finding about a declaration nothing references and an intra-function finding are not capped by it.

### The component cap

**confidence.28** A finding about a member of a dead component, a root member included, is capped as well by the lowest confidence among the findings about the component's root members, a root's name-literal cap included and the `possible` of production code only tests use excluded, so every finding of one component carries one confidence but those that `possible` lowers, and the minimum withholds or reports the component whole but for them. A test of dead code whose component holds a published declaration therefore carries no higher confidence than that declaration, whatever reachability class the test itself has.

**confidence.29** A component the merge joins across reports takes the component cap over the joined component, once, so every finding of the joined component carries the one confidence the cap gives, but those the `possible` of production code only tests use lowers.

## How the steps combine

**confidence.30** A finding's confidence is computed in one order, each step only lowering it: its reachability class with every evidence row that holds for it, the cap of its kind's `max_class`, the cap for roots the analysis cannot see, and last the component cap.

**confidence.31** A type error's skip removes the findings it withholds before the minimum and the severities are read, so a finding a type error skips is neither reported nor counted in `totals.withheld`.

## The minimum

**confidence.32** `analysis.min_confidence` defaults to `probable`, so a run over a library with no declared consumer withholds the findings about its published API, a run withholds every other finding this section places at `possible`, and every other finding is reported; the dial is the one mechanism that withholds a published API, and no kind's default depends on the target kind.

**confidence.33** The report's `totals.withheld` counts the findings the minimum withheld, each at the confidence this run assigns it, so its `certain` count is always 0. The text and SARIF outputs name the counts in one line with the setting that shows them, as [`text-line.md`](../text-line.md) states.

**confidence.34** The minimum and the severities are read before any cross-language edge is evaluated: a finding they withhold is the dead side of no edge evaluation, and an edge evaluation names only a finding the run reports.
