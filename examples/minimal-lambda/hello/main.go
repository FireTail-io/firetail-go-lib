package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"

	firetail "github.com/FireTail-io/firetail-go-lib/middlewares/api-gateway-v2"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(ctx context.Context, e events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	var buf bytes.Buffer

	body, err := json.Marshal(map[string]interface{}{
		"message": "Go Serverless v1.0! Your function executed successfully!",
	})
	if err != nil {
		return events.APIGatewayV2HTTPResponse{StatusCode: 404}, err
	}
	json.HTMLEscape(&buf, body)

	resp := events.APIGatewayV2HTTPResponse{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            buf.String(),
		Headers: map[string]string{
			"Content-Type":           "application/json",
			"X-MyCompany-Func-Reply": "hello-handler",
		},
	}

	return resp, nil
}

//go:embed app-spec.yaml
var oapispec string

func main() {
	middleware, err := firetail.GetMiddleware(&firetail.Options{
		OpenapiSpec: oapispec,
	})
	if err != nil {
		panic(err)
	}
	lambda.Start(middleware(firetail.HandlerFunc(Handler)))
}
