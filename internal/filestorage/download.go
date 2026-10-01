package filestorage

import (
	"context"
	"fmt"
	"io"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
)

type ProjectFileMetadataReader interface {
	Get(
		ctx context.Context,
		ownerUserID uuid.UUID,
		projectID uuid.UUID,
		projectFileID uuid.UUID,
	) (dbgen.ProjectFile, error)
}

type ContentReader interface {
	Open(
		ctx context.Context,
		contentSHA256 string,
	) (io.ReadCloser, error)
}

type DownloadService struct {
	metadata ProjectFileMetadataReader
	content  ContentReader
}

type DownloadResult struct {
	ProjectFile dbgen.ProjectFile
	Content     io.ReadCloser
}

func NewDownloadService(
	metadata ProjectFileMetadataReader,
	content ContentReader,
) *DownloadService {
	return &DownloadService{
		metadata: metadata,
		content:  content,
	}
}

func (s *DownloadService) Download(
	ctx context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	projectFileID uuid.UUID,
) (DownloadResult, error) {
	projectFile, err := s.metadata.Get(
		ctx,
		ownerUserID,
		projectID,
		projectFileID,
	)
	if err != nil {
		return DownloadResult{}, err
	}

	content, err := s.content.Open(
		ctx,
		projectFile.ContentSha256,
	)
	if err != nil {
		return DownloadResult{}, fmt.Errorf(
			"open project file content: %w",
			err,
		)
	}

	return DownloadResult{
		ProjectFile: projectFile,
		Content:     content,
	}, nil
}
