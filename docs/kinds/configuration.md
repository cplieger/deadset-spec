# deadset configuration: `DS9000` to `DS9099`

A suppression, root, declaration or edge deadset was given no longer matches anything. Suppressions, configured roots, configured declarations and declared edges that no longer match anything: the run checks its own inputs. Generated from `contract/kinds.json`; the [issue kinds](../kinds.md) page lists every family.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS9001` | `suppression-without-reason` | `go`, `ts` | `on` | `deny` | `none` |
| `DS9002` | `unscoped-ignore-entry` | `go`, `ts` | `on` | `deny` | `none` |
| `DS9003` | `stale-suppression` | `go`, `ts` | `on` | `deny`, fixed | `none` |
| `DS9004` | `unmatched-root` | `go`, `ts` | `on` | `deny`, fixed | `none` |
| `DS9005` | `stale-cross-language-edge` | `go`, `ts` | `on` | `deny` | `none` |
| `DS9006` | `unmatched-configured-declaration` | `ts` | `on` | `deny`, fixed | `none` |

## DS9001 suppression-without-reason

A suppression that gives no reason: nobody can later tell whether it still holds.

Before:

```go
//deadset:ignore DS1002
func legacy() {}
```

After:

```go
//deadset:ignore DS1002 -- kept for the plugin loader, which calls it by name
func legacy() {}
```

How to fix it: Add the reason after the code, or delete the suppression. The finding's fixability is `none`.

The rule: A suppression that carries no reason, in any of the three documents: an inline directive, an ignore-file entry or a baseline row. For the ignore file and the baseline, an entry or a row whose reason field is absent, empty or whitespace only is refused and reported rather than matched.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS9002 unscoped-ignore-entry

An ignore-file entry that names no path, so it can bind more than the one site it was written for.

Before:

```go
{"code": "DS1001", "symbol": "go://example.com/app#Run", "reason": "called by the scheduler"}
```

After:

```go
{"code": "DS1001", "symbol": "go://example.com/app#Run", "path": "run.go", "reason": "called by the scheduler"}
```

How to fix it: Give the entry the path of the file it is about. The finding's fixability is `none`.

The rule: An ignore-file entry or a baseline row that names a symbol and no file path. The entry is reported rather than matched, so a bare name cannot mask a match anywhere else in the project. The inline directive is scoped by its position and cannot be unscoped.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS9003 stale-suppression

A suppression that no longer withholds any finding: the code it names would not be reported there any more.

Before:

```ts
// deadset:ignore DS1002 -- used by the old router
function route(): void {} // route is now called, so nothing is reported here
```

After:

```ts
function route(): void {}
```

How to fix it: Delete the suppression. The finding's fixability is `none`.

The rule: A suppression record that is stale, as grammar/suppression.md defines it, at either mechanism and in the baseline alike. The run exits with the findings code when at least one is reported. The kind is fixed on at deny: no flag, no severity setting and no per-mechanism exception reduces it below a finding, and a configuration naming this code under a severity key is an unimplemented key.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS9004 unmatched-root

A configured root that names no declaration: the declaration was renamed or deleted.

Before:

```go
"roots": { "patterns": ["go://example.com/app#OldHandler"] }
```

After:

```go
"roots": { "patterns": ["go://example.com/app#Handler"] }
```

How to fix it: Correct the root to the declaration's present reference, or delete it. The finding's fixability is `none`.

The rule: A configured root that matches no symbol, or a configured root pattern that matches no symbol. A root set nobody checks silently changes every result, so a stale one is a finding. The kind is fixed on at deny: no flag, no severity setting and no exception reduces it below a finding, and a configuration naming this code under a severity key is an unimplemented key.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS9005 stale-cross-language-edge

A declared cross-language edge whose side names nothing any analyzer found.

Before:

```go
{"id": "render", "because": "generated", "provides": "go://example.com/app#RenderOld", "used_by": "ts://@example/web/src/render.ts#render"}
```

After:

```go
{"id": "render", "because": "generated", "provides": "go://example.com/app#Render", "used_by": "ts://@example/web/src/render.ts#render"}
```

How to fix it: Correct the edge to the present references, or delete it. The finding's fixability is `none`.

The rule: A declared cross-language edge with a side that at least one analyzer evaluated and every evaluation of that side reports absent, so no analyzer that ran enumerates that side's symbol. Emitted by the merge, once per edge, never by an analyzer, so a stale edge is visible rather than silently inert. A dead side paired with an absent side drops its pending finding, and the edge is the reported defect: a misspelled reference must not turn a live pairing into a deletion.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS9006 unmatched-configured-declaration

A configuration entry that names a declaration the analysis does not hold.

Before:

```ts
"ts": { "serializers": [{ "module": "@example/codec", "name": "encodeOld" }] }
```

After:

```ts
"ts": { "serializers": [{ "module": "@example/codec", "name": "encode" }] }
```

How to fix it: Correct the entry to the declaration's present reference, or delete it. The finding's fixability is `none`.

The rule: An entry of a configuration key that names a declaration, being an entry of ts.injection_registrations, of the components or the bases of an entry of ts.lifecycle_contracts, or of ts.serializers, that names no declaration in any project of the run, by the resolution the ts section of contract/config.schema.json states for its shape. An entry that names a declaration no call reaches matches. An exemption whose configured declarations nobody checks silently retains nothing, so a stale entry is a finding. The kind is fixed on at deny: no flag, no severity setting and no exception reduces it below a finding, and a configuration naming this code under a severity key is an unimplemented key.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- A finding about a member of a dead component carries no higher confidence than the component's root members.
