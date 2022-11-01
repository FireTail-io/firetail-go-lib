package firetail

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	firetailerrors "github.com/FireTail-io/firetail-go-lib/errors"
	firetailoptions "github.com/FireTail-io/firetail-go-lib/options"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/sbabiv/xml2map"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var healthHandler http.HandlerFunc = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte("{\"description\":\"test description\"}"))
})

var healthHandlerWithWrongResponseBody http.HandlerFunc = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte("{\"description\":\"another test description\"}"))
})

var healthHandlerWithWrongResponseCode http.HandlerFunc = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(201)
	w.Header().Add("Content-Type", "application/json")
	w.Write([]byte("{\"description\":\"another test description\"}"))
})

var authCallbacks = map[string]openapi3filter.AuthenticationFunc{
	"ApiKeyAuth1": func(ctx context.Context, ai *openapi3filter.AuthenticationInput) error {
		token := ai.RequestValidationInput.Request.Header.Get("X-Api-Key")
		if token != "valid-api-key" {
			return errors.New("invalid API key")
		}
		return nil
	},
	"ApiKeyAuth2": func(ctx context.Context, ai *openapi3filter.AuthenticationInput) error {
		token := ai.RequestValidationInput.Request.Header.Get("X-Api-Key")
		if token != "valid-api-key" {
			return errors.New("invalid API key")
		}
		return nil
	},
}

func TestValidRequestAndResponse(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
		AuthCallbacks:   authCallbacks,
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"test description\"}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Api-Key", "valid-api-key")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 200, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(t, "{\"description\":\"test description\"}", string(respBody))
}

func TestInvalidSpecPath(t *testing.T) {
	_, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec-not-here.yaml",
	})
	require.IsType(t, firetailerrors.ErrorInvalidConfiguration{}, err)
	require.Equal(t, "firetail - invalid configuration: open ./test-spec-not-here.yaml: no such file or directory", err.Error())
}

func TestInvalidSpec(t *testing.T) {
	_, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec-invalid.yaml",
	})
	require.IsType(t, firetailerrors.ErrorAppspecInvalid{}, err)
	require.Equal(t, "firetail - invalid appspec: invalid paths: a short description of the response is required", err.Error())
}

func TestRequestToInvalidRoute(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest("GET", "/not-implemented", nil)
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 404, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(t, "{\"code\":404,\"message\":\"firetail - no matching path found for \\\"/not-implemented\\\"\"}", string(respBody))
}

func TestRequestWithDisallowedMethod(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest("GET", "/implemented/1", nil)
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 405, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(t, "{\"code\":405,\"message\":\"firetail - \\\"/implemented/1\\\" path does not support GET method\"}", string(respBody))
}

func TestRequestWithInvalidHeader(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
		AuthCallbacks:   authCallbacks,
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"test description\"}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Api-Key", "valid-api-key")
	request.Header.Add("X-Test-Header", "invalid")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 400, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(
		t,
		"{\"code\":400,\"message\":\"firetail - request headers invalid: parameter \\\"X-Test-Header\\\" in header has an error: value invalid: an invalid number: invalid syntax\"}",
		string(respBody),
	)
}

func TestRequestWithInvalidQueryParam(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
		AuthCallbacks:   authCallbacks,
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1?test-param=invalid",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"test description\"}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Api-Key", "valid-api-key")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 400, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(
		t,
		"{\"code\":400,\"message\":\"firetail - request query parameter invalid: parameter \\\"test-param\\\" in query has an error: value invalid: an invalid number: invalid syntax\"}",
		string(respBody),
	)
}

func TestRequestWithInvalidPathParam(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
		AuthCallbacks:   authCallbacks,
	})
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/invalid-path-param",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"test description\"}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Api-Key", "valid-api-key")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 400, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(
		t,
		"{\"code\":400,\"message\":\"firetail - request path parameter invalid: parameter \\\"testparam\\\" in path has an error: value invalid-path-param: an invalid number: invalid syntax\"}",
		string(respBody),
	)
}

func TestRequestWithInvalidBody(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
		AuthCallbacks:   authCallbacks,
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("{}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Api-Key", "valid-api-key")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 400, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(
		t,
		"{\"code\":400,\"message\":\"firetail - request body invalid: request body has an error: doesn't match the schema:firetailerrors.Error at \\\"/description\\\": property \\\"description\\\" is missing\\nSchema:\\n  {\\n    \\\"additionalProperties\\\": false,\\n    \\\"properties\\\": {\\n      \\\"description\\\": {\\n        \\\"enum\\\": [\\n          \\\"test description\\\"\\n        ],\\n        \\\"type\\\": \\\"string\\\"\\n      }\\n    },\\n    \\\"required\\\": [\\n      \\\"description\\\"\\n    ],\\n    \\\"type\\\": \\\"object\\\"\\n  }\\n\\nValue:\\n  {}\\n\"}",
		string(respBody),
	)
}

func TestRequestWithValidAuth(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
		AuthCallbacks:   authCallbacks,
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"test description\"}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Api-Key", "valid-api-key")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 200, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(
		t,
		"{\"description\":\"test description\"}",
		string(respBody),
	)
}

func TestRequestWithUnimplementedAuth(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"test description\"}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Api-Key", "valid-api-key")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 401, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(
		t,
		"{\"code\":401,\"message\":\"firetail - request did not satisfy security requirements: Security requirements failed, errors: firetail - security scheme 'ApiKeyAuth1' has not been implemented in the application, firetail - security scheme 'ApiKeyAuth2' has not been implemented in the application\"}",
		string(respBody),
	)
}

func TestRequestWithMissingAuth(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
		AuthCallbacks:   authCallbacks,
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"test description\"}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 401, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(
		t,
		"{\"code\":401,\"message\":\"firetail - request did not satisfy security requirements: Security requirements failed, errors: invalid API key, invalid API key\"}",
		string(respBody),
	)
}

func TestRequestWithInvalidAuth(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
		AuthCallbacks:   authCallbacks,
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"test description\"}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Api-Key", "invalid-api-key")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 401, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(
		t,
		"{\"code\":401,\"message\":\"firetail - request did not satisfy security requirements: Security requirements failed, errors: invalid API key, invalid API key\"}",
		string(respBody),
	)
}

func TestInvalidResponseBody(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
		AuthCallbacks:   authCallbacks,
	})
	require.Nil(t, err)
	handler := middleware(healthHandlerWithWrongResponseBody)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"test description\"}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Api-Key", "valid-api-key")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 500, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(
		t,
		"{\"code\":500,\"message\":\"firetail - response body invalid: response body doesn't match the schema:firetailerrors.Error at \\\"/description\\\": value is not one of the allowed values\\nSchema:\\n  {\\n    \\\"enum\\\": [\\n      \\\"test description\\\"\\n    ],\\n    \\\"type\\\": \\\"string\\\"\\n  }\\n\\nValue:\\n  \\\"another test description\\\"\\n\"}",
		string(respBody),
	)
}

func TestInvalidResponseCode(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
		AuthCallbacks:   authCallbacks,
	})
	require.Nil(t, err)
	handler := middleware(healthHandlerWithWrongResponseCode)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"test description\"}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Api-Key", "valid-api-key")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 500, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(
		t,
		"{\"code\":500,\"message\":\"firetail - response status code invalid: 201\"}",
		string(respBody),
	)
}

func TestDisabledRequestValidation(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath:          "./test-spec.yaml",
		DisableRequestValidation: true,
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"Another test JSON object\"}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 200, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(t, "{\"description\":\"test description\"}", string(respBody))
}

func TestDisabledResponseValidation(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath:           "./test-spec.yaml",
		AuthCallbacks:             authCallbacks,
		DisableResponseValidation: true,
	})
	require.Nil(t, err)
	handler := middleware(healthHandlerWithWrongResponseBody)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"test description\"}"))),
	)
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-Api-Key", "valid-api-key")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 200, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(t, "{\"description\":\"another test description\"}", string(respBody))
}

func TestUnexpectedContentType(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
		AuthCallbacks:   authCallbacks,
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("{\"description\":\"test description\"}"))),
	)
	request.Header.Add("Content-Type", "text/plain")
	request.Header.Add("X-Api-Key", "valid-api-key")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 415, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(t, "{\"code\":415,\"message\":\"firetail - /implemented/{testparam} route does not support content type text/plain\"}", string(respBody))
}

func TestCustomXMLDecoder(t *testing.T) {
	middleware, err := GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "./test-spec.yaml",
		AuthCallbacks:   authCallbacks,
		CustomBodyDecoders: map[string]openapi3filter.BodyDecoder{
			"application/xml": func(r io.Reader, h http.Header, sr *openapi3.SchemaRef, ef openapi3filter.EncodingFn) (interface{}, error) {
				return xml2map.NewDecoder(r).Decode()
			},
		},
	})
	require.Nil(t, err)
	handler := middleware(healthHandler)
	responseRecorder := httptest.NewRecorder()

	request := httptest.NewRequest(
		"POST", "/implemented/1",
		io.NopCloser(bytes.NewBuffer([]byte("<description>test description</description>"))),
	)
	request.Header.Add("Content-Type", "application/xml")
	request.Header.Add("X-Api-Key", "valid-api-key")
	handler.ServeHTTP(responseRecorder, request)

	assert.Equal(t, 200, responseRecorder.Code)

	require.Contains(t, responseRecorder.HeaderMap, "Content-Type")
	require.GreaterOrEqual(t, len(responseRecorder.HeaderMap["Content-Type"]), 1)
	assert.Len(t, responseRecorder.HeaderMap["Content-Type"], 1)
	assert.Equal(t, "application/json", responseRecorder.HeaderMap["Content-Type"][0])

	respBody, err := io.ReadAll(responseRecorder.Body)
	require.Nil(t, err)
	assert.Equal(t, "{\"description\":\"test description\"}", string(respBody))
}
