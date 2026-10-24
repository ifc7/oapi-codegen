# Configuration options

`oapi-codegen` reads a YAML file (`--config`). Unknown keys are rejected. If `package` is omitted, the CLI infers it from the output directory or the spec filename. If the `generate` block is omitted, the generator emits a net/http server and models. With no config file at all, it emits a server, a client, and models.

## Top level

| Key | What it does |
| --- | --- |
| `package` | Go package name for the generated file. |
| `output` | Path of the generated file. The `-o` flag is used when this is omitted. |
| `generate` | Which outputs to emit. See below. |
| `output-options` | Filters and tweaks applied to the generated code. See below. |
| `import-mapping` | Maps a `$ref`'d document path or URL to a Go import path, so types from that document come from another package. A value of `-` means the current package. Keys must be document paths, not JSON pointers. |
| `additional-imports` | Extra imports to add. Each entry has `package` and an optional `alias`. |

## `generate`

Each flag defaults to false when the block is present. An omitted block means `server` and `models`.

| Key | What it does |
| --- | --- |
| `server` | Generate the net/http server interface, request binders, and handler glue. |
| `client` | Generate a client and, by default, a `ClientWithResponses` wrapper. |
| `models` | Generate type definitions for schemas, parameters, request bodies, and responses. |

## `output-options`

### What to include

| Key | What it does |
| --- | --- |
| `include-tags` | Generate only operations that have one of these tags. Ignored when empty. |
| `exclude-tags` | Skip operations that have one of these tags. Ignored when empty. |
| `include-operation-ids` | Generate only these operation IDs. Ignored when empty. |
| `exclude-operation-ids` | Skip these operation IDs. Ignored when empty. |
| `exclude-schemas` | Skip schemas with these names. Ignored when empty. |
| `overlay` | Apply an [OpenAPI Overlay](https://github.com/OAI/Overlay-Specification) before generation. `path` is the overlay file. `strict` (default true) reports overlay actions that match nothing. |

### Names and types

| Key | What it does |
| --- | --- |
| `name-normalizer` | How spec names become Go names. `ToCamelCase` (default), `ToCamelCaseWithDigits`, or `ToCamelCaseWithInitialisms`. |
| `additional-initialisms` | Extra initialisms for `ToCamelCaseWithInitialisms`. Requires that normalizer. |
| `always-prefix-enum-values` | Prefix every enum constant with its type name. By default the prefix is added only to avoid collisions. |
| `client-type-name` | Name of the generated client type. |
| `response-type-suffix` | Suffix appended to generated response type names. |
| `resolve-type-name-collisions` | Rename types that collide across component sections (and with client response wrappers) instead of failing. |
| `generate-types-for-anonymous-schemas` | Give inline object schemas a path-derived named type. The names are declared only when `generate.models` is on. A per-schema `x-go-type-name` wins. |
| `type-mapping` | Override the Go type for an OpenAPI type and format. Keys are `integer`, `number`, `boolean`, and `string`. Each has a `default` and a `formats` map. A mapping is `{ type, import? }` and is merged on top of the built-in defaults. |
| `nullable-type` | Generate a nullable wrapper for `nullable` fields. |
| `disable-type-aliases-for-type` | Do not emit a type alias for these OpenAPI types. Only `array` is supported. |
| `prefer-skip-optional-pointer` | Represent optional fields as values instead of pointers. Same as `x-go-type-skip-optional-pointer` on every field. |
| `prefer-skip-optional-pointer-with-omitzero` | With the option above, add `,omitzero` to the JSON tag. Requires Go 1.24. No effect on its own. |
| `prefer-skip-optional-pointer-on-container-types` | Do not use a pointer for optional slices and maps. |
| `allow-unexported-struct-field-names` | Allow `x-go-name` to produce unexported fields. |

### Enums and unions

| Key | What it does |
| --- | --- |
| `skip-enum-validate` | Do not generate `Valid()` on enum types. |
| `skip-enum-via-oneof` | Do not treat an OpenAPI 3.1 `oneOf` of `const` + `title` members as an enum. It is generated as a union instead. |
| `lenient-union-accessors` | A union variant with `additionalProperties: false` ignores unknown properties in `As*`, the same as other variants. By default those properties are an error. |

### Client and server shape

| Key | What it does |
| --- | --- |
| `client-response-bytes-function` | Add `Bytes()` on `ClientWithResponses` response objects. |
| `skip-client-response-content-type` | Do not generate `ContentType()` on those response objects. |
| `skip-response-body-getters` | Do not generate `GetBody()` and typed body getters on those response objects. |
| `streaming-content-types` | Extra regexes matched against a response `Content-Type`. Matches use a flush-per-chunk server path. Merged with `text/event-stream`, `application/jsonl`, and `application/x-ndjson`. |
| `content-types` | Map a short name to media-type regexes. Matches are used in generated type names (`FindPetsJSONRequestBody`) and get models. Merged by replacing built-in keys (`JSON`, `Formdata`, `Multipart`, `Text`). An empty list disables a key. Typed client response fields are still only generated for media types the client can deserialize. |
| `struct-tags` | Struct tags to emit. Each entry is `name` plus a Go text/template. Variables are `.FieldName`, `.IsOptional`, `.OmitEmpty`, `.OmitZero`, and `.NeedsFormTag`. An empty render suppresses that tag. Merged by name onto the default `json` and `form` tags, then emitted in alphabetical order. |
| `user-templates` | Replace a built-in template with a file. The key is the template name and the value is the file path. |

### Generation mechanics

| Key | What it does |
| --- | --- |
| `skip-fmt` | Skip `goimports` on the generated file. |
| `skip-prune` | Keep unused components instead of dropping them. |
| `skip-format-types` | Do not declare `OpenAPIDate`, `OpenAPIDuration`, `OpenAPIEmail`, and `OpenAPIFile`. Use this when another file in the same package already declares them. |
