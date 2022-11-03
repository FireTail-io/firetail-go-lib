package firetail

import (
	"encoding/json"

	firetailerrors "github.com/FireTail-io/firetail-go-lib/errors"
	"github.com/FireTail-io/firetail-go-lib/logging"
	"github.com/aws/aws-lambda-go/events"
)

func (o *Options) SetDefaults() {
	if o.LogApiUrl == "" {
		o.LogApiUrl = "https://api.logging.eu-west-1.sandbox.firetail.app/logs/bulk"
	}

	if o.ErrCallback == nil {
		o.ErrCallback = func(errAtRequest firetailerrors.ErrorAtRequest) (events.APIGatewayV2HTTPResponse, error) {
			type ErrorResponse struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			responseBody, err := json.Marshal(ErrorResponse{
				Code:    errAtRequest.StatusCode(),
				Message: errAtRequest.Error(),
			})
			if err != nil {
				return events.APIGatewayV2HTTPResponse{}, err
			}
			return events.APIGatewayV2HTTPResponse{
				StatusCode: errAtRequest.StatusCode(),
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body:            string(responseBody),
				IsBase64Encoded: false,
			}, nil
		}
	}

	if o.LogEntrySanitiser == nil {
		o.LogEntrySanitiser = logging.DefaultSanitiser()
	}
}
