//go:build integration

package database_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/versioning"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProjectStoreCreatesRevisionAndAdvancesBranchHead(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	firstRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               branchID,
			AuthorUserID:           userID,
			Message:                "Initial revision",
			ExpectedHeadRevisionID: nil,
			MergeParentRevisionID:  nil,
		},
	)
	if err != nil {
		t.Fatalf("create initial revision: %v", err)
	}

	if firstRevision.ParentRevisionID.Valid {
		t.Fatalf(
			"expected initial revision parent to be null, got %s",
			uuid.UUID(firstRevision.ParentRevisionID.Bytes),
		)
	}

	if firstRevision.MergeParentRevisionID.Valid {
		t.Fatalf(
			"expected initial merge parent to be null, got %s",
			uuid.UUID(firstRevision.MergeParentRevisionID.Bytes),
		)
	}

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		branchID,
		firstRevision.ID,
	)

	secondRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               branchID,
			AuthorUserID:           userID,
			Message:                "Second revision",
			ExpectedHeadRevisionID: &firstRevision.ID,
			MergeParentRevisionID:  nil,
		},
	)
	if err != nil {
		t.Fatalf("create second revision: %v", err)
	}

	if !secondRevision.ParentRevisionID.Valid {
		t.Fatal("expected second revision parent to be set")
	}

	if uuid.UUID(secondRevision.ParentRevisionID.Bytes) != firstRevision.ID {
		t.Fatalf(
			"expected second revision parent %s, got %s",
			firstRevision.ID,
			uuid.UUID(secondRevision.ParentRevisionID.Bytes),
		)
	}

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		branchID,
		secondRevision.ID,
	)

	var revisionCount int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revisions
		 WHERE project_id = $1`,
		projectID,
	).Scan(&revisionCount); err != nil {
		t.Fatalf("count project revisions: %v", err)
	}

	if revisionCount != 2 {
		t.Fatalf(
			"expected 2 project revisions, got %d",
			revisionCount,
		)
	}
}

func TestProjectStoreRollsBackRevisionOnBranchHeadConflict(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	firstRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     branchID,
			AuthorUserID: userID,
			Message:      "Initial revision",
		},
	)
	if err != nil {
		t.Fatalf("create initial revision: %v", err)
	}

	secondRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               branchID,
			AuthorUserID:           userID,
			Message:                "Second revision",
			ExpectedHeadRevisionID: &firstRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create second revision: %v", err)
	}

	var revisionsBefore int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revisions
		 WHERE project_id = $1`,
		projectID,
	).Scan(&revisionsBefore); err != nil {
		t.Fatalf("count revisions before conflict: %v", err)
	}

	_, err = store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               branchID,
			AuthorUserID:           userID,
			Message:                "Stale candidate revision",
			ExpectedHeadRevisionID: &firstRevision.ID,
		},
	)
	if !errors.Is(err, versioning.ErrBranchHeadConflict) {
		t.Fatalf(
			"expected branch head conflict, got %v",
			err,
		)
	}

	var revisionsAfter int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revisions
		 WHERE project_id = $1`,
		projectID,
	).Scan(&revisionsAfter); err != nil {
		t.Fatalf("count revisions after conflict: %v", err)
	}

	if revisionsAfter != revisionsBefore {
		t.Fatalf(
			"expected failed revision to roll back; count changed from %d to %d",
			revisionsBefore,
			revisionsAfter,
		)
	}

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		branchID,
		secondRevision.ID,
	)
}

func TestProjectStoreRollsBackRevisionWhenBranchIsMissing(
	t *testing.T,
) {
	ctx, pool, store, _, userID, projectID, _ :=
		setupVersioningStoreTest(t)

	var revisionsBefore int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revisions
		 WHERE project_id = $1`,
		projectID,
	).Scan(&revisionsBefore); err != nil {
		t.Fatalf("count revisions before missing branch: %v", err)
	}

	_, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     uuid.New(),
			AuthorUserID: userID,
			Message:      "Missing branch candidate",
		},
	)
	if !errors.Is(err, versioning.ErrBranchNotFound) {
		t.Fatalf(
			"expected project branch not found, got %v",
			err,
		)
	}

	var revisionsAfter int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revisions
		 WHERE project_id = $1`,
		projectID,
	).Scan(&revisionsAfter); err != nil {
		t.Fatalf("count revisions after missing branch: %v", err)
	}

	if revisionsAfter != revisionsBefore {
		t.Fatalf(
			"expected missing-branch revision to roll back; count changed from %d to %d",
			revisionsBefore,
			revisionsAfter,
		)
	}
}

func TestProjectStoreCreatesMergeRevisionAndAdvancesTargetBranch(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, mainBranchID :=
		setupVersioningStoreTest(t)

	rootRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     mainBranchID,
			AuthorUserID: userID,
			Message:      "Initial revision",
		},
	)
	if err != nil {
		t.Fatalf("create root revision: %v", err)
	}

	var sideBranchID uuid.UUID
	if err := pool.QueryRow(
		ctx,
		`INSERT INTO project_branches (
			project_id,
			name,
			head_revision_id
		)
		VALUES ($1, 'feature', $2)
		RETURNING id`,
		projectID,
		rootRevision.ID,
	).Scan(&sideBranchID); err != nil {
		t.Fatalf("create side branch: %v", err)
	}

	mainRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               mainBranchID,
			AuthorUserID:           userID,
			Message:                "Main branch revision",
			ExpectedHeadRevisionID: &rootRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create main branch revision: %v", err)
	}

	sideRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               sideBranchID,
			AuthorUserID:           userID,
			Message:                "Feature branch revision",
			ExpectedHeadRevisionID: &rootRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create side branch revision: %v", err)
	}

	mergeRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               mainBranchID,
			AuthorUserID:           userID,
			Message:                "Merge feature branch",
			ExpectedHeadRevisionID: &mainRevision.ID,
			MergeParentRevisionID:  &sideRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create merge revision: %v", err)
	}

	if !mergeRevision.ParentRevisionID.Valid {
		t.Fatal("expected merge revision primary parent to be set")
	}

	actualPrimaryParent :=
		uuid.UUID(mergeRevision.ParentRevisionID.Bytes)

	if actualPrimaryParent != mainRevision.ID {
		t.Fatalf(
			"expected merge primary parent %s, got %s",
			mainRevision.ID,
			actualPrimaryParent,
		)
	}

	if !mergeRevision.MergeParentRevisionID.Valid {
		t.Fatal("expected merge revision second parent to be set")
	}

	actualMergeParent :=
		uuid.UUID(mergeRevision.MergeParentRevisionID.Bytes)

	if actualMergeParent != sideRevision.ID {
		t.Fatalf(
			"expected merge second parent %s, got %s",
			sideRevision.ID,
			actualMergeParent,
		)
	}

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		mainBranchID,
		mergeRevision.ID,
	)

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		sideBranchID,
		sideRevision.ID,
	)
}

func setupVersioningStoreTest(
	t *testing.T,
) (
	context.Context,
	*pgxpool.Pool,
	*database.ProjectStore,
	*dbgen.Queries,
	uuid.UUID,
	uuid.UUID,
	uuid.UUID,
) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
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

	user, err := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{
			Email:       testID + "@versioning.example.com",
			DisplayName: "Versioning Store User",
		},
	)
	if err != nil {
		t.Fatalf("create versioning store user: %v", err)
	}

	store := database.NewProjectStore(pool)

	project, err := store.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: user.ID,
			Name:        "Versioning Store Project",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create versioning store project: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM projects WHERE id = $1",
			project.ID,
		)
		if err != nil {
			t.Errorf("delete versioning store project: %v", err)
		}

		_, err = pool.Exec(
			context.Background(),
			"DELETE FROM users WHERE id = $1",
			user.ID,
		)
		if err != nil {
			t.Errorf("delete versioning store user: %v", err)
		}
	})

	var branchID uuid.UUID
	if err := pool.QueryRow(
		ctx,
		`SELECT id
		 FROM project_branches
		 WHERE project_id = $1
		   AND name = 'main'`,
		project.ID,
	).Scan(&branchID); err != nil {
		t.Fatalf("get default project branch: %v", err)
	}

	return ctx, pool, store, queries, user.ID, project.ID, branchID
}

func assertBranchHead(
	t *testing.T,
	ctx context.Context,
	queries *dbgen.Queries,
	projectID uuid.UUID,
	branchID uuid.UUID,
	expectedRevisionID uuid.UUID,
) {
	t.Helper()

	branch, err := queries.GetProjectBranchByIDAndProject(
		ctx,
		dbgen.GetProjectBranchByIDAndProjectParams{
			BranchID:  branchID,
			ProjectID: projectID,
		},
	)
	if err != nil {
		t.Fatalf("get project branch: %v", err)
	}

	if !branch.HeadRevisionID.Valid {
		t.Fatal("expected project branch head to be set")
	}

	actualRevisionID := uuid.UUID(branch.HeadRevisionID.Bytes)
	if actualRevisionID != expectedRevisionID {
		t.Fatalf(
			"expected branch head %s, got %s",
			expectedRevisionID,
			actualRevisionID,
		)
	}
}
