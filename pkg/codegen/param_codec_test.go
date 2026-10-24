package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParamCodecEligibility verifies which parameters the generated code
// binds and styles itself, and which it leaves to the runtime library.
func TestParamCodecEligibility(t *testing.T) {
	const spec = `
openapi: "3.0.3"
info: {title: t, version: "1"}
paths:
  /things/{id}/{label}:
    get:
      operationId: getThing
      parameters:
        - {name: id, in: path, required: true, schema: {type: integer, format: int32}}
        - {name: label, in: path, required: true, style: label, schema: {type: integer}}
        - {name: ids, in: query, schema: {type: array, items: {type: string, format: uuid}}}
        - {name: color, in: query, schema: {$ref: '#/components/schemas/Color'}}
        - {name: on, in: query, schema: {type: string, format: date}}
        - {name: deep, in: query, style: deepObject, schema: {type: object, properties: {a: {type: string}}}}
        - {name: custom, in: query, schema: {type: string, x-go-type: MyType}}
        - {name: since, in: header, schema: {type: string, format: date-time}}
        - {name: session, in: cookie, schema: {type: array, items: {type: boolean}}}
        - {name: blob, in: query, schema: {type: string, format: byte}}
      responses:
        '200': {description: ok}
components:
  schemas:
    Color:
      type: string
      enum: [red, green]
`
	swagger := loadTestSpec(t, []byte(spec))
	_, err := Generate(swagger, Configuration{PackageName: "api", Generate: GenerateOptions{Server: true}})
	require.NoError(t, err)
	ops, err := OperationDefinitions(swagger)
	require.NoError(t, err)

	static := map[string]bool{}
	for _, p := range append(append(append([]ParameterDefinition{}, ops[0].PathParams...), ops[0].QueryParams...), append(ops[0].HeaderParams, ops[0].CookieParams...)...) {
		static[p.ParamName] = p.Codec() != nil
	}
	assert.Equal(t, map[string]bool{
		"id":      true,
		"label":   true,
		"ids":     true,
		"color":   true, // a named type over string
		"on":      true,
		"deep":    true,
		"custom":  true, // must implement encoding.TextMarshaler and TextUnmarshaler
		"since":   true,
		"session": true,
		"blob":    true,
	}, static)
}

// TestStaticParameterCodeNeedsNoRuntime verifies that a server and client
// whose parameters the codec handles don't import the runtime library.
func TestStaticParameterCodeNeedsNoRuntime(t *testing.T) {
	const spec = `
openapi: "3.0.3"
info: {title: t, version: "1"}
paths:
  /pets/{id}:
    get:
      operationId: getPet
      parameters:
        - {name: id, in: path, required: true, schema: {type: integer, format: int64}}
        - {name: tags, in: query, schema: {type: array, items: {type: string}}}
        - {name: X-Request-Id, in: header, required: true, schema: {type: string, format: uuid}}
      responses:
        '200':
          description: ok
          headers:
            X-Rate-Limit: {schema: {type: integer}}
`
	swagger := loadTestSpec(t, []byte(spec))
	code, err := Generate(swagger, Configuration{PackageName: "api", Generate: GenerateOptions{
		Server: true, Client: true, Models: true,
	}})
	require.NoError(t, err)
	assert.NotContains(t, code, `"github.com/oapi-codegen/runtime"`)
	assert.Contains(t, code, `strconv.ParseInt(r.PathValue("id"), 10, 64)`)
	assert.Contains(t, code, `pathParam0 = url.PathEscape(strconv.FormatInt(int64(id), 10))`)
}
