// Package responseheaders verifies that response headers declared on an
// operation are emitted as a Headers struct on the server
// response object and written by the generated Visit* method.
// Compile-time assertion only.
//
// From issue-1676.
package responseheaders

//go:generate go run github.com/ifc7/oapi-codegen/cmd/oapi-codegen --config=config.yaml spec.yaml
