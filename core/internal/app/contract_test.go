package app

// Contract tests: every response the harness produces on the management
// surface is validated against api/openapi.yaml with kin-openapi. A handler
// whose status code or response body drifts from the contract fails the test
// that exercised it. The Agent surface is excluded by design; its contract is
// api/agent-envelope/envelope-v1.md.

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

const openapiPath = "../../../api/openapi.yaml"

type contractValidator struct {
	router routers.Router
	spec   *openapi3.T
}

func newContractValidator(t *testing.T) *contractValidator {
	t.Helper()
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromFile(openapiPath)
	if err != nil {
		t.Fatalf("load %s: %v", openapiPath, err)
	}
	if err := spec.Validate(loader.Context); err != nil {
		t.Fatalf("openapi.yaml is invalid: %v", err)
	}
	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatalf("build router from %s: %v", openapiPath, err)
	}
	return &contractValidator{router: router, spec: spec}
}

// validate checks one response against the contract. Paths outside the spec
// (the Agent surface, unknown routes) are skipped; anything documented must
// match status and body exactly.
func (cv *contractValidator) validate(t *testing.T, method, path string,
	status int, header http.Header, body []byte) {
	t.Helper()
	u, err := url.Parse("http://core.test" + path)
	if err != nil {
		t.Errorf("contract: parse %q: %v", path, err)
		return
	}
	req := &http.Request{Method: method, URL: u}
	route, pathParams, err := cv.router.FindRoute(req)
	if err != nil {
		return // not part of the contract surface
	}
	if route.Operation.Responses.Status(status) == nil {
		t.Errorf("contract: %s %s -> %d is not documented in api/openapi.yaml", method, path, status)
		return
	}
	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    &http.Request{Method: method, URL: u},
			PathParams: pathParams,
			Route:      route,
		},
		Status: status,
		Header: header,
		Body:   io.NopCloser(bytes.NewReader(body)),
		Options: &openapi3filter.Options{
			MultiError: true,
		},
	}
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		t.Errorf("contract violation: %s %s -> %d: %v\nbody: %s",
			method, path, status, err, body)
	}
}
