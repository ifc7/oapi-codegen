// Package engines picks the engine a configuration names. It is separate from
// package engine because every engine imports that package.
package engines

import (
	"fmt"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/kin"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

// New returns the engine called name.
func New(name string) (engine.Engine, error) {
	switch name {
	case engine.Kin:
		return kin.Engine{}, nil
	case engine.LibOpenAPI:
		return nil, fmt.Errorf("the %s engine is not available yet; use %s", engine.LibOpenAPI, engine.Kin)
	default:
		return nil, engine.Validate(name)
	}
}

// FromConfig returns the engine for a configuration naming configured, which
// may be empty, after applying engine.EnvVar and engine.Default.
func FromConfig(configured string) (engine.Engine, error) {
	name, err := engine.Resolve(configured)
	if err != nil {
		return nil, err
	}
	return New(name)
}

// Load loads the document at path with the engine FromConfig returns for
// configured.
func Load(configured, path string, opts engine.LoadOptions) (*model.Document, error) {
	e, err := FromConfig(configured)
	if err != nil {
		return nil, err
	}
	return e.Load(path, opts)
}
