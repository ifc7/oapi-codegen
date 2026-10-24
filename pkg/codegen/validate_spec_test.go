package codegen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

// schemaWithExtension builds a minimal *model.Document whose single schema
// component carries the supplied extension map.
func schemaWithExtension(ext map[string]any) *model.Document {
	s := model.NewSchema()
	s.Type = &model.Types{"string"}
	s.Extensions = ext
	ref := &model.SchemaRef{Value: s}
	return &model.Document{
		OpenAPI: "3.0.3",
		Info:    &model.Info{Title: "t", Version: "0"},
		Components: &model.Components{
			Schemas: model.Schemas{"S": ref},
		},
	}
}

func TestValidateSpec_XGoTypeImport_Valid(t *testing.T) {
	cases := []struct {
		name string
		ext  map[string]any
	}{
		{
			name: "identifier alias",
			ext: map[string]any{
				"x-go-type": "mypkg.MyType",
				"x-go-type-import": map[string]any{
					"name": "mypkg",
					"path": "example.com/mymodule/mypkg",
				},
			},
		},
		{
			name: "blank alias",
			ext: map[string]any{
				"x-go-type": "something.T",
				"x-go-type-import": map[string]any{
					"name": "_",
					"path": "example.com/mod",
				},
			},
		},
		{
			name: "dot alias",
			ext: map[string]any{
				"x-go-type": "T",
				"x-go-type-import": map[string]any{
					"name": ".",
					"path": "example.com/mod",
				},
			},
		},
		{
			name: "no alias",
			ext: map[string]any{
				"x-go-type": "pkg.T",
				"x-go-type-import": map[string]any{
					"path": "example.com/mod",
				},
			},
		},
		{
			name: "empty alias",
			ext: map[string]any{
				"x-go-type": "pkg.T",
				"x-go-type-import": map[string]any{
					"name": "",
					"path": "example.com/mod",
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSpec(schemaWithExtension(tc.ext))
			assert.NoError(t, err)
		})
	}
}

func TestValidateSpec_XGoTypeImport_NameInjection(t *testing.T) {
	// GHSA-9c2f-gr95-7wqw: x-go-type-import.name is emitted unquoted into the
	// import block; a newline closes the block and allows arbitrary code injection.
	injected := "_ \"unsafe\"\n)\nfunc init() { panic(\"injected\") }\nvar (\n_ ="
	ext := map[string]any{
		"x-go-type": "os.FileInfo",
		"x-go-type-import": map[string]any{
			"name": injected,
			"path": "ignored",
		},
	}
	err := ValidateSpec(schemaWithExtension(ext))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "x-go-type-import name")
}

func TestValidateSpec_XGoTypeImport_NameWithNewline(t *testing.T) {
	ext := map[string]any{
		"x-go-type": "pkg.T",
		"x-go-type-import": map[string]any{
			"name": "alias\ninjected",
			"path": "example.com/mod",
		},
	}
	err := ValidateSpec(schemaWithExtension(ext))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "x-go-type-import name")
}

func TestValidateSpec_XGoTypeImport_NameWithBacktick(t *testing.T) {
	ext := map[string]any{
		"x-go-type": "pkg.T",
		"x-go-type-import": map[string]any{
			"name": "alias`bad",
			"path": "example.com/mod",
		},
	}
	err := ValidateSpec(schemaWithExtension(ext))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "x-go-type-import name")
}

func TestValidateSpec_XGoTypeImport_PathWithControlChar(t *testing.T) {
	ext := map[string]any{
		"x-go-type": "pkg.T",
		"x-go-type-import": map[string]any{
			"name": "mypkg",
			"path": "example.com/mod\x00injected",
		},
	}
	err := ValidateSpec(schemaWithExtension(ext))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "x-go-type-import path")
}

func TestValidateSpec_XGoTypeImport_CaseInsensitiveKeys(t *testing.T) {
	// Matches the case-insensitive key handling in ParseGoImportExtension.
	ext := map[string]any{
		"x-go-type": "pkg.T",
		"x-go-type-import": map[string]any{
			"Name": "mypkg",
			"Path": "example.com/mod",
		},
	}
	err := ValidateSpec(schemaWithExtension(ext))
	assert.NoError(t, err)
}

// Regression: pre-existing extensions must still be validated.
func TestValidateSpec_XGoName_StillValidated(t *testing.T) {
	ext := map[string]any{
		"x-go-name": "bad\nname",
	}
	err := ValidateSpec(schemaWithExtension(ext))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "x-go-name")
}

func TestValidateSpec_XGoType_StillValidated(t *testing.T) {
	ext := map[string]any{
		"x-go-type": "pkg.T\nfunc init() {}",
	}
	err := ValidateSpec(schemaWithExtension(ext))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "x-go-type")
}

func TestValidateSpec_XGoTypeName_StillValidated(t *testing.T) {
	ext := map[string]any{
		"x-go-type-name": "Bad\nName",
	}
	err := ValidateSpec(schemaWithExtension(ext))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "x-go-type-name")
}

func TestValidateSpec_XOapiCodegenExtraTags_StillValidated(t *testing.T) {
	ext := map[string]any{
		"x-oapi-codegen-extra-tags": map[string]any{
			"bad\ntag": "value",
		},
	}
	err := ValidateSpec(schemaWithExtension(ext))
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "x-oapi-codegen-extra-tags"))
}

func TestValidateOpenAPIVersion(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantErr string
	}{
		{
			name:    "OpenAPI 2.0",
			doc:     "swagger: \"2.0\"\ninfo: {title: t, version: \"1\"}\npaths: {}\n",
			wantErr: "the document is OpenAPI 2.0 (Swagger), which oapi-codegen doesn't support: convert it to OpenAPI 3, for instance with https://converter.swagger.io/",
		},
		{
			name:    "another version",
			doc:     "openapi: 4.0.0\ninfo: {title: t, version: \"1\"}\npaths: {}\n",
			wantErr: "the document is OpenAPI 4.0.0, which oapi-codegen doesn't support: it supports OpenAPI 3.0 and 3.1",
		},
		{name: "OpenAPI 3.0", doc: "openapi: 3.0.3\ninfo: {title: t, version: \"1\"}\npaths: {}\n"},
		{name: "OpenAPI 3.1", doc: "openapi: 3.1.0\ninfo: {title: t, version: \"1\"}\npaths: {}\n"},
		{name: "shared definitions with no version", doc: "components:\n  schemas:\n    Pet: {type: object}\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := loadTestSpec(t, []byte(tt.doc))
			err := ValidateOpenAPIVersion(spec)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

// Generate rejects an OpenAPI 2.0 document up front, rather than failing on
// whichever part of it generation first reaches.
func TestGenerateRejectsOpenAPI2(t *testing.T) {
	const doc = `
swagger: "2.0"
info: {title: Pets, version: "1.0"}
paths:
  /pets/{id}:
    get:
      operationId: getPet
      parameters:
        - {name: id, in: path, required: true, type: integer}
      responses:
        200:
          description: ok
`
	spec := loadTestSpec(t, []byte(doc))
	_, err := Generate(spec, Configuration{PackageName: "api", Generate: GenerateOptions{Server: true, Models: true}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OpenAPI 2.0 (Swagger)")
}
