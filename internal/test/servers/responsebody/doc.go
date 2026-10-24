// Package serversresponsebody verifies that server responses whose body
// schema lowers to any compile, and write their values to the wire: OpenAPI 3.1
// unions and nulls as JSON, and untyped text and form bodies.
package serversresponsebody

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config.yaml spec.yaml
