// Package parametersroundtrip is the parameter binding roundtrip harness: one
// shared spec (parameters.yaml), one generated client, one table-driven test
// (param_roundtrip_test.go), and a thin server adapter + generated net/http server
// in stdhttp/. The client serializes Go values into a request, the server echoes
// them back as JSON, and the test asserts the roundtrip for every parameter style.
// wire_format_test.go pins the server's parsing of hand-written requests.
//
// Folds in: parameters/{client,stdhttp}
package parametersroundtrip
