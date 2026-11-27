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
	"regexp"
	"strings"

	openapi "github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/index"
	"gopkg.in/yaml.v3"

	"github.com/ifc7/oapi-codegen/v2/pkg/engine"
	"github.com/ifc7/oapi-codegen/v2/pkg/model"
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
	return load(raw, specPath, remote, opts)
}

// LoadData loads the document in raw as if it were the file at specPath, so
// relative references resolve against that path's directory. specPath may be
// empty, in which case they resolve against the working directory.
func (Engine) LoadData(raw []byte, specPath string, opts engine.LoadOptions) (*model.Document, error) {
	if specPath != "" {
		abs, err := filepath.Abs(specPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load OpenAPI specification: %w", err)
		}
		specPath = abs
	}
	return load(raw, specPath, nil, opts)
}

func load(raw []byte, specPath string, remote *url.URL, opts engine.LoadOptions) (*model.Document, error) {
	raw, err := applyOverlay(raw, opts)
	if err != nil {
		return nil, err
	}
	// libopenapi builds OpenAPI 3 only. A Swagger document comes back as an
	// empty one that records the version it declared, so the generator can say
	// that OpenAPI 2.0 is not supported (see codegen.ValidateOpenAPIVersion).
	if swagger, ok := declaredSwagger(raw); ok {
		return &model.Document{Extensions: map[string]any{"swagger": swagger}}, nil
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
	if specPath == "" {
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
	return applyLibopenapiOverlay(spec, overlayBytes, opts)
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

// declaredSwagger returns the version of a document that declares `swagger`
// and no `openapi`.
func declaredSwagger(raw []byte) (any, bool) {
	var root map[string]any
	if err := yaml.Unmarshal(raw, &root); err != nil {
		return nil, false
	}
	swagger, ok := root["swagger"]
	if !ok {
		return nil, false
	}
	if _, hasOpenAPI := root["openapi"]; hasOpenAPI {
		return nil, false
	}
	return swagger, true
}

// documentStart matches what can precede the content of a YAML document:
// blank lines, comments, and an optional `---` marker.
var documentStart = regexp.MustCompile(`\A(?:[ \t]*(?:#[^\n]*)?\n)*(?:---[^\n]*\n)?`)

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
	// The version goes after any `---` marker, which would otherwise start a
	// second document and leave the version alone in the first.
	at := len(documentStart.Find(raw))
	out := make([]byte, 0, len(raw)+len("openapi: 3.0.3\n"))
	out = append(out, raw[:at]...)
	out = append(out, "openapi: 3.0.3\n"...)
	out = append(out, raw[at:]...)
	return out, &declared
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
