// Package serversrouterssmoke is a compile smoke test: the same spec is generated
// for the net/http server and the client, into sub-packages, to ensure the
// generated code compiles.
//
// issue #1799.
package serversrouterssmoke

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config-server.yaml spec.yaml
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=config-client.yaml spec.yaml
