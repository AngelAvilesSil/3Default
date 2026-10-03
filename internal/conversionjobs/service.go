package conversionjobs

import (
	"context"
	"errors"
	"fmt"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrOwnerRequired = errors.New(
		"project owner is required",
	)
	ErrProjectIDRequired = errors.New(
		"project ID is required",
	)
	ErrProjectFileIDRequired = errors.New(
		"project file ID is required",
	)
	ErrConversionJobIDRequired = errors.New(
		"conversion job ID is required",
	)
	ErrProjectNotFound = errors.New(
		"project not found",
	)
	ErrProjectFileNotFound = errors.New(
		"project file not found",
	)
	ErrConversionJobNotFound = errors.New(
		"conversion job not found",
	)
	ErrActiveConversionJobExists = errors.New(
		"active conversion job already exists",
	)
)

type ProjectReader interface {
	GetProjectByIDAndOwner(
		ctx context.Context,
		arg dbgen.GetProjectByIDAndOwnerParams,
	) (dbgen.Project, error)
}

type ProjectFileReader interface {
	GetProjectFileByIDAndProject(
		ctx context.Context,
		arg dbgen.GetProjectFileByIDAndProjectParams,
	) (dbgen.ProjectFile, error)
}

type Service struct {
	projects ProjectReader
	files    ProjectFileReader
	jobs     Store
}

type CreateInput struct {
	OwnerUserID   uuid.UUID
	ProjectID     uuid.UUID
	ProjectFileID uuid.UUID
}

func NewService(
	projects ProjectReader,
	files ProjectFileReader,
	jobs Store,
) *Service {
	return &Service{
		projects: projects,
		files:    files,
		jobs:     jobs,
	}
}

func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (dbgen.ConversionJob, error) {
	if input.OwnerUserID == uuid.Nil {
		return dbgen.ConversionJob{}, ErrOwnerRequired
	}

	if input.ProjectID == uuid.Nil {
		return dbgen.ConversionJob{}, ErrProjectIDRequired
	}

	if input.ProjectFileID == uuid.Nil {
		return dbgen.ConversionJob{}, ErrProjectFileIDRequired
	}

	if err := s.requireProjectOwner(
		ctx,
		input.OwnerUserID,
		input.ProjectID,
	); err != nil {
		return dbgen.ConversionJob{}, err
	}

	if err := s.requireProjectFile(
		ctx,
		input.ProjectID,
		input.ProjectFileID,
	); err != nil {
		return dbgen.ConversionJob{}, err
	}

	job, err := s.jobs.CreateConversionJob(
		ctx,
		dbgen.CreateConversionJobParams{
			ProjectID:     input.ProjectID,
			ProjectFileID: input.ProjectFileID,
		},
	)
	if errors.Is(err, ErrProjectFileNotFound) {
		return dbgen.ConversionJob{}, ErrProjectFileNotFound
	}
	if errors.Is(err, ErrActiveConversionJobExists) {
		return dbgen.ConversionJob{},
			ErrActiveConversionJobExists
	}
	if err != nil {
		return dbgen.ConversionJob{}, fmt.Errorf(
			"create conversion job: %w",
			err,
		)
	}

	return job, nil
}

func (s *Service) Get(
	ctx context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	conversionJobID uuid.UUID,
) (dbgen.ConversionJob, error) {
	if ownerUserID == uuid.Nil {
		return dbgen.ConversionJob{}, ErrOwnerRequired
	}

	if projectID == uuid.Nil {
		return dbgen.ConversionJob{}, ErrProjectIDRequired
	}

	if conversionJobID == uuid.Nil {
		return dbgen.ConversionJob{},
			ErrConversionJobIDRequired
	}

	if err := s.requireProjectOwner(
		ctx,
		ownerUserID,
		projectID,
	); err != nil {
		return dbgen.ConversionJob{}, err
	}

	job, err := s.jobs.GetConversionJobByIDAndProject(
		ctx,
		dbgen.GetConversionJobByIDAndProjectParams{
			ConversionJobID: conversionJobID,
			ProjectID:       projectID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ConversionJob{},
			ErrConversionJobNotFound
	}
	if err != nil {
		return dbgen.ConversionJob{}, fmt.Errorf(
			"get conversion job: %w",
			err,
		)
	}

	return job, nil
}

func (s *Service) ListForProjectFile(
	ctx context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	projectFileID uuid.UUID,
) ([]dbgen.ConversionJob, error) {
	if ownerUserID == uuid.Nil {
		return nil, ErrOwnerRequired
	}

	if projectID == uuid.Nil {
		return nil, ErrProjectIDRequired
	}

	if projectFileID == uuid.Nil {
		return nil, ErrProjectFileIDRequired
	}

	if err := s.requireProjectOwner(
		ctx,
		ownerUserID,
		projectID,
	); err != nil {
		return nil, err
	}

	if err := s.requireProjectFile(
		ctx,
		projectID,
		projectFileID,
	); err != nil {
		return nil, err
	}

	jobs, err := s.jobs.ListConversionJobsByProjectFile(
		ctx,
		dbgen.ListConversionJobsByProjectFileParams{
			ProjectID:     projectID,
			ProjectFileID: projectFileID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list project file conversion jobs: %w",
			err,
		)
	}

	if jobs == nil {
		return []dbgen.ConversionJob{}, nil
	}

	return jobs, nil
}

func (s *Service) requireProjectOwner(
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
			"get project for conversion jobs: %w",
			err,
		)
	}

	return nil
}

func (s *Service) requireProjectFile(
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
			"get project file for conversion job: %w",
			err,
		)
	}

	return nil
}
