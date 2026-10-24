package referencesmultipackage

import (
	"testing"

	packageA "github.com/oapi-codegen/oapi-codegen/v2/internal/test/references/multipackage/packageA"
	packageB "github.com/oapi-codegen/oapi-codegen/v2/internal/test/references/multipackage/packageB"
)

func TestParameters(t *testing.T) {
	b := &packageB.ObjectB{}
	_ = Container{
		ObjectA: &packageA.ObjectA{ObjectB: b},
		ObjectB: b,
	}
}
