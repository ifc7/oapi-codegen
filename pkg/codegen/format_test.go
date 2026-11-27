package codegen

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/tools/imports"

	"github.com/ifc7/oapi-codegen/v2/pkg/engine/enginetest"
	"github.com/ifc7/oapi-codegen/v2/pkg/engine/libopenapi"
)

// Splitting the file must give the bytes imports.Process gives, whatever the
// sizes of the pieces. The raw output of every fixture is formatted both ways.
func TestFormatInPiecesMatchesImportsProcess(t *testing.T) {
	// Generate sets the package's state; leave it as found for other tests.
	prevState := globalState
	t.Cleanup(func() { globalState = prevState })
	fixtures := enginetest.GenerateFixtures(t)
	// A large document can be checked as well:
	//   FORMAT_SPEC=/path/to/spec3.yaml go test ./pkg/codegen -run TestFormatInPieces
	if extra := os.Getenv("FORMAT_SPEC"); extra != "" {
		fixtures = append(fixtures, enginetest.Fixture{Name: extra, Spec: extra})
	}

	minSize, chunkSize := parallelFormatMin, formatChunkSize
	t.Cleanup(func() { parallelFormatMin, formatChunkSize = minSize, chunkSize })

	var compared, fallbacks int
	for _, fx := range fixtures {
		doc, err := libopenapi.Engine{}.Load(fx.Spec, fx.Opts)
		if err != nil {
			continue
		}
		raw, err := Generate(doc, Configuration{
			PackageName:   "api",
			Generate:      GenerateOptions{Models: true, Client: true, Server: true},
			OutputOptions: OutputOptions{SkipFmt: true},
		})
		if err != nil {
			continue
		}
		want, err := imports.Process("api.go", []byte(raw), nil)
		require.NoError(t, err, fx.Name)

		for _, size := range []int{256, 4 << 10, 64 << 10} {
			parallelFormatMin, formatChunkSize = 0, size
			// The split itself, so that a source it can't split is not hidden
			// by the fall back to the single call.
			got, ok := formatInPieces("api.go", []byte(raw))
			if !ok {
				fallbacks++
				t.Logf("%s with pieces of %d bytes: not split", fx.Name, size)
				continue
			}
			require.Equal(t, string(want), string(got), "%s with pieces of %d bytes", fx.Name, size)
			whole, err := formatGoSource("api.go", []byte(raw))
			require.NoError(t, err, fx.Name)
			require.Equal(t, string(want), string(whole), fx.Name)
		}
		compared++
	}
	require.NotZero(t, compared)
	require.Zero(t, fallbacks, "sources the split could not handle")
	t.Logf("compared %d documents", compared)
}

// A source that doesn't parse reports the error imports.Process reports.
func TestFormatGoSourceReportsSyntaxErrors(t *testing.T) {
	minSize := parallelFormatMin
	t.Cleanup(func() { parallelFormatMin = minSize })
	parallelFormatMin = 0

	src := []byte("package api\n\nimport \"fmt\"\n\nfunc broken( {\n")
	_, want := imports.Process("api.go", src, nil)
	require.Error(t, want)
	_, got := formatGoSource("api.go", src)
	require.EqualError(t, got, want.Error())
}
