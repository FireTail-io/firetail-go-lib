package firetailoptions

import (
	"encoding/json"
	"net/http"

	firetailerrors "github.com/FireTail-io/firetail-go-lib/errors"
	"github.com/FireTail-io/firetail-go-lib/logging"
)

func (o *Options) SetDefaults() {
	if o.LogApiUrl == "" {
		o.LogApiUrl = "https://api.logging.eu-west-1.sandbox.firetail.app/logs/bulk"
	}

	if o.ErrCallback == nil {
		o.ErrCallback = func(errAtRequest firetailerrors.ErrorAtRequest, w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Content-Type", "application/json")
			type ErrorResponse struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			responseBody, err := json.Marshal(ErrorResponse{
				Code:    errAtRequest.StatusCode(),
				Message: errAtRequest.Error(),
			})
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("{\"code\":500,\"message\":\"internal server error\"}"))
			}
			w.WriteHeader(errAtRequest.StatusCode())
			w.Write([]byte(responseBody))
		}
	}

	if o.LogEntrySanitiser == nil {
		o.LogEntrySanitiser = logging.DefaultSanitiser()
	}
}
