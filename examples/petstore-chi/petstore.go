package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/FireTail-io/firetail-go-lib/examples/petstore-chi/api"
	firetail "github.com/FireTail-io/firetail-go-lib/middlewares/http"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt"
)

func main() {
	var port = flag.Int("port", 8080, "Port for test HTTP server")
	flag.Parse()
	log.Printf("Serving on port %d", *port)

	// Create a Chi router & use our validation middleware to check all
	// requests against the OpenAPI schema. We'll use debug mode for this
	// demo, so the error responses have extra information about exactly
	// what the middleware is doing.
	r := chi.NewRouter()
	r.Use(getFiretaiLMiddleware(true))

	petStore := api.NewPetStore()
	api.HandlerFromMux(petStore, r)

	s := &http.Server{
		Handler: r,
		Addr:    fmt.Sprintf("0.0.0.0:%d", *port),
	}

	log.Fatal(s.ListenAndServe())
}

func getFiretaiLMiddleware(debug bool) func(next http.Handler) http.Handler {
	firetailMiddleware, err := firetail.GetMiddleware(&firetail.Options{
		OpenapiSpecPath: "./petstore-expanded.yaml",
		DebugErrs:       debug,
		LogBatchCallback: func(b [][]byte) {
			// Here we could send the logs to a custom destination
			log.Println("Log batch:")
			for _, logBytes := range b {
				log.Println(prettyPrintJson(logBytes))
			}
		},
		AuthCallbacks: map[string]openapi3filter.AuthenticationFunc{
			"MyBearerAuth": func(ctx context.Context, ai *openapi3filter.AuthenticationInput) error {
				authHeaderValue := ai.RequestValidationInput.Request.Header.Get("Authorization")
				if authHeaderValue == "" {
					return errors.New("no bearer token supplied for \"MyBearerAuth\"")
				}

				tokenParts := strings.Split(authHeaderValue, " ")
				if len(tokenParts) != 2 || strings.ToUpper(tokenParts[0]) != "BEARER" {
					return errors.New("invalid value in Authorization header for \"MyBearerAuth\"")
				}

				token, err := jwt.Parse(tokenParts[1], func(token *jwt.Token) (interface{}, error) {
					if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
						return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
					}
					hmacSampleSecret := []byte(os.Getenv("JWT_SECRET_KEY"))
					return hmacSampleSecret, nil
				})
				if err != nil {
					return err
				} else if !token.Valid {
					return errors.New("invalid jwt supplied for \"MyBearerAuth\"")
				}

				return nil
			},
		},
	})
	if err != nil {
		panic(err)
	}
	return firetailMiddleware
}

func prettyPrintJson(jsonBytes []byte) string {
	var unmarshalledJson json.RawMessage
	err := json.Unmarshal([]byte(jsonBytes), &unmarshalledJson)
	if err != nil {
		panic(err)
	}
	prettyJson, err := json.MarshalIndent(unmarshalledJson, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(prettyJson)
}
