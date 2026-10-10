//go:build integration

package database_test

import (
	"context"
	"errors"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/versioning"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestFinalizedRevisionMembershipCannotChange(t *testing.T) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	originalFile := createVersioningProjectFile(
		t, ctx, pool, queries, userID, projectID, "membership-original",
	)
	additionalFile := createVersioningProjectFile(
		t, ctx, pool, queries, userID, projectID, "membership-additional",
	)

	revision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:      projectID,
			BranchID:       branchID,
			AuthorUserID:   userID,
			Message:        "Finalized snapshot",
			ProjectFileIDs: []uuid.UUID{originalFile.ID},
		},
	)
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}
	if !revision.MembershipFinalized {
		t.Fatal("returned revision should be finalized")
	}

	persisted, err := queries.GetProjectRevisionByIDAndProject(
		ctx,
		dbgen.GetProjectRevisionByIDAndProjectParams{
			ProjectID:  projectID,
			RevisionID: revision.ID,
		},
	)
	if err != nil {
		t.Fatalf("read persisted revision: %v", err)
	}
	if !persisted.MembershipFinalized {
		t.Fatal("persisted revision should be finalized")
	}

	tests := []struct {
		name string
		sql  string
		args []any
	}{
		{
			name: "late membership insert",
			sql: `INSERT INTO project_revision_files
				(project_id, revision_id, project_file_id)
				VALUES ($1, $2, $3)`,
			args: []any{projectID, revision.ID, additionalFile.ID},
		},
		{
			name: "membership update",
			sql: `UPDATE project_revision_files
				SET project_file_id = $1
				WHERE project_id = $2 AND revision_id = $3
				  AND project_file_id = $4`,
			args: []any{
				additionalFile.ID, projectID, revision.ID, originalFile.ID,
			},
		},
		{
			name: "membership deletion",
			sql: `DELETE FROM project_revision_files
				WHERE project_id = $1 AND revision_id = $2
				  AND project_file_id = $3`,
			args: []any{projectID, revision.ID, originalFile.ID},
		},
		{
			name: "revision reopening",
			sql: `UPDATE project_revisions
				SET membership_finalized = FALSE
				WHERE project_id = $1 AND id = $2`,
			args: []any{projectID, revision.ID},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, tt.sql, tt.args...)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
				t.Fatalf("expected immutability error P0001, got %v", err)
			}
		})
	}

	files, err := queries.ListProjectFilesByRevision(
		ctx,
		dbgen.ListProjectFilesByRevisionParams{
			ProjectID:  projectID,
			RevisionID: revision.ID,
		},
	)
	if err != nil {
		t.Fatalf("read historical snapshot: %v", err)
	}
	if len(files) != 1 || files[0].ID != originalFile.ID {
		t.Fatalf("finalized membership changed: %+v", files)
	}
}

func TestRevisionCannotCommitWithoutFinalization(t *testing.T) {
	ctx, pool, _, _, userID, projectID, _ :=
		setupVersioningStoreTest(t)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
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
		"Unfinalized candidate",
	)
	if err != nil {
		t.Fatalf("insert candidate revision: %v", err)
	}

	err = tx.Commit(ctx)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "P0001" {
		t.Fatalf("expected commit-time finalization error, got %v", err)
	}

	var count int
	if err := pool.QueryRow(
		ctx,
		"SELECT count(*) FROM project_revisions WHERE id = $1",
		revisionID,
	).Scan(&count); err != nil {
		t.Fatalf("count rolled-back revision: %v", err)
	}
	if count != 0 {
		t.Fatalf("unfinalized revision persisted: %d rows", count)
	}
}

func TestFinalizedRevisionMembershipRejectsTruncate(t *testing.T) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	projectFile := createVersioningProjectFile(
		t, ctx, pool, queries, userID, projectID,
		"truncate-protection",
	)

	revision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:      projectID,
			BranchID:       branchID,
			AuthorUserID:   userID,
			Message:        "Protected against truncation",
			ProjectFileIDs: []uuid.UUID{projectFile.ID},
		},
	)
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}

	_, err = pool.Exec(ctx, "TRUNCATE TABLE project_revision_files")

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) ||
		pgErr.Code != "P0001" ||
		pgErr.Message != "revision file membership cannot be truncated" {
		t.Fatalf("expected truncation rejection, got %v", err)
	}

	files, err := queries.ListProjectFilesByRevision(
		ctx,
		dbgen.ListProjectFilesByRevisionParams{
			ProjectID:  projectID,
			RevisionID: revision.ID,
		},
	)
	if err != nil {
		t.Fatalf("read protected revision: %v", err)
	}
	if len(files) != 1 || files[0].ID != projectFile.ID {
		t.Fatalf("revision membership changed: %+v", files)
	}
}
