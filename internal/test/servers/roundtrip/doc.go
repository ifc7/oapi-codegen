// Package serversroundtrip is the server roundtrip harness: one shared spec
// (spec.yaml), one generated client, one table-driven test
// (roundtrip_test.go), and a thin server adapter + generated server in stdhttp/.
//
// Folds in: strict-server/{client,stdhttp}. The issue-1963 response buffering /
// error handler case is its own package, ../buffering.
package serversroundtrip
