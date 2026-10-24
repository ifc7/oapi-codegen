package main

import (
	"log"
	"net/http"

	"github.com/oapi-codegen/oapi-codegen/v2/examples/overlay/api"
)

func main() {
	// create a type that satisfies the `api.StrictServerInterface`, which contains an implementation of every operation from the generated code
	server := api.NewServer()

	// get an `http.Handler` that we can use
	h := api.Handler(api.NewStrictHandler(server, nil))

	s := &http.Server{
		Handler: h,
		Addr:    "0.0.0.0:8080",
	}

	// And we serve HTTP until the world ends.
	log.Fatal(s.ListenAndServe())
}
