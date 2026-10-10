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

func TestProjectFileRowsRemainImmutableAfterRevisionCreation(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	projectFile := createVersioningProjectFile(
		t, ctx, pool, queries, userID, projectID,
		"immutable-source",
	)

	replacementHash := newVersioningProjectFileSHA256(
		"replacement-source",
	)
	_, err := queries.EnsureContentObject(
		ctx,
		dbgen.EnsureContentObjectParams{
			Sha256:    replacementHash,
			SizeBytes: 10,
		},
	)
	if err != nil {
		t.Fatalf("create replacement content object: %v", err)
	}
	t.Cleanup(func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM content_objects WHERE sha256 = $1",
			replacementHash,
		)
		if err != nil {
			t.Errorf("delete replacement content object: %v", err)
		}
	})

	revision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:      projectID,
			BranchID:       branchID,
			AuthorUserID:   userID,
			Message:        "Immutable source snapshot",
			ProjectFileIDs: []uuid.UUID{projectFile.ID},
		},
	)
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}

	tests := []struct {
		name  string
		query string
		value string
	}{
		{
			name: "content hash",
			query: `UPDATE project_files
				SET content_sha256 = $1
				WHERE project_id = $2 AND id = $3`,
			value: replacementHash,
		},
		{
			name: "filename",
			query: `UPDATE project_files
				SET original_filename = $1
				WHERE project_id = $2 AND id = $3`,
			value: "rewritten.step",
		},
		{
			name: "media type",
			query: `UPDATE project_files
				SET media_type = $1
				WHERE project_id = $2 AND id = $3`,
			value: "model/step",
		},
		{
			name: "no-op update",
			query: `UPDATE project_files
				SET original_filename = $1
				WHERE project_id = $2 AND id = $3`,
			value: projectFile.OriginalFilename,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pool.Exec(
				ctx, tt.query,
				tt.value, projectID, projectFile.ID,
			)

			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) ||
				pgErr.Code != "P0001" ||
				pgErr.Message != "project_files rows are immutable" {
				t.Fatalf(
					"expected project-file immutability error, got %v",
					err,
				)
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
		t.Fatalf("read historical file snapshot: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected one historical file, got %d", len(files))
	}
	if files[0].ID != projectFile.ID ||
		files[0].ContentSha256 != projectFile.ContentSha256 ||
		files[0].OriginalFilename != projectFile.OriginalFilename {
		t.Fatalf(
			"historical project-file identity changed: %+v",
			files[0],
		)
	}
}

func TestProjectFileImmutabilityAllowsProjectDeletion(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	projectFile := createVersioningProjectFile(
		t, ctx, pool, queries, userID, projectID,
		"deleted-project-source",
	)

	_, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:      projectID,
			BranchID:       branchID,
			AuthorUserID:   userID,
			Message:        "Revision before project deletion",
			ProjectFileIDs: []uuid.UUID{projectFile.ID},
		},
	)
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		"DELETE FROM projects WHERE id = $1",
		projectID,
	)
	if err != nil {
		t.Fatalf("delete project with historical file: %v", err)
	}

	var remainingFiles int
	if err := pool.QueryRow(
		ctx,
		"SELECT count(*) FROM project_files WHERE project_id = $1",
		projectID,
	).Scan(&remainingFiles); err != nil {
		t.Fatalf("count remaining project files: %v", err)
	}
	if remainingFiles != 0 {
		t.Fatalf(
			"expected zero files after project deletion, got %d",
			remainingFiles,
		)
	}
}
