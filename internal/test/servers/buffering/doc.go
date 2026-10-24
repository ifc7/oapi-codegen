// Package serversbuffering is a server regression for response buffering /
// error-handler ordering: JSON server responses must be marshalled into a buffer BEFORE
// WriteHeader, so a ResponseErrorHandlerFunc can still change the status code if encoding
// fails (non-JSON responses keep the headers-first ordering).
//
// issue #1963.
package serversbuffering

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config.yaml spec.yaml
