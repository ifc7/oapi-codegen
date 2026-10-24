package kin

import (
	"maps"
	"slices"

	"github.com/getkin/kin-openapi/openapi3"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

// Convert copies a kin-openapi document into the generator's model.
//
// Each kin object becomes exactly one model object, so whatever kin shares
// stays shared: a schema reached through several $refs, or through a cycle,
// is one *model.Schema, as it is one *openapi3.Schema. The generator relies
// on that when it compares schemas by pointer.
//
// Values kin decoded into any (extensions, enum, default, example, const)
// are carried over as they are, so they keep kin's Go types.
func Convert(doc *openapi3.T) *model.Document {
	c := &converter{seen: make(map[any]any)}
	return c.document(doc)
}

type converter struct {
	// seen maps each kin pointer already converted to its model pointer.
	seen map[any]any
}

// once returns the model object for in, filling a new one the first time in
// is seen. The object is recorded before fill runs, so a cycle back to in
// gets the same object.
func once[In, Out any](c *converter, in *In, fill func(*Out)) *Out {
	if in == nil {
		return nil
	}
	if out, ok := c.seen[in]; ok {
		return out.(*Out)
	}
	out := new(Out)
	c.seen[in] = out
	fill(out)
	return out
}

func mapOf[OutMap ~map[string]*Out, InMap ~map[string]*In, In, Out any](in InMap, convert func(*In) *Out) OutMap {
	if in == nil {
		return nil
	}
	out := make(OutMap, len(in))
	for key, value := range in {
		out[key] = convert(value)
	}
	return out
}

func sliceOf[OutSlice ~[]*Out, InSlice ~[]*In, In, Out any](in InSlice, convert func(*In) *Out) OutSlice {
	if in == nil {
		return nil
	}
	out := make(OutSlice, len(in))
	for i, value := range in {
		out[i] = convert(value)
	}
	return out
}

func stringSlices(in map[string][]string) map[string][]string {
	if in == nil {
		return nil
	}
	out := make(map[string][]string, len(in))
	for key, value := range in {
		out[key] = slices.Clone(value)
	}
	return out
}

func (c *converter) origin(in *openapi3.Origin) *model.Origin {
	return once(c, in, func(out *model.Origin) {
		*out = model.Origin{
			Key: once(c, in.Key, func(key *model.Location) {
				*key = model.Location(*in.Key)
			}),
		}
		if in.Fields != nil {
			out.Fields = make(model.FieldLocations, len(in.Fields))
			for i, loc := range in.Fields {
				out.Fields[i] = model.Location(loc)
			}
		}
		if in.Sequences != nil {
			out.Sequences = make(map[string][]model.Location, len(in.Sequences))
			for name, locs := range in.Sequences {
				var items []model.Location
				if locs != nil {
					items = make([]model.Location, len(locs))
					for i, loc := range locs {
						items[i] = model.Location(loc)
					}
				}
				out.Sequences[name] = items
			}
		}
	})
}

func (c *converter) document(in *openapi3.T) *model.Document {
	return once(c, in, func(out *model.Document) {
		*out = model.Document{
			Extensions:   maps.Clone(in.Extensions),
			Origin:       c.origin(in.Origin),
			OpenAPI:      in.OpenAPI,
			Components:   c.components(in.Components),
			Info:         c.info(in.Info),
			Paths:        c.paths(in.Paths),
			Security:     c.securityRequirements(in.Security),
			Servers:      sliceOf[model.Servers](in.Servers, c.server),
			Tags:         sliceOf[model.Tags](in.Tags, c.tag),
			ExternalDocs: c.externalDocs(in.ExternalDocs),
			Webhooks:     mapOf[map[string]*model.PathItem](in.Webhooks, c.pathItem),
		}
	})
}

func (c *converter) info(in *openapi3.Info) *model.Info {
	return once(c, in, func(out *model.Info) {
		*out = model.Info{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Title:       in.Title,
			Summary:     in.Summary,
			Description: in.Description,
			Version:     in.Version,
		}
	})
}

func (c *converter) components(in *openapi3.Components) *model.Components {
	return once(c, in, func(out *model.Components) {
		*out = model.Components{
			Extensions:      maps.Clone(in.Extensions),
			Origin:          c.origin(in.Origin),
			Schemas:         mapOf[model.Schemas](in.Schemas, c.schemaRef),
			Parameters:      mapOf[model.ParametersMap](in.Parameters, c.parameterRef),
			Headers:         mapOf[model.Headers](in.Headers, c.headerRef),
			RequestBodies:   mapOf[model.RequestBodies](in.RequestBodies, c.requestBodyRef),
			Responses:       mapOf[model.ResponseBodies](in.Responses, c.responseRef),
			SecuritySchemes: mapOf[model.SecuritySchemes](in.SecuritySchemes, c.securitySchemeRef),
			Examples:        mapOf[model.Examples](in.Examples, c.exampleRef),
			Links:           mapOf[model.Links](in.Links, c.linkRef),
			Callbacks:       mapOf[model.Callbacks](in.Callbacks, c.callbackRef),
		}
	})
}

func (c *converter) securityRequirements(in openapi3.SecurityRequirements) model.SecurityRequirements {
	if in == nil {
		return nil
	}
	out := make(model.SecurityRequirements, len(in))
	for i, requirement := range in {
		out[i] = stringSlices(requirement)
	}
	return out
}

func (c *converter) securitySchemeRef(in *openapi3.SecuritySchemeRef) *model.SecuritySchemeRef {
	return once(c, in, func(out *model.SecuritySchemeRef) {
		*out = model.SecuritySchemeRef{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Summary:     in.Summary,
			Description: in.Description,
			Ref:         in.Ref,
			Value:       c.securityScheme(in.Value),
		}
	})
}

func (c *converter) securityScheme(in *openapi3.SecurityScheme) *model.SecurityScheme {
	return once(c, in, func(out *model.SecurityScheme) {
		*out = model.SecurityScheme{
			Extensions:       maps.Clone(in.Extensions),
			Origin:           c.origin(in.Origin),
			Type:             in.Type,
			Description:      in.Description,
			Name:             in.Name,
			In:               in.In,
			Scheme:           in.Scheme,
			BearerFormat:     in.BearerFormat,
			OpenIdConnectUrl: in.OpenIdConnectUrl,
		}
	})
}

func (c *converter) server(in *openapi3.Server) *model.Server {
	return once(c, in, func(out *model.Server) {
		*out = model.Server{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			URL:         in.URL,
			Description: in.Description,
			Variables:   mapOf[model.ServerVariables](in.Variables, c.serverVariable),
		}
	})
}

func (c *converter) serverVariable(in *openapi3.ServerVariable) *model.ServerVariable {
	return once(c, in, func(out *model.ServerVariable) {
		*out = model.ServerVariable{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Enum:        slices.Clone(in.Enum),
			Default:     in.Default,
			Description: in.Description,
		}
	})
}

func (c *converter) tag(in *openapi3.Tag) *model.Tag {
	return once(c, in, func(out *model.Tag) {
		*out = model.Tag{
			Extensions:   maps.Clone(in.Extensions),
			Origin:       c.origin(in.Origin),
			Name:         in.Name,
			Description:  in.Description,
			ExternalDocs: c.externalDocs(in.ExternalDocs),
		}
	})
}

func (c *converter) externalDocs(in *openapi3.ExternalDocs) *model.ExternalDocs {
	return once(c, in, func(out *model.ExternalDocs) {
		*out = model.ExternalDocs{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Description: in.Description,
			URL:         in.URL,
		}
	})
}

func (c *converter) paths(in *openapi3.Paths) *model.Paths {
	return once(c, in, func(out *model.Paths) {
		out.Extensions = maps.Clone(in.Extensions)
		out.Origin = c.origin(in.Origin)
		for key, item := range in.Map() {
			out.Set(key, c.pathItem(item))
		}
	})
}

func (c *converter) pathItem(in *openapi3.PathItem) *model.PathItem {
	return once(c, in, func(out *model.PathItem) {
		*out = model.PathItem{
			Extensions:           maps.Clone(in.Extensions),
			Origin:               c.origin(in.Origin),
			Ref:                  in.Ref,
			Summary:              in.Summary,
			Description:          in.Description,
			Connect:              c.operation(in.Connect),
			Delete:               c.operation(in.Delete),
			Get:                  c.operation(in.Get),
			Head:                 c.operation(in.Head),
			Options:              c.operation(in.Options),
			Patch:                c.operation(in.Patch),
			Post:                 c.operation(in.Post),
			Put:                  c.operation(in.Put),
			Trace:                c.operation(in.Trace),
			Query:                c.operation(in.Query),
			Servers:              sliceOf[model.Servers](in.Servers, c.server),
			Parameters:           sliceOf[model.Parameters](in.Parameters, c.parameterRef),
			AdditionalOperations: mapOf[map[string]*model.Operation](in.AdditionalOperations, c.operation),
		}
	})
}

func (c *converter) operation(in *openapi3.Operation) *model.Operation {
	return once(c, in, func(out *model.Operation) {
		*out = model.Operation{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Tags:        slices.Clone(in.Tags),
			Summary:     in.Summary,
			Description: in.Description,
			OperationID: in.OperationID,
			Parameters:  sliceOf[model.Parameters](in.Parameters, c.parameterRef),
			RequestBody: c.requestBodyRef(in.RequestBody),
			Responses:   c.responses(in.Responses),
			Callbacks:   mapOf[model.Callbacks](in.Callbacks, c.callbackRef),
			Deprecated:  in.Deprecated,
			Security: once(c, in.Security, func(security *model.SecurityRequirements) {
				*security = c.securityRequirements(*in.Security)
			}),
			Servers: once(c, in.Servers, func(servers *model.Servers) {
				*servers = sliceOf[model.Servers](*in.Servers, c.server)
			}),
			ExternalDocs: c.externalDocs(in.ExternalDocs),
		}
	})
}

func (c *converter) responses(in *openapi3.Responses) *model.Responses {
	return once(c, in, func(out *model.Responses) {
		out.Extensions = maps.Clone(in.Extensions)
		out.Origin = c.origin(in.Origin)
		for key, response := range in.Map() {
			out.Set(key, c.responseRef(response))
		}
	})
}

func (c *converter) callbackRef(in *openapi3.CallbackRef) *model.CallbackRef {
	return once(c, in, func(out *model.CallbackRef) {
		*out = model.CallbackRef{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Summary:     in.Summary,
			Description: in.Description,
			Ref:         in.Ref,
			Value:       c.callback(in.Value),
		}
	})
}

func (c *converter) callback(in *openapi3.Callback) *model.Callback {
	return once(c, in, func(out *model.Callback) {
		out.Extensions = maps.Clone(in.Extensions)
		out.Origin = c.origin(in.Origin)
		for key, item := range in.Map() {
			out.Set(key, c.pathItem(item))
		}
	})
}

func (c *converter) parameterRef(in *openapi3.ParameterRef) *model.ParameterRef {
	return once(c, in, func(out *model.ParameterRef) {
		*out = model.ParameterRef{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Summary:     in.Summary,
			Description: in.Description,
			Ref:         in.Ref,
			Value:       c.parameter(in.Value),
		}
	})
}

func (c *converter) parameter(in *openapi3.Parameter) *model.Parameter {
	return once(c, in, func(out *model.Parameter) {
		*out = c.parameterValue(in)
	})
}

func (c *converter) parameterValue(in *openapi3.Parameter) model.Parameter {
	return model.Parameter{
		Extensions:      maps.Clone(in.Extensions),
		Origin:          c.origin(in.Origin),
		Name:            in.Name,
		In:              in.In,
		Description:     in.Description,
		Style:           in.Style,
		Explode:         in.Explode,
		AllowEmptyValue: in.AllowEmptyValue,
		AllowReserved:   in.AllowReserved,
		Deprecated:      in.Deprecated,
		Required:        in.Required,
		Schema:          c.schemaRef(in.Schema),
		Example:         in.Example,
		Examples:        mapOf[model.Examples](in.Examples, c.exampleRef),
		Content:         mapOf[model.Content](in.Content, c.mediaType),
	}
}

func (c *converter) headerRef(in *openapi3.HeaderRef) *model.HeaderRef {
	return once(c, in, func(out *model.HeaderRef) {
		*out = model.HeaderRef{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Summary:     in.Summary,
			Description: in.Description,
			Ref:         in.Ref,
			Value:       c.header(in.Value),
		}
	})
}

func (c *converter) header(in *openapi3.Header) *model.Header {
	return once(c, in, func(out *model.Header) {
		out.Parameter = c.parameterValue(&in.Parameter)
	})
}

func (c *converter) requestBodyRef(in *openapi3.RequestBodyRef) *model.RequestBodyRef {
	return once(c, in, func(out *model.RequestBodyRef) {
		*out = model.RequestBodyRef{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Summary:     in.Summary,
			Description: in.Description,
			Ref:         in.Ref,
			Value:       c.requestBody(in.Value),
		}
	})
}

func (c *converter) requestBody(in *openapi3.RequestBody) *model.RequestBody {
	return once(c, in, func(out *model.RequestBody) {
		*out = model.RequestBody{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Description: in.Description,
			Required:    in.Required,
			Content:     mapOf[model.Content](in.Content, c.mediaType),
		}
	})
}

func (c *converter) responseRef(in *openapi3.ResponseRef) *model.ResponseRef {
	return once(c, in, func(out *model.ResponseRef) {
		*out = model.ResponseRef{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Summary:     in.Summary,
			Description: in.Description,
			Ref:         in.Ref,
			Value:       c.response(in.Value),
		}
	})
}

func (c *converter) response(in *openapi3.Response) *model.Response {
	return once(c, in, func(out *model.Response) {
		*out = model.Response{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Description: in.Description,
			Headers:     mapOf[model.Headers](in.Headers, c.headerRef),
			Content:     mapOf[model.Content](in.Content, c.mediaType),
			Links:       mapOf[model.Links](in.Links, c.linkRef),
		}
	})
}

func (c *converter) mediaType(in *openapi3.MediaType) *model.MediaType {
	return once(c, in, func(out *model.MediaType) {
		*out = model.MediaType{
			Extensions: maps.Clone(in.Extensions),
			Origin:     c.origin(in.Origin),
			Schema:     c.schemaRef(in.Schema),
			ItemSchema: c.schemaRef(in.ItemSchema),
			Example:    in.Example,
			Examples:   mapOf[model.Examples](in.Examples, c.exampleRef),
			Encoding:   mapOf[model.Encodings](in.Encoding, c.encoding),
		}
	})
}

func (c *converter) encoding(in *openapi3.Encoding) *model.Encoding {
	return once(c, in, func(out *model.Encoding) {
		*out = model.Encoding{
			Extensions:    maps.Clone(in.Extensions),
			Origin:        c.origin(in.Origin),
			ContentType:   in.ContentType,
			Headers:       mapOf[model.Headers](in.Headers, c.headerRef),
			Style:         in.Style,
			Explode:       in.Explode,
			AllowReserved: in.AllowReserved,
		}
	})
}

func (c *converter) exampleRef(in *openapi3.ExampleRef) *model.ExampleRef {
	return once(c, in, func(out *model.ExampleRef) {
		*out = model.ExampleRef{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Summary:     in.Summary,
			Description: in.Description,
			Ref:         in.Ref,
			Value:       c.example(in.Value),
		}
	})
}

func (c *converter) example(in *openapi3.Example) *model.Example {
	return once(c, in, func(out *model.Example) {
		*out = model.Example{
			Extensions:    maps.Clone(in.Extensions),
			Origin:        c.origin(in.Origin),
			Summary:       in.Summary,
			Description:   in.Description,
			Value:         in.Value,
			ExternalValue: in.ExternalValue,
		}
	})
}

func (c *converter) linkRef(in *openapi3.LinkRef) *model.LinkRef {
	return once(c, in, func(out *model.LinkRef) {
		*out = model.LinkRef{
			Extensions:  maps.Clone(in.Extensions),
			Origin:      c.origin(in.Origin),
			Summary:     in.Summary,
			Description: in.Description,
			Ref:         in.Ref,
			Value:       c.link(in.Value),
		}
	})
}

func (c *converter) link(in *openapi3.Link) *model.Link {
	return once(c, in, func(out *model.Link) {
		*out = model.Link{
			Extensions:   maps.Clone(in.Extensions),
			Origin:       c.origin(in.Origin),
			OperationRef: in.OperationRef,
			OperationID:  in.OperationID,
			Description:  in.Description,
			Parameters:   maps.Clone(in.Parameters),
			Server:       c.server(in.Server),
			RequestBody:  in.RequestBody,
		}
	})
}

func (c *converter) schemaRef(in *openapi3.SchemaRef) *model.SchemaRef {
	return once(c, in, func(out *model.SchemaRef) {
		*out = model.SchemaRef{
			Extensions: maps.Clone(in.Extensions),
			Origin:     c.origin(in.Origin),
			Ref:        in.Ref,
			Value:      c.schema(in.Value),
		}
	})
}

func (c *converter) boolSchema(in openapi3.BoolSchema) model.BoolSchema {
	return model.BoolSchema{Has: in.Has, Schema: c.schemaRef(in.Schema)}
}

func (c *converter) schema(in *openapi3.Schema) *model.Schema {
	return once(c, in, func(out *model.Schema) {
		*out = model.Schema{
			Extensions: maps.Clone(in.Extensions),
			Origin:     c.origin(in.Origin),

			OneOf: sliceOf[model.SchemaRefs](in.OneOf, c.schemaRef),
			AnyOf: sliceOf[model.SchemaRefs](in.AnyOf, c.schemaRef),
			AllOf: sliceOf[model.SchemaRefs](in.AllOf, c.schemaRef),
			Not:   c.schemaRef(in.Not),
			Type: once(c, in.Type, func(types *model.Types) {
				*types = model.Types(slices.Clone(*in.Type))
			}),
			Title:        in.Title,
			Format:       in.Format,
			Description:  in.Description,
			Enum:         slices.Clone(in.Enum),
			Default:      in.Default,
			Example:      in.Example,
			ExternalDocs: c.externalDocs(in.ExternalDocs),

			UniqueItems:  in.UniqueItems,
			ExclusiveMin: model.ExclusiveBound(in.ExclusiveMin),
			ExclusiveMax: model.ExclusiveBound(in.ExclusiveMax),

			Nullable:        in.Nullable,
			ReadOnly:        in.ReadOnly,
			WriteOnly:       in.WriteOnly,
			AllowEmptyValue: in.AllowEmptyValue,
			Deprecated:      in.Deprecated,
			XML:             c.xml(in.XML),

			Min:        in.Min,
			Max:        in.Max,
			MultipleOf: in.MultipleOf,

			MinLength: in.MinLength,
			MaxLength: in.MaxLength,
			Pattern:   in.Pattern,

			MinItems: in.MinItems,
			MaxItems: in.MaxItems,
			Items:    c.schemaRef(in.Items),

			Required:             slices.Clone(in.Required),
			Properties:           mapOf[model.Schemas](in.Properties, c.schemaRef),
			MinProps:             in.MinProps,
			MaxProps:             in.MaxProps,
			AdditionalProperties: c.boolSchema(in.AdditionalProperties),
			Discriminator:        c.discriminator(in.Discriminator),

			Const:                 in.Const,
			Examples:              slices.Clone(in.Examples),
			PrefixItems:           sliceOf[model.SchemaRefs](in.PrefixItems, c.schemaRef),
			Contains:              c.schemaRef(in.Contains),
			MinContains:           in.MinContains,
			MaxContains:           in.MaxContains,
			PatternProperties:     mapOf[model.Schemas](in.PatternProperties, c.schemaRef),
			DependentSchemas:      mapOf[model.Schemas](in.DependentSchemas, c.schemaRef),
			PropertyNames:         c.schemaRef(in.PropertyNames),
			UnevaluatedItems:      c.boolSchema(in.UnevaluatedItems),
			UnevaluatedProperties: c.boolSchema(in.UnevaluatedProperties),

			If:   c.schemaRef(in.If),
			Then: c.schemaRef(in.Then),
			Else: c.schemaRef(in.Else),

			DependentRequired: stringSlices(in.DependentRequired),

			Always: in.Always,

			Defs:          mapOf[model.Schemas](in.Defs, c.schemaRef),
			SchemaDialect: in.SchemaDialect,
			Comment:       in.Comment,

			SchemaID:      in.SchemaID,
			Anchor:        in.Anchor,
			DynamicRef:    in.DynamicRef,
			DynamicAnchor: in.DynamicAnchor,

			ContentMediaType: in.ContentMediaType,
			ContentEncoding:  in.ContentEncoding,
			ContentSchema:    c.schemaRef(in.ContentSchema),
		}
	})
}

func (c *converter) discriminator(in *openapi3.Discriminator) *model.Discriminator {
	return once(c, in, func(out *model.Discriminator) {
		*out = model.Discriminator{
			Extensions:   maps.Clone(in.Extensions),
			Origin:       c.origin(in.Origin),
			PropertyName: in.PropertyName,
		}
		if in.Mapping != nil {
			out.Mapping = make(map[string]model.MappingRef, len(in.Mapping))
			for key, ref := range in.Mapping {
				out.Mapping[key] = model.MappingRef{
					Extensions: maps.Clone(ref.Extensions),
					Origin:     c.origin(ref.Origin),
					Ref:        ref.Ref,
					Value:      c.schema(ref.Value),
				}
			}
		}
	})
}

func (c *converter) xml(in *openapi3.XML) *model.XML {
	return once(c, in, func(out *model.XML) {
		*out = model.XML{
			Extensions: maps.Clone(in.Extensions),
			Origin:     c.origin(in.Origin),
			Name:       in.Name,
			Namespace:  in.Namespace,
			Prefix:     in.Prefix,
			Attribute:  in.Attribute,
			Wrapped:    in.Wrapped,
		}
	})
}
