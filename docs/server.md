# The `net/http` Server

`oapi-codegen` generates a server built on the standard library's [`net/http`](https://pkg.go.dev/net/http), using the routing patterns added in Go 1.22. To generate it, you will want a configuration file such as:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/oapi-codegen/oapi-codegen/v2.8.0/configuration-schema.json
package: api
generate:
  server: true
  models: true
output: gen.go
```

## Generated code

For instance, let's take this straightforward specification:

```yaml
openapi: "3.0.0"
info:
  version: 1.0.0
  title: Minimal ping API server
paths:
  /ping:
    get:
      responses:
        '200':
          description: pet response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Pong'
components:
  schemas:
    # base types
    Pong:
      type: object
      required:
        - ping
      properties:
        ping:
          type: string
          example: pong
```

For each operation, this generates a request object holding the operation's parameters and body, a response type for each response the spec declares, and a method on the `ServerInterface`:

```go
// Pong defines model for Pong.
type Pong struct {
	Ping string `json:"ping"`
}

type GetPingRequestObject struct {
}

type GetPingResponseObject interface {
	VisitGetPingResponse(w http.ResponseWriter) error
}

// GetPing200JSONResponse writes itself as the 200 response's JSON body.
type GetPing200JSONResponse Pong

// ServerInterface represents all server handlers.
type ServerInterface interface {

	// (GET /ping)
	GetPing(ctx context.Context, request GetPingRequestObject) (GetPingResponseObject, error)
}

// Handler creates an http.Handler with routing matching the OpenAPI spec.
func Handler(si ServerInterface) http.Handler {
	return HandlerWithOptions(si, ServerOptions{})
}

// HandlerWithOptions creates an http.Handler with routing matching the
// OpenAPI spec, configured by options.
func HandlerWithOptions(si ServerInterface, options ServerOptions) http.Handler {
	// ... omitted for brevity
}
```

The generated handler binds each request's parameters and body into the operation's request object, calls your implementation, and writes the response object it returns, setting the status code, `Content-Type` and any response headers.

## Implementing the server

To implement this HTTP server, we need to write the following code in our [`api/impl.go`](../examples/minimal-server/stdhttp/api/impl.go):

```go
import (
	"context"
)

// ensure that we've conformed to the `ServerInterface` with a compile-time check
var _ ServerInterface = (*Server)(nil)

type Server struct{}

func NewServer() Server {
	return Server{}
}

// (GET /ping)
func (Server) GetPing(ctx context.Context, request GetPingRequestObject) (GetPingResponseObject, error) {
	return GetPing200JSONResponse{
		Ping: "pong",
	}, nil
}
```

A handler returns one of its operation's response types. When it returns an `error` instead, the server calls `ServerOptions.ResponseErrorHandlerFunc`, which by default responds with an [HTTP 500 Internal Server Error](https://http.cat/500).

Now we've got our implementation, we can then write the following code to wire it up and get a running server:

```go
import (
	"log"
	"net/http"

	"github.com/oapi-codegen/oapi-codegen/v2/examples/minimal-server/stdhttp/api"
)

func main() {
	// create a type that satisfies the `api.ServerInterface`, which contains an implementation of every operation from the generated code
	server := api.NewServer()

	// get an `http.Handler` that we can use
	h := api.Handler(server)

	s := &http.Server{
		Handler: h,
		Addr:    "0.0.0.0:8080",
	}

	// And we serve HTTP until the world ends.
	log.Fatal(s.ListenAndServe())
}
```

## Options

`HandlerWithOptions` takes a `ServerOptions`:

```go
type ServerOptions struct {
	// BaseURL prefixes every operation's path.
	BaseURL string
	// BaseRouter is the mux the operations are registered on. When nil, they
	// are registered on a new http.ServeMux.
	BaseRouter ServeMux
	// Middlewares wrap every operation's handler, in order, so the last is
	// the outermost.
	Middlewares []MiddlewareFunc
	// RequestErrorHandlerFunc answers a request whose parameters or body
	// can't be read. When nil, it responds 400 with the error's message.
	RequestErrorHandlerFunc func(w http.ResponseWriter, r *http.Request, err error)
	// ResponseErrorHandlerFunc answers a request whose handler returned an
	// error, or a response that couldn't be written. When nil, it responds
	// 500 with the error's message.
	ResponseErrorHandlerFunc func(w http.ResponseWriter, r *http.Request, err error)
}
```

To serve the API alongside other routes, register it on your own mux:

```go
mux := http.NewServeMux()
mux.HandleFunc("GET /healthz", healthz)

api.HandlerWithOptions(server, api.ServerOptions{
	BaseRouter: mux,
	BaseURL:    "/api/v1",
})
```

### Middleware

A `MiddlewareFunc` wraps an operation's handler, and is told which operation it is wrapping. It sees the operation's request object before your implementation does, and the response object your implementation returns:

```go
func logging(f api.HandlerFunc, operationID string) api.HandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		response, err := f(ctx, w, r, request)
		log.Printf("%s: %T, %v", operationID, response, err)
		return response, err
	}
}

h := api.HandlerWithOptions(server, api.ServerOptions{
	Middlewares: []api.MiddlewareFunc{logging},
})
```

For middleware that only needs the HTTP request, such as the [validation middleware](../README.md#requestresponse-validation-middleware), wrap the `http.Handler` that `Handler` returns.

> [!NOTE]
> This doesn't include [validation of incoming requests](../README.md#requestresponse-validation-middleware).

> [!NOTE]
> If you feel like you've done everything right, but are still receiving `404 page not found` errors, make sure that you've got the `go` directive in your `go.mod` set to at least `1.22`, which enables the enhanced `net/http` routing. In practice it should be at least:

```go.mod
go 1.22
```
