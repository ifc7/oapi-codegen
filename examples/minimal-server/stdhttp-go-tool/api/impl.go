package api

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
