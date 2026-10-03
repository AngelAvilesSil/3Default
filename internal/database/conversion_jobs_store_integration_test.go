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

func TestConversionJobStoreMapsEmptyQueue(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	_, err := store.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if !errors.Is(
		err,
		conversionjobs.ErrNoPendingConversionJob,
	) {
		t.Fatalf(
			"expected ErrNoPendingConversionJob, got %v",
			err,
		)
	}
}

func TestConversionJobStoreRequeuesRunningJobForRetry(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"requeue-running",
	)

	created, err := store.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)
	if err != nil {
		t.Fatalf("create conversion job: %v", err)
	}

	claimed, err := store.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if err != nil {
		t.Fatalf("claim conversion job: %v", err)
	}

	if claimed.ID != created.ID {
		t.Fatalf(
			"expected claimed job %s, got %s",
			created.ID,
			claimed.ID,
		)
	}

	if claimed.AttemptCount != 1 {
		t.Fatalf(
			"expected first attempt count 1, got %d",
			claimed.AttemptCount,
		)
	}

	requeued, err := store.RequeueConversionJob(
		fixture.ctx,
		claimed.ID,
	)
	if err != nil {
		t.Fatalf("requeue conversion job: %v", err)
	}

	if requeued.Status != "pending" {
		t.Fatalf(
			"expected pending status, got %q",
			requeued.Status,
		)
	}

	if requeued.AttemptCount != 1 {
		t.Fatalf(
			"expected attempt count to remain 1, got %d",
			requeued.AttemptCount,
		)
	}

	if requeued.StartedAt.Valid {
		t.Fatal("expected requeued started_at to be null")
	}

	if requeued.FinishedAt.Valid {
		t.Fatal("expected requeued finished_at to be null")
	}

	if requeued.LastError != nil {
		t.Fatalf(
			"expected requeued last_error nil, got %q",
			*requeued.LastError,
		)
	}

	reclaimed, err := store.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if err != nil {
		t.Fatalf("reclaim conversion job: %v", err)
	}

	if reclaimed.ID != created.ID {
		t.Fatalf(
			"expected reclaimed job %s, got %s",
			created.ID,
			reclaimed.ID,
		)
	}

	if reclaimed.AttemptCount != 2 {
		t.Fatalf(
			"expected second attempt count 2, got %d",
			reclaimed.AttemptCount,
		)
	}
}

func TestConversionJobStoreMapsRequeueOfNonRunningJob(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"requeue-pending",
	)

	job, err := store.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)
	if err != nil {
		t.Fatalf("create conversion job: %v", err)
	}

	_, err = store.RequeueConversionJob(
		fixture.ctx,
		job.ID,
	)
	if !errors.Is(
		err,
		conversionjobs.ErrConversionJobNotRunning,
	) {
		t.Fatalf(
			"expected ErrConversionJobNotRunning, got %v",
			err,
		)
	}
}

func TestConversionJobStoreRecoversRunningJobs(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	firstFile := fixture.createProjectFile(
		fixture.projectID,
		"recover-first",
	)
	secondFile := fixture.createProjectFile(
		fixture.projectID,
		"recover-second",
	)

	first, err := store.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: firstFile.ID,
		},
	)
	if err != nil {
		t.Fatalf("create first conversion job: %v", err)
	}

	second, err := store.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: secondFile.ID,
		},
	)
	if err != nil {
		t.Fatalf("create second conversion job: %v", err)
	}

	firstClaim, err := store.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if err != nil {
		t.Fatalf("claim first conversion job: %v", err)
	}

	secondClaim, err := store.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if err != nil {
		t.Fatalf("claim second conversion job: %v", err)
	}

	claimedIDs := map[uuid.UUID]bool{
		firstClaim.ID:  true,
		secondClaim.ID: true,
	}

	if !claimedIDs[first.ID] || !claimedIDs[second.ID] {
		t.Fatalf(
			"expected both jobs to be claimed, got %s and %s",
			firstClaim.ID,
			secondClaim.ID,
		)
	}

	if err := store.RequeueRunningConversionJobs(
		fixture.ctx,
	); err != nil {
		t.Fatalf(
			"recover running conversion jobs: %v",
			err,
		)
	}

	for _, job := range []dbgen.ConversionJob{
		first,
		second,
	} {
		recovered, err :=
			store.GetConversionJobByIDAndProject(
				fixture.ctx,
				dbgen.GetConversionJobByIDAndProjectParams{
					ConversionJobID: job.ID,
					ProjectID:       fixture.projectID,
				},
			)
		if err != nil {
			t.Fatalf(
				"get recovered conversion job %s: %v",
				job.ID,
				err,
			)
		}

		if recovered.Status != "pending" {
			t.Fatalf(
				"expected recovered job %s pending, got %q",
				job.ID,
				recovered.Status,
			)
		}

		if recovered.AttemptCount != 1 {
			t.Fatalf(
				"expected recovered job %s attempt count 1, got %d",
				job.ID,
				recovered.AttemptCount,
			)
		}

		if recovered.StartedAt.Valid {
			t.Fatalf(
				"expected recovered job %s started_at null",
				job.ID,
			)
		}
	}
}

func TestConversionJobStoreMapsCompletionOfNonRunningJob(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"complete-pending",
	)

	job, err := store.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)
	if err != nil {
		t.Fatalf("create conversion job: %v", err)
	}

	_, err = store.MarkConversionJobSucceeded(
		fixture.ctx,
		job.ID,
	)
	if !errors.Is(
		err,
		conversionjobs.ErrConversionJobNotRunning,
	) {
		t.Fatalf(
			"expected success transition to return ErrConversionJobNotRunning, got %v",
			err,
		)
	}

	failure := "should not be accepted"

	_, err = store.MarkConversionJobFailed(
		fixture.ctx,
		dbgen.MarkConversionJobFailedParams{
			LastError:       &failure,
			ConversionJobID: job.ID,
		},
	)
	if !errors.Is(
		err,
		conversionjobs.ErrConversionJobNotRunning,
	) {
		t.Fatalf(
			"expected failure transition to return ErrConversionJobNotRunning, got %v",
			err,
		)
	}
}
