package codegen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/libopenapi"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

// externalRefFile is the file a relative reference to "./external.yaml" in a
// test spec resolves to when withExternalRef is used.
const externalRefFile = "external.yaml"

// testSpecOption adjusts how the test helpers load a document.
type testSpecOption func(*testSpecConfig)

type testSpecConfig struct {
	path         string
	externalBody []byte
	hasExternal  bool
}

// withIncludeOrigin records each element's source location, which route
// registration order is taken from. libopenapi always records it, so this
// only documents that a test depends on it.
func withIncludeOrigin() testSpecOption {
	return func(*testSpecConfig) {}
}

// withExternalRefsAllowed lets the loader follow file references, resolved
// relative to the working directory. libopenapi always follows them, so this
// only documents that a test depends on it.
func withExternalRefsAllowed() testSpecOption {
	return func(*testSpecConfig) {}
}

// withExternalRef loads data as if it were the file path, next to a file
// named externalRefFile holding body, so a reference to "./external.yaml"
// resolves to body.
func withExternalRef(path string, body []byte) testSpecOption {
	return func(c *testSpecConfig) {
		c.path = path
		c.externalBody = body
		c.hasExternal = true
	}
}

// loadTestSpecData loads an OpenAPI document from data. Callers that want a
// load failure as a returned error, rather than a failed test, use this.
func loadTestSpecData(data []byte, opts ...testSpecOption) (*model.Document, error) {
	var cfg testSpecConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if !cfg.hasExternal {
		return libopenapi.Engine{}.LoadData(data, "", engine.LoadOptions{})
	}

	dir, err := os.MkdirTemp("", "oapi-codegen-test")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if err := os.WriteFile(filepath.Join(dir, externalRefFile), cfg.externalBody, 0o600); err != nil {
		return nil, err
	}
	return libopenapi.Engine{}.LoadData(data, filepath.Join(dir, cfg.path), engine.LoadOptions{})
}

// loadTestSpec loads an OpenAPI document from data. A load error fails t.
func loadTestSpec(t *testing.T, data []byte, opts ...testSpecOption) *model.Document {
	t.Helper()
	spec, err := loadTestSpecData(data, opts...)
	require.NoError(t, err)
	return spec
}

// loadTestSpecFile loads an OpenAPI document from path, following file
// references.
func loadTestSpecFile(t *testing.T, path string) *model.Document {
	t.Helper()
	spec, err := libopenapi.Engine{}.Load(path, engine.LoadOptions{})
	require.NoError(t, err)
	return spec
}
