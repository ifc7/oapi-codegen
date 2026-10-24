// Package optionsfilter drives generation of the operations/ and tags/ sub-packages,
// which exercise output filtering by operationId and by tag respectively. The actual
// generated servers and their tests live in those sub-packages (each is its own Go
// package because each emits a colliding server).
package optionsfilter

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config_tags.yaml server.yaml
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config_operations.yaml server.yaml
