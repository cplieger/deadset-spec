# Schemas: `DS1800` to `DS1899`

A schema definition nothing generated from it uses. Schema declarations no generated code in any analyzed language uses: messages and enums, fields, enum values, services and their operations. Generated from `contract/kinds.json`; the [issue kinds](../kinds.md) page lists every family.

| Code | Name | Languages | Default | Severity | Fixability |
| --- | --- | --- | --- | --- | --- |
| `DS1801` | `unused-schema-type` | `proto` | `on` | `deny` | `deletable` |
| `DS1802` | `unread-schema-field` | `proto` | `on` | `deny` | `manual` |
| `DS1803` | `unused-schema-enum-value` | `proto` | `on` | `deny` | `manual` |
| `DS1804` | `unused-schema-service` | `proto` | `on` | `deny` | `manual` |
| `DS1805` | `uncalled-schema-operation` | `proto` | `on` | `warn` | `manual` |

## DS1801 unused-schema-type

A message or enum that no generated code in any analyzed language references, and that no live schema declaration uses, has no reader or writer the analysis can see.

Before:

```proto
syntax = "proto3";
package v1;

message PingRequest { string text = 1; }

// Nothing generated from it is used.
message LegacyPing { string text = 1; }
```

After:

```proto
syntax = "proto3";
package v1;

message PingRequest { string text = 1; }
```

How to fix it: Delete the declaration and regenerate. Deleting a message changes no wire format, but check first that no program outside the run is generated from the schema. The finding's fixability is `deletable`.

The rule: A schema message or enum that no binding in any analyzed language references, that no live schema declaration uses, and none of whose members is live.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `possible` ([`schemas.11`](../../contract/grammar/analysis/schemas.md)): A schema declaration a service reaches that the generation configuration exposes over HTTP.
- `possible` ([`schemas.12`](../../contract/grammar/analysis/schemas.md)): A schema declaration a consumer binding of which carries the descriptor-exposure cap.
- `possible` ([`schemas.15`](../../contract/grammar/analysis/schemas.md)): A schema declaration whose generation the run cannot account for in full.
- `possible` ([`schemas.16`](../../contract/grammar/analysis/schemas.md)): A schema declaration a language whose target kind is library binds.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1802 unread-schema-field

A field that generated code writes but nothing reads carries information no reader receives.

Before:

```proto
message PingRequest {
  string text = 1;
  // Only ever written.
  int64 sent_at = 2;
}
```

After:

```proto
message PingRequest {
  string text = 1;
  reserved 2;
  reserved "sent_at";
}
```

How to fix it: Replace the field with reserved statements for its number and its name, so the number and the name are never reused, and regenerate. The finding's fixability is `manual`.

The rule: A schema field that no binding of it reads in any analyzed language: a field, a oneof member, a map field, a group field or an extension. A write is no read, because a field only written carries information no reader receives. Generated transport's encoding of it is no read, and a hand-code serializer's encoding is.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `possible` ([`schemas.11`](../../contract/grammar/analysis/schemas.md)): A schema declaration a service reaches that the generation configuration exposes over HTTP.
- `possible` ([`schemas.12`](../../contract/grammar/analysis/schemas.md)): A schema declaration a consumer binding of which carries the descriptor-exposure cap.
- `possible` ([`schemas.13`](../../contract/grammar/analysis/schemas.md)): A schema field or enum value a consumer binding of which carries the persistence cap.
- `possible` ([`schemas.14`](../../contract/grammar/analysis/schemas.md)): A schema field or enum value that states its JSON name.
- `possible` ([`schemas.15`](../../contract/grammar/analysis/schemas.md)): A schema declaration whose generation the run cannot account for in full.
- `possible` ([`schemas.16`](../../contract/grammar/analysis/schemas.md)): A schema declaration a language whose target kind is library binds.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1803 unused-schema-enum-value

An enum value no generated code names, of an enum whose names no hand code converts, is a value no program produces or tests for.

Before:

```proto
enum Mode {
  MODE_UNSPECIFIED = 0;
  MODE_FAST = 1;
  MODE_LEGACY = 2;
}
```

After:

```proto
enum Mode {
  MODE_UNSPECIFIED = 0;
  MODE_FAST = 1;
  reserved 2;
  reserved "MODE_LEGACY";
}
```

How to fix it: Replace the value with reserved statements for its number and its name, and regenerate. The finding's fixability is `manual`.

The rule: A schema enum value that no binding names, of an enum whose name conversion no hand code reaches. A value that shares its number with another is never reported.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `possible` ([`schemas.11`](../../contract/grammar/analysis/schemas.md)): A schema declaration a service reaches that the generation configuration exposes over HTTP.
- `possible` ([`schemas.12`](../../contract/grammar/analysis/schemas.md)): A schema declaration a consumer binding of which carries the descriptor-exposure cap.
- `possible` ([`schemas.13`](../../contract/grammar/analysis/schemas.md)): A schema field or enum value a consumer binding of which carries the persistence cap.
- `possible` ([`schemas.14`](../../contract/grammar/analysis/schemas.md)): A schema field or enum value that states its JSON name.
- `possible` ([`schemas.15`](../../contract/grammar/analysis/schemas.md)): A schema declaration whose generation the run cannot account for in full.
- `possible` ([`schemas.16`](../../contract/grammar/analysis/schemas.md)): A schema declaration a language whose target kind is library binds.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1804 unused-schema-service

A service that no client calls and that hand code only registers serves no request any analyzed program sends.

Before:

```proto
service PingService { rpc Ping(PingRequest) returns (PingResponse); }

// Registered, never called.
service AdminService { rpc Reset(ResetRequest) returns (ResetResponse); }
```

After:

```proto
service PingService { rpc Ping(PingRequest) returns (PingResponse); }
```

How to fix it: Delete the service and its registration, then regenerate. Check first that no client outside the run calls it. The finding's fixability is `manual`.

The rule: A schema service none of whose operations any client binding calls and none of whose service bindings hand code uses other than through a registration form. Its operations fall with it.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `possible` ([`schemas.10`](../../contract/grammar/analysis/schemas.md)): A schema service or operation, whose callers may run in programs the run does not analyze.
- `possible` ([`schemas.11`](../../contract/grammar/analysis/schemas.md)): A schema declaration a service reaches that the generation configuration exposes over HTTP.
- `possible` ([`schemas.12`](../../contract/grammar/analysis/schemas.md)): A schema declaration a consumer binding of which carries the descriptor-exposure cap.
- `possible` ([`schemas.15`](../../contract/grammar/analysis/schemas.md)): A schema declaration whose generation the run cannot account for in full.
- `possible` ([`schemas.16`](../../contract/grammar/analysis/schemas.md)): A schema declaration a language whose target kind is library binds.
- A finding about a member of a dead component carries no higher confidence than the component's root members.

## DS1805 uncalled-schema-operation

An operation no client binding calls answers no request any analyzed program sends; its server implementation is kept alive only by the registration.

Before:

```proto
service PingService {
  rpc Ping(PingRequest) returns (PingResponse);
  rpc Fail(FailRequest) returns (FailResponse);
}
```

After:

```proto
service PingService {
  rpc Ping(PingRequest) returns (PingResponse);
}
```

How to fix it: Deprecate the operation, or delete it with its server method once no client outside the run calls it, and regenerate. The finding's fixability is `manual`.

The rule: A schema operation that no client binding in any analyzed language calls. Each hand-written method implementing it that nothing else keeps falls in this finding's component, under the code its own references select.

What lowers its confidence:

- The kind's ceiling, `max_class`, is `certain`.
- `possible` ([`schemas.10`](../../contract/grammar/analysis/schemas.md)): A schema service or operation, whose callers may run in programs the run does not analyze.
- `possible` ([`schemas.11`](../../contract/grammar/analysis/schemas.md)): A schema declaration a service reaches that the generation configuration exposes over HTTP.
- `possible` ([`schemas.12`](../../contract/grammar/analysis/schemas.md)): A schema declaration a consumer binding of which carries the descriptor-exposure cap.
- `possible` ([`schemas.15`](../../contract/grammar/analysis/schemas.md)): A schema declaration whose generation the run cannot account for in full.
- `possible` ([`schemas.16`](../../contract/grammar/analysis/schemas.md)): A schema declaration a language whose target kind is library binds.
- A finding about a member of a dead component carries no higher confidence than the component's root members.
