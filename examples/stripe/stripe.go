// The Stripe spec is large, so it is fetched rather than checked in, and its
// generated output is gitignored.
//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config config.yaml https://raw.githubusercontent.com/stripe/openapi/master/openapi/spec3.yaml

package stripe
