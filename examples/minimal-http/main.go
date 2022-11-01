package main

import (
	"net/http"
	"os"
	"time"

	firetail "github.com/FireTail-io/firetail-go-lib/middlewares/http"
	firetailoptions "github.com/FireTail-io/firetail-go-lib/options"
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
	firetailMiddleware, err := firetail.GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "app-spec.yaml",
		LogApiKey:       os.Getenv("FIRETAIL_LOG_API_KEY"),
	})
	if err != nil {
		panic(err)
	}

	// We can setup our handler as usual, just wrap it in the firetailMiddleware :)
	healthHandler := firetailMiddleware(http.HandlerFunc(health))
	http.Handle("/health", healthHandler)

	http.ListenAndServe(":8080", nil)
}
