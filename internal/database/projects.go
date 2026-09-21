package database

import (
	"context"
	"fmt"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/projects"
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
			err,
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

var _ projects.ProjectStore = (*ProjectStore)(nil)
