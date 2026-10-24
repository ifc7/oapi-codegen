package parentchild

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/kin"
)

func TestIssue(t *testing.T) {
	swagger, err := kin.Engine{}.Load("child.api.yaml", engine.LoadOptions{})
	require.NoError(t, err)

	opts := codegen.Configuration{
		PackageName: "parentchild",
		Generate: codegen.GenerateOptions{
			Server: true,
			Models: true,
		},
		ImportMapping: map[string]string{
			"parent.api.yaml": "github.com/oapi-codegen/oapi-codegen/v2/internal/test/references/multipackage/parent_child/api/parent",
		},
	}

	_, err = codegen.Generate(swagger, opts)
	require.NoError(t, err)
}
