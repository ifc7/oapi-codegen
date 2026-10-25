package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/oapi-codegen/oapi-codegen/v2/pkg/model"
)

func TestDedupeGoFieldNames(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  []goFieldRename
	}{
		{
			name:  "no collisions",
			names: []string{"StartTime", "EndTime"},
		},
		{
			name:  "later occurrences get increasing suffixes",
			names: []string{"StartTime", "StartTime", "StartTime"},
			want: []goFieldRename{
				{index: 1, firstIndex: 0, name: "StartTime", newName: "StartTime2"},
				{index: 2, firstIndex: 0, name: "StartTime", newName: "StartTime3"},
			},
		},
		{
			name:  "suffix skips names already in use",
			names: []string{"StartTime", "StartTime2", "StartTime"},
			want: []goFieldRename{
				{index: 2, firstIndex: 0, name: "StartTime", newName: "StartTime3"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, dedupeGoFieldNames(tc.names))
		})
	}
}

func TestGenerateParamsTypesDedupesFieldNames(t *testing.T) {
	prev := globalState
	t.Cleanup(func() { globalState = prev })
	globalState.warnedFieldNameCollisions = nil

	param := func(name string) ParameterDefinition {
		return ParameterDefinition{ParamName: name, In: "query", Spec: &model.Parameter{Name: name, In: "query"}, Schema: Schema{GoType: "string"}}
	}
	op := OperationDefinition{
		OperationId: "ListCall",
		QueryParams: []ParameterDefinition{param("StartTime"), param("StartTime<"), param("StartTime>")},
	}
	op.dedupeParamGoNames()

	assert.Equal(t, "StartTime", op.QueryParams[0].GoName())
	assert.Equal(t, "StartTime2", op.QueryParams[1].GoName())
	assert.Equal(t, "StartTime3", op.QueryParams[2].GoName())

	typeDefs := GenerateParamsTypes(op)
	goType := typeDefs[len(typeDefs)-1].Schema.GoType
	assert.Contains(t, goType, "StartTime2 *string`form:\"StartTime<,omitempty\" json:\"StartTime<,omitempty\"`")
	assert.Contains(t, goType, "StartTime3 *string`form:\"StartTime>,omitempty\" json:\"StartTime>,omitempty\"`")
}
