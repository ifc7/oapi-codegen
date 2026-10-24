package externaljsonref

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/util"
)

func TestOverlayApply(t *testing.T) {
	spec, err := util.LoadSwaggerWithOverlay("spec/spec.yaml", util.LoadSwaggerWithOverlayOpts{
		Path:   "overlay.yaml",
		Strict: true,
	})
	require.NoError(t, err)

	require.Equal(t, "structured-overlay", spec.Info.Extensions["x-overlay-applied"])
}
