package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/getkin/kin-openapi/openapi3"
	middleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/oapi-codegen/oapi-codegen/v2/examples/petstore-expanded/api"
)

// NewServer creates a fully configured *http.Server with the petstore
// handler and OpenAPI validation middleware. The caller should set Addr before
// calling ListenAndServe, or provide a net.Listener and call Serve.
func NewServer() (*http.Server, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	swagger, err := loader.LoadFromFile(filepath.Join(filepath.Dir(thisFile), "..", "petstore-expanded.yaml"))
	if err != nil {
		return nil, fmt.Errorf("error loading swagger spec: %w", err)
	}

	swagger.Servers = nil

	petStore := NewPetStore()

	h := api.Handler(petStore)

	return &http.Server{Handler: middleware.OapiRequestValidator(swagger)(h)}, nil
}
