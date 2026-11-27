// Package requestvalidator is a small net/http middleware that validates
// incoming requests against an OpenAPI document, using libopenapi-validator.
package requestvalidator

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/pb33f/libopenapi"
	validator "github.com/pb33f/libopenapi-validator"
	"github.com/pb33f/libopenapi-validator/config"
	"github.com/pb33f/libopenapi-validator/helpers"
)

// New loads the OpenAPI document at specPath and returns a middleware that
// rejects requests which do not match it. Requests that fail security
// validation get a 401, all other failures a 400. When authenticate is not nil
// it is called for every security scheme a request has to satisfy.
func New(specPath string, authenticate config.AuthenticationFunc) (func(http.Handler) http.Handler, error) {
	raw, err := os.ReadFile(specPath)
	if err != nil {
		return nil, fmt.Errorf("reading spec: %w", err)
	}
	doc, err := libopenapi.NewDocument(raw)
	if err != nil {
		return nil, fmt.Errorf("parsing spec: %w", err)
	}

	var opts []config.Option
	if authenticate != nil {
		opts = append(opts, config.WithAuthenticationFunc(authenticate))
	} else {
		opts = append(opts, config.WithoutSecurityValidation())
	}
	v, errs := validator.NewValidator(doc, opts...)
	if len(errs) > 0 {
		return nil, fmt.Errorf("building validator: %w", errs[0])
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ok, failures := v.ValidateHttpRequest(r)
			if ok {
				next.ServeHTTP(w, r)
				return
			}
			status := http.StatusBadRequest
			messages := make([]string, 0, len(failures))
			for _, f := range failures {
				if f.ValidationType == helpers.SecurityValidation {
					status = http.StatusUnauthorized
				}
				messages = append(messages, f.Message)
			}
			http.Error(w, strings.Join(messages, "; "), status)
		})
	}, nil
}
