package parentchild

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ifc7/oapi-codegen/pkg/codegen"
	"github.com/ifc7/oapi-codegen/pkg/engine"
	"github.com/ifc7/oapi-codegen/pkg/engine/libopenapi"
)

func TestIssue(t *testing.T) {
	swagger, err := libopenapi.Engine{}.Load("child.api.yaml", engine.LoadOptions{})
	require.NoError(t, err)

	opts := codegen.Configuration{
		PackageName: "parentchild",
		Generate: codegen.GenerateOptions{
			Server: true,
			Models: true,
		},
		ImportMapping: map[string]string{
			"parent.api.yaml": "github.com/ifc7/oapi-codegen/internal/test/references/multipackage/parent_child/api/parent",
		},
	}

	_, err = codegen.Generate(swagger, opts)
	require.NoError(t, err)
}
