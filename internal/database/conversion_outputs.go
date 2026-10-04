package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/AngelAvilesSil/3Default/internal/conversionjobs"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/jackc/pgx/v5"
)

func (s *ConversionJobStore) FinalizeConversionJobSuccess(
	ctx context.Context,
	input conversionjobs.FinalizeSuccessInput,
) (dbgen.ConversionJobOutput, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return dbgen.ConversionJobOutput{}, fmt.Errorf(
			"begin conversion success transaction: %w",
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
			Sha256:    input.ContentSHA256,
			SizeBytes: input.SizeBytes,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ConversionJobOutput{},
			conversionjobs.ErrOutputContentSizeConflict
	}
	if err != nil {
		return dbgen.ConversionJobOutput{}, fmt.Errorf(
			"ensure conversion output content object: %w",
			err,
		)
	}

	output, err := queries.CreateConversionJobOutput(
		ctx,
		dbgen.CreateConversionJobOutputParams{
			ContentSha256:   input.ContentSHA256,
			MediaType:       input.MediaType,
			ConversionJobID: input.ConversionJobID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ConversionJobOutput{},
			conversionjobs.ErrConversionJobNotRunning
	}
	if err != nil {
		return dbgen.ConversionJobOutput{}, fmt.Errorf(
			"create conversion job output: %w",
			err,
		)
	}

	_, err = queries.MarkConversionJobSucceeded(
		ctx,
		input.ConversionJobID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ConversionJobOutput{},
			conversionjobs.ErrConversionJobNotRunning
	}
	if err != nil {
		return dbgen.ConversionJobOutput{}, fmt.Errorf(
			"mark conversion job succeeded: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return dbgen.ConversionJobOutput{}, fmt.Errorf(
			"commit conversion success transaction: %w",
			err,
		)
	}

	return output, nil
}
