package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	assert.NoError(t, Validate(""))
	assert.NoError(t, Validate(Kin))
	assert.NoError(t, Validate(LibOpenAPI))
	assert.EqualError(t, Validate("swagger"), `unknown engine "swagger": must be one of kin, libopenapi`)
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name       string
		env        string
		configured string
		want       string
		wantErr    string
	}{
		{name: "default", want: Default},
		{name: "configured", configured: LibOpenAPI, want: LibOpenAPI},
		{name: "env overrides configured", env: LibOpenAPI, configured: Kin, want: LibOpenAPI},
		{name: "env without configured", env: Kin, want: Kin},
		{name: "unknown configured", configured: "swagger", wantErr: `unknown engine "swagger": must be one of kin, libopenapi`},
		{name: "unknown env", env: "swagger", configured: Kin, wantErr: `OAPI_CODEGEN_ENGINE: unknown engine "swagger": must be one of kin, libopenapi`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvVar, tt.env)
			got, err := Resolve(tt.configured)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
