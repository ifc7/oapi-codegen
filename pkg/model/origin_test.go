package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFieldLocationsLookup(t *testing.T) {
	t.Parallel()

	fields := FieldLocations{
		{Name: "title", Line: 2, Column: 3},
		{Name: "type", Line: 4, Column: 5},
	}
	loc, ok := fields.Lookup("type")
	require.True(t, ok)
	require.Equal(t, Location{Name: "type", Line: 4, Column: 5}, loc)
	require.Equal(t, loc, fields.Get("type"))

	_, ok = fields.Lookup("missing")
	require.False(t, ok)
	require.Equal(t, Location{}, fields.Get("missing"))
}
