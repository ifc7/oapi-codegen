package codegen

import (
	"os/exec"
	"strings"
	"testing"
)

// TestCodegenDoesNotImportOpenAPIEngines fails when the generator package
// depends on an OpenAPI library. Engines live under pkg/engine and hand the
// generator a model.Document, so neither library should appear in this
// package's non-test dependencies.
func TestCodegenDoesNotImportOpenAPIEngines(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.Contains(line, "getkin/kin-openapi") || strings.Contains(line, "github.com/pb33f/libopenapi") {
			t.Errorf("pkg/codegen depends on %s", line)
		}
	}
}
