package firetail

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/FireTail-io/firetail-go-lib/logging"
	"github.com/getkin/kin-openapi/openapi3filter"
)

// Options is an options struct used when creating a Firetail middleware (GetMiddleware)
type Options struct {
	// SpecPath is the path at which your openapi spec can be found. Supplying an empty string disables any validation.
	OpenapiSpecPath string

	// OpenapiBytes is the raw bytes of your openapi spec. Supplying an empty slice disables any validation. OpenapiBytes takes
	// precedence over OpenapiSpecPath if both are provided. OpenapiSpecPath will be used if OpenapiBytes is nil or len() == 0
	OpenapiBytes []byte

	// LogsApiToken is the API token which will be used when sending logs to the Firetail logging API with the default batch callback.
	// This value should typically be loaded in from an environment variable. If unset, the default batch callback will not forward
	// logs to the Firetail SaaS
	LogsApiToken string

	// LogsApiUrl is the URL of the Firetail logging API endpoint to which logs will be sent by the default batch callback. This value
	// should typically be loaded in from an environment variable. If unset, the default value is the Firetail SaaS' bulk logs endpoint
	// in the default region (firetail.app). If another region is being used, this option will need to be configured appropriately. For
	// example, for us.firetail.app LogsApiUrl should normally be https://api.logging.us-east-2.prod.firetail.app/logs/bulk
	LogsApiUrl string

	// LogBatchCallback is an optional callback which is provided with a batch of Firetail log entries ready to be sent to Firetail. The
	// default callback sends log entries to the Firetail logging API. It may be customised to, for example, additionally log the entries
	// to a file on disk
	LogBatchCallback func([][]byte)

	// MaxBatchSize is the maximum size of a logging batch in bytes which will be passed to the LogBatchCallback, or the default callback
	// if it is used.
	MaxBatchSize int

	// MaxLogAge is the maximum age of the oldest log in a batch which will be passed to the LogBatchCallback, or the default callback if
	// it is used.
	MaxLogAge time.Duration

	// ErrCallback is an optional callback func which is given an error and a ResponseWriter to which an apropriate response can be written
	// for the error. This allows you customise the responses given, when for example a request or response fails to validate against the
	// openapi spec, to be consistent with the format in which the rest of your application returns error responses
	ErrCallback func(ErrorAtRequest, http.ResponseWriter, *http.Request)

	// DebugErrs is a flag which, when set to true, will enable the default ErrCallback to send more verbose information in the RFC7807
	// error responses' `details` member.
	DebugErrs bool

	// AuthCallbacks is a map of strings, which should match the names of your appspec's securitySchemes, to callback funcs which must be
	// defined if you wish to use security schemas in your openapi specification. See the openapi3filter package's reference for further
	// documentation
	AuthCallbacks map[string]openapi3filter.AuthenticationFunc

	// EnableRequestValidation is an optional flag which, if set to true, enables request validation against the openapi spec provided -
	// if no openapi spec is provided, then no validation will be performed
	EnableRequestValidation bool

	// EnableResponseValidation is an optional flag which, if set to true, enables response validation against the openapi spec provided -
	// if no openapi spec is provided, then no validation will be performed
	EnableResponseValidation bool

	// AllowUndefinedRoutes is an optional flag which, if set to true, allows requests to routes which are not defined in the openapi spec
	// to pass through the middleware without validation. If set to false (default), requests to undefined routes will be rejected and a
	// 404 response will be returned
	AllowUndefinedRoutes bool

	// CustomBodyDecoders is a map of Content-Type header values to openapi3 decoders - if the kin-openapi module does not support your
	// Content-Type by default, you will need to add a custom decoder here
	CustomBodyDecoders map[string]openapi3filter.BodyDecoder

	// LogEntrySanitiser is a function used to sanitise the log entries sent to Firetail. You may wish to use this to redact sensitive
	// information, or anonymise identifiable information using a custom implementation of this callback for your application. A default
	// implementation is provided in the firetail logging package
	LogEntrySanitiser func(logging.LogEntry) logging.LogEntry
}

func (o *Options) setDefaults() {
	if o.LogsApiUrl == "" {
		o.LogsApiUrl = "https://api.logging.eu-west-1.prod.firetail.app/logs/bulk"
	}

	if o.ErrCallback == nil {
		o.ErrCallback = func(errAtRequest ErrorAtRequest, w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			type ErrorResponse struct {
				Code   int    `json:"code"`
				Title  string `json:"title"`
				Detail string `json:"detail,omitempty"`
			}
			errorResponse := ErrorResponse{
				Code:  errAtRequest.StatusCode(),
				Title: errAtRequest.Title(),
			}
			if o.DebugErrs {
				errorResponse.Detail = errAtRequest.Error()
			}
			responseBody, err := json.Marshal(errorResponse)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{
					"code": 500,
					"title": "internal server error"
				}`))
			}
			w.WriteHeader(errAtRequest.StatusCode())
			w.Write([]byte(responseBody))
		}
	}

	if o.LogEntrySanitiser == nil {
		o.LogEntrySanitiser = logging.DefaultSanitiser()
	}
}
