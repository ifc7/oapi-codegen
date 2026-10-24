package codegen

// These tests cover the allOf merge rules.

import (
	"maps"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

// mergeTwo merges two allOf members, allOf/0 and allOf/1.
func mergeTwo(s1, s2 model.Schema) (model.Schema, error) {
	m := newAllOfMerge(newGenContext(nil), allOfOptions{})
	if err := m.add(nil, s1, "allOf/0", map[string]bool{}); err != nil {
		return model.Schema{}, err
	}
	if err := m.add(nil, s2, "allOf/1", map[string]bool{}); err != nil {
		return model.Schema{}, err
	}
	return m.result()
}

func TestMergeOpenapiSchemas_DiscriminatorPropagation(t *testing.T) {
	disc := &model.Discriminator{
		PropertyName: "type",
	}

	t.Run("allOf with single discriminator on s1 propagates it", func(t *testing.T) {
		s1 := model.Schema{Discriminator: disc}
		s2 := model.Schema{}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Equal(t, disc, result.Discriminator)
	})

	t.Run("allOf with single discriminator on s2 propagates it", func(t *testing.T) {
		s1 := model.Schema{}
		s2 := model.Schema{Discriminator: disc}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Equal(t, disc, result.Discriminator)
	})

	t.Run("allOf with different discriminators errors", func(t *testing.T) {
		disc2 := &model.Discriminator{PropertyName: "kind"}
		s1 := model.Schema{Discriminator: disc}
		s2 := model.Schema{Discriminator: disc2}

		_, err := mergeTwo(s1, s2)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "allOf can't merge allOf/0 (discriminator type) with allOf/1 (discriminator kind)")
	})

	t.Run("allOf with the same discriminator on both schemas merges", func(t *testing.T) {
		mapping := map[string]model.MappingRef{"cat": {Ref: "#/components/schemas/Cat"}}
		s1 := model.Schema{Discriminator: &model.Discriminator{PropertyName: "type", Mapping: mapping}}
		s2 := model.Schema{Discriminator: &model.Discriminator{PropertyName: "type", Mapping: maps.Clone(mapping)}}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Equal(t, "type", result.Discriminator.PropertyName)

		s2.Discriminator.Mapping["dog"] = model.MappingRef{Ref: "#/components/schemas/Dog"}
		_, err = mergeTwo(s1, s2)
		require.Error(t, err, "a different mapping is a different discriminator")
	})

	t.Run("allOf with no discriminators succeeds with nil discriminator", func(t *testing.T) {
		s1 := model.Schema{}
		s2 := model.Schema{}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Nil(t, result.Discriminator)
	})
}

// TestMergeOpenapiSchemas_TypePropagation covers the one-sided type rule
// (issue #2524): a type declared by only one allOf member propagates to the
// merged result regardless of member order, while two members declaring
// different types remain an error.
func TestMergeOpenapiSchemas_TypePropagation(t *testing.T) {
	stringType := &model.Types{"string"}
	numberType := &model.Types{"number"}
	unionType := &model.Types{"string", "number"}

	t.Run("type on s2 only propagates", func(t *testing.T) {
		s1 := model.Schema{}
		s2 := model.Schema{Type: stringType}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Equal(t, stringType, result.Type)
	})

	t.Run("type on s1 only propagates", func(t *testing.T) {
		s1 := model.Schema{Type: stringType}
		s2 := model.Schema{}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Equal(t, stringType, result.Type)
	})

	t.Run("multi-type union on s2 only propagates", func(t *testing.T) {
		s1 := model.Schema{
			Properties: model.Schemas{
				"name": model.NewSchemaRef("", model.NewStringSchema()),
			},
		}
		s2 := model.Schema{Type: unionType}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Equal(t, unionType, result.Type)
	})

	t.Run("equal types on both members merge", func(t *testing.T) {
		s1 := model.Schema{Type: stringType}
		s2 := model.Schema{Type: stringType}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Equal(t, stringType, result.Type)
	})

	t.Run("different types on both members error", func(t *testing.T) {
		s1 := model.Schema{Type: stringType}
		s2 := model.Schema{Type: numberType}

		_, err := mergeTwo(s1, s2)
		require.Error(t, err)
		assert.EqualError(t, err, "allOf can't merge allOf/0 (type string) with allOf/1 (type number): no value has both types")
	})

	t.Run("integer narrows number", func(t *testing.T) {
		for _, types := range [][2]*model.Types{{numberType, {"integer"}}, {{"integer"}, numberType}} {
			result, err := mergeTwo(model.Schema{Type: types[0]}, model.Schema{Type: types[1]})
			require.NoError(t, err)
			assert.Equal(t, []string{"integer"}, result.Type.Slice())
		}
	})

	t.Run("a type list with number and integer is number", func(t *testing.T) {
		intNum := func() *model.Types { return &model.Types{"integer", "number"} }
		for want, other := range map[string]*model.Types{"number": numberType, "integer": {"integer"}} {
			result, err := mergeTwo(model.Schema{Type: intNum()}, model.Schema{Type: other})
			require.NoError(t, err)
			assert.Equal(t, []string{want}, result.Type.Slice())
		}
	})

	t.Run("multi-type arrays intersect", func(t *testing.T) {
		result, err := mergeTwo(model.Schema{Type: &model.Types{"string", "number", "boolean"}},
			model.Schema{Type: &model.Types{"integer", "string"}})
		require.NoError(t, err)
		assert.Equal(t, []string{"string", "integer"}, result.Type.Slice())
	})

	t.Run("neither member typed stays typeless", func(t *testing.T) {
		s1 := model.Schema{}
		s2 := model.Schema{}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Nil(t, result.Type.Slice())
	})
}

// TestMergeOpenapiSchemas_FormatPropagation covers the same one-sided rule
// for format: a format declared by only one member propagates instead of
// erroring, which is what the allOf decorator idiom over format-carrying
// scalars produces (e.g. $ref to {type: string, format: uuid} + nullable).
func TestMergeOpenapiSchemas_FormatPropagation(t *testing.T) {
	t.Run("format on s2 only propagates", func(t *testing.T) {
		s1 := model.Schema{}
		s2 := model.Schema{Format: "uuid"}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Equal(t, "uuid", result.Format)
	})

	t.Run("format on s1 only propagates", func(t *testing.T) {
		s1 := model.Schema{Format: "uuid"}
		s2 := model.Schema{}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Equal(t, "uuid", result.Format)
	})

	t.Run("equal formats merge", func(t *testing.T) {
		s1 := model.Schema{Format: "uuid"}
		s2 := model.Schema{Format: "uuid"}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Equal(t, "uuid", result.Format)
	})

	t.Run("different formats error", func(t *testing.T) {
		s1 := model.Schema{Format: "uuid"}
		s2 := model.Schema{Format: "date-time"}

		_, err := mergeTwo(s1, s2)
		require.Error(t, err)
		assert.EqualError(t, err, "allOf can't merge allOf/0 (format uuid) with allOf/1 (format date-time): a value can't have both formats")
	})

	t.Run("nullable decorator over format-carrying scalar merges", func(t *testing.T) {
		s1 := model.Schema{Type: &model.Types{"string"}, Format: "uuid"}
		s2 := model.Schema{Nullable: true}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.Equal(t, "uuid", result.Format)
		assert.True(t, result.Nullable)
	})
}

// TestMergeOpenapiSchemas_NullableUnion covers the OpenAPI 3.0 idiom of
// decorating a $ref with `nullable: true` via allOf (issue #1898). Members
// disagreeing on nullability must merge (union) rather than error.
func TestMergeOpenapiSchemas_NullableUnion(t *testing.T) {
	t.Run("nullable on s2 propagates to result", func(t *testing.T) {
		s1 := model.Schema{}
		s2 := model.Schema{Nullable: true}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.True(t, result.Nullable)
	})

	t.Run("nullable on s1 propagates to result", func(t *testing.T) {
		s1 := model.Schema{Nullable: true}
		s2 := model.Schema{}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.True(t, result.Nullable)
	})

	t.Run("both nullable stays nullable", func(t *testing.T) {
		s1 := model.Schema{Nullable: true}
		s2 := model.Schema{Nullable: true}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.True(t, result.Nullable)
	})

	t.Run("neither nullable stays non-nullable", func(t *testing.T) {
		s1 := model.Schema{}
		s2 := model.Schema{}

		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		assert.False(t, result.Nullable)
	})
}

// TestMergeOpenapiSchemas_Annotations covers keywords that don't shape the Go
// type. Merging them must never fail: the allOf decorator idiom sets them on
// one member only, and kin-openapi can't tell an unset flag from false.
func TestMergeOpenapiSchemas_Annotations(t *testing.T) {
	merge := func(t *testing.T, s1, s2 model.Schema) model.Schema {
		t.Helper()
		result, err := mergeTwo(s1, s2)
		require.NoError(t, err)
		return result
	}

	t.Run("default on one member is kept (issue #1379)", func(t *testing.T) {
		assert.Equal(t, "asc", merge(t, model.Schema{Default: "asc"}, model.Schema{}).Default)
		assert.Equal(t, "asc", merge(t, model.Schema{}, model.Schema{Default: "asc"}).Default)
	})

	t.Run("defaults on both members keep the later one", func(t *testing.T) {
		assert.Equal(t, "desc", merge(t, model.Schema{Default: "asc"}, model.Schema{Default: "desc"}).Default)
	})

	t.Run("flags set on either member are set on the result", func(t *testing.T) {
		set := model.Schema{ReadOnly: true, WriteOnly: true, AllowEmptyValue: true}
		for name, pair := range map[string][2]model.Schema{
			"s1": {set, {}},
			"s2": {{}, set},
		} {
			t.Run(name, func(t *testing.T) {
				result := merge(t, pair[0], pair[1])
				assert.True(t, result.ReadOnly, "readOnly")
				assert.True(t, result.WriteOnly, "writeOnly")
				assert.True(t, result.AllowEmptyValue, "allowEmptyValue")
			})
		}
	})
}

// TestMergeOpenapiSchemas_NullInTypeArray covers OpenAPI 3.1 type arrays,
// where "null" spells nullability. It is unioned like 3.0's `nullable`
// instead of taking part in the type comparison.
func TestMergeOpenapiSchemas_NullInTypeArray(t *testing.T) {
	object := &model.Types{"object"}
	nullableObject := &model.Types{"object", "null"}

	merge := func(s1, s2 model.Schema) (model.Schema, error) {
		return mergeTwo(s1, s2)
	}

	t.Run("a nullable member makes the result nullable", func(t *testing.T) {
		for name, pair := range map[string][2]*model.Types{
			"s1 nullable": {nullableObject, object},
			"s2 nullable": {object, nullableObject},
		} {
			t.Run(name, func(t *testing.T) {
				result, err := merge(model.Schema{Type: pair[0]}, model.Schema{Type: pair[1]})
				require.NoError(t, err)
				assert.ElementsMatch(t, []string{"object", "null"}, result.Type.Slice())
			})
		}
	})

	t.Run("types that already agree stay as they are", func(t *testing.T) {
		result, err := merge(model.Schema{Type: object}, model.Schema{Type: &model.Types{"object"}})
		require.NoError(t, err)
		assert.Equal(t, []string{"object"}, result.Type.Slice())
	})

	t.Run("type arrays compare as sets", func(t *testing.T) {
		_, err := merge(model.Schema{Type: &model.Types{"string", "integer"}},
			model.Schema{Type: &model.Types{"integer", "string"}})
		assert.NoError(t, err)
	})

	t.Run("different non-null types still conflict", func(t *testing.T) {
		_, err := merge(model.Schema{Type: &model.Types{"string"}},
			model.Schema{Type: &model.Types{"integer", "null"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no value has both types")
	})
}

// TestMergeOpenapiSchemas_AdditionalProperties covers two members that both
// declare an additionalProperties schema.
func TestMergeOpenapiSchemas_AdditionalProperties(t *testing.T) {
	withAP := func(ap *model.SchemaRef) model.Schema {
		return model.Schema{AdditionalProperties: model.AdditionalProperties{Schema: ap}}
	}
	merge := func(s1, s2 model.Schema) (model.Schema, error) {
		return mergeTwo(s1, s2)
	}

	t.Run("identical inline schemas merge", func(t *testing.T) {
		result, err := merge(
			withAP(model.NewSchemaRef("", model.NewStringSchema())),
			withAP(model.NewSchemaRef("", model.NewStringSchema())))
		require.NoError(t, err)
		assert.True(t, result.AdditionalProperties.Schema.Value.Type.Is("string"))
	})

	t.Run("the same $ref merges", func(t *testing.T) {
		result, err := merge(
			withAP(model.NewSchemaRef("#/components/schemas/X", model.NewStringSchema())),
			withAP(model.NewSchemaRef("#/components/schemas/X", model.NewStringSchema())))
		require.NoError(t, err)
		assert.Equal(t, "#/components/schemas/X", result.AdditionalProperties.Schema.Ref)
	})

	t.Run("different schemas become an allOf of both", func(t *testing.T) {
		s1 := model.NewSchemaRef("", model.NewStringSchema())
		s2 := model.NewSchemaRef("", model.NewStringSchema().WithMaxLength(3))
		result, err := merge(withAP(s1), withAP(s2))
		require.NoError(t, err)
		assertAllOfOf(t, result.AdditionalProperties.Schema, s1, s2)
	})

	t.Run("false on either member closes the object", func(t *testing.T) {
		closed := model.Schema{}
		closed.WithoutAdditionalProperties()
		result, err := merge(withAP(model.NewSchemaRef("", model.NewStringSchema())), closed)
		require.NoError(t, err)
		assert.True(t, isAdditionalPropertiesExplicitFalse(&result))
	})
}

// assertAllOfOf asserts that ref is an allOf of copies of the want schemas,
// which is how the merge combines schemas that members declare for one
// position.
func assertAllOfOf(t *testing.T, ref *model.SchemaRef, want ...*model.SchemaRef) {
	t.Helper()
	require.NotNil(t, ref)
	require.Len(t, ref.Value.AllOf, len(want))
	for i, w := range want {
		assert.Same(t, w.Value, ref.Value.AllOf[i].Value)
		assert.Equal(t, w.Ref, ref.Value.AllOf[i].Ref)
	}
}

// TestMergeOpenapiSchemas_Items covers array items, which used to be dropped
// by every allOf merge.
func TestMergeOpenapiSchemas_Items(t *testing.T) {
	merge := func(s1, s2 model.Schema) (model.Schema, error) {
		return mergeTwo(s1, s2)
	}
	itemRef := model.NewSchemaRef("#/components/schemas/Item", model.NewObjectSchema())

	t.Run("items on one member carry over", func(t *testing.T) {
		for name, pair := range map[string][2]model.Schema{
			"s1": {{Items: itemRef}, {}},
			"s2": {{}, {Items: itemRef}},
		} {
			t.Run(name, func(t *testing.T) {
				result, err := merge(pair[0], pair[1])
				require.NoError(t, err)
				assert.Same(t, itemRef, result.Items)
			})
		}
	})

	t.Run("the same $ref is kept", func(t *testing.T) {
		result, err := merge(model.Schema{Items: itemRef},
			model.Schema{Items: model.NewSchemaRef("#/components/schemas/Item", model.NewObjectSchema())})
		require.NoError(t, err)
		assert.Same(t, itemRef, result.Items)
	})

	t.Run("different item schemas become an allOf of both", func(t *testing.T) {
		a := model.NewSchemaRef("", model.NewObjectSchema().WithProperty("a", model.NewStringSchema()))
		b := model.NewSchemaRef("", model.NewObjectSchema().WithProperty("b", model.NewStringSchema()))
		result, err := merge(model.Schema{Items: a}, model.Schema{Items: b})
		require.NoError(t, err)
		assertAllOfOf(t, result.Items, a, b)
	})
}

func TestMergeOpenapiSchemas_Enum(t *testing.T) {
	named := func(enum []any, names ...any) model.Schema {
		s := model.Schema{Enum: enum}
		if len(names) > 0 {
			s.Extensions = map[string]any{extEnumVarNames: names}
		}
		return s
	}

	t.Run("an enum on one member is kept", func(t *testing.T) {
		result, err := mergeTwo(named([]any{"a", "b"}, "A", "B"), model.Schema{})
		require.NoError(t, err)
		assert.Equal(t, []any{"a", "b"}, result.Enum)
		assert.Equal(t, []any{"A", "B"}, result.Extensions[extEnumVarNames])
	})

	t.Run("two enums intersect, in the first member's order, with the later member's names (issue #1633)", func(t *testing.T) {
		result, err := mergeTwo(named([]any{"a", "b", "c"}, "A", "B", "C"), named([]any{"c", "a", "z"}, "X", "Y", "Z"))
		require.NoError(t, err)
		assert.Equal(t, []any{"a", "c"}, result.Enum)
		assert.Equal(t, []any{"Y", "X"}, result.Extensions[extEnumVarNames])
	})

	t.Run("a later member without names keeps the earlier names", func(t *testing.T) {
		result, err := mergeTwo(named([]any{"a", "b", "c"}, "A", "B", "C"), named([]any{"c", "a"}))
		require.NoError(t, err)
		assert.Equal(t, []any{"A", "C"}, result.Extensions[extEnumVarNames])
	})

	t.Run("names come from the second member when the first has none", func(t *testing.T) {
		result, err := mergeTwo(named([]any{"a", "b", "c"}), named([]any{"c", "a"}, "X", "Y"))
		require.NoError(t, err)
		assert.Equal(t, []any{"a", "c"}, result.Enum)
		assert.Equal(t, []any{"Y", "X"}, result.Extensions[extEnumVarNames])
	})

	t.Run("a member without an enum can rename its values", func(t *testing.T) {
		result, err := mergeTwo(named([]any{"a", "b"}, "A", "B"),
			model.Schema{Extensions: map[string]any{extEnumNames: []any{"First", "Second"}}})
		require.NoError(t, err)
		assert.Equal(t, []any{"First", "Second"}, result.Extensions[extEnumVarNames])
	})

	t.Run("enums with no value in common error", func(t *testing.T) {
		_, err := mergeTwo(named([]any{"a", "b"}), named([]any{"c"}))
		assert.EqualError(t, err, "allOf can't merge allOf/0 (enum [a b]) with allOf/1 (enum [c]): no value is in both. "+
			"To allow the values of either, set x-oapi-codegen-enum-merge: union on the schema with this allOf")
	})

	t.Run("x-oapi-codegen-enum-merge: union takes the values of either", func(t *testing.T) {
		m := newAllOfMerge(newGenContext(nil), allOfOptions{})
		m.unionEnums = true
		require.NoError(t, m.add(nil, named([]any{"a", "b"}, "A", "B"), "allOf/0", map[string]bool{}))
		require.NoError(t, m.add(nil, named([]any{"b", "c"}), "allOf/1", map[string]bool{}))
		result, err := m.result()
		require.NoError(t, err)
		assert.Equal(t, []any{"a", "b", "c"}, result.Enum)
		assert.Equal(t, []any{"A", "B", "c"}, result.Extensions[extEnumVarNames],
			"a value no member names is named after itself")
	})
}

func TestMergeOpenapiSchemas_Const(t *testing.T) {
	t.Run("the same const merges", func(t *testing.T) {
		result, err := mergeTwo(model.Schema{Const: "cat"}, model.Schema{Const: "cat"})
		require.NoError(t, err)
		assert.Equal(t, "cat", result.Const)
	})

	t.Run("different consts error", func(t *testing.T) {
		_, err := mergeTwo(model.Schema{Const: "cat"}, model.Schema{Const: "dog"})
		assert.EqualError(t, err, "allOf can't merge allOf/0 (const cat) with allOf/1 (const dog): no value is both")
	})

	t.Run("a const narrows an enum", func(t *testing.T) {
		result, err := mergeTwo(model.Schema{Enum: []any{"cat", "dog"}}, model.Schema{Const: "dog"})
		require.NoError(t, err)
		assert.Equal(t, []any{"dog"}, result.Enum)
	})

	t.Run("a const the enum doesn't allow errors", func(t *testing.T) {
		_, err := mergeTwo(model.Schema{Enum: []any{"cat", "dog"}}, model.Schema{Const: "cow"})
		assert.EqualError(t, err, "allOf can't merge allOf/1 (const cow) with allOf/0 (enum [cat dog]): the enum doesn't allow the const")
	})
}

func TestMergeOpenapiSchemas_Required(t *testing.T) {
	result, err := mergeTwo(model.Schema{Required: []string{"a", "b"}}, model.Schema{Required: []string{"b", "c"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, result.Required)
}

// TestMergeOpenapiSchemas_NestedAllOf: a member that is itself an allOf
// contributes its members and its own keywords.
func TestMergeOpenapiSchemas_NestedAllOf(t *testing.T) {
	nested := model.Schema{
		Properties: model.Schemas{"own": model.NewSchemaRef("", model.NewStringSchema())},
		AllOf: model.SchemaRefs{model.NewSchemaRef("", &model.Schema{
			Properties: model.Schemas{"inner": model.NewSchemaRef("", model.NewStringSchema())},
		})},
	}
	result, err := mergeTwo(nested, model.Schema{
		Properties: model.Schemas{"other": model.NewSchemaRef("", model.NewStringSchema())},
	})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"own", "inner", "other"}, slices.Collect(maps.Keys(result.Properties)))

	nested.AllOf[0].Value.Type = &model.Types{"array"}
	_, err = mergeTwo(nested, model.Schema{Type: &model.Types{"object"}})
	assert.EqualError(t, err, "allOf can't merge allOf/0/allOf/0 (type array) with allOf/1 (type object): no value has both types",
		"the error names the nested member")
}

func TestMergeSchemasEndToEnd(t *testing.T) {
	t.Run("number, and number or integer, is number (3.1)", func(t *testing.T) {
		code := generateSpec(t, opaqueSpecHeader31+`
    Amount:
      allOf:
        - type: [integer, number]
        - type: number
`)
		assert.Contains(t, code, "type Amount = float32")
	})

	t.Run("integer narrows number", func(t *testing.T) {
		code := generateSpec(t, opaqueSpecHeader+`
    Count:
      allOf:
        - type: number
        - type: integer
          format: int64
`)
		assert.Contains(t, code, "type Count = int64")
	})

	t.Run("a conflict names both members", func(t *testing.T) {
		_, err := generateSpecErr(opaqueSpecHeader + `
    Base:
      allOf:
        - type: object
          properties:
            name: {type: string}
    Odd:
      type: object
      properties:
        extra: {type: string}
      allOf:
        - $ref: '#/components/schemas/Base'
        - type: string
`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error converting Schema Odd to Go type: error merging schemas: "+
			"allOf can't merge #/components/schemas/Base/allOf/0 (type object) with allOf/1 (type string): no value has both types")
	})

	t.Run("the schema's own keywords are the schema itself", func(t *testing.T) {
		_, err := generateSpecErr(opaqueSpecHeader + `
    Odd:
      type: string
      required: [x]
      allOf:
        - type: integer
`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "allOf can't merge allOf/0 (type integer) with the schema itself (type string)")
	})
}

// TestMergeSchemasProperties covers properties, items and
// additionalProperties that several members declare (issue #2107).
func TestMergeSchemasProperties(t *testing.T) {
	const base = `
    Base:
      type: object
      required: [name]
      properties:
        name: {type: string, description: The name.}
        age: {type: integer}
        address:
          type: object
          properties:
            street: {type: string}
`

	t.Run("a member refines a property instead of replacing it", func(t *testing.T) {
		code := generateSpec(t, opaqueSpecHeader+base+`
    Patch:
      allOf:
        - $ref: '#/components/schemas/Base'
        - properties:
            name:
              nullable: true
              x-go-name: Moniker
            address:
              properties:
                zip: {type: string}
`)
		assert.Regexp(t, `// Moniker The name\.\n\tMoniker \*string `+"`json:\"name\"`", code,
			"name keeps Base's type and description, and gains nullable and the member's field name")
		assertField(t, code, "Age", "*int")
		assert.Regexp(t, `Address \*struct \{\n\t\tStreet \*string [^\n]*\n\t\tZip    \*string`, code)
	})

	t.Run("a property whose types conflict names both members", func(t *testing.T) {
		_, err := generateSpecErr(opaqueSpecHeader + base + `
    Odd:
      allOf:
        - $ref: '#/components/schemas/Base'
        - properties:
            age: {type: string}
`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error generating Go schema for property 'age': error merging schemas: "+
			"allOf can't merge #/components/schemas/Base/properties/age (type integer) with allOf/1/properties/age (type string): "+
			"no value has both types")
	})

	t.Run("items and additionalProperties merge the same way", func(t *testing.T) {
		code := generateSpec(t, opaqueSpecHeader+`
    Tags:
      allOf:
        - type: array
          items: {type: string}
        - type: array
          items: {maxLength: 10, nullable: true}
    Labels:
      allOf:
        - additionalProperties: {type: string}
        - additionalProperties: {maxLength: 10}
`)
		assert.Contains(t, code, "type Tags = []*string")
		assert.Contains(t, code, "type Labels map[string]string")

		_, err := generateSpecErr(opaqueSpecHeader + `
    Tags:
      allOf:
        - type: array
          items: {type: string}
        - type: array
          items: {type: integer}
`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "allOf can't merge allOf/0/items (type string) with allOf/1/items (type integer)")
	})

	t.Run("a merged property that refers back to the composition", func(t *testing.T) {
		code := generateSpec(t, opaqueSpecHeader+`
    Node:
      allOf:
        - type: object
          properties:
            name: {type: string}
            next: {$ref: '#/components/schemas/Node'}
        - properties:
            next: {nullable: true}
`)
		// next is an allOf of Node and {nullable: true}, which only annotates
		// Node: it is Node.
		assert.Contains(t, code, "type Node struct {")
		assertField(t, code, "Next", "*Node")
	})

	t.Run("the same $ref with extensions of its own stays that $ref", func(t *testing.T) {
		code := generateSpec(t, opaqueSpecHeader+`
    User:
      type: object
      properties:
        name: {type: string}
    Base:
      type: object
      properties:
        owner: {$ref: '#/components/schemas/User'}
    Patch:
      allOf:
        - $ref: '#/components/schemas/Base'
        - properties:
            owner:
              $ref: '#/components/schemas/User'
              x-go-name: Proprietor
`)
		patch := code[strings.Index(code, "type Patch struct"):]
		assertField(t, patch, "Proprietor", "*User")
	})

	t.Run("a recursive $ref with extensions of its own", func(t *testing.T) {
		code := generateSpec(t, opaqueSpecHeader+`
    Node:
      allOf:
        - type: object
          properties:
            name: {type: string}
            next:
              $ref: '#/components/schemas/Node'
              x-go-name: First
        - properties:
            next:
              $ref: '#/components/schemas/Node'
              x-go-name: Second
        - properties:
            next: {nullable: true}
`)
		assert.Contains(t, code, "type Node struct {")
		assertField(t, code, "Second", "*Node")
	})

	t.Run("recursive compositions still generate", func(t *testing.T) {
		code := generateSpec(t, specRecursiveObject)
		assert.Contains(t, code, "type Node struct {")
	})
}

// TestEnumMergeExtension covers x-oapi-codegen-enum-merge, which lets a
// composition add values to an enum.
func TestEnumMergeExtension(t *testing.T) {
	const statuses = `
    BaseStatus:
      type: string
      enum: [active, inactive]
    ExtendedStatus:
      x-oapi-codegen-enum-merge: union
      allOf:
        - $ref: '#/components/schemas/BaseStatus'
        - enum: [archived]
`
	enumValues := func(t *testing.T, code, typeName string) []string {
		t.Helper()
		start := strings.Index(code, "// Defines values for "+typeName+".")
		require.GreaterOrEqual(t, start, 0, "no enum %s", typeName)
		block := code[start:]
		block = block[:strings.Index(block, ")")]
		var values []string
		for _, m := range regexp.MustCompile(typeName+` = "([^"]*)"`).FindAllStringSubmatch(block, -1) {
			values = append(values, m[1])
		}
		return values
	}

	t.Run("union", func(t *testing.T) {
		code := generateSpec(t, opaqueSpecHeader+statuses+`
    Decorated:
      allOf:
        - $ref: '#/components/schemas/ExtendedStatus'
        - description: A decorated ExtendedStatus.
    Narrowed:
      allOf:
        - $ref: '#/components/schemas/ExtendedStatus'
        - enum: [inactive, archived]
    Holder:
      type: object
      properties:
        status:
          x-oapi-codegen-enum-merge: union
          allOf:
            - $ref: '#/components/schemas/BaseStatus'
            - enum: [archived]
    Base:
      type: object
      properties:
        status: {$ref: '#/components/schemas/BaseStatus'}
    Patch:
      allOf:
        - $ref: '#/components/schemas/Base'
        - properties:
            status:
              enum: [archived]
              x-oapi-codegen-enum-merge: union
`)
		for typeName, why := range map[string]string{
			"ExtendedStatus": "a component",
			"HolderStatus":   "a property that is a composition",
			"PatchStatus":    "a property a member refines, where the extension is on the member's property",
		} {
			assert.ElementsMatch(t, []string{"active", "inactive", "archived"}, enumValues(t, code, typeName), why)
		}
		assert.Contains(t, code, "type Decorated = ExtendedStatus")
		assert.ElementsMatch(t, []string{"inactive", "archived"}, enumValues(t, code, "Narrowed"),
			"merging the union composition with another enum starts from its values")
	})

	t.Run("intersection is the default", func(t *testing.T) {
		_, err := generateSpecErr(opaqueSpecHeader + strings.ReplaceAll(statuses, "x-oapi-codegen-enum-merge: union", "x-oapi-codegen-enum-merge: intersection"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no value is in both")
	})

	t.Run("an invalid value is an error", func(t *testing.T) {
		_, err := generateSpecErr(opaqueSpecHeader + strings.ReplaceAll(statuses, "union", "both"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), `invalid value for "x-oapi-codegen-enum-merge": must be "union" or "intersection", not both`)
	})
}

// TestEnumMergeScope: x-oapi-codegen-enum-merge applies to the schema it's
// on. A nested composition merges its own way, the default included, and a
// merged property that needs the union says where to put it.
func TestEnumMergeScope(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Inner:
      allOf:
        - type: string
          enum: [a, b]
        - enum: [b, c]
    Outer:
      x-oapi-codegen-enum-merge: union
      allOf:
        - $ref: '#/components/schemas/Inner'
        - enum: [x]
`)
	assert.Contains(t, code, `OuterB Outer = "b"`)
	assert.Contains(t, code, `OuterX Outer = "x"`)
	assert.NotContains(t, code, "OuterA", "Inner only allows b")
	assert.NotContains(t, code, "OuterC")

	_, err := generateSpecErr(opaqueSpecHeader + `
    Base:
      type: object
      properties:
        status: {type: string, enum: ["on", "off"]}
    Extended:
      x-oapi-codegen-enum-merge: union
      allOf:
        - $ref: '#/components/schemas/Base'
        - properties:
            status: {enum: [unknown]}
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "set x-oapi-codegen-enum-merge: union on allOf/1/properties/status")
}

// TestEnumRenameMakesANewEnum: x-enum-varnames next to a $ref'd enum
// renames its values, which takes a new enum type.
func TestEnumRenameMakesANewEnum(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+decoratorComponents+`
    Renamed:
      allOf:
        - $ref: '#/components/schemas/Color'
        - x-enum-varnames: [Rouge, Vert]
`)
	assert.Contains(t, code, "type Renamed string")
	assert.Contains(t, code, `Rouge Renamed = "red"`)
	assert.Contains(t, code, `Vert  Renamed = "green"`)

	// Restating the enum's type doesn't make the names an annotation.
	code = generateSpec(t, opaqueSpecHeader+decoratorComponents+`
    Renamed:
      allOf:
        - $ref: '#/components/schemas/Color'
        - type: string
          x-enumNames: [Rouge, Vert]
`)
	assert.Contains(t, code, "type Renamed string")
	assert.Contains(t, code, `Rouge Renamed = "red"`)
}

// TestOwnKeywords: the schema's own keywords next to allOf constrain the
// value like one more member: `{type: string, allOf: [A]}` is
// `{allOf: [A, {type: string}]}`.
func TestOwnKeywords(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Str: {type: string}
    Letters:
      enum: [a, b]
      allOf:
        - $ref: '#/components/schemas/Str'
    Day:
      format: date
      allOf:
        - $ref: '#/components/schemas/Str'
    Counts:
      type: array
      items: {type: integer}
      allOf:
        - type: array
          minItems: 1
`)
	assert.Contains(t, code, `A Letters = "a"`)
	assert.Contains(t, code, "type Letters string")
	assert.Contains(t, code, "type Day = OpenAPIDate")
	assert.Contains(t, code, "type Counts = []int")

	_, err := generateSpecErr(opaqueSpecHeader + `
    Base:
      type: object
      properties:
        name: {type: string}
    Odd:
      type: string
      allOf:
        - $ref: '#/components/schemas/Base'
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(),
		"allOf can't merge #/components/schemas/Base (type object) with the schema itself (type string): no value has both types")
}

func TestConstraintOnlyUnion(t *testing.T) {
	object := &model.Schema{Type: &model.Types{"object"}}
	required := func(names ...string) *model.SchemaRef {
		return model.NewSchemaRef("", &model.Schema{Required: names})
	}
	null := model.NewSchemaRef("", &model.Schema{Type: &model.Types{"null"}})
	closed := false
	for name, tc := range map[string]struct {
		branches model.SchemaRefs
		want     bool
	}{
		"required only":                {model.SchemaRefs{required("a"), required("b")}, true},
		"restated type":                {model.SchemaRefs{model.NewSchemaRef("", &model.Schema{Type: &model.Types{"object"}, Required: []string{"a"}})}, true},
		"validation too":               {model.SchemaRefs{model.NewSchemaRef("", &model.Schema{Required: []string{"a"}, MinProps: 1})}, true},
		"another type":                 {model.SchemaRefs{model.NewSchemaRef("", &model.Schema{Type: &model.Types{"string"}})}, false},
		"properties":                   {model.SchemaRefs{model.NewSchemaRef("", model.NewObjectSchema().WithProperty("a", model.NewStringSchema()))}, false},
		"a type branch":                {model.SchemaRefs{required("a"), model.NewSchemaRef("#/components/schemas/A", model.NewObjectSchema().WithProperty("a", model.NewStringSchema()))}, false},
		"a local $ref to a constraint": {model.SchemaRefs{model.NewSchemaRef("#/components/schemas/NeedA", &model.Schema{Required: []string{"a"}})}, true},
		"a $ref into another document": {model.SchemaRefs{model.NewSchemaRef("./other.yaml#/components/schemas/NeedA", &model.Schema{Required: []string{"a"}})}, false},
		"a null branch":                {model.SchemaRefs{required("a"), null}, true},
		"only a null branch":           {model.SchemaRefs{null}, false},
		"closed":                       {model.SchemaRefs{model.NewSchemaRef("", &model.Schema{Required: []string{"a"}, AdditionalProperties: model.AdditionalProperties{Has: &closed}})}, true},
		"additionalProperties schema":  {model.SchemaRefs{model.NewSchemaRef("", &model.Schema{AdditionalProperties: model.AdditionalProperties{Schema: model.NewStringSchema().NewRef()}})}, false},
		"x-go-type":                    {model.SchemaRefs{model.NewSchemaRef("", &model.Schema{Required: []string{"a"}, Extensions: map[string]any{extPropGoType: "T"}})}, false},
		"x-go-type-name":               {model.SchemaRefs{model.NewSchemaRef("", &model.Schema{Required: []string{"a"}, Extensions: map[string]any{extGoTypeName: "NeedsA"}})}, false},
		"empty":                        {nil, false},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, isConstraintOnlyUnion(tc.branches, object))
		})
	}

	// An owner with no type of its own has the type its allOf members declare.
	composed := &model.Schema{AllOf: model.SchemaRefs{model.NewSchemaRef("#/components/schemas/Contact", object)}}
	restated := model.SchemaRefs{model.NewSchemaRef("", &model.Schema{Type: &model.Types{"object"}, Required: []string{"a"}})}
	assert.True(t, isConstraintOnlyUnion(restated, composed))
	assert.False(t, isConstraintOnlyUnion(restated, &model.Schema{}), "nothing is declared to restate")
	annotated := &model.Schema{AllOf: model.SchemaRefs{
		{Value: &model.Schema{AllOf: model.SchemaRefs{{Value: &model.Schema{Description: "x"}}}}},
		model.NewSchemaRef("#/components/schemas/Contact", object),
	}}
	assert.True(t, isConstraintOnlyUnion(restated, annotated), "a later member declares the type")
}

// TestConstraintOnlyUnionEndToEnd: a oneOf or anyOf that only adds
// constraints makes no union wherever it is (issue #839).
func TestConstraintOnlyUnionEndToEnd(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Contact:
      type: object
      properties:
        email: {type: string}
        phone: {type: string}
      oneOf:
        - required: [email]
        - required: [phone]
    Holder:
      type: object
      properties:
        contact:
          properties:
            email: {type: string}
          anyOf:
            - required: [email]
    Nested:
      allOf:
        - allOf:
            - $ref: '#/components/schemas/Contact'
            - oneOf:
                - required: [email]
        - properties:
            extra: {type: string}
    Dated:
      oneOf:
        - type: string
          format: date
        - type: string
          format: date-time
`)
	assert.Equal(t, 1, strings.Count(code, "union json.RawMessage"), "only Dated is a union")
	assert.NotContains(t, code, "Contact0")
	assert.Contains(t, code, "type Contact struct {")
	assert.Regexp(t, `Contact \*struct \{\n\t\tEmail \*string`, code)
	assert.Contains(t, code, "type Nested struct {")
	assert.Contains(t, code, "func (t Dated) AsDated0()", "branches with a type and format of their own are types")
}

// TestConstraintOnlyUnionRestatedType: a oneOf restating the type an allOf
// member declares still only adds constraints, whether it's in a member of
// the allOf or next to it, so the composition is the member's type.
func TestConstraintOnlyUnionRestatedType(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Contact:
      type: object
      properties:
        email: {type: string}
        phone: {type: string}
    InMember:
      allOf:
        - $ref: '#/components/schemas/Contact'
        - oneOf:
            - type: object
              required: [email]
            - type: object
              required: [phone]
    Beside:
      allOf:
        - $ref: '#/components/schemas/Contact'
      oneOf:
        - type: object
          required: [email]
        - {type: 'null'}
`)
	assert.NotContains(t, code, "union json.RawMessage")
	assert.Contains(t, code, "type InMember = Contact")
	assert.Contains(t, code, "type Beside = Contact")
}

// TestParentListsChildren: a member whose oneOf or anyOf lists the
// composition being merged, or a schema the merge flattens into it, adds no
// union: a child that is an allOf of its parent is one of the parent's
// branches, not a union of all of them.
func TestParentListsChildren(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Pet:
      type: object
      required: [petType]
      properties:
        petType: {type: string}
      discriminator:
        propertyName: petType
      oneOf:
        - $ref: '#/components/schemas/Cat'
        - $ref: '#/components/schemas/Dog'
    Cat:
      allOf:
        - $ref: '#/components/schemas/Pet'
        - properties:
            meow: {type: string}
    Dog:
      allOf:
        - $ref: '#/components/schemas/Pet'
        - properties:
            bark: {type: string}
    Kitten:
      allOf:
        - $ref: '#/components/schemas/Cat'
        - properties:
            age: {type: integer}
    Shape:
      anyOf:
        - $ref: '#/components/schemas/Square'
    Square:
      allOf:
        - $ref: '#/components/schemas/Shape'
        - properties:
            side: {type: number}
    Unlisted:
      allOf:
        - $ref: '#/components/schemas/Pet'
        - properties:
            purr: {type: string}
`)
	for _, child := range []string{"Cat", "Dog", "Kitten", "Square"} {
		start := strings.Index(code, "type "+child+" struct {")
		require.GreaterOrEqual(t, start, 0, child)
		body := code[start : start+strings.Index(code[start:], "\n}")]
		assert.NotContains(t, body, "union json.RawMessage", child)
	}
	assert.Contains(t, code, "func (t Pet) AsCat() (Cat, error)", "the parent is still the union")
	assert.Contains(t, code, "func (t Unlisted) AsCat() (Cat, error)",
		"a schema the parent doesn't list is still one of its branches")
}

// TestParentListsChildOnly: a child that is only an allOf of the parent
// that lists it, alone or with members that only annotate, is a struct of the
// parent's fields, not an alias of the parent's union. Only the composition
// being merged counts as listed: a property inside a child is still the
// parent's union.
func TestParentListsChildOnly(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Pet:
      type: object
      required: [petType]
      properties:
        petType: {type: string}
      discriminator:
        propertyName: petType
      oneOf:
        - $ref: '#/components/schemas/Cat'
        - $ref: '#/components/schemas/Dog'
        - $ref: '#/components/schemas/Bird'
    Cat:
      allOf:
        - $ref: '#/components/schemas/Pet'
    Dog:
      allOf:
        - $ref: '#/components/schemas/Pet'
        - description: A dog.
    Bird:
      allOf:
        - $ref: '#/components/schemas/Pet'
        - type: object
          properties:
            best:
              allOf:
                - $ref: '#/components/schemas/Pet'
                - properties:
                    since: {type: string}
`)
	assert.Contains(t, code, "type Cat struct {\n\tPetType string")
	assert.Contains(t, code, "type Dog struct {\n\tPetType string")
	assert.NotContains(t, code, "type Cat = Pet")
	assert.NotContains(t, code, "type Dog = Pet")
	assert.Contains(t, code, "func (t Bird_Best) AsCat() (Cat, error)", "best is a Pet, one of its branches")
}

// TestUnionComponents: an allOf of several unions is one union type whose
// variants' From* replace only the keys their own union's variants declare
// and keep the rest, the other unions' data. A key several unions' variants
// declare is kept.
func TestUnionComponents(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Card:
      type: object
      properties:
        kind: {type: string}
        card: {type: string}
        id: {type: string}
    Transfer:
      type: object
      properties:
        kind: {type: string}
        iban: {type: string}
    Courier:
      type: object
      properties:
        address: {type: string}
        id: {type: string}
    Pickup:
      type: object
      properties:
        store: {type: string}
    Locker:
      type: object
      properties:
        locker: {type: string}
    Payment:
      oneOf:
        - $ref: '#/components/schemas/Card'
        - $ref: '#/components/schemas/Transfer'
      discriminator:
        propertyName: kind
    Delivery:
      anyOf:
        - $ref: '#/components/schemas/Courier'
        - $ref: '#/components/schemas/Pickup'
        - $ref: '#/components/schemas/Locker'
    Order:
      allOf:
        - $ref: '#/components/schemas/Payment'
        - $ref: '#/components/schemas/Delivery'
    Inline:
      allOf:
        - oneOf:
            - $ref: '#/components/schemas/Card'
            - $ref: '#/components/schemas/Transfer'
        - oneOf:
            - $ref: '#/components/schemas/Courier'
            - $ref: '#/components/schemas/Pickup'
    Custom:
      type: object
      x-go-type: map[string]any
      properties:
        secret: {type: string}
    Opaque:
      allOf:
        - oneOf:
            - $ref: '#/components/schemas/Card'
            - $ref: '#/components/schemas/Custom'
        - $ref: '#/components/schemas/Delivery'
`)
	fromCard := methodBody(t, code, "func (t *Order) FromCard(")
	assert.Contains(t, fromCard, `delete(kept, "card")`)
	assert.Contains(t, fromCard, `delete(kept, "iban")`)
	assert.Contains(t, fromCard, `delete(kept, "kind")`)
	assert.NotContains(t, fromCard, `delete(kept, "id")`, "Courier declares id too")
	assert.Contains(t, fromCard, "{\"kind\":\"Card\"}", "Payment's discriminator is stamped")
	fromPickup := methodBody(t, code, "func (t *Order) FromPickup(")
	assert.Contains(t, fromPickup, `delete(kept, "address")`)
	assert.Contains(t, fromPickup, `delete(kept, "store")`)
	assert.NotContains(t, fromPickup, `delete(kept, "id")`)
	assert.Contains(t, code, "func (t Order) AsPayment() (Payment, error)")
	assert.Contains(t, methodBody(t, code, "func (t *Order) FromDelivery("), `delete(kept, "store")`)
	assert.Contains(t, code, "func (t Order) Discriminator() (string, error)")
	assert.NotContains(t, methodBody(t, code, "func (t *Order) FromCourier("), `{"kind":`)
	valueBy := methodBody(t, code, "func (t Order) ValueByDiscriminator(")
	assert.Contains(t, valueBy, `case "Card":`)
	assert.NotContains(t, valueBy, "Courier")

	assert.Contains(t, code, "func (t *Inline) FromCourier(")
	assert.NotContains(t, code, "func (t Inline) AsStruct", "an inline union has no type to get")

	fromCustom := methodBody(t, code, "func (t *Opaque) FromCustom(")
	assert.Contains(t, fromCustom, `delete(kept, "card")`)
	assert.NotContains(t, fromCustom, `delete(kept, "secret")`, "an x-go-type declares no keys")
}

// TestUnionComponentsCollapse: an allOf that brings one union, however many
// times, is generated as that union: From* replaces all of the union data.
func TestUnionComponentsCollapse(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Contact:
      type: object
      properties:
        email: {type: string}
        phone: {type: string}
    Card:
      type: object
      properties:
        card: {type: string}
    Transfer:
      type: object
      properties:
        iban: {type: string}
    Constrained:
      allOf:
        - $ref: '#/components/schemas/Contact'
        - oneOf:
            - type: object
              required: [email]
            - type: object
              required: [phone]
        - oneOf:
            - $ref: '#/components/schemas/Card'
            - $ref: '#/components/schemas/Transfer'
    Twice:
      allOf:
        - anyOf:
            - $ref: '#/components/schemas/Card'
            - $ref: '#/components/schemas/Transfer'
        - oneOf:
            - $ref: '#/components/schemas/Card'
            - $ref: '#/components/schemas/Transfer'
`)
	assert.NotContains(t, code, "delete(kept")
	assert.Contains(t, code, "func (t *Constrained) FromCard(")
	assert.NotContains(t, code, "ConstrainedOneOf")
	assert.Contains(t, code, "func (t *Twice) FromCard(")
	assert.Equal(t, 1, strings.Count(code, "func (t *Twice) FromCard("))
}

// methodBody returns the code of the function that starts with signature.
func methodBody(t *testing.T, code, signature string) string {
	t.Helper()
	start := strings.Index(code, signature)
	require.GreaterOrEqual(t, start, 0, "no %s", signature)
	end := strings.Index(code[start:], "\n}\n")
	require.GreaterOrEqual(t, end, 0)
	return code[start : start+end]
}

// unionComponentsSchemas are components for tests of allOfs of unions.
const unionComponentsSchemas = `
    Card:
      type: object
      properties:
        kind: {type: string}
        card: {type: string}
    Transfer:
      type: object
      properties:
        kind: {type: string}
        iban: {type: string}
    Courier:
      type: object
      properties:
        address: {type: string}
    Pickup:
      type: object
      properties:
        store: {type: string}
    Payment:
      required: [kind]
      oneOf:
        - $ref: '#/components/schemas/Card'
        - $ref: '#/components/schemas/Transfer'
    Delivery:
      oneOf:
        - $ref: '#/components/schemas/Courier'
        - $ref: '#/components/schemas/Pickup'
`

// TestUnionComponentsDiscriminator: a discriminator declared next to an
// allOf of several unions, or in a member of its own, belongs to the union it
// can tell apart; it is an error for it to fit several or none, or to be
// declared for two.
func TestUnionComponentsDiscriminator(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+unionComponentsSchemas+`
    Beside:
      discriminator:
        propertyName: kind
      allOf:
        - $ref: '#/components/schemas/Payment'
        - $ref: '#/components/schemas/Delivery'
    Mapped:
      allOf:
        - $ref: '#/components/schemas/Delivery'
        - $ref: '#/components/schemas/Payment'
        - discriminator:
            propertyName: kind
            mapping:
              card: '#/components/schemas/Card'
              transfer: '#/components/schemas/Transfer'
    Shipment:
      anyOf:
        - $ref: '#/components/schemas/Courier'
        - $ref: '#/components/schemas/Pickup'
      discriminator:
        propertyName: method
    AnyOf:
      allOf:
        - $ref: '#/components/schemas/Shipment'
        - $ref: '#/components/schemas/Payment'
`)
	assert.Contains(t, methodBody(t, code, "func (t *Beside) FromCard("), "`{\"kind\":\"Card\"}`")
	assert.NotContains(t, methodBody(t, code, "func (t *Beside) FromCourier("), `"kind"`)
	assert.Contains(t, methodBody(t, code, "func (t *Mapped) FromTransfer("), "`{\"kind\":\"transfer\"}`")
	assert.Contains(t, methodBody(t, code, "func (t Mapped) ValueByDiscriminator("), `case "card":`)
	assert.Contains(t, methodBody(t, code, "func (t *AnyOf) FromPickup("), "`{\"method\":\"Pickup\"}`",
		"an anyOf keeps its discriminator")
	assert.Contains(t, code, "func (t Beside) AsPayment() (Payment, error)", "a required list doesn't stop a $ref union's accessors")

	for name, tc := range map[string]struct{ spec, want string }{
		"fits several": {`
    Kinds:
      discriminator:
        propertyName: kind
      allOf:
        - $ref: '#/components/schemas/Payment'
        - oneOf:
            - $ref: '#/components/schemas/Transfer'
            - $ref: '#/components/schemas/Card'
`, "allOf can't tell which of its unions (#/components/schemas/Payment, allOf/1) the discriminator kind is for"},
		"fits none": {`
    Kinds:
      discriminator:
        propertyName: method
      allOf:
        - $ref: '#/components/schemas/Payment'
        - $ref: '#/components/schemas/Delivery'
`, "the discriminator method is for"},
		"declared for two": {`
    Kinds:
      allOf:
        - oneOf:
            - $ref: '#/components/schemas/Card'
            - $ref: '#/components/schemas/Transfer'
          discriminator:
            propertyName: kind
        - oneOf:
            - $ref: '#/components/schemas/Courier'
            - $ref: '#/components/schemas/Pickup'
          discriminator:
            propertyName: kind
`, "allOf can't use the discriminator kind for both allOf/0 and allOf/1"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := generateSpecErr(opaqueSpecHeader + unionComponentsSchemas + tc.spec)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestInlineDiscriminatorValue: an inline variant's discriminator value is a
// const or a single-value enum on the discriminator property, spelled as the
// mapping spells it.
func TestInlineDiscriminatorValue(t *testing.T) {
	variant := func(p *model.Schema) *model.Schema {
		return model.NewObjectSchema().WithProperty("kind", p)
	}
	for name, tc := range map[string]struct {
		schema *model.Schema
		want   string
		ok     bool
	}{
		"const":             {variant(&model.Schema{Const: "cat"}), "cat", true},
		"single enum":       {variant(model.NewStringSchema().WithEnum("cat")), "cat", true},
		"integer":           {variant(&model.Schema{Const: float64(2)}), "2", true},
		"boolean":           {variant(&model.Schema{Enum: []any{true}}), "true", true},
		"two values":        {variant(model.NewStringSchema().WithEnum("cat", "dog")), "", false},
		"no value":          {variant(model.NewStringSchema()), "", false},
		"no property":       {model.NewObjectSchema(), "", false},
		"from allOf member": {&model.Schema{AllOf: model.SchemaRefs{variant(&model.Schema{Const: "cat"}).NewRef()}}, "cat", true},
		"from a later member": {&model.Schema{AllOf: model.SchemaRefs{
			variant(model.NewStringSchema()).NewRef(),
			variant(model.NewStringSchema().WithEnum("cat")).NewRef(),
		}}, "cat", true},
		"in the property's allOf": {variant(&model.Schema{AllOf: model.SchemaRefs{
			model.NewStringSchema().NewRef(), {Value: &model.Schema{Enum: []any{"cat"}}},
		}}), "cat", true},
		"nullable enum":  {variant(&model.Schema{Enum: []any{"cat", nil}}), "cat", true},
		"small fraction": {variant(&model.Schema{Const: 1.5e-7}), "1.5e-7", true},
		"fractional":     {variant(&model.Schema{Const: 2.5}), "2.5", true},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok, err := inlineDiscriminatorValue(tc.schema, "kind")
			require.NoError(t, err)
			assert.Equal(t, tc.ok, ok)
			assert.Equal(t, tc.want, got)
		})
	}

	// kin-openapi reads numbers as float64, so an integer from 2^53 up may
	// already be rounded: 9007199254740993 arrives as 9007199254740992.
	for _, v := range []float64{9007199254740993, -(1 << 53), 1e21} {
		_, _, err := inlineDiscriminatorValue(variant(&model.Schema{Const: v}), "kind")
		assert.ErrorContains(t, err, "can't be read exactly", "%v", v)
	}
	_, _, err := inlineDiscriminatorValue(variant(&model.Schema{Const: float64(1<<53 - 1)}), "kind")
	assert.NoError(t, err)
}

// TestInlineDiscriminatedVariants: inline variants of a discriminated union
// take the value their discriminator property pins, with or without a
// mapping for the other variants.
func TestInlineDiscriminatedVariants(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Dog:
      type: object
      properties:
        petType: {type: string}
        bark: {type: string}
    Implicit:
      oneOf:
        - type: object
          properties:
            petType: {type: string, enum: [cat]}
            meow: {type: string}
        - type: object
          properties:
            petType: {const: dog}
            bark: {type: string}
      discriminator:
        propertyName: petType
    Mapped:
      oneOf:
        - type: object
          properties:
            petType: {type: string, enum: [cat]}
            meow: {type: string}
        - $ref: '#/components/schemas/Dog'
      discriminator:
        propertyName: petType
        mapping:
          dog: '#/components/schemas/Dog'
    Versioned:
      oneOf:
        - type: object
          properties:
            version: {type: integer, enum: [1]}
        - type: object
          properties:
            version: {type: integer, enum: [2]}
      discriminator:
        propertyName: version
    Base:
      type: object
      properties:
        petType: {type: string}
    Extended:
      oneOf:
        - allOf:
            - $ref: '#/components/schemas/Base'
            - type: object
              properties:
                petType: {type: string, enum: [cat]}
                meow: {type: string}
        - $ref: '#/components/schemas/Dog'
      discriminator:
        propertyName: petType
        mapping:
          dog: '#/components/schemas/Dog'
`)
	implicit := methodBody(t, code, "func (t Implicit) ValueByDiscriminator(")
	assert.Contains(t, implicit, `case "cat":`+"\n\t\treturn t.AsImplicit0()")
	assert.Contains(t, implicit, `case "dog":`+"\n\t\treturn t.AsImplicit1()")
	assert.Contains(t, methodBody(t, code, "func (t *Implicit) FromImplicit1("), "`{\"petType\":\"dog\"}`")
	mapped := methodBody(t, code, "func (t Mapped) ValueByDiscriminator(")
	assert.Contains(t, mapped, `case "cat":`+"\n\t\treturn t.AsMapped0()")
	assert.Contains(t, mapped, `case "dog":`+"\n\t\treturn t.AsDog()")
	assert.Contains(t, methodBody(t, code, "func (t *Versioned) FromVersioned1("), "`{\"version\":2}`")
	versioned := methodBody(t, code, "func (t Versioned) ValueByDiscriminator(")
	assert.Contains(t, versioned, "Value *int64 `json:\"version\"`", "an integer is read as one")
	assert.Contains(t, versioned, "case 1:")
	assert.Contains(t, versioned, "case 2:")
	assert.Contains(t, methodBody(t, code, "func (t Implicit) ValueByDiscriminator("), `case "cat":`)
	extended := methodBody(t, code, "func (t Extended) ValueByDiscriminator(")
	assert.Contains(t, extended, `case "cat":`+"\n\t\treturn t.AsExtended0()", "a later allOf member pins the value")

	// A 3.1 const without a type is a value of the const's JSON type.
	code = generateSpec(t, opaqueSpecHeader31+`
    Flag:
      oneOf:
        - type: object
          properties:
            enabled: {const: true}
            on: {type: string}
        - type: object
          properties:
            enabled: {const: false}
            off: {type: string}
      discriminator:
        propertyName: enabled
`)
	assert.Contains(t, methodBody(t, code, "func (t *Flag) FromFlag0("), "`{\"enabled\":true}`")
	assert.Contains(t, methodBody(t, code, "func (t Flag) Discriminator("), "json.RawMessage")

	// A declared type wins over what the values pinned look like.
	code = generateSpec(t, opaqueSpecHeader31+`
    Flag:
      type: object
      properties:
        enabled: {type: string}
      oneOf:
        - type: object
          properties:
            enabled: {type: string, const: true}
        - type: object
          properties:
            enabled: {type: string, const: false}
      discriminator:
        propertyName: enabled
`)
	assert.Contains(t, methodBody(t, code, "func (t *Flag) FromFlag0("), "`{\"enabled\":\"true\"}`")

	// Spellings of one number for one variant are one case.
	code = generateSpec(t, opaqueSpecHeader+`
    One:
      type: object
      properties:
        version: {type: number}
    Other:
      type: object
      properties:
        version: {type: number}
    Aliased:
      oneOf:
        - $ref: '#/components/schemas/One'
        - $ref: '#/components/schemas/Other'
      discriminator:
        propertyName: version
        mapping:
          '1': '#/components/schemas/One'
          '1.0': '#/components/schemas/One'
          '2': '#/components/schemas/Other'
`)
	aliased := methodBody(t, code, "func (t Aliased) ValueByDiscriminator(")
	assert.Contains(t, aliased, "Value *float64")
	assert.Equal(t, 1, strings.Count(aliased, "case 1:"))
	assert.Contains(t, aliased, "case 2:")
	assert.Contains(t, methodBody(t, code, "func (t *Aliased) FromOne("), "`{\"version\":1}`",
		"one value, however it is spelled, is stamped")
}

// TestInlineDiscriminatedVariantErrors: two variants can't take one
// value, and a value must fit in the generated string literals.
func TestInlineDiscriminatedVariantErrors(t *testing.T) {
	for name, tc := range map[string]struct{ spec, want string }{
		"a mapped value": {`
    Dog:
      type: object
      properties:
        petType: {type: string}
    Pet:
      oneOf:
        - type: object
          properties:
            petType: {const: dog}
        - $ref: '#/components/schemas/Dog'
      discriminator:
        propertyName: petType
        mapping:
          dog: '#/components/schemas/Dog'
`, `the inline schema Pet.0 takes the petType value "dog", which another schema is mapped to`},
		"another inline variant's value": {`
    Pet:
      oneOf:
        - type: object
          properties:
            petType: {const: cat}
        - type: object
          properties:
            petType: {const: cat}
      discriminator:
        propertyName: petType
`, `the inline schema Pet.1 takes the petType value "cat"`},
		"a $ref's implicit value": {`
    Dog:
      type: object
      properties:
        petType: {type: string}
    Pet:
      oneOf:
        - type: object
          properties:
            petType: {const: Dog}
        - $ref: '#/components/schemas/Dog'
      discriminator:
        propertyName: petType
`, `#/components/schemas/Dog takes the petType value "Dog", which an inline schema also takes`},
		"a $ref's implicit value first": {`
    Dog:
      type: object
      properties:
        petType: {type: string}
    Pet:
      oneOf:
        - $ref: '#/components/schemas/Dog'
        - type: object
          properties:
            petType: {const: Dog}
      discriminator:
        propertyName: petType
`, `the inline schema Pet.1 takes the petType value "Dog", which another schema is mapped to`},
		"pins of different types": {`
    Flag:
      oneOf:
        - type: object
          properties:
            k: {const: true}
        - type: object
          properties:
            k: {const: 1}
      discriminator:
        propertyName: k
`, "discriminator: the variants pin k values of different JSON types, boolean and number"},
		"a large integer": {`
    Big:
      oneOf:
        - type: object
          properties:
            id: {const: 9007199254740993}
      discriminator:
        propertyName: id
`, "discriminator: the id value of the inline schema Big.0 can't be read exactly"},
		"one number for two variants": {`
    One:
      type: object
      properties:
        version: {type: number}
    Other:
      type: object
      properties:
        version: {type: number}
    Versioned:
      oneOf:
        - $ref: '#/components/schemas/One'
        - $ref: '#/components/schemas/Other'
      discriminator:
        propertyName: version
        mapping:
          '1': '#/components/schemas/One'
          '1.0': '#/components/schemas/Other'
`, "discriminator: the version values 1 and 1.0 are the same number, but lead to One and Other"},
		"a backtick": {"\n    Pet:\n      oneOf:\n        - type: object\n          properties:\n            petType: {const: 'a`b'}\n      discriminator:\n        propertyName: petType\n",
			`the petType value "a` + "`" + `b" of the inline schema Pet.0 may not contain a backtick`},
		"a quote": {"\n    Pet:\n      oneOf:\n        - type: object\n          properties:\n            petType: {enum: ['a\"b']}\n      discriminator:\n        propertyName: petType\n",
			`may not contain`},
		"a backslash": {"\n    Pet:\n      oneOf:\n        - type: object\n          properties:\n            petType: {enum: ['a\\b']}\n      discriminator:\n        propertyName: petType\n",
			`may not contain`},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := generateSpecErr(opaqueSpecHeader + tc.spec)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

// TestUnionComponentsDedupe: a union an allOf brings twice is one, with the
// $ref accessors either copy has; a union of only 3.1 null branches says
// nullable, and isn't one of the unions.
func TestUnionComponentsDedupe(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader31+unionComponentsSchemas+`
    Twice:
      allOf:
        - oneOf:
            - $ref: '#/components/schemas/Card'
            - $ref: '#/components/schemas/Transfer'
        - $ref: '#/components/schemas/Payment'
        - $ref: '#/components/schemas/Delivery'
    Nullable:
      allOf:
        - oneOf:
            - type: 'null'
        - anyOf:
            - $ref: '#/components/schemas/Card'
            - type: 'null'
`)
	assert.Contains(t, code, "func (t Twice) AsPayment() (Payment, error)")
	assert.Equal(t, 1, strings.Count(code, "func (t *Twice) FromCard("))
	assert.Contains(t, code, "type Nullable = Card")
}

// TestParentListsMember: a union that lists another member of the same
// allOf, or a schema inside one, is left out whatever the members' order, and
// its discriminator goes with it.
func TestParentListsMember(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Pet:
      type: object
      required: [petType]
      properties:
        petType: {type: string}
      discriminator:
        propertyName: petType
      oneOf:
        - $ref: '#/components/schemas/Cat'
        - $ref: '#/components/schemas/Dog'
    Cat:
      allOf:
        - $ref: '#/components/schemas/Pet'
        - properties:
            meow: {type: string}
    Dog:
      allOf:
        - $ref: '#/components/schemas/Pet'
        - oneOf:
            - $ref: '#/components/schemas/Indoor'
            - $ref: '#/components/schemas/Outdoor'
    Indoor:
      type: object
      properties:
        room: {type: string}
    Outdoor:
      type: object
      properties:
        yard: {type: string}
    KittenA:
      allOf:
        - $ref: '#/components/schemas/Pet'
        - $ref: '#/components/schemas/Cat'
        - properties:
            age: {type: integer}
    KittenB:
      allOf:
        - $ref: '#/components/schemas/Cat'
        - $ref: '#/components/schemas/Pet'
        - properties:
            age: {type: integer}
    Nested:
      allOf:
        - $ref: '#/components/schemas/Pet'
        - allOf:
            - $ref: '#/components/schemas/Cat'
    Card:
      type: object
      properties:
        card: {type: string}
    Cash:
      type: object
      properties:
        amount: {type: number}
    Payment:
      oneOf:
        - $ref: '#/components/schemas/Card'
        - $ref: '#/components/schemas/Cash'
    Wallet:
      oneOf:
        - $ref: '#/components/schemas/Payment'
        - $ref: '#/components/schemas/Cash'
    BothA:
      allOf:
        - $ref: '#/components/schemas/Wallet'
        - $ref: '#/components/schemas/Payment'
    BothB:
      allOf:
        - $ref: '#/components/schemas/Payment'
        - $ref: '#/components/schemas/Wallet'
`)
	for _, name := range []string{"KittenA", "KittenB", "Nested"} {
		assert.NotContains(t, code, "func (t "+name+") AsDog()", name)
	}
	for _, name := range []string{"BothA", "BothB"} {
		assert.Contains(t, code, "func (t "+name+") AsCard() (Card, error)", name)
		assert.NotContains(t, code, "func (t "+name+") AsPayment()", name)
	}
	assert.Contains(t, code, "func (t Dog) AsIndoor() (Indoor, error)")
	assert.NotContains(t, code, "func (t Dog) ValueByDiscriminator(", "petType tells Pet's children apart, not Dog's own union")
	start := strings.Index(code, "func (t *Dog) FromIndoor(")
	require.GreaterOrEqual(t, start, 0)
	fromIndoor := code[start : start+strings.Index(code[start:], "\n}\n")]
	assert.NotContains(t, fromIndoor, `"Indoor"`, "nothing stamps petType")
}

// TestUnionComponentsDiscriminatorKey: the discriminator's property belongs
// to its union even when the variants leave it to the stamp, so setting the
// other union keeps it and setting its own union replaces it.
func TestUnionComponentsDiscriminatorKey(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Card:
      type: object
      properties:
        card: {type: string}
    Transfer:
      type: object
      properties:
        iban: {type: string}
    Courier:
      type: object
      properties:
        address: {type: string}
    Payment:
      oneOf:
        - $ref: '#/components/schemas/Card'
        - $ref: '#/components/schemas/Transfer'
      discriminator:
        propertyName: kind
    Delivery:
      oneOf:
        - $ref: '#/components/schemas/Courier'
    Order:
      allOf:
        - $ref: '#/components/schemas/Payment'
        - $ref: '#/components/schemas/Delivery'
`)
	assert.Contains(t, methodBody(t, code, "func (t *Order) FromCard("), `delete(kept, "kind")`)
	assert.Contains(t, methodBody(t, code, "func (t *Order) FromPayment("), `delete(kept, "kind")`)
	assert.NotContains(t, methodBody(t, code, "func (t *Order) FromCourier("), `delete(kept, "kind")`)
}

// TestInlineUnpinnedVariants: an inline variant that pins no discriminator
// value has none, with or without a mapping: the discriminator doesn't lead
// to it and From* stamps none, while the variants that have a value keep
// theirs.
func TestInlineUnpinnedVariants(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Dog:
      type: object
      properties:
        petType: {type: string}
    Unmapped:
      oneOf:
        - type: object
          properties:
            petType: {const: cat}
        - type: object
          properties:
            petType: {type: string}
            bark: {type: string}
        - type: object
          properties:
            petType: {type: string}
            chirp: {type: string}
      discriminator:
        propertyName: petType
    Mapped:
      oneOf:
        - type: object
          properties:
            petType: {type: string}
            meow: {type: string}
        - $ref: '#/components/schemas/Dog'
      discriminator:
        propertyName: petType
        mapping:
          dog: '#/components/schemas/Dog'
`)
	unmapped := methodBody(t, code, "func (t Unmapped) ValueByDiscriminator(")
	assert.Contains(t, unmapped, `case "cat":`)
	assert.NotContains(t, unmapped, `case "":`)
	assert.Contains(t, methodBody(t, code, "func (t *Unmapped) FromUnmapped0("), "`{\"petType\":\"cat\"}`")
	assert.NotContains(t, methodBody(t, code, "func (t *Unmapped) FromUnmapped1("), "petType")
	assert.Contains(t, code, "func (t Unmapped) AsUnmapped2() (Unmapped2, error)")

	mapped := methodBody(t, code, "func (t Mapped) ValueByDiscriminator(")
	assert.Contains(t, mapped, `case "dog":`)
	assert.NotContains(t, mapped, "Mapped0")
	assert.Contains(t, methodBody(t, code, "func (t *Mapped) FromDog("), "`{\"petType\":\"dog\"}`")
	assert.NotContains(t, methodBody(t, code, "func (t *Mapped) FromMapped0("), "petType")
}

// TestConstraintOnlyUnionMember: a member's list whose branches restate the
// type another member declares only adds constraints, also when another
// member brings a real union; and a branch that names its Go type is a type.
func TestConstraintOnlyUnionMember(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Card:
      type: object
      properties:
        card: {type: string}
    Cash:
      type: object
      properties:
        amount: {type: number}
    Mixed:
      allOf:
        - type: object
          properties:
            email: {type: string}
            phone: {type: string}
        - oneOf:
            - type: object
              required: [email]
            - type: object
              required: [phone]
        - oneOf:
            - $ref: '#/components/schemas/Card'
            - $ref: '#/components/schemas/Cash'
    Named:
      type: object
      properties:
        email: {type: string}
        phone: {type: string}
      oneOf:
        - required: [email]
          x-go-type-name: EmailContact
        - required: [phone]
          x-go-type-name: PhoneContact
`)
	assert.Contains(t, code, "func (t Mixed) AsCard() (Card, error)")
	assert.NotContains(t, code, "Mixed0", "the constraint branches are no variants")
	assert.Contains(t, code, "type EmailContact = any")
	assert.Contains(t, code, "func (t Named) AsNamed0() (Named0, error)")

	// The type the branches restate is the one the members narrow to.
	code = generateSpec(t, opaqueSpecHeader31+`
    Card:
      type: object
      properties:
        card: {type: string}
    Cash:
      type: object
      properties:
        amount: {type: number}
    Narrowed:
      allOf:
        - type: [object, string]
        - type: object
          properties:
            email: {type: string}
            phone: {type: string}
        - oneOf:
            - type: object
              required: [email]
            - type: object
              required: [phone]
        - oneOf:
            - $ref: '#/components/schemas/Card'
            - $ref: '#/components/schemas/Cash'
`)
	assert.Contains(t, code, "func (t Narrowed) AsCard() (Card, error)")
	assert.NotContains(t, code, "Narrowed0", "the constraint branches are no variants")
}

// TestDeclaredTypes: a schema's declared types are its own, narrowed by what
// its allOf members declare, as the merge intersects them.
func TestDeclaredTypes(t *testing.T) {
	types := func(ts ...string) *model.Schema { return &model.Schema{Type: (*model.Types)(&ts)} }
	allOf := func(own *model.Schema, members ...*model.Schema) *model.Schema {
		for _, m := range members {
			own.AllOf = append(own.AllOf, m.NewRef())
		}
		return own
	}
	for name, tc := range map[string]struct {
		schema *model.Schema
		want   []string
	}{
		"own type":                 {types("object", "null"), []string{"object"}},
		"properties":               {model.NewObjectSchema().WithProperty("a", model.NewStringSchema()), []string{"object"}},
		"nothing":                  {&model.Schema{}, nil},
		"a later member narrows":   {allOf(&model.Schema{}, types("object", "string"), &model.Schema{}, types("object")), []string{"object"}},
		"a member narrows its own": {allOf(types("object", "string"), types("string")), []string{"string"}},
		"integer under number":     {allOf(&model.Schema{}, types("number"), types("integer")), []string{"integer"}},
		"nested":                   {allOf(&model.Schema{}, allOf(&model.Schema{}, types("object", "array")), types("array")), []string{"array"}},
		"no value":                 {allOf(&model.Schema{}, types("object"), types("string")), nil},
	} {
		t.Run(name, func(t *testing.T) {
			got := declaredTypes(tc.schema)
			if len(tc.want) == 0 {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestUnionComponentsVariantKeys: a variant's keys are the names its fields
// marshal as, from allOf members however deep; a variant of two unions
// replaces what each of them owns.
func TestUnionComponentsVariantKeys(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+`
    Card:
      type: object
      properties:
        card:
          type: string
          x-oapi-codegen-extra-tags:
            json: card_number,omitempty
        note:
          type: string
          x-go-json-ignore: true
    Transfer:
      type: object
      properties:
        iban: {type: string}
    Courier:
      type: object
      properties:
        address: {type: string}
    Shared:
      allOf:
        - oneOf:
            - $ref: '#/components/schemas/Card'
            - $ref: '#/components/schemas/Transfer'
        - oneOf:
            - $ref: '#/components/schemas/Card'
            - $ref: '#/components/schemas/Courier'
    Renamed:
      allOf:
        - oneOf:
            - $ref: '#/components/schemas/Card'
            - $ref: '#/components/schemas/Transfer'
        - oneOf:
            - $ref: '#/components/schemas/Courier'
    Nested:
      allOf:
        - oneOf:
            - $ref: '#/components/schemas/Deep0'
            - $ref: '#/components/schemas/Transfer'
        - oneOf:
            - $ref: '#/components/schemas/Courier'
    Deep0:
      allOf:
        - $ref: '#/components/schemas/Deep1'
    Deep1:
      allOf:
        - $ref: '#/components/schemas/Deep2'
    Deep2:
      allOf:
        - $ref: '#/components/schemas/Deep3'
    Deep3:
      allOf:
        - $ref: '#/components/schemas/Deep4'
    Deep4:
      allOf:
        - $ref: '#/components/schemas/Deep5'
    Deep5:
      allOf:
        - $ref: '#/components/schemas/Deep6'
    Deep6:
      allOf:
        - $ref: '#/components/schemas/Deep7'
    Deep7:
      allOf:
        - $ref: '#/components/schemas/Deep8'
    Deep8:
      allOf:
        - $ref: '#/components/schemas/Deep9'
    Deep9:
      allOf:
        - $ref: '#/components/schemas/Deep10'
    Deep10:
      type: object
      properties:
        deep: {type: string}
`)
	fromCard := methodBody(t, code, "func (t *Shared) FromCard(")
	assert.Contains(t, fromCard, `delete(kept, "iban")`, "Card is the first union's")
	assert.Contains(t, fromCard, `delete(kept, "address")`, "and the second's")
	assert.NotContains(t, methodBody(t, code, "func (t *Shared) FromTransfer("), `delete(kept, "card`, "Card's keys are shared")

	fromTransfer := methodBody(t, code, "func (t *Renamed) FromTransfer(")
	assert.Contains(t, fromTransfer, `delete(kept, "card_number")`)
	assert.NotContains(t, fromTransfer, `delete(kept, "card")`)
	assert.NotContains(t, fromTransfer, `delete(kept, "note")`, "an ignored field isn't on the wire")

	assert.Contains(t, methodBody(t, code, "func (t *Nested) FromTransfer("), `delete(kept, "deep")`)
}

// TestDiscriminatorStampsPerVariant: each variant that exactly one
// discriminator value leads to is stamped, so aliases for one variant don't
// take the stamps of the others away (#2071).
func TestDiscriminatorStampsPerVariant(t *testing.T) {
	spec := opaqueSpecHeader + `
    Dog:
      type: object
      properties:
        petType: {type: string}
    Cat:
      type: object
      properties:
        petType: {type: string}
    Pet:
      oneOf:
        - type: object
          properties:
            petType: {const: bird}
        - $ref: '#/components/schemas/Cat'
        - $ref: '#/components/schemas/Dog'
      discriminator:
        propertyName: petType
        mapping:
          dog: '#/components/schemas/Dog'
          canine: '#/components/schemas/Dog'
          cat: '#/components/schemas/Cat'
`
	code := generateSpec(t, spec)
	assert.Contains(t, methodBody(t, code, "func (t *Pet) FromPet0("), "`{\"petType\":\"bird\"}`")
	assert.Contains(t, methodBody(t, code, "func (t *Pet) FromCat("), "`{\"petType\":\"cat\"}`")
	assert.NotContains(t, methodBody(t, code, "func (t *Pet) FromDog("), "JSONMerge", "dog or canine would be arbitrary")
	valueBy := methodBody(t, code, "func (t Pet) ValueByDiscriminator(")
	for _, value := range []string{"bird", "canine", "cat", "dog"} {
		assert.Contains(t, valueBy, `case "`+value+`":`)
	}
}

// TestNumberLiteral: a value of an integer or number discriminator becomes a
// Go literal of the type it is read as, int64 or float64: the value itself,
// exactly, whatever its spelling.
func TestNumberLiteral(t *testing.T) {
	for _, tc := range []struct{ text, valueType, want, err string }{
		{"1", "integer", "1", ""},
		{"-0", "integer", "0", ""},
		{"9007199254740993", "integer", "9007199254740993", ""},
		{"9223372036854775807", "integer", "9223372036854775807", ""},
		{"1.0", "integer", "1", ""},
		{"1e3", "integer", "1000", ""},
		{"2.5", "integer", "", "isn't an integer"},
		{"9223372036854775808", "integer", "", "doesn't fit in an int64"},
		{"9007199254740993.0", "integer", "", "can't be read exactly"},
		{"1", "number", "1", ""},
		{"1.0", "number", "1", ""},
		{"2.50", "number", "2.5", ""},
		{"25e-1", "number", "2.5", ""},
		{"1e21", "number", "1e+21", ""},
		{"-0", "number", "0", ""},
		{"-0.0e5", "number", "0", ""},
		{"1e400", "number", "", "doesn't fit in a float64"},
		{"007", "integer", "", "isn't a JSON number"},
		{"0x10", "integer", "", "isn't a JSON number"},
		{" 1", "number", "", "isn't a JSON number"},
		{"one", "number", "", "isn't a JSON number"},
	} {
		got, err := numberLiteral(tc.text, tc.valueType)
		if tc.err != "" {
			assert.ErrorContains(t, err, tc.err, "%s %s", tc.valueType, tc.text)
			continue
		}
		require.NoError(t, err, "%s %s", tc.valueType, tc.text)
		assert.Equal(t, tc.want, got, "%s %s", tc.valueType, tc.text)
	}
}

// TestNumericMappingKeyErrors: a mapping key of an integer or number
// discriminator must be a number of that kind.
func TestNumericMappingKeyErrors(t *testing.T) {
	_, err := generateSpecErr(opaqueSpecHeader + `
    One:
      type: object
      properties:
        version: {type: integer}
    Versioned:
      oneOf:
        - $ref: '#/components/schemas/One'
      discriminator:
        propertyName: version
        mapping:
          '2.5': '#/components/schemas/One'
`)
	assert.ErrorContains(t, err, "discriminator: the version value 2.5 isn't an integer")

	_, err = generateSpecErr(opaqueSpecHeader + `
    One:
      type: object
      properties:
        version: {type: number}
    Other:
      type: object
      properties:
        version: {type: number}
    Versioned:
      oneOf:
        - $ref: '#/components/schemas/One'
        - $ref: '#/components/schemas/Other'
      discriminator:
        propertyName: version
        mapping:
          '0': '#/components/schemas/One'
          '-0': '#/components/schemas/Other'
`)
	assert.ErrorContains(t, err, "discriminator: the version values -0 and 0 are the same number, but lead to Other and One")
}

// TestClosedUnionVariants: As* of a variant with additionalProperties: false
// allows the keys the variant and the rest of the object declare (issue
// #668), unless they can't all be known, and v1/v2 and
// lenient-union-accessors leave As* as it was.
func TestClosedUnionVariants(t *testing.T) {
	spec := opaqueSpecHeader + `
    Renamed:
      type: object
      additionalProperties: false
      properties:
        card:
          type: string
          x-oapi-codegen-extra-tags:
            json: card_number,omitempty
        note:
          type: string
          x-go-json-ignore: true
    Open:
      type: object
      properties:
        iban: {type: string}
    Typed:
      type: object
      additionalProperties: false
      x-go-type: string
      properties:
        a: {type: string}
    Nesting:
      type: object
      additionalProperties: false
      properties:
        a: {type: string}
      oneOf:
        - $ref: '#/components/schemas/Open'
        - $ref: '#/components/schemas/Renamed'
    Constrained:
      type: object
      additionalProperties: false
      properties:
        a: {type: string}
        b: {type: string}
      oneOf:
        - required: [a]
        - required: [b]
    Deep:
      allOf:
        - $ref: '#/components/schemas/DeepBase'
        - type: object
          properties:
            c: {type: string}
    DeepBase:
      allOf:
        - type: object
          additionalProperties: false
          properties:
            d: {type: string}
    Restated:
      type: object
      additionalProperties: false
      properties:
        a: {type: string}
      allOf:
        - oneOf:
            - type: object
              required: [a]
            - type: object
    Empty:
      type: object
      additionalProperties: false
    Pet:
      oneOf:
        - $ref: '#/components/schemas/Renamed'
        - $ref: '#/components/schemas/Open'
        - $ref: '#/components/schemas/Typed'
        - $ref: '#/components/schemas/Nesting'
        - $ref: '#/components/schemas/Constrained'
        - $ref: '#/components/schemas/Deep'
        - $ref: '#/components/schemas/Empty'
        - $ref: '#/components/schemas/Restated'
    WithTyped:
      allOf:
        - oneOf:
            - $ref: '#/components/schemas/Renamed'
            - $ref: '#/components/schemas/Open'
        - oneOf:
            - $ref: '#/components/schemas/Typed'
            - $ref: '#/components/schemas/Deep'
`
	code := generateSpec(t, spec)
	assert.Contains(t, methodBody(t, code, "func (t Pet) AsRenamed("), `case "card", "card_number", "note":`,
		"a renamed key under both names, an ignored one under its own")
	assert.Contains(t, methodBody(t, code, "func (t Pet) AsConstrained("), `case "a", "b":`,
		"a oneOf of constraints declares nothing")
	assert.Contains(t, methodBody(t, code, "func (t Pet) AsDeep("), `case "c", "d":`,
		"a member however deep closes the variant")
	assert.Contains(t, methodBody(t, code, "func (t Pet) AsRestated("), `case "a":`,
		"a typeless member's oneOf that restates the variant's type only constrains it")
	empty := methodBody(t, code, "func (t Pet) AsEmpty(")
	assert.Contains(t, empty, `Empty doesn't allow the property %q`)
	assert.NotContains(t, empty, "switch key")
	for _, lenient := range []string{"AsOpen", "AsTyped", "AsNesting"} {
		assert.NotContains(t, methodBody(t, code, "func (t Pet) "+lenient+"("), "doesn't allow", lenient)
	}

	assert.NotContains(t, methodBody(t, code, "func (t WithTyped) AsRenamed("), "doesn't allow",
		"the other union has a variant whose keys can't be known")
	assert.Contains(t, methodBody(t, code, "func (t WithTyped) AsDeep("), `case "c", "card", "card_number", "d", "iban", "note":`)

	lenientCode := generateSpec(t, spec, func(c *Configuration) { c.OutputOptions.LenientUnionAccessors = true })
	assert.NotContains(t, lenientCode, "doesn't allow", "lenient")
}

// TestPlacedDiscriminator: As* of a union that is a $ref leaves out a
// discriminator the merge gave it only when neither the union type nor any of
// its variants may declare it.
func TestPlacedDiscriminator(t *testing.T) {
	union := func(d *model.Discriminator) *model.SchemaRef {
		return &model.SchemaRef{Ref: "#/components/schemas/Payment", Value: &model.Schema{Discriminator: d}}
	}
	method := &model.Discriminator{PropertyName: "method"}
	for name, test := range map[string]struct {
		c      unionComponent
		keys   []string
		opaque bool
		want   bool
	}{
		"placed":             {unionComponent{discriminator: method, ref: union(nil)}, []string{"card"}, false, true},
		"no discriminator":   {unionComponent{ref: union(nil)}, []string{"card"}, false, false},
		"inline union":       {unionComponent{discriminator: method}, []string{"card"}, false, false},
		"the union's own":    {unionComponent{discriminator: method, ref: union(method)}, []string{"card"}, false, false},
		"a variant declares": {unionComponent{discriminator: method, ref: union(nil)}, []string{"card", "method"}, false, false},
		"an opaque variant":  {unionComponent{discriminator: method, ref: union(nil)}, []string{"card"}, true, false},
	} {
		assert.Equal(t, test.want, placedDiscriminator(test.c, test.keys, test.opaque), name)
	}
}

// TestFieldKey: the key a struct field marshals as, from its json tag as
// GenFieldsFromProperties writes it: the template with the field's own flags,
// x-go-json-ignore, or x-oapi-codegen-extra-tags.
func TestFieldKey(t *testing.T) {
	options, generator := globalState.options, globalState.fieldTagGenerator
	t.Cleanup(func() { globalState.options, globalState.fieldTagGenerator = options, generator })
	globalState.options = Configuration{}
	var err error
	globalState.fieldTagGenerator, err = newStructTagGenerator(StructTagsConfig{Tags: []StructTagTemplate{
		{Name: "json", Template: `{{if .IsOptional}}opt_{{end}}{{.FieldName}}{{if .OmitEmpty}},omitempty{{end}}`},
	}})
	require.NoError(t, err)
	for name, test := range map[string]struct {
		p    Property
		want string
	}{
		"required":            {Property{JsonFieldName: "card_number", Required: true}, "card_number"},
		"optional":            {Property{JsonFieldName: "card_number"}, "opt_card_number"},
		"extra tag":           {Property{JsonFieldName: "card_number", Extensions: map[string]any{extPropExtraTags: map[string]any{"json": "number,omitempty"}}}, "number"},
		"extra tag, no name":  {Property{JsonFieldName: "card_number", Extensions: map[string]any{extPropExtraTags: map[string]any{"json": ",omitempty"}}}, "CardNumber"},
		"extra tag, left off": {Property{JsonFieldName: "card_number", Extensions: map[string]any{extPropExtraTags: map[string]any{"json": "-"}}}, ""},
		"ignored":             {Property{JsonFieldName: "card_number", Extensions: map[string]any{extPropGoJsonIgnore: true}}, ""},
	} {
		key, ok := fieldKey(test.p)
		assert.Equal(t, test.want, key, name)
		assert.Equal(t, test.want != "", ok, name)
	}
}
