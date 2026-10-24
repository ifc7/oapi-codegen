package kin

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unsafe"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

// omittedFields are the kin fields the model leaves out because the
// generator never reads them.
var omittedFields = []string{
	"T.JSONSchemaDialect",
	"Info.TermsOfService",
	"Info.Contact",
	"Info.License",
	"SecurityScheme.Flows",
}

// fixture is one document the repository generates code from: the spec
// named by a go:generate directive, with the Overlay its configuration
// applies.
type fixture struct {
	name string
	spec string
	opts engine.LoadOptions
}

// generateFixtures finds every oapi-codegen go:generate directive under the
// repository's internal/test and examples modules.
func generateFixtures(t *testing.T) []fixture {
	t.Helper()
	root, err := filepath.Abs("../../..")
	require.NoError(t, err)

	var fixtures []fixture
	for _, dir := range []string{"internal/test", "examples"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanner := bufio.NewScanner(bytes.NewReader(data))
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "//go:generate ") || !strings.Contains(line, "oapi-codegen") {
					continue
				}
				fx, err := parseDirective(filepath.Dir(path), strings.Fields(line))
				if err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
				rel, _ := filepath.Rel(root, fx.spec)
				fx.name = rel
				fixtures = append(fixtures, fx)
			}
			return scanner.Err()
		})
		require.NoError(t, err)
	}
	require.NotEmpty(t, fixtures)
	return fixtures
}

func parseDirective(dir string, fields []string) (fixture, error) {
	spec := fields[len(fields)-1]
	var config string
	for i, field := range fields {
		switch {
		case field == "-config" || field == "--config":
			if i+1 < len(fields) {
				config = fields[i+1]
			}
		case strings.HasPrefix(field, "-config="), strings.HasPrefix(field, "--config="):
			_, config, _ = strings.Cut(field, "=")
		}
	}
	fx := fixture{
		spec: filepath.Join(dir, spec),
		opts: engine.LoadOptions{OverlayStrict: true},
	}
	if config == "" {
		return fx, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, config))
	if err != nil {
		return fx, err
	}
	var cfg struct {
		OutputOptions struct {
			Overlay struct {
				Path   string `yaml:"path"`
				Strict *bool  `yaml:"strict"`
			} `yaml:"overlay"`
		} `yaml:"output-options"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fx, fmt.Errorf("reading %s: %w", config, err)
	}
	if overlay := cfg.OutputOptions.Overlay; overlay.Path != "" {
		fx.opts.OverlayPath = filepath.Join(dir, overlay.Path)
		if overlay.Strict != nil {
			fx.opts.OverlayStrict = *overlay.Strict
		}
	}
	return fx, nil
}

// TestConvertFidelity converts every document the repository generates from
// and checks the model against kin's value field by field.
func TestConvertFidelity(t *testing.T) {
	for _, fx := range generateFixtures(t) {
		t.Run(fx.name, func(t *testing.T) {
			spec, err := LoadSpecWithOverlay(fx.spec, fx.opts)
			require.NoError(t, err)

			doc := Convert(spec)
			assert.Equal(t, spec.IsOpenAPI31OrLater(), doc.IsOpenAPI31OrLater())

			c := newComparison()
			c.compare("T", reflect.ValueOf(spec), reflect.ValueOf(doc))
			for _, problem := range c.problems {
				t.Error(problem)
			}
		})
	}
}

type pointerKey struct {
	typ  reflect.Type
	addr uintptr
}

// comparison walks a kin value and its model conversion side by side.
type comparison struct {
	// toModel and toKin record each pointer pair seen, so that one kin
	// object always maps to one model object and back.
	toModel  map[pointerKey]uintptr
	toKin    map[pointerKey]uintptr
	problems []string
}

func newComparison() *comparison {
	return &comparison{
		toModel: make(map[pointerKey]uintptr),
		toKin:   make(map[pointerKey]uintptr),
	}
}

func (c *comparison) fail(path, format string, args ...any) {
	c.problems = append(c.problems, path+": "+fmt.Sprintf(format, args...))
}

func (c *comparison) compare(path string, k, m reflect.Value) {
	if k.Kind() != m.Kind() {
		c.fail(path, "kind %s, model kind %s", k.Kind(), m.Kind())
		return
	}
	switch k.Kind() {
	case reflect.Pointer:
		if k.IsNil() != m.IsNil() {
			c.fail(path, "kin nil %t, model nil %t", k.IsNil(), m.IsNil())
			return
		}
		if k.IsNil() {
			return
		}
		kinKey := pointerKey{k.Type(), k.Pointer()}
		modelKey := pointerKey{m.Type(), m.Pointer()}
		if seen, ok := c.toModel[kinKey]; ok {
			if seen != m.Pointer() {
				c.fail(path, "one kin %s converted to two model objects", k.Type())
			}
			return
		}
		if seen, ok := c.toKin[modelKey]; ok && seen != k.Pointer() {
			c.fail(path, "two kin %s converted to one model object", k.Type())
			return
		}
		c.toModel[kinKey] = m.Pointer()
		c.toKin[modelKey] = k.Pointer()
		c.compare(path, k.Elem(), m.Elem())

	case reflect.Struct:
		c.compareStruct(path, k, m)

	case reflect.Map:
		c.compareMap(path, k, m, false)

	case reflect.Slice:
		if k.IsNil() != m.IsNil() {
			c.fail(path, "kin nil %t, model nil %t", k.IsNil(), m.IsNil())
			return
		}
		if k.Len() != m.Len() {
			c.fail(path, "kin length %d, model length %d", k.Len(), m.Len())
			return
		}
		for i := range k.Len() {
			c.compare(fmt.Sprintf("%s[%d]", path, i), k.Index(i), m.Index(i))
		}

	case reflect.Interface:
		if k.IsNil() != m.IsNil() {
			c.fail(path, "kin nil %t, model nil %t", k.IsNil(), m.IsNil())
			return
		}
		if k.IsNil() {
			return
		}
		if pkg := k.Elem().Type().PkgPath(); strings.Contains(pkg, "kin-openapi") {
			c.fail(path, "holds a kin-openapi value of type %s", k.Elem().Type())
		}
		if !reflect.DeepEqual(k.Interface(), m.Interface()) {
			c.fail(path, "kin %#v, model %#v", k.Interface(), m.Interface())
		}

	case reflect.Bool:
		if k.Bool() != m.Bool() {
			c.fail(path, "kin %t, model %t", k.Bool(), m.Bool())
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if k.Int() != m.Int() {
			c.fail(path, "kin %d, model %d", k.Int(), m.Int())
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if k.Uint() != m.Uint() {
			c.fail(path, "kin %d, model %d", k.Uint(), m.Uint())
		}
	case reflect.Float32, reflect.Float64:
		if k.Float() != m.Float() {
			c.fail(path, "kin %v, model %v", k.Float(), m.Float())
		}
	case reflect.String:
		if k.String() != m.String() {
			c.fail(path, "kin %q, model %q", k.String(), m.String())
		}
	default:
		c.fail(path, "unexpected kind %s", k.Kind())
	}
}

func (c *comparison) compareStruct(path string, k, m reflect.Value) {
	kt, mt := k.Type(), m.Type()
	for i := range kt.NumField() {
		field := kt.Field(i)
		fieldPath := path + "." + field.Name
		if !field.IsExported() {
			// Paths, Responses and Callback keep their entries in m. Other
			// unexported kin fields are loader bookkeeping.
			if field.Name != "m" {
				continue
			}
			mf, ok := mt.FieldByName("m")
			if !ok {
				c.fail(fieldPath, "model %s has no entries field", mt)
				continue
			}
			c.compareMap(fieldPath, unexported(k, field), unexported(m, mf), true)
			continue
		}
		mf := m.FieldByName(field.Name)
		if !mf.IsValid() {
			if !slices.Contains(omittedFields, kt.Name()+"."+field.Name) {
				c.fail(fieldPath, "model %s has no field %s", mt, field.Name)
			}
			continue
		}
		c.compare(fieldPath, k.Field(i), mf)
	}
	for i := range mt.NumField() {
		field := mt.Field(i)
		if _, ok := kt.FieldByName(field.Name); field.IsExported() && !ok {
			c.fail(path+"."+field.Name, "model field has no kin counterpart")
		}
	}
}

// unexported returns the named unexported field of v, made readable.
func unexported(v reflect.Value, field reflect.StructField) reflect.Value {
	f := v.FieldByIndex(field.Index)
	return reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
}

// compareMap compares maps by key. When emptyIsNil is set a nil map and an
// empty one are equal, as they are behind Paths, Responses and Callback.
func (c *comparison) compareMap(path string, k, m reflect.Value, emptyIsNil bool) {
	if !emptyIsNil && k.IsNil() != m.IsNil() {
		c.fail(path, "kin nil %t, model nil %t", k.IsNil(), m.IsNil())
		return
	}
	if k.Len() != m.Len() {
		c.fail(path, "kin has %d entries, model %d", k.Len(), m.Len())
		return
	}
	for _, key := range k.MapKeys() {
		value := m.MapIndex(key.Convert(m.Type().Key()))
		if !value.IsValid() {
			c.fail(path, "model has no entry %q", key)
			continue
		}
		c.compare(fmt.Sprintf("%s[%q]", path, key), k.MapIndex(key), value)
	}
}

func loadFromData(t *testing.T, data string) *openapi3.T {
	t.Helper()
	spec, err := newLoader().LoadFromData([]byte(data))
	require.NoError(t, err)
	return spec
}

func TestConvertSharesReferencedSchemas(t *testing.T) {
	doc := Convert(loadFromData(t, `
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
`))
	pet := doc.Components.Schemas["Pet"].Value
	owner := doc.Components.Schemas["Owner"].Value
	assert.Same(t, pet, owner.Properties["first"].Value)
	assert.Same(t, pet, owner.Properties["second"].Value)
	assert.Equal(t, "#/components/schemas/Pet", owner.Properties["first"].Ref)
}

func TestConvertRecursiveSchema(t *testing.T) {
	doc := Convert(loadFromData(t, `
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
`))
	node := doc.Components.Schemas["Node"].Value
	assert.Same(t, node, node.Properties["children"].Value.Items.Value)
	assert.True(t, node.Type.Is(model.TypeObject))
}

func TestConvertKeepsNilAndEmptyDistinct(t *testing.T) {
	doc := Convert(loadFromData(t, `
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
`))
	empty := doc.Components.Schemas["Empty"].Value
	unset := doc.Components.Schemas["Unset"].Value
	assert.NotNil(t, empty.Required)
	assert.Empty(t, empty.Required)
	assert.Nil(t, unset.Required)
}

func TestConvertExternalReference(t *testing.T) {
	spec, err := LoadSpec("../../../internal/test/references/external/response_refs/pkg1.yaml")
	require.NoError(t, err)
	doc := Convert(spec)

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

func TestEngineLoad(t *testing.T) {
	doc, err := Engine{}.Load("../../../internal/test/references/external/response_refs/pkg1.yaml", engine.LoadOptions{})
	require.NoError(t, err)
	assert.NotZero(t, doc.Paths.Len())

	_, err = Engine{}.Load("does-not-exist.yaml", engine.LoadOptions{})
	assert.ErrorContains(t, err, "failed to load OpenAPI specification")
}
