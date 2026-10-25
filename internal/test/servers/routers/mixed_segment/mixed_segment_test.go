//go:build go1.22

package serversroutersmixedsegment

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testServer struct {
	called string
}

func (s *testServer) ListAddresses(ctx context.Context, request ListAddressesRequestObject) (ListAddressesResponseObject, error) {
	s.called = "list"
	return ListAddresses200TextResponse(request.AccountSid), nil
}

func (s *testServer) FetchAddress(ctx context.Context, request FetchAddressRequestObject) (FetchAddressResponseObject, error) {
	s.called = "fetch"
	return FetchAddress200TextResponse(request.AccountSid + " " + request.Sid), nil
}

func (s *testServer) ApplyResource(ctx context.Context, request ApplyResourceRequestObject) (ApplyResourceResponseObject, error) {
	s.called = "apply"
	return ApplyResource200TextResponse(request.ResourceId), nil
}

func (s *testServer) FetchReadme(ctx context.Context, request FetchReadmeRequestObject) (FetchReadmeResponseObject, error) {
	s.called = "readme"
	return FetchReadme200TextResponse("readme"), nil
}

func (s *testServer) FetchFile(ctx context.Context, request FetchFileRequestObject) (FetchFileResponseObject, error) {
	s.called = "file"
	return FetchFile200TextResponse(request.Name + " " + request.Ext), nil
}

func TestMixedPathSegments(t *testing.T) {
	server := &testServer{}
	handler := Handler(server)

	t.Run("suffix is stripped from the parameter", func(t *testing.T) {
		server.called = ""
		rec := serve(handler, http.MethodGet, "/2010-04-01/Accounts/AC123/Addresses/AD456.json")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "AC123 AD456", rec.Body.String())
		assert.Equal(t, "fetch", server.called)
	})

	t.Run("segment without the suffix is not the operation", func(t *testing.T) {
		server.called = ""
		rec := serve(handler, http.MethodGet, "/2010-04-01/Accounts/AC123/Addresses/AD456")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Empty(t, server.called)
	})

	t.Run("suffix alone is not a parameter", func(t *testing.T) {
		server.called = ""
		rec := serve(handler, http.MethodGet, "/2010-04-01/Accounts/AC123/Addresses/.json")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Empty(t, server.called)
	})

	t.Run("literal json segment is its own route", func(t *testing.T) {
		server.called = ""
		rec := serve(handler, http.MethodGet, "/2010-04-01/Accounts/AC123/Addresses.json")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "AC123", rec.Body.String())
		assert.Equal(t, "list", server.called)
	})

	t.Run("literal suffix after the parameter", func(t *testing.T) {
		server.called = ""
		rec := serve(handler, http.MethodPost, "/resources/res-1:apply")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "res-1", rec.Body.String())
		assert.Equal(t, "apply", server.called)
	})

	t.Run("custom method without the suffix is not the operation", func(t *testing.T) {
		server.called = ""
		rec := serve(handler, http.MethodPost, "/resources/res-1")
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Empty(t, server.called)
	})

	t.Run("two parameters split on the literal between them", func(t *testing.T) {
		server.called = ""
		rec := serve(handler, http.MethodGet, "/files/report.pdf")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "report pdf", rec.Body.String())
		assert.Equal(t, "file", server.called)
	})

	t.Run("concrete path wins over the templated segment", func(t *testing.T) {
		server.called = ""
		rec := serve(handler, http.MethodGet, "/files/readme.json")
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "readme", rec.Body.String())
		assert.Equal(t, "readme", server.called)
	})
}

func serve(handler http.Handler, method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}
