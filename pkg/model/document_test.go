package model

import (
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func TestOpenAPIVersionMatchesKin(t *testing.T) {
	t.Parallel()

	var nilDoc *Document
	var nilKin *openapi3.T
	require.Equal(t, nilKin.OpenAPIMajorMinor(), nilDoc.OpenAPIMajorMinor())
	require.Equal(t, nilKin.IsOpenAPI31OrLater(), nilDoc.IsOpenAPI31OrLater())

	versions := []string{
		"",
		"3", "3.0", "3.0.0", "3.0.1", "3.0.2", "3.0.3", "3.0.4",
		"3.1", "3.1.0", "3.1.1", "3.1.2",
		"3.2", "3.2.0", "3.2.1",
		// Kin does not treat an unlisted patch as that minor version.
		"3.0.5", "3.1.3", "3.2.2", "3.3", "2.0", "swagger",
	}
	for _, version := range versions {
		ours := &Document{OpenAPI: version}
		kin := &openapi3.T{OpenAPI: version}
		require.Equal(t, kin.OpenAPIMajorMinor(), ours.OpenAPIMajorMinor(), version)
		require.Equal(t, kin.IsOpenAPI31OrLater(), ours.IsOpenAPI31OrLater(), version)
	}
}
