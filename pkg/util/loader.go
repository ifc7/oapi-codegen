package util

import (
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/kin"
)

// Deprecated: LoadSwagger returns a kin-openapi document. Load a
// *model.Document with [kin.Engine.Load] instead.
func LoadSwagger(filePath string) (swagger *openapi3.T, err error) {
	return kin.LoadSpec(filePath)
}

type LoadSwaggerWithOverlayOpts struct {
	Path   string
	Strict bool
}

// Deprecated: LoadSwaggerWithOverlay returns a kin-openapi document. Load a
// *model.Document with [kin.Engine.Load] instead.
func LoadSwaggerWithOverlay(filePath string, opts LoadSwaggerWithOverlayOpts) (swagger *openapi3.T, err error) {
	return kin.LoadSpecWithOverlay(filePath, engine.LoadOptions{
		OverlayPath:   opts.Path,
		OverlayStrict: opts.Strict,
	})
}
