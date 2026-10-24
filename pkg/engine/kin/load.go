// Package kin loads OpenAPI documents with kin-openapi and converts them
// into the generator's model.
package kin

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/speakeasy-api/openapi/overlay/loader"
	"gopkg.in/yaml.v3"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

// Engine loads documents with kin-openapi.
type Engine struct{}

var _ engine.Engine = Engine{}

// Load loads the document at path, applies any Overlay, and converts it.
func (Engine) Load(path string, opts engine.LoadOptions) (*model.Document, error) {
	spec, err := LoadSpecWithOverlay(path, opts)
	if err != nil {
		return nil, err
	}
	return Convert(spec), nil
}

func newLoader() *openapi3.Loader {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true
	// Record each element's source location so route registration can be
	// emitted in the order paths are declared in the spec (issue #1887).
	loader.IncludeOrigin = true
	return loader
}

// LoadSpec loads the document at filePath, a file path or an http(s) URL.
func LoadSpec(filePath string) (*openapi3.T, error) {
	loader := newLoader()

	u, err := url.Parse(filePath)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return loader.LoadFromURI(u)
	}
	return loader.LoadFromFile(filePath)
}

// LoadSpecWithOverlay loads the document at filePath and applies the Overlay
// named in opts, if any.
func LoadSpecWithOverlay(filePath string, opts engine.LoadOptions) (*openapi3.T, error) {
	spec, err := LoadSpec(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI specification: %w", err)
	}

	if opts.OverlayPath == "" {
		return spec, nil
	}

	// parse out the yaml.Node, which is required by the overlay library
	buf := &bytes.Buffer{}
	enc := yaml.NewEncoder(buf)
	// set to 2 to work around https://github.com/yaml/go-yaml/issues/76
	enc.SetIndent(2)
	err = enc.Encode(spec)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal spec from %#v as YAML: %w", filePath, err)
	}

	var node yaml.Node
	err = yaml.NewDecoder(buf).Decode(&node)
	if err != nil {
		return nil, fmt.Errorf("failed to parse spec from %#v: %w", filePath, err)
	}

	overlay, err := loader.LoadOverlay(opts.OverlayPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load Overlay from %#v: %v", opts.OverlayPath, err)
	}

	err = overlay.Validate()
	if err != nil {
		return nil, fmt.Errorf("the Overlay in %#v was not valid: %v", opts.OverlayPath, err)
	}

	if opts.OverlayStrict {
		vs, err := overlay.ApplyToStrict(&node)
		if err != nil {
			return nil, fmt.Errorf("failed to apply Overlay %#v to specification %#v: %v\nAdditionally, the following validation errors were found:\n- %s", opts.OverlayPath, filePath, err, strings.Join(vs, "\n- "))
		}
	} else {
		err = overlay.ApplyTo(&node)
		if err != nil {
			return nil, fmt.Errorf("failed to apply Overlay %#v to specification %#v: %v", opts.OverlayPath, filePath, err)
		}
	}

	b, err := yaml.Marshal(&node)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize Overlay'd specification %#v: %v", opts.OverlayPath, err)
	}

	spec, err = newLoader().LoadFromDataWithPath(b, &url.URL{
		Path: filepath.ToSlash(filePath),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to serialize Overlay'd specification %#v: %v", opts.OverlayPath, err)
	}

	return spec, nil
}
