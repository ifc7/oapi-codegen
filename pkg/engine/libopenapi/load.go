// Package libopenapi loads OpenAPI documents with pb33f/libopenapi and
// converts them into the generator's model.
package libopenapi

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	openapi "github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/index"
	"github.com/speakeasy-api/openapi/overlay/loader"
	"gopkg.in/yaml.v3"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

// Engine loads documents with libopenapi.
type Engine struct{}

var _ engine.Engine = Engine{}

// Load loads the document at path, a file path or an http(s) or file URL,
// applies any Overlay, and converts it.
func (Engine) Load(path string, opts engine.LoadOptions) (*model.Document, error) {
	raw, specPath, remote, err := readSpec(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI specification: %w", err)
	}
	raw, err = applyOverlay(raw, opts)
	if err != nil {
		return nil, err
	}
	// libopenapi will not build a document that has no openapi version.
	// Some specs this repository generates from omit it (or misspell it).
	// Parse those as 3.0.3, then keep the version the file actually declared.
	raw, declared := withOpenAPIVersion(raw)

	doc, err := openapi.NewDocumentWithConfiguration(raw, documentConfig(specPath, remote))
	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI specification: %w", err)
	}
	built, err := doc.BuildV3Model()
	if built == nil {
		if err == nil {
			err = errors.New("no document was built")
		}
		return nil, fmt.Errorf("failed to load OpenAPI specification: %w", err)
	}
	// A circular reference is reported alongside a built model. Other build
	// errors mean the document was not read the way the generator needs.
	if err != nil && !onlyCircular(err) {
		return nil, fmt.Errorf("failed to load OpenAPI specification: %w", err)
	}
	out := Convert(&built.Model)
	if declared != nil {
		out.OpenAPI = *declared
	}
	return out, nil
}

func documentConfig(specPath string, remote *url.URL) *datamodel.DocumentConfiguration {
	cfg := datamodel.NewDocumentConfiguration()
	cfg.Logger = slog.New(slog.DiscardHandler)
	cfg.AllowFileReferences = true
	// A file spec can still $ref an http(s) document.
	cfg.AllowRemoteReferences = true
	// Recursive schemas are valid. Leave $ref siblings as authored so a later
	// conversion sees the same shape kin-openapi does.
	cfg.IgnorePolymorphicCircularReferences = true
	cfg.IgnoreArrayCircularReferences = true
	cfg.TransformSiblingRefs = false
	cfg.MergeReferencedProperties = false
	if remote != nil {
		cfg.AllowRemoteReferences = true
		cfg.BaseURL = remote
		return cfg
	}
	cfg.BasePath = filepath.Dir(specPath)
	cfg.SpecFilePath = specPath
	return cfg
}

func readSpec(path string) (raw []byte, specPath string, remote *url.URL, err error) {
	u, parseErr := url.Parse(path)
	if parseErr == nil && u.Scheme == "file" {
		path = fileURLPath(u)
		u = nil
	}
	if parseErr == nil && u != nil && u.Host != "" && (u.Scheme == "http" || u.Scheme == "https") {
		raw, err = readHTTP(u)
		return raw, path, u, err
	}
	raw, err = os.ReadFile(path)
	if err != nil {
		return nil, "", nil, err
	}
	specPath, err = filepath.Abs(path)
	return raw, specPath, nil, err
}

func fileURLPath(u *url.URL) string {
	path := u.Path
	if u.Host != "" && u.Host != "localhost" {
		path = "//" + u.Host + path
	}
	return path
}

func readHTTP(u *url.URL) ([]byte, error) {
	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			slog.Error("failed to close HTTP response body", "err", err)
		}
	}(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", u, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func applyOverlay(spec []byte, opts engine.LoadOptions) ([]byte, error) {
	if opts.OverlayPath == "" {
		return spec, nil
	}
	overlayBytes, err := os.ReadFile(opts.OverlayPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load Overlay from %q: %w", opts.OverlayPath, err)
	}
	applied, err := applyLibopenapiOverlay(spec, overlayBytes, opts)
	if err == nil {
		return applied, nil
	}
	// libopenapi rejects an update whose JSONPath matches nodes of more than
	// one YAML kind. The repository's overlay example does that, and the kin
	// engine applies it with speakeasy, so fall back to that implementation.
	if !strings.Contains(err.Error(), "mixed target types") {
		return nil, err
	}
	return applySpeakeasyOverlay(spec, opts)
}

func applyLibopenapiOverlay(spec, overlayBytes []byte, opts engine.LoadOptions) ([]byte, error) {
	overlayDoc, err := openapi.NewOverlayDocument(overlayBytes)
	if err != nil {
		return nil, fmt.Errorf("the Overlay in %q was not valid: %w", opts.OverlayPath, err)
	}
	result, err := openapi.ApplyOverlayToSpecBytes(spec, overlayDoc)
	if err != nil {
		return nil, fmt.Errorf("failed to apply Overlay %q: %w", opts.OverlayPath, err)
	}
	if opts.OverlayStrict && len(result.Warnings) > 0 {
		msgs := make([]string, len(result.Warnings))
		for i, warning := range result.Warnings {
			msgs[i] = warning.String()
		}
		return nil, fmt.Errorf("failed to apply Overlay %q:\n- %s", opts.OverlayPath, strings.Join(msgs, "\n- "))
	}
	return result.Bytes, nil
}

func applySpeakeasyOverlay(spec []byte, opts engine.LoadOptions) ([]byte, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(spec, &node); err != nil {
		return nil, fmt.Errorf("failed to parse specification for Overlay %q: %w", opts.OverlayPath, err)
	}
	overlayDoc, err := loader.LoadOverlay(opts.OverlayPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load Overlay from %q: %w", opts.OverlayPath, err)
	}
	if err := overlayDoc.Validate(); err != nil {
		return nil, fmt.Errorf("the Overlay in %q was not valid: %w", opts.OverlayPath, err)
	}
	if opts.OverlayStrict {
		vs, err := overlayDoc.ApplyToStrict(&node)
		if err != nil {
			return nil, fmt.Errorf("failed to apply Overlay %q: %w\n- %s", opts.OverlayPath, err, strings.Join(vs, "\n- "))
		}
	} else if err := overlayDoc.ApplyTo(&node); err != nil {
		return nil, fmt.Errorf("failed to apply Overlay %q: %w", opts.OverlayPath, err)
	}
	out, err := yaml.Marshal(&node)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize Overlay'd specification %q: %w", opts.OverlayPath, err)
	}
	return out, nil
}

// withOpenAPIVersion returns the bytes to parse and, when the file did not
// declare one, the version string to keep (the empty string). The second
// return is nil when the file already declared openapi.
func withOpenAPIVersion(raw []byte) ([]byte, *string) {
	var root map[string]any
	if err := yaml.Unmarshal(raw, &root); err == nil {
		if version, ok := root["openapi"].(string); ok && version != "" {
			return raw, nil
		}
	}
	declared := ""
	return append([]byte("openapi: 3.0.3\n"), raw...), &declared
}

func onlyCircular(err error) bool {
	errs := flatten(err)
	if len(errs) == 0 {
		return false
	}
	for _, err := range errs {
		var refErr *index.ResolvingError
		if !errors.As(err, &refErr) || refErr.CircularReference == nil {
			return false
		}
	}
	return true
}

func flatten(err error) []error {
	if err == nil {
		return nil
	}
	type joined interface{ Unwrap() []error }
	if u, ok := err.(joined); ok {
		var out []error
		for _, inner := range u.Unwrap() {
			out = append(out, flatten(inner)...)
		}
		return out
	}
	if inner := errors.Unwrap(err); inner != nil {
		return flatten(inner)
	}
	return []error{err}
}
