package stdhttpparams

import (
	"context"
	"encoding/json"
	"net/http"

	gen "github.com/oapi-codegen/oapi-codegen/v2/internal/test/parameters/roundtrip/stdhttp/gen"
)

// Server echoes every bound parameter back as JSON text, so the roundtrip test
// can compare what the server parsed against what the client sent.
type Server struct{}

var _ gen.StrictServerInterface = (*Server)(nil)

func echo(v any) (string, error) {
	b, err := json.Marshal(v)
	return string(b), err
}

func (s *Server) GetContentObject(ctx context.Context, request gen.GetContentObjectRequestObject) (gen.GetContentObjectResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetContentObject200TextResponse(body), err
}

func (s *Server) GetCookie(ctx context.Context, request gen.GetCookieRequestObject) (gen.GetCookieResponseObject, error) {
	body, err := echo(request.Params)
	return gen.GetCookiedefaultTextResponse{Body: body, StatusCode: http.StatusOK}, err
}

func (s *Server) EnumParams(ctx context.Context, request gen.EnumParamsRequestObject) (gen.EnumParamsResponseObject, error) {
	return gen.EnumParams204Response{}, nil
}

func (s *Server) GetHeader(ctx context.Context, request gen.GetHeaderRequestObject) (gen.GetHeaderResponseObject, error) {
	body, err := echo(request.Params)
	return gen.GetHeaderdefaultTextResponse{Body: body, StatusCode: http.StatusOK}, err
}

func (s *Server) GetLabelExplodeArray(ctx context.Context, request gen.GetLabelExplodeArrayRequestObject) (gen.GetLabelExplodeArrayResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetLabelExplodeArray200TextResponse(body), err
}

func (s *Server) GetLabelExplodeObject(ctx context.Context, request gen.GetLabelExplodeObjectRequestObject) (gen.GetLabelExplodeObjectResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetLabelExplodeObject200TextResponse(body), err
}

func (s *Server) GetLabelExplodePrimitive(ctx context.Context, request gen.GetLabelExplodePrimitiveRequestObject) (gen.GetLabelExplodePrimitiveResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetLabelExplodePrimitive200TextResponse(body), err
}

func (s *Server) GetLabelNoExplodeArray(ctx context.Context, request gen.GetLabelNoExplodeArrayRequestObject) (gen.GetLabelNoExplodeArrayResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetLabelNoExplodeArray200TextResponse(body), err
}

func (s *Server) GetLabelNoExplodeObject(ctx context.Context, request gen.GetLabelNoExplodeObjectRequestObject) (gen.GetLabelNoExplodeObjectResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetLabelNoExplodeObject200TextResponse(body), err
}

func (s *Server) GetLabelPrimitive(ctx context.Context, request gen.GetLabelPrimitiveRequestObject) (gen.GetLabelPrimitiveResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetLabelPrimitive200TextResponse(body), err
}

func (s *Server) GetMatrixExplodeArray(ctx context.Context, request gen.GetMatrixExplodeArrayRequestObject) (gen.GetMatrixExplodeArrayResponseObject, error) {
	body, err := echo(request.Id)
	return gen.GetMatrixExplodeArray200TextResponse(body), err
}

func (s *Server) GetMatrixExplodeObject(ctx context.Context, request gen.GetMatrixExplodeObjectRequestObject) (gen.GetMatrixExplodeObjectResponseObject, error) {
	body, err := echo(request.Id)
	return gen.GetMatrixExplodeObject200TextResponse(body), err
}

func (s *Server) GetMatrixExplodePrimitive(ctx context.Context, request gen.GetMatrixExplodePrimitiveRequestObject) (gen.GetMatrixExplodePrimitiveResponseObject, error) {
	body, err := echo(request.Id)
	return gen.GetMatrixExplodePrimitive200TextResponse(body), err
}

func (s *Server) GetMatrixNoExplodeArray(ctx context.Context, request gen.GetMatrixNoExplodeArrayRequestObject) (gen.GetMatrixNoExplodeArrayResponseObject, error) {
	body, err := echo(request.Id)
	return gen.GetMatrixNoExplodeArray200TextResponse(body), err
}

func (s *Server) GetMatrixNoExplodeObject(ctx context.Context, request gen.GetMatrixNoExplodeObjectRequestObject) (gen.GetMatrixNoExplodeObjectResponseObject, error) {
	body, err := echo(request.Id)
	return gen.GetMatrixNoExplodeObject200TextResponse(body), err
}

func (s *Server) GetMatrixPrimitive(ctx context.Context, request gen.GetMatrixPrimitiveRequestObject) (gen.GetMatrixPrimitiveResponseObject, error) {
	body, err := echo(request.Id)
	return gen.GetMatrixPrimitive200TextResponse(body), err
}

func (s *Server) GetPassThrough(ctx context.Context, request gen.GetPassThroughRequestObject) (gen.GetPassThroughResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetPassThrough200TextResponse(body), err
}

func (s *Server) GetSimpleString(ctx context.Context, request gen.GetSimpleStringRequestObject) (gen.GetSimpleStringResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetSimpleString200TextResponse(body), err
}

func (s *Server) GetDeepObject(ctx context.Context, request gen.GetDeepObjectRequestObject) (gen.GetDeepObjectResponseObject, error) {
	body, err := echo(request.Params)
	return gen.GetDeepObjectdefaultTextResponse{Body: body, StatusCode: http.StatusOK}, err
}

func (s *Server) GetQueryDelimited(ctx context.Context, request gen.GetQueryDelimitedRequestObject) (gen.GetQueryDelimitedResponseObject, error) {
	body, err := echo(request.Params)
	return gen.GetQueryDelimited200TextResponse(body), err
}

func (s *Server) GetQueryForm(ctx context.Context, request gen.GetQueryFormRequestObject) (gen.GetQueryFormResponseObject, error) {
	body, err := echo(request.Params)
	return gen.GetQueryForm200TextResponse(body), err
}

func (s *Server) GetSimpleExplodeArray(ctx context.Context, request gen.GetSimpleExplodeArrayRequestObject) (gen.GetSimpleExplodeArrayResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetSimpleExplodeArray200TextResponse(body), err
}

func (s *Server) GetSimpleExplodeObject(ctx context.Context, request gen.GetSimpleExplodeObjectRequestObject) (gen.GetSimpleExplodeObjectResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetSimpleExplodeObject200TextResponse(body), err
}

func (s *Server) GetSimpleExplodePrimitive(ctx context.Context, request gen.GetSimpleExplodePrimitiveRequestObject) (gen.GetSimpleExplodePrimitiveResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetSimpleExplodePrimitive200TextResponse(body), err
}

func (s *Server) GetSimpleNoExplodeArray(ctx context.Context, request gen.GetSimpleNoExplodeArrayRequestObject) (gen.GetSimpleNoExplodeArrayResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetSimpleNoExplodeArray200TextResponse(body), err
}

func (s *Server) GetSimpleNoExplodeObject(ctx context.Context, request gen.GetSimpleNoExplodeObjectRequestObject) (gen.GetSimpleNoExplodeObjectResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetSimpleNoExplodeObject200TextResponse(body), err
}

func (s *Server) GetSimplePrimitive(ctx context.Context, request gen.GetSimplePrimitiveRequestObject) (gen.GetSimplePrimitiveResponseObject, error) {
	body, err := echo(request.Param)
	return gen.GetSimplePrimitive200TextResponse(body), err
}

func (s *Server) GetStartingWithNumber(ctx context.Context, request gen.GetStartingWithNumberRequestObject) (gen.GetStartingWithNumberResponseObject, error) {
	body, err := echo(request.N1param)
	return gen.GetStartingWithNumber200TextResponse(body), err
}
