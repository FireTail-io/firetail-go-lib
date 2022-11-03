package firetail

import (
	"context"
	"log"
	"strings"
	"time"

	firetailerrors "github.com/FireTail-io/firetail-go-lib/errors"
	"github.com/FireTail-io/firetail-go-lib/logging"
	"github.com/aws/aws-lambda-go/events"
	"github.com/awslabs/aws-lambda-go-api-proxy/core"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

type HandlerFunc = func(context.Context, events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error)

func convertHeaders(resp *events.APIGatewayV2HTTPResponse) (headers map[string][]string) {
	for header, val := range resp.Headers {
		headers[header] = []string{val}
	}
	return headers
}

// GetMiddleware creates & returns a firetail middleware. Errs if the openapi spec can't be found, validated, or loaded into a gorillamux router.
func GetMiddleware(options *Options) (func(next HandlerFunc) HandlerFunc, error) {
	options.SetDefaults() // Fill in any defaults where apropriate

	// Load in our appspec, validate it & create a router from it.
	loader := &openapi3.Loader{Context: context.Background(), IsExternalRefsAllowed: true}
	doc, err := loader.LoadFromData([]byte(options.OpenapiSpec))
	if err != nil {
		return nil, firetailerrors.ErrorInvalidConfiguration{Err: err}
	}
	err = doc.Validate(context.Background())
	if err != nil {
		return nil, firetailerrors.ErrorAppspecInvalid{Err: err}
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, err
	}

	// Register any custom body decoders
	for contentType, bodyDecoder := range options.CustomBodyDecoders {
		openapi3filter.RegisterBodyDecoder(contentType, bodyDecoder)
	}

	// Create a batchLogger to pass all our log entries to
	batchLogger := logging.NewBatchLogger(logging.BatchLoggerOptions{
		MaxBatchSize:  1024 * 512,
		MaxLogAge:     time.Minute,
		BatchCallback: options.LogBatchCallback,
		LogApiKey:     options.LogApiKey,
		LogApiUrl:     options.LogApiUrl,
	})

	middleware := func(next HandlerFunc) HandlerFunc {
		return func(ctx context.Context, r events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
			// Create a LogEntry populated with everything we know right now
			logEntry := logging.LogEntry{
				Version:     logging.The100Alpha,
				DateCreated: time.Now().UnixMilli(),
				Request: logging.Request{
					HTTPProtocol: logging.HTTPProtocol(r.RequestContext.HTTP.Protocol),
					// Headers:      r.Headers,
					Method: logging.Method(r.RequestContext.HTTP.Method),
					IP:     r.RequestContext.HTTP.SourceIP,
					URI:    r.RequestContext.DomainName + r.RequestContext.HTTP.Path,
					Body:   r.Body,
				},
			}

			// Create a Firetail ResponseWriter so we can access the response body, status code etc. for logging & validation later
			var nextProxyResponsePtr *events.APIGatewayV2HTTPResponse
			var nextErr error

			// No matter what happens, read the response from the local response writer, enqueue the log entry & publish the response that was written to the ResponseWriter
			defer func() {
				if nextProxyResponsePtr != nil {
					logEntry.Response = logging.Response{
						StatusCode: int64(nextProxyResponsePtr.StatusCode),
						Body:       nextProxyResponsePtr.Body,
						Headers:    convertHeaders(nextProxyResponsePtr),
					}
				}

				// Remember to sanitise the log entry before enqueueing it!
				logEntry = options.LogEntrySanitiser(logEntry)

				batchLogger.Enqueue(&logEntry)
			}()

			httpRequest, err := (&core.RequestAccessor{}).EventToRequest(r)
			if err != nil {
				return options.ErrCallback(firetailerrors.ErrorAtRequestUnspecified{Err: err})
			}

			log.Println(r, httpRequest)

			// Check there's a corresponding route for this request
			route, pathParams, err := router.FindRoute(httpRequest)
			if err == routers.ErrMethodNotAllowed {
				return options.ErrCallback(firetailerrors.ErrorUnsupportedMethod{RequestedPath: r.RequestContext.HTTP.Path, RequestedMethod: r.RequestContext.HTTP.Method})

			} else if err == routers.ErrPathNotFound {
				return options.ErrCallback(firetailerrors.ErrorRouteNotFound{RequestedPath: r.RequestContext.HTTP.Path})
			} else if err != nil {
				return options.ErrCallback(firetailerrors.ErrorAtRequestUnspecified{Err: err})
			}

			// We now know the resource that was requested, so we can fill it into our log entry
			logEntry.Request.Resource = route.Path

			// If it hasn't been disabled, validate the request against the OpenAPI spec.
			if !options.DisableRequestValidation {
				requestValidationInput := &openapi3filter.RequestValidationInput{
					Request:    httpRequest,
					PathParams: pathParams,
					Route:      route,
					Options: &openapi3filter.Options{
						AuthenticationFunc: func(ctx context.Context, ai *openapi3filter.AuthenticationInput) error {
							authCallback, hasAuthCallback := options.AuthCallbacks[ai.SecuritySchemeName]
							if !hasAuthCallback {
								return firetailerrors.ErrorAuthSchemeNotImplemented{MissingScheme: ai.SecuritySchemeName}
							}
							return authCallback(ctx, ai)
						},
					},
				}
				err = openapi3filter.ValidateRequest(context.Background(), requestValidationInput)
				if err != nil {
					// If the err is an openapi3filter RequestError, we can extract more information from the err...
					if err, isRequestErr := err.(*openapi3filter.RequestError); isRequestErr {
						// TODO: Using strings.Contains is janky here and may break - should replace with something more reliable
						// See the following open issue on the kin-openapi repo: https://github.com/getkin/kin-openapi/issues/477
						// TODO: Open source contribution to kin-openapi?
						if strings.Contains(err.Reason, "header Content-Type has unexpected value") {
							return options.ErrCallback(firetailerrors.ErrorRequestContentTypeInvalid{RequestedContentType: r.Headers["Content-Type"], RequestedRoute: route.Path})
						}
						if strings.Contains(err.Error(), "body has an error") {
							return options.ErrCallback(firetailerrors.ErrorRequestBodyInvalid{Err: err})
						}
						if strings.Contains(err.Error(), "header has an error") {
							return options.ErrCallback(firetailerrors.ErrorRequestHeadersInvalid{Err: err})
						}
						if strings.Contains(err.Error(), "query has an error") {
							return options.ErrCallback(firetailerrors.ErrorRequestQueryParamsInvalid{Err: err})
						}
						if strings.Contains(err.Error(), "path has an error") {
							return options.ErrCallback(firetailerrors.ErrorRequestPathParamsInvalid{Err: err})
						}
					}

					// If the validation fails due to a security requirement, we pass a SecurityRequirementsError to the ErrCallback
					if err, isSecurityErr := err.(*openapi3filter.SecurityRequirementsError); isSecurityErr {
						return options.ErrCallback(firetailerrors.ErrorAuthNoMatchingScheme{Err: err})

					}

					// Else, we just use a non-specific ValidationError error
					return options.ErrCallback(firetailerrors.ErrorAtRequestUnspecified{Err: err})
				}
			}

			// Serve the next handler down the chain & take note of the execution time
			startTime := time.Now()
			nextProxyResponse, nextErr := next(ctx, r)
			logEntry.ExecutionTime = float64(time.Since(startTime).Milliseconds())
			if err == nil {
				nextProxyResponsePtr = &nextProxyResponse
			}

			// If it hasn't been disabled, validate the response against the openapi spec
			if !options.DisableResponseValidation {
				responseValidationInput := &openapi3filter.ResponseValidationInput{
					RequestValidationInput: &openapi3filter.RequestValidationInput{
						Request:    httpRequest,
						PathParams: pathParams,
						Route:      route,
					},
					Status: nextProxyResponse.StatusCode,
					Header: convertHeaders(nextProxyResponsePtr),
					Options: &openapi3filter.Options{
						IncludeResponseStatus: true,
					},
				}
				responseValidationInput.SetBodyBytes([]byte(nextProxyResponse.Body))
				err = openapi3filter.ValidateResponse(context.Background(), responseValidationInput)
				if err != nil {
					if responseError, isResponseError := err.(*openapi3filter.ResponseError); isResponseError {
						if responseError.Reason == "response body doesn't match the schema" {
							return options.ErrCallback(firetailerrors.ErrorResponseBodyInvalid{Err: responseError})
						} else if responseError.Reason == "status is not supported" {
							return options.ErrCallback(firetailerrors.ErrorResponseStatusCodeInvalid{RespondedStatusCode: responseError.Input.Status})
						}
					}
					return options.ErrCallback(firetailerrors.ErrorAtRequestUnspecified{Err: err})
				}
			}

			// If the response written down the chain passed all of the enabled validation, we can now return it
			return nextProxyResponse, nextErr
		}
	}

	return middleware, nil
}
