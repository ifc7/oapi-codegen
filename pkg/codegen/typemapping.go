package codegen

import (
	"maps"
	"regexp"
	"slices"
)

// SimpleTypeSpec defines the Go type for an OpenAPI type/format combination,
// along with any import required to use it.
type SimpleTypeSpec struct {
	Type   string `yaml:"type" json:"type"`
	Import string `yaml:"import,omitempty" json:"import,omitempty"`
}

// FormatMapping defines the default Go type and format-specific overrides
// for an OpenAPI type.
type FormatMapping struct {
	Default SimpleTypeSpec            `yaml:"default" json:"default"`
	Formats map[string]SimpleTypeSpec `yaml:"formats,omitempty" json:"formats,omitempty"`
}

// TypeMapping defines the mapping from OpenAPI types to Go types.
type TypeMapping struct {
	Integer FormatMapping `yaml:"integer,omitempty" json:"integer"`
	Number  FormatMapping `yaml:"number,omitempty" json:"number"`
	Boolean FormatMapping `yaml:"boolean,omitempty" json:"boolean"`
	String  FormatMapping `yaml:"string,omitempty" json:"string"`
}

// Merge returns a new TypeMapping with user overrides applied on top of base.
func (base TypeMapping) Merge(user TypeMapping) TypeMapping {
	return TypeMapping{
		Integer: base.Integer.merge(user.Integer),
		Number:  base.Number.merge(user.Number),
		Boolean: base.Boolean.merge(user.Boolean),
		String:  base.String.merge(user.String),
	}
}

func (base FormatMapping) merge(user FormatMapping) FormatMapping {
	result := FormatMapping{
		Default: base.Default,
		Formats: make(map[string]SimpleTypeSpec),
	}

	// Copy base formats
	maps.Copy(result.Formats, base.Formats)

	// Override with user default if specified
	if user.Default.Type != "" {
		result.Default = user.Default
	}

	// Override/add user formats
	maps.Copy(result.Formats, user.Formats)

	return result
}

// Resolve returns the SimpleTypeSpec for a given format string.
// If the format has a specific mapping, that is returned; otherwise the default is used.
func (fm FormatMapping) Resolve(format string) SimpleTypeSpec {
	if format != "" {
		if spec, ok := fm.Formats[format]; ok {
			return spec
		}
	}
	return fm.Default
}

// DefaultTypeMapping provides the default OpenAPI type/format to Go type mappings.
var DefaultTypeMapping = TypeMapping{
	Integer: FormatMapping{
		Default: SimpleTypeSpec{Type: "int"},
		Formats: map[string]SimpleTypeSpec{
			"int":    {Type: "int"},
			"int8":   {Type: "int8"},
			"int16":  {Type: "int16"},
			"int32":  {Type: "int32"},
			"int64":  {Type: "int64"},
			"uint":   {Type: "uint"},
			"uint8":  {Type: "uint8"},
			"uint16": {Type: "uint16"},
			"uint32": {Type: "uint32"},
			"uint64": {Type: "uint64"},
		},
	},
	Number: FormatMapping{
		Default: SimpleTypeSpec{Type: "float32"},
		Formats: map[string]SimpleTypeSpec{
			"float":  {Type: "float32"},
			"double": {Type: "float64"},
		},
	},
	Boolean: FormatMapping{
		Default: SimpleTypeSpec{Type: "bool"},
	},
	String: FormatMapping{
		Default: SimpleTypeSpec{Type: "string"},
		Formats: map[string]SimpleTypeSpec{
			"byte":      {Type: "[]byte"},
			"email":     {Type: "OpenAPIEmail"},
			"date":      {Type: "OpenAPIDate"},
			"date-time": {Type: "time.Time", Import: "time"},
			"duration":  {Type: "OpenAPIDuration"},
			"json":      {Type: "json.RawMessage", Import: "encoding/json"},
			"uuid":      {Type: "uuid.UUID", Import: "github.com/google/uuid"},
			"binary":    {Type: "OpenAPIFile"},
		},
	},
}

// packageQualifier matches the package name qualifying a type in a type
// expression, such as "uuid" in "[]uuid.UUID".
var packageQualifier = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\.`)

// usesPackage reports whether a type the mapping maps to is qualified by the
// package name name.
func (tm TypeMapping) usesPackage(name string) bool {
	for _, fm := range []FormatMapping{tm.Integer, tm.Number, tm.Boolean, tm.String} {
		for _, spec := range append([]SimpleTypeSpec{fm.Default}, slices.Collect(maps.Values(fm.Formats))...) {
			for _, m := range packageQualifier.FindAllStringSubmatch(spec.Type, -1) {
				if m[1] == name {
					return true
				}
			}
		}
	}
	return false
}

// formatTypes lists, in the order they're emitted, the types that
// format-types.tmpl declares in the generated package for the string formats
// that have no suitable standard library or third-party type.
var formatTypes = []string{"OpenAPIDate", "OpenAPIDuration", "OpenAPIEmail", "OpenAPIFile"}

// useTypeSpec returns spec's Go type, recording what the generated code needs
// in order to use it: the package to import, or the format type to declare.
func useTypeSpec(spec SimpleTypeSpec) string {
	if spec.Import != "" {
		if globalState.typeMappingImports == nil {
			globalState.typeMappingImports = map[string]goImport{}
		}
		globalState.typeMappingImports[spec.Import] = goImport{Path: spec.Import}
	} else if slices.Contains(formatTypes, spec.Type) {
		if globalState.usedFormatTypes == nil {
			globalState.usedFormatTypes = map[string]bool{}
		}
		globalState.usedFormatTypes[spec.Type] = true
	}
	return spec.Type
}
