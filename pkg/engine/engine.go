// Package engine defines how an OpenAPI library loads a document into the
// generator's model. Each library lives in its own subpackage.
package engine

import "github.com/oapi-codegen/oapi-codegen/v2/pkg/model"

// Engine loads the OpenAPI document at path, a file path or an http(s) URL.
type Engine interface {
	Load(path string, opts LoadOptions) (*model.Document, error)
}

// LoadOptions controls how a document is loaded.
type LoadOptions struct {
	// OverlayPath is an OpenAPI Overlay applied to the document before it is
	// read. Ignored when empty.
	OverlayPath string
	// OverlayStrict fails the load when an Overlay action has no effect.
	OverlayStrict bool
}
