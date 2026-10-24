package codegen

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatMapping_Resolve(t *testing.T) {
	fm := FormatMapping{
		Default: SimpleTypeSpec{Type: "int"},
		Formats: map[string]SimpleTypeSpec{
			"int32": {Type: "int32"},
			"int64": {Type: "int64"},
		},
	}

	assert.Equal(t, "int", fm.Resolve("").Type)
	assert.Equal(t, "int32", fm.Resolve("int32").Type)
	assert.Equal(t, "int64", fm.Resolve("int64").Type)
	assert.Equal(t, "int", fm.Resolve("unknown-format").Type)
}

func TestTypeMapping_Merge(t *testing.T) {
	base := DefaultTypeMapping

	user := TypeMapping{
		Integer: FormatMapping{
			Default: SimpleTypeSpec{Type: "int64"},
		},
		String: FormatMapping{
			Formats: map[string]SimpleTypeSpec{
				"date-time": {Type: "civil.DateTime", Import: "cloud.google.com/go/civil"},
			},
		},
	}

	merged := base.Merge(user)

	// Integer default overridden
	assert.Equal(t, "int64", merged.Integer.Default.Type)
	// Integer formats still inherited from base
	assert.Equal(t, "int32", merged.Integer.Formats["int32"].Type)

	// String date-time overridden
	assert.Equal(t, "civil.DateTime", merged.String.Formats["date-time"].Type)
	assert.Equal(t, "cloud.google.com/go/civil", merged.String.Formats["date-time"].Import)
	// String default still inherited from base
	assert.Equal(t, "string", merged.String.Default.Type)
	// Other string formats still inherited
	assert.Equal(t, "uuid.UUID", merged.String.Formats["uuid"].Type)

	// Number and Boolean unchanged
	assert.Equal(t, "float32", merged.Number.Default.Type)
	assert.Equal(t, "bool", merged.Boolean.Default.Type)
}

func TestDefaultTypeMapping_Completeness(t *testing.T) {
	// Verify all the default mappings match what was previously hardcoded
	dm := DefaultTypeMapping

	// Integer
	assert.Equal(t, "int", dm.Integer.Resolve("").Type)
	assert.Equal(t, "int32", dm.Integer.Resolve("int32").Type)
	assert.Equal(t, "int64", dm.Integer.Resolve("int64").Type)
	assert.Equal(t, "uint32", dm.Integer.Resolve("uint32").Type)
	assert.Equal(t, "int", dm.Integer.Resolve("unknown").Type)

	// Number
	assert.Equal(t, "float32", dm.Number.Resolve("").Type)
	assert.Equal(t, "float32", dm.Number.Resolve("float").Type)
	assert.Equal(t, "float64", dm.Number.Resolve("double").Type)
	assert.Equal(t, "float32", dm.Number.Resolve("unknown").Type)

	// Boolean
	assert.Equal(t, "bool", dm.Boolean.Resolve("").Type)

	// String
	assert.Equal(t, "string", dm.String.Resolve("").Type)
	assert.Equal(t, "[]byte", dm.String.Resolve("byte").Type)
	assert.Equal(t, "OpenAPIEmail", dm.String.Resolve("email").Type)
	assert.Equal(t, "OpenAPIDate", dm.String.Resolve("date").Type)
	assert.Equal(t, "time.Time", dm.String.Resolve("date-time").Type)
	assert.Equal(t, "OpenAPIDuration", dm.String.Resolve("duration").Type)
	assert.Equal(t, "json.RawMessage", dm.String.Resolve("json").Type)
	assert.Equal(t, "uuid.UUID", dm.String.Resolve("uuid").Type)
	assert.Equal(t, "OpenAPIFile", dm.String.Resolve("binary").Type)
	assert.Equal(t, "string", dm.String.Resolve("unknown").Type)
}

// TestDurationFormatMapping verifies that `type: string, format: duration`
// generates the OpenAPIDuration type, declared in the generated package, by default, and that the
// historical plain-string behavior is restored by explicitly mapping the
// format back to string via type-mapping.
// Please see https://github.com/oapi-codegen/oapi-codegen/issues/2456
func TestDurationFormatMapping(t *testing.T) {
	const spec = `
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Durations
paths: {}
components:
  schemas:
    RetryPolicy:
      type: object
      required: [backoff]
      properties:
        backoff:
          type: string
          format: duration
`
	loader := openapi3.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)

	opts := Configuration{
		PackageName:   "api",
		Generate:      GenerateOptions{Models: true},
		OutputOptions: OutputOptions{SkipPrune: true},
	}

	code, err := Generate(swagger, opts)
	require.NoError(t, err)
	assert.Contains(t, code, "Backoff OpenAPIDuration `json:\"backoff\"`")
	assert.Contains(t, code, "type OpenAPIDuration struct {")
	assert.NotContains(t, code, "github.com/oapi-codegen/runtime")

	// The pre-duration behavior is a plain string; restore it by mapping
	// the format back explicitly.
	opts.OutputOptions.TypeMapping = &TypeMapping{
		String: FormatMapping{
			Formats: map[string]SimpleTypeSpec{
				"duration": {Type: "string"},
			},
		},
	}
	code, err = Generate(swagger, opts)
	require.NoError(t, err)
	assert.Contains(t, code, "Backoff string `json:\"backoff\"`")
	assert.NotContains(t, code, "OpenAPIDuration", "an unused format type isn't declared")
}

// TestTypeMappingImport verifies that a type mapping's import is emitted with
// the generated code rather than left for goimports to guess, for both the
// default uuid mapping and a user-specified one.
func TestTypeMappingImport(t *testing.T) {
	const spec = `
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Imports
paths: {}
components:
  schemas:
    Event:
      type: object
      required: [id, at]
      properties:
        id:
          type: string
          format: uuid
        at:
          type: string
          format: date-time
`
	loader := openapi3.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)

	opts := Configuration{
		PackageName:   "api",
		Generate:      GenerateOptions{Models: true},
		OutputOptions: OutputOptions{SkipPrune: true},
	}
	code, err := Generate(swagger, opts)
	require.NoError(t, err)
	assert.Contains(t, code, "Id uuid.UUID `json:\"id\"`")
	assert.Contains(t, code, `"github.com/google/uuid"`)

	opts.OutputOptions.TypeMapping = &TypeMapping{
		String: FormatMapping{
			Formats: map[string]SimpleTypeSpec{
				"date-time": {Type: "civil.DateTime", Import: "cloud.google.com/go/civil"},
			},
		},
	}
	code, err = Generate(swagger, opts)
	require.NoError(t, err)
	assert.Contains(t, code, "At civil.DateTime `json:\"at\"`")
	assert.Contains(t, code, `"cloud.google.com/go/civil"`)
}

// TestSkipFormatTypes verifies that skip-format-types leaves declaring the
// format types to another file in the package while still using them.
func TestSkipFormatTypes(t *testing.T) {
	const spec = `
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Dates
paths: {}
components:
  schemas:
    Holiday:
      type: object
      required: [on]
      properties:
        on:
          type: string
          format: date
`
	loader := openapi3.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)

	opts := Configuration{
		PackageName:   "api",
		Generate:      GenerateOptions{Models: true},
		OutputOptions: OutputOptions{SkipPrune: true},
	}
	code, err := Generate(swagger, opts)
	require.NoError(t, err)
	assert.Contains(t, code, "type OpenAPIDate struct {")

	opts.OutputOptions.SkipFormatTypes = true
	code, err = Generate(swagger, opts)
	require.NoError(t, err)
	assert.Contains(t, code, "On OpenAPIDate `json:\"on\"`")
	assert.NotContains(t, code, "type OpenAPIDate struct {")
}

// TestParameterNamedAfterMappedPackage verifies that a parameter named after a
// package the type mapping uses doesn't shadow it: with two uuid path
// parameters, `var uuid uuid.UUID` would leave the second's type undefined.
func TestParameterNamedAfterMappedPackage(t *testing.T) {
	const spec = `
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Shadowing
paths:
  /things/{uuid}/parts/{partId}:
    get:
      operationId: getPart
      parameters:
        - {name: uuid, in: path, required: true, schema: {type: string, format: uuid}}
        - {name: partId, in: path, required: true, schema: {type: string, format: uuid}}
      responses:
        '200': {description: ok}
`
	loader := openapi3.NewLoader()
	swagger, err := loader.LoadFromData([]byte(spec))
	require.NoError(t, err)

	code, err := Generate(swagger, Configuration{
		PackageName: "api",
		Generate:    GenerateOptions{Server: true},
	})
	require.NoError(t, err)
	assert.Contains(t, code, "var pUuid uuid.UUID")
	assert.NotContains(t, code, "var uuid uuid.UUID")
}
