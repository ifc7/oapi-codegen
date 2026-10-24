package model

import (
	"reflect"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
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

func TestTypesMethodsMatchKin(t *testing.T) {
	t.Parallel()

	cases := [][]string{nil, {}, {"string"}, {"string", "null"}, {"integer", "number"}}
	probes := []string{"string", "null", "integer", "number", "object", ""}
	for _, types := range cases {
		var ours *Types
		var kin *openapi3.Types
		if types != nil {
			copied := append([]string(nil), types...)
			ours = (*Types)(&copied)
			kinCopied := append([]string(nil), types...)
			kin = (*openapi3.Types)(&kinCopied)
		}
		require.Equal(t, kin.Slice(), ours.Slice())
		for _, probe := range probes {
			require.Equal(t, kin.Is(probe), ours.Is(probe), "Is %q of %v", probe, types)
			require.Equal(t, kin.Includes(probe), ours.Includes(probe), "Includes %q of %v", probe, types)
			require.Equal(t, kin.Permits(probe), ours.Permits(probe), "Permits %q of %v", probe, types)
		}
	}
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

// TestSchemaFieldsMatchKin guards the reflection the allOf merger uses.
// Every kin Schema field must exist here, in the same order, with the same
// json keyword (the name before the first comma). An extra field would be
// reported as a keyword the merger cannot fold.
func TestSchemaFieldsMatchKin(t *testing.T) {
	t.Parallel()

	kinFields := reflect.TypeFor[openapi3.Schema]()
	ourFields := reflect.TypeFor[Schema]()
	require.Equal(t, kinFields.NumField(), ourFields.NumField())

	for i := range kinFields.NumField() {
		kinField := kinFields.Field(i)
		ourField := ourFields.Field(i)
		require.Equal(t, kinField.Name, ourField.Name, "field %d", i)
		require.Equal(t, jsonKeyword(kinField), jsonKeyword(ourField), "json keyword of %s", kinField.Name)
	}
}

func jsonKeyword(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	return name
}
