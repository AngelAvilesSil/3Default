package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/AngelAvilesSil/3Default/internal/conversionjobs"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConversionJobStore struct {
	*dbgen.Queries
}

func NewConversionJobStore(
	pool *pgxpool.Pool,
) *ConversionJobStore {
	return &ConversionJobStore{
		Queries: dbgen.New(pool),
	}
}

func (s *ConversionJobStore) CreateConversionJob(
	ctx context.Context,
	arg dbgen.CreateConversionJobParams,
) (dbgen.ConversionJob, error) {
	job, err := s.Queries.CreateConversionJob(ctx, arg)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ConversionJob{},
			conversionjobs.ErrProjectFileNotFound
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName ==
				"conversion_jobs_one_active_per_project_file" {
			return dbgen.ConversionJob{},
				conversionjobs.ErrActiveConversionJobExists
		}

		return dbgen.ConversionJob{}, fmt.Errorf(
			"create conversion job row: %w",
			err,
		)
	}

	return job, nil
}

func (s *ConversionJobStore) GetConversionJobByIDAndProject(
	ctx context.Context,
	arg dbgen.GetConversionJobByIDAndProjectParams,
) (dbgen.ConversionJob, error) {
	return s.Queries.GetConversionJobByIDAndProject(
		ctx,
		arg,
	)
}

func (s *ConversionJobStore) ListConversionJobsByProjectFile(
	ctx context.Context,
	arg dbgen.ListConversionJobsByProjectFileParams,
) ([]dbgen.ConversionJob, error) {
	return s.Queries.ListConversionJobsByProjectFile(
		ctx,
		arg,
	)
}

var _ conversionjobs.Store = (*ConversionJobStore)(nil)
