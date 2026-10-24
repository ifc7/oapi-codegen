package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterOperationsByTag(t *testing.T) {
	packageName := "testswagger"
	t.Run("include tags", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Generate: GenerateOptions{
				Server: true,
				Client: true,
				Models: true,
			},
			OutputOptions: OutputOptions{
				IncludeTags: []string{"hippo", "giraffe", "cat"},
			},
		}

		// Get a spec from the test definition in this file:
		swagger := loadTestSpec(t, []byte(testOpenAPIDefinition), withExternalRefsAllowed())

		// Run our code generation:
		code, err := Generate(swagger, opts)
		assert.NoError(t, err)
		assert.NotEmpty(t, code)
		assert.NotContains(t, code, `"/test/{name}"`)
		assert.Contains(t, code, `"/cat"`)
	})

	t.Run("exclude tags", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Generate: GenerateOptions{
				Server: true,
				Client: true,
				Models: true,
			},
			OutputOptions: OutputOptions{
				ExcludeTags: []string{"hippo", "giraffe", "cat"},
			},
		}

		// Get a spec from the test definition in this file:
		swagger := loadTestSpec(t, []byte(testOpenAPIDefinition), withExternalRefsAllowed())

		// Run our code generation:
		code, err := Generate(swagger, opts)
		assert.NoError(t, err)
		assert.NotEmpty(t, code)
		assert.Contains(t, code, `"/test/{name}"`)
		assert.NotContains(t, code, `"/cat"`)
	})
}

func TestFilterOperationsByOperationID(t *testing.T) {
	packageName := "testswagger"
	t.Run("include operation ids", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Generate: GenerateOptions{
				Server: true,
				Client: true,
				Models: true,
			},
			OutputOptions: OutputOptions{
				IncludeOperationIDs: []string{"getCatStatus"},
			},
		}

		// Get a spec from the test definition in this file:
		swagger := loadTestSpec(t, []byte(testOpenAPIDefinition), withExternalRefsAllowed())

		// Run our code generation:
		code, err := Generate(swagger, opts)
		assert.NoError(t, err)
		assert.NotEmpty(t, code)
		assert.NotContains(t, code, `"/test/{name}"`)
		assert.Contains(t, code, `"/cat"`)
	})

	t.Run("exclude operation ids", func(t *testing.T) {
		opts := Configuration{
			PackageName: packageName,
			Generate: GenerateOptions{
				Server: true,
				Client: true,
				Models: true,
			},
			OutputOptions: OutputOptions{
				ExcludeOperationIDs: []string{"getCatStatus"},
			},
		}

		// Get a spec from the test definition in this file:
		swagger := loadTestSpec(t, []byte(testOpenAPIDefinition), withExternalRefsAllowed())

		// Run our code generation:
		code, err := Generate(swagger, opts)
		assert.NoError(t, err)
		assert.NotEmpty(t, code)
		assert.Contains(t, code, `"/test/{name}"`)
		assert.NotContains(t, code, `"/cat"`)
	})
}
