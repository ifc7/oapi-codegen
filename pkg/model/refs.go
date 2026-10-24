package model

// SchemaRef is a schema or a $ref to one. Ref is the authored reference.
// When both are set, Ref names the reference and Value is what it resolved to.
type SchemaRef struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Ref   string
	Value *Schema
}

// NewSchemaRef builds a SchemaRef.
func NewSchemaRef(ref string, value *Schema) *SchemaRef {
	return &SchemaRef{Ref: ref, Value: value}
}

// ParameterRef is a parameter or a $ref to one.
type ParameterRef struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`
	// Summary and Description are OpenAPI 3.1 reference-object fields.
	Summary     *string `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`

	Ref   string
	Value *Parameter
}

// RequestBodyRef is a request body or a $ref to one.
type RequestBodyRef struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`
	// Summary and Description are OpenAPI 3.1 reference-object fields.
	Summary     *string `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`

	Ref   string
	Value *RequestBody
}

// ResponseRef is a response or a $ref to one.
type ResponseRef struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`
	// Summary and Description are OpenAPI 3.1 reference-object fields.
	Summary     *string `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`

	Ref   string
	Value *Response
}

// HeaderRef is a header or a $ref to one.
type HeaderRef struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`
	// Summary and Description are OpenAPI 3.1 reference-object fields.
	Summary     *string `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`

	Ref   string
	Value *Header
}

// CallbackRef is a callback or a $ref to one.
type CallbackRef struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`
	// Summary and Description are OpenAPI 3.1 reference-object fields.
	Summary     *string `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`

	Ref   string
	Value *Callback
}

// ExampleRef is an example or a $ref to one. The generator tracks the
// reference; it does not read the example value.
type ExampleRef struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`
	// Summary and Description are OpenAPI 3.1 reference-object fields.
	Summary     *string `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`

	Ref   string
	Value *Example
}

// LinkRef is a link or a $ref to one. The generator tracks the reference;
// it does not read the link.
type LinkRef struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`
	// Summary and Description are OpenAPI 3.1 reference-object fields.
	Summary     *string `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`

	Ref   string
	Value *Link
}

// SecuritySchemeRef is a security scheme or a $ref to one.
type SecuritySchemeRef struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`
	// Summary and Description are OpenAPI 3.1 reference-object fields.
	Summary     *string `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description *string `json:"description,omitempty" yaml:"description,omitempty"`

	Ref   string
	Value *SecurityScheme
}

// Example is an OpenAPI Example Object. Present so a reference can resolve;
// the generator does not read its fields.
type Example struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Summary       string `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description   string `json:"description,omitempty" yaml:"description,omitempty"`
	Value         any    `json:"value,omitempty" yaml:"value,omitempty"`
	ExternalValue string `json:"externalValue,omitempty" yaml:"externalValue,omitempty"`
}

// Link is an OpenAPI Link Object. Present so a reference can resolve; the
// generator does not read its fields.
type Link struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	OperationRef string         `json:"operationRef,omitempty" yaml:"operationRef,omitempty"`
	OperationID  string         `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Description  string         `json:"description,omitempty" yaml:"description,omitempty"`
	Parameters   map[string]any `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Server       *Server        `json:"server,omitempty" yaml:"server,omitempty"`
	RequestBody  any            `json:"requestBody,omitempty" yaml:"requestBody,omitempty"`
}

// Examples are named example components.
type Examples map[string]*ExampleRef

// Links are named link components.
type Links map[string]*LinkRef
