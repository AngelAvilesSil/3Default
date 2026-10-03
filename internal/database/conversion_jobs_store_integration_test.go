//go:build integration

package database_test

import (
	"errors"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/conversionjobs"
	"github.com/AngelAvilesSil/3Default/internal/database"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
)

func TestConversionJobStoreMapsMissingProjectFile(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	_, err := store.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: uuid.New(),
		},
	)
	if !errors.Is(
		err,
		conversionjobs.ErrProjectFileNotFound,
	) {
		t.Fatalf(
			"expected ErrProjectFileNotFound, got %v",
			err,
		)
	}
}

func TestConversionJobStoreMapsActiveJobConflict(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"store-active-conflict",
	)

	_, err := store.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)
	if err != nil {
		t.Fatalf(
			"create first conversion job: %v",
			err,
		)
	}

	_, err = store.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)
	if !errors.Is(
		err,
		conversionjobs.ErrActiveConversionJobExists,
	) {
		t.Fatalf(
			"expected ErrActiveConversionJobExists, got %v",
			err,
		)
	}
}
