package util

import (
	"github.com/getkin/kin-openapi/openapi3"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/kin"
)

func LoadSwagger(filePath string) (swagger *openapi3.T, err error) {
	return kin.LoadSpec(filePath)
}

type LoadSwaggerWithOverlayOpts struct {
	Path   string
	Strict bool
}

func LoadSwaggerWithOverlay(filePath string, opts LoadSwaggerWithOverlayOpts) (swagger *openapi3.T, err error) {
	return kin.LoadSpecWithOverlay(filePath, engine.LoadOptions{
		OverlayPath:   opts.Path,
		OverlayStrict: opts.Strict,
	})
}
