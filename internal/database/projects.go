package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/projects"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultProjectBranchName = "main"

type ProjectStore struct {
	pool *pgxpool.Pool
	*dbgen.Queries
}

func NewProjectStore(pool *pgxpool.Pool) *ProjectStore {
	return &ProjectStore{
		pool:    pool,
		Queries: dbgen.New(pool),
	}
}

func (s *ProjectStore) CreateProject(
	ctx context.Context,
	arg dbgen.CreateProjectParams,
) (dbgen.Project, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return dbgen.Project{}, fmt.Errorf(
			"begin project creation transaction: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	queries := s.Queries.WithTx(tx)

	project, err := queries.CreateProject(ctx, arg)
	if err != nil {
		return dbgen.Project{}, fmt.Errorf(
			"create project row: %w",
			mapProjectNameError(err),
		)
	}

	_, err = queries.CreateProjectBranch(
		ctx,
		dbgen.CreateProjectBranchParams{
			ProjectID: project.ID,
			Name:      defaultProjectBranchName,
		},
	)
	if err != nil {
		return dbgen.Project{}, fmt.Errorf(
			"create default project branch: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return dbgen.Project{}, fmt.Errorf(
			"commit project creation transaction: %w",
			err,
		)
	}

	return project, nil
}

func (s *ProjectStore) UpdateProjectMetadataByIDAndOwner(
	ctx context.Context,
	arg dbgen.UpdateProjectMetadataByIDAndOwnerParams,
) (dbgen.Project, error) {
	project, err := s.Queries.UpdateProjectMetadataByIDAndOwner(ctx, arg)
	if err != nil {
		return dbgen.Project{}, fmt.Errorf(
			"update project row: %w",
			mapProjectNameError(err),
		)
	}

	return project, nil
}

var _ projects.ProjectStore = (*ProjectStore)(nil)

const projectNameUniqueIndex = "projects_owner_name_case_insensitive_unique"

func mapProjectNameError(err error) error {
	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) &&
		pgErr.Code == "23505" &&
		pgErr.ConstraintName == projectNameUniqueIndex {
		return projects.ErrNameConflict
	}

	return err
}
