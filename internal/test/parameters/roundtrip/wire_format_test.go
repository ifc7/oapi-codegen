package parametersroundtrip_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/oapi-codegen/testutil"
	"github.com/stretchr/testify/assert"

	. "github.com/oapi-codegen/oapi-codegen/v2/internal/test/parameters/roundtrip/stdhttp/gen"
)

var _ StrictServerInterface = (*testServer)(nil)

type testServer struct {
	array                []int32
	object               *Object
	complexObject        *ComplexObject
	passThrough          *string
	n1param              *string
	primitive            *int32
	primitiveString      *string
	cookieParams         *GetCookieParams
	queryParams          *GetQueryFormParams
	queryDelimitedParams *GetQueryDelimitedParams
	headerParams         *GetHeaderParams
}

func (t *testServer) reset() {
	t.array = nil
	t.object = nil
	t.complexObject = nil
	t.passThrough = nil
	t.n1param = nil
	t.primitive = nil
	t.primitiveString = nil
	t.cookieParams = nil
	t.queryParams = nil
	t.queryDelimitedParams = nil
	t.headerParams = nil
}

// (GET /contentObject/{param})
func (t *testServer) GetContentObject(ctx context.Context, request GetContentObjectRequestObject) (GetContentObjectResponseObject, error) {
	t.complexObject = &request.Param
	return GetContentObject200TextResponse(""), nil
}

// (GET /labelExplodeArray/{.param*})
func (t *testServer) GetLabelExplodeArray(ctx context.Context, request GetLabelExplodeArrayRequestObject) (GetLabelExplodeArrayResponseObject, error) {
	t.array = request.Param
	return GetLabelExplodeArray200TextResponse(""), nil
}

// (GET /labelExplodeObject/{.param*})
func (t *testServer) GetLabelExplodeObject(ctx context.Context, request GetLabelExplodeObjectRequestObject) (GetLabelExplodeObjectResponseObject, error) {
	t.object = &request.Param
	return GetLabelExplodeObject200TextResponse(""), nil
}

// (GET /labelNoExplodeArray/{.param})
func (t *testServer) GetLabelNoExplodeArray(ctx context.Context, request GetLabelNoExplodeArrayRequestObject) (GetLabelNoExplodeArrayResponseObject, error) {
	t.array = request.Param
	return GetLabelNoExplodeArray200TextResponse(""), nil
}

// (GET /labelNoExplodeObject/{.param})
func (t *testServer) GetLabelNoExplodeObject(ctx context.Context, request GetLabelNoExplodeObjectRequestObject) (GetLabelNoExplodeObjectResponseObject, error) {
	t.object = &request.Param
	return GetLabelNoExplodeObject200TextResponse(""), nil
}

// (GET /matrixExplodeArray/{.param*})
func (t *testServer) GetMatrixExplodeArray(ctx context.Context, request GetMatrixExplodeArrayRequestObject) (GetMatrixExplodeArrayResponseObject, error) {
	t.array = request.Id
	return GetMatrixExplodeArray200TextResponse(""), nil
}

// (GET /matrixExplodeObject/{.param*})
func (t *testServer) GetMatrixExplodeObject(ctx context.Context, request GetMatrixExplodeObjectRequestObject) (GetMatrixExplodeObjectResponseObject, error) {
	t.object = &request.Id
	return GetMatrixExplodeObject200TextResponse(""), nil
}

// (GET /matrixNoExplodeArray/{.param})
func (t *testServer) GetMatrixNoExplodeArray(ctx context.Context, request GetMatrixNoExplodeArrayRequestObject) (GetMatrixNoExplodeArrayResponseObject, error) {
	t.array = request.Id
	return GetMatrixNoExplodeArray200TextResponse(""), nil
}

// (GET /matrixNoExplodeObject/{.param})
func (t *testServer) GetMatrixNoExplodeObject(ctx context.Context, request GetMatrixNoExplodeObjectRequestObject) (GetMatrixNoExplodeObjectResponseObject, error) {
	t.object = &request.Id
	return GetMatrixNoExplodeObject200TextResponse(""), nil
}

// (GET /simpleExplodeArray/{param*})
func (t *testServer) GetSimpleExplodeArray(ctx context.Context, request GetSimpleExplodeArrayRequestObject) (GetSimpleExplodeArrayResponseObject, error) {
	t.array = request.Param
	return GetSimpleExplodeArray200TextResponse(""), nil
}

// (GET /simpleExplodeObject/{param*})
func (t *testServer) GetSimpleExplodeObject(ctx context.Context, request GetSimpleExplodeObjectRequestObject) (GetSimpleExplodeObjectResponseObject, error) {
	t.object = &request.Param
	return GetSimpleExplodeObject200TextResponse(""), nil
}

// (GET /simpleNoExplodeArray/{param})
func (t *testServer) GetSimpleNoExplodeArray(ctx context.Context, request GetSimpleNoExplodeArrayRequestObject) (GetSimpleNoExplodeArrayResponseObject, error) {
	t.array = request.Param
	return GetSimpleNoExplodeArray200TextResponse(""), nil
}

// (GET /simpleNoExplodeObject/{param})
func (t *testServer) GetSimpleNoExplodeObject(ctx context.Context, request GetSimpleNoExplodeObjectRequestObject) (GetSimpleNoExplodeObjectResponseObject, error) {
	t.object = &request.Param
	return GetSimpleNoExplodeObject200TextResponse(""), nil
}

// (GET /passThrough/{param})
func (t *testServer) GetPassThrough(ctx context.Context, request GetPassThroughRequestObject) (GetPassThroughResponseObject, error) {
	t.passThrough = &request.Param
	return GetPassThrough200TextResponse(""), nil
}

// (GET /startingWithjNumber/{param})
func (t *testServer) GetStartingWithNumber(ctx context.Context, request GetStartingWithNumberRequestObject) (GetStartingWithNumberResponseObject, error) {
	t.n1param = &request.N1param
	return GetStartingWithNumber200TextResponse(""), nil
}

// (GET /queryDeepObject)
func (t *testServer) GetDeepObject(ctx context.Context, request GetDeepObjectRequestObject) (GetDeepObjectResponseObject, error) {
	t.complexObject = &request.Params.DeepObj
	return GetDeepObjectdefaultTextResponse{StatusCode: http.StatusOK}, nil
}

// (GET /simplePrimitive/{param})
func (t *testServer) GetSimplePrimitive(ctx context.Context, request GetSimplePrimitiveRequestObject) (GetSimplePrimitiveResponseObject, error) {
	t.primitive = &request.Param
	return GetSimplePrimitive200TextResponse(""), nil
}

// (GET /simpleString/{param})
func (t *testServer) GetSimpleString(ctx context.Context, request GetSimpleStringRequestObject) (GetSimpleStringResponseObject, error) {
	return GetSimpleString200TextResponse(""), nil
}

// (GET /simpleExplodePrimitive/{param})
func (t *testServer) GetSimpleExplodePrimitive(ctx context.Context, request GetSimpleExplodePrimitiveRequestObject) (GetSimpleExplodePrimitiveResponseObject, error) {
	t.primitive = &request.Param
	return GetSimpleExplodePrimitive200TextResponse(""), nil
}

// (GET /labelPrimitive/{.param})
func (t *testServer) GetLabelPrimitive(ctx context.Context, request GetLabelPrimitiveRequestObject) (GetLabelPrimitiveResponseObject, error) {
	t.primitive = &request.Param
	return GetLabelPrimitive200TextResponse(""), nil
}

// (GET /labelExplodePrimitive/{.param*})
func (t *testServer) GetLabelExplodePrimitive(ctx context.Context, request GetLabelExplodePrimitiveRequestObject) (GetLabelExplodePrimitiveResponseObject, error) {
	t.primitive = &request.Param
	return GetLabelExplodePrimitive200TextResponse(""), nil
}

// (GET /matrixPrimitive/{;id})
func (t *testServer) GetMatrixPrimitive(ctx context.Context, request GetMatrixPrimitiveRequestObject) (GetMatrixPrimitiveResponseObject, error) {
	t.primitive = &request.Id
	return GetMatrixPrimitive200TextResponse(""), nil
}

// (GET /matrixExplodePrimitive/{;id*})
func (t *testServer) GetMatrixExplodePrimitive(ctx context.Context, request GetMatrixExplodePrimitiveRequestObject) (GetMatrixExplodePrimitiveResponseObject, error) {
	t.primitive = &request.Id
	return GetMatrixExplodePrimitive200TextResponse(""), nil
}

// (GET /queryDelimited)
func (t *testServer) GetQueryDelimited(ctx context.Context, request GetQueryDelimitedRequestObject) (GetQueryDelimitedResponseObject, error) {
	t.queryDelimitedParams = &request.Params
	if request.Params.Sa != nil {
		t.array = *request.Params.Sa
	}
	if request.Params.Pa != nil {
		t.array = *request.Params.Pa
	}
	return GetQueryDelimited200TextResponse(""), nil
}

// (GET /queryForm)
func (t *testServer) GetQueryForm(ctx context.Context, request GetQueryFormRequestObject) (GetQueryFormResponseObject, error) {
	t.queryParams = &request.Params
	if request.Params.Ea != nil {
		t.array = *request.Params.Ea
	}
	if request.Params.A != nil {
		t.array = *request.Params.A
	}
	if request.Params.Eo != nil {
		t.object = request.Params.Eo
	}
	if request.Params.O != nil {
		t.object = request.Params.O
	}
	if request.Params.P != nil {
		t.primitive = request.Params.P
	}
	if request.Params.Ps != nil {
		t.primitiveString = request.Params.Ps
	}
	if request.Params.Ep != nil {
		t.primitive = request.Params.Ep
	}
	if request.Params.Co != nil {
		t.complexObject = request.Params.Co
	}
	if request.Params.N1s != nil {
		t.n1param = request.Params.N1s
	}
	return GetQueryForm200TextResponse(""), nil
}

// (GET /header)
func (t *testServer) GetHeader(ctx context.Context, request GetHeaderRequestObject) (GetHeaderResponseObject, error) {
	t.headerParams = &request.Params
	if request.Params.XPrimitive != nil {
		t.primitive = request.Params.XPrimitive
	}
	if request.Params.XPrimitiveExploded != nil {
		t.primitive = request.Params.XPrimitiveExploded
	}
	if request.Params.XArray != nil {
		t.array = *request.Params.XArray
	}
	if request.Params.XArrayExploded != nil {
		t.array = *request.Params.XArrayExploded
	}
	if request.Params.XObject != nil {
		t.object = request.Params.XObject
	}
	if request.Params.XObjectExploded != nil {
		t.object = request.Params.XObjectExploded
	}
	if request.Params.XComplexObject != nil {
		t.complexObject = request.Params.XComplexObject
	}
	if request.Params.N1StartingWithNumber != nil {
		t.n1param = request.Params.N1StartingWithNumber
	}
	return GetHeaderdefaultTextResponse{StatusCode: http.StatusOK}, nil
}

// (GET /cookie)
func (t *testServer) GetCookie(ctx context.Context, request GetCookieRequestObject) (GetCookieResponseObject, error) {
	t.cookieParams = &request.Params
	if request.Params.Ea != nil {
		t.array = *request.Params.Ea
	}
	if request.Params.A != nil {
		t.array = *request.Params.A
	}
	if request.Params.Eo != nil {
		t.object = request.Params.Eo
	}
	if request.Params.O != nil {
		t.object = request.Params.O
	}
	if request.Params.P != nil {
		t.primitive = request.Params.P
	}
	if request.Params.Ep != nil {
		t.primitive = request.Params.Ep
	}
	if request.Params.Co != nil {
		t.complexObject = request.Params.Co
	}
	if request.Params.N1s != nil {
		t.n1param = request.Params.N1s
	}
	return GetCookiedefaultTextResponse{StatusCode: http.StatusOK}, nil
}

// (GET /enums)
func (t *testServer) EnumParams(ctx context.Context, request EnumParamsRequestObject) (EnumParamsResponseObject, error) {
	return EnumParams204Response{}, nil
}

func TestParameterBinding(t *testing.T) {
	var ts testServer
	e := Handler(NewStrictHandler(&ts, nil))

	expectedObject := Object{
		FirstName: "Alex",
		Role:      "admin",
	}

	expectedComplexObject := ComplexObject{
		Object:  expectedObject,
		Id:      12345,
		IsAdmin: true,
	}

	expectedArray := []int32{3, 4, 5}

	var expectedPrimitive int32 = 5

	var expectedPrimitiveString = "123;456"

	var expectedN1Param = "foo"

	// Check the passthrough case
	//  (GET /passThrough/{param})
	result := testutil.NewRequest().Get("/passThrough/some%20string").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.NotNil(t, ts.passThrough)
	assert.EqualValues(t, "some string", *ts.passThrough)
	ts.reset()

	// Check JSON marshaling of Content based parameter
	//  (GET /contentObject/{param})
	marshaledComplexObject, err := json.Marshal(expectedComplexObject)
	assert.NoError(t, err)
	q := fmt.Sprintf("/contentObject/%s", string(marshaledComplexObject))
	result = testutil.NewRequest().Get(q).GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedComplexObject, ts.complexObject)
	ts.reset()

	//  (GET /labelExplodeArray/{.param*})
	result = testutil.NewRequest().Get("/labelExplodeArray/.3.4.5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, expectedArray, ts.array)
	ts.reset()

	//  (GET /labelExplodeObject/{.param*})
	result = testutil.NewRequest().Get("/labelExplodeObject/.role=admin.firstName=Alex").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedObject, ts.object)
	ts.reset()

	//  (GET /labelNoExplodeArray/{.param})
	result = testutil.NewRequest().Get("/labelNoExplodeArray/.3,4,5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, expectedArray, ts.array)
	ts.reset()

	//  (GET /labelNoExplodeObject/{.param})
	result = testutil.NewRequest().Get("/labelNoExplodeObject/.role,admin,firstName,Alex").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedObject, ts.object)
	ts.reset()

	//  (GET /matrixExplodeArray/{.param*})
	uri := "/matrixExplodeArray/;id=3;id=4;id=5"
	result = testutil.NewRequest().Get(uri).GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, expectedArray, ts.array)
	ts.reset()

	//  (GET /matrixExplodeObject/{.param*})
	uri = "/matrixExplodeObject/;role=admin;firstName=Alex"
	result = testutil.NewRequest().Get(uri).GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedObject, ts.object)
	ts.reset()

	//  (GET /matrixNoExplodeArray/{.param})
	result = testutil.NewRequest().Get("/matrixNoExplodeArray/;id=3,4,5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, expectedArray, ts.array)
	ts.reset()

	//  (GET /matrixNoExplodeObject/{.param})
	result = testutil.NewRequest().Get("/matrixNoExplodeObject/;id=role,admin,firstName,Alex").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedObject, ts.object)
	ts.reset()

	//  (GET /simpleExplodeArray/{param*})
	result = testutil.NewRequest().Get("/simpleExplodeArray/3,4,5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, expectedArray, ts.array)
	ts.reset()

	//  (GET /simpleExplodeObject/{param*})
	result = testutil.NewRequest().Get("/simpleExplodeObject/role=admin,firstName=Alex").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedObject, ts.object)
	ts.reset()

	//  (GET /simpleNoExplodeArray/{param})
	result = testutil.NewRequest().Get("/simpleNoExplodeArray/3,4,5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, expectedArray, ts.array)
	ts.reset()

	//  (GET /simpleNoExplodeObject/{param})
	result = testutil.NewRequest().Get("/simpleNoExplodeObject/role,admin,firstName,Alex").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedObject, ts.object)
	ts.reset()

	//  (GET /simplePrimitive/{param})
	result = testutil.NewRequest().Get("/simplePrimitive/5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitive, ts.primitive)
	ts.reset()

	//  (GET /startingWithNumber/{1param})
	result = testutil.NewRequest().Get("/startingWithNumber/foo").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedN1Param, ts.n1param)
	ts.reset()

	//  (GET /simpleExplodePrimitive/{param})
	result = testutil.NewRequest().Get("/simpleExplodePrimitive/5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitive, ts.primitive)
	ts.reset()

	//  (GET /labelPrimitive/{.param})
	result = testutil.NewRequest().Get("/labelPrimitive/.5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitive, ts.primitive)
	ts.reset()

	//  (GET /labelExplodePrimitive/{.param*})
	result = testutil.NewRequest().Get("/labelExplodePrimitive/.5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitive, ts.primitive)
	ts.reset()

	//  (GET /matrixPrimitive/{;id})
	result = testutil.NewRequest().Get("/matrixPrimitive/;id=5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitive, ts.primitive)
	ts.reset()

	//  (GET /matrixExplodePrimitive/{;id*})
	result = testutil.NewRequest().Get("/matrixExplodePrimitive/;id=5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitive, ts.primitive)
	ts.reset()

	// ---------------------- Test Form Query Parameters ----------------------
	//  (GET /queryForm)

	// unexploded array
	result = testutil.NewRequest().Get("/queryForm?a=3,4,5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, expectedArray, ts.array)
	ts.reset()

	// exploded array
	result = testutil.NewRequest().Get("/queryForm?ea=3&ea=4&ea=5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, expectedArray, ts.array)
	ts.reset()

	// unexploded object
	result = testutil.NewRequest().Get("/queryForm?o=role,admin,firstName,Alex").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedObject, ts.object)
	ts.reset()

	// exploded object
	result = testutil.NewRequest().Get("/queryForm?role=admin&firstName=Alex").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedObject, ts.object)
	ts.reset()

	// exploded primitive
	result = testutil.NewRequest().Get("/queryForm?ep=5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitive, ts.primitive)
	ts.reset()

	// unexploded primitive
	result = testutil.NewRequest().Get("/queryForm?p=5").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitive, ts.primitive)
	ts.reset()

	// primitive string within reserved char, i.e., ';' escaped to '%3B'
	result = testutil.NewRequest().Get("/queryForm?ps=123%3B456").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitiveString, ts.primitiveString)
	ts.reset()

	// complex object
	q = fmt.Sprintf("/queryForm?co=%s", string(marshaledComplexObject))
	result = testutil.NewRequest().Get(q).GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedComplexObject, ts.complexObject)
	ts.reset()

	// starting with number
	result = testutil.NewRequest().Get("/queryForm?1s=foo").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedN1Param, ts.n1param)
	ts.reset()

	// -------------------- Test Delimited Query Parameters --------------------
	//  (GET /queryDelimited)

	t.Run("spaceDelimited array", func(t *testing.T) {
		result = testutil.NewRequest().Get("/queryDelimited?sa=3%204%205").GoWithHTTPHandler(t, e)
		assert.Equal(t, http.StatusOK, result.Code())
		assert.EqualValues(t, expectedArray, ts.array)
		ts.reset()
	})

	t.Run("pipeDelimited array", func(t *testing.T) {
		result = testutil.NewRequest().Get("/queryDelimited?pa=3|4|5").GoWithHTTPHandler(t, e)
		assert.Equal(t, http.StatusOK, result.Code())
		assert.EqualValues(t, expectedArray, ts.array)
		ts.reset()
	})

	// complex object via deepObject
	do := `deepObj[Id]=12345&deepObj[IsAdmin]=true&deepObj[Object][firstName]=Alex&deepObj[Object][role]=admin`
	q = "/queryDeepObject?" + do
	result = testutil.NewRequest().Get(q).GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedComplexObject, ts.complexObject)
	ts.reset()

	// ---------------------- Test Header Query Parameters --------------------

	// unexploded header primitive.
	result = testutil.NewRequest().WithHeader("X-Primitive", "5").Get("/header").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitive, ts.primitive)
	ts.reset()

	// exploded header primitive.
	result = testutil.NewRequest().WithHeader("X-Primitive-Exploded", "5").Get("/header").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitive, ts.primitive)
	ts.reset()

	// unexploded header array
	result = testutil.NewRequest().WithHeader("X-Array", "3,4,5").Get("/header").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, expectedArray, ts.array)
	ts.reset()

	// exploded header array
	result = testutil.NewRequest().WithHeader("X-Array-Exploded", "3,4,5").Get("/header").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, expectedArray, ts.array)
	ts.reset()

	// unexploded header object
	result = testutil.NewRequest().WithHeader("X-Object",
		"role,admin,firstName,Alex").Get("/header").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedObject, ts.object)
	ts.reset()

	// exploded header object
	result = testutil.NewRequest().WithHeader("X-Object-Exploded",
		"role=admin,firstName=Alex").Get("/header").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedObject, ts.object)
	ts.reset()

	// complex object
	result = testutil.NewRequest().WithHeader("X-Complex-Object",
		string(marshaledComplexObject)).Get("/header").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedComplexObject, ts.complexObject)
	ts.reset()

	// starting with number
	result = testutil.NewRequest().WithHeader("1-Starting-With-Number",
		"foo").Get("/header").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedN1Param, ts.n1param)
	ts.reset()

	// ------------------------- Test Cookie Parameters ------------------------
	result = testutil.NewRequest().WithCookieNameValue("p", "5").Get("/cookie").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitive, ts.primitive)
	ts.reset()

	result = testutil.NewRequest().WithCookieNameValue("ep", "5").Get("/cookie").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedPrimitive, ts.primitive)
	ts.reset()

	result = testutil.NewRequest().WithCookieNameValue("a", "3,4,5").Get("/cookie").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, expectedArray, ts.array)
	ts.reset()

	result = testutil.NewRequest().WithCookieNameValue(
		"o", "role,admin,firstName,Alex").Get("/cookie").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedObject, ts.object)
	ts.reset()

	result = testutil.NewRequest().WithCookieNameValue("1s", "foo").Get("/cookie").GoWithHTTPHandler(t, e)
	assert.Equal(t, http.StatusOK, result.Code())
	assert.EqualValues(t, &expectedN1Param, ts.n1param)
	ts.reset()
}

// TestClientPathParams, TestClientQueryParams, and the doRequest helper have been
// superseded by the multi-router TestParameterRoundTrip in param_roundtrip_test.go.
