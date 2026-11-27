package codegen

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ifc7/oapi-codegen/pkg/engine/enginetest"
	"github.com/ifc7/oapi-codegen/pkg/engine/libopenapi"
	"github.com/ifc7/oapi-codegen/pkg/model"
)

// The keys a plain object variant gets from its properties must be the ones
// its generated type gives, for every object in every fixture document.
func TestPlainObjectKeysMatchGeneratedType(t *testing.T) {
	// Generate sets the package's state; leave it as found for other tests.
	prevState := globalState
	t.Cleanup(func() { globalState = prevState })
	var compared, skipped int
	fixtures := enginetest.GenerateFixtures(t)
	// A large document can be checked as well, such as a fetched Stripe spec:
	//   VARIANT_KEYS_SPEC=/path/to/spec3.yaml go test ./pkg/codegen -run TestPlainObjectKeys
	if extra := os.Getenv("VARIANT_KEYS_SPEC"); extra != "" {
		fixtures = append(fixtures, enginetest.Fixture{Name: extra, Spec: extra})
	}
	for _, fx := range fixtures {
		doc, err := libopenapi.Engine{}.Load(fx.Spec, fx.Opts)
		if err != nil {
			continue
		}
		_, err = Generate(doc, Configuration{
			PackageName: "api",
			Generate:    GenerateOptions{Models: true, Client: true},
		})
		if err != nil {
			continue
		}

		seen := make(map[*model.Schema]bool)
		_ = walkSwagger(doc, func(w RefWrapper) (bool, error) {
			ref, ok := w.SourceRef.(*model.SchemaRef)
			if !ok || ref == nil || ref.Value == nil || seen[ref.Value] {
				return false, nil
			}
			seen[ref.Value] = true
			if isOpaqueSchema(ref) {
				return true, nil
			}
			fast, ok := plainObjectKeys(ref.Value)
			if !ok {
				skipped++
				return true, nil
			}
			ctx := newGenContext(nil)
			ctx.composition.variantKeyCache[ref.Value] = nil
			slow, err := ctx.variantKeysFromGeneratedType(&model.SchemaRef{Value: ref.Value})
			require.NoError(t, err, "%s", fx.Name)
			require.ElementsMatch(t, slow, fast, "%s: %s", fx.Name, w.Ref)
			compared++
			return true, nil
		})
	}
	t.Logf("compared %d schemas, %d left to the generated type", compared, skipped)
	require.NotZero(t, compared)
}
