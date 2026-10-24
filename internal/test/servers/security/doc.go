// Package serverssecurity verifies that each operation's security
// requirements reach the request's context, where SecurityRequirements reads
// them: the document's requirement for operations that don't declare their
// own, an operation's alternatives (OR), combined schemes (AND) and anonymous
// ({}) alternatives, and none for an operation that declares an empty list.
package serverssecurity

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config.yaml spec.yaml
