// Package schemasobjects exercises object schemas with additionalProperties, including
// additionalProperties:true inside an array item's inline schema (the server path,
// issue-1277), plus the additionalProperties / readOnly+writeOnly / raw-JSON object cases
// folded from the components.yaml kitchen sink.
package schemasobjects

//go:generate go run github.com/ifc7/oapi-codegen/cmd/oapi-codegen --config=config.yaml spec.yaml
