package engines

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/kin"
)

func TestNew(t *testing.T) {
	e, err := New(engine.Kin)
	require.NoError(t, err)
	assert.Equal(t, kin.Engine{}, e)

	_, err = New(engine.LibOpenAPI)
	assert.EqualError(t, err, "the libopenapi engine is not available yet; use kin")

	_, err = New("swagger")
	assert.EqualError(t, err, `unknown engine "swagger": must be one of kin, libopenapi`)
}

func TestFromConfig(t *testing.T) {
	t.Setenv(engine.EnvVar, "")
	e, err := FromConfig("")
	require.NoError(t, err)
	assert.Equal(t, kin.Engine{}, e)

	t.Setenv(engine.EnvVar, engine.LibOpenAPI)
	_, err = FromConfig(engine.Kin)
	assert.EqualError(t, err, "the libopenapi engine is not available yet; use kin")
}

func TestLoad(t *testing.T) {
	t.Setenv(engine.EnvVar, "")
	doc, err := Load("", "../../codegen/test_specs/bodyless-response-default.yaml", engine.LoadOptions{})
	require.NoError(t, err)
	assert.NotNil(t, doc.Paths)
}
