package model

import "slices"

// JSON Schema type names, matching kin-openapi's constants.
const (
	TypeArray   = "array"
	TypeBoolean = "boolean"
	TypeInteger = "integer"
	TypeNumber  = "number"
	TypeObject  = "object"
	TypeString  = "string"
	TypeNull    = "null"
)

// SchemaRefs is a list of schema references, as in allOf, anyOf and oneOf.
type SchemaRefs []*SchemaRef

// Schemas is a map of named schemas, as in properties and components.
type Schemas map[string]*SchemaRef

// Schema is an OpenAPI Schema Object. Field order and json keywords match
// kin-openapi's Schema: the generator reflects over every field to name the
// keywords an allOf merge cannot fold, so a missing or reordered field
// changes that result.
type Schema struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	OneOf        SchemaRefs    `json:"oneOf,omitempty" yaml:"oneOf,omitempty"`
	AnyOf        SchemaRefs    `json:"anyOf,omitempty" yaml:"anyOf,omitempty"`
	AllOf        SchemaRefs    `json:"allOf,omitempty" yaml:"allOf,omitempty"`
	Not          *SchemaRef    `json:"not,omitempty" yaml:"not,omitempty"`
	Type         *Types        `json:"type,omitempty" yaml:"type,omitempty"`
	Title        string        `json:"title,omitempty" yaml:"title,omitempty"`
	Format       string        `json:"format,omitempty" yaml:"format,omitempty"`
	Description  string        `json:"description,omitempty" yaml:"description,omitempty"`
	Enum         []any         `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default      any           `json:"default,omitempty" yaml:"default,omitempty"`
	Example      any           `json:"example,omitempty" yaml:"example,omitempty"`
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`

	// Array-related, here for struct compactness
	UniqueItems bool `json:"uniqueItems,omitempty" yaml:"uniqueItems,omitempty"`
	// Number-related, here for struct compactness
	ExclusiveMin ExclusiveBound `json:"exclusiveMinimum,omitempty" yaml:"exclusiveMinimum,omitempty"` // Number for v3.1+ otherwise boolean
	ExclusiveMax ExclusiveBound `json:"exclusiveMaximum,omitempty" yaml:"exclusiveMaximum,omitempty"` // Number for v3.1+ otherwise boolean
	// Properties
	Nullable        bool `json:"nullable,omitempty" yaml:"nullable,omitempty"`
	ReadOnly        bool `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
	WriteOnly       bool `json:"writeOnly,omitempty" yaml:"writeOnly,omitempty"`
	AllowEmptyValue bool `json:"allowEmptyValue,omitempty" yaml:"allowEmptyValue,omitempty"`
	Deprecated      bool `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	XML             *XML `json:"xml,omitempty" yaml:"xml,omitempty"`

	// Number
	Min        *float64 `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Max        *float64 `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	MultipleOf *float64 `json:"multipleOf,omitempty" yaml:"multipleOf,omitempty"`

	// String
	MinLength uint64  `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength *uint64 `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Pattern   string  `json:"pattern,omitempty" yaml:"pattern,omitempty"`

	// Array
	MinItems uint64     `json:"minItems,omitempty" yaml:"minItems,omitempty"`
	MaxItems *uint64    `json:"maxItems,omitempty" yaml:"maxItems,omitempty"`
	Items    *SchemaRef `json:"items,omitempty" yaml:"items,omitempty"`

	// Object
	Required             []string             `json:"required,omitempty" yaml:"required,omitempty"`
	Properties           Schemas              `json:"properties,omitempty" yaml:"properties,omitempty"`
	MinProps             uint64               `json:"minProperties,omitempty" yaml:"minProperties,omitempty"`
	MaxProps             *uint64              `json:"maxProperties,omitempty" yaml:"maxProperties,omitempty"`
	AdditionalProperties AdditionalProperties `json:"additionalProperties,omitempty" yaml:"additionalProperties,omitempty"`
	Discriminator        *Discriminator       `json:"discriminator,omitempty" yaml:"discriminator,omitempty"`

	Const                 any        `json:"const,omitempty" yaml:"const,omitempty"`                                 // OpenAPI >=3.1
	Examples              []any      `json:"examples,omitempty" yaml:"examples,omitempty"`                           // OpenAPI >=3.1
	PrefixItems           SchemaRefs `json:"prefixItems,omitempty" yaml:"prefixItems,omitempty"`                     // OpenAPI >=3.1
	Contains              *SchemaRef `json:"contains,omitempty" yaml:"contains,omitempty"`                           // OpenAPI >=3.1
	MinContains           *uint64    `json:"minContains,omitempty" yaml:"minContains,omitempty"`                     // OpenAPI >=3.1
	MaxContains           *uint64    `json:"maxContains,omitempty" yaml:"maxContains,omitempty"`                     // OpenAPI >=3.1
	PatternProperties     Schemas    `json:"patternProperties,omitempty" yaml:"patternProperties,omitempty"`         // OpenAPI >=3.1
	DependentSchemas      Schemas    `json:"dependentSchemas,omitempty" yaml:"dependentSchemas,omitempty"`           // OpenAPI >=3.1
	PropertyNames         *SchemaRef `json:"propertyNames,omitempty" yaml:"propertyNames,omitempty"`                 // OpenAPI >=3.1
	UnevaluatedItems      BoolSchema `json:"unevaluatedItems,omitempty" yaml:"unevaluatedItems,omitempty"`           // OpenAPI >=3.1
	UnevaluatedProperties BoolSchema `json:"unevaluatedProperties,omitempty" yaml:"unevaluatedProperties,omitempty"` // OpenAPI >=3.1

	If   *SchemaRef `json:"if,omitempty" yaml:"if,omitempty"`     // OpenAPI >=3.1
	Then *SchemaRef `json:"then,omitempty" yaml:"then,omitempty"` // OpenAPI >=3.1
	Else *SchemaRef `json:"else,omitempty" yaml:"else,omitempty"` // OpenAPI >=3.1

	DependentRequired map[string][]string `json:"dependentRequired,omitempty" yaml:"dependentRequired,omitempty"` // OpenAPI >=3.1

	// Always is set when the schema was written as a JSON Schema boolean
	// schema rather than an object: true accepts every instance, false
	// accepts none. Nil for an ordinary object schema. OpenAPI >=3.1.
	Always *bool `json:"-" yaml:"-"`

	Defs          Schemas `json:"$defs,omitempty" yaml:"$defs,omitempty"`       // OpenAPI >=3.1
	SchemaDialect string  `json:"$schema,omitempty" yaml:"$schema,omitempty"`   // OpenAPI >=3.1
	Comment       string  `json:"$comment,omitempty" yaml:"$comment,omitempty"` // OpenAPI >=3.1

	SchemaID      string `json:"$id,omitempty" yaml:"$id,omitempty"`                       // OpenAPI >=3.1
	Anchor        string `json:"$anchor,omitempty" yaml:"$anchor,omitempty"`               // OpenAPI >=3.1
	DynamicRef    string `json:"$dynamicRef,omitempty" yaml:"$dynamicRef,omitempty"`       // OpenAPI >=3.1
	DynamicAnchor string `json:"$dynamicAnchor,omitempty" yaml:"$dynamicAnchor,omitempty"` // OpenAPI >=3.1

	ContentMediaType string     `json:"contentMediaType,omitempty" yaml:"contentMediaType,omitempty"` // OpenAPI >=3.1
	ContentEncoding  string     `json:"contentEncoding,omitempty" yaml:"contentEncoding,omitempty"`   // OpenAPI >=3.1
	ContentSchema    *SchemaRef `json:"contentSchema,omitempty" yaml:"contentSchema,omitempty"`       // OpenAPI >=3.1
}

// NewSchema builds an empty schema.
func NewSchema() *Schema {
	return &Schema{}
}

// NewStringSchema builds a schema of type string.
func NewStringSchema() *Schema {
	return &Schema{Type: &Types{TypeString}}
}

// NewObjectSchema builds a schema of type object with an empty property map.
func NewObjectSchema() *Schema {
	return &Schema{
		Type:       &Types{TypeObject},
		Properties: make(Schemas),
	}
}

// NewRef builds a SchemaRef whose value is schema.
func (schema *Schema) NewRef() *SchemaRef {
	return &SchemaRef{Value: schema}
}

// WithEnum sets the schema's enum values.
func (schema *Schema) WithEnum(values ...any) *Schema {
	schema.Enum = values
	return schema
}

// WithMaxLength sets maxLength.
func (schema *Schema) WithMaxLength(i int64) *Schema {
	n := uint64(i)
	schema.MaxLength = &n
	return schema
}

// WithProperty sets a property to propertySchema.
func (schema *Schema) WithProperty(name string, propertySchema *Schema) *Schema {
	return schema.WithPropertyRef(name, &SchemaRef{Value: propertySchema})
}

// WithPropertyRef sets a property to ref.
func (schema *Schema) WithPropertyRef(name string, ref *SchemaRef) *Schema {
	if schema.Properties == nil {
		schema.Properties = make(Schemas)
	}
	schema.Properties[name] = ref
	return schema
}

// WithAnyAdditionalProperties sets additionalProperties to true.
func (schema *Schema) WithAnyAdditionalProperties() *Schema {
	has := true
	schema.AdditionalProperties = AdditionalProperties{Has: &has}
	return schema
}

// WithoutAdditionalProperties sets additionalProperties to false.
func (schema *Schema) WithoutAdditionalProperties() *Schema {
	has := false
	schema.AdditionalProperties = AdditionalProperties{Has: &has}
	return schema
}

// Types is the type or types of a schema. OpenAPI 3.0 has one entry;
// OpenAPI 3.1 may have several, including "null".
type Types []string

// Is reports whether the schema has exactly one type and it is typ.
func (types *Types) Is(typ string) bool {
	return types != nil && len(*types) == 1 && (*types)[0] == typ
}

// Slice returns the types as a string slice, or nil when types is nil.
func (types *Types) Slice() []string {
	if types == nil {
		return nil
	}
	return *types
}

// Includes reports whether typ is one of the types. Nil types include nothing.
func (types *Types) Includes(typ string) bool {
	if types == nil {
		return false
	}
	return slices.Contains(*types, typ)
}

// Permits reports whether typ is allowed. Nil types permit every type.
func (types *Types) Permits(typ string) bool {
	if types == nil {
		return true
	}
	return types.Includes(typ)
}

// BoolSchema is a keyword that is either a boolean or a schema, used for
// additionalProperties, unevaluatedProperties and unevaluatedItems.
type BoolSchema struct {
	Has    *bool
	Schema *SchemaRef
}

// AdditionalProperties is a BoolSchema. The name matches kin-openapi.
type AdditionalProperties = BoolSchema

// ExclusiveBound is exclusiveMinimum or exclusiveMaximum. OpenAPI 3.0
// stores a boolean in Bool; OpenAPI 3.1 stores the bound in Value.
type ExclusiveBound struct {
	Bool  *bool
	Value *float64
}

// Discriminator is an OpenAPI Discriminator Object.
type Discriminator struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	PropertyName string                `json:"propertyName" yaml:"propertyName"`
	Mapping      map[string]MappingRef `json:"mapping,omitempty" yaml:"mapping,omitempty"`
}

// MappingRef is one discriminator mapping value. Kin serializes it as a
// plain string; the generator reads Ref.
type MappingRef struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Ref   string
	Value *Schema
}

// XML is an OpenAPI XML Object.
type XML struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Name      string `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Prefix    string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	Attribute bool   `json:"attribute,omitempty" yaml:"attribute,omitempty"`
	Wrapped   bool   `json:"wrapped,omitempty" yaml:"wrapped,omitempty"`
}

// ExternalDocs is an OpenAPI External Documentation Object.
type ExternalDocs struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	URL         string `json:"url,omitempty" yaml:"url,omitempty"`
}
