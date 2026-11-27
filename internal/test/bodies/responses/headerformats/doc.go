// Package responseheaderformats verifies that server response headers
// are serialized through the runtime styling helpers rather than fmt.Sprint, so
// that format-carrying types (date-time, uuid) go on the wire in the
// representation the OpenAPI spec requires.
//
// From issue-2512.
package responseheaderformats

//go:generate go run github.com/ifc7/oapi-codegen/cmd/oapi-codegen --config=config.yaml spec.yaml
