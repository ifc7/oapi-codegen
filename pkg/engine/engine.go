// Package engine defines how an OpenAPI library loads a document into the
// generator's model. Each library lives in its own subpackage.
package engine

import (
	"fmt"
	"os"
	"strings"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

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

// Names of the engines a configuration can choose.
const (
	Kin        = "kin"
	LibOpenAPI = "libopenapi"
)

// Default is the engine used when neither the configuration nor EnvVar
// names one.
const Default = Kin

// EnvVar names an engine that overrides the configuration's `engine`.
const EnvVar = "OAPI_CODEGEN_ENGINE"

// Names returns the engine names a configuration can choose.
func Names() []string {
	return []string{Kin, LibOpenAPI}
}

// Validate returns an error when name is neither empty nor an engine name.
func Validate(name string) error {
	if name == "" {
		return nil
	}
	for _, known := range Names() {
		if name == known {
			return nil
		}
	}
	return fmt.Errorf("unknown engine %q: must be one of %s", name, strings.Join(Names(), ", "))
}

// Resolve returns the engine to use for a configuration naming configured,
// which may be empty. A non-empty EnvVar takes precedence over configured,
// and Default applies when neither names one.
func Resolve(configured string) (string, error) {
	if env := os.Getenv(EnvVar); env != "" {
		if err := Validate(env); err != nil {
			return "", fmt.Errorf("%s: %w", EnvVar, err)
		}
		return env, nil
	}
	if err := Validate(configured); err != nil {
		return "", err
	}
	if configured == "" {
		return Default, nil
	}
	return configured, nil
}
