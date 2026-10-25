package libopenapi

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/pb33f/go-yaml"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

// Convert copies a libopenapi document into the generator's model.
//
// A schema reached through several $refs is one *model.Schema. The identity
// is the resolved schema's source node, because libopenapi builds a fresh
// high-level schema for each proxy.
func Convert(doc *v3.Document) *model.Document {
	if doc == nil {
		return nil
	}
	c := &converter{
		root:    doc,
		schemas: make(map[string]*model.Schema),
		seen:    make(map[any]any),
	}
	return c.document(doc)
}

type converter struct {
	root *v3.Document
	// schemas maps a resolved schema node to the one model schema built for it.
	schemas map[string]*model.Schema
	// seen maps a libopenapi low-level object to the model object built for it.
	seen map[any]any
}

func (c *converter) document(in *v3.Document) *model.Document {
	out := &model.Document{
		Extensions:   documentExtensions(in),
		OpenAPI:      in.Version,
		Components:   c.components(in.Components),
		Info:         c.info(in.Info),
		Paths:        c.paths(in.Paths),
		Security:     c.securityRequirements(in.Security),
		Servers:      c.documentServers(in),
		Tags:         c.tags(in.Tags),
		ExternalDocs: c.externalDocs(in.ExternalDocs),
		Webhooks:     c.pathItemMap(in.Webhooks),
	}
	return out
}

func (c *converter) info(in *base.Info) *model.Info {
	if in == nil {
		return nil
	}
	return &model.Info{
		Extensions:  decodeExtensions(in.Extensions),
		Title:       in.Title,
		Summary:     in.Summary,
		Description: in.Description,
		Version:     in.Version,
	}
}

func (c *converter) components(in *v3.Components) *model.Components {
	if in == nil {
		return nil
	}
	return &model.Components{
		Extensions:      nodeExtensions(rootNode(in.GoLow()), componentExtensionKeys, in.Extensions),
		Schemas:         c.schemaMap(in.Schemas),
		Parameters:      c.parameterMap(in.Parameters),
		Headers:         c.headerMap(in.Headers),
		RequestBodies:   c.requestBodyMap(in.RequestBodies),
		Responses:       c.responseMap(in.Responses),
		SecuritySchemes: c.securitySchemeMap(in.SecuritySchemes),
		Examples:        c.exampleMap(in.Examples),
		Links:           c.linkMap(in.Links),
		Callbacks:       c.callbackMap(in.Callbacks),
	}
}

func (c *converter) paths(in *v3.Paths) *model.Paths {
	if in == nil {
		return nil
	}
	out := model.NewPaths()
	out.Extensions = emptyMap(decodeExtensions(in.Extensions))
	lines := pathKeyLines(in)
	for pair := orderedmap.First(in.PathItems); pair != nil; pair = pair.Next() {
		item := c.pathItem(pair.Value())
		if item != nil {
			if line, ok := lines[pair.Key()]; ok {
				item.Origin = &model.Origin{Key: &model.Location{Line: line}}
			}
		}
		out.Set(pair.Key(), item)
	}
	return out
}

func pathKeyLines(in *v3.Paths) map[string]int {
	low := in.GoLow()
	if low == nil || low.PathItems == nil {
		return nil
	}
	lines := make(map[string]int)
	for pair := orderedmap.First(low.PathItems); pair != nil; pair = pair.Next() {
		key := pair.Key()
		if key.KeyNode != nil {
			lines[key.Value] = key.KeyNode.Line
		}
	}
	return lines
}

func (c *converter) pathItemMap(in *orderedmap.Map[string, *v3.PathItem]) map[string]*model.PathItem {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(map[string]*model.PathItem, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = c.pathItem(pair.Value())
	}
	return out
}

func (c *converter) pathItem(in *v3.PathItem) *model.PathItem {
	if in == nil {
		return nil
	}
	if low := in.GoLow(); low != nil {
		if seen, ok := c.seen[low]; ok {
			return seen.(*model.PathItem)
		}
	}
	out := &model.PathItem{
		Extensions:  decodeExtensions(in.Extensions),
		Ref:         authoredReference(in.GetReference(), in.GoLow()),
		Summary:     in.Summary,
		Description: in.Description,
	}
	if low := in.GoLow(); low != nil {
		c.seen[low] = out
	}
	out.Delete = c.operation(in.Delete)
	out.Get = c.operation(in.Get)
	out.Head = c.operation(in.Head)
	out.Options = c.operation(in.Options)
	out.Patch = c.operation(in.Patch)
	out.Post = c.operation(in.Post)
	out.Put = c.operation(in.Put)
	out.Trace = c.operation(in.Trace)
	out.Query = c.operation(in.Query)
	out.Servers = c.servers(in.Servers)
	out.Parameters = c.parameters(in.Parameters)
	if orderedmap.Len(in.AdditionalOperations) > 0 {
		out.AdditionalOperations = make(map[string]*model.Operation, orderedmap.Len(in.AdditionalOperations))
		for pair := orderedmap.First(in.AdditionalOperations); pair != nil; pair = pair.Next() {
			out.AdditionalOperations[pair.Key()] = c.operation(pair.Value())
		}
	}
	return out
}

func (c *converter) operation(in *v3.Operation) *model.Operation {
	if in == nil {
		return nil
	}
	if low := in.GoLow(); low != nil {
		if seen, ok := c.seen[low]; ok {
			return seen.(*model.Operation)
		}
	}
	out := &model.Operation{
		Extensions:  nodeExtensions(rootNode(in.GoLow()), operationExtensionKeys, in.Extensions),
		Tags:        slices.Clone(in.Tags),
		Summary:     in.Summary,
		Description: in.Description,
		OperationID: in.OperationId,
		Deprecated:  boolValue(in.Deprecated),
	}
	if low := in.GoLow(); low != nil {
		c.seen[low] = out
	}
	out.Parameters = c.parameters(in.Parameters)
	if out.Parameters == nil && mappingHasKey(rootNode(in.GoLow()), "parameters") {
		out.Parameters = model.Parameters{}
	}
	out.RequestBody = c.requestBody(in.RequestBody)
	out.Responses = c.responses(in.Responses)
	out.Callbacks = c.callbackMap(in.Callbacks)
	out.ExternalDocs = c.externalDocs(in.ExternalDocs)
	if in.Security != nil {
		security := c.securityRequirements(in.Security)
		out.Security = &security
	}
	if in.Servers != nil {
		servers := c.servers(in.Servers)
		out.Servers = &servers
	}
	return out
}

func (c *converter) responses(in *v3.Responses) *model.Responses {
	if in == nil {
		return nil
	}
	if low := in.GoLow(); low != nil {
		if seen, ok := c.seen[low]; ok {
			return seen.(*model.Responses)
		}
	}
	out := model.NewResponses()
	out.Extensions = emptyMap(decodeExtensions(in.Extensions))
	if in.Default != nil {
		out.Set("default", c.response(in.Default))
	}
	for pair := orderedmap.First(in.Codes); pair != nil; pair = pair.Next() {
		out.Set(pair.Key(), c.response(pair.Value()))
	}
	if low := in.GoLow(); low != nil {
		c.seen[low] = out
	}
	return out
}

func (c *converter) response(in *v3.Response) *model.ResponseRef {
	if in == nil {
		return nil
	}
	return &model.ResponseRef{
		Ref:   authoredReference(in.GetReference(), in.GoLow()),
		Value: c.responseValue(in),
	}
}

func (c *converter) responseValue(in *v3.Response) *model.Response {
	if low := in.GoLow(); low != nil {
		if seen, ok := c.seen[low]; ok {
			return seen.(*model.Response)
		}
	}
	out := &model.Response{
		Extensions: nodeExtensions(rootNode(in.GoLow()), responseExtensionKeys, in.Extensions),
		Description: presentString(in.GoLow(), func() (string, bool) {
			low := in.GoLow()
			if low == nil || low.Description.IsEmpty() {
				return "", false
			}
			return low.Description.Value, true
		}),
		Headers: c.headerMap(in.Headers),
		Content: c.content(in.Content),
		Links:   c.linkMap(in.Links),
	}
	if low := in.GoLow(); low != nil {
		c.seen[low] = out
	}
	return out
}

func (c *converter) parameters(in []*v3.Parameter) model.Parameters {
	if in == nil {
		return nil
	}
	out := make(model.Parameters, len(in))
	for i, param := range in {
		out[i] = c.parameter(param)
	}
	return out
}

func (c *converter) parameterMap(in *orderedmap.Map[string, *v3.Parameter]) model.ParametersMap {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(model.ParametersMap, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = c.parameter(pair.Value())
	}
	return out
}

func (c *converter) parameter(in *v3.Parameter) *model.ParameterRef {
	if in == nil {
		return nil
	}
	return &model.ParameterRef{
		Ref:   authoredReference(in.GetReference(), in.GoLow()),
		Value: c.parameterValue(in),
	}
}

func (c *converter) parameterValue(in *v3.Parameter) *model.Parameter {
	if low := in.GoLow(); low != nil {
		if seen, ok := c.seen[low]; ok {
			return seen.(*model.Parameter)
		}
	}
	out := &model.Parameter{
		Extensions:      decodeExtensions(in.Extensions),
		Name:            in.Name,
		In:              in.In,
		Description:     in.Description,
		Style:           in.Style,
		Explode:         in.Explode,
		AllowEmptyValue: in.AllowEmptyValue,
		AllowReserved:   in.AllowReserved,
		Deprecated:      in.Deprecated,
		Required:        boolValue(in.Required),
		Schema:          c.schemaProxy(in.Schema),
		Example:         decodeNode(in.Example),
		Examples:        c.exampleMap(in.Examples),
		Content:         c.content(in.Content),
	}
	if low := in.GoLow(); low != nil {
		c.seen[low] = out
	}
	return out
}

func (c *converter) headerMap(in *orderedmap.Map[string, *v3.Header]) model.Headers {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(model.Headers, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = c.header(pair.Value())
	}
	return out
}

func (c *converter) header(in *v3.Header) *model.HeaderRef {
	if in == nil {
		return nil
	}
	return &model.HeaderRef{
		Ref:   authoredReference(in.GetReference(), in.GoLow()),
		Value: c.headerValue(in),
	}
}

func (c *converter) headerValue(in *v3.Header) *model.Header {
	if low := in.GoLow(); low != nil {
		if seen, ok := c.seen[low]; ok {
			return seen.(*model.Header)
		}
	}
	out := &model.Header{Parameter: model.Parameter{
		Extensions:      decodeExtensions(in.Extensions),
		Description:     in.Description,
		Style:           in.Style,
		AllowEmptyValue: in.AllowEmptyValue,
		AllowReserved:   in.AllowReserved,
		Deprecated:      in.Deprecated,
		Required:        in.Required,
		Schema:          c.schemaProxy(in.Schema),
		Example:         decodeNode(in.Example),
		Examples:        c.exampleMap(in.Examples),
		Content:         c.content(in.Content),
	}}
	if low := in.GoLow(); low != nil && !low.Explode.IsEmpty() {
		value := low.Explode.Value
		out.Explode = &value
	}
	if low := in.GoLow(); low != nil {
		c.seen[low] = out
	}
	return out
}

func (c *converter) requestBodyMap(in *orderedmap.Map[string, *v3.RequestBody]) model.RequestBodies {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(model.RequestBodies, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = c.requestBody(pair.Value())
	}
	return out
}

func (c *converter) requestBody(in *v3.RequestBody) *model.RequestBodyRef {
	if in == nil {
		return nil
	}
	return &model.RequestBodyRef{
		Ref:   authoredReference(in.GetReference(), in.GoLow()),
		Value: c.requestBodyValue(in),
	}
}

func (c *converter) requestBodyValue(in *v3.RequestBody) *model.RequestBody {
	if low := in.GoLow(); low != nil {
		if seen, ok := c.seen[low]; ok {
			return seen.(*model.RequestBody)
		}
	}
	out := &model.RequestBody{
		Extensions:  decodeExtensions(in.Extensions),
		Description: in.Description,
		Required:    boolValue(in.Required),
		Content:     c.content(in.Content),
	}
	if low := in.GoLow(); low != nil {
		c.seen[low] = out
	}
	return out
}

func (c *converter) responseMap(in *orderedmap.Map[string, *v3.Response]) model.ResponseBodies {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(model.ResponseBodies, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = c.response(pair.Value())
	}
	return out
}

func (c *converter) content(in *orderedmap.Map[string, *v3.MediaType]) model.Content {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(model.Content, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = c.mediaType(pair.Value())
	}
	return out
}

func (c *converter) mediaType(in *v3.MediaType) *model.MediaType {
	if in == nil {
		return nil
	}
	if low := in.GoLow(); low != nil {
		if seen, ok := c.seen[low]; ok {
			return seen.(*model.MediaType)
		}
	}
	out := &model.MediaType{
		Extensions: nodeExtensions(rootNode(in.GoLow()), mediaTypeExtensionKeys, in.Extensions),
		Schema:     c.schemaProxy(in.Schema),
		ItemSchema: c.schemaProxy(in.ItemSchema),
		Example:    decodeNode(in.Example),
		Examples:   c.exampleMap(in.Examples),
		Encoding:   c.encodings(in.Encoding),
	}
	if low := in.GoLow(); low != nil {
		c.seen[low] = out
	}
	return out
}

func (c *converter) encodings(in *orderedmap.Map[string, *v3.Encoding]) model.Encodings {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(model.Encodings, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = c.encoding(pair.Value())
	}
	return out
}

func (c *converter) encoding(in *v3.Encoding) *model.Encoding {
	if in == nil {
		return nil
	}
	return &model.Encoding{
		ContentType:   in.ContentType,
		Headers:       c.headerMap(in.Headers),
		Style:         in.Style,
		Explode:       in.Explode,
		AllowReserved: in.AllowReserved,
	}
}

func (c *converter) exampleMap(in *orderedmap.Map[string, *base.Example]) model.Examples {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(model.Examples, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = c.example(pair.Value())
	}
	return out
}

func (c *converter) example(in *base.Example) *model.ExampleRef {
	if in == nil {
		return nil
	}
	return &model.ExampleRef{
		Ref:   authoredReference(in.GetReference(), in.GoLow()),
		Value: c.exampleValue(in),
	}
}

func (c *converter) exampleValue(in *base.Example) *model.Example {
	if low := in.GoLow(); low != nil {
		if seen, ok := c.seen[low]; ok {
			return seen.(*model.Example)
		}
	}
	out := &model.Example{
		Extensions:    decodeExtensions(in.Extensions),
		Summary:       in.Summary,
		Description:   in.Description,
		Value:         decodeNode(in.Value),
		ExternalValue: in.ExternalValue,
	}
	if low := in.GoLow(); low != nil {
		c.seen[low] = out
	}
	return out
}

func (c *converter) linkMap(in *orderedmap.Map[string, *v3.Link]) model.Links {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(model.Links, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = c.link(pair.Value())
	}
	return out
}

func (c *converter) link(in *v3.Link) *model.LinkRef {
	if in == nil {
		return nil
	}
	return &model.LinkRef{
		Ref:   authoredReference(in.GetReference(), in.GoLow()),
		Value: c.linkValue(in),
	}
}

func (c *converter) linkValue(in *v3.Link) *model.Link {
	if low := in.GoLow(); low != nil {
		if seen, ok := c.seen[low]; ok {
			return seen.(*model.Link)
		}
	}
	out := &model.Link{
		Extensions:   decodeExtensions(in.Extensions),
		OperationRef: in.OperationRef,
		OperationID:  in.OperationId,
		Description:  in.Description,
		Parameters:   stringMapAny(in.Parameters),
		Server:       c.server(in.Server),
	}
	if in.RequestBody != "" {
		out.RequestBody = in.RequestBody
	}
	if low := in.GoLow(); low != nil {
		c.seen[low] = out
	}
	return out
}

func (c *converter) callbackMap(in *orderedmap.Map[string, *v3.Callback]) model.Callbacks {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(model.Callbacks, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = c.callback(pair.Value())
	}
	return out
}

func (c *converter) callback(in *v3.Callback) *model.CallbackRef {
	if in == nil {
		return nil
	}
	return &model.CallbackRef{
		Ref:   authoredReference(in.GetReference(), in.GoLow()),
		Value: c.callbackValue(in),
	}
}

func (c *converter) callbackValue(in *v3.Callback) *model.Callback {
	if low := in.GoLow(); low != nil {
		if seen, ok := c.seen[low]; ok {
			return seen.(*model.Callback)
		}
	}
	out := model.NewCallback()
	out.Extensions = emptyMap(decodeExtensions(in.Extensions))
	for pair := orderedmap.First(in.Expression); pair != nil; pair = pair.Next() {
		out.Set(pair.Key(), c.pathItem(pair.Value()))
	}
	if low := in.GoLow(); low != nil {
		c.seen[low] = out
	}
	return out
}

func (c *converter) securitySchemeMap(in *orderedmap.Map[string, *v3.SecurityScheme]) model.SecuritySchemes {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(model.SecuritySchemes, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = c.securityScheme(pair.Value())
	}
	return out
}

func (c *converter) securityScheme(in *v3.SecurityScheme) *model.SecuritySchemeRef {
	if in == nil {
		return nil
	}
	return &model.SecuritySchemeRef{
		Ref: authoredReference(in.GetReference(), in.GoLow()),
		Value: &model.SecurityScheme{
			Extensions:       decodeExtensions(in.Extensions),
			Type:             in.Type,
			Description:      in.Description,
			Name:             in.Name,
			In:               in.In,
			Scheme:           in.Scheme,
			BearerFormat:     in.BearerFormat,
			OpenIdConnectUrl: in.OpenIdConnectUrl,
		},
	}
}

func (c *converter) documentServers(in *v3.Document) model.Servers {
	servers := c.servers(in.Servers)
	if servers == nil && in.GoLow() != nil && !in.GoLow().Servers.IsEmpty() {
		return model.Servers{}
	}
	return servers
}

func (c *converter) servers(in []*v3.Server) model.Servers {
	if in == nil {
		return nil
	}
	out := make(model.Servers, len(in))
	for i, server := range in {
		out[i] = c.server(server)
	}
	return out
}

func (c *converter) server(in *v3.Server) *model.Server {
	if in == nil {
		return nil
	}
	return &model.Server{
		Extensions:  decodeExtensions(in.Extensions),
		URL:         in.URL,
		Description: in.Description,
		Variables:   c.serverVariables(in.Variables),
	}
}

func (c *converter) serverVariables(in *orderedmap.Map[string, *v3.ServerVariable]) model.ServerVariables {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(model.ServerVariables, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		variable := pair.Value()
		if variable == nil {
			continue
		}
		out[pair.Key()] = &model.ServerVariable{
			Extensions:  decodeExtensions(variable.Extensions),
			Enum:        slices.Clone(variable.Enum),
			Default:     variable.Default,
			Description: variable.Description,
		}
	}
	return out
}

func (c *converter) tags(in []*base.Tag) model.Tags {
	if in == nil {
		return nil
	}
	out := make(model.Tags, len(in))
	for i, tag := range in {
		if tag == nil {
			continue
		}
		out[i] = &model.Tag{
			Extensions:   decodeExtensions(tag.Extensions),
			Name:         tag.Name,
			Description:  tag.Description,
			ExternalDocs: c.externalDocs(tag.ExternalDocs),
		}
	}
	return out
}

func (c *converter) externalDocs(in *base.ExternalDoc) *model.ExternalDocs {
	if in == nil {
		return nil
	}
	return &model.ExternalDocs{
		Extensions:  decodeExtensions(in.Extensions),
		Description: in.Description,
		URL:         in.URL,
	}
}

func (c *converter) securityRequirements(in []*base.SecurityRequirement) model.SecurityRequirements {
	if in == nil {
		return nil
	}
	out := make(model.SecurityRequirements, len(in))
	for i, requirement := range in {
		out[i] = securityRequirement(requirement)
	}
	return out
}

func securityRequirement(in *base.SecurityRequirement) model.SecurityRequirement {
	if in == nil || orderedmap.Len(in.Requirements) == 0 {
		return model.SecurityRequirement{}
	}
	out := make(model.SecurityRequirement, orderedmap.Len(in.Requirements))
	for pair := orderedmap.First(in.Requirements); pair != nil; pair = pair.Next() {
		scopes := slices.Clone(pair.Value())
		if scopes == nil {
			// `scheme: []` is an empty list, which kin keeps distinct from a missing scheme.
			scopes = []string{}
		}
		out[pair.Key()] = scopes
	}
	return out
}

func (c *converter) schemaMap(in *orderedmap.Map[string, *base.SchemaProxy]) model.Schemas {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(model.Schemas, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = c.schemaProxy(pair.Value())
	}
	return out
}

func (c *converter) schemaRefs(in []*base.SchemaProxy) model.SchemaRefs {
	if in == nil {
		return nil
	}
	out := make(model.SchemaRefs, len(in))
	for i, proxy := range in {
		out[i] = c.schemaProxy(proxy)
	}
	return out
}

func (c *converter) schemaProxy(in *base.SchemaProxy) *model.SchemaRef {
	if in == nil {
		return nil
	}
	ref := ""
	if in.IsReference() {
		ref = in.GetReference()
	}
	schema := in.Schema()
	if schema == nil {
		return &model.SchemaRef{Ref: ref, Extensions: refExtensions(in)}
	}
	return &model.SchemaRef{
		Ref:        ref,
		Extensions: refExtensions(in),
		Value:      c.withRefSiblings(c.schema(schema), in),
	}
}

func (c *converter) schema(in *base.Schema) *model.Schema {
	if in == nil {
		return nil
	}
	if key := schemaIdentity(in); key != "" {
		if seen, ok := c.schemas[key]; ok {
			return seen
		}
		out := &model.Schema{}
		c.schemas[key] = out
		c.fillSchema(out, in)
		return out
	}
	out := &model.Schema{}
	c.fillSchema(out, in)
	return out
}

// schemaIdentity names the resolved schema node. A $ref and the component it
// points at share that node, so both become one *model.Schema. A component
// written as a $ref keeps its own root node in the referring file; the index
// still knows the node it resolved to.
func schemaIdentity(in *base.Schema) string {
	low := in.GoLow()
	if low == nil {
		return ""
	}
	node := low.RootNode
	file := ""
	if low.Index != nil {
		file = low.Index.GetSpecAbsolutePath()
	}
	if low.ParentProxy != nil && low.ParentProxy.IsReference() && low.Index != nil {
		if ref, idx := low.Index.SearchIndexForReference(low.ParentProxy.GetReference()); ref != nil && ref.Node != nil {
			node = ref.Node
			if idx != nil {
				file = idx.GetSpecAbsolutePath()
			}
		}
	}
	if node == nil {
		return ""
	}
	return file + "\x00" + fmt.Sprintf("%p", node)
}

func (c *converter) fillSchema(out *model.Schema, in *base.Schema) {
	*out = model.Schema{
		Extensions:            decodeExtensions(in.Extensions),
		OneOf:                 c.schemaRefs(in.OneOf),
		AnyOf:                 c.schemaRefs(in.AnyOf),
		AllOf:                 c.schemaRefs(in.AllOf),
		Not:                   c.schemaProxy(in.Not),
		Type:                  schemaTypes(in.Type),
		Title:                 in.Title,
		Format:                in.Format,
		Description:           in.Description,
		Enum:                  decodeNodes(in.Enum),
		Default:               decodeNode(in.Default),
		Example:               decodeNode(in.Example),
		ExternalDocs:          c.externalDocs(in.ExternalDocs),
		UniqueItems:           boolValue(in.UniqueItems),
		ExclusiveMin:          exclusiveBound(in.ExclusiveMinimum),
		ExclusiveMax:          exclusiveBound(in.ExclusiveMaximum),
		Nullable:              boolValue(in.Nullable),
		ReadOnly:              boolValue(in.ReadOnly),
		WriteOnly:             boolValue(in.WriteOnly),
		Deprecated:            boolValue(in.Deprecated),
		XML:                   c.xml(in.XML),
		Min:                   cloneFloat(in.Minimum),
		Max:                   cloneFloat(in.Maximum),
		MultipleOf:            cloneFloat(in.MultipleOf),
		MinLength:             uint64Value(in.MinLength),
		MaxLength:             uint64Ptr(in.MaxLength),
		Pattern:               in.Pattern,
		MinItems:              uint64Value(in.MinItems),
		MaxItems:              uint64Ptr(in.MaxItems),
		Items:                 schemaOrBool(c, in.Items),
		Required:              slices.Clone(in.Required),
		Properties:            c.schemaMap(in.Properties),
		MinProps:              uint64Value(in.MinProperties),
		MaxProps:              uint64Ptr(in.MaxProperties),
		AdditionalProperties:  boolSchema(c, in.AdditionalProperties),
		Discriminator:         c.discriminator(in.Discriminator),
		Const:                 decodeNode(in.Const),
		Examples:              decodeNodes(in.Examples),
		PrefixItems:           c.schemaRefs(in.PrefixItems),
		Contains:              c.schemaProxy(in.Contains),
		MinContains:           uint64Ptr(in.MinContains),
		MaxContains:           uint64Ptr(in.MaxContains),
		PatternProperties:     c.schemaMap(in.PatternProperties),
		DependentSchemas:      c.schemaMap(in.DependentSchemas),
		PropertyNames:         c.schemaProxy(in.PropertyNames),
		UnevaluatedItems:      c.proxyBoolSchema(in.UnevaluatedItems),
		UnevaluatedProperties: boolSchema(c, in.UnevaluatedProperties),
		If:                    c.schemaProxy(in.If),
		Then:                  c.schemaProxy(in.Then),
		Else:                  c.schemaProxy(in.Else),
		DependentRequired:     stringMapStrings(in.DependentRequired),
		Defs:                  c.schemaMap(in.Defs),
		Comment:               in.Comment,
		SchemaID:              in.Id,
		Anchor:                in.Anchor,
		DynamicRef:            in.DynamicRef,
		DynamicAnchor:         in.DynamicAnchor,
		ContentMediaType:      in.ContentMediaType,
		ContentEncoding:       in.ContentEncoding,
		ContentSchema:         c.schemaProxy(in.ContentSchema),
	}
	if low := in.GoLow(); low != nil && !low.SchemaTypeRef.IsEmpty() {
		out.SchemaDialect = low.SchemaTypeRef.Value
	}
	if low := in.GoLow(); low != nil && low.RootNode != nil && tagName(low.RootNode.ShortTag()) == "bool" {
		value := scalarBool(low.RootNode.Value)
		out.Always = &value
	}
	if out.Properties == nil && in.GoLow() != nil && !in.GoLow().Properties.IsEmpty() {
		out.Properties = model.Schemas{}
	}
}

func (c *converter) proxyBoolSchema(in *base.SchemaProxy) model.BoolSchema {
	if in == nil {
		return model.BoolSchema{}
	}
	return model.BoolSchema{Schema: c.schemaProxy(in)}
}

func (c *converter) discriminator(in *base.Discriminator) *model.Discriminator {
	if in == nil {
		return nil
	}
	out := &model.Discriminator{PropertyName: in.PropertyName}
	if orderedmap.Len(in.Mapping) == 0 {
		return out
	}
	out.Mapping = make(map[string]model.MappingRef, orderedmap.Len(in.Mapping))
	for pair := orderedmap.First(in.Mapping); pair != nil; pair = pair.Next() {
		// kin-openapi keeps the mapping as a string. The resolved schema stays nil.
		out.Mapping[pair.Key()] = model.MappingRef{Ref: pair.Value()}
	}
	return out
}

func (c *converter) xml(in *base.XML) *model.XML {
	if in == nil {
		return nil
	}
	return &model.XML{
		Name:      in.Name,
		Namespace: in.Namespace,
		Prefix:    in.Prefix,
		Attribute: in.Attribute,
		Wrapped:   in.Wrapped,
	}
}

func schemaTypes(in []string) *model.Types {
	if in == nil {
		return nil
	}
	types := model.Types(slices.Clone(in))
	return &types
}

func exclusiveBound(in *base.DynamicValue[bool, float64]) model.ExclusiveBound {
	if in == nil {
		return model.ExclusiveBound{}
	}
	if in.IsA() {
		value := in.A
		return model.ExclusiveBound{Bool: &value}
	}
	value := in.B
	return model.ExclusiveBound{Value: &value}
}

func boolSchema(c *converter, in *base.DynamicValue[*base.SchemaProxy, bool]) model.BoolSchema {
	if in == nil {
		return model.BoolSchema{}
	}
	if in.IsA() {
		return model.BoolSchema{Schema: c.schemaProxy(in.A)}
	}
	value := in.B
	return model.BoolSchema{Has: &value}
}

func schemaOrBool(c *converter, in *base.DynamicValue[*base.SchemaProxy, bool]) *model.SchemaRef {
	if in == nil {
		return nil
	}
	if in.IsA() {
		return c.schemaProxy(in.A)
	}
	value := in.B
	return &model.SchemaRef{Value: &model.Schema{Always: &value}}
}

// authoredReference returns the $ref as it was written. libopenapi keeps that
// string on the low-level object; the high-level Reference field is often empty.
func authoredReference[T interface{ GetReference() string }](high string, low *T) string {
	if high != "" {
		return high
	}
	if low == nil {
		return ""
	}
	return (*low).GetReference()
}

func boolValue(in *bool) bool {
	return in != nil && *in
}

func uint64Value(in *int64) uint64 {
	if in == nil || *in < 0 {
		return 0
	}
	return uint64(*in)
}

func uint64Ptr(in *int64) *uint64 {
	if in == nil || *in < 0 {
		return nil
	}
	value := uint64(*in)
	return &value
}

func cloneFloat(in *float64) *float64 {
	if in == nil {
		return nil
	}
	value := *in
	return &value
}

func presentString(_ any, value func() (string, bool)) *string {
	text, ok := value()
	if !ok {
		return nil
	}
	return &text
}

func stringMapStrings(in *orderedmap.Map[string, []string]) map[string][]string {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(map[string][]string, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = slices.Clone(pair.Value())
	}
	return out
}

func stringMapAny(in *orderedmap.Map[string, string]) map[string]any {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(map[string]any, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = pair.Value()
	}
	return out
}

func emptyMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}

func decodeExtensions(in *orderedmap.Map[string, *yaml.Node]) map[string]any {
	if orderedmap.Len(in) == 0 {
		return nil
	}
	out := make(map[string]any, orderedmap.Len(in))
	for pair := orderedmap.First(in); pair != nil; pair = pair.Next() {
		out[pair.Key()] = decodeNode(pair.Value())
	}
	return out
}

func decodeNodes(in []*yaml.Node) []any {
	if in == nil {
		return nil
	}
	out := make([]any, len(in))
	for i, node := range in {
		out[i] = decodeNode(node)
	}
	return out
}

// decodeNode turns a libopenapi YAML node into the Go value kin-openapi
// leaves in an any field. kin decodes YAML through JSON, so every number is a
// float64, and timestamps stay strings.
func decodeNode(node *yaml.Node) any {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case yaml.DocumentNode, yaml.StreamNode:
		if len(node.Content) == 0 {
			return nil
		}
		return decodeNode(node.Content[0])
	case yaml.AliasNode:
		return decodeNode(node.Alias)
	case yaml.MappingNode:
		out := make(map[string]any, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			out[scalarKey(node.Content[i])] = decodeNode(node.Content[i+1])
		}
		return out
	case yaml.SequenceNode:
		out := make([]any, len(node.Content))
		for i, item := range node.Content {
			out[i] = decodeNode(item)
		}
		return out
	case yaml.ScalarNode:
		return decodeScalar(node)
	default:
		return nil
	}
}

func scalarKey(node *yaml.Node) string {
	if node == nil {
		return ""
	}
	switch tagName(node.ShortTag()) {
	case "str", "timestamp":
		return node.Value
	default:
		return fmt.Sprint(decodeNode(node))
	}
}

func decodeScalar(node *yaml.Node) any {
	switch tagName(node.ShortTag()) {
	case "null":
		return nil
	case "bool":
		return scalarBool(node.Value)
	case "int", "float":
		if value, ok := scalarNumber(node.Value); ok {
			return value
		}
		return node.Value
	default:
		return node.Value
	}
}

func tagName(tag string) string {
	if name, ok := strings.CutPrefix(tag, "!!"); ok {
		return name
	}
	if i := strings.LastIndex(tag, ":"); i >= 0 {
		return tag[i+1:]
	}
	return tag
}

func scalarBool(value string) bool {
	switch strings.ToLower(value) {
	case "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func scalarNumber(value string) (float64, bool) {
	value = strings.ReplaceAll(value, "_", "")
	switch strings.ToLower(value) {
	case ".inf", "+.inf", "inf", "+inf":
		value = "+Inf"
	case "-.inf", "-inf":
		value = "-Inf"
	case ".nan", "nan":
		value = "NaN"
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil {
		return number, true
	}
	if number, err := strconv.ParseInt(value, 0, 64); err == nil {
		return float64(number), true
	}
	if number, err := strconv.ParseUint(value, 0, 64); err == nil {
		return float64(number), true
	}
	return 0, false
}
