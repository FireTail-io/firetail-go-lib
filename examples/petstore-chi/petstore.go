package main

import (
	"context"
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

	swagger, err := api.GetSwagger()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading swagger spec\n: %s", err)
		os.Exit(1)
	}

	// Clear out the servers array in the swagger spec, that skips validating
	// that server names match. We don't know how this thing will be run.
	swagger.Servers = nil

	// Create an instance of our handler which satisfies the generated interface
	petStore := api.NewPetStore()

	// This is how you set up a basic chi router
	r := chi.NewRouter()

	// Use our validation middleware to check all requests against the
	// OpenAPI schema.
	r.Use(getFiretaiLMiddleware())

	// We now register our petStore above as the handler for the interface
	api.HandlerFromMux(petStore, r)

	s := &http.Server{
		Handler: r,
		Addr:    fmt.Sprintf("0.0.0.0:%d", *port),
	}

	// And we serve HTTP until the world ends.
	log.Fatal(s.ListenAndServe())
}

func getFiretaiLMiddleware() func(next http.Handler) http.Handler {
	firetailMiddleware, err := firetail.GetMiddleware(&firetail.Options{
		OpenapiSpecPath: "./petstore-expanded.yaml",
		DebugErrs:       true,
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
