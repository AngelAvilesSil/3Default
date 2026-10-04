package conversionjobs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakePreviewOutputReader struct {
	called bool
	params dbgen.GetLatestConversionJobOutputByProjectFileParams
	output dbgen.ConversionJobOutput
	err    error
}

func (f *fakePreviewOutputReader) GetLatestConversionJobOutputByProjectFile(
	_ context.Context,
	arg dbgen.GetLatestConversionJobOutputByProjectFileParams,
) (dbgen.ConversionJobOutput, error) {
	f.called = true
	f.params = arg

	return f.output, f.err
}

type fakePreviewContentReader struct {
	called        bool
	contentSHA256 string
	content       io.ReadCloser
	err           error
}

func (f *fakePreviewContentReader) Open(
	_ context.Context,
	contentSHA256 string,
) (io.ReadCloser, error) {
	f.called = true
	f.contentSHA256 = contentSHA256

	return f.content, f.err
}

func TestGetLatestPreviewRejectsInvalidInput(
	t *testing.T,
) {
	ownerUserID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()

	tests := []struct {
		name          string
		ownerUserID   uuid.UUID
		projectID     uuid.UUID
		projectFileID uuid.UUID
		want          error
	}{
		{
			name:          "missing owner",
			projectID:     projectID,
			projectFileID: projectFileID,
			want:          ErrOwnerRequired,
		},
		{
			name:          "missing project",
			ownerUserID:   ownerUserID,
			projectFileID: projectFileID,
			want:          ErrProjectIDRequired,
		},
		{
			name:        "missing project file",
			ownerUserID: ownerUserID,
			projectID:   projectID,
			want:        ErrProjectFileIDRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := &fakeProjectReader{}
			files := &fakeProjectFileReader{}
			outputs := &fakePreviewOutputReader{}
			content := &fakePreviewContentReader{}

			service := NewPreviewService(
				projects,
				files,
				outputs,
				content,
			)

			result, err := service.GetLatest(
				context.Background(),
				test.ownerUserID,
				test.projectID,
				test.projectFileID,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf(
					"expected %v, got %v",
					test.want,
					err,
				)
			}

			if result.Content != nil {
				_ = result.Content.Close()
				t.Fatal("expected no preview content")
			}

			if projects.called ||
				files.called ||
				outputs.called ||
				content.called {
				t.Fatal(
					"expected no dependency calls for invalid input",
				)
			}
		})
	}
}

func TestGetLatestPreviewMapsMissingProject(
	t *testing.T,
) {
	projects := &fakeProjectReader{
		err: pgx.ErrNoRows,
	}
	files := &fakeProjectFileReader{}
	outputs := &fakePreviewOutputReader{}
	content := &fakePreviewContentReader{}

	service := NewPreviewService(
		projects,
		files,
		outputs,
		content,
	)

	_, err := service.GetLatest(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf(
			"expected ErrProjectNotFound, got %v",
			err,
		)
	}

	if !projects.called {
		t.Fatal("expected project ownership lookup")
	}

	if files.called ||
		outputs.called ||
		content.called {
		t.Fatal(
			"expected preview lookup to stop after missing project",
		)
	}
}

func TestGetLatestPreviewMapsMissingProjectFile(
	t *testing.T,
) {
	projects := &fakeProjectReader{}
	files := &fakeProjectFileReader{
		err: pgx.ErrNoRows,
	}
	outputs := &fakePreviewOutputReader{}
	content := &fakePreviewContentReader{}

	service := NewPreviewService(
		projects,
		files,
		outputs,
		content,
	)

	_, err := service.GetLatest(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, ErrProjectFileNotFound) {
		t.Fatalf(
			"expected ErrProjectFileNotFound, got %v",
			err,
		)
	}

	if !projects.called || !files.called {
		t.Fatal(
			"expected project and project file lookups",
		)
	}

	if outputs.called || content.called {
		t.Fatal(
			"expected preview lookup to stop after missing project file",
		)
	}
}

func TestGetLatestPreviewMapsMissingOutputToNotFound(
	t *testing.T,
) {
	outputs := &fakePreviewOutputReader{
		err: pgx.ErrNoRows,
	}
	content := &fakePreviewContentReader{}

	service := NewPreviewService(
		&fakeProjectReader{},
		&fakeProjectFileReader{},
		outputs,
		content,
	)

	_, err := service.GetLatest(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, ErrPreviewNotFound) {
		t.Fatalf(
			"expected ErrPreviewNotFound, got %v",
			err,
		)
	}

	if !outputs.called {
		t.Fatal("expected latest preview output lookup")
	}

	if content.called {
		t.Fatal(
			"expected no physical content lookup without preview metadata",
		)
	}
}

func TestGetLatestPreviewWrapsOutputLookupError(
	t *testing.T,
) {
	outputErr := errors.New("database unavailable")
	outputs := &fakePreviewOutputReader{
		err: outputErr,
	}

	service := NewPreviewService(
		&fakeProjectReader{},
		&fakeProjectFileReader{},
		outputs,
		&fakePreviewContentReader{},
	)

	_, err := service.GetLatest(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, outputErr) {
		t.Fatalf(
			"expected wrapped output lookup error %v, got %v",
			outputErr,
			err,
		)
	}
}

func TestGetLatestPreviewRejectsUnsupportedMediaType(
	t *testing.T,
) {
	outputs := &fakePreviewOutputReader{
		output: dbgen.ConversionJobOutput{
			ConversionJobID: uuid.New(),
			ContentSha256: strings.Repeat(
				"a",
				64,
			),
			MediaType: "model/gltf+json",
		},
	}
	content := &fakePreviewContentReader{}

	service := NewPreviewService(
		&fakeProjectReader{},
		&fakeProjectFileReader{},
		outputs,
		content,
	)

	_, err := service.GetLatest(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(
		err,
		ErrPreviewMediaTypeUnsupported,
	) {
		t.Fatalf(
			"expected ErrPreviewMediaTypeUnsupported, got %v",
			err,
		)
	}

	if content.called {
		t.Fatal(
			"expected unsupported preview not to open physical content",
		)
	}
}

func TestGetLatestPreviewWrapsContentOpenError(
	t *testing.T,
) {
	contentSHA256 := strings.Repeat("b", 64)
	contentErr := errors.New("physical storage unavailable")

	outputs := &fakePreviewOutputReader{
		output: dbgen.ConversionJobOutput{
			ConversionJobID: uuid.New(),
			ContentSha256:   contentSHA256,
			MediaType:       GLBMediaType,
		},
	}
	content := &fakePreviewContentReader{
		err: contentErr,
	}

	service := NewPreviewService(
		&fakeProjectReader{},
		&fakeProjectFileReader{},
		outputs,
		content,
	)

	result, err := service.GetLatest(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
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
		t.Fatal("expected no preview content")
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
}

func TestGetLatestPreviewReturnsGLBContent(
	t *testing.T,
) {
	ownerUserID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()
	conversionJobID := uuid.New()
	contentSHA256 := strings.Repeat("c", 64)
	expectedContent := []byte("derived GLB preview bytes")

	projects := &fakeProjectReader{
		project: dbgen.Project{
			ID:          projectID,
			OwnerUserID: ownerUserID,
		},
	}
	files := &fakeProjectFileReader{
		projectFile: dbgen.ProjectFile{
			ID:        projectFileID,
			ProjectID: projectID,
		},
	}
	outputs := &fakePreviewOutputReader{
		output: dbgen.ConversionJobOutput{
			ConversionJobID: conversionJobID,
			ContentSha256:   contentSHA256,
			MediaType:       GLBMediaType,
		},
	}
	content := &fakePreviewContentReader{
		content: io.NopCloser(
			bytes.NewReader(expectedContent),
		),
	}

	service := NewPreviewService(
		projects,
		files,
		outputs,
		content,
	)

	result, err := service.GetLatest(
		context.Background(),
		ownerUserID,
		projectID,
		projectFileID,
	)
	if err != nil {
		t.Fatalf("get latest preview: %v", err)
	}
	defer result.Content.Close()

	if projects.params.OwnerUserID != ownerUserID ||
		projects.params.ProjectID != projectID {
		t.Fatalf(
			"unexpected project lookup params: %+v",
			projects.params,
		)
	}

	if files.params.ProjectID != projectID ||
		files.params.ProjectFileID != projectFileID {
		t.Fatalf(
			"unexpected project file lookup params: %+v",
			files.params,
		)
	}

	if outputs.params.ProjectID != projectID ||
		outputs.params.ProjectFileID != projectFileID {
		t.Fatalf(
			"unexpected preview output lookup params: %+v",
			outputs.params,
		)
	}

	if content.contentSHA256 != contentSHA256 {
		t.Fatalf(
			"expected content SHA-256 %q, got %q",
			contentSHA256,
			content.contentSHA256,
		)
	}

	if result.Output.ConversionJobID != conversionJobID ||
		result.Output.ContentSha256 != contentSHA256 ||
		result.Output.MediaType != GLBMediaType {
		t.Fatalf(
			"unexpected preview output: %+v",
			result.Output,
		)
	}

	actualContent, err := io.ReadAll(result.Content)
	if err != nil {
		t.Fatalf("read preview content: %v", err)
	}

	if !bytes.Equal(actualContent, expectedContent) {
		t.Fatalf(
			"expected preview bytes %q, got %q",
			expectedContent,
			actualContent,
		)
	}
}
