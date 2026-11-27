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
	slowHeaderFixes.Store(0)
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
	require.Zero(t, slowHeaderFixes.Load(), "headers left to imports.Process")
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

func TestWithStandardImports(t *testing.T) {
	header := []byte("package api\n\nimport (\n\t\"fmt\"\n\t\"sort\"\n)")
	refs := func(names ...string) map[string]map[string]bool {
		m := map[string]map[string]bool{}
		for _, n := range names {
			m[n] = map[string]bool{"X": true}
		}
		return m
	}

	// Used and missing: added, in order, inside the block.
	got := withStandardImports([]byte("package api\n\nimport (\n\t\"fmt\"\n)"), refs("sort", "math", "fmt", "base64"))
	require.Equal(t, "package api\n\nimport (\n\t\"fmt\"\n\t\"encoding/base64\"\n\t\"math\"\n\t\"sort\"\n)", string(got))

	// Into the first group of the block, before the others.
	grouped := []byte("package api\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/x/y\"\n)")
	require.Equal(t, "package api\n\nimport (\n\t\"fmt\"\n\t\"sort\"\n\n\t\"github.com/x/y\"\n)",
		string(withStandardImports(grouped, refs("sort"))))

	// Already imported, or not used: unchanged.
	require.Equal(t, string(header), string(withStandardImports(header, refs("sort", "fmt"))))
	require.Equal(t, string(header), string(withStandardImports(header, refs("fmt"))))

	// An ambiguous name is left to goimports.
	require.Equal(t, string(header), string(withStandardImports(header, refs("template", "rand"))))

	// No import block to add to.
	require.Equal(t, "package api", string(withStandardImports([]byte("package api"), refs("sort"))))
}

func TestWithoutUnusedThirdPartyImports(t *testing.T) {
	header := []byte("package api\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/oapi-codegen/nullable\"\n\t\"go.yaml.in/yaml/v3\"\n\t\"github.com/x/y\"\n)")
	refs := func(names ...string) map[string]map[string]bool {
		m := map[string]map[string]bool{}
		for _, n := range names {
			m[n] = map[string]bool{"X": true}
		}
		return m
	}
	// Unused ones are removed, others, and imports not of the template, stay.
	require.Equal(t, "package api\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/x/y\"\n)",
		string(withoutUnusedThirdPartyImports(header, refs("fmt"))))
	require.Equal(t, "package api\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/oapi-codegen/nullable\"\n\t\"github.com/x/y\"\n)",
		string(withoutUnusedThirdPartyImports(header, refs("nullable"))))
	// The input is not changed.
	require.Contains(t, string(header), "go.yaml.in/yaml/v3")
}

func TestFixHeaderImports(t *testing.T) {
	refs := func(names ...string) map[string]map[string]bool {
		m := map[string]map[string]bool{}
		for _, n := range names {
			m[n] = map[string]bool{"X": true}
		}
		return m
	}
	const block = "package api\n\nimport (\n\t\"os\"\n\t\"fmt\"\n\t\"go.yaml.in/yaml/v3\"\n\t\"mime/multipart\"\n\n\t\"github.com/oapi-codegen/nullable\"\n\text \"github.com/x/ext\"\n)"

	// Unused imports go, missing known ones are added, the rest is sorted and
	// grouped as goimports does.
	got, ok := fixHeaderImports("api.go", []byte(block), refs("fmt", "multipart", "sort", "ext"))
	require.True(t, ok)
	require.Equal(t, "package api\n\nimport (\n\t\"fmt\"\n\t\"mime/multipart\"\n\t\"sort\"\n\n\text \"github.com/x/ext\"\n)\n", string(got))

	// Everything unused leaves no import block entries to keep, and none of
	// the groups left empty.
	got, ok = fixHeaderImports("api.go", []byte(block), refs("nullable"))
	require.True(t, ok)
	require.Equal(t, "package api\n\nimport (\n\t\"github.com/oapi-codegen/nullable\"\n)\n", string(got))

	// What it can't be sure of is left to imports.Process.
	for name, header := range map[string]string{
		"unnamed third-party":   "package api\n\nimport (\n\t\"fmt\"\n\t\"github.com/x/unknown\"\n)",
		"comment in the block":  "package api\n\nimport (\n\t\"fmt\" // used\n)",
		"no import block":       "package api\n",
		"a line comment inside": "package api\n\nimport (\n\t// fmt is used\n\t\"fmt\"\n)",
	} {
		_, ok := fixHeaderImports("api.go", []byte(header), refs("fmt"))
		require.False(t, ok, name)
	}
	// A use of a package that is neither imported nor known.
	_, ok = fixHeaderImports("api.go", []byte(block), refs("fmt", "template"))
	require.False(t, ok)
}
