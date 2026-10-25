package main

import (
	"testing"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine"
	"github.com/oapi-codegen/oapi-codegen/v2/pkg/engine/engines"
)

func TestLoader(t *testing.T) {
	paths := []string{
		"../../examples/petstore-expanded/petstore-expanded.yaml",
		"https://raw.githubusercontent.com/oapi-codegen/oapi-codegen/v2.4.1/examples/petstore-expanded/petstore-expanded.yaml",
	}

	for _, v := range paths {
		doc, err := engines.Load("", v, engine.LoadOptions{})
		if err != nil {
			t.Error(err)
		}
		if doc == nil || doc.Info == nil || doc.Info.Version == "" {
			t.Error("missing data")
		}
	}
}
