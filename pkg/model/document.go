package model

import "slices"

// Document is the root of an OpenAPI 3 document. Kin-openapi calls this T.
type Document struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	OpenAPI      string               `json:"openapi" yaml:"openapi"`
	Components   *Components          `json:"components,omitempty" yaml:"components,omitempty"`
	Info         *Info                `json:"info" yaml:"info"`
	Paths        *Paths               `json:"paths" yaml:"paths"`
	Security     SecurityRequirements `json:"security,omitempty" yaml:"security,omitempty"`
	Servers      Servers              `json:"servers,omitempty" yaml:"servers,omitempty"`
	Tags         Tags                 `json:"tags,omitempty" yaml:"tags,omitempty"`
	ExternalDocs *ExternalDocs        `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
	Webhooks     map[string]*PathItem `json:"webhooks,omitempty" yaml:"webhooks,omitempty"` // OpenAPI >=3.1
}

// IsOpenAPI31OrLater reports whether the document is OpenAPI 3.1 or 3.2.
// Only the version strings kin-openapi recognizes count: a patch it does
// not list, such as "3.1.3", is not.
func (doc *Document) IsOpenAPI31OrLater() bool {
	return slices.Contains([]string{"3.1", "3.2"}, doc.OpenAPIMajorMinor())
}

// OpenAPIMajorMinor returns the "3.y" of an OpenAPI version string kin
// recognizes, or "" for a nil document and for any string it does not.
func (doc *Document) OpenAPIMajorMinor() string {
	if doc == nil {
		return ""
	}
	switch doc.OpenAPI {
	case "3", "3.0", "3.0.0", "3.0.1", "3.0.2", "3.0.3", "3.0.4":
		return "3.0"
	case "3.1", "3.1.0", "3.1.1", "3.1.2":
		return "3.1"
	case "3.2", "3.2.0", "3.2.1":
		return "3.2"
	default:
		return ""
	}
}

// Info is an OpenAPI Info Object. Contact, license and terms of service are
// omitted: the generator does not read them.
type Info struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Title       string `json:"title" yaml:"title"`
	Summary     string `json:"summary,omitempty" yaml:"summary,omitempty"` // OpenAPI >=3.1
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Version     string `json:"version" yaml:"version"`
}

// Components is an OpenAPI Components Object.
type Components struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Schemas         Schemas         `json:"schemas,omitempty" yaml:"schemas,omitempty"`
	Parameters      ParametersMap   `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Headers         Headers         `json:"headers,omitempty" yaml:"headers,omitempty"`
	RequestBodies   RequestBodies   `json:"requestBodies,omitempty" yaml:"requestBodies,omitempty"`
	Responses       ResponseBodies  `json:"responses,omitempty" yaml:"responses,omitempty"`
	SecuritySchemes SecuritySchemes `json:"securitySchemes,omitempty" yaml:"securitySchemes,omitempty"`
	Examples        Examples        `json:"examples,omitempty" yaml:"examples,omitempty"`
	Links           Links           `json:"links,omitempty" yaml:"links,omitempty"`
	Callbacks       Callbacks       `json:"callbacks,omitempty" yaml:"callbacks,omitempty"`
}

// SecurityRequirements is a list of alternative security requirements.
type SecurityRequirements []SecurityRequirement

// SecurityRequirement maps a security scheme name to the scopes it requires.
type SecurityRequirement map[string][]string

// SecuritySchemes are the components' named security schemes.
type SecuritySchemes map[string]*SecuritySchemeRef

// SecurityScheme is an OpenAPI Security Scheme Object. OAuth flows are
// omitted: the generator does not read them.
type SecurityScheme struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Type             string `json:"type,omitempty" yaml:"type,omitempty"`
	Description      string `json:"description,omitempty" yaml:"description,omitempty"`
	Name             string `json:"name,omitempty" yaml:"name,omitempty"`
	In               string `json:"in,omitempty" yaml:"in,omitempty"`
	Scheme           string `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	BearerFormat     string `json:"bearerFormat,omitempty" yaml:"bearerFormat,omitempty"`
	OpenIdConnectUrl string `json:"openIdConnectUrl,omitempty" yaml:"openIdConnectUrl,omitempty"`
}

// Servers is a list of servers.
type Servers []*Server

// Server is an OpenAPI Server Object.
type Server struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	URL         string          `json:"url" yaml:"url"`
	Description string          `json:"description,omitempty" yaml:"description,omitempty"`
	Variables   ServerVariables `json:"variables,omitempty" yaml:"variables,omitempty"`
}

// ServerVariables maps a variable name to its definition.
type ServerVariables map[string]*ServerVariable

// ServerVariable is an OpenAPI Server Variable Object.
type ServerVariable struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Enum        []string `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default     string   `json:"default,omitempty" yaml:"default,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
}

// Tags is a list of tags.
type Tags []*Tag

// Tag is an OpenAPI Tag Object.
type Tag struct {
	Extensions map[string]any `json:"-" yaml:"-"`
	Origin     *Origin        `json:"-" yaml:"-"`

	Name         string        `json:"name,omitempty" yaml:"name,omitempty"`
	Description  string        `json:"description,omitempty" yaml:"description,omitempty"`
	ExternalDocs *ExternalDocs `json:"externalDocs,omitempty" yaml:"externalDocs,omitempty"`
}
