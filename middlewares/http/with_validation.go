package firetail

import (
	"context"
	"net/http"
	"time"

	"github.com/FireTail-io/firetail-go-lib/utils"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
)

func handleWithValidation(w *utils.ResponseWriter, r *http.Request, next http.Handler, route *routers.Route, pathParams map[string]string) (time.Duration, error) {
	// Validate the request against the OpenAPI spec. We'll also need the requestValidationInput again later when validating the response.
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: pathParams,
		Route:      route,
	}
	err := openapi3filter.ValidateRequest(context.Background(), requestValidationInput)
	if err != nil {
		return time.Duration(0), utils.ErrRequestValidationFailed
	}

	// Serve the next handler down the chain & take note of the execution time
	startTime := time.Now()
	next.ServeHTTP(w, r)
	executionTime := time.Since(startTime)

	// Validate the response against the openapi spec
	responseValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    r,
			PathParams: pathParams,
			Route:      route,
		},
		Status: w.StatusCode,
		Header: w.Header(),
		Options: &openapi3filter.Options{
			IncludeResponseStatus: true,
		},
	}
	responseValidationInput.SetBodyBytes(w.ResponseBody)
	err = openapi3filter.ValidateResponse(context.Background(), responseValidationInput)
	if err != nil {
		return time.Duration(0), utils.ErrResponseValidationFailed
	}

	return executionTime, nil
}