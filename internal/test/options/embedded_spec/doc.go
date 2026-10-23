// Package optionsembeddedspec tests that the embedded spec keeps the raw
// operationId string from the spec, verifying that mixed-case and
// kebab+camel+snake operationIds are preserved exactly as written in the
// OpenAPI spec.
package optionsembeddedspec

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config.yaml spec.yaml
