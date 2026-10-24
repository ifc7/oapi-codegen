package headerref

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/oapi-codegen/oapi-codegen/v2/internal/test/references/multipackage/header_ref/gen/api"
	"github.com/oapi-codegen/oapi-codegen/v2/internal/test/references/multipackage/header_ref/gen/common"
)

// TestExternalHeaderSchemaIsQualified asserts that a response header whose
// schema is an external $ref resolves to the imported package's type in both
// the server response headers struct and the client response wrapper.
// The assignments below only compile if the ETag fields are typed as
// *common.ETagSchema (the imported type), which is the regression guard for
// issue-2060. The rest of the generated package failing to compile would also
// catch a regression, since these files are part of the test module.
func TestExternalHeaderSchemaIsQualified(t *testing.T) {
	etag := common.ETagSchema("\"abc123\"")

	serverHeaders := api.GetThing200ResponseHeaders{ETag: &etag}
	assert.Equal(t, &etag, serverHeaders.ETag)

	clientResp := api.GetThingResponse{
		Headers200: &api.GetThingResponse200Headers{ETag: &etag},
	}
	assert.Equal(t, &etag, clientResp.Headers200.ETag)
}
