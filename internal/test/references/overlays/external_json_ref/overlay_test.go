package externaljsonref

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/kin"
)

func TestOverlayApply(t *testing.T) {
	spec, err := kin.Engine{}.Load("spec/spec.yaml", engine.LoadOptions{
		OverlayPath:   "overlay.yaml",
		OverlayStrict: true,
	})
	require.NoError(t, err)

	require.Equal(t, "structured-overlay", spec.Info.Extensions["x-overlay-applied"])
}
