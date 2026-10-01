package filestorage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/storage"
	"github.com/google/uuid"
)

type fakeProjectFileMetadataReader struct {
	called        bool
	ownerUserID   uuid.UUID
	projectID     uuid.UUID
	projectFileID uuid.UUID
	projectFile   dbgen.ProjectFile
	err           error
}

func (f *fakeProjectFileMetadataReader) Get(
	_ context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	projectFileID uuid.UUID,
) (dbgen.ProjectFile, error) {
	f.called = true
	f.ownerUserID = ownerUserID
	f.projectID = projectID
	f.projectFileID = projectFileID

	return f.projectFile, f.err
}

type fakeDownloadContentReader struct {
	called        bool
	contentSHA256 string
	content       io.ReadCloser
	err           error
}

func (f *fakeDownloadContentReader) Open(
	_ context.Context,
	contentSHA256 string,
) (io.ReadCloser, error) {
	f.called = true
	f.contentSHA256 = contentSHA256

	return f.content, f.err
}

func TestDownloadReturnsProjectFileAndContent(
	t *testing.T,
) {
	ownerUserID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()
	contentSHA256 := strings.Repeat("a", 64)
	expectedContent := []byte("authoritative CAD source")

	metadata := &fakeProjectFileMetadataReader{
		projectFile: dbgen.ProjectFile{
			ID:               projectFileID,
			ProjectID:        projectID,
			UploadedByUserID: ownerUserID,
			ContentSha256:    contentSHA256,
			OriginalFilename: "assembly.step",
		},
	}

	content := &fakeDownloadContentReader{
		content: io.NopCloser(
			bytes.NewReader(expectedContent),
		),
	}

	service := NewDownloadService(
		metadata,
		content,
	)

	result, err := service.Download(
		context.Background(),
		ownerUserID,
		projectID,
		projectFileID,
	)
	if err != nil {
		t.Fatalf("download project file: %v", err)
	}
	defer result.Content.Close()

	if !metadata.called {
		t.Fatal("expected project file metadata lookup")
	}

	if metadata.ownerUserID != ownerUserID {
		t.Fatalf(
			"expected owner user ID %s, got %s",
			ownerUserID,
			metadata.ownerUserID,
		)
	}

	if metadata.projectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			metadata.projectID,
		)
	}

	if metadata.projectFileID != projectFileID {
		t.Fatalf(
			"expected project file ID %s, got %s",
			projectFileID,
			metadata.projectFileID,
		)
	}

	if !content.called {
		t.Fatal("expected physical content lookup")
	}

	if content.contentSHA256 != contentSHA256 {
		t.Fatalf(
			"expected content SHA-256 %q, got %q",
			contentSHA256,
			content.contentSHA256,
		)
	}

	if result.ProjectFile.ID != projectFileID {
		t.Fatalf(
			"expected project file ID %s, got %s",
			projectFileID,
			result.ProjectFile.ID,
		)
	}

	actualContent, err := io.ReadAll(result.Content)
	if err != nil {
		t.Fatalf("read downloaded content: %v", err)
	}

	if !bytes.Equal(actualContent, expectedContent) {
		t.Fatalf(
			"expected downloaded content %q, got %q",
			expectedContent,
			actualContent,
		)
	}
}

func TestDownloadPropagatesMetadataErrorsBeforeContentLookup(
	t *testing.T,
) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "owner required",
			err:  ErrOwnerRequired,
		},
		{
			name: "project ID required",
			err:  ErrProjectIDRequired,
		},
		{
			name: "project file ID required",
			err:  ErrProjectFileIDRequired,
		},
		{
			name: "project not found",
			err:  ErrProjectNotFound,
		},
		{
			name: "project file not found",
			err:  ErrProjectFileNotFound,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			metadata := &fakeProjectFileMetadataReader{
				err: test.err,
			}
			content := &fakeDownloadContentReader{}

			service := NewDownloadService(
				metadata,
				content,
			)

			result, err := service.Download(
				context.Background(),
				uuid.New(),
				uuid.New(),
				uuid.New(),
			)

			if !errors.Is(err, test.err) {
				t.Fatalf(
					"expected %v, got %v",
					test.err,
					err,
				)
			}

			if result.Content != nil {
				_ = result.Content.Close()
				t.Fatal("expected no content")
			}

			if content.called {
				t.Fatal(
					"expected content lookup not to run after metadata error",
				)
			}
		})
	}
}

func TestDownloadWrapsContentOpenError(
	t *testing.T,
) {
	projectFile := dbgen.ProjectFile{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		ContentSha256: strings.Repeat("b", 64),
	}

	metadata := &fakeProjectFileMetadataReader{
		projectFile: projectFile,
	}

	contentErr := storage.ErrContentNotFound
	content := &fakeDownloadContentReader{
		err: contentErr,
	}

	service := NewDownloadService(
		metadata,
		content,
	)

	result, err := service.Download(
		context.Background(),
		uuid.New(),
		projectFile.ProjectID,
		projectFile.ID,
	)
	if !errors.Is(err, contentErr) {
		t.Fatalf(
			"expected wrapped content error %v, got %v",
			contentErr,
			err,
		)
	}

	if result.Content != nil {
		_ = result.Content.Close()
		t.Fatal("expected no content")
	}

	if !content.called {
		t.Fatal("expected physical content lookup")
	}

	if content.contentSHA256 != projectFile.ContentSha256 {
		t.Fatalf(
			"expected content SHA-256 %q, got %q",
			projectFile.ContentSha256,
			content.contentSHA256,
		)
	}
}
