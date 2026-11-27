// Package schemassecurity exercises OpenAPI security scheme handling: specifically
// that a global security reference naming a scheme not defined under
// components/securitySchemes is tolerated by the code generator and produces
// a package that compiles without error.
package schemassecurity

//go:generate go run github.com/ifc7/oapi-codegen/cmd/oapi-codegen --config=config.yaml spec.yaml
