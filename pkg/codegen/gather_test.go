package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

func TestGatherSchemas_ComponentSchemas(t *testing.T) {
	spec := &model.Document{
		Components: &model.Components{
			Schemas: model.Schemas{
				"Pet": &model.SchemaRef{
					Value: &model.Schema{Type: &model.Types{"object"}},
				},
				"Owner": &model.SchemaRef{
					Value: &model.Schema{Type: &model.Types{"object"}},
				},
			},
		},
	}

	opts := Configuration{}
	schemas := GatherSchemas(spec, opts)

	require.Len(t, schemas, 2)

	// Sorted order: Owner, Pet
	assert.Equal(t, SchemaPath{"components", "schemas", "Owner"}, schemas[0].Path)
	assert.Equal(t, ContextComponentSchema, schemas[0].Context)
	assert.Equal(t, "Owner", schemas[0].ComponentName)

	assert.Equal(t, SchemaPath{"components", "schemas", "Pet"}, schemas[1].Path)
	assert.Equal(t, ContextComponentSchema, schemas[1].Context)
	assert.Equal(t, "Pet", schemas[1].ComponentName)
}

func TestGatherSchemas_ComponentParameters(t *testing.T) {
	spec := &model.Document{
		Components: &model.Components{
			Parameters: model.ParametersMap{
				"Limit": &model.ParameterRef{
					Value: &model.Parameter{
						Name: "limit",
						In:   "query",
						Schema: &model.SchemaRef{
							Value: &model.Schema{Type: &model.Types{"integer"}},
						},
					},
				},
			},
		},
	}

	opts := Configuration{}
	schemas := GatherSchemas(spec, opts)

	require.Len(t, schemas, 1)
	assert.Equal(t, SchemaPath{"components", "parameters", "Limit"}, schemas[0].Path)
	assert.Equal(t, ContextComponentParameter, schemas[0].Context)
	assert.Equal(t, "Limit", schemas[0].ComponentName)
}

func TestGatherSchemas_ComponentResponses(t *testing.T) {
	spec := &model.Document{
		Components: &model.Components{
			Responses: model.ResponseBodies{
				"Error": &model.ResponseRef{
					Value: &model.Response{
						Content: model.Content{
							"application/json": &model.MediaType{
								Schema: &model.SchemaRef{
									Value: &model.Schema{Type: &model.Types{"object"}},
								},
							},
						},
					},
				},
			},
		},
	}

	opts := Configuration{}
	schemas := GatherSchemas(spec, opts)

	require.Len(t, schemas, 1)
	assert.Equal(t, SchemaPath{"components", "responses", "Error", "content", "application/json"}, schemas[0].Path)
	assert.Equal(t, ContextComponentResponse, schemas[0].Context)
	assert.Equal(t, "Error", schemas[0].ComponentName)
	assert.Equal(t, "application/json", schemas[0].ContentType)
}

func TestGatherSchemas_ComponentRequestBodies(t *testing.T) {
	spec := &model.Document{
		Components: &model.Components{
			RequestBodies: model.RequestBodies{
				"CreatePet": &model.RequestBodyRef{
					Value: &model.RequestBody{
						Content: model.Content{
							"application/json": &model.MediaType{
								Schema: &model.SchemaRef{
									Value: &model.Schema{Type: &model.Types{"object"}},
								},
							},
						},
					},
				},
			},
		},
	}

	opts := Configuration{}
	schemas := GatherSchemas(spec, opts)

	require.Len(t, schemas, 1)
	assert.Equal(t, SchemaPath{"components", "requestBodies", "CreatePet", "content", "application/json"}, schemas[0].Path)
	assert.Equal(t, ContextComponentRequestBody, schemas[0].Context)
	assert.Equal(t, "CreatePet", schemas[0].ComponentName)
}

func TestGatherSchemas_ComponentHeaders(t *testing.T) {
	spec := &model.Document{
		Components: &model.Components{
			Headers: model.Headers{
				"X-Rate-Limit": &model.HeaderRef{
					Value: &model.Header{
						Parameter: model.Parameter{
							Schema: &model.SchemaRef{
								Value: &model.Schema{Type: &model.Types{"integer"}},
							},
						},
					},
				},
			},
		},
	}

	opts := Configuration{}
	schemas := GatherSchemas(spec, opts)

	require.Len(t, schemas, 1)
	assert.Equal(t, SchemaPath{"components", "headers", "X-Rate-Limit"}, schemas[0].Path)
	assert.Equal(t, ContextComponentHeader, schemas[0].Context)
}

func TestGatherSchemas_ClientResponseWrappers(t *testing.T) {
	paths := model.NewPaths()
	paths.Set("/pets", &model.PathItem{
		Get: &model.Operation{
			OperationID: "listPets",
		},
		Post: &model.Operation{
			OperationID: "createPet",
		},
	})

	spec := &model.Document{
		Paths: paths,
	}

	// Without client generation, no wrappers
	opts := Configuration{Generate: GenerateOptions{Client: false}}
	schemas := GatherSchemas(spec, opts)
	assert.Len(t, schemas, 0)

	// With client generation, wrappers are gathered
	opts = Configuration{Generate: GenerateOptions{Client: true}}
	schemas = GatherSchemas(spec, opts)
	assert.Len(t, schemas, 2)

	// Check they're sorted by operationID
	assert.Equal(t, ContextClientResponseWrapper, schemas[0].Context)
	assert.Equal(t, "createPet", schemas[0].OperationID)
	assert.Equal(t, ContextClientResponseWrapper, schemas[1].Context)
	assert.Equal(t, "listPets", schemas[1].OperationID)
}

func TestGatherSchemas_GoNameOverride_Schema(t *testing.T) {
	spec := &model.Document{
		Components: &model.Components{
			Schemas: model.Schemas{
				"Renamer": &model.SchemaRef{
					Value: &model.Schema{
						Type:       &model.Types{"object"},
						Extensions: map[string]any{"x-go-name": "SpecialName"},
					},
				},
			},
		},
	}

	opts := Configuration{}
	schemas := GatherSchemas(spec, opts)

	require.Len(t, schemas, 1)
	assert.Equal(t, "SpecialName", schemas[0].GoNameOverride)
}

func TestGatherSchemas_GoNameOverride_Response(t *testing.T) {
	spec := &model.Document{
		Components: &model.Components{
			Responses: model.ResponseBodies{
				"Outcome": &model.ResponseRef{
					Value: &model.Response{
						Extensions: map[string]any{"x-go-name": "OutcomeResult"},
						Content: model.Content{
							"application/json": &model.MediaType{
								Schema: &model.SchemaRef{
									Value: &model.Schema{Type: &model.Types{"object"}},
								},
							},
						},
					},
				},
			},
		},
	}

	opts := Configuration{}
	schemas := GatherSchemas(spec, opts)

	require.Len(t, schemas, 1)
	assert.Equal(t, "OutcomeResult", schemas[0].GoNameOverride)
}

func TestGatherSchemas_GoNameOverride_RequestBody(t *testing.T) {
	spec := &model.Document{
		Components: &model.Components{
			RequestBodies: model.RequestBodies{
				"Payload": &model.RequestBodyRef{
					Value: &model.RequestBody{
						Extensions: map[string]any{"x-go-name": "PayloadBody"},
						Content: model.Content{
							"application/json": &model.MediaType{
								Schema: &model.SchemaRef{
									Value: &model.Schema{Type: &model.Types{"object"}},
								},
							},
						},
					},
				},
			},
		},
	}

	opts := Configuration{}
	schemas := GatherSchemas(spec, opts)

	require.Len(t, schemas, 1)
	assert.Equal(t, "PayloadBody", schemas[0].GoNameOverride)
}

func TestGatherSchemas_GoNameOverride_SkippedForRef(t *testing.T) {
	spec := &model.Document{
		Components: &model.Components{
			Schemas: model.Schemas{
				"AliasedPet": &model.SchemaRef{
					Ref: "#/components/schemas/Pet",
					Value: &model.Schema{
						Type:       &model.Types{"object"},
						Extensions: map[string]any{"x-go-name": "ShouldBeIgnored"},
					},
				},
			},
		},
	}

	opts := Configuration{}
	schemas := GatherSchemas(spec, opts)

	require.Len(t, schemas, 1)
	assert.Equal(t, "", schemas[0].GoNameOverride)
}

func TestGatherSchemas_AllSections(t *testing.T) {
	// Spec with "Bar" in schemas, parameters, responses, requestBodies, headers
	// This is the issue #200 scenario (cross-section collision)
	paths := model.NewPaths()
	spec := &model.Document{
		Paths: paths,
		Components: &model.Components{
			Schemas: model.Schemas{
				"Bar": &model.SchemaRef{
					Value: &model.Schema{Type: &model.Types{"object"}},
				},
			},
			Parameters: model.ParametersMap{
				"Bar": &model.ParameterRef{
					Value: &model.Parameter{
						Name: "Bar",
						In:   "query",
						Schema: &model.SchemaRef{
							Value: &model.Schema{Type: &model.Types{"string"}},
						},
					},
				},
			},
			Responses: model.ResponseBodies{
				"Bar": &model.ResponseRef{
					Value: &model.Response{
						Content: model.Content{
							"application/json": &model.MediaType{
								Schema: &model.SchemaRef{
									Value: &model.Schema{Type: &model.Types{"object"}},
								},
							},
						},
					},
				},
			},
			RequestBodies: model.RequestBodies{
				"Bar": &model.RequestBodyRef{
					Value: &model.RequestBody{
						Content: model.Content{
							"application/json": &model.MediaType{
								Schema: &model.SchemaRef{
									Value: &model.Schema{Type: &model.Types{"object"}},
								},
							},
						},
					},
				},
			},
			Headers: model.Headers{
				"Bar": &model.HeaderRef{
					Value: &model.Header{
						Parameter: model.Parameter{
							Schema: &model.SchemaRef{
								Value: &model.Schema{Type: &model.Types{"boolean"}},
							},
						},
					},
				},
			},
		},
	}

	opts := Configuration{}
	schemas := GatherSchemas(spec, opts)

	// Should have 5 entries: schema, parameter, response, requestBody, header
	assert.Len(t, schemas, 5)

	// Verify contexts are all different
	contexts := make(map[SchemaContext]bool)
	for _, s := range schemas {
		contexts[s.Context] = true
	}
	assert.True(t, contexts[ContextComponentSchema])
	assert.True(t, contexts[ContextComponentParameter])
	assert.True(t, contexts[ContextComponentResponse])
	assert.True(t, contexts[ContextComponentRequestBody])
	assert.True(t, contexts[ContextComponentHeader])
}
