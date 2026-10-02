//go:build integration

package database_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConversionJobQueriesCreateAndReadPendingJob(t *testing.T) {
	fixture := setupConversionJobTest(t)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"create-read",
	)

	job, err := fixture.queries.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)
	if err != nil {
		t.Fatalf("create conversion job: %v", err)
	}

	if job.ProjectID != fixture.projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			fixture.projectID,
			job.ProjectID,
		)
	}

	if job.ProjectFileID != projectFile.ID {
		t.Fatalf(
			"expected project file ID %s, got %s",
			projectFile.ID,
			job.ProjectFileID,
		)
	}

	assertPendingConversionJob(t, job)

	found, err := fixture.queries.GetConversionJobByIDAndProject(
		fixture.ctx,
		dbgen.GetConversionJobByIDAndProjectParams{
			ConversionJobID: job.ID,
			ProjectID:       fixture.projectID,
		},
	)
	if err != nil {
		t.Fatalf("get conversion job: %v", err)
	}

	if found.ID != job.ID {
		t.Fatalf(
			"expected conversion job ID %s, got %s",
			job.ID,
			found.ID,
		)
	}

	_, err = fixture.queries.GetConversionJobByIDAndProject(
		fixture.ctx,
		dbgen.GetConversionJobByIDAndProjectParams{
			ConversionJobID: job.ID,
			ProjectID:       fixture.otherProjectID,
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected wrong-project lookup to return pgx.ErrNoRows, got %v",
			err,
		)
	}

	jobs, err := fixture.queries.ListConversionJobsByProjectFile(
		fixture.ctx,
		dbgen.ListConversionJobsByProjectFileParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)
	if err != nil {
		t.Fatalf("list project-file conversion jobs: %v", err)
	}

	if len(jobs) != 1 {
		t.Fatalf(
			"expected 1 conversion job, got %d",
			len(jobs),
		)
	}

	if jobs[0].ID != job.ID {
		t.Fatalf(
			"expected listed conversion job %s, got %s",
			job.ID,
			jobs[0].ID,
		)
	}
}

func TestConversionJobQueriesRejectMissingOrWrongProjectFile(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)

	_, err := fixture.queries.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: uuid.New(),
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected missing project file to return pgx.ErrNoRows, got %v",
			err,
		)
	}

	otherProjectFile := fixture.createProjectFile(
		fixture.otherProjectID,
		"wrong-project",
	)

	_, err = fixture.queries.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: otherProjectFile.ID,
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected wrong-project file to return pgx.ErrNoRows, got %v",
			err,
		)
	}
}

func TestConversionJobSchemaRejectsCrossProjectReference(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)

	otherProjectFile := fixture.createProjectFile(
		fixture.otherProjectID,
		"cross-project-schema",
	)

	_, err := fixture.pool.Exec(
		fixture.ctx,
		`INSERT INTO conversion_jobs (
			project_id,
			project_file_id
		)
		VALUES ($1, $2)`,
		fixture.projectID,
		otherProjectFile.ID,
	)

	assertPostgresConstraint(
		t,
		err,
		"23503",
		"conversion_jobs_project_file_same_project",
	)
}

func TestConversionJobQueriesRejectSecondActiveJob(t *testing.T) {
	fixture := setupConversionJobTest(t)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"one-active",
	)

	first, err := fixture.queries.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)
	if err != nil {
		t.Fatalf("create first conversion job: %v", err)
	}

	_, err = fixture.queries.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)

	assertPostgresConstraint(
		t,
		err,
		"23505",
		"conversion_jobs_one_active_per_project_file",
	)

	claimed, err := fixture.queries.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if err != nil {
		t.Fatalf("claim first conversion job: %v", err)
	}

	if claimed.ID != first.ID {
		t.Fatalf(
			"expected claimed job %s, got %s",
			first.ID,
			claimed.ID,
		)
	}

	_, err = fixture.queries.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)

	assertPostgresConstraint(
		t,
		err,
		"23505",
		"conversion_jobs_one_active_per_project_file",
	)
}

func TestConversionJobQueriesAllowNewJobAfterTerminalCompletion(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"terminal-history",
	)

	first, err := fixture.queries.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)
	if err != nil {
		t.Fatalf("create first conversion job: %v", err)
	}

	claimed, err := fixture.queries.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if err != nil {
		t.Fatalf("claim first conversion job: %v", err)
	}

	if claimed.ID != first.ID {
		t.Fatalf(
			"expected claimed job %s, got %s",
			first.ID,
			claimed.ID,
		)
	}

	if _, err := fixture.queries.MarkConversionJobSucceeded(
		fixture.ctx,
		first.ID,
	); err != nil {
		t.Fatalf("mark first conversion job succeeded: %v", err)
	}

	second, err := fixture.queries.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)
	if err != nil {
		t.Fatalf(
			"create new conversion job after terminal completion: %v",
			err,
		)
	}

	if second.ID == first.ID {
		t.Fatalf(
			"expected distinct conversion job IDs, both were %s",
			first.ID,
		)
	}

	assertPendingConversionJob(t, second)

	jobs, err := fixture.queries.ListConversionJobsByProjectFile(
		fixture.ctx,
		dbgen.ListConversionJobsByProjectFileParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFile.ID,
		},
	)
	if err != nil {
		t.Fatalf("list conversion job history: %v", err)
	}

	if len(jobs) != 2 {
		t.Fatalf(
			"expected 2 conversion jobs in history, got %d",
			len(jobs),
		)
	}

	var pendingCount int
	var succeededCount int

	for _, job := range jobs {
		switch job.Status {
		case "pending":
			pendingCount++
		case "succeeded":
			succeededCount++
		}
	}

	if pendingCount != 1 || succeededCount != 1 {
		t.Fatalf(
			"expected one pending and one succeeded job, got pending=%d succeeded=%d",
			pendingCount,
			succeededCount,
		)
	}
}

func TestConversionJobQueriesClaimOldestPendingJob(t *testing.T) {
	fixture := setupConversionJobTest(t)

	firstFile := fixture.createProjectFile(
		fixture.projectID,
		"claim-first",
	)
	secondFile := fixture.createProjectFile(
		fixture.projectID,
		"claim-second",
	)
	thirdFile := fixture.createProjectFile(
		fixture.projectID,
		"claim-third",
	)

	first := createConversionJob(
		t,
		fixture,
		firstFile.ID,
	)
	second := createConversionJob(
		t,
		fixture,
		secondFile.ID,
	)
	third := createConversionJob(
		t,
		fixture,
		thirdFile.ID,
	)

	baseTime := time.Now().UTC().Add(-3 * time.Hour)

	setConversionJobTime(
		t,
		fixture,
		first.ID,
		baseTime,
	)
	setConversionJobTime(
		t,
		fixture,
		second.ID,
		baseTime.Add(time.Hour),
	)
	setConversionJobTime(
		t,
		fixture,
		third.ID,
		baseTime.Add(2*time.Hour),
	)

	expectedOrder := []uuid.UUID{
		first.ID,
		second.ID,
		third.ID,
	}

	for index, expectedID := range expectedOrder {
		claimed, err :=
			fixture.queries.ClaimNextPendingConversionJob(
				fixture.ctx,
			)
		if err != nil {
			t.Fatalf(
				"claim conversion job %d: %v",
				index,
				err,
			)
		}

		if claimed.ID != expectedID {
			t.Fatalf(
				"expected claim %d to return %s, got %s",
				index,
				expectedID,
				claimed.ID,
			)
		}

		if claimed.Status != "running" {
			t.Fatalf(
				"expected claimed job status running, got %q",
				claimed.Status,
			)
		}

		if claimed.AttemptCount != 1 {
			t.Fatalf(
				"expected claimed job attempt count 1, got %d",
				claimed.AttemptCount,
			)
		}
	}

	_, err := fixture.queries.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected empty pending queue to return pgx.ErrNoRows, got %v",
			err,
		)
	}
}

func TestConversionJobQueriesSkipLockedPendingJob(t *testing.T) {
	fixture := setupConversionJobTest(t)

	firstFile := fixture.createProjectFile(
		fixture.projectID,
		"locked-first",
	)
	secondFile := fixture.createProjectFile(
		fixture.projectID,
		"available-second",
	)

	first := createConversionJob(
		t,
		fixture,
		firstFile.ID,
	)
	second := createConversionJob(
		t,
		fixture,
		secondFile.ID,
	)

	baseTime := time.Now().UTC().Add(-2 * time.Hour)

	setConversionJobTime(
		t,
		fixture,
		first.ID,
		baseTime,
	)
	setConversionJobTime(
		t,
		fixture,
		second.ID,
		baseTime.Add(time.Hour),
	)

	lockTx, err := fixture.pool.Begin(fixture.ctx)
	if err != nil {
		t.Fatalf("begin row-lock transaction: %v", err)
	}

	defer func() {
		_ = lockTx.Rollback(context.Background())
	}()

	var lockedID uuid.UUID

	err = lockTx.QueryRow(
		fixture.ctx,
		`SELECT id
		 FROM conversion_jobs
		 WHERE id = $1
		 FOR UPDATE`,
		first.ID,
	).Scan(&lockedID)
	if err != nil {
		t.Fatalf("lock oldest pending conversion job: %v", err)
	}

	if lockedID != first.ID {
		t.Fatalf(
			"expected locked job %s, got %s",
			first.ID,
			lockedID,
		)
	}

	claimCtx, cancel := context.WithTimeout(
		fixture.ctx,
		2*time.Second,
	)
	defer cancel()

	claimed, err :=
		fixture.queries.ClaimNextPendingConversionJob(claimCtx)
	if err != nil {
		t.Fatalf(
			"claim next conversion job while oldest is locked: %v",
			err,
		)
	}

	if claimed.ID != second.ID {
		t.Fatalf(
			"expected SKIP LOCKED claim to return %s, got %s",
			second.ID,
			claimed.ID,
		)
	}

	persistedFirst, err :=
		fixture.queries.GetConversionJobByIDAndProject(
			fixture.ctx,
			dbgen.GetConversionJobByIDAndProjectParams{
				ConversionJobID: first.ID,
				ProjectID:       fixture.projectID,
			},
		)
	if err != nil {
		t.Fatalf(
			"get locked conversion job after other claim: %v",
			err,
		)
	}

	if persistedFirst.Status != "pending" {
		t.Fatalf(
			"expected locked job to remain pending, got %q",
			persistedFirst.Status,
		)
	}
}

func TestConversionJobQueriesMarkSucceeded(t *testing.T) {
	fixture := setupConversionJobTest(t)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"succeeded",
	)

	job := createConversionJob(
		t,
		fixture,
		projectFile.ID,
	)

	claimed, err :=
		fixture.queries.ClaimNextPendingConversionJob(
			fixture.ctx,
		)
	if err != nil {
		t.Fatalf("claim conversion job: %v", err)
	}

	succeeded, err :=
		fixture.queries.MarkConversionJobSucceeded(
			fixture.ctx,
			claimed.ID,
		)
	if err != nil {
		t.Fatalf("mark conversion job succeeded: %v", err)
	}

	if succeeded.ID != job.ID {
		t.Fatalf(
			"expected succeeded job %s, got %s",
			job.ID,
			succeeded.ID,
		)
	}

	if succeeded.Status != "succeeded" {
		t.Fatalf(
			"expected succeeded status, got %q",
			succeeded.Status,
		)
	}

	if succeeded.AttemptCount != 1 {
		t.Fatalf(
			"expected attempt count 1, got %d",
			succeeded.AttemptCount,
		)
	}

	if !succeeded.StartedAt.Valid {
		t.Fatal("expected succeeded job started_at to be set")
	}

	if !succeeded.FinishedAt.Valid {
		t.Fatal("expected succeeded job finished_at to be set")
	}

	if succeeded.LastError != nil {
		t.Fatalf(
			"expected succeeded job last_error to be nil, got %q",
			*succeeded.LastError,
		)
	}
}

func TestConversionJobQueriesMarkFailed(t *testing.T) {
	fixture := setupConversionJobTest(t)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"failed",
	)

	job := createConversionJob(
		t,
		fixture,
		projectFile.ID,
	)

	claimed, err :=
		fixture.queries.ClaimNextPendingConversionJob(
			fixture.ctx,
		)
	if err != nil {
		t.Fatalf("claim conversion job: %v", err)
	}

	failure := "converter exited with status 1"

	failed, err := fixture.queries.MarkConversionJobFailed(
		fixture.ctx,
		dbgen.MarkConversionJobFailedParams{
			LastError:       &failure,
			ConversionJobID: claimed.ID,
		},
	)
	if err != nil {
		t.Fatalf("mark conversion job failed: %v", err)
	}

	if failed.ID != job.ID {
		t.Fatalf(
			"expected failed job %s, got %s",
			job.ID,
			failed.ID,
		)
	}

	if failed.Status != "failed" {
		t.Fatalf(
			"expected failed status, got %q",
			failed.Status,
		)
	}

	if failed.AttemptCount != 1 {
		t.Fatalf(
			"expected attempt count 1, got %d",
			failed.AttemptCount,
		)
	}

	if !failed.StartedAt.Valid {
		t.Fatal("expected failed job started_at to be set")
	}

	if !failed.FinishedAt.Valid {
		t.Fatal("expected failed job finished_at to be set")
	}

	if failed.LastError == nil {
		t.Fatal("expected failed job last_error to be set")
	}

	if *failed.LastError != failure {
		t.Fatalf(
			"expected failure %q, got %q",
			failure,
			*failed.LastError,
		)
	}
}

func TestConversionJobQueriesDoNotCompleteTerminalJobTwice(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"terminal-once",
	)

	job := createConversionJob(
		t,
		fixture,
		projectFile.ID,
	)

	claimed, err :=
		fixture.queries.ClaimNextPendingConversionJob(
			fixture.ctx,
		)
	if err != nil {
		t.Fatalf("claim conversion job: %v", err)
	}

	if claimed.ID != job.ID {
		t.Fatalf(
			"expected claimed job %s, got %s",
			job.ID,
			claimed.ID,
		)
	}

	if _, err := fixture.queries.MarkConversionJobSucceeded(
		fixture.ctx,
		job.ID,
	); err != nil {
		t.Fatalf("complete conversion job: %v", err)
	}

	_, err = fixture.queries.MarkConversionJobSucceeded(
		fixture.ctx,
		job.ID,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected second success transition to return pgx.ErrNoRows, got %v",
			err,
		)
	}

	failure := "late failure"

	_, err = fixture.queries.MarkConversionJobFailed(
		fixture.ctx,
		dbgen.MarkConversionJobFailedParams{
			LastError:       &failure,
			ConversionJobID: job.ID,
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected terminal job failure transition to return pgx.ErrNoRows, got %v",
			err,
		)
	}
}

func TestConversionJobQueriesRejectInvalidDirectStates(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"invalid-states",
	)

	job := createConversionJob(
		t,
		fixture,
		projectFile.ID,
	)

	tests := []struct {
		name               string
		expectedConstraint string
		query              string
	}{
		{
			name:               "negative attempt count",
			expectedConstraint: "conversion_jobs_attempt_count_nonnegative",
			query: `UPDATE conversion_jobs
				SET attempt_count = -1
				WHERE id = $1`,
		},
		{
			name:               "running without attempt",
			expectedConstraint: "conversion_jobs_attempt_count_state_valid",
			query: `UPDATE conversion_jobs
				SET
					status = 'running',
					started_at = now()
				WHERE id = $1`,
		},
		{
			name:               "pending with started timestamp",
			expectedConstraint: "conversion_jobs_state_valid",
			query: `UPDATE conversion_jobs
				SET started_at = now()
				WHERE id = $1`,
		},
		{
			name:               "failed without last error",
			expectedConstraint: "conversion_jobs_state_valid",
			query: `UPDATE conversion_jobs
				SET
					status = 'failed',
					attempt_count = 1,
					started_at = now(),
					finished_at = now()
				WHERE id = $1`,
		},
		{
			name:               "failed with blank last error",
			expectedConstraint: "conversion_jobs_last_error_valid",
			query: `UPDATE conversion_jobs
				SET
					status = 'failed',
					attempt_count = 1,
					started_at = now(),
					finished_at = now(),
					last_error = '   '
				WHERE id = $1`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := fixture.pool.Exec(
				fixture.ctx,
				test.query,
				job.ID,
			)

			assertPostgresConstraint(
				t,
				err,
				"23514",
				test.expectedConstraint,
			)
		})
	}

	persisted, err :=
		fixture.queries.GetConversionJobByIDAndProject(
			fixture.ctx,
			dbgen.GetConversionJobByIDAndProjectParams{
				ConversionJobID: job.ID,
				ProjectID:       fixture.projectID,
			},
		)
	if err != nil {
		t.Fatalf(
			"get conversion job after rejected state changes: %v",
			err,
		)
	}

	assertPendingConversionJob(t, persisted)
}

type conversionJobFixture struct {
	ctx               context.Context
	pool              *pgxpool.Pool
	queries           *dbgen.Queries
	projectID         uuid.UUID
	otherProjectID    uuid.UUID
	createProjectFile func(
		projectID uuid.UUID,
		label string,
	) dbgen.ProjectFile
}

func setupConversionJobTest(
	t *testing.T,
) *conversionJobFixture {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip(
			"DATABASE_URL is required for database integration tests",
		)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	t.Cleanup(cancel)

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	queries := dbgen.New(pool)

	testID := uuid.New().String()

	firstUser, err := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{
			Email: testID +
				"-one@conversion-jobs.example.com",
			DisplayName: "Conversion Jobs User One",
		},
	)
	if err != nil {
		t.Fatalf("create first conversion-jobs user: %v", err)
	}

	secondUser, err := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{
			Email: testID +
				"-two@conversion-jobs.example.com",
			DisplayName: "Conversion Jobs User Two",
		},
	)
	if err != nil {
		t.Fatalf("create second conversion-jobs user: %v", err)
	}

	projectStore := database.NewProjectStore(pool)

	firstProject, err := projectStore.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: firstUser.ID,
			Name:        "Conversion Jobs Project One",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create first conversion-jobs project: %v", err)
	}

	secondProject, err := projectStore.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: secondUser.ID,
			Name:        "Conversion Jobs Project Two",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create second conversion-jobs project: %v", err)
	}

	var contentHashes []string

	createProjectFile := func(
		projectID uuid.UUID,
		label string,
	) dbgen.ProjectFile {
		t.Helper()

		var uploaderUserID uuid.UUID

		switch projectID {
		case firstProject.ID:
			uploaderUserID = firstUser.ID
		case secondProject.ID:
			uploaderUserID = secondUser.ID
		default:
			t.Fatalf(
				"unsupported fixture project ID %s",
				projectID,
			)
		}

		digest := sha256.Sum256(
			[]byte(
				testID +
					":" +
					label +
					":" +
					uuid.New().String(),
			),
		)

		contentSHA256 := hex.EncodeToString(digest[:])
		contentHashes = append(
			contentHashes,
			contentSHA256,
		)

		_, err := queries.EnsureContentObject(
			ctx,
			dbgen.EnsureContentObjectParams{
				Sha256:    contentSHA256,
				SizeBytes: int64(len(label)),
			},
		)
		if err != nil {
			t.Fatalf(
				"ensure content object for %q: %v",
				label,
				err,
			)
		}

		projectFile, err := queries.CreateProjectFile(
			ctx,
			dbgen.CreateProjectFileParams{
				ProjectID:        projectID,
				UploadedByUserID: uploaderUserID,
				ContentSha256:    contentSHA256,
				OriginalFilename: label + ".step",
				MediaType:        nil,
			},
		)
		if err != nil {
			t.Fatalf(
				"create project file for %q: %v",
				label,
				err,
			)
		}

		return projectFile
	}

	t.Cleanup(func() {
		for _, projectID := range []uuid.UUID{
			firstProject.ID,
			secondProject.ID,
		} {
			_, err := pool.Exec(
				context.Background(),
				"DELETE FROM projects WHERE id = $1",
				projectID,
			)
			if err != nil {
				t.Errorf(
					"delete conversion-jobs project %s: %v",
					projectID,
					err,
				)
			}
		}

		for _, contentSHA256 := range contentHashes {
			_, err := pool.Exec(
				context.Background(),
				"DELETE FROM content_objects WHERE sha256 = $1",
				contentSHA256,
			)
			if err != nil {
				t.Errorf(
					"delete conversion-jobs content object %q: %v",
					contentSHA256,
					err,
				)
			}
		}

		for _, userID := range []uuid.UUID{
			firstUser.ID,
			secondUser.ID,
		} {
			_, err := pool.Exec(
				context.Background(),
				"DELETE FROM users WHERE id = $1",
				userID,
			)
			if err != nil {
				t.Errorf(
					"delete conversion-jobs user %s: %v",
					userID,
					err,
				)
			}
		}
	})

	return &conversionJobFixture{
		ctx:               ctx,
		pool:              pool,
		queries:           queries,
		projectID:         firstProject.ID,
		otherProjectID:    secondProject.ID,
		createProjectFile: createProjectFile,
	}
}

func createConversionJob(
	t *testing.T,
	fixture *conversionJobFixture,
	projectFileID uuid.UUID,
) dbgen.ConversionJob {
	t.Helper()

	job, err := fixture.queries.CreateConversionJob(
		fixture.ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     fixture.projectID,
			ProjectFileID: projectFileID,
		},
	)
	if err != nil {
		t.Fatalf("create conversion job: %v", err)
	}

	return job
}

func setConversionJobTime(
	t *testing.T,
	fixture *conversionJobFixture,
	conversionJobID uuid.UUID,
	createdAt time.Time,
) {
	t.Helper()

	_, err := fixture.pool.Exec(
		fixture.ctx,
		`UPDATE conversion_jobs
		 SET
			created_at = $1,
			updated_at = $1
		 WHERE id = $2`,
		createdAt,
		conversionJobID,
	)
	if err != nil {
		t.Fatalf(
			"set conversion job %s timestamp: %v",
			conversionJobID,
			err,
		)
	}
}

func assertPendingConversionJob(
	t *testing.T,
	job dbgen.ConversionJob,
) {
	t.Helper()

	if job.Status != "pending" {
		t.Fatalf(
			"expected pending status, got %q",
			job.Status,
		)
	}

	if job.AttemptCount != 0 {
		t.Fatalf(
			"expected pending attempt count 0, got %d",
			job.AttemptCount,
		)
	}

	if job.LastError != nil {
		t.Fatalf(
			"expected pending last_error nil, got %q",
			*job.LastError,
		)
	}

	if job.StartedAt.Valid {
		t.Fatal("expected pending started_at to be null")
	}

	if job.FinishedAt.Valid {
		t.Fatal("expected pending finished_at to be null")
	}
}

func assertPostgresConstraint(
	t *testing.T,
	err error,
	expectedCode string,
	expectedConstraint string,
) {
	t.Helper()

	if err == nil {
		t.Fatalf(
			"expected PostgreSQL constraint %q to fail",
			expectedConstraint,
		)
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf(
			"expected PostgreSQL error for constraint %q, got %T: %v",
			expectedConstraint,
			err,
			err,
		)
	}

	if pgErr.Code != expectedCode {
		t.Fatalf(
			"expected PostgreSQL code %q, got %q for constraint %q",
			expectedCode,
			pgErr.Code,
			expectedConstraint,
		)
	}

	if pgErr.ConstraintName != expectedConstraint {
		t.Fatalf(
			"expected constraint %q, got %q",
			expectedConstraint,
			pgErr.ConstraintName,
		)
	}
}
