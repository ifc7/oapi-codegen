package libopenapi

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/enginetest"
)

func TestLoadEveryFixture(t *testing.T) {
	for _, fx := range enginetest.GenerateFixtures(t) {
		t.Run(fx.Name, func(t *testing.T) {
			doc, err := Engine{}.Load(fx.Spec, fx.Opts)
			require.NoError(t, err)
			assert.NotNil(t, doc)
		})
	}
}

func TestLoadFileURLMatchesPath(t *testing.T) {
	spec := filepath.Join("..", "..", "..", "examples", "petstore-expanded", "petstore-expanded.yaml")
	fromPath, err := Engine{}.Load(spec, engine.LoadOptions{})
	require.NoError(t, err)

	abs, err := filepath.Abs(spec)
	require.NoError(t, err)
	fromURL, err := Engine{}.Load((&url.URL{Scheme: "file", Path: abs}).String(), engine.LoadOptions{})
	require.NoError(t, err)

	assert.Equal(t, fromPath.OpenAPI, fromURL.OpenAPI)
	assert.Equal(t, fromPath.Info.Title, fromURL.Info.Title)
	assert.Equal(t, fromPath.Info.Version, fromURL.Info.Version)
}

func TestOverlayStrict(t *testing.T) {
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.yaml")
	overlay := filepath.Join(dir, "overlay.yaml")
	require.NoError(t, os.WriteFile(spec, []byte(`
openapi: 3.0.3
info: {title: t, version: "1"}
paths: {}
`), 0o644))
	require.NoError(t, os.WriteFile(overlay, []byte(`
overlay: 1.0.0
info: {title: o, version: "1.0.0"}
actions:
  - target: $.does.not.exist
    update:
      description: nowhere
`), 0o644))

	_, err := Engine{}.Load(spec, engine.LoadOptions{OverlayPath: overlay, OverlayStrict: true})
	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to apply Overlay")
	assert.ErrorContains(t, err, "$.does.not.exist")

	doc, err := Engine{}.Load(spec, engine.LoadOptions{OverlayPath: overlay, OverlayStrict: false})
	require.NoError(t, err)
	assert.Equal(t, "3.0.3", doc.OpenAPI)
	assert.Equal(t, "t", doc.Info.Title)
}

func TestOverlayExample(t *testing.T) {
	var found bool
	for _, fx := range enginetest.GenerateFixtures(t) {
		if !strings.HasSuffix(fx.Name, filepath.Join("examples", "overlay", "api", "api.yaml")) {
			continue
		}
		found = true
		doc, err := Engine{}.Load(fx.Spec, fx.Opts)
		require.NoError(t, err)
		assert.NotEmpty(t, doc.Info.Title)
	}
	require.True(t, found, "overlay example fixture was not found")
}
