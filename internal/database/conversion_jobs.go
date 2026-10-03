package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/AngelAvilesSil/3Default/internal/conversionjobs"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
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

func (s *ConversionJobStore) ClaimNextPendingConversionJob(
	ctx context.Context,
) (dbgen.ConversionJob, error) {
	job, err := s.Queries.ClaimNextPendingConversionJob(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ConversionJob{},
			conversionjobs.ErrNoPendingConversionJob
	}
	if err != nil {
		return dbgen.ConversionJob{}, fmt.Errorf(
			"claim next conversion job row: %w",
			err,
		)
	}

	return job, nil
}

func (s *ConversionJobStore) RequeueConversionJob(
	ctx context.Context,
	conversionJobID uuid.UUID,
) (dbgen.ConversionJob, error) {
	job, err := s.Queries.RequeueConversionJob(
		ctx,
		conversionJobID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ConversionJob{},
			conversionjobs.ErrConversionJobNotRunning
	}
	if err != nil {
		return dbgen.ConversionJob{}, fmt.Errorf(
			"requeue conversion job row: %w",
			err,
		)
	}

	return job, nil
}

func (s *ConversionJobStore) MarkConversionJobSucceeded(
	ctx context.Context,
	conversionJobID uuid.UUID,
) (dbgen.ConversionJob, error) {
	job, err := s.Queries.MarkConversionJobSucceeded(
		ctx,
		conversionJobID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ConversionJob{},
			conversionjobs.ErrConversionJobNotRunning
	}
	if err != nil {
		return dbgen.ConversionJob{}, fmt.Errorf(
			"mark conversion job succeeded: %w",
			err,
		)
	}

	return job, nil
}

func (s *ConversionJobStore) MarkConversionJobFailed(
	ctx context.Context,
	arg dbgen.MarkConversionJobFailedParams,
) (dbgen.ConversionJob, error) {
	job, err := s.Queries.MarkConversionJobFailed(ctx, arg)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ConversionJob{},
			conversionjobs.ErrConversionJobNotRunning
	}
	if err != nil {
		return dbgen.ConversionJob{}, fmt.Errorf(
			"mark conversion job failed: %w",
			err,
		)
	}

	return job, nil
}

var _ conversionjobs.WorkerStore = (*ConversionJobStore)(nil)
