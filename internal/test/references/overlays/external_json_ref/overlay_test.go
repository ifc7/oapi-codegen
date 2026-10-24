package externaljsonref

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/engines"
)

func TestOverlayApply(t *testing.T) {
	spec, err := engines.Load("", "spec/spec.yaml", engine.LoadOptions{
		OverlayPath:   "overlay.yaml",
		OverlayStrict: true,
	})
	require.NoError(t, err)

	require.Equal(t, "structured-overlay", spec.Info.Extensions["x-overlay-applied"])
}
