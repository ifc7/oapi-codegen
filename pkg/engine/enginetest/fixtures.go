// Package enginetest finds the documents the repository generates from, so
// every engine can be checked against the same set.
package enginetest

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ifc7/oapi-codegen/v2/pkg/engine"
)

// Fixture is one document the repository generates code from: the spec named
// by a go:generate directive, with the Overlay its configuration applies.
type Fixture struct {
	// Name is the spec path relative to the repository root.
	Name string
	// Spec is the absolute path of the OpenAPI document.
	Spec string
	// Opts carries the Overlay the configuration applies, if any.
	Opts engine.LoadOptions
}

// GenerateFixtures finds every oapi-codegen go:generate directive under the
// repository's internal/test and examples modules.
func GenerateFixtures(t *testing.T) []Fixture {
	t.Helper()
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}

	var fixtures []Fixture
	for _, dir := range []string{"internal/test", "examples"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			scanner := bufio.NewScanner(bytes.NewReader(data))
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "//go:generate ") || !strings.Contains(line, "oapi-codegen") {
					continue
				}
				fields := strings.Fields(line)
				if spec := fields[len(fields)-1]; strings.HasPrefix(spec, "http://") || strings.HasPrefix(spec, "https://") {
					// Fetched specs are too large to check in and stay out of unit tests.
					continue
				}
				fx, err := parseDirective(filepath.Dir(path), fields)
				if err != nil {
					return fmt.Errorf("%s: %w", path, err)
				}
				rel, err := filepath.Rel(root, fx.Spec)
				if err != nil {
					return err
				}
				fx.Name = rel
				fixtures = append(fixtures, fx)
			}
			return scanner.Err()
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(fixtures) == 0 {
		t.Fatal("found no oapi-codegen go:generate directives")
	}
	return fixtures
}

func parseDirective(dir string, fields []string) (Fixture, error) {
	spec := fields[len(fields)-1]
	var config string
	for i, field := range fields {
		switch {
		case field == "-config" || field == "--config":
			if i+1 < len(fields) {
				config = fields[i+1]
			}
		case strings.HasPrefix(field, "-config="), strings.HasPrefix(field, "--config="):
			_, config, _ = strings.Cut(field, "=")
		}
	}
	fx := Fixture{
		Spec: filepath.Join(dir, spec),
		Opts: engine.LoadOptions{OverlayStrict: true},
	}
	if config == "" {
		return fx, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, config))
	if err != nil {
		return fx, err
	}
	var cfg struct {
		OutputOptions struct {
			Overlay struct {
				Path   string `yaml:"path"`
				Strict *bool  `yaml:"strict"`
			} `yaml:"overlay"`
		} `yaml:"output-options"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fx, fmt.Errorf("reading %s: %w", config, err)
	}
	if overlay := cfg.OutputOptions.Overlay; overlay.Path != "" {
		fx.Opts.OverlayPath = filepath.Join(dir, overlay.Path)
		if overlay.Strict != nil {
			fx.Opts.OverlayStrict = *overlay.Strict
		}
	}
	return fx, nil
}

// repoRoot returns the nearest directory above the working directory that
// holds both fixture trees, so tests in any package can use the fixtures.
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		_, errTest := os.Stat(filepath.Join(dir, "internal", "test"))
		_, errExamples := os.Stat(filepath.Join(dir, "examples"))
		if errTest == nil && errExamples == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no directory above the working directory holds internal/test and examples")
		}
		dir = parent
	}
}
