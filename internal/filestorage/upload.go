package filestorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/storage"
	"github.com/google/uuid"
)

var ErrUploadSourceRequired = errors.New(
	"upload source is required",
)

type ContentStore interface {
	Put(
		ctx context.Context,
		source io.Reader,
	) (storage.PutResult, error)
}

type UploadService struct {
	metadata *Service
	content  ContentStore
}

type UploadInput struct {
	OwnerUserID      uuid.UUID
	ProjectID        uuid.UUID
	OriginalFilename string
	MediaType        *string
	Source           io.Reader
}

type normalizedUploadInput struct {
	OwnerUserID      uuid.UUID
	ProjectID        uuid.UUID
	OriginalFilename string
	MediaType        *string
	Source           io.Reader
}

func NewUploadService(
	metadata *Service,
	content ContentStore,
) *UploadService {
	return &UploadService{
		metadata: metadata,
		content:  content,
	}
}

func (s *UploadService) Upload(
	ctx context.Context,
	input UploadInput,
) (dbgen.ProjectFile, error) {
	normalized, err := normalizeUploadInput(input)
	if err != nil {
		return dbgen.ProjectFile{}, err
	}

	if err := s.metadata.requireProjectOwner(
		ctx,
		normalized.OwnerUserID,
		normalized.ProjectID,
	); err != nil {
		return dbgen.ProjectFile{}, err
	}

	stored, err := s.content.Put(
		ctx,
		normalized.Source,
	)
	if err != nil {
		return dbgen.ProjectFile{}, fmt.Errorf(
			"store upload content: %w",
			err,
		)
	}

	contentSHA256, err := normalizeSHA256(stored.SHA256)
	if err != nil {
		return dbgen.ProjectFile{}, fmt.Errorf(
			"validate stored content SHA-256: %w",
			err,
		)
	}

	if stored.SizeBytes < 0 {
		return dbgen.ProjectFile{}, fmt.Errorf(
			"validate stored content size: %w",
			ErrSizeBytesInvalid,
		)
	}

	return s.metadata.createAuthorized(
		ctx,
		CreateProjectFileParams{
			ProjectID:        normalized.ProjectID,
			UploadedByUserID: normalized.OwnerUserID,
			ContentSHA256:    contentSHA256,
			SizeBytes:        stored.SizeBytes,
			OriginalFilename: normalized.OriginalFilename,
			MediaType:        normalized.MediaType,
		},
	)
}

func normalizeUploadInput(
	input UploadInput,
) (normalizedUploadInput, error) {
	if input.OwnerUserID == uuid.Nil {
		return normalizedUploadInput{}, ErrOwnerRequired
	}

	if input.ProjectID == uuid.Nil {
		return normalizedUploadInput{}, ErrProjectIDRequired
	}

	originalFilename := strings.TrimSpace(input.OriginalFilename)
	if originalFilename == "" {
		return normalizedUploadInput{}, ErrOriginalFilenameRequired
	}

	if input.Source == nil {
		return normalizedUploadInput{}, ErrUploadSourceRequired
	}

	return normalizedUploadInput{
		OwnerUserID:      input.OwnerUserID,
		ProjectID:        input.ProjectID,
		OriginalFilename: originalFilename,
		MediaType:        normalizeOptionalText(input.MediaType),
		Source:           input.Source,
	}, nil
}
