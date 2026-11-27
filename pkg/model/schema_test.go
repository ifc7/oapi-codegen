package model

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTypesMethods(t *testing.T) {
	t.Parallel()

	var nilTypes *Types
	require.False(t, nilTypes.Is("string"))
	require.Nil(t, nilTypes.Slice())
	require.False(t, nilTypes.Includes("string"))
	require.True(t, nilTypes.Permits("anything"))

	one := &Types{"string"}
	require.True(t, one.Is("string"))
	require.False(t, one.Is("number"))
	require.Equal(t, []string{"string"}, one.Slice())
	require.True(t, one.Includes("string"))
	require.True(t, one.Permits("string"))
	require.False(t, one.Permits("number"))

	many := &Types{"string", "null"}
	require.False(t, many.Is("string"))
	require.Equal(t, []string{"string", "null"}, many.Slice())
	require.True(t, many.Includes("null"))
	require.False(t, many.Includes("number"))
	require.True(t, many.Permits("null"))
	require.False(t, many.Permits("number"))
}

func TestAdditionalPropertiesHelpers(t *testing.T) {
	t.Parallel()

	schema := &Schema{}
	require.Same(t, schema, schema.WithAnyAdditionalProperties())
	require.NotNil(t, schema.AdditionalProperties.Has)
	require.True(t, *schema.AdditionalProperties.Has)
	require.Nil(t, schema.AdditionalProperties.Schema)

	require.Same(t, schema, schema.WithoutAdditionalProperties())
	require.NotNil(t, schema.AdditionalProperties.Has)
	require.False(t, *schema.AdditionalProperties.Has)
	require.Nil(t, schema.AdditionalProperties.Schema)
}

func TestNewSchemaRef(t *testing.T) {
	t.Parallel()

	value := &Schema{Title: "name"}
	ref := NewSchemaRef("#/components/schemas/Name", value)
	require.Equal(t, "#/components/schemas/Name", ref.Ref)
	require.Same(t, value, ref.Value)
}

// TestSchemaFields guards the reflection the allOf merger uses. Schema's
// fields must be exactly these, in this order, with these json keywords (the
// name before the first comma). A field added without updating the merger
// would be reported as a keyword it cannot fold, so adding one means
// extending this list and the merger together.
func TestSchemaFields(t *testing.T) {
	t.Parallel()

	want := []struct{ name, keyword string }{
		{"Extensions", "-"},
		{"Origin", "-"},
		{"OneOf", "oneOf"},
		{"AnyOf", "anyOf"},
		{"AllOf", "allOf"},
		{"Not", "not"},
		{"Type", "type"},
		{"Title", "title"},
		{"Format", "format"},
		{"Description", "description"},
		{"Enum", "enum"},
		{"Default", "default"},
		{"Example", "example"},
		{"ExternalDocs", "externalDocs"},
		{"UniqueItems", "uniqueItems"},
		{"ExclusiveMin", "exclusiveMinimum"},
		{"ExclusiveMax", "exclusiveMaximum"},
		{"Nullable", "nullable"},
		{"ReadOnly", "readOnly"},
		{"WriteOnly", "writeOnly"},
		{"AllowEmptyValue", "allowEmptyValue"},
		{"Deprecated", "deprecated"},
		{"XML", "xml"},
		{"Min", "minimum"},
		{"Max", "maximum"},
		{"MultipleOf", "multipleOf"},
		{"MinLength", "minLength"},
		{"MaxLength", "maxLength"},
		{"Pattern", "pattern"},
		{"MinItems", "minItems"},
		{"MaxItems", "maxItems"},
		{"Items", "items"},
		{"Required", "required"},
		{"Properties", "properties"},
		{"MinProps", "minProperties"},
		{"MaxProps", "maxProperties"},
		{"AdditionalProperties", "additionalProperties"},
		{"Discriminator", "discriminator"},
		{"Const", "const"},
		{"Examples", "examples"},
		{"PrefixItems", "prefixItems"},
		{"Contains", "contains"},
		{"MinContains", "minContains"},
		{"MaxContains", "maxContains"},
		{"PatternProperties", "patternProperties"},
		{"DependentSchemas", "dependentSchemas"},
		{"PropertyNames", "propertyNames"},
		{"UnevaluatedItems", "unevaluatedItems"},
		{"UnevaluatedProperties", "unevaluatedProperties"},
		{"If", "if"},
		{"Then", "then"},
		{"Else", "else"},
		{"DependentRequired", "dependentRequired"},
		{"Always", "-"},
		{"Defs", "$defs"},
		{"SchemaDialect", "$schema"},
		{"Comment", "$comment"},
		{"SchemaID", "$id"},
		{"Anchor", "$anchor"},
		{"DynamicRef", "$dynamicRef"},
		{"DynamicAnchor", "$dynamicAnchor"},
		{"ContentMediaType", "contentMediaType"},
		{"ContentEncoding", "contentEncoding"},
		{"ContentSchema", "contentSchema"},
	}

	fields := reflect.TypeFor[Schema]()
	require.Equal(t, len(want), fields.NumField())
	for i, w := range want {
		field := fields.Field(i)
		require.Equal(t, w.name, field.Name, "field %d", i)
		require.Equal(t, w.keyword, jsonKeyword(field), "json keyword of %s", w.name)
	}
}

func jsonKeyword(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	return name
}
