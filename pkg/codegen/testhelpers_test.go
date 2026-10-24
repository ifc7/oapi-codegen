package codegen

import (
	"net/url"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

// testSpecOption adjusts the kin-openapi loader the test helpers use.
// The default matches openapi3.NewLoader().LoadFromData: external refs
// are refused and source locations are not recorded.
type testSpecOption func(*testSpecConfig)

type testSpecConfig struct {
	includeOrigin bool
	allowExternal bool
	path          string
	externalBody  []byte
	hasExternal   bool
}

// withIncludeOrigin records each element's source location, which route
// registration order is taken from.
func withIncludeOrigin() testSpecOption {
	return func(c *testSpecConfig) { c.includeOrigin = true }
}

// withExternalRefsAllowed lets the loader follow file references, resolved
// relative to the working directory, as LoadFromData does.
func withExternalRefsAllowed() testSpecOption {
	return func(c *testSpecConfig) { c.allowExternal = true }
}

// withExternalRef serves body for every external reference and loads data
// as if it came from path, so relative references resolve against that name.
func withExternalRef(path string, body []byte) testSpecOption {
	return func(c *testSpecConfig) {
		c.allowExternal = true
		c.path = path
		c.externalBody = body
		c.hasExternal = true
	}
}

func newTestLoader(opts ...testSpecOption) (*openapi3.Loader, testSpecConfig) {
	var cfg testSpecConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = cfg.allowExternal
	loader.IncludeOrigin = cfg.includeOrigin
	if cfg.hasExternal {
		body := cfg.externalBody
		loader.ReadFromURIFunc = func(_ *openapi3.Loader, _ *url.URL) ([]byte, error) {
			return body, nil
		}
	}
	return loader, cfg
}

// loadTestSpecData loads an OpenAPI document from data. Callers that want a
// load failure as a returned error, rather than a failed test, use this.
func loadTestSpecData(data []byte, opts ...testSpecOption) (*openapi3.T, error) {
	loader, cfg := newTestLoader(opts...)
	if cfg.path != "" || cfg.hasExternal {
		return loader.LoadFromDataWithPath(data, &url.URL{Path: cfg.path})
	}
	return loader.LoadFromData(data)
}

// loadTestSpec loads an OpenAPI document from data. A load error fails t.
func loadTestSpec(t *testing.T, data []byte, opts ...testSpecOption) *openapi3.T {
	t.Helper()
	spec, err := loadTestSpecData(data, opts...)
	require.NoError(t, err)
	return spec
}

// loadTestSpecFile loads an OpenAPI document from path, following file
// references.
func loadTestSpecFile(t *testing.T, path string) *openapi3.T {
	t.Helper()
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	spec, err := loader.LoadFromFile(path)
	require.NoError(t, err)
	return spec
}
