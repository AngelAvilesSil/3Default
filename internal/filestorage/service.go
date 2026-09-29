package filestorage

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

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
	ErrContentSHA256Required = errors.New(
		"content SHA-256 is required",
	)
	ErrContentSHA256Invalid = errors.New(
		"content SHA-256 is invalid",
	)
	ErrSizeBytesInvalid = errors.New(
		"content size is invalid",
	)
	ErrOriginalFilenameRequired = errors.New(
		"original filename is required",
	)
	ErrProjectNotFound = errors.New(
		"project not found",
	)
	ErrProjectFileNotFound = errors.New(
		"project file not found",
	)
	ErrContentObjectSizeConflict = errors.New(
		"content object size conflicts with existing hash",
	)
)

type ProjectReader interface {
	GetProjectByIDAndOwner(
		ctx context.Context,
		arg dbgen.GetProjectByIDAndOwnerParams,
	) (dbgen.Project, error)
}

type FileStore interface {
	CreateProjectFileWithContentObject(
		ctx context.Context,
		arg CreateProjectFileParams,
	) (dbgen.ProjectFile, error)
	GetProjectFileByIDAndProject(
		ctx context.Context,
		arg dbgen.GetProjectFileByIDAndProjectParams,
	) (dbgen.ProjectFile, error)
	ListProjectFilesByProject(
		ctx context.Context,
		projectID uuid.UUID,
	) ([]dbgen.ProjectFile, error)
}

type Service struct {
	projects ProjectReader
	files    FileStore
}

type CreateProjectFileParams struct {
	ProjectID        uuid.UUID
	UploadedByUserID uuid.UUID
	ContentSHA256    string
	SizeBytes        int64
	OriginalFilename string
	MediaType        *string
}

type CreateInput struct {
	OwnerUserID      uuid.UUID
	ProjectID        uuid.UUID
	ContentSHA256    string
	SizeBytes        int64
	OriginalFilename string
	MediaType        *string
}

func NewService(
	projects ProjectReader,
	files FileStore,
) *Service {
	return &Service{
		projects: projects,
		files:    files,
	}
}

func (s *Service) Create(
	ctx context.Context,
	input CreateInput,
) (dbgen.ProjectFile, error) {
	params, err := normalizeCreateInput(input)
	if err != nil {
		return dbgen.ProjectFile{}, err
	}

	if err := s.requireProjectOwner(
		ctx,
		input.OwnerUserID,
		input.ProjectID,
	); err != nil {
		return dbgen.ProjectFile{}, err
	}

	return s.createAuthorized(ctx, params)
}

func normalizeCreateInput(
	input CreateInput,
) (CreateProjectFileParams, error) {
	if input.OwnerUserID == uuid.Nil {
		return CreateProjectFileParams{}, ErrOwnerRequired
	}

	if input.ProjectID == uuid.Nil {
		return CreateProjectFileParams{}, ErrProjectIDRequired
	}

	contentSHA256, err := normalizeSHA256(input.ContentSHA256)
	if err != nil {
		return CreateProjectFileParams{}, err
	}

	if input.SizeBytes < 0 {
		return CreateProjectFileParams{}, ErrSizeBytesInvalid
	}

	originalFilename := strings.TrimSpace(input.OriginalFilename)
	if originalFilename == "" {
		return CreateProjectFileParams{}, ErrOriginalFilenameRequired
	}

	return CreateProjectFileParams{
		ProjectID:        input.ProjectID,
		UploadedByUserID: input.OwnerUserID,
		ContentSHA256:    contentSHA256,
		SizeBytes:        input.SizeBytes,
		OriginalFilename: originalFilename,
		MediaType:        normalizeOptionalText(input.MediaType),
	}, nil
}

func (s *Service) createAuthorized(
	ctx context.Context,
	params CreateProjectFileParams,
) (dbgen.ProjectFile, error) {
	projectFile, err := s.files.CreateProjectFileWithContentObject(
		ctx,
		params,
	)
	if errors.Is(err, ErrContentObjectSizeConflict) {
		return dbgen.ProjectFile{}, ErrContentObjectSizeConflict
	}
	if err != nil {
		return dbgen.ProjectFile{}, fmt.Errorf(
			"create project file: %w",
			err,
		)
	}

	return projectFile, nil
}

func (s *Service) Get(
	ctx context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	projectFileID uuid.UUID,
) (dbgen.ProjectFile, error) {
	if ownerUserID == uuid.Nil {
		return dbgen.ProjectFile{}, ErrOwnerRequired
	}

	if projectID == uuid.Nil {
		return dbgen.ProjectFile{}, ErrProjectIDRequired
	}

	if projectFileID == uuid.Nil {
		return dbgen.ProjectFile{}, ErrProjectFileIDRequired
	}

	if err := s.requireProjectOwner(
		ctx,
		ownerUserID,
		projectID,
	); err != nil {
		return dbgen.ProjectFile{}, err
	}

	projectFile, err := s.files.GetProjectFileByIDAndProject(
		ctx,
		dbgen.GetProjectFileByIDAndProjectParams{
			ProjectFileID: projectFileID,
			ProjectID:     projectID,
		},
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return dbgen.ProjectFile{}, ErrProjectFileNotFound
	}
	if err != nil {
		return dbgen.ProjectFile{}, fmt.Errorf(
			"get project file: %w",
			err,
		)
	}

	return projectFile, nil
}

func (s *Service) List(
	ctx context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
) ([]dbgen.ProjectFile, error) {
	if ownerUserID == uuid.Nil {
		return nil, ErrOwnerRequired
	}

	if projectID == uuid.Nil {
		return nil, ErrProjectIDRequired
	}

	if err := s.requireProjectOwner(
		ctx,
		ownerUserID,
		projectID,
	); err != nil {
		return nil, err
	}

	projectFiles, err := s.files.ListProjectFilesByProject(
		ctx,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"list project files: %w",
			err,
		)
	}

	if projectFiles == nil {
		return []dbgen.ProjectFile{}, nil
	}

	return projectFiles, nil
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
			"get project for file storage: %w",
			err,
		)
	}

	return nil
}

func normalizeSHA256(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "", ErrContentSHA256Required
	}

	if len(normalized) != 64 {
		return "", ErrContentSHA256Invalid
	}

	decoded, err := hex.DecodeString(normalized)
	if err != nil || len(decoded) != 32 {
		return "", ErrContentSHA256Invalid
	}

	return normalized, nil
}

func normalizeOptionalText(value *string) *string {
	if value == nil {
		return nil
	}

	normalized := strings.TrimSpace(*value)
	if normalized == "" {
		return nil
	}

	return &normalized
}
