# Template vectors

One case per directory, named for what it establishes. A case holds a template, the report it renders and the outcome: the exact bytes the rendering writes, or the exit code the template ends the run with. The language and the data model the cases exercise are the ones [`template.md`](../../contract/grammar/template.md) states.

## The files of a case

| File | What it holds |
| --- | --- |
| `template.tmpl` | the template, UTF-8 text |
| `report.json` | the report the template renders, an instance of [`report.schema.json`](../../contract/report.schema.json) |
| `expected.txt` | the rendering, byte for byte, where the template renders |
| `expected_exit` | the exit code, one integer, where it does not: 2 for a template refused before any analysis, 3 for a rendering that fails |

A case holds `template.tmpl`, `report.json` and exactly one of `expected.txt` and `expected_exit`.

## Running a case

Parse `template.tmpl`. Where it does not parse, or uses a form the subset leaves out, the outcome is 2. Otherwise execute it with `report.json`, read as a JSON value, as the dot. Where the rendering fails the outcome is 3; otherwise compare the bytes it writes with `expected.txt`.

## What the cases establish

| Case | What it establishes | Outcome |
| --- | --- | --- |
| `character-constant` | A character constant. The subset refuses it at parse, so the run ends before any analysis. | 2 |
| `comparison-of-two-kinds` | `eq` between an integer and a string. A comparison of two kinds of value fails the rendering. | 3 |
| `field-on-a-string` | A field on a string member. A field on a value that is not an object fails the rendering. | 3 |
| `findings-by-member-name` | A range over the findings, each line built from members named as the JSON report names them, then two totals. | rendered |
| `function-outside-the-subset` | `html`, a function the subset leaves out, is refused at parse. | 2 |
| `index-of-a-missing-key` | `index` on an object member the object does not carry fails the rendering, after a member it does carry was read. | 3 |
| `index-past-the-array` | `index` one element past the end of an array fails the rendering, after the last element was read. | 3 |
| `integers-print-in-decimal` | A report integer above a million prints as its decimal digits in an action, under `%d`, `%v` and `%s`, and as an operand of `print`. | rendered |
| `length-in-bytes` | `len` counts the UTF-8 bytes of a string, seven for a string holding one two-byte character, the elements of an array and the members of an object. | rendered |
| `member-the-document-lacks` | A field naming a member the object does not carry fails the rendering, after a member it does carry was printed. | 3 |
| `nil-as-a-command` | `nil` written as the command of an action. A constant `nil` is an operand of a function alone, so the rendering fails after the text before it was read. | 3 |
| `nil-as-an-argument` | `nil` as an operand of `print` and of `printf` under `%v` prints as `<nil>`, and `print` puts one space between it and an integer. | rendered |
| `number-with-a-fraction` | A number constant with a fraction is refused at parse. | 2 |
| `octal-escape` | An octal escape in an interpreted string is refused at parse. | 2 |
| `print-spacing` | `print` puts one space between two operands neither of which is a string and none beside a string; `println` separates every operand and ends with a line feed; a pipeline passes its value as the last argument. | rendered |
| `printf-operand-left-over` | A `printf` operand no verb takes fails the rendering. | 3 |
| `printf-verb-outside-the-subset` | `%x`, a verb outside the subset, fails the rendering. | 3 |
| `printf-verbs` | Every verb of the subset once: `%q` escapes a quote, a backslash, a tab, U+0001 and U+007F, and writes a non-ASCII character as itself; `%v` prints an array of objects in the bracketed form. | rendered |
| `range-orders-and-stops` | A range over an object visits its members in the bytewise order of their names; a range over an empty array runs its `else`; `continue` skips a visit and `break` ends the range. | rendered |
| `strings-compare-by-bytes` | `lt` orders U+FF5E before U+1F600, the order of their UTF-8 bytes, which their UTF-16 code units reverse; `eq` takes several operands and `ge` compares integers. | rendered |
| `template-definition` | `define` and `template` are refused at parse. | 2 |
| `truth-and-logic` | `and` and `or` return the operand that decides them, an empty array and a zero are false, `else if` chains within `if`, and `with` makes its value the dot or runs its `else`. | rendered |
| `unclosed-action` | An action with no closing delimiter does not parse. | 2 |
| `values-print-bracketed` | An array prints as `[` and `]`, an object as `map[` and `]` with its members ordered by name, an empty array as `[]`, and `nil` as an operand of `print` and `printf` as `<nil>`. | rendered |
| `variables-and-trim-markers` | Trim markers remove the white space beside them, a comment writes nothing, a variable declared before a range keeps the value assigned inside it, `$` reads the document from inside a pipeline, and a raw string keeps its backslash. | rendered |
