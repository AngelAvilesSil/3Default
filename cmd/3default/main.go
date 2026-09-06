package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/config"
	"github.com/AngelAvilesSil/3Default/internal/database"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/httpapi"
	"github.com/AngelAvilesSil/3Default/internal/projects"
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

	queries := dbgen.New(db)
	projectService := projects.NewService(queries)

	handler := httpapi.NewHandler(
		httpapi.NewServer(
			db,
			projectService,
		),
	)

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
