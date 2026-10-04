package conversionjobs

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/storage"
	"github.com/google/uuid"
)

var (
	ErrWorkerStoreRequired = errors.New(
		"conversion worker store is required",
	)
	ErrContentStoreRequired = errors.New(
		"conversion content store is required",
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
	ErrOutputContentRequired = errors.New(
		"conversion output content is required",
	)
	ErrOutputMediaTypeRequired = errors.New(
		"conversion output media type is required",
	)
	ErrStoredOutputSHA256Invalid = errors.New(
		"stored conversion output SHA-256 is invalid",
	)
	ErrStoredOutputSizeInvalid = errors.New(
		"stored conversion output size is invalid",
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
	FinalizeConversionJobSuccess(
		ctx context.Context,
		input FinalizeSuccessInput,
	) (dbgen.ConversionJobOutput, error)
	MarkConversionJobFailed(
		ctx context.Context,
		arg dbgen.MarkConversionJobFailedParams,
	) (dbgen.ConversionJob, error)
}

type ContentStore interface {
	Open(
		ctx context.Context,
		contentSHA256 string,
	) (io.ReadCloser, error)
	Put(
		ctx context.Context,
		source io.Reader,
	) (storage.PutResult, error)
}

type Converter interface {
	Convert(
		ctx context.Context,
		projectFile dbgen.ProjectFile,
		source io.Reader,
	) (ConversionResult, error)
}

type Worker struct {
	store     WorkerStore
	content   ContentStore
	converter Converter
	idleDelay time.Duration
}

func NewWorker(
	store WorkerStore,
	content ContentStore,
	converter Converter,
	idleDelay time.Duration,
) (*Worker, error) {
	if store == nil {
		return nil, ErrWorkerStoreRequired
	}

	if content == nil {
		return nil, ErrContentStoreRequired
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

	finalizeInput, err := w.processClaimedJob(ctx, job)
	if err != nil {
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

	if err := w.finalizeSuccess(finalizeInput); err != nil {
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
) (FinalizeSuccessInput, error) {
	projectFile, err := w.store.GetProjectFileByIDAndProject(
		ctx,
		dbgen.GetProjectFileByIDAndProjectParams{
			ProjectFileID: job.ProjectFileID,
			ProjectID:     job.ProjectID,
		},
	)
	if err != nil {
		return FinalizeSuccessInput{}, fmt.Errorf(
			"get conversion source metadata: %w",
			err,
		)
	}

	source, err := w.content.Open(
		ctx,
		projectFile.ContentSha256,
	)
	if err != nil {
		return FinalizeSuccessInput{}, fmt.Errorf(
			"open conversion source content: %w",
			err,
		)
	}

	result, convertErr := w.converter.Convert(
		ctx,
		projectFile,
		source,
	)

	sourceCloseErr := source.Close()

	if convertErr != nil {
		if result.Content != nil {
			_ = result.Content.Close()
		}

		return FinalizeSuccessInput{}, fmt.Errorf(
			"convert project file: %w",
			convertErr,
		)
	}

	if sourceCloseErr != nil {
		if result.Content != nil {
			_ = result.Content.Close()
		}

		return FinalizeSuccessInput{}, fmt.Errorf(
			"close conversion source content: %w",
			sourceCloseErr,
		)
	}

	if result.Content == nil {
		return FinalizeSuccessInput{}, ErrOutputContentRequired
	}

	mediaType := strings.TrimSpace(result.MediaType)
	if mediaType == "" {
		_ = result.Content.Close()
		return FinalizeSuccessInput{}, ErrOutputMediaTypeRequired
	}

	stored, putErr := w.content.Put(
		ctx,
		result.Content,
	)

	outputCloseErr := result.Content.Close()

	if putErr != nil {
		if outputCloseErr != nil {
			return FinalizeSuccessInput{}, fmt.Errorf(
				"store conversion output: %w",
				errors.Join(
					putErr,
					fmt.Errorf(
						"close conversion output content: %w",
						outputCloseErr,
					),
				),
			)
		}

		return FinalizeSuccessInput{}, fmt.Errorf(
			"store conversion output: %w",
			putErr,
		)
	}

	if outputCloseErr != nil {
		return FinalizeSuccessInput{}, fmt.Errorf(
			"close conversion output content: %w",
			outputCloseErr,
		)
	}

	if err := validateStoredOutput(stored); err != nil {
		return FinalizeSuccessInput{}, err
	}

	return FinalizeSuccessInput{
		ConversionJobID: job.ID,
		ContentSHA256:   stored.SHA256,
		SizeBytes:       stored.SizeBytes,
		MediaType:       mediaType,
	}, nil
}

func validateStoredOutput(
	stored storage.PutResult,
) error {
	if len(stored.SHA256) != 64 ||
		stored.SHA256 != strings.ToLower(stored.SHA256) {
		return ErrStoredOutputSHA256Invalid
	}

	if _, err := hex.DecodeString(stored.SHA256); err != nil {
		return ErrStoredOutputSHA256Invalid
	}

	if stored.SizeBytes < 0 {
		return ErrStoredOutputSizeInvalid
	}

	return nil
}

func (w *Worker) finalizeSuccess(
	input FinalizeSuccessInput,
) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		workerFinalizationTimeout,
	)
	defer cancel()

	_, err := w.store.FinalizeConversionJobSuccess(
		ctx,
		input,
	)
	return err
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
