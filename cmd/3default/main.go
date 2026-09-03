package main

import (
	"log"
	"net/http"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/httpapi"
)

func main() {
	const address = ":8080"

	handler := httpapi.NewHandler(httpapi.NewServer())

	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("3Default listening on %s", address)

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
