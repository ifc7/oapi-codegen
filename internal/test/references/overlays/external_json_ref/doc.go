package externaljsonref

// We place the spec in a subdirectory, so that its external reference is resolved relative to it.
//go:generate go run github.com/ifc7/oapi-codegen/v2/cmd/oapi-codegen --config=config.yaml spec/spec.yaml
