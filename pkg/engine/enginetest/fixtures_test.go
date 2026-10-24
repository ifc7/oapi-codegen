package enginetest

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateFixtures(t *testing.T) {
	fixtures := GenerateFixtures(t)
	require.NotEmpty(t, fixtures)

	var overlays int
	for _, fx := range fixtures {
		assert.NotEmpty(t, fx.Name)
		assert.False(t, strings.HasPrefix(fx.Name, "/"), "name %q should be relative to the repository", fx.Name)
		_, err := os.Stat(fx.Spec)
		assert.NoError(t, err, fx.Name)
		if fx.Opts.OverlayPath == "" {
			continue
		}
		overlays++
		_, err = os.Stat(fx.Opts.OverlayPath)
		assert.NoError(t, err, fx.Name)
	}
	assert.NotZero(t, overlays, "expected at least one fixture to apply an Overlay")
}
