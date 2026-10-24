package model

// Where a parameter lives.
const (
	ParameterInPath   = "path"
	ParameterInQuery  = "query"
	ParameterInHeader = "header"
	ParameterInCookie = "cookie"
)

// Parameters is an operation or path item's parameter list.
type Parameters []*ParameterRef

// ParametersMap is the components' named parameters.
type ParametersMap map[string]*ParameterRef

// Parameter is an OpenAPI Parameter Object.
type Parameter struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Name            string     `json:"name,omitempty" yaml:"name,omitempty"`
	In              string     `json:"in,omitempty" yaml:"in,omitempty"`
	Description     string     `json:"description,omitempty" yaml:"description,omitempty"`
	Style           string     `json:"style,omitempty" yaml:"style,omitempty"`
	Explode         *bool      `json:"explode,omitempty" yaml:"explode,omitempty"`
	AllowEmptyValue bool       `json:"allowEmptyValue,omitempty" yaml:"allowEmptyValue,omitempty"`
	AllowReserved   bool       `json:"allowReserved,omitempty" yaml:"allowReserved,omitempty"`
	Deprecated      bool       `json:"deprecated,omitempty" yaml:"deprecated,omitempty"`
	Required        bool       `json:"required,omitempty" yaml:"required,omitempty"`
	Schema          *SchemaRef `json:"schema,omitempty" yaml:"schema,omitempty"`
	Example         any        `json:"example,omitempty" yaml:"example,omitempty"`
	Examples        Examples   `json:"examples,omitempty" yaml:"examples,omitempty"`
	Content         Content    `json:"content,omitempty" yaml:"content,omitempty"`
}

// Header is an OpenAPI Header Object. It is a parameter without name or in.
type Header struct {
	Parameter
}

// Headers are named headers, on a response, encoding or in components.
type Headers map[string]*HeaderRef

// RequestBody is an OpenAPI Request Body Object.
type RequestBody struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Description string  `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool    `json:"required,omitempty" yaml:"required,omitempty"`
	Content     Content `json:"content" yaml:"content"`
}

// RequestBodies are the components' named request bodies.
type RequestBodies map[string]*RequestBodyRef

// Response is an OpenAPI Response Object.
type Response struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Description *string `json:"description,omitempty" yaml:"description,omitempty"`
	Headers     Headers `json:"headers,omitempty" yaml:"headers,omitempty"`
	Content     Content `json:"content,omitempty" yaml:"content,omitempty"`
	Links       Links   `json:"links,omitempty" yaml:"links,omitempty"`
}

// ResponseBodies are the components' named responses.
type ResponseBodies map[string]*ResponseRef

// Content maps a media type to its contents.
type Content map[string]*MediaType

// MediaType is an OpenAPI Media Type Object.
type MediaType struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Schema     *SchemaRef `json:"schema,omitempty" yaml:"schema,omitempty"`
	ItemSchema *SchemaRef `json:"itemSchema,omitempty" yaml:"itemSchema,omitempty"` // OpenAPI >=3.2
	Example    any        `json:"example,omitempty" yaml:"example,omitempty"`
	Examples   Examples   `json:"examples,omitempty" yaml:"examples,omitempty"`
	Encoding   Encodings  `json:"encoding,omitempty" yaml:"encoding,omitempty"`
}

// Encodings maps a property name to how that property is encoded.
type Encodings map[string]*Encoding

// Encoding is an OpenAPI Encoding Object.
type Encoding struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	ContentType   string  `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	Headers       Headers `json:"headers,omitempty" yaml:"headers,omitempty"`
	Style         string  `json:"style,omitempty" yaml:"style,omitempty"`
	Explode       *bool   `json:"explode,omitempty" yaml:"explode,omitempty"`
	AllowReserved bool    `json:"allowReserved,omitempty" yaml:"allowReserved,omitempty"`
}
