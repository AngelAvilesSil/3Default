package conversionjobs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
)

var (
	ErrWorkerStoreRequired = errors.New(
		"conversion worker store is required",
	)
	ErrContentReaderRequired = errors.New(
		"conversion content reader is required",
	)
	ErrConverterRequired = errors.New(
		"converter is required",
	)
	ErrIdleDelayInvalid = errors.New(
		"conversion worker idle delay must be positive",
	)
	ErrNoPendingConversionJob = errors.New(
		"no pending conversion job",
	)
	ErrConversionJobNotRunning = errors.New(
		"conversion job is not running",
	)
)

const workerFinalizationTimeout = 5 * time.Second

type WorkerStore interface {
	RequeueRunningConversionJobs(
		ctx context.Context,
	) error
	ClaimNextPendingConversionJob(
		ctx context.Context,
	) (dbgen.ConversionJob, error)
	RequeueConversionJob(
		ctx context.Context,
		conversionJobID uuid.UUID,
	) (dbgen.ConversionJob, error)
	GetProjectFileByIDAndProject(
		ctx context.Context,
		arg dbgen.GetProjectFileByIDAndProjectParams,
	) (dbgen.ProjectFile, error)
	MarkConversionJobSucceeded(
		ctx context.Context,
		conversionJobID uuid.UUID,
	) (dbgen.ConversionJob, error)
	MarkConversionJobFailed(
		ctx context.Context,
		arg dbgen.MarkConversionJobFailedParams,
	) (dbgen.ConversionJob, error)
}

type SourceContentReader interface {
	Open(
		ctx context.Context,
		contentSHA256 string,
	) (io.ReadCloser, error)
}

type Converter interface {
	Convert(
		ctx context.Context,
		projectFile dbgen.ProjectFile,
		source io.Reader,
	) error
}

type Worker struct {
	store     WorkerStore
	content   SourceContentReader
	converter Converter
	idleDelay time.Duration
}

func NewWorker(
	store WorkerStore,
	content SourceContentReader,
	converter Converter,
	idleDelay time.Duration,
) (*Worker, error) {
	if store == nil {
		return nil, ErrWorkerStoreRequired
	}

	if content == nil {
		return nil, ErrContentReaderRequired
	}

	if converter == nil {
		return nil, ErrConverterRequired
	}

	if idleDelay <= 0 {
		return nil, ErrIdleDelayInvalid
	}

	return &Worker{
		store:     store,
		content:   content,
		converter: converter,
		idleDelay: idleDelay,
	}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return nil
	}

	if err := w.store.RequeueRunningConversionJobs(ctx); err != nil {
		return fmt.Errorf(
			"recover running conversion jobs: %w",
			err,
		)
	}

	for {
		processed, err := w.RunOnce(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}

			return err
		}

		if processed {
			continue
		}

		timer := time.NewTimer(w.idleDelay)

		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil

		case <-timer.C:
		}
	}
}

func (w *Worker) RunOnce(
	ctx context.Context,
) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	job, err := w.store.ClaimNextPendingConversionJob(ctx)
	if errors.Is(err, ErrNoPendingConversionJob) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf(
			"claim conversion job: %w",
			err,
		)
	}

	if err := w.processClaimedJob(ctx, job); err != nil {
		if ctx.Err() != nil {
			if requeueErr := w.requeueAfterCancellation(
				job.ID,
			); requeueErr != nil {
				return true, fmt.Errorf(
					"requeue canceled conversion job: %w",
					requeueErr,
				)
			}

			return true, ctx.Err()
		}

		if failErr := w.failJob(
			job.ID,
			err,
		); failErr != nil {
			return true, fmt.Errorf(
				"record conversion job failure: %w",
				failErr,
			)
		}

		return true, nil
	}

	if err := w.succeedJob(job.ID); err != nil {
		return true, fmt.Errorf(
			"record conversion job success: %w",
			err,
		)
	}

	return true, nil
}

func (w *Worker) processClaimedJob(
	ctx context.Context,
	job dbgen.ConversionJob,
) error {
	projectFile, err := w.store.GetProjectFileByIDAndProject(
		ctx,
		dbgen.GetProjectFileByIDAndProjectParams{
			ProjectFileID: job.ProjectFileID,
			ProjectID:     job.ProjectID,
		},
	)
	if err != nil {
		return fmt.Errorf(
			"get conversion source metadata: %w",
			err,
		)
	}

	source, err := w.content.Open(
		ctx,
		projectFile.ContentSha256,
	)
	if err != nil {
		return fmt.Errorf(
			"open conversion source content: %w",
			err,
		)
	}

	convertErr := w.converter.Convert(
		ctx,
		projectFile,
		source,
	)

	closeErr := source.Close()

	if convertErr != nil {
		return fmt.Errorf(
			"convert project file: %w",
			convertErr,
		)
	}

	if closeErr != nil {
		return fmt.Errorf(
			"close conversion source content: %w",
			closeErr,
		)
	}

	return nil
}

func (w *Worker) succeedJob(
	conversionJobID uuid.UUID,
) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		workerFinalizationTimeout,
	)
	defer cancel()

	_, err := w.store.MarkConversionJobSucceeded(
		ctx,
		conversionJobID,
	)
	if err != nil {
		return err
	}

	return nil
}

func (w *Worker) failJob(
	conversionJobID uuid.UUID,
	cause error,
) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		workerFinalizationTimeout,
	)
	defer cancel()

	message := strings.TrimSpace(cause.Error())
	if message == "" {
		message = "conversion failed"
	}

	_, err := w.store.MarkConversionJobFailed(
		ctx,
		dbgen.MarkConversionJobFailedParams{
			LastError:       &message,
			ConversionJobID: conversionJobID,
		},
	)
	if err != nil {
		return err
	}

	return nil
}

func (w *Worker) requeueAfterCancellation(
	conversionJobID uuid.UUID,
) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		workerFinalizationTimeout,
	)
	defer cancel()

	_, err := w.store.RequeueConversionJob(
		ctx,
		conversionJobID,
	)
	if errors.Is(err, ErrConversionJobNotRunning) {
		return nil
	}

	return err
}
