# Confidence evidence

The evidence that lowers a finding's reachability class, generated from `contract/confidence.json`. Each row names the rule of [the analysis](../contract/grammar/analysis.md) that states when it holds; a finding takes the lowest class any row that holds for it gives, then its kind's `max_class` and the component cap apply.

| Evidence | Lowers to | Languages | Rule | When |
| --- | --- | --- | --- | --- |
| `consumers-not-loaded` | `probable` | `go`, `ts` | [`confidence.2`](../contract/grammar/analysis/confidence.md) | A subject of a library's published API, while a declared consumer did not load. |
| `no-consumer-information` | `possible` | `go`, `ts` | [`confidence.2`](../contract/grammar/analysis/confidence.md) | A subject of a library's published API, in a run that holds no consumer information. |
| `unlinked-go-package` | `possible` | `go` | [`roots.25`](../contract/grammar/analysis/roots.md) | A declaration outside an internal tree of a Go package of an application that no build links, in a run that declares no consumer. |
| `name-literal` | `possible` | `go` | [`confidence.8`](../contract/grammar/analysis/confidence.md) | A package-level Go declaration whose identifier a string literal of a production file spells. |
| `in-operand` | `possible` | `ts` | [`confidence.13`](../contract/grammar/analysis/confidence.md) | A TypeScript member whose name the left operand of an in expression of a production file spells. |
| `outside-filler` | `possible` | `go` | [`confidence.14`](../contract/grammar/analysis/confidence.md) | A field the encoding-reflection class records only as filled by a callee outside the analyzed program. |
| `decoded-outside` | `possible` | `go`, `ts` | [`confidence.19`](../contract/grammar/analysis/confidence.md) | A member of a type the program only decodes into, where a callee outside the analysis, or an any or unknown value, fills it. |
| `test-reached` | `possible` | `ts` | [`confidence.20`](../contract/grammar/analysis/confidence.md) | A declaration of a TypeScript file only tests reach that is no test-support file, and each dead declaration it reaches through dead declarations. |
| `test-reached` | `possible` | `go` | [`confidence.21`](../contract/grammar/analysis/confidence.md) | A declaration of a Go package only tests reach that is no test-support package and that test code references, and each dead declaration it reaches or holds. |
| `possible-class` | `possible` | `go`, `ts` | [`confidence.23`](../contract/grammar/analysis/confidence.md) | A member that only records of exemption classes whose confidence is possible hold. |
| `roots-incomplete` | `probable` | `ts` | [`confidence.25`](../contract/grammar/analysis/confidence.md) | A TypeScript declaration found dead under reachability alone, or a file only dead code imports, while ts.roots_complete is not true. |
| `computed-load` | `probable` | `ts` | [`confidence.26`](../contract/grammar/analysis/confidence.md) | A finding the roots cap applies to, in a configuration that compiles a load by a computed path. |
| `schema-operation-default` | `possible` | `proto` | [`schemas.10`](../contract/grammar/analysis/schemas.md) | A schema service or operation, whose callers may run in programs the run does not analyze. |
| `http-exposure` | `possible` | `proto` | [`schemas.11`](../contract/grammar/analysis/schemas.md) | A schema declaration a service reaches that the generation configuration exposes over HTTP. |
| `descriptor-exposure` | `possible` | `proto` | [`schemas.12`](../contract/grammar/analysis/schemas.md) | A schema declaration a consumer binding of which carries the descriptor-exposure cap. |
| `persistence` | `possible` | `proto` | [`schemas.13`](../contract/grammar/analysis/schemas.md) | A schema field or enum value a consumer binding of which carries the persistence cap. |
| `explicit-json-name` | `possible` | `proto` | [`schemas.14`](../contract/grammar/analysis/schemas.md) | A schema field or enum value that states its JSON name. |
| `generation-config-unknown` | `possible` | `proto` | [`schemas.15`](../contract/grammar/analysis/schemas.md) | A schema declaration whose generation the run cannot account for in full. |
| `library-language` | `possible` | `proto` | [`schemas.16`](../contract/grammar/analysis/schemas.md) | A schema declaration a language whose target kind is library binds. |
