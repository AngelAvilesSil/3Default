package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/versioning"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *ProjectStore) CreateBranch(
	ctx context.Context,
	arg versioning.CreateBranchParams,
) (dbgen.ProjectBranch, error) {
	branch, err := s.Queries.CreateProjectBranchWithHead(
		ctx,
		dbgen.CreateProjectBranchWithHeadParams{
			ProjectID:      arg.ProjectID,
			Name:           arg.Name,
			HeadRevisionID: nullableUUID(arg.HeadRevisionID),
		},
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) &&
			pgErr.Code == "23505" &&
			pgErr.ConstraintName ==
				"project_branches_project_name_unique" {
			return dbgen.ProjectBranch{},
				versioning.ErrBranchNameConflict
		}

		return dbgen.ProjectBranch{}, fmt.Errorf(
			"create project branch: %w",
			err,
		)
	}

	return branch, nil
}

func (s *ProjectStore) CreateRevisionOnBranch(
	ctx context.Context,
	arg versioning.CreateRevisionOnBranchParams,
) (dbgen.ProjectRevision, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return dbgen.ProjectRevision{}, fmt.Errorf(
			"begin revision creation transaction: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	queries := s.Queries.WithTx(tx)

	expectedHead := nullableUUID(arg.ExpectedHeadRevisionID)

	revision, err := queries.CreateProjectRevision(
		ctx,
		dbgen.CreateProjectRevisionParams{
			ProjectID:             arg.ProjectID,
			AuthorUserID:          arg.AuthorUserID,
			Message:               arg.Message,
			ParentRevisionID:      expectedHead,
			MergeParentRevisionID: nullableUUID(arg.MergeParentRevisionID),
		},
	)
	if err != nil {
		return dbgen.ProjectRevision{}, fmt.Errorf(
			"create project revision: %w",
			err,
		)
	}

	_, err = queries.AdvanceProjectBranchHead(
		ctx,
		dbgen.AdvanceProjectBranchHeadParams{
			NewHeadRevisionID: pgtype.UUID{
				Bytes: revision.ID,
				Valid: true,
			},
			BranchID:               arg.BranchID,
			ProjectID:              arg.ProjectID,
			ExpectedHeadRevisionID: expectedHead,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		_, branchErr := queries.GetProjectBranchByIDAndProject(
			ctx,
			dbgen.GetProjectBranchByIDAndProjectParams{
				BranchID:  arg.BranchID,
				ProjectID: arg.ProjectID,
			},
		)

		switch {
		case errors.Is(branchErr, pgx.ErrNoRows):
			return dbgen.ProjectRevision{},
				versioning.ErrBranchNotFound
		case branchErr != nil:
			return dbgen.ProjectRevision{}, fmt.Errorf(
				"resolve project branch after head update failure: %w",
				branchErr,
			)
		default:
			return dbgen.ProjectRevision{},
				versioning.ErrBranchHeadConflict
		}
	}
	if err != nil {
		return dbgen.ProjectRevision{}, fmt.Errorf(
			"advance project branch head: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return dbgen.ProjectRevision{}, fmt.Errorf(
			"commit revision creation transaction: %w",
			err,
		)
	}

	return revision, nil
}

func nullableUUID(value *uuid.UUID) pgtype.UUID {
	if value == nil {
		return pgtype.UUID{}
	}

	return pgtype.UUID{
		Bytes: *value,
		Valid: true,
	}
}

var _ versioning.RevisionStore = (*ProjectStore)(nil)
