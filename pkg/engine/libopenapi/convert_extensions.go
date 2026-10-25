package libopenapi

import (
	"context"
	"reflect"
	"strings"

	"github.com/pb33f/go-yaml"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	lowbase "github.com/pb33f/libopenapi/datamodel/low/base"
	"github.com/pb33f/libopenapi/index"
	"github.com/pb33f/libopenapi/orderedmap"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

// Keys kin-openapi deletes before keeping the rest of an object as extensions.
// A field kin does not know, such as components.pathItems or a media type's
// type, therefore survives as an extension.
var (
	documentExtensionKeys = []string{
		"openapi", "components", "info", "paths", "security", "servers",
		"tags", "externalDocs", "webhooks", "jsonSchemaDialect",
	}
	componentExtensionKeys = []string{
		"schemas", "parameters", "headers", "requestBodies", "responses",
		"securitySchemes", "examples", "links", "callbacks",
	}
	operationExtensionKeys = []string{
		"tags", "summary", "description", "operationId", "parameters",
		"requestBody", "responses", "callbacks", "deprecated", "security",
		"servers", "externalDocs",
	}
	responseExtensionKeys  = []string{"description", "headers", "content", "links", "$ref"}
	mediaTypeExtensionKeys = []string{"schema", "itemSchema", "example", "examples", "encoding"}
)

// refSiblingAnnotations are the keywords kin copies onto a 3.1 $ref's schema.
// Every other sibling keyword becomes another allOf entry.
var refSiblingAnnotations = map[string]struct{}{
	"$comment": {}, "allowEmptyValue": {}, "default": {}, "deprecated": {},
	"description": {}, "discriminator": {}, "example": {}, "examples": {},
	"externalDocs": {}, "nullable": {}, "readOnly": {}, "title": {},
	"writeOnly": {}, "xml": {},
}

func documentExtensions(in *v3.Document) map[string]any {
	var node *yaml.Node
	if low := in.GoLow(); low != nil && low.Index != nil {
		node = low.Index.GetRootNode()
	}
	return nodeExtensions(node, documentExtensionKeys, in.Extensions)
}

func nodeExtensions(node *yaml.Node, known []string, fallback *orderedmap.Map[string, *yaml.Node]) map[string]any {
	if mappingNode(node) == nil {
		return decodeExtensions(fallback)
	}
	return objectExtensions(node, known)
}

func rootNode(low interface{ GetRootNode() *yaml.Node }) *yaml.Node {
	if low == nil {
		return nil
	}
	value := reflect.ValueOf(low)
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return nil
	}
	return low.GetRootNode()
}

func mappingNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return mappingNode(node.Content[0])
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}
	return node
}

func objectExtensions(node *yaml.Node, known []string) map[string]any {
	node = mappingNode(node)
	if node == nil {
		return nil
	}
	skip := make(map[string]struct{}, len(known))
	for _, key := range known {
		skip[key] = struct{}{}
	}
	var out map[string]any
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		if key == nil {
			continue
		}
		if _, ok := skip[key.Value]; ok {
			continue
		}
		if out == nil {
			out = map[string]any{}
		}
		out[key.Value] = decodeNode(node.Content[i+1])
	}
	return out
}

// refExtensions reads x- keys written beside a schema $ref. kin stores those
// on the reference, not on the resolved schema.
func refExtensions(in *base.SchemaProxy) map[string]any {
	node := authoredSchemaNode(in)
	if node == nil {
		return nil
	}
	var out map[string]any
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		if key == nil || !strings.HasPrefix(key.Value, "x-") {
			continue
		}
		if out == nil {
			out = map[string]any{}
		}
		out[key.Value] = decodeNode(node.Content[i+1])
	}
	return out
}

// withRefSiblings copies a resolved schema and applies keywords written next
// to its $ref. OpenAPI 3.0 ignores those keywords. OpenAPI 3.1 annotations
// replace the referenced schema's own, and any other keyword is added as an
// allOf entry, which is what kin-openapi does. The copy keeps the referenced
// schema shared with everyone who uses it without siblings.
func (c *converter) withRefSiblings(shared *model.Schema, proxy *base.SchemaProxy) *model.Schema {
	if shared == nil || proxy == nil || !proxy.IsReference() || !c.openAPI31() || proxy.GoLow() == nil {
		return shared
	}
	keys, node := siblingNode(authoredSchemaNode(proxy))
	if len(keys) == 0 {
		return shared
	}
	sibling := c.schemaFromNode(node, proxy.GoLow().GetIndex())
	if sibling == nil {
		return shared
	}
	out := *shared
	var assertions []string
	for _, key := range keys {
		if _, ok := refSiblingAnnotations[key]; ok {
			applySchemaField(&out, sibling, key)
			continue
		}
		assertions = append(assertions, key)
	}
	if len(assertions) > 0 {
		conjunct := &model.Schema{}
		for _, key := range assertions {
			applySchemaField(conjunct, sibling, key)
		}
		out.AllOf = append(append(model.SchemaRefs{}, shared.AllOf...), &model.SchemaRef{Value: conjunct})
	}
	return &out
}

func (c *converter) openAPI31() bool {
	if c.root == nil {
		return false
	}
	switch c.root.Version {
	case "3.1", "3.1.0", "3.1.1", "3.1.2", "3.2", "3.2.0", "3.2.1":
		return true
	default:
		return false
	}
}

// authoredSchemaNode is the mapping that still contains the authored $ref.
// The proxy's value node is often rewritten to the resolved schema, while the
// built schema's root node stays the mapping the user wrote.
func authoredSchemaNode(proxy *base.SchemaProxy) *yaml.Node {
	if proxy == nil {
		return nil
	}
	if node := authoredRefNode(proxy.GetReferenceNode()); node != nil {
		return node
	}
	if schema := proxy.Schema(); schema != nil && schema.GoLow() != nil {
		if node := authoredRefNode(schema.GoLow().RootNode); node != nil {
			return node
		}
	}
	if proxy.GoLow() != nil {
		return authoredRefNode(proxy.GoLow().GetValueNode())
	}
	return nil
}

// authoredRefNode is the mapping that literally contains $ref. Once libopenapi
// has followed the reference, the value node is the target schema and no longer
// carries the authored $ref, so it is not a sibling source.
func authoredRefNode(node *yaml.Node) *yaml.Node {
	node = mappingNode(node)
	if node == nil {
		return nil
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i] != nil && node.Content[i].Value == "$ref" {
			return node
		}
	}
	return nil
}

func mappingHasKey(node *yaml.Node, name string) bool {
	node = mappingNode(node)
	if node == nil {
		return false
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i] != nil && node.Content[i].Value == name {
			return true
		}
	}
	return false
}

func siblingNode(node *yaml.Node) ([]string, *yaml.Node) {
	node = authoredRefNode(node)
	if node == nil {
		return nil, nil
	}
	out := &yaml.Node{Kind: yaml.MappingNode}
	var keys []string
	for i := 0; i+1 < len(node.Content); i += 2 {
		key := node.Content[i]
		if key == nil || key.Value == "$ref" || strings.HasPrefix(key.Value, "x-") {
			continue
		}
		keys = append(keys, key.Value)
		out.Content = append(out.Content, node.Content[i], node.Content[i+1])
	}
	if len(keys) == 0 {
		return nil, nil
	}
	return keys, out
}

func (c *converter) schemaFromNode(node *yaml.Node, idx *index.SpecIndex) *model.Schema {
	var low lowbase.Schema
	if err := low.Build(context.Background(), node, idx); err != nil {
		return nil
	}
	return c.schema(base.NewSchema(&low))
}

func applySchemaField(dst, src *model.Schema, field string) {
	switch field {
	case "oneOf":
		dst.OneOf = src.OneOf
	case "anyOf":
		dst.AnyOf = src.AnyOf
	case "allOf":
		dst.AllOf = src.AllOf
	case "not":
		dst.Not = src.Not
	case "type":
		dst.Type = src.Type
	case "title":
		dst.Title = src.Title
	case "format":
		dst.Format = src.Format
	case "description":
		dst.Description = src.Description
	case "enum":
		dst.Enum = src.Enum
	case "default":
		dst.Default = src.Default
	case "example":
		dst.Example = src.Example
	case "externalDocs":
		dst.ExternalDocs = src.ExternalDocs
	case "uniqueItems":
		dst.UniqueItems = src.UniqueItems
	case "exclusiveMinimum":
		dst.ExclusiveMin = src.ExclusiveMin
	case "exclusiveMaximum":
		dst.ExclusiveMax = src.ExclusiveMax
	case "nullable":
		dst.Nullable = src.Nullable
	case "readOnly":
		dst.ReadOnly = src.ReadOnly
	case "writeOnly":
		dst.WriteOnly = src.WriteOnly
	case "allowEmptyValue":
		dst.AllowEmptyValue = src.AllowEmptyValue
	case "deprecated":
		dst.Deprecated = src.Deprecated
	case "xml":
		dst.XML = src.XML
	case "minimum":
		dst.Min = src.Min
	case "maximum":
		dst.Max = src.Max
	case "multipleOf":
		dst.MultipleOf = src.MultipleOf
	case "minLength":
		dst.MinLength = src.MinLength
	case "maxLength":
		dst.MaxLength = src.MaxLength
	case "pattern":
		dst.Pattern = src.Pattern
	case "minItems":
		dst.MinItems = src.MinItems
	case "maxItems":
		dst.MaxItems = src.MaxItems
	case "items":
		dst.Items = src.Items
	case "required":
		dst.Required = src.Required
	case "properties":
		dst.Properties = src.Properties
	case "minProperties":
		dst.MinProps = src.MinProps
	case "maxProperties":
		dst.MaxProps = src.MaxProps
	case "additionalProperties":
		dst.AdditionalProperties = src.AdditionalProperties
	case "discriminator":
		dst.Discriminator = src.Discriminator
	case "const":
		dst.Const = src.Const
	case "examples":
		dst.Examples = src.Examples
	case "prefixItems":
		dst.PrefixItems = src.PrefixItems
	case "contains":
		dst.Contains = src.Contains
	case "minContains":
		dst.MinContains = src.MinContains
	case "maxContains":
		dst.MaxContains = src.MaxContains
	case "patternProperties":
		dst.PatternProperties = src.PatternProperties
	case "dependentSchemas":
		dst.DependentSchemas = src.DependentSchemas
	case "propertyNames":
		dst.PropertyNames = src.PropertyNames
	case "unevaluatedItems":
		dst.UnevaluatedItems = src.UnevaluatedItems
	case "unevaluatedProperties":
		dst.UnevaluatedProperties = src.UnevaluatedProperties
	case "if":
		dst.If = src.If
	case "then":
		dst.Then = src.Then
	case "else":
		dst.Else = src.Else
	case "dependentRequired":
		dst.DependentRequired = src.DependentRequired
	case "$defs":
		dst.Defs = src.Defs
	case "$schema":
		dst.SchemaDialect = src.SchemaDialect
	case "$comment":
		dst.Comment = src.Comment
	case "$id":
		dst.SchemaID = src.SchemaID
	case "$anchor":
		dst.Anchor = src.Anchor
	case "$dynamicRef":
		dst.DynamicRef = src.DynamicRef
	case "$dynamicAnchor":
		dst.DynamicAnchor = src.DynamicAnchor
	case "contentMediaType":
		dst.ContentMediaType = src.ContentMediaType
	case "contentEncoding":
		dst.ContentEncoding = src.ContentEncoding
	case "contentSchema":
		dst.ContentSchema = src.ContentSchema
	}
}
