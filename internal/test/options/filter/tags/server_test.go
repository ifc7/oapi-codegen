package optionsfiltertags

import (
	"context"
	"testing"
)

// filter/tags: with include-tags included-tag1,included-tag2 the generated
// ServerInterface exposes exactly IncludedOperation1 and IncludedOperation2 (the
// filtered-tag operation is excluded). The struct below satisfying ServerInterface is
// the compile-time proof; the calls exercise the generated handler signatures.
type server struct{}

func (s server) IncludedOperation1(ctx context.Context, request IncludedOperation1RequestObject) (IncludedOperation1ResponseObject, error) {
	return IncludedOperation1200Response{}, nil
}

func (s server) IncludedOperation2(ctx context.Context, request IncludedOperation2RequestObject) (IncludedOperation2ResponseObject, error) {
	return IncludedOperation2200Response{}, nil
}

func TestServer(_ *testing.T) {
	var s ServerInterface = server{}
	_, _ = s.IncludedOperation1(context.Background(), IncludedOperation1RequestObject{})
	_, _ = s.IncludedOperation2(context.Background(), IncludedOperation2RequestObject{})
}
