package libopenapi

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/enginetest"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/kin"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
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

// TestConvertParity checks that both engines produce the same model for every
// spec the repository generates. Origin is skipped: line numbers only have to
// keep declaration order, which TestConvertPathKeyOrder covers.
func TestConvertParity(t *testing.T) {
	for _, fx := range enginetest.GenerateFixtures(t) {
		t.Run(fx.Name, func(t *testing.T) {
			fromKin, err := kin.Engine{}.Load(fx.Spec, fx.Opts)
			require.NoError(t, err)
			fromLib, err := Engine{}.Load(fx.Spec, fx.Opts)
			require.NoError(t, err)

			c := &comparison{seen: make(map[[2]uintptr]struct{})}
			c.compare("Document", reflect.ValueOf(fromKin), reflect.ValueOf(fromLib))
			for _, problem := range c.problems {
				t.Error(problem)
			}
		})
	}
}

type comparison struct {
	seen     map[[2]uintptr]struct{}
	problems []string
}

func (c *comparison) fail(path, format string, args ...any) {
	if len(c.problems) >= 12 {
		return
	}
	c.problems = append(c.problems, path+": "+fmt.Sprintf(format, args...))
}

func (c *comparison) compare(path string, a, b reflect.Value) {
	if !a.IsValid() || !b.IsValid() {
		if a.IsValid() != b.IsValid() {
			c.fail(path, "kin valid %t, libopenapi valid %t", a.IsValid(), b.IsValid())
		}
		return
	}
	if a.Kind() == reflect.Interface || b.Kind() == reflect.Interface {
		if a.Kind() != b.Kind() || a.IsNil() != b.IsNil() {
			c.fail(path, "kin %T %#v, libopenapi %T %#v", valueOf(a), valueOf(a), valueOf(b), valueOf(b))
			return
		}
		if a.IsNil() {
			return
		}
		c.compare(path, a.Elem(), b.Elem())
		return
	}
	if a.Kind() != b.Kind() {
		c.fail(path, "kin %s %#v, libopenapi %s %#v", a.Kind(), valueOf(a), b.Kind(), valueOf(b))
		return
	}
	switch a.Kind() {
	case reflect.Pointer:
		if a.IsNil() != b.IsNil() {
			c.fail(path, "kin nil %t, libopenapi nil %t", a.IsNil(), b.IsNil())
			return
		}
		if a.IsNil() {
			return
		}
		key := [2]uintptr{a.Pointer(), b.Pointer()}
		if _, ok := c.seen[key]; ok {
			return
		}
		c.seen[key] = struct{}{}
		c.compare(path, a.Elem(), b.Elem())
	case reflect.Struct:
		c.compareStruct(path, a, b)
	case reflect.Map:
		c.compareMap(path, a, b, false)
	case reflect.Slice:
		if a.IsNil() != b.IsNil() {
			c.fail(path, "kin nil %t, libopenapi nil %t", a.IsNil(), b.IsNil())
			return
		}
		if a.Len() != b.Len() {
			c.fail(path, "kin length %d, libopenapi length %d", a.Len(), b.Len())
			return
		}
		for i := range a.Len() {
			c.compare(fmt.Sprintf("%s[%d]", path, i), a.Index(i), b.Index(i))
		}
	case reflect.Bool:
		if a.Bool() != b.Bool() {
			c.fail(path, "kin %t, libopenapi %t", a.Bool(), b.Bool())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if a.Int() != b.Int() {
			c.fail(path, "kin %d, libopenapi %d", a.Int(), b.Int())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if a.Uint() != b.Uint() {
			c.fail(path, "kin %d, libopenapi %d", a.Uint(), b.Uint())
		}
	case reflect.Float32, reflect.Float64:
		if a.Float() != b.Float() {
			c.fail(path, "kin %v, libopenapi %v", a.Float(), b.Float())
		}
	case reflect.String:
		if a.String() != b.String() {
			c.fail(path, "kin %q, libopenapi %q", a.String(), b.String())
		}
	default:
		c.fail(path, "unexpected kind %s", a.Kind())
	}
}

func (c *comparison) compareStruct(path string, a, b reflect.Value) {
	at, bt := a.Type(), b.Type()
	for i := range at.NumField() {
		field := at.Field(i)
		if field.Name == "Origin" {
			continue
		}
		fieldPath := path + "." + field.Name
		if !field.IsExported() {
			if field.Name != "m" {
				continue
			}
			bf, ok := bt.FieldByName("m")
			if !ok {
				c.fail(fieldPath, "libopenapi document has no entries field")
				continue
			}
			c.compareMap(fieldPath, unexported(a, field), unexported(b, bf), true)
			continue
		}
		bf := b.FieldByName(field.Name)
		if !bf.IsValid() {
			c.fail(fieldPath, "missing on libopenapi document")
			continue
		}
		c.compare(fieldPath, a.Field(i), bf)
	}
}

func (c *comparison) compareMap(path string, a, b reflect.Value, emptyIsNil bool) {
	if !a.IsValid() || !b.IsValid() {
		c.fail(path, "kin valid %t, libopenapi valid %t", a.IsValid(), b.IsValid())
		return
	}
	aEmpty := a.IsNil() || a.Len() == 0
	bEmpty := b.IsNil() || b.Len() == 0
	if emptyIsNil && aEmpty && bEmpty {
		return
	}
	if a.IsNil() != b.IsNil() {
		c.fail(path, "kin nil %t (%d), libopenapi nil %t (%d)", a.IsNil(), a.Len(), b.IsNil(), b.Len())
		return
	}
	if a.Len() != b.Len() {
		c.fail(path, "kin has %d entries, libopenapi %d", a.Len(), b.Len())
		return
	}
	for _, key := range a.MapKeys() {
		value := b.MapIndex(key)
		if !value.IsValid() {
			c.fail(path, "libopenapi has no entry %q", key)
			continue
		}
		c.compare(fmt.Sprintf("%s[%q]", path, key), a.MapIndex(key), value)
	}
}

func unexported(v reflect.Value, field reflect.StructField) reflect.Value {
	f := v.FieldByIndex(field.Index)
	return reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
}

func valueOf(v reflect.Value) any {
	if !v.IsValid() || (v.Kind() == reflect.Interface && v.IsNil()) {
		return nil
	}
	return v.Interface()
}
