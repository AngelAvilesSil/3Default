package filestorage

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/storage"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeContentStore struct {
	called bool
	source io.Reader
	result storage.PutResult
	err    error
}

func (f *fakeContentStore) Put(
	_ context.Context,
	source io.Reader,
) (storage.PutResult, error) {
	f.called = true
	f.source = source

	return f.result, f.err
}

func TestUploadStoresContentAndCreatesMetadata(
	t *testing.T,
) {
	ownerID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()

	source := strings.NewReader(
		"authoritative CAD source bytes",
	)

	mediaType := "  model/step  "

	projects := &fakeProjectReader{}
	files := &fakeFileStore{
		createFile: dbgen.ProjectFile{
			ID:               projectFileID,
			ProjectID:        projectID,
			UploadedByUserID: ownerID,
			ContentSha256:    strings.Repeat("a", 64),
			OriginalFilename: "gripper.step",
		},
	}
	content := &fakeContentStore{
		result: storage.PutResult{
			SHA256:    strings.Repeat("A", 64),
			SizeBytes: 2048,
		},
	}

	metadata := NewService(projects, files)
	uploads := NewUploadService(metadata, content)

	projectFile, err := uploads.Upload(
		context.Background(),
		UploadInput{
			OwnerUserID:      ownerID,
			ProjectID:        projectID,
			OriginalFilename: "  gripper.step  ",
			MediaType:        &mediaType,
			Source:           source,
		},
	)
	if err != nil {
		t.Fatalf("upload project file: %v", err)
	}

	if !projects.called {
		t.Fatal("expected project ownership lookup")
	}

	if projects.params.OwnerUserID != ownerID {
		t.Fatalf(
			"expected owner ID %s, got %s",
			ownerID,
			projects.params.OwnerUserID,
		)
	}

	if projects.params.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			projects.params.ProjectID,
		)
	}

	if !content.called {
		t.Fatal("expected content store call")
	}

	if content.source != source {
		t.Fatal("expected original upload source to reach content store")
	}

	if !files.createCalled {
		t.Fatal("expected metadata store call")
	}

	if files.createParams.ProjectID != projectID {
		t.Fatalf(
			"expected metadata project ID %s, got %s",
			projectID,
			files.createParams.ProjectID,
		)
	}

	if files.createParams.UploadedByUserID != ownerID {
		t.Fatalf(
			"expected metadata uploader ID %s, got %s",
			ownerID,
			files.createParams.UploadedByUserID,
		)
	}

	expectedSHA256 := strings.Repeat("a", 64)
	if files.createParams.ContentSHA256 != expectedSHA256 {
		t.Fatalf(
			"expected normalized stored SHA-256 %q, got %q",
			expectedSHA256,
			files.createParams.ContentSHA256,
		)
	}

	if files.createParams.SizeBytes != 2048 {
		t.Fatalf(
			"expected stored size %d, got %d",
			2048,
			files.createParams.SizeBytes,
		)
	}

	if files.createParams.OriginalFilename != "gripper.step" {
		t.Fatalf(
			"expected normalized filename %q, got %q",
			"gripper.step",
			files.createParams.OriginalFilename,
		)
	}

	if files.createParams.MediaType == nil {
		t.Fatal("expected normalized media type")
	}

	if *files.createParams.MediaType != "model/step" {
		t.Fatalf(
			"expected normalized media type %q, got %q",
			"model/step",
			*files.createParams.MediaType,
		)
	}

	if projectFile.ID != projectFileID {
		t.Fatalf(
			"expected project file ID %s, got %s",
			projectFileID,
			projectFile.ID,
		)
	}
}

func TestUploadRejectsInvalidInputBeforeStores(
	t *testing.T,
) {
	validOwnerID := uuid.New()
	validProjectID := uuid.New()

	tests := []struct {
		name  string
		input UploadInput
		want  error
	}{
		{
			name: "missing owner",
			input: UploadInput{
				ProjectID:        validProjectID,
				OriginalFilename: "part.step",
				Source: strings.NewReader(
					"content",
				),
			},
			want: ErrOwnerRequired,
		},
		{
			name: "missing project ID",
			input: UploadInput{
				OwnerUserID:      validOwnerID,
				OriginalFilename: "part.step",
				Source: strings.NewReader(
					"content",
				),
			},
			want: ErrProjectIDRequired,
		},
		{
			name: "blank original filename",
			input: UploadInput{
				OwnerUserID:      validOwnerID,
				ProjectID:        validProjectID,
				OriginalFilename: "   ",
				Source: strings.NewReader(
					"content",
				),
			},
			want: ErrOriginalFilenameRequired,
		},
		{
			name: "missing source",
			input: UploadInput{
				OwnerUserID:      validOwnerID,
				ProjectID:        validProjectID,
				OriginalFilename: "part.step",
			},
			want: ErrUploadSourceRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := &fakeProjectReader{}
			files := &fakeFileStore{}
			content := &fakeContentStore{}

			uploads := NewUploadService(
				NewService(projects, files),
				content,
			)

			_, err := uploads.Upload(
				context.Background(),
				test.input,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf(
					"expected %v, got %v",
					test.want,
					err,
				)
			}

			if projects.called {
				t.Fatal(
					"expected project store not to be called",
				)
			}

			if content.called {
				t.Fatal(
					"expected content store not to be called",
				)
			}

			if files.createCalled {
				t.Fatal(
					"expected metadata store not to be called",
				)
			}
		})
	}
}

func TestUploadRejectsMissingProjectBeforeStoringContent(
	t *testing.T,
) {
	projects := &fakeProjectReader{
		err: pgx.ErrNoRows,
	}
	files := &fakeFileStore{}
	content := &fakeContentStore{}

	uploads := NewUploadService(
		NewService(projects, files),
		content,
	)

	_, err := uploads.Upload(
		context.Background(),
		UploadInput{
			OwnerUserID:      uuid.New(),
			ProjectID:        uuid.New(),
			OriginalFilename: "part.step",
			Source: strings.NewReader(
				"content",
			),
		},
	)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf(
			"expected ErrProjectNotFound, got %v",
			err,
		)
	}

	if content.called {
		t.Fatal(
			"expected content not to be stored for missing project",
		)
	}

	if files.createCalled {
		t.Fatal(
			"expected metadata not to be created",
		)
	}
}

func TestUploadWrapsProjectLookupErrorBeforeStoringContent(
	t *testing.T,
) {
	databaseErr := errors.New("database unavailable")

	projects := &fakeProjectReader{
		err: databaseErr,
	}
	files := &fakeFileStore{}
	content := &fakeContentStore{}

	uploads := NewUploadService(
		NewService(projects, files),
		content,
	)

	_, err := uploads.Upload(
		context.Background(),
		UploadInput{
			OwnerUserID:      uuid.New(),
			ProjectID:        uuid.New(),
			OriginalFilename: "part.step",
			Source: strings.NewReader(
				"content",
			),
		},
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped project lookup error, got %v",
			err,
		)
	}

	if content.called {
		t.Fatal(
			"expected content not to be stored after project lookup failure",
		)
	}

	if files.createCalled {
		t.Fatal(
			"expected metadata not to be created",
		)
	}
}

func TestUploadStopsWhenContentStorageFails(
	t *testing.T,
) {
	storageErr := errors.New("content storage unavailable")

	projects := &fakeProjectReader{}
	files := &fakeFileStore{}
	content := &fakeContentStore{
		err: storageErr,
	}

	uploads := NewUploadService(
		NewService(projects, files),
		content,
	)

	_, err := uploads.Upload(
		context.Background(),
		UploadInput{
			OwnerUserID:      uuid.New(),
			ProjectID:        uuid.New(),
			OriginalFilename: "part.step",
			Source: strings.NewReader(
				"content",
			),
		},
	)
	if !errors.Is(err, storageErr) {
		t.Fatalf(
			"expected wrapped content storage error, got %v",
			err,
		)
	}

	if !content.called {
		t.Fatal("expected content store to be called")
	}

	if files.createCalled {
		t.Fatal(
			"expected metadata not to be created after storage failure",
		)
	}
}

func TestUploadRejectsInvalidStoredSHA256(
	t *testing.T,
) {
	files := &fakeFileStore{}
	content := &fakeContentStore{
		result: storage.PutResult{
			SHA256:    "invalid",
			SizeBytes: 10,
		},
	}

	uploads := NewUploadService(
		NewService(&fakeProjectReader{}, files),
		content,
	)

	_, err := uploads.Upload(
		context.Background(),
		UploadInput{
			OwnerUserID:      uuid.New(),
			ProjectID:        uuid.New(),
			OriginalFilename: "part.step",
			Source: strings.NewReader(
				"content",
			),
		},
	)
	if !errors.Is(err, ErrContentSHA256Invalid) {
		t.Fatalf(
			"expected ErrContentSHA256Invalid, got %v",
			err,
		)
	}

	if files.createCalled {
		t.Fatal(
			"expected metadata not to be created for invalid stored SHA-256",
		)
	}
}

func TestUploadRejectsInvalidStoredSize(
	t *testing.T,
) {
	files := &fakeFileStore{}
	content := &fakeContentStore{
		result: storage.PutResult{
			SHA256:    strings.Repeat("a", 64),
			SizeBytes: -1,
		},
	}

	uploads := NewUploadService(
		NewService(&fakeProjectReader{}, files),
		content,
	)

	_, err := uploads.Upload(
		context.Background(),
		UploadInput{
			OwnerUserID:      uuid.New(),
			ProjectID:        uuid.New(),
			OriginalFilename: "part.step",
			Source: strings.NewReader(
				"content",
			),
		},
	)
	if !errors.Is(err, ErrSizeBytesInvalid) {
		t.Fatalf(
			"expected ErrSizeBytesInvalid, got %v",
			err,
		)
	}

	if files.createCalled {
		t.Fatal(
			"expected metadata not to be created for invalid stored size",
		)
	}
}

func TestUploadPreservesMetadataConflictAfterContentStorage(
	t *testing.T,
) {
	files := &fakeFileStore{
		createErr: ErrContentObjectSizeConflict,
	}
	content := &fakeContentStore{
		result: storage.PutResult{
			SHA256:    strings.Repeat("a", 64),
			SizeBytes: 100,
		},
	}

	uploads := NewUploadService(
		NewService(&fakeProjectReader{}, files),
		content,
	)

	_, err := uploads.Upload(
		context.Background(),
		UploadInput{
			OwnerUserID:      uuid.New(),
			ProjectID:        uuid.New(),
			OriginalFilename: "part.step",
			Source: strings.NewReader(
				"content",
			),
		},
	)
	if !errors.Is(
		err,
		ErrContentObjectSizeConflict,
	) {
		t.Fatalf(
			"expected ErrContentObjectSizeConflict, got %v",
			err,
		)
	}

	if !content.called {
		t.Fatal("expected content store to be called")
	}

	if !files.createCalled {
		t.Fatal("expected metadata store to be called")
	}
}

func TestUploadWrapsMetadataErrorAfterContentStorage(
	t *testing.T,
) {
	databaseErr := errors.New("database unavailable")

	files := &fakeFileStore{
		createErr: databaseErr,
	}
	content := &fakeContentStore{
		result: storage.PutResult{
			SHA256:    strings.Repeat("a", 64),
			SizeBytes: 100,
		},
	}

	uploads := NewUploadService(
		NewService(&fakeProjectReader{}, files),
		content,
	)

	_, err := uploads.Upload(
		context.Background(),
		UploadInput{
			OwnerUserID:      uuid.New(),
			ProjectID:        uuid.New(),
			OriginalFilename: "part.step",
			Source: strings.NewReader(
				"content",
			),
		},
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped metadata error, got %v",
			err,
		)
	}

	if !content.called {
		t.Fatal("expected content store to be called")
	}

	if !files.createCalled {
		t.Fatal("expected metadata store to be called")
	}
}
