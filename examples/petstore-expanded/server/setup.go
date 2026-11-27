package server

import (
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"

	"github.com/ifc7/oapi-codegen/examples/internal/requestvalidator"
	"github.com/ifc7/oapi-codegen/examples/petstore-expanded/api"
)

// NewServer creates a fully configured *http.Server with the petstore
// handler and OpenAPI validation middleware. The caller should set Addr before
// calling ListenAndServe, or provide a net.Listener and call Serve.
func NewServer() (*http.Server, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	validate, err := requestvalidator.New(filepath.Join(filepath.Dir(thisFile), "..", "petstore-expanded.yaml"), nil)
	if err != nil {
		return nil, fmt.Errorf("error loading swagger spec: %w", err)
	}

	petStore := NewPetStore()

	h := api.Handler(petStore)

	return &http.Server{Handler: validate(h)}, nil
}
