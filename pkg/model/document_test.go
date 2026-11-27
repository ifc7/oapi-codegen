package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAPIVersion(t *testing.T) {
	t.Parallel()

	var nilDoc *Document
	require.Equal(t, "", nilDoc.OpenAPIMajorMinor())
	require.False(t, nilDoc.IsOpenAPI31OrLater())

	cases := []struct {
		version    string
		majorMinor string
		is31       bool
	}{
		{"", "", false},
		{"3", "3.0", false},
		{"3.0", "3.0", false},
		{"3.0.0", "3.0", false},
		{"3.0.1", "3.0", false},
		{"3.0.2", "3.0", false},
		{"3.0.3", "3.0", false},
		{"3.0.4", "3.0", false},
		{"3.1", "3.1", true},
		{"3.1.0", "3.1", true},
		{"3.1.1", "3.1", true},
		{"3.1.2", "3.1", true},
		{"3.2", "3.2", true},
		{"3.2.0", "3.2", true},
		{"3.2.1", "3.2", true},
		// An unlisted patch is not read as that minor version.
		{"3.0.5", "", false},
		{"3.1.3", "", false},
		{"3.2.2", "", false},
		{"3.3", "", false},
		{"2.0", "", false},
		{"swagger", "", false},
	}
	for _, c := range cases {
		doc := &Document{OpenAPI: c.version}
		require.Equal(t, c.majorMinor, doc.OpenAPIMajorMinor(), c.version)
		require.Equal(t, c.is31, doc.IsOpenAPI31OrLater(), c.version)
	}
}
