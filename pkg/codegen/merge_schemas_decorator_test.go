package codegen

// These tests cover how an allOf of a $ref and members that only annotate it
// generates: as that $ref's type, rather than as a copy of the schema it
// refers to.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const decoratorComponents = `
    Named:
      type: object
      properties:
        name: {type: string}
    Color:
      type: string
      enum: [red, green]
`

func TestAnnotatedRef(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+decoratorComponents+`
    Described:
      allOf:
        - $ref: '#/components/schemas/Named'
        - description: A described Named.
    Restated:
      allOf:
        - $ref: '#/components/schemas/Named'
        - type: object
          description: A Named that says it's an object.
    Paint:
      allOf:
        - $ref: '#/components/schemas/Color'
        - description: The paint's color.
    Renamed:
      allOf:
        - $ref: '#/components/schemas/Named'
        - x-go-type-name: TheName
    Wrapped:
      allOf:
        - allOf:
            - $ref: '#/components/schemas/Named'
            - nullable: true
        - description: A wrapper of a wrapper.
    Holder:
      type: object
      required: [plain]
      properties:
        maybe:
          allOf:
            - $ref: '#/components/schemas/Named'
            - nullable: true
              readOnly: true
        plain:
          allOf:
            - $ref: '#/components/schemas/Named'
            - description: Always there.
        noPointer:
          allOf:
            - $ref: '#/components/schemas/Named'
            - x-go-type-skip-optional-pointer: true
        color:
          allOf:
            - $ref: '#/components/schemas/Color'
            - default: red
`)
	for _, alias := range []string{
		"type Described = Named",
		"type Restated = Named",
		"type Paint = Color",
		"type Renamed = TheName",
		"type TheName = Named",
		"type Wrapped = Named",
	} {
		assert.Contains(t, code, alias)
	}
	assertField(t, code, "Maybe", "*Named")
	assertField(t, code, "Plain", "Named")
	assertField(t, code, "NoPointer", "Named")
	assertField(t, code, "Color", "*Color")
	assert.NotContains(t, code, "PaintRed", "an enum decorated with a description is the same enum, with no constants of its own")
	assert.NotContains(t, code, "HolderColor")
}

// TestAnnotatedRefMerges: a member that shapes the type, or a type that
// disagrees with the $ref's, makes a new type, as before.
func TestAnnotatedRefMerges(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+decoratorComponents+`
    RequiredName:
      allOf:
        - $ref: '#/components/schemas/Named'
        - required: [name]
    Warm:
      allOf:
        - $ref: '#/components/schemas/Color'
        - enum: [red]
`)
	assert.Contains(t, code, "type RequiredName struct {")
	assert.Contains(t, code, `WarmRed Warm = "red"`)

	_, err := generateSpecErr(opaqueSpecHeader + decoratorComponents + `
    Odd:
      allOf:
        - $ref: '#/components/schemas/Named'
        - type: string
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no value has both types")
}

func TestAnnotatedRef_31(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader31+decoratorComponents+`
    Holder:
      type: object
      properties:
        maybe:
          allOf:
            - $ref: '#/components/schemas/Named'
            - type: 'null'
`)
	assertField(t, code, "Maybe", "*Named")
}

// TestAnnotatedRefAliases: a chain of annotated $refs is a chain of
// aliases, but a component is never made an alias of itself.
func TestAnnotatedRefAliases(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+decoratorComponents+`
    Once:
      allOf:
        - $ref: '#/components/schemas/Named'
        - description: Once.
    Twice:
      allOf:
        - $ref: '#/components/schemas/Once'
        - description: Twice.
`)
	assert.Contains(t, code, "type Once = Named")
	assert.Contains(t, code, "type Twice = Once")

	for name, spec := range map[string]string{
		"itself": `
    Self:
      allOf:
        - $ref: '#/components/schemas/Self'
        - description: Itself.
`,
		"each other": `
    Ping:
      allOf:
        - $ref: '#/components/schemas/Pong'
        - description: Pong.
    Pong:
      allOf:
        - $ref: '#/components/schemas/Ping'
        - description: Ping.
`,
	} {
		t.Run(name, func(t *testing.T) {
			code := generateSpec(t, opaqueSpecHeader+spec)
			for _, alias := range []string{"type Self = Self", "type Ping = Pong", "type Pong = Ping"} {
				assert.NotContains(t, code, alias)
			}
			assert.False(t, strings.Contains(code, "type Ping = Pong") && strings.Contains(code, "type Pong = Ping"))
		})
	}
}

// TestAnnotatedRefKeepsCustomJSON: a response whose schema is an alias of
// a type with generated MarshalJSON, here for additionalProperties, gets an
// envelope that delegates to it (issue #2549), whether the alias comes from a
// single-member allOf or a $ref with annotations.
func TestAnnotatedRefKeepsCustomJSON(t *testing.T) {
	const spec = `openapi: 3.0.3
info: {title: repro, version: "1.0.0"}
paths:
  /single:
    get:
      operationId: getSingle
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {$ref: '#/components/schemas/Single'}
  /decorated:
    get:
      operationId: getDecorated
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {$ref: '#/components/schemas/Decorated'}
components:
  schemas:
    Open:
      type: object
      properties:
        b: {type: string}
      additionalProperties: {type: string}
    Single:
      allOf:
        - $ref: '#/components/schemas/Open'
    Decorated:
      allOf:
        - $ref: '#/components/schemas/Open'
        - description: A decorated Open.
`
	code := generateSpec(t, spec, func(c *Configuration) {
		c.Generate.Server = true
	})
	assert.Contains(t, code, "type Decorated = Open")
	for _, envelope := range []string{"GetSingle200JSONResponse", "GetDecorated200JSONResponse"} {
		assert.Regexp(t, `func \(\w+ `+envelope+`\) MarshalJSON\(\)`, code)
	}
}

// TestAnnotatedRefRestatements: a member that restates the $ref's own type
// and format only annotates it, whether it's a member, nested, or the
// schema's own keywords; one that narrows or changes the type doesn't.
func TestAnnotatedRefRestatements(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+decoratorComponents+`
    Amount: {type: number}
    Id: {type: string, format: uuid}
    Untyped:
      properties:
        a: {type: string}
    Typed:
      type: string
      x-go-type: MyString
    Nested:
      allOf:
        - allOf:
            - $ref: '#/components/schemas/Named'
            - type: object
        - description: A restatement inside a wrapper.
    OwnType:
      type: object
      allOf:
        - $ref: '#/components/schemas/Named'
    FormatToo:
      allOf:
        - $ref: '#/components/schemas/Id'
        - type: string
          format: uuid
    ObjectByProperties:
      allOf:
        - $ref: '#/components/schemas/Untyped'
        - type: object
    Twice:
      allOf:
        - $ref: '#/components/schemas/Named'
        - $ref: '#/components/schemas/Named'
    TypedAgain:
      allOf:
        - $ref: '#/components/schemas/Typed'
        - type: string
          minLength: 1
    Narrowed:
      allOf:
        - $ref: '#/components/schemas/Amount'
        - type: integer
`)
	for _, alias := range []string{
		"type Nested = Named",
		"type OwnType = Named",
		"type FormatToo = Id",
		"type ObjectByProperties = Untyped",
		"type Twice = Named",
		"type TypedAgain = Typed",
	} {
		assert.Contains(t, code, alias)
	}
	assert.Contains(t, code, "type Narrowed = int", "integer narrows number, so it isn't a restatement")

	_, err := generateSpecErr(opaqueSpecHeader + `
    Typed:
      type: string
      x-go-type: MyString
    Odd:
      allOf:
        - $ref: '#/components/schemas/Typed'
        - type: integer
`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "allOf can't merge #/components/schemas/Typed with an inline schema with type")
}

// TestAnnotatedRefDescription: an annotating member's description describes
// the alias.
func TestAnnotatedRefDescription(t *testing.T) {
	code := generateSpec(t, opaqueSpecHeader+decoratorComponents+`
    Described:
      allOf:
        - $ref: '#/components/schemas/Named'
        - description: A described Named.
`)
	assert.Contains(t, code, "// Described A described Named.\ntype Described = Named")
}

// TestAnnotatedRefCustomJSONThroughAliases: a response that reaches a type
// with generated MarshalJSON through x-go-type-name, or through a long chain
// of annotated $refs, still gets an envelope that delegates to it.
func TestAnnotatedRefCustomJSONThroughAliases(t *testing.T) {
	var spec strings.Builder
	spec.WriteString(`openapi: 3.0.3
info: {title: repro, version: "1.0.0"}
paths:
  /named:
    get:
      operationId: getNamed
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema:
                allOf:
                  - $ref: '#/components/schemas/Open'
                  - x-go-type-name: NamedOpen
  /chain:
    get:
      operationId: getChain
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {$ref: '#/components/schemas/Link60'}
components:
  schemas:
    Open:
      type: object
      properties:
        b: {type: string}
      additionalProperties: {type: string}
    Link0:
      $ref: '#/components/schemas/Open'
`)
	for i := 1; i <= 60; i++ {
		fmt.Fprintf(&spec, "    Link%d:\n      allOf:\n        - $ref: '#/components/schemas/Link%d'\n        - description: Link %d.\n", i, i-1, i)
	}
	code := generateSpec(t, spec.String(), func(c *Configuration) {
		c.Generate.Server = true
	})
	assert.Contains(t, code, "type Link60 = Link59")
	assert.Contains(t, code, "type NamedOpen = Open")
	for _, envelope := range []string{"GetNamed200JSONResponse", "GetChain200JSONResponse"} {
		assert.Regexp(t, `func \(\w+ `+envelope+`\) MarshalJSON\(\)`, code)
	}
}
