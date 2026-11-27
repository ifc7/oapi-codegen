// The Twilio spec is large, so it is fetched rather than checked in, and its
// generated output is gitignored.
//go:generate go run github.com/ifc7/oapi-codegen/v2/cmd/oapi-codegen -config config.yaml https://raw.githubusercontent.com/twilio/twilio-oai/main/spec/yaml/twilio_api_v2010.yaml

package twilio
