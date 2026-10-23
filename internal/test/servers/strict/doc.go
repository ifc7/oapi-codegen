// Package serversstrict is the strict-server roundtrip harness: one shared spec
// (strict-schema.yaml), one generated client, one table-driven test
// (strict_test.go), and a thin strict server adapter + generated server in stdhttp/.
//
// Folds in: strict-server/{client,stdhttp} (issue-1963 response buffering / error
// handler is folded as an additional case — see below).
package serversstrict
