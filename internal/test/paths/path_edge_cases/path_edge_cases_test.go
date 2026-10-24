package pathedgecases

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const hostname = "http://host"

// From issue-312: client URL construction and server unescaping.

func TestClient_WhenPathHasColon_RequestHasCorrectPath(t *testing.T) {
	doer := &HTTPRequestDoerMock{}
	client, _ := NewClientWithResponses(hostname, WithHTTPClient(doer))
	_ = client

	doer.On("Do", mock.Anything).Return(nil, errors.New("something went wrong")).Run(func(args mock.Arguments) {
		req, ok := args.Get(0).(*http.Request)
		assert.True(t, ok)
		assert.Equal(t, "http://host/pets:validate", req.URL.String())
	})

	// TODO: this err should be checked, but it defaults to returning the "something went wrong", so we should refactor accordingly
	_, _ = client.ValidatePetsWithResponse(context.Background(), ValidatePetsJSONRequestBody{
		Names: []string{"fido"},
	})
	doer.AssertExpectations(t)
}

func TestClient_WhenPathHasId_RequestHasCorrectPath(t *testing.T) {
	doer := &HTTPRequestDoerMock{}
	client, _ := NewClientWithResponses(hostname, WithHTTPClient(doer))
	_ = client

	doer.On("Do", mock.Anything).Return(nil, errors.New("something went wrong")).Run(func(args mock.Arguments) {
		req, ok := args.Get(0).(*http.Request)
		assert.True(t, ok)
		assert.Equal(t, "/pets/id", req.URL.Path)
	})
	petID := "id"
	// TODO: this err should be checked, but it defaults to returning the "something went wrong", so we should refactor accordingly
	_, _ = client.GetPetWithResponse(context.Background(), petID)
	doer.AssertExpectations(t)
}

func TestClient_WhenPathHasIdContainingReservedCharacter_RequestHasCorrectPath(t *testing.T) {
	doer := &HTTPRequestDoerMock{}
	client, _ := NewClientWithResponses(hostname, WithHTTPClient(doer))
	_ = client

	doer.On("Do", mock.Anything).Return(nil, errors.New("something went wrong")).Run(func(args mock.Arguments) {
		req, ok := args.Get(0).(*http.Request)
		assert.True(t, ok)
		assert.Equal(t, "http://host/pets/id1%2Fid2", req.URL.String())
	})
	petID := "id1/id2"
	// TODO: this err should be checked, but it defaults to returning the "something went wrong", so we should refactor accordingly
	_, _ = client.GetPetWithResponse(context.Background(), petID)
	doer.AssertExpectations(t)
}

func TestClient_ServerUnescapesEscapedArg(t *testing.T) {
	m := &petServer{}
	svr := httptest.NewServer(Handler(m))
	defer svr.Close()

	client, err := NewClientWithResponses(svr.URL)
	require.NoError(t, err)

	petID := "id1/id2"
	response, err := client.GetPetWithResponse(context.Background(), petID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode())
	assert.Equal(t, petID, m.receivedPetID)
}

// From issue-1180: TestParameterPrecedence validates that the parameter `param`
// is a string type, rather than an int, as we should prioritise the `param`
// definition closest to the operation.
func TestParameterPrecedence(t *testing.T) {
	_, _ = NewGetSimplePrimitiveRequest("http://example.com/", "test-string")
}

// HTTPRequestDoerMock mocks the interface HttpRequestDoerMock.
type HTTPRequestDoerMock struct {
	mock.Mock
}

func (m *HTTPRequestDoerMock) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// petServer records the petId the server unescaped from the path. It
// implements only GetPet; the embedded interface stands in for the rest.
type petServer struct {
	ServerInterface
	receivedPetID string
}

func (m *petServer) GetPet(ctx context.Context, request GetPetRequestObject) (GetPetResponseObject, error) {
	m.receivedPetID = request.PetId
	return GetPet200JSONResponse{}, nil
}
