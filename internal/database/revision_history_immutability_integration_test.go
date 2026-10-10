//go:build integration

package database_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/versioning"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func requireRevisionHistoryRejection(
	t *testing.T,
	err error,
	expectedMessage string,
) {
	t.Helper()

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) ||
		pgErr.Code != "P0001" ||
		pgErr.Message != expectedMessage {
		t.Fatalf(
			"expected revision-history error %q (P0001), got %v",
			expectedMessage,
			err,
		)
	}
}

func TestRevisionHistoryRejectsMetadataAndParentEdits(t *testing.T) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	root, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     branchID,
			AuthorUserID: userID,
			Message:      "Original root",
		},
	)
	if err != nil {
		t.Fatalf("create root revision: %v", err)
	}

	child, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               branchID,
			AuthorUserID:           userID,
			Message:                "Original child",
			ExpectedHeadRevisionID: &root.ID,
		},
	)
	if err != nil {
		t.Fatalf("create child revision: %v", err)
	}

	otherBranch, err := queries.CreateProjectBranch(
		ctx,
		dbgen.CreateProjectBranchParams{
			ProjectID: projectID,
			Name:      "alternate-history",
		},
	)
	if err != nil {
		t.Fatalf("create alternate branch: %v", err)
	}

	otherRoot, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     otherBranch.ID,
			AuthorUserID: userID,
			Message:      "Alternate root",
		},
	)
	if err != nil {
		t.Fatalf("create alternate root: %v", err)
	}

	tests := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "message rewrite",
			sql: `UPDATE project_revisions
				SET message = $1
				WHERE project_id = $2 AND id = $3`,
			args: []any{"Rewritten message", projectID, child.ID},
		},
		{
			name: "author no-op update",
			sql: `UPDATE project_revisions
				SET author_user_id = author_user_id
				WHERE project_id = $1 AND id = $2`,
			args: []any{projectID, child.ID},
		},
		{
			name: "creation time rewrite",
			sql: `UPDATE project_revisions
				SET created_at = created_at + INTERVAL '1 second'
				WHERE project_id = $1 AND id = $2`,
			args: []any{projectID, child.ID},
		},
		{
			name: "first parent removal",
			sql: `UPDATE project_revisions
				SET parent_revision_id = NULL
				WHERE project_id = $1 AND id = $2`,
			args: []any{projectID, child.ID},
		},
		{
			name: "merge parent addition",
			sql: `UPDATE project_revisions
				SET merge_parent_revision_id = $1
				WHERE project_id = $2 AND id = $3`,
			args: []any{otherRoot.ID, projectID, child.ID},
		},
		{
			name: "revision identity no-op update",
			sql: `UPDATE project_revisions
				SET id = id
				WHERE project_id = $1 AND id = $2`,
			args: []any{projectID, child.ID},
		},
		{
			name: "message no-op update",
			sql: `UPDATE project_revisions
				SET message = message
				WHERE project_id = $1 AND id = $2`,
			args: []any{projectID, child.ID},
		},
		{
			name: "repeat finalization",
			sql: `UPDATE project_revisions
				SET membership_finalized = TRUE
				WHERE project_id = $1 AND id = $2`,
			args: []any{projectID, child.ID},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tt.sql, tt.args...)

			requireRevisionHistoryRejection(
				t,
				err,
				"project revision history is immutable",
			)
		})
	}

	persisted, err := queries.GetProjectRevisionByIDAndProject(
		ctx,
		dbgen.GetProjectRevisionByIDAndProjectParams{
			ProjectID:  projectID,
			RevisionID: child.ID,
		},
	)
	if err != nil {
		t.Fatalf("read historical revision: %v", err)
	}

	if persisted.Message != child.Message ||
		persisted.AuthorUserID != child.AuthorUserID ||
		!persisted.CreatedAt.Equal(child.CreatedAt) ||
		!persisted.ParentRevisionID.Valid ||
		persisted.ParentRevisionID.Bytes != root.ID ||
		persisted.MergeParentRevisionID.Valid ||
		!persisted.MembershipFinalized {
		t.Fatalf("historical revision changed: %+v", persisted)
	}
}

func TestRevisionHistoryRejectsIndependentDeletion(t *testing.T) {
	ctx, pool, _, _, userID, projectID, _ :=
		setupVersioningStoreTest(t)

	// Create an unreferenced finalized revision with an empty snapshot.
	// No branch, child revision, or membership row will prevent deletion
	// through a foreign key. The history trigger must reject it itself.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin revision transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	revisionID := uuid.New()

	_, err = tx.Exec(
		ctx,
		`INSERT INTO project_revisions
			(id, project_id, author_user_id, message)
		 VALUES ($1, $2, $3, $4)`,
		revisionID,
		projectID,
		userID,
		"Unreferenced finalized revision",
	)
	if err != nil {
		t.Fatalf("insert unreferenced revision: %v", err)
	}

	_, err = tx.Exec(
		ctx,
		`UPDATE project_revisions
		 SET membership_finalized = TRUE
		 WHERE project_id = $1 AND id = $2`,
		projectID,
		revisionID,
	)
	if err != nil {
		t.Fatalf("finalize unreferenced revision: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit unreferenced revision: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`DELETE FROM project_revisions
		 WHERE project_id = $1 AND id = $2`,
		projectID,
		revisionID,
	)

	requireRevisionHistoryRejection(
		t,
		err,
		"project revisions cannot be deleted independently",
	)

	// Deleting the owning project must still be permitted.
	_, err = pool.Exec(
		ctx,
		"DELETE FROM projects WHERE id = $1",
		projectID,
	)
	if err != nil {
		t.Fatalf("delete project with revision history: %v", err)
	}

	var count int
	if err := pool.QueryRow(
		ctx,
		"SELECT count(*) FROM project_revisions WHERE id = $1",
		revisionID,
	).Scan(&count); err != nil {
		t.Fatalf("count remaining revisions: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected revision cascade cleanup, got %d", count)
	}
}

func TestRevisionHistoryRejectsTruncate(t *testing.T) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	revision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     branchID,
			AuthorUserID: userID,
			Message:      "History must survive truncation",
		},
	)
	if err != nil {
		t.Fatalf("create historical revision: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin truncation test transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	truncateCtx, cancel := context.WithTimeout(
		ctx,
		5*time.Second,
	)
	defer cancel()

	_, err = tx.Exec(
		truncateCtx,
		"TRUNCATE TABLE project_revisions CASCADE",
	)

	requireRevisionHistoryRejection(
		t,
		err,
		"project revision history cannot be truncated",
	)

	if err := tx.Rollback(context.Background()); err != nil {
		t.Fatalf("roll back rejected truncation: %v", err)
	}

	persisted, err := queries.GetProjectRevisionByIDAndProject(
		ctx,
		dbgen.GetProjectRevisionByIDAndProjectParams{
			ProjectID:  projectID,
			RevisionID: revision.ID,
		},
	)
	if err != nil {
		t.Fatalf("read revision after rejected truncation: %v", err)
	}

	if persisted.ID != revision.ID ||
		persisted.Message != revision.Message ||
		!persisted.MembershipFinalized {
		t.Fatalf("historical revision changed: %+v", persisted)
	}
}
