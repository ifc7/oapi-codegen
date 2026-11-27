// The Stripe spec is large, so it is fetched rather than checked in, and its
// generated output is gitignored.
//go:generate go run github.com/ifc7/oapi-codegen/cmd/oapi-codegen -config config.yaml https://raw.githubusercontent.com/stripe/openapi/master/openapi/spec3.yaml

package stripe
