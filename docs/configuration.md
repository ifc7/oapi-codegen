# Configuration Reference

`oapi-codegen` is configured using a YAML configuration file.

This page is a structural overview. The authoritative, always-current reference for every setting is:

- the GoDoc for [`codegen.Configuration`](https://pkg.go.dev/github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen#Configuration), which documents each field alongside its YAML key
- [the JSON Schema](../configuration-schema.json), which gives your editor autocompletion, hover documentation and validation via the Language Server Protocol (LSP)

To get the IDE experience, add this line to the top of your configuration file:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/oapi-codegen/oapi-codegen/v2.8.0/configuration-schema.json
package: api
# ...
```

# Structure at a glance

Every setting is shown below with its default value. Each section links to the GoDoc describing its settings in detail.

<pre>
# Required: Go package name
package: api

# The OpenAPI library that loads the spec: kin or libopenapi.
# The OAPI_CODEGEN_ENGINE environment variable overrides it.
engine: kin

# What to generate.
# If the `generate` block is omitted entirely, it defaults to generating
# a net/http server with models.
# See <a href="https://pkg.go.dev/github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen#GenerateOptions">GenerateOptions</a>
generate:
  server: false
  client: false
  models: false

# Output modification options
# See <a href="https://pkg.go.dev/github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen#OutputOptions">OutputOptions</a>
output-options:
  include-tags: []
  exclude-tags: []
  include-operation-ids: []
  exclude-operation-ids: []
  exclude-schemas: []
  skip-fmt: false
  skip-prune: false
  # Don't declare the types that string formats map to by default (OpenAPIDate,
  # OpenAPIDuration, OpenAPIEmail, OpenAPIFile); set this when another
  # generated file in the same package already declares them.
  skip-format-types: false
  # Always prefix enum values with their type name, rather than only when
  # they would conflict.
  always-prefix-enum-values: false
  # Allow struct fields that x-go-name names to be unexported; see
  # <a href="extensions.md#x-oapi-codegen-only-honour-go-name">extensions.md</a>
  allow-unexported-struct-field-names: false
  name-normalizer: ""
  initialism-overrides: false
  additional-initialisms: []
  response-type-suffix: ""
  client-type-name: ""
  nullable-type: false
  disable-type-aliases-for-type: []
  resolve-type-name-collisions: false
  prefer-skip-optional-pointer: false
  prefer-skip-optional-pointer-with-omitzero: false
  prefer-skip-optional-pointer-on-container-types: false
  skip-enum-validate: false
  skip-enum-via-oneof: false
  lenient-union-accessors: false   # see <a href="schema-merging.md#variants-with-additionalproperties-false">schema-merging.md</a>
  generate-types-for-anonymous-schemas: false
  # How OpenAPI type/format combinations map to Go types; user-specified
  # mappings are merged on top of these defaults.
  type-mapping:
    integer:
      default:
        type: int
      formats:
        int:    { type: int }
        int8:   { type: int8 }
        int16:  { type: int16 }
        int32:  { type: int32 }
        int64:  { type: int64 }
        uint:   { type: uint }
        uint8:  { type: uint8 }
        uint16: { type: uint16 }
        uint32: { type: uint32 }
        uint64: { type: uint64 }
    number:
      default:
        type: float32
      formats:
        float:  { type: float32 }
        double: { type: float64 }
    boolean:
      default:
        type: bool
    string:
      default:
        type: string
      formats:
        byte:      { type: "[]byte" }
        email:     { type: OpenAPIEmail }
        date:      { type: OpenAPIDate }
        date-time: { type: time.Time, import: time }
        json:      { type: json.RawMessage, import: encoding/json }
        duration:  { type: OpenAPIDuration }
        uuid:      { type: uuid.UUID, import: github.com/google/uuid }
        binary:    { type: OpenAPIFile }
  # Configure generated struct tags. Entries are merged by name on top of the
  # defaults shown below, so redefining `json` replaces its template while new
  # names add tags. Values are Go text/templates over the variables
  # .FieldName, .IsOptional, .OmitEmpty, .OmitZero and .NeedsFormTag; a
  # template that renders empty suppresses the tag on that field. Tags are
  # emitted in alphabetical order. Per-field x-oapi-codegen-extra-tags and
  # x-go-json-ignore are applied on top of the rendered tags.
  struct-tags:
    tags:
      - name: json
        template: '{{.FieldName}}{{if .OmitEmpty}},omitempty{{end}}{{if .OmitZero}},omitzero{{end}}'
      - name: form
        template: '{{if .NeedsFormTag}}{{.FieldName}}{{if .OmitEmpty}},omitempty{{end}}{{end}}'
      # Additional tags can be added, e.g.:
      # - name: db
      #   template: '{{.FieldName}}'
  client-response-bytes-function: false
  skip-client-response-content-type: false
  skip-response-body-getters: false
  streaming-content-types: []
  # Short names for media types, used in generated type names. Keys are the
  # tag spliced into type names (e.g. the JSON in FindPetsJSONRequestBody),
  # values are regex patterns matched against request/response media types.
  # Matching media types get models generated and named with the short name.
  # User entries are merged on top of these defaults by key replacement
  # (an empty list disables a key); unmatched media types keep the
  # remaining built-in behavior.
  content-types:
    JSON:      ['^application/json$']
    Formdata:  ['^application/x-www-form-urlencoded$']
    Multipart: ['^multipart/']
    Text:      ['^text/plain$']
  user-templates: {}
  # OpenAPI Overlay applied to the spec before generation
  overlay:
    path: ""
    strict: true

# External reference to Go package mapping, used when
# <a href="../README.md#import-mapping">splitting specs across multiple packages</a>
import-mapping: {}

# Additional Go imports to add to the generated code
additional-imports: []
</pre>
