package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/filestorage"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FileStore struct {
	pool *pgxpool.Pool
	*dbgen.Queries
}

func NewFileStore(
	pool *pgxpool.Pool,
) *FileStore {
	return &FileStore{
		pool:    pool,
		Queries: dbgen.New(pool),
	}
}

func (s *FileStore) CreateProjectFileWithContentObject(
	ctx context.Context,
	arg filestorage.CreateProjectFileParams,
) (dbgen.ProjectFile, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return dbgen.ProjectFile{}, fmt.Errorf(
			"begin project file creation transaction: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	queries := s.Queries.WithTx(tx)

	_, err = queries.EnsureContentObject(
		ctx,
		dbgen.EnsureContentObjectParams{
			Sha256:    arg.ContentSHA256,
			SizeBytes: arg.SizeBytes,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ProjectFile{},
			filestorage.ErrContentObjectSizeConflict
	}
	if err != nil {
		return dbgen.ProjectFile{}, fmt.Errorf(
			"ensure content object: %w",
			err,
		)
	}

	projectFile, err := queries.CreateProjectFile(
		ctx,
		dbgen.CreateProjectFileParams{
			ProjectID:        arg.ProjectID,
			UploadedByUserID: arg.UploadedByUserID,
			ContentSha256:    arg.ContentSHA256,
			OriginalFilename: arg.OriginalFilename,
			MediaType:        arg.MediaType,
		},
	)
	if err != nil {
		return dbgen.ProjectFile{}, fmt.Errorf(
			"create project file row: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return dbgen.ProjectFile{}, fmt.Errorf(
			"commit project file creation transaction: %w",
			err,
		)
	}

	return projectFile, nil
}

var _ filestorage.FileStore = (*FileStore)(nil)
