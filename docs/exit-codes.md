# Exit codes

Every analyzer and the orchestrator return one of five exit codes, so one gate reads every product
the same way. A code is a fact about the run, not about one finding: it says whether a verdict
exists and, when one does, what the verdict is.

| Code | Name | Meaning |
| --- | --- | --- |
| 0 | `clean` | The report holds no finding at or above the failing severity, no stale suppression and no pending finding. Also the code a run returns when the exit code is configured off, while still printing the report. |
| 1 | `findings` | The report holds at least one finding at or above the failing severity, or at least one stale suppression, whatever severity the configuration assigns to any other kind; the count of stale suppressions is named. |
| 2 | `usage` | The invocation is malformed, no configuration source supplies the target kind, a configuration source names a key the product does not implement, an explanation request names a symbol that does not exist, a template the invocation names cannot be read or does not parse, or a flag asks the product to edit source. The usage text is printed and no analysis runs. |
| 3 | `failure` | A configuration the invocation or the configuration document names cannot be read, the run holds no program to analyze, a setup failure stops the analysis, the analysis needs more memory than the machine makes available, a declared consumer or a declared build configuration failed to load, an analyzer could not be found, described or admitted, an acquired artifact's digest did not match, a pending finding met no other side at the merge, or a requested rendering could not be produced, in which case the report is already written and the rendering is not. A type error in source the program holds is none of these: it skips the function holding it, as grammar/analysis.md states, and the exit code follows the findings. The errors are printed and no finding list is, so a partial result is never read as a clean tree. |
| 4 | `pending` | The report holds at least one pending finding, a finding whose cross-language edge the other side has not evaluated, and the count of pending findings is named. A report with this code is an input to a merge, not an answer. |

From `contract/exit-codes.json`, one row per code.

## Precedence

Codes 2 and 3 end a run before any verdict exists, and each prints no finding list. Codes 4, 1 and
0 are verdicts about a complete report, and when more than one verdict condition holds the highest
code wins: a pending finding outranks findings and stale suppressions, because an unmerged report
cannot be read as an answer at all, and those outrank a clean result. A finding fails a run when its
severity is at or above `reporters.fail_on`, `deny` by default, so under the default a `warn`
finding never fails a run (from `contract/exit-codes.json`, the document's `description`).

Two consequences a gate can rely on. A run that returns 0 or 1 has produced a complete report, so a
gate reads the report for either code. A run that returns 4 has produced a report that is an input
to a merge, so a gate that treats 4 as a failure of the target is reading a partial answer; merge
the reports and read the merged verdict instead (from `contract/grammar/merge.md`, step 7).

## Setup failures and memory

A setup failure is a file or a component the analysis needs and the project does not provide. It
ends the run with code 3 before any finding list exists, and the run prints one line per failure on
standard error that starts with `setup failure:`, a space, the class, a colon and a space, then
names what is missing and the fix (from `contract/exit-codes.json`, `setup_failures`, and `contract/grammar/analysis.md`,
"Setup failures"):

| Class | Meaning | Fix |
| --- | --- | --- |
| `missing-module` | An import names a module the project expects to exist and nothing provides it: code a generator writes that was not generated, a package that was not built, or a declared dependency that was not installed. | Name the import and the file that writes it, and tell the user to run the generator, the build or the install that provides the module. |
| `incomplete-module-sum` | The module sum file lacks a checksum the build of the target or of a declared consumer needs. | Name the module and tell the user to run go mod tidy in the module that requires it. |
| `test-build-tag` | Test files of the target build under no configuration of the run, because a build constraint they carry is satisfied by no configuration. | Name the files and the exact entry of analysis.configurations that builds them. |
| `missing-consumer` | A declared consumer is absent from the path the scope names for it, or its own dependencies are not installed. | Name the consumer and tell the user to check it out at that path and install its dependencies. |
| `workspace-member-without-source` | An import resolves to a member of the workspace, and neither the file the default resolution reaches nor the member's manifest entry read back through its emit mappings is a source file of the member. | Name the member, the subpath, the importing file and the targets the member's manifest names, and tell the user to build the member, or to give its compiler configuration an output and a root directory that map the target to its source. |
| `convention-not-literal` | A convention row applies and the configuration property that moves one of its directories is not a literal in the framework configuration file. | Name the file and the property, and tell the user to write the property as a literal, or to disable the row in ts.disabled_conventions and name the files in ts.entry_files. |

A run that needs more memory than the machine makes available also ends with code 3, before it
writes a report, and prints `memory exhausted: at least N GB were needed, M GB are available`
(from `contract/exit-codes.json`, `memory_exhaustion`). A type error in source the program holds
ends nothing: it skips the function that holds it and the exit code follows the findings (from
`contract/grammar/analysis.md`, "Type errors").

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

The merge's verdict is 0 or 1, and it applies the caller's `fail_on` as an analyzer's verdict
applies `reporters.fail_on` (from `contract/grammar/merge.md`, step 7). It returns 3 in two places,
admission and an edge whose paired side no report evaluated, and in both a merged report never
exists. It never returns 4, because the merged report holds no pending finding, and never 2, because
a malformed invocation is reported before any report exists (from `contract/grammar/merge.md`, the
step-and-exit-code table).
