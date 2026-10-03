# Exit codes

Every analyzer and the orchestrator return one of five exit codes, so one gate reads every product the same way. A code is a fact about the run, not about one finding. It says whether a verdict exists and, when one does, what the verdict is.

| Code | Name | Meaning |
| --- | --- | --- |
| 0 | `clean` | The report holds no finding at or above the failing severity, no stale suppression and no pending finding. |
| 1 | `findings` | The report holds at least one finding at or above the failing severity, or at least one stale suppression. |
| 2 | `usage` | The invocation cannot run. The usage text is printed and no analysis runs. |
| 3 | `failure` | The run cannot reach a verdict. The errors are printed and no finding list is. |
| 4 | `pending` | The report holds at least one pending finding, a finding whose cross-language edge the other side has not evaluated. |

The source is `contract/exit-codes.json`, one row per code. The conditions of each code follow.

## The conditions of each code

Code 0 is also the code a run returns when the exit code is configured off, and the run still prints the report.

Code 1 holds whatever severity the configuration assigns to any other kind, and the run names the count of stale suppressions.

Code 2 covers six cases:

- The invocation is malformed.
- No configuration source supplies the target kind.
- A configuration source names a key the product does not implement.
- An explanation request names a symbol that does not exist.
- A template the invocation names cannot be read or does not parse.
- A flag asks the product to edit source.

Code 3 covers nine cases:

- A configuration the invocation or the configuration document names cannot be read.
- The run holds no program to analyze.
- A setup failure stops the analysis of a declared build configuration, or of every configuration the analysis derived.
- The analysis needs more memory than the machine makes available.
- A declared consumer or a declared build configuration failed to load.
- An analyzer could not be found, described or admitted.
- An acquired artifact's digest did not match.
- A pending finding met no other side at the merge.
- A requested rendering could not be produced. The report is already written and the rendering is not.

Under code 3 no finding list is printed, so a partial result is never read as a clean tree. A type error in source the program holds is none of these cases. It skips the function holding it, as `contract/grammar/analysis.md` states, and the exit code follows the findings.

Code 4 names the count of pending findings. A report with this code is an input to a merge, not an answer.

## Precedence

Codes 2 and 3 end a run before any verdict exists, and each prints no finding list. Codes 4, 1 and 0 are verdicts about a complete report, and when more than one verdict condition holds the highest code wins. A pending finding outranks findings and stale suppressions, because an unmerged report cannot be read as an answer at all, and those outrank a clean result. A finding fails a run when its severity is at or above `reporters.fail_on`, `deny` by default, so under the default a `warn` finding never fails a run. The source is `contract/exit-codes.json`, the document's `description`.

Two consequences a gate can rely on. A run that returns 0 or 1 has produced a complete report, so a gate reads the report for either code. A run that returns 4 has produced a report that is an input to a merge, so a gate that treats 4 as a failure of the target is reading a partial answer. Merge the reports and read the merged verdict instead. The source is `contract/grammar/merge.md`, step 7.

## Setup failures and memory

A setup failure is a file or a component the analysis needs and the project does not provide. In a declared build configuration it ends the run with code 3 before any finding list exists. A configuration the analysis derived is dropped instead, and the run analyzes the rest. The report lists the dropped configuration in `configurations_not_built`, with the failure's line as its error. When every derived configuration is dropped, the run ends with code 3 as well.

For a failure that ends the run, the run prints one line per failure on standard error. Each line starts with `setup failure:`, a space, the class, a colon and a space, then names what is missing and the fix. The sources are `contract/exit-codes.json`, `setup_failures`, and `contract/grammar/analysis.md`, "Setup failures". The six classes are these:

- `missing-module` means an import names a module the project expects to exist and nothing provides it. That is code a generator writes that was not generated, a package that was not built, or a declared dependency that was not installed. The line names the import and the file that writes it, and tells the user to run the generator, the build or the install that provides the module.
- `incomplete-module-sum` means the module sum file lacks a checksum the build of the target or of a declared consumer needs. The line names the module and tells the user to run `go mod tidy` in the module that requires it.
- `test-build-tag` means test files of the target build under no configuration of the run, because a build constraint they carry is satisfied by no configuration. The line names the files and the exact entry of `analysis.configurations` that builds them.
- `missing-consumer` means a declared consumer is absent from the path the scope names for it, or its own dependencies are not installed. The line names the consumer and tells the user to check it out at that path and install its dependencies.
- `workspace-member-without-source` means an import resolves to a member of the workspace, and no source file of the member is found. Neither the file the default resolution reaches nor the member's manifest entry, read back through its emit mappings, is a source file of the member. The line names the member, the subpath, the importing file and the targets the member's manifest names. It tells the user to build the member, or to give its compiler configuration an output and a root directory that map the target to its source.
- `convention-not-literal` means a convention row applies and the configuration property that moves one of its directories is not a literal in the framework configuration file. The line names the file and the property. It tells the user to write the property as a literal, or to disable the row in `ts.disabled_conventions` and name the files in `ts.entry_files`.

A run that needs more memory than the machine makes available also ends with code 3, before it writes a report. It prints one line on standard error, `memory exhausted: at least N GB were needed, M GB are available`. N is the memory the analysis was about to need when it stopped, rounded up. M is the memory the machine made available to it, rounded down, so N is always greater than M. Each is a decimal number with at most one digit after the point. The source is `contract/exit-codes.json`, `memory_exhaustion`.

A type error in source the program holds ends nothing. It skips the function that holds it and the exit code follows the findings. The source is `contract/grammar/analysis.md`, "Type errors".

## What decides code 1

Three inputs decide it, and each is declared elsewhere in the contract:

- The severity of each finding. A kind's default severity is its `default_severity` in `contract/kinds.json`. A configuration overrides it by naming the code or a two-digit family prefix under `severity` in `contract/config.schema.json`.
- The failing severity, which is `reporters.fail_on` in `contract/config.schema.json`. A finding at or above it fails the run.
- Any stale suppression. One is enough, whatever severity the configuration assigns to any other kind. `DS1703` is the kind that reports it, as `contract/kinds.json`, row `DS1703`, states.

## What the merge returns

The merge's verdict is 0 or 1, and it applies the caller's `fail_on` as an analyzer's verdict applies `reporters.fail_on`. The source is `contract/grammar/merge.md`, step 7. The merge returns 3 in two places, admission and an edge whose paired side no report evaluated, and in both a merged report never exists. It never returns 4, because the merged report holds no pending finding, and never 2, because a malformed invocation is reported before any report exists. The source is `contract/grammar/merge.md`, the step-and-exit-code table.
