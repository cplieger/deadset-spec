# Schemas

Rules `schemas.1` to `schemas.24` of [the analysis](../analysis.md). A rule is cited by its id, which is never reused for another rule. These rules are followed by the schema languages of [`languages.json`](../../languages.json), the rows of `kind` `schema`, and by no analyzer.

## Schema declarations

**schemas.1** A schema reader reads every schema file of its language in scope, as contract/languages.json's row puts it in scope, and publishes one schema record per declaration of the file: for Protocol Buffers a message, an enum, a field, a oneof member, a map field, a group field, an extension, an enum value, a service and an RPC, nested declarations included. The record's `symbol` and `schema` are the declaration's `proto://` reference, its package and its dotted name inside the file, as [the reference grammar](../symbol-ref.md) states, and its `schema_file` is the file's import path. A declaration of a file outside the target, a well-known type of `google/protobuf` read from the include path included, is never published, because its readers are not the target's.

**schemas.2** A schema record's `uses` lists the declarations it names as a type: a field's message or enum type, a map field's value type, an RPC's input and output types and an extension's type. Its `members` lists its direct members and nested declarations: a message's fields, oneof members, nested messages, enums and extensions, an enum's values and a service's RPCs. Both are unique and in bytewise order, and a declaration that has none writes an empty list.

**schemas.3** The import roots are the directories `proto.import_paths` names, each relative to the target root and inside it. Where it names none, they are the module roots of the target's `buf.yaml` files, computed as `buf` computes them, and where the target holds no `buf.yaml`, the target root and then each schema file's own directory. A schema file's import path, its `schema_file`, is its path from the first import root in that order that holds it. The file an import path names is the one of the first import root holding a file of that path, as `protoc` searches its `--proto_path`, so a schema file whose import path names another file is shadowed: it publishes no declaration, and a `schema-unparsed` note names it and the file its import path names. A directory `proto.import_paths` names that does not exist ends the run with `setup-failures.11`.

**schemas.4** A schema file that fails to parse, to link or to pass the language's own validation, or that declares an edition outside proto2, proto3, 2023, 2024 and 2026, publishes no declaration, and a `schema-unparsed` note names it with the parser's line and message. A binding that names one of its declarations finds no schema record and decides nothing. A link-time error, such as a duplicate field number or a JSON-name conflict, is such a failure though the file parses.

**schemas.5** An import no import root resolves holds every declaration whose type it would supply: the declaration is published `live`, and an `unresolved-import` note keyed `proto.import_paths` names the importing file. A type name no scope resolves holds the declaration that writes it the same way, with no note: a reader's report names no configuration, so it counts no unanswered question.

## Exemptions

**schemas.6** `schema-option`: an extension a schema option of the target names, as `[(my.opt) = ...]`, `option (my.opt) = ...` or an `import option` statement, is published `live` with every declaration its type reaches through field types, because a plugin or the runtime reads options, which no binding shows. An extension no option names is decided by its bindings.

**schemas.7** `schema-vendored`: every declaration of a schema file that is a copy of an upstream schema is published `live`. A file is a copy when its import path and its package together match one of: `google/protobuf/*.proto` with `google.protobuf`, `google/api/*.proto` with `google.api`, `google/rpc/*.proto` with `google.rpc`, `google/type/*.proto` with `google.type`, `buf/validate/validate.proto` with `buf.validate`, `validate/validate.proto` with `validate`, `gogoproto/gogo.proto` with `gogoproto`, and `protoc-gen-openapiv2/options/*.proto` with `grpc.gateway.protoc_gen_openapiv2.options`, or when its target-relative path matches a pattern of `proto.vendored_paths`. A file of another path declaring one of those packages is no copy.

**schemas.8** `schema-default-value`: the first value of an open enum, one of proto3 or of an edition whose `enum_type` feature resolves to `OPEN` for it, is published `live`, because it is the value of every unset field of the enum's type.

## Confidence of a schema finding

**schemas.9** A schema finding is `certain` when `proto.closed` is `true`, every language that binds its declaration is an application under `target.kind`, and none of `schemas.10` to `schemas.16` applies. `proto.closed` lifts every one of those caps but `schemas.16`, because it states that no outside caller, stored document or dynamic reader exists. Otherwise the finding is `probable` unless one of `schemas.10` to `schemas.16` lowers it to `possible`. The reader applies the caps its own inputs decide, `schemas.10`, `schemas.11`, `schemas.14` and `schemas.15`, when it publishes the record, and the merge applies `schemas.12` and `schemas.13` from the bindings' `cap` to a finding the reader did not publish `certain`, and `schemas.16` from the target kind of each binding's language, taking the lower class. An analyzer reads no `proto` key: it publishes every `cap` its evidence shows, and `proto.closed` is decided by the reader and the merge alone.

**schemas.10** Every service and every RPC is `possible`, because a registered server answers callers in programs the run does not analyze, and an RPC with no client in the target is the case where an outside caller is most likely.

**schemas.11** Every declaration reached, through RPC input and output types and field types, from a service that a `protoc-gen-grpc-gateway` or `protoc-gen-openapiv2` plugin of the generation configuration covers is `possible`, whether or not its RPCs carry `google.api.http`, because a gateway serves unannotated services too. The rule reads the plugin in the generation configuration, not its output.

**schemas.12** Every declaration a consumer binding of which carries `cap: descriptor-exposure` is `possible`: hand code serves the file's descriptors through server reflection or converts them to descriptor protos, and a dynamic client reads them.

**schemas.13** Every field and enum value a consumer binding of which carries `cap: persistence` is `possible`: hand code decodes the message type from ProtoJSON keeping unknown fields refused, so a stored or incoming document the bindings do not show may still name it.

**schemas.14** A field that states `json_name`, and an enum value that states its JSON name under Edition 2026, is `possible`, because the maintainer has written a JSON contract for readers outside the bindings.

**schemas.15** Every declaration whose generation the run cannot account for in full is `possible`: no generation configuration is read, a plugin that generates it has no row of [`generators.json`](../../generators.json) for its language, a plugin writes a language the run does not analyze, a `buf.yaml` declares a module `name` or a `buf.gen.yaml` input names a remote module for the file, or a `generation-config-unread` note covers the file.

**schemas.16** Every declaration a language whose target kind is `library` binds is `possible`, as a library's own published declarations are.

## Generation configuration

**schemas.17** The generation configuration is `proto.plugins` where it names a plugin, and otherwise every `buf.gen.yaml` of the target. A `buf.gen.yaml`, `buf.yaml` or `buf.work.yaml` is read in a subset of YAML: block mappings, and block and flow sequences of scalars, a quoted scalar holding any character its quotes admit. A double-quoted scalar reads YAML's escapes. An anchor, an alias, a tag, a second document, a flow mapping, an escape YAML does not define or a version the reader does not know, a `buf.work.yaml` knowing `v1` alone, makes a `generation-config-unread` note keyed `proto.plugins` naming the file and the construct, and the module the file configures is read as though no generation configuration existed: for a `buf.yaml` or a `buf.work.yaml`, every schema file below its directory, whose import roots are then that directory. An `out` directory of an analyzed language that holds no generated file makes the same note, naming the directory.

**schemas.18** A plugin is named by the basename after `protoc-gen-` of its local path, by the first element of a list after `go run` or `go tool`, by the name of a `buf.build/<org>/<name>` remote with its version dropped, or by its `protoc_builtin` language, and is matched against the plugins of [`generators.json`](../../generators.json) and the tokens `proto.plugins` admits. A name nothing matches is `other`.

## Records

**schemas.19** A schema record no exemption, suppression or withholding holds is published `dead` with a pending finding: DS1801 for a message or an enum, DS1802 for a field, a oneof member, a map field, a group field or an extension, DS1803 for an enum value, DS1804 for a service and DS1805 for an RPC, positioned at the declaration's first token in its schema file, where an inline directive binds, its class decided by `schemas.9` to `schemas.16`, and its component minted `deadset-proto/c-N` in record order. An enum value that shares its number with another value of its enum is published `live` and never reported. The merge decides the record against the bindings, as [the merge](../merge.md) states.

**schemas.20** A record whose finding's class lies below `analysis.min_confidence` is published `live`, and the run's `withheld` totals count it. The merge withholds the same way a finding the bindings lower below the minimum ([the merge](../merge.md), binding rule 3), so no finding the profile withholds is promoted.

**schemas.21** An inline directive in a schema file's comment, an entry of the ignore file or a row of the baseline that matches a record's pending finding, as [suppression](../suppression.md) states, publishes the record `live` and counts the suppression as any other.

**schemas.22** A declared edge with a `proto://` side is evaluated by the schema reader on that side, as `edges` states for an analyzer's own side: the side is `live` when its record is, and an edge whose side names no declaration is stale.

**schemas.23** The schema reader's facts document names `language` `proto`, writes `evaluated` `true` and no `generated` entry, and holds only `role: schema` records. Its report is named `deadset-proto`, as the language row's `analyzer` is, holds the reader's notes and edge evaluations, and carries a target with no `kind`, because a schema language takes no target kind.

## Roots

**schemas.24** The reader reads each entry of `roots.patterns` that its report reads, as [the merge](../merge.md) states for a `DS9004` finding: one with no language prefix, one whose prefix holds a wildcard, and one with the prefix `proto`. Every declaration whose reference an entry matches is a root, published `live` before any exemption, class or suppression decides it, and an entry that matches no declaration is a `DS9004` finding of the reader, its `symbol.ref` the entry as the configuration spells it, at line 1, column 1 of the document the `provenance` of `roots.patterns` names, as a path relative to the target, or of `deadset.json` where that document lies outside the target or no document supplied the key, so the merge reads the reader's answer for that string beside every analyzer's, at the position each analyzer gives it.
