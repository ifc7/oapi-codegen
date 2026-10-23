package optionsfilteroperations

import (
	"net/http"
	"testing"
)

// filter/operations: with include-operation-ids included-operation1,included-operation2
// the generated ServerInterface exposes exactly IncludedOperation1 and IncludedOperation2
// (filtered-operation is excluded). The struct below satisfying ServerInterface is the
// compile-time proof; the calls exercise the generated handler signatures.
type server struct{}

func (s server) IncludedOperation1(w http.ResponseWriter, r *http.Request) {}

func (s server) IncludedOperation2(w http.ResponseWriter, r *http.Request) {}

func TestServer(_ *testing.T) {
	var s ServerInterface = server{}
	s.IncludedOperation1(nil, nil)
	s.IncludedOperation2(nil, nil)
}
