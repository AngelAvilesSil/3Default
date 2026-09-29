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
	"github.com/AngelAvilesSil/3Default/internal/filestorage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFileStoreCreatesContentObjectAndProjectFileAtomically(
	t *testing.T,
) {
	ctx, _, store, queries, userID, projectID, trackContentHash :=
		setupFileStoreTest(t)

	contentSHA256 := newFileStoreTestSHA256(
		"atomic-project-file",
	)
	trackContentHash(contentSHA256)

	mediaType := "model/step"

	projectFile, err := store.CreateProjectFileWithContentObject(
		ctx,
		filestorage.CreateProjectFileParams{
			ProjectID:        projectID,
			UploadedByUserID: userID,
			ContentSHA256:    contentSHA256,
			SizeBytes:        512,
			OriginalFilename: "gripper.step",
			MediaType:        &mediaType,
		},
	)
	if err != nil {
		t.Fatalf("create project file with content object: %v", err)
	}

	if projectFile.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			projectFile.ProjectID,
		)
	}

	if projectFile.UploadedByUserID != userID {
		t.Fatalf(
			"expected uploader ID %s, got %s",
			userID,
			projectFile.UploadedByUserID,
		)
	}

	if projectFile.ContentSha256 != contentSHA256 {
		t.Fatalf(
			"expected content SHA-256 %q, got %q",
			contentSHA256,
			projectFile.ContentSha256,
		)
	}

	if projectFile.OriginalFilename != "gripper.step" {
		t.Fatalf(
			"expected filename %q, got %q",
			"gripper.step",
			projectFile.OriginalFilename,
		)
	}

	if projectFile.MediaType == nil ||
		*projectFile.MediaType != mediaType {
		t.Fatalf(
			"expected media type %q, got %v",
			mediaType,
			projectFile.MediaType,
		)
	}

	contentObject, err := queries.GetContentObjectBySHA256(
		ctx,
		contentSHA256,
	)
	if err != nil {
		t.Fatalf("get created content object: %v", err)
	}

	if contentObject.SizeBytes != 512 {
		t.Fatalf(
			"expected content size %d, got %d",
			512,
			contentObject.SizeBytes,
		)
	}

	foundFile, err := queries.GetProjectFileByIDAndProject(
		ctx,
		dbgen.GetProjectFileByIDAndProjectParams{
			ProjectFileID: projectFile.ID,
			ProjectID:     projectID,
		},
	)
	if err != nil {
		t.Fatalf("get created project file: %v", err)
	}

	if foundFile.ID != projectFile.ID {
		t.Fatalf(
			"expected project file ID %s, got %s",
			projectFile.ID,
			foundFile.ID,
		)
	}
}

func TestFileStoreReusesMatchingContentObject(
	t *testing.T,
) {
	ctx, pool, store, _, userID, projectID, trackContentHash :=
		setupFileStoreTest(t)

	contentSHA256 := newFileStoreTestSHA256(
		"reused-content-object",
	)
	trackContentHash(contentSHA256)

	firstFile, err := store.CreateProjectFileWithContentObject(
		ctx,
		filestorage.CreateProjectFileParams{
			ProjectID:        projectID,
			UploadedByUserID: userID,
			ContentSHA256:    contentSHA256,
			SizeBytes:        1024,
			OriginalFilename: "first.step",
		},
	)
	if err != nil {
		t.Fatalf("create first project file: %v", err)
	}

	secondFile, err := store.CreateProjectFileWithContentObject(
		ctx,
		filestorage.CreateProjectFileParams{
			ProjectID:        projectID,
			UploadedByUserID: userID,
			ContentSHA256:    contentSHA256,
			SizeBytes:        1024,
			OriginalFilename: "second.step",
		},
	)
	if err != nil {
		t.Fatalf("create second project file: %v", err)
	}

	if firstFile.ID == secondFile.ID {
		t.Fatalf(
			"expected distinct project file IDs, both were %s",
			firstFile.ID,
		)
	}

	if firstFile.ContentSha256 != contentSHA256 ||
		secondFile.ContentSha256 != contentSHA256 {
		t.Fatalf(
			"expected both project files to reference %q",
			contentSHA256,
		)
	}

	var contentObjectCount int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM content_objects
		 WHERE sha256 = $1`,
		contentSHA256,
	).Scan(&contentObjectCount); err != nil {
		t.Fatalf("count matching content objects: %v", err)
	}

	if contentObjectCount != 1 {
		t.Fatalf(
			"expected 1 shared content object, got %d",
			contentObjectCount,
		)
	}

	var projectFileCount int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_files
		 WHERE project_id = $1
		   AND content_sha256 = $2`,
		projectID,
		contentSHA256,
	).Scan(&projectFileCount); err != nil {
		t.Fatalf("count project file references: %v", err)
	}

	if projectFileCount != 2 {
		t.Fatalf(
			"expected 2 project file references, got %d",
			projectFileCount,
		)
	}
}

func TestFileStoreMapsContentObjectSizeConflict(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, trackContentHash :=
		setupFileStoreTest(t)

	contentSHA256 := newFileStoreTestSHA256(
		"content-size-conflict",
	)
	trackContentHash(contentSHA256)

	_, err := store.CreateProjectFileWithContentObject(
		ctx,
		filestorage.CreateProjectFileParams{
			ProjectID:        projectID,
			UploadedByUserID: userID,
			ContentSHA256:    contentSHA256,
			SizeBytes:        2048,
			OriginalFilename: "original.step",
		},
	)
	if err != nil {
		t.Fatalf("create original project file: %v", err)
	}

	_, err = store.CreateProjectFileWithContentObject(
		ctx,
		filestorage.CreateProjectFileParams{
			ProjectID:        projectID,
			UploadedByUserID: userID,
			ContentSHA256:    contentSHA256,
			SizeBytes:        2049,
			OriginalFilename: "conflicting.step",
		},
	)
	if !errors.Is(
		err,
		filestorage.ErrContentObjectSizeConflict,
	) {
		t.Fatalf(
			"expected content object size conflict, got %v",
			err,
		)
	}

	contentObject, err := queries.GetContentObjectBySHA256(
		ctx,
		contentSHA256,
	)
	if err != nil {
		t.Fatalf("get original content object: %v", err)
	}

	if contentObject.SizeBytes != 2048 {
		t.Fatalf(
			"expected original content size %d, got %d",
			2048,
			contentObject.SizeBytes,
		)
	}

	var projectFileCount int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_files
		 WHERE project_id = $1
		   AND content_sha256 = $2`,
		projectID,
		contentSHA256,
	).Scan(&projectFileCount); err != nil {
		t.Fatalf(
			"count project files after size conflict: %v",
			err,
		)
	}

	if projectFileCount != 1 {
		t.Fatalf(
			"expected size conflict to leave 1 project file, got %d",
			projectFileCount,
		)
	}
}

func TestFileStoreRollsBackContentObjectWhenProjectFileCreationFails(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, _, trackContentHash :=
		setupFileStoreTest(t)

	contentSHA256 := newFileStoreTestSHA256(
		"rolled-back-content-object",
	)
	trackContentHash(contentSHA256)

	_, err := store.CreateProjectFileWithContentObject(
		ctx,
		filestorage.CreateProjectFileParams{
			ProjectID:        uuid.New(),
			UploadedByUserID: userID,
			ContentSHA256:    contentSHA256,
			SizeBytes:        4096,
			OriginalFilename: "orphan.step",
		},
	)
	if err == nil {
		t.Fatal(
			"expected project file creation with missing project to fail",
		)
	}

	if errors.Is(
		err,
		filestorage.ErrContentObjectSizeConflict,
	) {
		t.Fatalf(
			"expected project foreign-key failure, got size conflict: %v",
			err,
		)
	}

	_, err = queries.GetContentObjectBySHA256(
		ctx,
		contentSHA256,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected failed project file transaction to roll back content object, got %v",
			err,
		)
	}

	var projectFileCount int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_files
		 WHERE content_sha256 = $1`,
		contentSHA256,
	).Scan(&projectFileCount); err != nil {
		t.Fatalf(
			"count project files after rollback: %v",
			err,
		)
	}

	if projectFileCount != 0 {
		t.Fatalf(
			"expected rollback to leave 0 project files, got %d",
			projectFileCount,
		)
	}
}

func setupFileStoreTest(
	t *testing.T,
) (
	context.Context,
	*pgxpool.Pool,
	*database.FileStore,
	*dbgen.Queries,
	uuid.UUID,
	uuid.UUID,
	func(string),
) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip(
			"DATABASE_URL is required for database integration tests",
		)
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
			Email:       testID + "@file-storage.example.com",
			DisplayName: "File Storage Store User",
		},
	)
	if err != nil {
		t.Fatalf("create file storage store user: %v", err)
	}

	projectStore := database.NewProjectStore(pool)

	project, err := projectStore.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: user.ID,
			Name:        "File Storage Store Project",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create file storage store project: %v", err)
	}

	var contentHashes []string

	trackContentHash := func(contentSHA256 string) {
		contentHashes = append(
			contentHashes,
			contentSHA256,
		)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM projects WHERE id = $1",
			project.ID,
		)
		if err != nil {
			t.Errorf(
				"delete file storage store project: %v",
				err,
			)
		}

		for _, contentSHA256 := range contentHashes {
			_, err := pool.Exec(
				context.Background(),
				"DELETE FROM content_objects WHERE sha256 = $1",
				contentSHA256,
			)
			if err != nil {
				t.Errorf(
					"delete file storage content object %q: %v",
					contentSHA256,
					err,
				)
			}
		}

		_, err = pool.Exec(
			context.Background(),
			"DELETE FROM users WHERE id = $1",
			user.ID,
		)
		if err != nil {
			t.Errorf(
				"delete file storage store user: %v",
				err,
			)
		}
	})

	return ctx,
		pool,
		database.NewFileStore(pool),
		queries,
		user.ID,
		project.ID,
		trackContentHash
}

func newFileStoreTestSHA256(
	label string,
) string {
	digest := sha256.Sum256(
		[]byte(label + ":" + uuid.New().String()),
	)

	return hex.EncodeToString(digest[:])
}
