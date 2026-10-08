package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/auth"
	"github.com/AngelAvilesSil/3Default/internal/config"
	"github.com/AngelAvilesSil/3Default/internal/conversionjobs"
	"github.com/AngelAvilesSil/3Default/internal/database"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/filestorage"
	"github.com/AngelAvilesSil/3Default/internal/httpapi"
	"github.com/AngelAvilesSil/3Default/internal/projects"
	"github.com/AngelAvilesSil/3Default/internal/storage"
	"github.com/AngelAvilesSil/3Default/internal/versioning"
)

func main() {
	const (
		address                    = ":8080"
		sessionLifetime            = 7 * 24 * time.Hour
		loginAttemptCapacity       = 5
		loginAttemptRefillInterval = time.Minute
		loginAttemptMaxEntries     = 10_000
		conversionWorkerIdleDelay  = time.Second
	)

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

	projectStore := database.NewProjectStore(db)
	projectService := projects.NewService(projectStore)
	versioningService := versioning.NewService(
		projectStore,
		projectStore,
	)

	fileStore := database.NewFileStore(db)
	fileService := filestorage.NewService(
		projectStore,
		fileStore,
	)

	contentStore, err := storage.NewFilesystemStore(
		cfg.StorageRoot,
	)
	if err != nil {
		log.Fatalf(
			"initialize content storage: %v",
			err,
		)
	}

	fileUploadService := filestorage.NewUploadService(
		fileService,
		contentStore,
	)

	fileDownloadService := filestorage.NewDownloadService(
		fileService,
		contentStore,
	)

	conversionJobStore := database.NewConversionJobStore(db)
	conversionJobService := conversionjobs.NewService(
		projectStore,
		fileStore,
		conversionJobStore,
	)

	conversionPreviewService := conversionjobs.NewPreviewService(
		projectStore,
		fileStore,
		conversionJobStore,
		contentStore,
	)

	converter, err := conversionjobs.NewConfiguredConverter(
		cfg.MayoExecutable,
	)
	if err != nil {
		log.Fatalf(
			"initialize conversion converter: %v",
			err,
		)
	}

	conversionWorker, err := conversionjobs.NewWorker(
		conversionJobStore,
		contentStore,
		converter,
		conversionWorkerIdleDelay,
	)
	if err != nil {
		log.Fatalf(
			"initialize conversion worker: %v",
			err,
		)
	}

	registrationStore := database.NewRegistrationStore(db)

	passwordBlocklist := auth.NewLocalPasswordBlocklist(
		"3default",
		"3default.com",
	)

	passwordPolicy := auth.NewPasswordPolicy(
		passwordBlocklist,
	)

	registrationService := auth.NewRegistrationService(
		registrationStore,
		passwordPolicy,
	)

	sessionService, err := auth.NewSessionService(
		queries,
		sessionLifetime,
	)
	if err != nil {
		log.Fatal(err)
	}

	loginAttemptLimiter, err := auth.NewInMemoryLoginAttemptLimiter(
		auth.LoginAttemptLimiterConfig{
			Capacity:       loginAttemptCapacity,
			RefillInterval: loginAttemptRefillInterval,
			MaxEntries:     loginAttemptMaxEntries,
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	loginService, err := auth.NewLoginService(
		queries,
		sessionService,
		loginAttemptLimiter,
	)
	if err != nil {
		log.Fatal(err)
	}

	handler := httpapi.NewHandler(
		httpapi.NewServerWithConversionPreviews(
			db,
			projectService,
			versioningService,
			fileService,
			fileUploadService,
			fileDownloadService,
			conversionJobService,
			conversionPreviewService,
			registrationService,
			loginService,
			sessionService,
			queries,
		),
		sessionService,
	)

	server := &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	runtimeErr := make(chan error, 2)

	go func() {
		if err := conversionWorker.Run(ctx); err != nil {
			runtimeErr <- fmt.Errorf(
				"run conversion worker: %w",
				err,
			)
		}
	}()

	go func() {
		if err := server.ListenAndServe(); err != nil {
			runtimeErr <- fmt.Errorf(
				"serve HTTP: %w",
				err,
			)
		}
	}()

	log.Printf("3Default listening on %s", address)
	log.Fatal(<-runtimeErr)
}
