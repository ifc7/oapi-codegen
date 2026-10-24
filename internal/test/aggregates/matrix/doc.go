// Package matrix is the aggregate-schema (allOf / anyOf / oneOf) test matrix.
//
// Each file in shapes/ describes one schema shape: a subject schema, the
// components it needs, and JSON samples that are valid instances of it.
// specgen turns every shape into a package, <shape>/, whose spec places the
// subject at each position we generate code for (component schema, property,
// array items, additionalProperties value, request and response bodies, inline
// or referenced, component request bodies and responses, a `default:` response
// and a response with headers). The
// generated harness sends every sample through a generated client and
// server and checks that it comes back unchanged, both on the wire and after
// parsing.
//
// The generator can reject a shape instead (the shape's rejects key). Its package
// then holds no generated code, and its test checks that generation fails with
// the expected error.
//
// Shape-specific checks (Go types, From/As behavior, discriminators) live in
// hand-written test files next to the harness.

package matrix

//go:generate go run ./specgen
