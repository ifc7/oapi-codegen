package serversroutersbasic

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fakeServer implements only the operation the tests reach; the embedded
// interface stands in for the rest.
type fakeServer struct {
	StrictServerInterface
	t      *testing.T
	called bool
}

// Create a resource with inline parameter
// (POST /resource2/{inline_argument})
func (s *fakeServer) CreateResource2(ctx context.Context, request CreateResource2RequestObject) (CreateResource2ResponseObject, error) {
	assert.Equal(s.t, 99, *request.Params.InlineQueryArgument)
	assert.Equal(s.t, 1, request.InlineArgument)
	s.called = true
	return CreateResource2200JSONResponse{}, nil
}

func TestParameters(t *testing.T) {
	m := fakeServer{
		t: t,
	}

	h := Handler(NewStrictHandler(&m, nil))

	req := httptest.NewRequest("POST", "http://openapitest.deepmap.ai/resource2/1?inline_query_argument=99", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.True(t, m.called)
}

func TestErrorHandlerFunc(t *testing.T) {
	m := fakeServer{}

	h := HandlerWithOptions(NewStrictHandler(&m, nil), StdHTTPServerOptions{
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "application/json")
			var requiredParamError *RequiredParamError
			assert.True(t, errors.As(err, &requiredParamError))
		},
	})

	s := httptest.NewServer(h)
	defer s.Close()

	req, err := http.DefaultClient.Get(s.URL + "/get-with-args")
	assert.Nil(t, err)
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestErrorHandlerFuncBackwardsCompatible(t *testing.T) {
	m := fakeServer{}

	h := HandlerWithOptions(NewStrictHandler(&m, nil), StdHTTPServerOptions{})

	s := httptest.NewServer(h)
	defer s.Close()

	req, err := http.DefaultClient.Get(s.URL + "/get-with-args")
	b, _ := io.ReadAll(req.Body)
	assert.Nil(t, err)
	assert.Equal(t, "text/plain; charset=utf-8", req.Header.Get("Content-Type"))
	assert.Equal(t, "Query argument required_argument is required, but not found\n", string(b))
}
