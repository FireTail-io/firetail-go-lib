package main

import (
	"net/http"
	"os"
	"time"

	firetail "github.com/FireTail-io/firetail-go-lib/middlewares/http"
)

func health(w http.ResponseWriter, r *http.Request) {
	// Firetail will log the execution time, let's pretend this endpoint takes about 50ms...
	time.Sleep(50 * time.Millisecond)

	// Firetail will also log the response code...
	w.WriteHeader(200)

	// Firetail will also capture response headers...
	w.Header().Add("Content-Type", "text/plain")

	// And, finally, it'll also log the response body...
	w.Write([]byte("I'm Healthy! :)"))
}

func main() {
	// Get a firetail middleware
	firetailMiddleware, err := firetail.GetMiddleware(&firetail.Options{
		OpenapiSpecPath: "app-spec.yaml",
		LogsApiToken:    os.Getenv("FIRETAIL_LOG_API_KEY"),
	})
	if err != nil {
		panic(err)
	}

	// We can setup our handler as usual, just wrap it in the firetailMiddleware :)
	healthHandler := firetailMiddleware(http.HandlerFunc(health))
	http.Handle("/health", healthHandler)

	// If you want the FireTail middleware to catch all requests, you need to register a 404 handler and wrap it. If you
	// don't have a 404 handler, you can use http.NotFoundHandler() from the stdlib
	http.Handle("/", firetailMiddleware(http.NotFoundHandler()))

	http.ListenAndServe(":8080", nil)
}
