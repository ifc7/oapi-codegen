package libopenapi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ifc7/oapi-codegen/pkg/engine"
	"github.com/ifc7/oapi-codegen/pkg/model"
)

func loadYAML(t *testing.T, data string) *model.Document {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spec.yaml")
	require.NoError(t, os.WriteFile(path, []byte(data), 0o644))
	doc, err := Engine{}.Load(path, engine.LoadOptions{})
	require.NoError(t, err)
	return doc
}

func TestConvertSharesReferencedSchemas(t *testing.T) {
	doc := loadYAML(t, `
openapi: 3.0.3
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Pet:
      type: object
      properties:
        name: {type: string}
    Owner:
      type: object
      properties:
        first: {$ref: '#/components/schemas/Pet'}
        second: {$ref: '#/components/schemas/Pet'}
`)
	pet := doc.Components.Schemas["Pet"].Value
	owner := doc.Components.Schemas["Owner"].Value
	assert.Same(t, pet, owner.Properties["first"].Value)
	assert.Same(t, pet, owner.Properties["second"].Value)
	assert.Equal(t, "#/components/schemas/Pet", owner.Properties["first"].Ref)
}

func TestConvertRecursiveSchema(t *testing.T) {
	doc := loadYAML(t, `
openapi: 3.0.3
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Node:
      type: object
      properties:
        children:
          type: array
          items: {$ref: '#/components/schemas/Node'}
`)
	node := doc.Components.Schemas["Node"].Value
	assert.Same(t, node, node.Properties["children"].Value.Items.Value)
	assert.True(t, node.Type.Is(model.TypeObject))
}

func TestConvertKeepsNilAndEmptyDistinct(t *testing.T) {
	doc := loadYAML(t, `
openapi: 3.1.0
info: {title: t, version: "1"}
paths: {}
components:
  schemas:
    Empty:
      type: object
      required: []
    Unset:
      type: object
`)
	empty := doc.Components.Schemas["Empty"].Value
	unset := doc.Components.Schemas["Unset"].Value
	assert.NotNil(t, empty.Required)
	assert.Empty(t, empty.Required)
	assert.Nil(t, unset.Required)
}

func TestConvertExternalReference(t *testing.T) {
	doc, err := Engine{}.Load("../../../internal/test/references/external/response_refs/pkg1.yaml", engine.LoadOptions{})
	require.NoError(t, err)

	var refs []string
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			for _, response := range op.Responses.Map() {
				if response.Ref != "" {
					refs = append(refs, response.Ref)
				}
			}
		}
	}
	assert.Equal(t, []string{"pkg2.yaml#/components/responses/ResponseWithReference"}, refs)
}

func TestConvertNullableStaysNullable(t *testing.T) {
	doc, err := Engine{}.Load("../../../internal/test/schemas/nullable/spec_30.yaml", engine.LoadOptions{})
	require.NoError(t, err)
	pet := doc.Components.Schemas["Pet"].Value
	nickname := pet.Properties["nickname"].Value
	assert.True(t, nickname.Nullable)
	assert.True(t, nickname.Type.Is("string"))
	assert.False(t, nickname.Type.Includes("null"))
}

func TestConvertExtensionNumber(t *testing.T) {
	doc := loadYAML(t, `
openapi: 3.0.3
info:
  title: t
  version: "1"
  x-count: 2
  x-ratio: 1.5
paths: {}
`)
	assert.Equal(t, float64(2), doc.Info.Extensions["x-count"])
	assert.Equal(t, float64(1.5), doc.Info.Extensions["x-ratio"])
}

func TestConvertHeaderExampleNotHoistedFromSchema(t *testing.T) {
	doc := loadYAML(t, `
openapi: 3.0.3
info:
  title: t
  version: "1"
paths:
  /x:
    get:
      responses:
        "200":
          description: ok
          headers:
            Own:
              example: from-header
              schema:
                type: string
                example: from-schema
            Nested:
              schema:
                type: string
                example: from-schema
`)
	headers := doc.Paths.Value("/x").Get.Responses.Value("200").Value.Headers
	assert.Equal(t, "from-header", headers["Own"].Value.Example)
	assert.Nil(t, headers["Nested"].Value.Example)
	assert.Equal(t, "from-schema", headers["Nested"].Value.Schema.Value.Example)
}

func TestConvertPathKeyOrder(t *testing.T) {
	doc := loadYAML(t, `
openapi: 3.0.3
info: {title: t, version: "1"}
paths:
  /c:
    get:
      responses:
        '200': {description: c}
  /a:
    get:
      responses:
        '200': {description: a}
  /b:
    get:
      responses:
        '200': {description: b}
`)
	c := doc.Paths.Find("/c").Origin.Key.Line
	a := doc.Paths.Find("/a").Origin.Key.Line
	b := doc.Paths.Find("/b").Origin.Key.Line
	assert.Less(t, c, a)
	assert.Less(t, a, b)
}
