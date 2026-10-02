# The template rendering

The `template` format renders the report through a template the user supplies. The template is written in a subset of the action grammar of Go's [`text/template`](https://pkg.go.dev/text/template), and it is executed over the report document by the document's own JSON member names, so one template renders the report of any product the same way. This page states the subset, the data the template reads, how each value prints, and what a template that cannot be read, cannot be parsed or cannot be rendered does to the run. The vectors under `vectors/template/` pin an implementation against declared templates, reports and output bytes.

## Reading the template

The invocation names the template file. A product reads it and parses it before any analysis runs, whether or not the `template` format is requested, so a broken template is refused before it costs an analysis. The file is UTF-8 text.

| Condition | Outcome |
| --- | --- |
| The `template` format is requested and the invocation names no template | Exit code 2, before any analysis. |
| The named file cannot be read | Exit code 2, before any analysis, the file named. |
| The template does not parse, or uses a form this page leaves out of the subset | Exit code 2, before any analysis, the file and the line named. |
| The rendering fails | Exit code 3. The report is already written, the template's output is not written, and no partial output is left. |

## The data

The dot at the start of the template is the report document the run writes, the document whose bytes the JSON report holds, read as a JSON value: an object, whose members a field names by their JSON member names (`.findings`, `.position.path`, `.totals.omitted`); an array; a string; a number; `true` or `false`; or `null`. Every number a report holds is an integer. A merged report is read the same way.

## The subset

A template is text and actions. An action is delimited by `{{` and `}}`; the delimiters are fixed. The forms below have the meaning `text/template` gives them, and any form not listed is refused at parse.

- **Trim markers.** A left delimiter followed by a hyphen and a white-space character, as in `{{- .name}}`, removes the white space before the action, and a white-space character, a hyphen and a right delimiter, as in `{{.name -}}`, remove the white space after it. White space is space, tab, carriage return and line feed, and the marker needs the white-space character beside the hyphen: `{{-3}}` is the number -3.
- **Comments.** `{{/* text */}}`, alone in its action, with or without trim markers.
- **Constants.** An interpreted string in double quotes, whose escapes are `\a`, `\b`, `\f`, `\n`, `\r`, `\t`, `\v`, `\\`, `\"`, `\x` with two hexadecimal digits, `\u` with four and `\U` with eight; a raw string in back quotes; a decimal integer with an optional sign and no leading zero; `true`, `false` and `nil`.
- **Operands.** The dot `.`; a field chain on the dot (`.a.b`), on a variable (`$f.code`) or on a parenthesized pipeline (`(index .findings 0).code`); a variable; a constant; a parenthesized pipeline; a function call.
- **Pipelines and variables.** Commands joined by `|`, each command's value passed as the last argument of the next. A pipeline may begin with `$name :=`, which declares a variable, or `$name =`, which assigns a declared one; `range` admits `$index, $element :=`. `$` is the starting dot. A variable is in scope from its declaration to the `end` of the control structure that holds it.
- **Actions.** `{{pipeline}}`; `{{if pipeline}}`, with `{{else if pipeline}}` and `{{else}}`, closed by `{{end}}`; `{{with pipeline}}`, with `{{else with pipeline}}` and `{{else}}`, closed by `{{end}}`; `{{range pipeline}}`, with `{{else}}`, closed by `{{end}}`; and `{{break}}` and `{{continue}}` inside a `range`.
- **Functions.** `and`, `or`, `not`, `len`, `index`, `eq`, `ne`, `lt`, `le`, `gt`, `ge`, `print`, `printf` and `println`.

Refused at parse: every other function name, `html`, `js`, `urlquery`, `slice` and `call` among them; `define`, `template` and `block`; a character constant; an octal escape; and a number with a leading zero, a fraction, an exponent, a base prefix, an underscore or an imaginary part.

## Executing the template

The rendering is the text the template produces, written as UTF-8. Any condition this section names as failing fails the rendering as a whole.

**Fields and indexes.** A field names a member of an object. A field on an object that does not carry the member fails, and so does a field on any value that is not an object, `null` included. `index x k1 k2 …` reads `x[k1][k2]…`: an integer key reads an array element at that position, counted from 0, and a string key reads an object member; a key outside the array, a member the object does not carry, and a key of any other value fail.

**Truth.** `false`, `0`, `null`, the empty string, the empty array and the empty object are false, and every other value is true. `if` takes its first branch on a true value; `with` does as well and makes the value the dot inside it. `and` and `or` evaluate their operands in order and return the first operand that decides the result, or the last operand; `not` returns the negation of its operand's truth.

**Comparison.** `eq a b …` is true when `a` equals any later operand, and `ne a b` when the two differ. `lt`, `le`, `gt` and `ge` compare two operands. Two integers compare as numbers, two strings by the bytes of their UTF-8 encodings, and `eq` and `ne` compare two booleans as well; any other pair of operands fails, a string beside an integer and every array and object included.

**Length.** `len` of a string is the number of bytes of its UTF-8 encoding, of an array the number of its elements, and of an object the number of its members; `len` of any other value fails.

**Range.** `range` over an array visits each element in order, binding `$index` to its position counted from 0; over an object it visits each member in the bytewise order of the member names, binding `$index` to the name; over `null` or an empty array or object it runs the `else` branch. A range over any other value fails. `break` ends the range and `continue` ends the current visit.

**Printing.** An action that declares or assigns no variable writes its value: a string as itself; an integer as its decimal digits, a leading `-` when negative; `true` and `false`; `null` as `<no value>`; an array as `[`, its elements printed by the rule below and separated by one space, then `]`; and an object as `map[`, each member written as its name, `:` and its value printed by the rule below, ordered by name compared bytewise and separated by one space, then `]`. Inside an array or an object, and as an operand of `print`, `println` and `printf`, a value prints the same way except that `null` prints as `<nil>`.

`print` writes its operands one after another, with one space between two operands neither of which is a string. `println` writes its operands separated by one space, then a line feed. `printf` takes a format string first and writes it with each verb replaced by the next operand:

| Verb | Writes |
| --- | --- |
| `%v`, `%s` | The operand, printed as an operand of `print` prints it. |
| `%d` | An integer operand as its decimal digits; any other operand fails. |
| `%t` | A boolean operand as `true` or `false`; any other operand fails. |
| `%q` | The text `%s` writes, quoted: a `"`, then each character, with `"` and `\` written as `\"` and `\\`, U+0007, U+0008, U+000C, U+000A, U+000D, U+0009 and U+000B written as `\a`, `\b`, `\f`, `\n`, `\r`, `\t` and `\v`, every other character below U+0020 and U+007F written as `\x` and two lowercase hexadecimal digits, and every other character written as itself, then a `"`. |
| `%%` | A `%`, and takes no operand. |

A verb is `%` and exactly one of these characters, with no flag, width or precision. Any other character after a `%` fails, a verb with no operand left fails, and an operand no verb takes fails.

## What other documents fix

- [`report.schema.json`](../report.schema.json): the document the template reads and its member names.
- [`exit-codes.json`](../exit-codes.json): the meaning of codes 2 and 3.
- [`config.schema.json`](../config.schema.json): the `template` value of `reporters.formats`.
- `vectors/template/`: one directory per case, each with its template, its report and either the exact output or the exit code the template ends the run with.
