package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/config"
	"github.com/AngelAvilesSil/3Default/internal/database"
	"github.com/AngelAvilesSil/3Default/internal/httpapi"
)

func main() {
	const address = ":8080"

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	db, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	handler := httpapi.NewHandler(httpapi.NewServer(db))

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
