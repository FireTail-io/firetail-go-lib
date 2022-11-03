package main

import (
	"net/http"
	"os"

	firetail "github.com/FireTail-io/firetail-go-lib/middlewares/http"
	firetailoptions "github.com/FireTail-io/firetail-go-lib/options"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	firetailMiddleware, err := firetail.GetMiddleware(&firetailoptions.Options{
		OpenapiSpecPath: "app-spec.yaml",
		LogApiKey:       os.Getenv("FIRETAIL_LOG_API_KEY"),
	})
	if err != nil {
		panic(err)
	}

	r := chi.NewRouter()
	r.Use(firetailMiddleware)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "text/plain")
		w.Write([]byte("I'm Healthy! :)"))
	})

	http.ListenAndServe(":3333", r)
}
