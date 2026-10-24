package serverssecurity

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// server records the security requirements each handler sees.
type server struct {
	seen []SecurityRequirement
}

func (s *server) Inherited(ctx context.Context, request InheritedRequestObject) (InheritedResponseObject, error) {
	s.seen = SecurityRequirements(ctx)
	return Inherited204Response{}, nil
}

func (s *server) Alternatives(ctx context.Context, request AlternativesRequestObject) (AlternativesResponseObject, error) {
	s.seen = SecurityRequirements(ctx)
	return Alternatives204Response{}, nil
}

func (s *server) Public(ctx context.Context, request PublicRequestObject) (PublicResponseObject, error) {
	s.seen = SecurityRequirements(ctx)
	return Public204Response{}, nil
}

func TestSecurityRequirements(t *testing.T) {
	tests := []struct {
		name string
		path string
		want []SecurityRequirement
	}{
		{
			name: "inherited from the document",
			path: "/inherited",
			want: []SecurityRequirement{{"bearerAuth": {"things:read"}}},
		},
		{
			name: "alternatives, combined schemes and anonymous",
			path: "/alternatives",
			want: []SecurityRequirement{
				{"oauth2": {"things:read", `things:write "all"`}},
				{"apiKey": {}, "bearerAuth": {}},
				{},
			},
		},
		{
			name: "removed by an empty list",
			path: "/public",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &server{}
			var middlewareSaw []SecurityRequirement
			h := HandlerWithOptions(s, ServerOptions{
				Middlewares: []MiddlewareFunc{func(f HandlerFunc, operationID string) HandlerFunc {
					return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
						middlewareSaw = SecurityRequirements(ctx)
						return f(ctx, w, r, request)
					}
				}},
			})

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
			assert.Equal(t, tt.want, s.seen, "handler")
			assert.Equal(t, tt.want, middlewareSaw, "middleware")
		})
	}
}

// A request that fails binding is rejected before any middleware runs, but
// the RequestErrorHandlerFunc can still tell which requirements applied, for
// instance to answer 401 rather than 400 to an unauthenticated request.
func TestSecurityRequirementsInRequestErrorHandler(t *testing.T) {
	var saw []SecurityRequirement
	h := HandlerWithOptions(&server{}, ServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			saw = SecurityRequirements(r.Context())
			http.Error(w, err.Error(), http.StatusBadRequest)
		},
	})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/inherited?limit=many", nil))

	require.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, []SecurityRequirement{{"bearerAuth": {"things:read"}}}, saw)
}
