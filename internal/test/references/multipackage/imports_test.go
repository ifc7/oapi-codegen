package referencesmultipackage

import (
	"testing"

	packageA "github.com/ifc7/oapi-codegen/internal/test/references/multipackage/packageA"
	packageB "github.com/ifc7/oapi-codegen/internal/test/references/multipackage/packageB"
)

func TestParameters(t *testing.T) {
	b := &packageB.ObjectB{}
	_ = Container{
		ObjectA: &packageA.ObjectA{ObjectB: b},
		ObjectB: b,
	}
}
