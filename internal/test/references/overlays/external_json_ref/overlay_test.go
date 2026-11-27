package externaljsonref

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ifc7/oapi-codegen/pkg/engine"
	"github.com/ifc7/oapi-codegen/pkg/engine/libopenapi"
)

func TestOverlayApply(t *testing.T) {
	spec, err := libopenapi.Engine{}.Load("spec/spec.yaml", engine.LoadOptions{
		OverlayPath:   "overlay.yaml",
		OverlayStrict: true,
	})
	require.NoError(t, err)

	require.Equal(t, "structured-overlay", spec.Info.Extensions["x-overlay-applied"])
}
