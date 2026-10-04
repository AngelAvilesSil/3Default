package conversionjobs

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrPreviewNotFound = errors.New(
		"conversion preview not found",
	)
	ErrPreviewMediaTypeUnsupported = errors.New(
		"conversion preview media type is unsupported",
	)
)

type PreviewOutputReader interface {
	GetLatestConversionJobOutputByProjectFile(
		ctx context.Context,
		arg dbgen.GetLatestConversionJobOutputByProjectFileParams,
	) (dbgen.ConversionJobOutput, error)
}

type PreviewContentReader interface {
	Open(
		ctx context.Context,
		contentSHA256 string,
	) (io.ReadCloser, error)
}

type PreviewService struct {
	projects ProjectReader
	files    ProjectFileReader
	outputs  PreviewOutputReader
	content  PreviewContentReader
}

type PreviewResult struct {
	Output  dbgen.ConversionJobOutput
	Content io.ReadCloser
}

func NewPreviewService(
	projects ProjectReader,
	files ProjectFileReader,
	outputs PreviewOutputReader,
	content PreviewContentReader,
) *PreviewService {
	return &PreviewService{
		projects: projects,
		files:    files,
		outputs:  outputs,
		content:  content,
	}
}

func (s *PreviewService) GetLatest(
	ctx context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	projectFileID uuid.UUID,
) (PreviewResult, error) {
	if ownerUserID == uuid.Nil {
		return PreviewResult{}, ErrOwnerRequired
	}

	if projectID == uuid.Nil {
		return PreviewResult{}, ErrProjectIDRequired
	}

	if projectFileID == uuid.Nil {
		return PreviewResult{}, ErrProjectFileIDRequired
	}

	if err := s.requireProjectOwner(
		ctx,
		ownerUserID,
		projectID,
	); err != nil {
		return PreviewResult{}, err
	}

	if err := s.requireProjectFile(
		ctx,
		projectID,
		projectFileID,
	); err != nil {
		return PreviewResult{}, err
	}

	output, err :=
		s.outputs.GetLatestConversionJobOutputByProjectFile(
			ctx,
			dbgen.GetLatestConversionJobOutputByProjectFileParams{
				ProjectID:     projectID,
				ProjectFileID: projectFileID,
			},
		)
	if errors.Is(err, pgx.ErrNoRows) {
		return PreviewResult{}, ErrPreviewNotFound
	}
	if err != nil {
		return PreviewResult{}, fmt.Errorf(
			"get latest conversion preview output: %w",
			err,
		)
	}

	if output.MediaType != GLBMediaType {
		return PreviewResult{},
			ErrPreviewMediaTypeUnsupported
	}

	content, err := s.content.Open(
		ctx,
		output.ContentSha256,
	)
	if err != nil {
		return PreviewResult{}, fmt.Errorf(
			"open conversion preview content: %w",
			err,
		)
	}

	return PreviewResult{
		Output:  output,
		Content: content,
	}, nil
}

func (s *PreviewService) requireProjectOwner(
	ctx context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
) error {
	_, err := s.projects.GetProjectByIDAndOwner(
		ctx,
		dbgen.GetProjectByIDAndOwnerParams{
			ProjectID:   projectID,
			OwnerUserID: ownerUserID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProjectNotFound
	}
	if err != nil {
		return fmt.Errorf(
			"get project for conversion preview: %w",
			err,
		)
	}

	return nil
}

func (s *PreviewService) requireProjectFile(
	ctx context.Context,
	projectID uuid.UUID,
	projectFileID uuid.UUID,
) error {
	_, err := s.files.GetProjectFileByIDAndProject(
		ctx,
		dbgen.GetProjectFileByIDAndProjectParams{
			ProjectFileID: projectFileID,
			ProjectID:     projectID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrProjectFileNotFound
	}
	if err != nil {
		return fmt.Errorf(
			"get project file for conversion preview: %w",
			err,
		)
	}

	return nil
}
