package codegen

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// This file generates the code that binds request parameters on the server,
// and styles them on the client, for the parameters whose Go type is known
// well enough to do that without the runtime library's reflection: builtin
// scalars and named types over them (such as enums), the time, date, duration
// and UUID types, and arrays of those. Other parameters keep calling the
// runtime library.
//
// The generated code mirrors what the runtime does for the same parameter,
// including which parse method it picks for each location, so that moving a
// parameter from one to the other doesn't change what it accepts or sends.

// scalarKind is how a scalar parameter's value is parsed and formatted.
type scalarKind int

const (
	kindString scalarKind = iota
	kindInt
	kindUint
	kindFloat
	kindBool
	// kindTime is time.Time: RFC 3339, or a date when bound as an object.
	kindTime
	// kindBinder is a type with Bind, UnmarshalText and String methods, such
	// as OpenAPIDate and OpenAPIDuration.
	kindBinder
	// kindText is a type with UnmarshalText and String methods but no Bind,
	// such as uuid.UUID.
	kindText
)

// scalarKinds maps the Go types the type mapping can produce to their kind
// and, for numbers, their bit size.
var scalarKinds = map[string]struct {
	kind scalarKind
	bits int
}{
	"string":          {kindString, 0},
	"int":             {kindInt, 0},
	"int8":            {kindInt, 8},
	"int16":           {kindInt, 16},
	"int32":           {kindInt, 32},
	"int64":           {kindInt, 64},
	"uint":            {kindUint, 0},
	"uint8":           {kindUint, 8},
	"uint16":          {kindUint, 16},
	"uint32":          {kindUint, 32},
	"uint64":          {kindUint, 64},
	"float32":         {kindFloat, 32},
	"float64":         {kindFloat, 64},
	"bool":            {kindBool, 0},
	"OpenAPIEmail":    {kindString, 0},
	"time.Time":       {kindTime, 0},
	"OpenAPIDate":     {kindBinder, 0},
	"OpenAPIDuration": {kindBinder, 0},
	"uuid.UUID":       {kindText, 0},
}

// namedType matches a type name, optionally qualified by a package.
var namedType = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

// ScalarCodec binds and formats one scalar value of a parameter.
type ScalarCodec struct {
	goType string
	kind   scalarKind
	bits   int
	// timeLike reports whether the runtime treats the type as a struct
	// rather than a primitive when binding query parameters.
	timeLike bool
}

// newScalarCodec returns the codec for a scalar of Go type goType generated
// from schema, or nil when the type isn't one the generated code can bind
// itself: a type x-go-type or a custom type mapping chose, or a type that
// isn't scalar.
func newScalarCodec(goType string, schema *openapi3.Schema) *ScalarCodec {
	if schema == nil || schema.Extensions[extPropGoType] != nil {
		return nil
	}
	types := schemaPrimaryType(schema.Type).Slice()
	if len(types) != 1 {
		return nil
	}
	var expected string
	switch types[0] {
	case "string":
		// contentMediaType and contentEncoding select a binary format.
		if schema.Format == "" && (schema.ContentMediaType != "" || schema.ContentEncoding != "") {
			return nil
		}
		expected = globalState.typeMapping.String.Resolve(schema.Format).Type
	case "integer":
		expected = globalState.typeMapping.Integer.Resolve(schema.Format).Type
	case "number":
		expected = globalState.typeMapping.Number.Resolve(schema.Format).Type
	case "boolean":
		expected = globalState.typeMapping.Boolean.Resolve(schema.Format).Type
	default:
		return nil
	}
	sk, ok := scalarKinds[expected]
	if !ok {
		return nil
	}
	c := &ScalarCodec{goType: goType, kind: sk.kind, bits: sk.bits}
	c.timeLike = expected == "time.Time" || expected == "OpenAPIDate" || expected == "OpenAPIDuration"
	_, goTypeIsKnown := scalarKinds[goType]
	switch {
	case goType == expected:
	case sk.kind <= kindBool && namedType.MatchString(goType) && !goTypeIsKnown && goType != "any":
		// A named type over a builtin, such as an enum, converts to and
		// from it. Other kinds need the type's own methods, which a
		// defined type over them doesn't have.
	default:
		return nil
	}
	return c
}

// convert returns expr, of the builtin type the parse produced, converted to
// the codec's type when that differs.
func (c *ScalarCodec) convert(expr, builtin string) string {
	if c.goType == builtin {
		return expr
	}
	return c.goType + "(" + expr + ")"
}

// Bind returns statements that parse the string src into dst, an
// addressable variable of the codec's type, and set err when it can't.
// asText selects how the runtime reads the time, date, duration and UUID
// types in this position: with UnmarshalText, as it does a path, header or
// cookie value, or as an object, as it does a query value or an array
// element, which uses Bind and reads time.Time as RFC 3339 or a date.
func (c *ScalarCodec) Bind(src, dst string, asText bool) string {
	switch c.kind {
	case kindString:
		return fmt.Sprintf("%s = %s", dst, c.convert(src, "string"))
	case kindInt:
		return fmt.Sprintf("if v_, err_ := strconv.ParseInt(%s, 10, %d); err_ != nil {\nerr = err_\n} else {\n%s = %s\n}", src, c.bits, dst, c.convert("v_", "int64"))
	case kindUint:
		return fmt.Sprintf("if v_, err_ := strconv.ParseUint(%s, 10, %d); err_ != nil {\nerr = err_\n} else {\n%s = %s\n}", src, c.bits, dst, c.convert("v_", "uint64"))
	case kindFloat:
		return fmt.Sprintf("if v_, err_ := strconv.ParseFloat(%s, %d); err_ != nil {\nerr = err_\n} else {\n%s = %s\n}", src, c.bits, dst, c.convert("v_", "float64"))
	case kindBool:
		return fmt.Sprintf("if v_, err_ := strconv.ParseBool(%s); err_ != nil {\nerr = err_\n} else {\n%s = %s\n}", src, dst, c.convert("v_", "bool"))
	case kindTime:
		if asText {
			return fmt.Sprintf("err = %s.UnmarshalText([]byte(%s))", dst, src)
		}
		return fmt.Sprintf("if %[1]s != \"\" {\nif v_, err_ := time.Parse(time.RFC3339Nano, %[1]s); err_ == nil {\n%[2]s = v_\n} else if v_, err_ := time.Parse(time.DateOnly, %[1]s); err_ == nil {\n%[2]s = v_\n} else {\nerr = err_\n}\n}", src, dst)
	case kindBinder:
		if asText {
			return fmt.Sprintf("err = %s.UnmarshalText([]byte(%s))", dst, src)
		}
		return fmt.Sprintf("err = %s.Bind(%s)", dst, src)
	default: // kindText
		return fmt.Sprintf("err = %s.UnmarshalText([]byte(%s))", dst, src)
	}
}

// Format returns an expression that formats v, an expression of the codec's
// type, as the string the runtime would send for it.
func (c *ScalarCodec) Format(v string) string {
	if strings.HasPrefix(v, "*") {
		v = "(" + v + ")"
	}
	switch c.kind {
	case kindString:
		if c.goType == "string" {
			return v
		}
		return "string(" + v + ")"
	case kindInt:
		return "strconv.FormatInt(int64(" + v + "), 10)"
	case kindUint:
		return "strconv.FormatUint(uint64(" + v + "), 10)"
	case kindFloat:
		return fmt.Sprintf("strconv.FormatFloat(float64(%s), 'f', -1, %d)", v, c.bits)
	case kindBool:
		return "strconv.FormatBool(bool(" + v + "))"
	case kindTime:
		return v + ".Format(time.RFC3339Nano)"
	case kindBinder:
		if c.goType == "OpenAPIDate" {
			return v + ".Format(time.DateOnly)"
		}
		return v + ".String()"
	default: // kindText
		return v + ".String()"
	}
}

// ParamCodec binds and formats a parameter's value: a scalar, or an array of
// scalars.
type ParamCodec struct {
	// Scalar is the codec of the value, or of an array's elements.
	Scalar *ScalarCodec
	// IsArray reports whether the value is an array of Scalar.
	IsArray bool
	goType  string
}

// newParamCodec returns the codec for a value of the generated schema s,
// whose OpenAPI schema is spec, or nil when the runtime binds it instead.
func newParamCodec(s Schema, spec *openapi3.Schema) *ParamCodec {
	if spec == nil || spec.Extensions[extPropGoType] != nil {
		return nil
	}
	types := schemaPrimaryType(spec.Type).Slice()
	if len(types) == 1 && types[0] == "array" {
		if spec.Items == nil || spec.Items.Value == nil || s.ArrayType == nil {
			return nil
		}
		elem := newScalarCodec(s.ArrayType.TypeDecl(), spec.Items.Value)
		if elem == nil {
			return nil
		}
		return &ParamCodec{Scalar: elem, IsArray: true, goType: s.TypeDecl()}
	}
	scalar := newScalarCodec(s.TypeDecl(), spec)
	if scalar == nil {
		return nil
	}
	return &ParamCodec{Scalar: scalar, goType: s.TypeDecl()}
}

// TimeLike reports whether the runtime binds a query value of this scalar as
// a struct: it takes the first of several values rather than rejecting them.
func (c *ParamCodec) TimeLike() bool {
	return !c.IsArray && c.Scalar.timeLike
}

// BindStyled returns statements that bind src, a `simple`-style value of a
// path, header or cookie parameter, into dst, and set err when it can't: a
// scalar is the whole value, an array its comma-separated parts.
func (c *ParamCodec) BindStyled(src, dst string) string {
	if !c.IsArray {
		return c.Scalar.Bind(src, dst, true)
	}
	return c.BindList(fmt.Sprintf("strings.Split(%s, \",\")", src), dst)
}

// BindList returns statements that bind each string of list, an expression
// of type []string, into the elements of dst, an array, and set err when one
// can't be.
func (c *ParamCodec) BindList(list, dst string) string {
	if c.Scalar.kind == kindString {
		// Strings can't fail to bind.
		if c.Scalar.goType == "string" {
			if c.goType == "[]string" {
				return fmt.Sprintf("%s = %s", dst, list)
			}
			return fmt.Sprintf("%s = %s(%s)", dst, c.goType, list)
		}
		return fmt.Sprintf("{\nparts_ := %s\n%s = make(%s, len(parts_))\nfor i_, part_ := range parts_ {\n%s\n}\n}",
			list, dst, c.goType, c.Scalar.Bind("part_", dst+"[i_]", false))
	}
	return fmt.Sprintf("{\nparts_ := %s\n%s = make(%s, len(parts_))\nfor i_, part_ := range parts_ {\n%s\nif err != nil {\nbreak\n}\n}\n}",
		list, dst, c.goType, c.Scalar.Bind("part_", dst+"[i_]", false))
}

// BindQueryValue returns statements that bind value, the one value of a
// non-exploded `form` query parameter, into dst, and set err when it can't.
func (c *ParamCodec) BindQueryValue(value, dst string) string {
	if c.IsArray {
		return c.BindList(fmt.Sprintf("strings.Split(%s, \",\")", value), dst)
	}
	// The runtime reads a struct-like value by splitting it like an object
	// and requiring one part, then with Bind, or for time.Time, which has
	// no Bind, UnmarshalText.
	if c.Scalar.timeLike {
		return fmt.Sprintf("if strings.Contains(%s, \",\") {\nerr = errors.New(\"multiple values for single value parameter\")\n} else {\n%s\n}",
			value, c.Scalar.Bind(value, dst, c.Scalar.kind == kindTime))
	}
	return c.Scalar.Bind(value, dst, false)
}

// Style returns statements that set out to the `simple`-style serialization
// of v, the value of a path, header or cookie parameter, with each value
// escaped by escape, a function name or "" for none.
func (c *ParamCodec) Style(v, out, escape string) string {
	esc := func(s string) string {
		if escape == "" {
			return s
		}
		return escape + "(" + s + ")"
	}
	if !c.IsArray {
		return fmt.Sprintf("%s = %s", out, esc(c.Scalar.Format(v)))
	}
	if strings.HasPrefix(v, "*") {
		v = "(" + v + ")"
	}
	return fmt.Sprintf("{\nparts_ := make([]string, len(%[1]s))\nfor i_, e_ := range %[1]s {\nparts_[i_] = %[2]s\n}\n%[3]s = strings.Join(parts_, \",\")\n}",
		v, esc(c.Scalar.Format("e_")), out)
}

// StyleQuery returns statements that append the `form`-style serialization
// of v, the value of query parameter name, to fragments, a []string of
// `key=value` query fragments.
func (c *ParamCodec) StyleQuery(name string, explode bool, v, fragments string) string {
	prefix := StringToGoString(url.QueryEscape(name) + "=")
	if !c.IsArray {
		return fmt.Sprintf("%[1]s = append(%[1]s, %[2]s+url.QueryEscape(%[3]s))", fragments, prefix, c.Scalar.Format(v))
	}
	if strings.HasPrefix(v, "*") {
		v = "(" + v + ")"
	}
	elem := "url.QueryEscape(" + c.Scalar.Format("e_") + ")"
	if explode {
		// An empty array still sends the bare key, as the runtime does.
		return fmt.Sprintf("if len(%[1]s) == 0 {\n%[2]s = append(%[2]s, %[3]s)\n}\nfor _, e_ := range %[1]s {\n%[2]s = append(%[2]s, %[3]s+%[4]s)\n}",
			v, fragments, prefix, elem)
	}
	return fmt.Sprintf("{\nparts_ := make([]string, len(%[1]s))\nfor i_, e_ := range %[1]s {\nparts_[i_] = %[4]s\n}\n%[2]s = append(%[2]s, %[3]s+strings.Join(parts_, \",\"))\n}",
		v, fragments, prefix, elem)
}

// Codec returns the codec that binds and styles the parameter without the
// runtime library, or nil when the runtime does that: the parameter isn't
// `simple` style in a path or header, `form` style in a query, or in a
// cookie, which is always `simple`, or its type isn't one the codec knows.
func (pd ParameterDefinition) Codec() *ParamCodec {
	if !pd.IsStyled() || len(pd.SchemaTypes()) > 0 {
		return nil
	}
	switch pd.In {
	case "path", "header":
		if pd.Style() != "simple" {
			return nil
		}
	case "query":
		if pd.Style() != "form" {
			return nil
		}
	case "cookie":
	default:
		return nil
	}
	return newParamCodec(pd.Schema, pd.Spec.Schema.Value)
}

// Codec returns the codec that binds and styles the response header without
// the runtime library, or nil when the runtime does that.
func (h ResponseHeaderDefinition) Codec() *ParamCodec {
	if h.Style != "simple" || len(h.SchemaTypes()) > 0 {
		return nil
	}
	return newParamCodec(h.Schema, h.Schema.OAPISchema)
}
