package multijson_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	multijson "github.com/oapi-codegen/oapi-codegen/v2/internal/test/bodies/content_types/multi_json"
	"github.com/stretchr/testify/assert"
)

type testServer struct {
	t *testing.T
}

// (GET /test)
func (s *testServer) Test(ctx context.Context, request multijson.TestRequestObject) (multijson.TestResponseObject, error) {
	return nil, nil
}

// (GET /suffix)
func (s *testServer) SuffixTest(ctx context.Context, request multijson.SuffixTestRequestObject) (multijson.SuffixTestResponseObject, error) {
	assert.Equal(s.t, "test1", request.Body.Field1)
	assert.Equal(s.t, "test2", request.Body.Field2)
	return multijson.SuffixTest204Response{}, nil
}

// From issue-1298: a request body with a custom +json suffix content type must
// round-trip through the server via the generated typed client method.
func TestIssue1298(t *testing.T) {
	h := multijson.Handler(&testServer{t: t})
	ts := httptest.NewServer(h)
	defer ts.Close()

	c, err := multijson.NewClientWithResponses(ts.URL)
	assert.NoError(t, err)
	res, err := c.SuffixTestWithApplicationTestPlusJSONBodyWithResponse(
		context.TODO(),
		multijson.SuffixBody{
			Field1: "test1",
			Field2: "test2",
		},
	)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, res.StatusCode())
}
