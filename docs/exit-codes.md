# Exit codes

Every analyzer and the orchestrator return one of five exit codes, so one gate reads every product
the same way. A code is a fact about the run, not about one finding: it says whether a verdict
exists and, when one does, what the verdict is.

| Code | Name | Meaning |
| --- | --- | --- |
| 0 | `clean` | The report holds no finding at or above the failing severity, no stale suppression and no pending finding. Also the code a run returns when the exit code is configured off, while still printing the report. |
| 1 | `findings` | The report holds at least one finding at or above the failing severity, or at least one stale suppression, whatever severity the configuration assigns to any other kind; the count of stale suppressions is named. |
| 2 | `usage` | The invocation is malformed, no configuration source supplies the target kind, a configuration source names a key the product does not implement, an explanation request names a symbol that does not exist, or a flag asks the product to edit source. The usage text is printed and no analysis runs. |
| 3 | `failure` | The target, a declared consumer or a build configuration failed to load or type-check, a file's references could not be resolved, an analyzer could not be found, described or admitted, an acquired artifact's digest did not match, or a pending finding met no other side at the merge. The errors are printed and no finding list is, so a partial result is never read as a clean tree. |
| 4 | `pending` | The report holds at least one pending finding, a finding whose cross-language edge the other side has not evaluated, and the count of pending findings is named. A report with this code is an input to a merge, not an answer. |

From `contract/exit-codes.json`, one row per code.

## Precedence

Codes 2 and 3 end a run before any verdict exists, and each prints no finding list. Codes 4, 1 and
0 are verdicts about a complete report, and when more than one verdict condition holds the highest
code wins: a pending finding outranks findings and stale suppressions, because an unmerged report
cannot be read as an answer at all, and those outrank a clean result. A finding whose severity is
`warn` never fails a run (from `contract/exit-codes.json`, the document's `description`).

Two consequences a gate can rely on. A run that returns 0 or 1 has produced a complete report, so a
gate reads the report for either code. A run that returns 4 has produced a report that is an input
to a merge, so a gate that treats 4 as a failure of the target is reading a partial answer; merge
the reports and read the merged verdict instead (from `contract/grammar/merge.md`, step 7).

## What decides code 1

Three inputs decide it, and each is declared elsewhere in the contract:

- The severity of each finding. A kind's default severity is its `default_severity` (from
  `contract/kinds.json`), and a configuration overrides it by naming the code or a two-digit family
  prefix (from `contract/config.schema.json`, `severity`).
- The failing severity, which is `reporters.fail_on` (from `contract/config.schema.json`). A
  finding at or above it fails the run.
- Any stale suppression. One is enough, whatever severity the configuration assigns to any other
  kind, and `DS1703` is the kind that reports it (from `contract/kinds.json`, row `DS1703`).

## What the merge returns

The merge's verdict is 0 or 1. It returns 3 in two places, admission and an edge whose paired side
no report evaluated, and in both a merged report never exists. It never returns 4, because the
merged report holds no pending finding, and never 2, because a malformed invocation is reported
before any report exists (from `contract/grammar/merge.md`, the step-and-exit-code table).
