# Issue kinds

Every finding carries a code that names its kind. A code is `DS` and four digits: the first two name the family, the question a reader asks of the finding, and the last two the kind within it. A kind is a concept, not a language, so a language joins the codes whose concept it has. A code names at most one kind for the life of the code space, and a retired code is never reused. This page and the family pages are generated from `contract/kinds.json`, `contract/confidence.json` and the examples in `authoring/kinds-guide.json`; the normative rule of each kind is its row in `contract/kinds.json`.

DS19 to DS89 are left for families not yet defined, and DS90 to DS99 hold the findings about deadset's own configuration.

| Family | Codes | The question it answers | Kinds |
| --- | --- | --- | --- |
| [Unused](kinds/unused.md) | `DS1000` to `DS1099` | Nothing refers to it. | `DS1001`, `DS1002`, `DS1003`, `DS1004`, `DS1005`, `DS1006` |
| [Only dead code or tests use it](kinds/dead-or-test-only.md) | `DS1100` to `DS1199` | Something refers to it, but only dead code or tests. | `DS1101`, `DS1102`, `DS1103`, `DS1104` |
| [Interfaces](kinds/interfaces.md) | `DS1200` to `DS1299` | The interface or its method is never used through the interface. | `DS1201`, `DS1202`, `DS1203` |
| [Data nobody reads](kinds/unread-data.md) | `DS1300` to `DS1399` | It is written, but nothing reads what it stores. | `DS1301` |
| [Inside functions](kinds/inside-functions.md) | `DS1400` to `DS1499` | A part of a function does nothing. | `DS1401`, `DS1402`, `DS1403`, `DS1404`, `DS1405`, `DS1406` |
| [Too visible](kinds/too-visible.md) | `DS1500` to `DS1599` | It is used, but exposed more widely than its uses need. | `DS1501`, `DS1502`, `DS1503` |
| [Files](kinds/files.md) | `DS1600` to `DS1699` | The file is never built or never imported. | `DS1601`, `DS1602` |
| [Dependencies and module files](kinds/dependencies.md) | `DS1700` to `DS1799` | The declared dependency or module directive changes nothing. | `DS1701`, `DS1702` |
| [Schemas](kinds/schemas.md) | `DS1800` to `DS1899` | A schema definition nothing generated from it uses. | `DS1801`, `DS1802`, `DS1803`, `DS1804`, `DS1805` |
| [deadset configuration](kinds/configuration.md) | `DS9000` to `DS9099` | A suppression, root, declaration or edge deadset was given no longer matches anything. | `DS9001`, `DS9002`, `DS9003`, `DS9004`, `DS9005`, `DS9006` |

## The columns

- Languages are the languages that report the kind: `go` is Go, `ts` is TypeScript.
- Default is `default_enabled`: `on` means an analyzer reports the kind unless a configuration disables it.
- Severity is `default_severity`, one of `allow`, `warn` and `deny`. A finding at or above `reporters.fail_on`, `deny` by default, fails the run.
- Fixability is `fixability`: what a mechanical edit may do with the finding. A finding in a generated file carries `none` under every kind.
- Confidence is the finding's reachability class capped by the kind's `max_class` and lowered by the evidence each family page lists. `analysis.min_confidence`, `probable` by default, withholds the findings below it.

## Retired codes

No code is retired.
