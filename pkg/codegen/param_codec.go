package codegen

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// This file generates the code that binds request parameters on the server,
// and styles them on the client, without the runtime library: scalars, arrays
// of scalars, and their styles. The generated code follows what the runtime
// library did for the same parameter, including which parse method it picks
// for each position, except where noted.

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
	// kindBinder is a type with Bind and UnmarshalText methods, formatted by
	// formatExpr: OpenAPIDate and OpenAPIDuration.
	kindBinder
	// kindUUID is uuid.UUID: UnmarshalText and String.
	kindUUID
	// kindUnion is a generated union of scalars, which has Bind,
	// UnmarshalText and MarshalText methods.
	kindUnion
	// kindText is a type the generator knows nothing about, from x-go-type
	// or a custom type mapping, which must implement encoding.TextMarshaler
	// and encoding.TextUnmarshaler.
	kindText
	// kindBytes is []byte, base64-encoded.
	kindBytes
	// kindAny is any: an untyped value, or an OpenAPI 3.1 multi-type union
	// of scalars, bound to the first of its types the value parses as.
	kindAny
	// kindFile is OpenAPIFile, holding the value's bytes.
	kindFile
	// kindJSON is a value that isn't a scalar, such as an object in an
	// array, carried as its JSON text.
	kindJSON
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
	"uuid.UUID":       {kindUUID, 0},
	"[]byte":          {kindBytes, 0},
	"json.RawMessage": {kindString, 0},
	"OpenAPIFile":     {kindFile, 0},
}

// namedType matches a type name, optionally qualified by a package.
var namedType = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)?$`)

// ScalarCodec binds and formats one scalar value of a parameter.
type ScalarCodec struct {
	goType string
	kind   scalarKind
	bits   int
	// date reports whether a kindBinder value is a date, formatted with
	// time.DateOnly rather than String.
	date bool
	// types are the members of a kindAny union; none for an untyped value.
	types []string
	// format is the OpenAPI format; a kindAny union's string member is
	// base64 with format byte.
	format string
}

// newScalarCodec returns the codec for a scalar value of the generated
// schema s, whose OpenAPI schema is spec, or nil when it isn't a scalar.
func newScalarCodec(s Schema, spec *openapi3.Schema) *ScalarCodec {
	goType := s.TypeDecl()
	if spec == nil {
		return nil
	}
	if spec.Extensions[extPropGoType] != nil {
		// x-go-type chose a type the generator knows nothing about.
		return &ScalarCodec{goType: goType, kind: kindText}
	}
	if len(spec.Properties) == 0 && !hasAdditionalProperties(spec) && len(spec.AllOf) == 0 && len(slices.Concat(spec.AnyOf, spec.OneOf)) > 0 {
		if len(unionTextKinds(slices.Concat(spec.AnyOf, spec.OneOf))) == 0 || !namedType.MatchString(goType) {
			return nil
		}
		return &ScalarCodec{goType: goType, kind: kindUnion}
	}
	types := schemaPrimaryType(spec.Type).Slice()
	if goType == "any" || goType == "interface{}" {
		if slices.Contains(types, "array") || slices.Contains(types, "object") {
			return nil
		}
		return &ScalarCodec{goType: goType, kind: kindAny, types: schemaUnionTypes(spec.Type), format: spec.Format}
	}
	if len(types) != 1 {
		return nil
	}
	var expected string
	switch types[0] {
	case "string":
		// contentMediaType and contentEncoding select a binary format.
		if spec.Format == "" && (spec.ContentMediaType != "" || spec.ContentEncoding != "") {
			return nil
		}
		expected = globalState.typeMapping.String.Resolve(spec.Format).Type
	case "integer":
		expected = globalState.typeMapping.Integer.Resolve(spec.Format).Type
	case "number":
		expected = globalState.typeMapping.Number.Resolve(spec.Format).Type
	case "boolean":
		expected = globalState.typeMapping.Boolean.Resolve(spec.Format).Type
	default:
		return nil
	}
	sk, ok := scalarKinds[expected]
	if !ok {
		// A custom type mapping chose a type the generator knows nothing
		// about.
		return &ScalarCodec{goType: goType, kind: kindText}
	}
	if sk.kind == kindBytes && goType != expected {
		return nil
	}
	// A named type over the expected one, such as an enum or an alias,
	// converts to and from it; one over a type with methods keeps them as
	// an alias does.
	return &ScalarCodec{goType: goType, kind: sk.kind, bits: sk.bits, date: expected == "OpenAPIDate", format: spec.Format}
}

// hasAdditionalProperties reports whether the schema allows properties it
// doesn't declare.
func hasAdditionalProperties(spec *openapi3.Schema) bool {
	return spec.AdditionalProperties.Schema != nil || (spec.AdditionalProperties.Has != nil && *spec.AdditionalProperties.Has)
}

// structLike reports whether the runtime library treated the type as a
// struct rather than a primitive when binding a query value: it read the
// first of several exploded values, and split an unexploded one like an
// object, rather than rejecting several values or reading one whole.
func (c *ScalarCodec) structLike() bool {
	return c.kind == kindTime || c.kind == kindBinder || c.kind == kindUnion
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
// asText selects how a type with an UnmarshalText method is read: with it, as
// the whole value of a path, header or cookie parameter is, or as an object,
// as a query value or an array element is, which uses Bind if there is one
// and reads time.Time as RFC 3339 or a date.
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
		return fmt.Sprintf("if %[1]s != \"\" {\nif v_, err_ := time.Parse(time.RFC3339Nano, %[1]s); err_ == nil {\n%[2]s = %[3]s\n} else if v_, err_ := time.Parse(time.DateOnly, %[1]s); err_ == nil {\n%[2]s = %[3]s\n} else {\nerr = err_\n}\n}",
			src, dst, c.convert("v_", "time.Time"))
	case kindBinder, kindUnion:
		if asText {
			return fmt.Sprintf("err = %s.UnmarshalText([]byte(%s))", dst, src)
		}
		return fmt.Sprintf("err = %s.Bind(%s)", dst, src)
	case kindBytes:
		return bindBase64(src, dst)
	case kindFile:
		return fmt.Sprintf("%s.InitFromBytes([]byte(%s), \"\")", dst, src)
	case kindJSON:
		return fmt.Sprintf("err = json.Unmarshal([]byte(%s), &%s)", src, dst)
	case kindAny:
		return c.bindAny(src, dst)
	default: // kindUUID, kindText
		return fmt.Sprintf("err = %s.UnmarshalText([]byte(%s))", dst, src)
	}
}

// bindBase64 returns statements that decode src, in standard or URL-safe
// base64, padded or not, into dst, and set err when it can't.
func bindBase64(src, dst string) string {
	return fmt.Sprintf(`{
enc_ := base64.RawStdEncoding
switch {
case strings.ContainsRune(%[1]s, '=') && strings.ContainsAny(%[1]s, "-_"):
enc_ = base64.URLEncoding
case strings.ContainsRune(%[1]s, '='):
enc_ = base64.StdEncoding
case strings.ContainsAny(%[1]s, "-_"):
enc_ = base64.RawURLEncoding
}
%[2]s, err = enc_.DecodeString(%[1]s)
}`, src, dst)
}

// bindAny returns statements that bind src into dst, of type any, as the
// first of the union's types it parses as, trying the most restrictive
// first: boolean, integer, number, then string. Numbers follow the JSON
// number grammar, so "007" and "+1" are strings. An untyped value is a
// string.
func (c *ScalarCodec) bindAny(src, dst string) string {
	if len(c.types) == 0 {
		return fmt.Sprintf("%s = %s", dst, src)
	}
	has := func(t string) bool { return slices.Contains(c.types, t) }
	var b strings.Builder
	b.WriteString("{\n")
	if has("integer") || has("number") {
		fmt.Fprintf(&b, "number_ := %[1]s != \"\" && (%[1]s[0] == '-' || (%[1]s[0] >= '0' && %[1]s[0] <= '9')) && strings.TrimSpace(%[1]s) == %[1]s && json.Valid([]byte(%[1]s))\n", src)
	}
	b.WriteString("switch {\n")
	if has("boolean") {
		fmt.Fprintf(&b, "case %[1]s == \"true\" || %[1]s == \"false\":\n%[2]s = %[1]s == \"true\"\n", src, dst)
	}
	if has("integer") {
		fmt.Fprintf(&b, "case number_ && !strings.ContainsAny(%[1]s, \".eE\") && func() bool { _, err_ := strconv.ParseInt(%[1]s, 10, 64); return err_ == nil }():\n%[2]s, _ = strconv.ParseInt(%[1]s, 10, 64)\n", src, dst)
	}
	if has("number") {
		fmt.Fprintf(&b, "case number_ && func() bool { _, err_ := strconv.ParseFloat(%[1]s, 64); return err_ == nil }():\n%[2]s, _ = strconv.ParseFloat(%[1]s, 64)\n", src, dst)
	}
	if has("string") {
		b.WriteString("default:\n")
		if c.format == "byte" {
			fmt.Fprintf(&b, "var bytes_ []byte\n%s\n%s = bytes_\n", bindBase64(src, "bytes_"), dst)
		} else {
			fmt.Fprintf(&b, "%s = %s\n", dst, src)
		}
	} else {
		var members []string
		for _, t := range c.types {
			if t != "null" {
				members = append(members, t)
			}
		}
		fmt.Fprintf(&b, "default:\nerr = fmt.Errorf(\"value '%%s' does not match any member of type union %v\", %s)\n", members, src)
	}
	b.WriteString("}\n}")
	return b.String()
}

// Format returns statements that set out to v, an expression of the codec's
// type, as text escaped by escape (a function name, or "" for none), and set
// err when it can't be formatted.
func (c *ScalarCodec) Format(v, out, escape string) string {
	esc := func(s string) string {
		if escape == "" {
			return s
		}
		return escape + "(" + s + ")"
	}
	if strings.HasPrefix(v, "*") {
		v = "(" + v + ")"
	}
	switch c.kind {
	case kindUnion, kindText:
		return fmt.Sprintf("if b_, err_ := %s.MarshalText(); err_ != nil {\nerr = err_\n} else {\n%s = %s\n}", v, out, esc("string(b_)"))
	case kindJSON:
		return fmt.Sprintf("if b_, err_ := json.Marshal(%s); err_ != nil {\nerr = err_\n} else {\n%s = %s\n}", v, out, esc("string(b_)"))
	case kindFile:
		return fmt.Sprintf("if b_, err_ := %s.Bytes(); err_ != nil {\nerr = err_\n} else {\n%s = %s\n}", v, out, esc("string(b_)"))
	case kindAny:
		return fmt.Sprintf(`switch v_ := %[1]s.(type) {
case string:
%[2]s = %[3]s
case []byte:
%[2]s = %[4]s
case bool:
%[2]s = %[5]s
case float32:
%[2]s = %[6]s
case float64:
%[2]s = %[7]s
default:
%[2]s = %[8]s
}`, v, out, esc("v_"), esc("base64.StdEncoding.EncodeToString(v_)"), esc("strconv.FormatBool(v_)"),
			esc("strconv.FormatFloat(float64(v_), 'f', -1, 32)"), esc("strconv.FormatFloat(v_, 'f', -1, 64)"), esc("fmt.Sprint(v_)"))
	}
	return fmt.Sprintf("%s = %s", out, esc(c.formatExpr(v)))
}

// formatExpr returns an expression formatting v, of a kind whose formatting
// can't fail.
func (c *ScalarCodec) formatExpr(v string) string {
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
		if c.date {
			return v + ".Format(time.DateOnly)"
		}
		return v + ".String()"
	case kindBytes:
		return "base64.StdEncoding.EncodeToString(" + v + ")"
	default: // kindUUID
		return v + ".String()"
	}
}

// ParamCodec binds and styles a parameter's value: a scalar, or an array of
// scalars, in the parameter's location, style and explode.
type ParamCodec struct {
	// Scalar is the codec of the value, or of an array's elements.
	Scalar *ScalarCodec
	// IsArray reports whether the value is an array of Scalar.
	IsArray bool
	// Object is the codec of an object value, in place of Scalar.
	Object  *ObjectCodec
	deep    *deepCodec
	goType  string
	name    string
	in      string
	style   string
	explode bool
}

// newParamCodec returns the codec for a value of the generated schema s,
// whose OpenAPI schema is spec, or nil when the generated code can't bind it
// without the runtime library.
func newParamCodec(s Schema, ref *openapi3.SchemaRef) *ParamCodec {
	if ref == nil || ref.Value == nil {
		return nil
	}
	spec := ref.Value
	if types := schemaPrimaryType(spec.Type).Slice(); len(types) == 1 && types[0] == "array" && spec.Extensions[extPropGoType] == nil {
		if spec.Items == nil || spec.Items.Value == nil || s.ArrayType == nil {
			return nil
		}
		elem := newScalarCodec(*s.ArrayType, spec.Items.Value)
		if elem == nil {
			// Neither the runtime library nor the OpenAPI styles say how to
			// lay out an array of objects or arrays; each element is its
			// JSON text.
			elem = &ScalarCodec{goType: s.ArrayType.TypeDecl(), kind: kindJSON}
		}
		return &ParamCodec{Scalar: elem, IsArray: true, goType: s.TypeDecl()}
	}
	if scalar := newScalarCodec(s, spec); scalar != nil {
		return &ParamCodec{Scalar: scalar, goType: s.TypeDecl()}
	}
	if object := newObjectCodec(s, ref); object != nil {
		return &ParamCodec{Object: object, goType: s.TypeDecl()}
	}
	return nil
}

// escape returns the function that escapes the parameter's names and values
// for its location, or "" for none.
func (c *ParamCodec) escape() string {
	switch c.in {
	case "path":
		return "url.PathEscape"
	case "query":
		return "url.QueryEscape"
	default:
		return ""
	}
}

// escapedName returns the parameter's name escaped for its location.
func (c *ParamCodec) escapedName() string {
	switch c.in {
	case "path":
		return url.PathEscape(c.name)
	case "query":
		return url.QueryEscape(c.name)
	default:
		return c.name
	}
}

// split returns statements that split src, a styled path, header or cookie
// value, into parts_, and set err when it isn't in the style.
func (c *ParamCodec) split(src string) string {
	errorf := func(format string) string {
		return fmt.Sprintf("err = errors.New(%s)", StringToGoString(fmt.Sprintf(format, c.name)))
	}
	switch {
	case c.style == "label" && c.explode:
		return fmt.Sprintf("parts_ := strings.Split(%s, \".\")\nif parts_[0] != \"\" {\n%s\n}\nparts_ = parts_[1:]", src,
			errorf("invalid format for label parameter '%s', should start with '.'"))
	case c.style == "label":
		return fmt.Sprintf("var parts_ []string\nif !strings.HasPrefix(%[1]s, \".\") {\n%[2]s\n} else {\nparts_ = strings.Split(%[1]s[1:], \",\")\n}", src,
			errorf("invalid format for label parameter '%s', should start with '.'"))
	case c.style == "matrix" && c.explode && c.Object != nil:
		return fmt.Sprintf("parts_ := strings.Split(%s, \";\")\nif parts_[0] != \"\" {\n%s\n}\nparts_ = parts_[1:]", src,
			errorf("invalid format for matrix parameter '%s', should start with ';'"))
	case c.style == "matrix" && c.explode:
		return fmt.Sprintf("parts_ := strings.Split(%s, \";\")\nif parts_[0] != \"\" {\n%s\n}\nparts_ = parts_[1:]\nfor i_ := range parts_ {\nparts_[i_] = strings.TrimPrefix(parts_[i_], %s)\n}", src,
			errorf("invalid format for matrix parameter '%s', should start with ';'"), StringToGoString(c.name+"="))
	case c.style == "matrix":
		prefix := ";" + c.name + "="
		return fmt.Sprintf("var parts_ []string\nif !strings.HasPrefix(%[1]s, %[2]s) {\n%[3]s\n} else {\nparts_ = strings.Split(strings.TrimPrefix(%[1]s, %[2]s), \",\")\n}", src,
			StringToGoString(prefix), errorf("expected parameter '%s' to start with "+strings.ReplaceAll(prefix, "%", "%%")))
	default: // simple
		return fmt.Sprintf("parts_ := strings.Split(%s, \",\")", src)
	}
}

// BindStyled returns statements that bind src, the value of a path, header or
// cookie parameter, into dst, and set err when it can't: a scalar is the
// value without the style's prefix, an array its parts.
//
// The runtime library read a value with an UnmarshalText method whole, prefix
// and all, so a `label` or `matrix` date, time, UUID or union never bound;
// the generated code strips the prefix first.
func (c *ParamCodec) BindStyled(src, dst string) string {
	if c.Object != nil {
		return fmt.Sprintf("{\n%s\nif err == nil {\n%s\nif err == nil {\n%s\n}\n}\n}", c.split(src), pairs(c.name, c.explode), c.Object.bindPairs(dst))
	}
	if !c.IsArray && c.style == "simple" && c.Scalar.kind != kindBytes {
		return c.Scalar.Bind(src, dst, true)
	}
	if !c.IsArray {
		// A scalar is the one part of the value.
		bind := c.Scalar.Bind("parts_[0]", dst, c.Scalar.kind != kindBytes)
		if c.style == "simple" { // kindBytes, which can't contain a comma
			bind = c.Scalar.Bind(src, dst, false)
			return fmt.Sprintf("if strings.Contains(%s, \",\") {\nerr = errors.New(%s)\n} else {\n%s\n}",
				src, StringToGoString(fmt.Sprintf("expected single base64 value for byte slice parameter '%s'", c.name)), bind)
		}
		return fmt.Sprintf("{\n%s\nif err == nil {\nif len(parts_) != 1 {\nerr = errors.New(%s)\n} else {\n%s\n}\n}\n}",
			c.split(src), StringToGoString(fmt.Sprintf("parameter '%s': expected single value", c.name)), bind)
	}
	return fmt.Sprintf("{\n%s\nif err == nil {\n%s\n}\n}", c.split(src), c.BindList("parts_", dst))
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

// BindQuery returns statements that bind values, the values of the query
// parameter, of which there is at least one, into dst, and set err when they
// can't be.
func (c *ParamCodec) BindQuery(values, dst string) string {
	errorf := func(format string) string {
		return fmt.Sprintf("err = errors.New(%s)", StringToGoString(fmt.Sprintf(format, c.name)))
	}
	if c.Object != nil {
		sep := map[string]string{"spaceDelimited": " ", "pipeDelimited": "|"}[c.style]
		if sep == "" {
			sep = ","
		}
		return fmt.Sprintf("if len(%[1]s) != 1 {\n%[2]s\n} else {\nparts_ := strings.Split(%[1]s[0], %[3]q)\n%[4]s\nif err == nil {\n%[5]s\n}\n}", values,
			errorf("parameter '%s' is not exploded, but is specified multiple times"), sep, pairs(c.name, false), c.Object.bindPairs(dst))
	}
	if c.explode {
		switch {
		case c.IsArray:
			return c.BindList(values, dst)
		case c.Scalar.structLike():
			return c.Scalar.Bind(values+"[0]", dst, false)
		default:
			return fmt.Sprintf("if len(%s) != 1 {\n%s\n} else {\n%s\n}", values,
				errorf("multiple values for single value parameter '%s'"), c.Scalar.Bind(values+"[0]", dst, false))
		}
	}
	var bind string
	switch {
	case c.IsArray:
		sep := map[string]string{"spaceDelimited": " ", "pipeDelimited": "|"}[c.style]
		if sep == "" {
			sep = ","
		}
		bind = c.BindList(fmt.Sprintf("strings.Split(%s[0], %q)", values, sep), dst)
	case c.Scalar.structLike():
		// The runtime library split a struct like an object and required
		// one part, then read it with Bind, or for time.Time, which has no
		// Bind, UnmarshalText.
		bind = fmt.Sprintf("if strings.Contains(%s[0], \",\") {\n%s\n} else {\n%s\n}", values,
			errorf("multiple values for single value parameter '%s'"), c.Scalar.Bind(values+"[0]", dst, c.Scalar.kind == kindTime))
	default:
		bind = c.Scalar.Bind(values+"[0]", dst, false)
	}
	return fmt.Sprintf("if len(%s) != 1 {\n%s\n} else {\n%s\n}", values,
		errorf("parameter '%s' is not exploded, but is specified multiple times"), bind)
}

// Style returns statements that set out to the styled value v of a path,
// header or cookie parameter, escaped for its location, and set err when it
// can't be formatted.
func (c *ParamCodec) Style(v, out string) string {
	if c.Object != nil {
		var prefix, sep string
		switch c.style {
		case "label":
			prefix, sep = ".", ","
			if c.explode {
				sep = "."
			}
		case "matrix":
			prefix, sep = ";"+c.escapedName()+"=", ","
			if c.explode {
				prefix, sep = ";", ";"
			}
		default:
			sep = ","
		}
		return fmt.Sprintf("{\n%s\nif err == nil {\n%s\n%s = %s + strings.Join(parts_, %s)\n}\n}",
			c.Object.collect(v), c.objectParts(), out, StringToGoString(prefix), StringToGoString(sep))
	}
	esc := c.escape()
	var prefix, sep string
	switch c.style {
	case "label":
		prefix, sep = ".", ","
		if c.explode {
			sep = "."
		}
	case "matrix":
		prefix, sep = ";"+c.escapedName()+"=", ","
		if c.explode {
			sep = prefix
		}
	default:
		sep = ","
	}
	if !c.IsArray {
		if prefix == "" {
			return c.Scalar.Format(v, out, esc)
		}
		return fmt.Sprintf("%s\n%s = %s + %s", c.Scalar.Format(v, out, esc), out, StringToGoString(prefix), out)
	}
	if strings.HasPrefix(v, "*") {
		v = "(" + v + ")"
	}
	return fmt.Sprintf("{\nparts_ := make([]string, len(%[1]s))\nfor i_, e_ := range %[1]s {\n%[2]s\n}\n%[3]s = %[4]s + strings.Join(parts_, %[5]s)\n}",
		v, c.Scalar.Format("e_", "parts_[i_]", esc), out, StringToGoString(prefix), StringToGoString(sep))
}

// StyleQuery returns statements that append the styled value v of the query
// parameter to fragments, a []string of `key=value` query fragments, and set
// err when it can't be formatted.
//
// The runtime library couldn't style a scalar in `spaceDelimited` or
// `pipeDelimited` style; the generated code styles it as `form`.
func (c *ParamCodec) StyleQuery(v, fragments string) string {
	if c.deep != nil {
		return c.deep.style(c.name, v, fragments)
	}
	prefix := StringToGoString(c.escapedName() + "=")
	if c.Object != nil {
		if c.explode {
			return fmt.Sprintf("{\n%s\nif err == nil {\n%s\n%s = append(%s, parts_...)\n}\n}", c.Object.collect(v), c.objectParts(), fragments, fragments)
		}
		sep := map[string]string{"spaceDelimited": " ", "pipeDelimited": "|"}[c.style]
		if sep == "" {
			sep = ","
		}
		return fmt.Sprintf("{\n%s\nif err == nil {\n%s\n%s = append(%s, %s+strings.Join(parts_, %q))\n}\n}", c.Object.collect(v), c.objectParts(), fragments, fragments, prefix, sep)
	}
	if !c.IsArray {
		return fmt.Sprintf("{\nvar s_ string\n%s\n%s = append(%s, %s+s_)\n}", c.Scalar.Format(v, "s_", "url.QueryEscape"), fragments, fragments, prefix)
	}
	if strings.HasPrefix(v, "*") {
		v = "(" + v + ")"
	}
	elem := c.Scalar.Format("e_", "s_", "url.QueryEscape")
	if c.explode {
		// An empty array still sends the bare key, as the runtime library
		// did.
		return fmt.Sprintf("if len(%[1]s) == 0 {\n%[2]s = append(%[2]s, %[3]s)\n}\nfor _, e_ := range %[1]s {\nvar s_ string\n%[4]s\n%[2]s = append(%[2]s, %[3]s+s_)\n}",
			v, fragments, prefix, elem)
	}
	sep := map[string]string{"spaceDelimited": " ", "pipeDelimited": "|"}[c.style]
	if sep == "" {
		sep = ","
	}
	return fmt.Sprintf("{\nparts_ := make([]string, len(%[1]s))\nfor i_, e_ := range %[1]s {\nvar s_ string\n%[4]s\nparts_[i_] = s_\n}\n%[2]s = append(%[2]s, %[3]s+strings.Join(parts_, %[5]q))\n}",
		v, fragments, prefix, elem, sep)
}

// objectParts returns statements that lay out kv_, an object's formatted
// properties, as parts_: `key=value` when exploded, keys and values in turn
// otherwise, in key order, with values escaped for the location.
func (c *ParamCodec) objectParts() string {
	value := "kv_[k_]"
	if esc := c.escape(); esc != "" {
		value = esc + "(" + value + ")"
	}
	if c.explode {
		return fmt.Sprintf("var parts_ []string\nfor _, k_ := range slices.Sorted(maps.Keys(kv_)) {\nparts_ = append(parts_, k_+\"=\"+%s)\n}", value)
	}
	return fmt.Sprintf("var parts_ []string\nfor _, k_ := range slices.Sorted(maps.Keys(kv_)) {\nparts_ = append(parts_, k_, %s)\n}", value)
}

// ExplodedQueryObject reports whether the parameter is an exploded object in
// a query, whose properties are query parameters of their own: see
// BindQueryObject.
func (c *ParamCodec) ExplodedQueryObject() bool {
	return c.deep != nil || (c.Object != nil && c.in == "query" && c.explode)
}

// BindQueryObject returns statements that bind the properties of an
// exploded query object, each a query parameter of its own in query_, into
// dst, set found_ when one is present, and set err when one can't be bound.
// An object whose properties aren't known, or that only has additional
// properties, can't be told apart from the other query parameters, so is
// never found, as with the runtime library.
func (c *ParamCodec) BindQueryObject(query, dst string) string {
	if c.deep != nil {
		return c.deep.bind(c.name, query, dst)
	}
	o := c.Object
	if o.opaque || o.isMap {
		return ""
	}
	var b strings.Builder
	for _, f := range o.fields {
		fmt.Fprintf(&b, "if vs_, ok_ := %s[%s]; ok_ {\nfound_ = true\nif len(vs_) != 1 {\nerr = errors.New(%s)\n} else {\n%s\n}\n}\n",
			query, StringToGoString(f.json), StringToGoString(fmt.Sprintf("field '%s' specified multiple times for param '%s'", f.json, c.name)),
			o.bindField(f, "vs_[0]", dst))
	}
	return b.String()
}

// Codec returns the codec that binds and styles the parameter without the
// runtime library, or nil when the runtime library does that.
func (pd ParameterDefinition) Codec() *ParamCodec {
	if !pd.IsStyled() {
		return nil
	}
	style := pd.Style()
	switch pd.In {
	case "path":
		if style != "simple" && style != "label" && style != "matrix" {
			return nil
		}
	case "header":
		if style != "simple" {
			return nil
		}
	case "query":
		if style == "deepObject" {
			c := &ParamCodec{deep: newDeepCodec(pd.Schema, pd.Spec.Schema), goType: pd.Schema.TypeDecl()}
			c.name, c.in, c.style, c.explode = pd.ParamName, pd.In, style, true
			return c
		}
		if style != "form" && style != "spaceDelimited" && style != "pipeDelimited" {
			return nil
		}
	case "cookie":
		// Cookies are always `simple` style.
		style = "simple"
	default:
		return nil
	}
	c := newParamCodec(pd.Schema, pd.Spec.Schema)
	if c == nil {
		return nil
	}
	c.name, c.in, c.style, c.explode = pd.ParamName, pd.In, style, pd.Explode()
	return c
}

// Codec returns the codec that binds and styles the response header without
// the runtime library, or nil when the runtime library does that.
func (h ResponseHeaderDefinition) Codec() *ParamCodec {
	if h.Style != "simple" {
		return nil
	}
	c := newParamCodec(h.Schema, &openapi3.SchemaRef{Value: h.Schema.OAPISchema})
	if c == nil {
		return nil
	}
	c.name, c.in, c.style, c.explode = h.Name, "header", "simple", h.Explode
	return c
}

// ObjectCodec binds and styles an object parameter: an object's properties
// are key-value pairs, which the style lays out in the value.
type ObjectCodec struct {
	// fields are the object's properties, by JSON name.
	fields []objectField
	// additional is the codec of additional properties' values, kept in the
	// AdditionalProperties field, or in the map itself when isMap; nil when
	// there are none, or they are JSON text (additionalJSON).
	additional     *ScalarCodec
	additionalType string
	additionalJSON bool
	hasAdditional  bool
	isMap          bool
	// opaque reports whether the type's fields aren't known, as for a union
	// of objects or a type from another package: the value goes through its
	// JSON encoding.
	opaque bool
}

// objectField is one property of an object parameter.
type objectField struct {
	json, goName, goType string
	// codec binds and formats the value, or is nil for a value that isn't a
	// scalar, carried as JSON text.
	codec   *ScalarCodec
	pointer bool
}

// newObjectCodec returns the codec for an object parameter of the generated
// schema s, from the OpenAPI schema ref.
func newObjectCodec(s Schema, ref *openapi3.SchemaRef) *ObjectCodec {
	if ref == nil || ref.Value == nil {
		return nil
	}
	gs := s
	if len(gs.Properties) == 0 && !gs.HasAdditionalProperties && len(gs.UnionElements) == 0 {
		// A $ref is generated as the referenced type's name; generate the
		// component the way its declaration is, to learn its fields.
		gs = Schema{}
		for r := ref; r != nil; {
			name, ok := strings.CutPrefix(r.Ref, "#/components/schemas/")
			if !ok || globalState.spec == nil || globalState.spec.Components == nil {
				break
			}
			next := globalState.spec.Components.Schemas[name]
			if next == nil {
				break
			}
			if next.Ref == "" {
				generated, err := generateGoSchema(newRootGenContext([]string{name}), next, []string{name})
				if err == nil {
					gs = generated
				}
				break
			}
			r = next
		}
	}
	types := schemaPrimaryType(ref.Value.Type).Slice()
	isObject := len(ref.Value.Properties) > 0 || hasAdditionalProperties(ref.Value) || len(ref.Value.AllOf) > 0 ||
		len(ref.Value.AnyOf) > 0 || len(ref.Value.OneOf) > 0 || (len(types) == 1 && types[0] == "object")
	if !isObject {
		return nil
	}
	// An object with only additionalProperties is generated as a map,
	// without HasAdditionalProperties, which marks a struct that holds them.
	hasAdditional := gs.HasAdditionalProperties || (len(gs.Properties) == 0 && gs.AdditionalPropertiesType != nil)
	if len(gs.UnionElements) > 0 || (len(gs.Properties) == 0 && !hasAdditional) {
		return &ObjectCodec{opaque: true}
	}
	c := &ObjectCodec{hasAdditional: hasAdditional, isMap: len(gs.Properties) == 0}
	if hasAdditional {
		if at := gs.AdditionalPropertiesType; at != nil {
			c.additionalType = additionalPropertiesType(gs)
			if !strings.HasPrefix(c.additionalType, "*") {
				c.additional = newScalarCodec(*at, at.OAPISchema)
			}
			if c.additional != nil && c.additional.kind == kindText {
				c.additional = nil
			}
		}
		if c.additionalType == "" {
			c.additionalType = "any"
		}
		c.additionalJSON = c.additional == nil
	}
	for _, p := range gs.Properties {
		f := objectField{json: p.JsonFieldName, goName: p.GoFieldName(), goType: p.Schema.TypeDecl()}
		switch p.GoTypeDef() {
		case f.goType:
		case "*" + f.goType:
			f.pointer = true
		default:
			c.fields = append(c.fields, f)
			continue
		}
		f.codec = newScalarCodec(p.Schema, p.Schema.OAPISchema)
		if f.codec != nil && f.codec.kind == kindText {
			// A type the generator knows nothing about is carried as
			// JSON, which every type has, rather than text, which a type
			// in an object needn't have.
			f.codec = nil
		}
		c.fields = append(c.fields, f)
	}
	slices.SortFunc(c.fields, func(a, b objectField) int { return strings.Compare(a.json, b.json) })
	return c
}

// bindPairs returns statements that bind pairs_, a [][2]string of keys and
// values, into the object dst, and set err when one can't be bound.
func (o *ObjectCodec) bindPairs(dst string) string {
	if o.opaque {
		return fmt.Sprintf(`{
m_ := make(map[string]any, len(pairs_))
for _, kv_ := range pairs_ {
switch {
case kv_[1] == "true" || kv_[1] == "false":
m_[kv_[0]] = kv_[1] == "true"
case kv_[1] != "" && (kv_[1][0] == '-' || (kv_[1][0] >= '0' && kv_[1][0] <= '9')) && strings.TrimSpace(kv_[1]) == kv_[1] && json.Valid([]byte(kv_[1])):
m_[kv_[0]] = json.Number(kv_[1])
default:
m_[kv_[0]] = kv_[1]
}
}
if b_, err_ := json.Marshal(m_); err_ != nil {
err = err_
} else {
err = json.Unmarshal(b_, &%s)
}
}`, dst)
	}
	var b strings.Builder
	if o.isMap {
		fmt.Fprintf(&b, "%s = make(map[string]%s, len(pairs_))\n", dst, o.additionalType)
	}
	b.WriteString("for _, kv_ := range pairs_ {\nswitch kv_[0] {\n")
	for _, f := range o.fields {
		fmt.Fprintf(&b, "case %s:\n%s\n", StringToGoString(f.json), o.bindField(f, "kv_[1]", dst))
	}
	if o.hasAdditional {
		b.WriteString("default:\n")
		m := dst + ".AdditionalProperties"
		if o.isMap {
			m = dst
		} else {
			fmt.Fprintf(&b, "if %[1]s == nil {\n%[1]s = make(map[string]%[2]s)\n}\n", m, o.additionalType)
		}
		if o.additional == nil {
			fmt.Fprintf(&b, "var a_ %s\nerr = json.Unmarshal([]byte(kv_[1]), &a_)\n%s[kv_[0]] = a_\n", o.additionalType, m)
		} else {
			fmt.Fprintf(&b, "var a_ %s\n%s\n%s[kv_[0]] = a_\n", o.additionalType, o.additional.Bind("kv_[1]", "a_", false), m)
		}
	}
	b.WriteString("}\nif err != nil {\nbreak\n}\n}")
	return b.String()
}

// bindField returns statements that bind src, a string, into the field f of
// the object dst, and set err when it can't be bound.
func (o *ObjectCodec) bindField(f objectField, src, dst string) string {
	target := dst + "." + f.goName
	switch {
	case f.codec == nil:
		return fmt.Sprintf("err = json.Unmarshal([]byte(%s), &%s)", src, target)
	case f.pointer:
		return fmt.Sprintf("{\nvar f_ %s\n%s\n%s = &f_\n}", f.goType, f.codec.Bind(src, "f_", false), target)
	default:
		return f.codec.Bind(src, target, false)
	}
}

// pairs returns statements that read the key-value pairs of an object from
// parts_, the parts of its styled value, into pairs_, and set err when they
// aren't pairs: `key=value` parts when exploded, alternating keys and values
// otherwise.
func pairs(name string, explode bool) string {
	if explode {
		return fmt.Sprintf("pairs_ := make([][2]string, 0, len(parts_))\nfor _, part_ := range parts_ {\nkv_ := strings.Split(part_, \"=\")\nif len(kv_) != 2 {\nerr = errors.New(%s)\nbreak\n}\npairs_ = append(pairs_, [2]string{kv_[0], kv_[1]})\n}",
			StringToGoString(fmt.Sprintf("parameter '%s' has invalid exploded format", name)))
	}
	return fmt.Sprintf("pairs_ := make([][2]string, 0, len(parts_)/2)\nif len(parts_)%%2 != 0 {\nerr = errors.New(%s)\n} else {\nfor i_ := 0; i_ < len(parts_); i_ += 2 {\npairs_ = append(pairs_, [2]string{parts_[i_], parts_[i_+1]})\n}\n}",
		StringToGoString(fmt.Sprintf("parameter '%s' has invalid format, property/values need to be pairs", name)))
}

// collect returns statements that set kv_, a map[string]string, to the
// object v's properties formatted as text, and set err when one can't be.
// Unset optional properties are left out.
func (o *ObjectCodec) collect(v string) string {
	if strings.HasPrefix(v, "*") {
		v = "(" + v + ")"
	}
	var b strings.Builder
	b.WriteString("kv_ := map[string]string{}\n")
	if o.opaque {
		fmt.Fprintf(&b, `if b_, err_ := json.Marshal(%s); err_ != nil {
err = err_
} else {
var m_ map[string]any
dec_ := json.NewDecoder(bytes.NewReader(b_))
dec_.UseNumber()
if err = dec_.Decode(&m_); err == nil {
for k_, e_ := range m_ {
switch e_ := e_.(type) {
case string:
kv_[k_] = e_
case json.Number:
kv_[k_] = e_.String()
case bool:
kv_[k_] = strconv.FormatBool(e_)
default:
b_, _ := json.Marshal(e_)
kv_[k_] = string(b_)
}
}
}
}`, v)
		return b.String()
	}
	if o.isMap {
		fmt.Fprintf(&b, "for k_, e_ := range %s {\nvar s_ string\n%s\nkv_[k_] = s_\n}", v, o.formatAdditional("e_"))
		return b.String()
	}
	for _, f := range o.fields {
		field := v + "." + f.goName
		key := "kv_[" + StringToGoString(f.json) + "]"
		switch {
		case f.codec == nil && (f.pointer || strings.HasPrefix(f.goType, "[]") || strings.HasPrefix(f.goType, "map[")):
			fmt.Fprintf(&b, "if %[1]s != nil {\nif b_, err_ := json.Marshal(%[1]s); err_ != nil {\nerr = err_\n} else {\n%[2]s = string(b_)\n}\n}\n", field, key)
		case f.codec == nil:
			fmt.Fprintf(&b, "if b_, err_ := json.Marshal(%s); err_ != nil {\nerr = err_\n} else {\n%s = string(b_)\n}\n", field, key)
		case f.pointer:
			fmt.Fprintf(&b, "if %s != nil {\n%s\n}\n", field, f.codec.Format("*"+field, key, ""))
		default:
			fmt.Fprintf(&b, "%s\n", f.codec.Format(field, key, ""))
		}
	}
	if o.hasAdditional {
		fmt.Fprintf(&b, "for k_, e_ := range %s.AdditionalProperties {\nvar s_ string\n%s\nkv_[k_] = s_\n}", v, o.formatAdditional("e_"))
	}
	return b.String()
}

// formatAdditional returns statements that set s_ to the additional
// property value e, and set err when it can't be formatted.
func (o *ObjectCodec) formatAdditional(e string) string {
	if o.additional == nil {
		return fmt.Sprintf("if b_, err_ := json.Marshal(%s); err_ != nil {\nerr = err_\n} else {\ns_ = string(b_)\n}", e)
	}
	return o.additional.Format(e, "s_", "")
}

// deepCodec binds and styles a `deepObject` query parameter: an object whose
// properties, and theirs in turn, are query parameters named by their path,
// as in `p[a][b]=v`.
//
// The server parses the query parameters into a tree, which it converts to
// the parameter's JSON, by the kinds of value the schema gives each property,
// and decodes. kinds names the objects the parameter reaches, so recursive
// types need no recursive code.
type deepCodec struct {
	root  string
	kinds map[string]map[string]string
}

// newDeepCodec returns the codec for a deepObject parameter of the
// generated schema s, from the OpenAPI schema ref.
func newDeepCodec(s Schema, ref *openapi3.SchemaRef) *deepCodec {
	d := &deepCodec{kinds: map[string]map[string]string{}}
	d.root = d.kind(s, ref.Value, "$")
	return d
}

// componentNamed returns the name of the component schema spec is, or "".
func componentNamed(spec *openapi3.Schema) string {
	if spec == nil || globalState.spec == nil || globalState.spec.Components == nil {
		return ""
	}
	for _, name := range SortedSchemaKeys(globalState.spec.Components.Schemas) {
		if ref := globalState.spec.Components.Schemas[name]; ref != nil && ref.Value == spec {
			return name
		}
	}
	return ""
}

// kind returns the kind of a value of the generated schema s, from the
// OpenAPI schema spec, at the position name, adding the objects it reaches to
// d.kinds: "string", "int", "float", "bool", "any" (JSON scalars inferred
// from the text), "object:<name>", "array:<kind>" or "map:<kind>".
func (d *deepCodec) kind(s Schema, spec *openapi3.Schema, name string) string {
	if spec == nil {
		return "any"
	}
	if spec.Extensions[extPropGoType] == nil {
		if c := newScalarCodec(s, spec); c != nil {
			switch c.kind {
			case kindInt, kindUint:
				return "int"
			case kindFloat:
				return "float"
			case kindBool:
				return "bool"
			case kindAny, kindUnion:
				return "any"
			case kindFile:
				return "skip"
			default:
				return "string"
			}
		}
	}
	types := schemaPrimaryType(spec.Type).Slice()
	if len(types) == 1 && types[0] == "array" {
		if spec.Items == nil || s.ArrayType == nil {
			return "array:any"
		}
		return "array:" + d.kind(*s.ArrayType, spec.Items.Value, name+"[]")
	}
	if spec.Extensions[extPropGoType] != nil || len(spec.AnyOf) > 0 || len(spec.OneOf) > 0 {
		return "any"
	}
	if component := componentNamed(spec); component != "" {
		name = component
		if _, done := d.kinds[name]; done {
			return "object:" + name
		}
		if generated, err := generateGoSchema(newRootGenContext([]string{component}), globalState.spec.Components.Schemas[component], []string{component}); err == nil {
			s = generated
		}
	}
	if len(s.Properties) == 0 && (s.HasAdditionalProperties || s.AdditionalPropertiesType != nil) {
		if s.AdditionalPropertiesType == nil {
			return "map:any"
		}
		return "map:" + d.kind(*s.AdditionalPropertiesType, s.AdditionalPropertiesType.OAPISchema, name+"{}")
	}
	fields := map[string]string{}
	d.kinds[name] = fields
	for _, p := range s.Properties {
		fields[p.JsonFieldName] = d.kind(p.Schema, p.Schema.OAPISchema, name+"."+p.JsonFieldName)
	}
	if s.HasAdditionalProperties {
		if s.AdditionalPropertiesType == nil {
			fields["*"] = "any"
		} else {
			fields["*"] = d.kind(*s.AdditionalPropertiesType, s.AdditionalPropertiesType.OAPISchema, name+"{}")
		}
	}
	return "object:" + name
}

// kindsLiteral returns d.kinds as a Go map literal.
func (d *deepCodec) kindsLiteral() string {
	var b strings.Builder
	b.WriteString("map[string]map[string]string{\n")
	for _, name := range SortedMapKeys(d.kinds) {
		fmt.Fprintf(&b, "%s: {", StringToGoString(name))
		fields := d.kinds[name]
		for i, field := range SortedMapKeys(fields) {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s: %s", StringToGoString(field), StringToGoString(fields[field]))
		}
		b.WriteString("},\n")
	}
	b.WriteString("}")
	return b.String()
}

// bind returns statements that bind the deepObject parameter name from
// query, a url.Values, into dst, set found_ when any of its query parameters
// is present, and set err when they can't be bound.
func (d *deepCodec) bind(name, query, dst string) string {
	collect := fmt.Sprintf(`for key_, values_ := range %[1]s {
rest_, ok_ := strings.CutPrefix(key_, %[2]s)
if !ok_ {
continue
}
found_ = true
path_ := strings.Split(strings.TrimRight(rest_, "]"), "][")
if len(values_) == 1 {
insert_(path_, values_[0])
} else {
for i_, value_ := range values_ {
insert_(append(path_[:len(path_):len(path_)], strconv.Itoa(i_)), value_)
}
}
}`, query, StringToGoString(name+"["))
	return d.bindTree(collect, dst, false)
}

// bindForm returns statements that bind form, a url.Values of an
// application/x-www-form-urlencoded body, into dst, and set err when it can't
// be bound. Keys name a property by its path, as in `a[b][0]`; an array's
// values may also repeat its key, with or without a trailing `[]`. Unknown
// properties are ignored.
func (d *deepCodec) bindForm(form, dst string) string {
	collect := fmt.Sprintf(`for key_, values_ := range %s {
key_ = strings.TrimSuffix(key_, "[]")
name_, rest_, _ := strings.Cut(key_, "[")
path_ := []string{name_}
if rest_ != "" {
path_ = append(path_, strings.Split(strings.TrimRight(rest_, "]"), "][")...)
}
found_ = true
if len(values_) == 1 {
insert_(path_, values_[0])
} else {
for i_, value_ := range values_ {
insert_(append(path_[:len(path_):len(path_)], strconv.Itoa(i_)), value_)
}
}
}`, form)
	return fmt.Sprintf("{\nfound_ := false\n%s\n}", d.bindTree(collect, dst, true))
}

// bindTree returns statements that run collect, which fills tree_, by
// insert_, with the values of the parameter or form by path, and sets found_
// when there are any, then bind tree_ into dst, and set err when it can't be
// bound. In a form, unknown properties are ignored, and a single value of an
// array is an array of one.
func (d *deepCodec) bindTree(collect, dst string, form bool) string {
	return fmt.Sprintf(`{
form_ := %[6]v
_ = form_
tree_ := map[string]any{}
insert_ := func(path []string, value string) {
node := tree_
for _, key := range path[:len(path)-1] {
next, ok := node[key].(map[string]any)
if !ok {
next = map[string]any{}
node[key] = next
}
node = next
}
node[path[len(path)-1]] = value
}
%[2]s
if found_ {
kinds_ := %[4]s
// In a form, a value of the wrong shape is skipped, as the runtime
// library skipped it.
skip_ := &struct{}{}
var convert_ func(node any, kind string) (any, error)
convert_ = func(node any, kind string) (any, error) {
fields, isMap := node.(map[string]any)
if form_ {
compound := strings.HasPrefix(kind, "object:") || strings.HasPrefix(kind, "map:") || strings.HasPrefix(kind, "array:")
switch {
case isMap && !compound && kind != "any":
// A repeated value: the first one counts.
first, ok := fields["0"]
if !ok {
return skip_, nil
}
return convert_(first, kind)
case !isMap && compound && !strings.HasPrefix(kind, "array:"):
return skip_, nil
}
}
text, _ := node.(string)
switch {
case strings.HasPrefix(kind, "object:"):
if !isMap {
return nil, fmt.Errorf("expected an object, got %%q", text)
}
object := kinds_[strings.TrimPrefix(kind, "object:")]
out := make(map[string]any, len(fields))
for key, value := range fields {
fieldKind, ok := object[key]
if !ok {
if fieldKind, ok = object["*"]; !ok {
if form_ {
continue
}
return nil, fmt.Errorf("field [%%s] is not present in destination object", key)
}
}
if fieldKind == "skip" {
continue
}
v, err := convert_(value, fieldKind)
if err != nil {
return nil, fmt.Errorf("error assigning field [%%s]: %%w", key, err)
}
if v == skip_ {
continue
}
out[key] = v
}
return out, nil
case strings.HasPrefix(kind, "map:"):
if !isMap {
return nil, fmt.Errorf("expected an object, got %%q", text)
}
out := make(map[string]any, len(fields))
for key, value := range fields {
v, err := convert_(value, strings.TrimPrefix(kind, "map:"))
if err != nil {
return nil, err
}
if v == skip_ {
continue
}
out[key] = v
}
return out, nil
case strings.HasPrefix(kind, "array:"):
if !isMap && form_ {
v, err := convert_(node, strings.TrimPrefix(kind, "array:"))
if err != nil {
return nil, fmt.Errorf("error binding array element 0: %%w", err)
}
return []any{v}, nil
}
if !isMap {
return nil, fmt.Errorf("expected an array, got %%q", text)
}
size := len(fields)
if form_ {
// Missing elements of a form's array are zero values.
size = 0
for key := range fields {
if i, err := strconv.Atoi(key); err == nil && i >= size {
size = i + 1
}
}
}
out := make([]any, size)
for i := range out {
value, ok := fields[strconv.Itoa(i)]
if !ok && form_ {
continue
}
if !ok {
return nil, errors.New("array deepObjects must have consecutive indices")
}
v, err := convert_(value, strings.TrimPrefix(kind, "array:"))
if err != nil {
return nil, fmt.Errorf("error binding array element %%d: %%w", i, err)
}
if v == skip_ {
v = nil
}
out[i] = v
}
return out, nil
case isMap:
if kind != "any" {
return nil, fmt.Errorf("expected a value, got an object")
}
keys := make([]int, 0, len(fields))
for key := range fields {
if i, err := strconv.Atoi(key); err == nil && i >= 0 && i < len(fields) {
keys = append(keys, i)
}
}
out := make(map[string]any, len(fields))
for key, value := range fields {
v, err := convert_(value, "any")
if err != nil {
return nil, err
}
out[key] = v
}
if len(keys) != len(fields) {
return out, nil
}
array := make([]any, len(fields))
for key, value := range out {
i, _ := strconv.Atoi(key)
array[i] = value
}
return array, nil
case kind == "int":
i, err := strconv.ParseInt(text, 10, 64)
if err != nil {
return nil, fmt.Errorf("expected a valid int, got %%s", text)
}
return i, nil
case kind == "float":
f, err := strconv.ParseFloat(text, 64)
if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
return nil, fmt.Errorf("expected a valid float, got %%s", text)
}
return f, nil
case kind == "bool":
b, err := strconv.ParseBool(text)
if err != nil {
return nil, fmt.Errorf("expected a valid bool, got %%s", text)
}
return b, nil
case kind == "any":
switch text {
case "true":
return true, nil
case "false":
return false, nil
case "null":
return nil, nil
}
var f float64
if err := json.Unmarshal([]byte(text), &f); err == nil {
return f, nil
}
return text, nil
default:
return text, nil
}
}
if v_, err_ := convert_(tree_, %[5]s); err_ != nil {
err = err_
} else if b_, err_ := json.Marshal(v_); err_ != nil {
err = err_
} else {
err = json.Unmarshal(b_, &%[1]s)
}
}
}`, dst, collect, "", d.kindsLiteral(), StringToGoString(d.root), form)
}

// style returns statements that append the query fragments of the
// deepObject parameter name, with value v, to fragments, and set err when v
// can't be encoded: `name[key]...=value` for each value v's JSON holds, by
// path, with keys in order.
func (d *deepCodec) style(name, v, fragments string) string {
	if strings.HasPrefix(v, "*") {
		v = "(" + v + ")"
	}
	return fmt.Sprintf(`if b_, err_ := json.Marshal(%[1]s); err_ != nil {
err = err_
} else {
var tree_ any
dec_ := json.NewDecoder(bytes.NewReader(b_))
dec_.UseNumber()
if err = dec_.Decode(&tree_); err == nil {
var walk_ func(prefix string, node any)
walk_ = func(prefix string, node any) {
switch node := node.(type) {
case []any:
for i, e := range node {
walk_(prefix+"["+strconv.Itoa(i)+"]", e)
}
case map[string]any:
for _, k := range slices.Sorted(maps.Keys(node)) {
walk_(prefix+"["+url.QueryEscape(k)+"]", node[k])
}
case nil:
%[2]s = append(%[2]s, prefix+"=null")
default:
%[2]s = append(%[2]s, prefix+"="+url.QueryEscape(fmt.Sprint(node)))
}
}
walk_(%[3]s, tree_)
}
}`, v, fragments, StringToGoString(url.QueryEscape(name)))
}

// formBind returns statements that bind form, the url.Values of an
// application/x-www-form-urlencoded body, into dst, a value of the generated
// schema s, and set err when it can't be bound.
func formBind(s Schema, form, dst string) string {
	return newDeepCodec(s, &openapi3.SchemaRef{Value: s.OAPISchema}).bindForm(form, dst)
}

// formEncode returns statements that add the properties of v to out, a
// url.Values, as an application/x-www-form-urlencoded body, and set err when
// v can't be encoded: `a`, `a[b]` and `a[0]` keys for each value v's JSON
// holds, by path. Unset values are left out. A url.Values, such as an
// untyped body may hold, is the form itself.
func formEncode(v, out string) string {
	return fmt.Sprintf(`if values_, ok_ := any(%[1]s).(url.Values); ok_ {
for k_, vs_ := range values_ {
%[2]s[k_] = append(%[2]s[k_], vs_...)
}
} else if b_, err_ := json.Marshal(%[1]s); err_ != nil {
err = err_
} else {
var tree_ any
dec_ := json.NewDecoder(bytes.NewReader(b_))
dec_.UseNumber()
if err = dec_.Decode(&tree_); err == nil {
var walk_ func(key string, node any)
walk_ = func(key string, node any) {
child := func(name string) string {
if key == "" {
return name
}
return key + "[" + name + "]"
}
switch node := node.(type) {
case map[string]any:
for k, e := range node {
walk_(child(k), e)
}
case []any:
for i, e := range node {
walk_(child(strconv.Itoa(i)), e)
}
case nil:
case string:
%[2]s.Add(key, node)
default:
%[2]s.Add(key, fmt.Sprint(node))
}
}
walk_("", tree_)
}
}`, v, out)
}
