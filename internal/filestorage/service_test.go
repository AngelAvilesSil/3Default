package filestorage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeProjectReader struct {
	called  bool
	params  dbgen.GetProjectByIDAndOwnerParams
	project dbgen.Project
	err     error
}

func (f *fakeProjectReader) GetProjectByIDAndOwner(
	_ context.Context,
	arg dbgen.GetProjectByIDAndOwnerParams,
) (dbgen.Project, error) {
	f.called = true
	f.params = arg

	return f.project, f.err
}

type fakeFileStore struct {
	createCalled bool
	createParams CreateProjectFileParams
	createFile   dbgen.ProjectFile
	createErr    error

	getCalled bool
	getParams dbgen.GetProjectFileByIDAndProjectParams
	getFile   dbgen.ProjectFile
	getErr    error

	listCalled    bool
	listProjectID uuid.UUID
	files         []dbgen.ProjectFile
	listErr       error
}

func (f *fakeFileStore) CreateProjectFileWithContentObject(
	_ context.Context,
	arg CreateProjectFileParams,
) (dbgen.ProjectFile, error) {
	f.createCalled = true
	f.createParams = arg

	return f.createFile, f.createErr
}

func (f *fakeFileStore) GetProjectFileByIDAndProject(
	_ context.Context,
	arg dbgen.GetProjectFileByIDAndProjectParams,
) (dbgen.ProjectFile, error) {
	f.getCalled = true
	f.getParams = arg

	return f.getFile, f.getErr
}

func (f *fakeFileStore) ListProjectFilesByProject(
	_ context.Context,
	projectID uuid.UUID,
) ([]dbgen.ProjectFile, error) {
	f.listCalled = true
	f.listProjectID = projectID

	return f.files, f.listErr
}

func TestCreateNormalizesInputAndCreatesProjectFile(
	t *testing.T,
) {
	ownerID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()

	uppercaseSHA256 := strings.Repeat("A", 64)
	expectedSHA256 := strings.Repeat("a", 64)

	mediaType := "  model/step  "

	projects := &fakeProjectReader{
		project: dbgen.Project{
			ID:          projectID,
			OwnerUserID: ownerID,
		},
	}

	files := &fakeFileStore{
		createFile: dbgen.ProjectFile{
			ID:               projectFileID,
			ProjectID:        projectID,
			UploadedByUserID: ownerID,
			ContentSha256:    expectedSHA256,
			OriginalFilename: "gripper.step",
		},
	}

	service := NewService(projects, files)

	projectFile, err := service.Create(
		context.Background(),
		CreateInput{
			OwnerUserID:      ownerID,
			ProjectID:        projectID,
			ContentSHA256:    "  " + uppercaseSHA256 + "  ",
			SizeBytes:        512,
			OriginalFilename: "  gripper.step  ",
			MediaType:        &mediaType,
		},
	)
	if err != nil {
		t.Fatalf("create project file: %v", err)
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

	if !files.createCalled {
		t.Fatal("expected project file store call")
	}

	if files.createParams.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			files.createParams.ProjectID,
		)
	}

	if files.createParams.UploadedByUserID != ownerID {
		t.Fatalf(
			"expected uploader ID %s, got %s",
			ownerID,
			files.createParams.UploadedByUserID,
		)
	}

	if files.createParams.ContentSHA256 != expectedSHA256 {
		t.Fatalf(
			"expected normalized SHA-256 %q, got %q",
			expectedSHA256,
			files.createParams.ContentSHA256,
		)
	}

	if files.createParams.SizeBytes != 512 {
		t.Fatalf(
			"expected size %d, got %d",
			512,
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

func TestCreateConvertsBlankMediaTypeToNil(
	t *testing.T,
) {
	blankMediaType := "   "

	projects := &fakeProjectReader{}
	files := &fakeFileStore{}

	service := NewService(projects, files)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			OwnerUserID:      uuid.New(),
			ProjectID:        uuid.New(),
			ContentSHA256:    strings.Repeat("a", 64),
			SizeBytes:        0,
			OriginalFilename: "empty.step",
			MediaType:        &blankMediaType,
		},
	)
	if err != nil {
		t.Fatalf("create project file: %v", err)
	}

	if files.createParams.MediaType != nil {
		t.Fatalf(
			"expected nil media type, got %q",
			*files.createParams.MediaType,
		)
	}
}

func TestCreateRejectsInvalidInput(
	t *testing.T,
) {
	ownerID := uuid.New()
	projectID := uuid.New()
	validSHA256 := strings.Repeat("a", 64)

	tests := []struct {
		name  string
		input CreateInput
		want  error
	}{
		{
			name: "missing owner",
			input: CreateInput{
				ProjectID:        projectID,
				ContentSHA256:    validSHA256,
				OriginalFilename: "part.step",
			},
			want: ErrOwnerRequired,
		},
		{
			name: "missing project ID",
			input: CreateInput{
				OwnerUserID:      ownerID,
				ContentSHA256:    validSHA256,
				OriginalFilename: "part.step",
			},
			want: ErrProjectIDRequired,
		},
		{
			name: "missing content SHA-256",
			input: CreateInput{
				OwnerUserID:      ownerID,
				ProjectID:        projectID,
				ContentSHA256:    "   ",
				OriginalFilename: "part.step",
			},
			want: ErrContentSHA256Required,
		},
		{
			name: "short content SHA-256",
			input: CreateInput{
				OwnerUserID:      ownerID,
				ProjectID:        projectID,
				ContentSHA256:    strings.Repeat("a", 63),
				OriginalFilename: "part.step",
			},
			want: ErrContentSHA256Invalid,
		},
		{
			name: "nonhex content SHA-256",
			input: CreateInput{
				OwnerUserID:      ownerID,
				ProjectID:        projectID,
				ContentSHA256:    strings.Repeat("g", 64),
				OriginalFilename: "part.step",
			},
			want: ErrContentSHA256Invalid,
		},
		{
			name: "negative content size",
			input: CreateInput{
				OwnerUserID:      ownerID,
				ProjectID:        projectID,
				ContentSHA256:    validSHA256,
				SizeBytes:        -1,
				OriginalFilename: "part.step",
			},
			want: ErrSizeBytesInvalid,
		},
		{
			name: "blank original filename",
			input: CreateInput{
				OwnerUserID:      ownerID,
				ProjectID:        projectID,
				ContentSHA256:    validSHA256,
				OriginalFilename: "   ",
			},
			want: ErrOriginalFilenameRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := &fakeProjectReader{}
			files := &fakeFileStore{}

			service := NewService(projects, files)

			_, err := service.Create(
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

			if files.createCalled {
				t.Fatal(
					"expected file store not to be called",
				)
			}
		})
	}
}

func TestCreateMapsMissingProjectToNotFound(
	t *testing.T,
) {
	projects := &fakeProjectReader{
		err: pgx.ErrNoRows,
	}
	files := &fakeFileStore{}

	service := NewService(projects, files)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			OwnerUserID:      uuid.New(),
			ProjectID:        uuid.New(),
			ContentSHA256:    strings.Repeat("a", 64),
			OriginalFilename: "part.step",
		},
	)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf(
			"expected ErrProjectNotFound, got %v",
			err,
		)
	}

	if files.createCalled {
		t.Fatal(
			"expected file store not to be called",
		)
	}
}

func TestCreateWrapsProjectLookupError(
	t *testing.T,
) {
	databaseErr := errors.New("database unavailable")

	projects := &fakeProjectReader{
		err: databaseErr,
	}
	files := &fakeFileStore{}

	service := NewService(projects, files)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			OwnerUserID:      uuid.New(),
			ProjectID:        uuid.New(),
			ContentSHA256:    strings.Repeat("a", 64),
			OriginalFilename: "part.step",
		},
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped project lookup error, got %v",
			err,
		)
	}

	if files.createCalled {
		t.Fatal(
			"expected file store not to be called",
		)
	}
}

func TestCreatePreservesContentObjectSizeConflict(
	t *testing.T,
) {
	projects := &fakeProjectReader{}
	files := &fakeFileStore{
		createErr: ErrContentObjectSizeConflict,
	}

	service := NewService(projects, files)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			OwnerUserID:      uuid.New(),
			ProjectID:        uuid.New(),
			ContentSHA256:    strings.Repeat("a", 64),
			SizeBytes:        100,
			OriginalFilename: "part.step",
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
}

func TestCreateWrapsFileStoreError(
	t *testing.T,
) {
	databaseErr := errors.New("database unavailable")

	projects := &fakeProjectReader{}
	files := &fakeFileStore{
		createErr: databaseErr,
	}

	service := NewService(projects, files)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			OwnerUserID:      uuid.New(),
			ProjectID:        uuid.New(),
			ContentSHA256:    strings.Repeat("a", 64),
			OriginalFilename: "part.step",
		},
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped file store error, got %v",
			err,
		)
	}
}

func TestGetReturnsProjectFileForOwnedProject(
	t *testing.T,
) {
	ownerID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()

	projects := &fakeProjectReader{}

	files := &fakeFileStore{
		getFile: dbgen.ProjectFile{
			ID:               projectFileID,
			ProjectID:        projectID,
			UploadedByUserID: ownerID,
			ContentSha256:    strings.Repeat("a", 64),
			OriginalFilename: "gripper.step",
		},
	}

	service := NewService(projects, files)

	projectFile, err := service.Get(
		context.Background(),
		ownerID,
		projectID,
		projectFileID,
	)
	if err != nil {
		t.Fatalf("get project file: %v", err)
	}

	if !projects.called {
		t.Fatal("expected project ownership lookup")
	}

	if projects.params.OwnerUserID != ownerID ||
		projects.params.ProjectID != projectID {
		t.Fatalf(
			"unexpected project lookup params: %+v",
			projects.params,
		)
	}

	if !files.getCalled {
		t.Fatal("expected project file lookup")
	}

	if files.getParams.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			files.getParams.ProjectID,
		)
	}

	if files.getParams.ProjectFileID != projectFileID {
		t.Fatalf(
			"expected project file ID %s, got %s",
			projectFileID,
			files.getParams.ProjectFileID,
		)
	}

	if projectFile.ID != projectFileID {
		t.Fatalf(
			"expected returned project file ID %s, got %s",
			projectFileID,
			projectFile.ID,
		)
	}
}

func TestGetRejectsInvalidInput(
	t *testing.T,
) {
	ownerID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()

	tests := []struct {
		name          string
		ownerID       uuid.UUID
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
			name:          "missing project ID",
			ownerID:       ownerID,
			projectFileID: projectFileID,
			want:          ErrProjectIDRequired,
		},
		{
			name:      "missing project file ID",
			ownerID:   ownerID,
			projectID: projectID,
			want:      ErrProjectFileIDRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := &fakeProjectReader{}
			files := &fakeFileStore{}

			service := NewService(projects, files)

			_, err := service.Get(
				context.Background(),
				test.ownerID,
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

			if projects.called {
				t.Fatal(
					"expected project store not to be called",
				)
			}

			if files.getCalled {
				t.Fatal(
					"expected file store not to be called",
				)
			}
		})
	}
}

func TestGetMapsMissingProjectToNotFound(
	t *testing.T,
) {
	projects := &fakeProjectReader{
		err: pgx.ErrNoRows,
	}
	files := &fakeFileStore{}

	service := NewService(projects, files)

	_, err := service.Get(
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

	if files.getCalled {
		t.Fatal(
			"expected project file lookup not to be called",
		)
	}
}

func TestGetWrapsProjectLookupError(
	t *testing.T,
) {
	databaseErr := errors.New("database unavailable")

	projects := &fakeProjectReader{
		err: databaseErr,
	}
	files := &fakeFileStore{}

	service := NewService(projects, files)

	_, err := service.Get(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped project lookup error, got %v",
			err,
		)
	}

	if files.getCalled {
		t.Fatal(
			"expected project file lookup not to be called",
		)
	}
}

func TestGetMapsMissingProjectFileToNotFound(
	t *testing.T,
) {
	service := NewService(
		&fakeProjectReader{},
		&fakeFileStore{
			getErr: pgx.ErrNoRows,
		},
	)

	_, err := service.Get(
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
}

func TestGetWrapsFileStoreError(
	t *testing.T,
) {
	databaseErr := errors.New("database unavailable")

	service := NewService(
		&fakeProjectReader{},
		&fakeFileStore{
			getErr: databaseErr,
		},
	)

	_, err := service.Get(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped file store error, got %v",
			err,
		)
	}
}

func TestListReturnsProjectFilesForOwnedProject(
	t *testing.T,
) {
	ownerID := uuid.New()
	projectID := uuid.New()

	expected := []dbgen.ProjectFile{
		{
			ID:               uuid.New(),
			ProjectID:        projectID,
			UploadedByUserID: ownerID,
			ContentSha256:    strings.Repeat("a", 64),
			OriginalFilename: "first.step",
		},
		{
			ID:               uuid.New(),
			ProjectID:        projectID,
			UploadedByUserID: ownerID,
			ContentSha256:    strings.Repeat("b", 64),
			OriginalFilename: "second.step",
		},
	}

	projects := &fakeProjectReader{}
	files := &fakeFileStore{
		files: expected,
	}

	service := NewService(projects, files)

	projectFiles, err := service.List(
		context.Background(),
		ownerID,
		projectID,
	)
	if err != nil {
		t.Fatalf("list project files: %v", err)
	}

	if !projects.called {
		t.Fatal("expected project ownership lookup")
	}

	if projects.params.OwnerUserID != ownerID ||
		projects.params.ProjectID != projectID {
		t.Fatalf(
			"unexpected project lookup params: %+v",
			projects.params,
		)
	}

	if !files.listCalled {
		t.Fatal("expected project file list store call")
	}

	if files.listProjectID != projectID {
		t.Fatalf(
			"expected list project ID %s, got %s",
			projectID,
			files.listProjectID,
		)
	}

	if len(projectFiles) != len(expected) {
		t.Fatalf(
			"expected %d project files, got %d",
			len(expected),
			len(projectFiles),
		)
	}

	for i := range expected {
		if projectFiles[i].ID != expected[i].ID {
			t.Fatalf(
				"expected project file %d ID %s, got %s",
				i,
				expected[i].ID,
				projectFiles[i].ID,
			)
		}
	}
}

func TestListReturnsEmptySliceWhenStoreReturnsNil(
	t *testing.T,
) {
	service := NewService(
		&fakeProjectReader{},
		&fakeFileStore{},
	)

	projectFiles, err := service.List(
		context.Background(),
		uuid.New(),
		uuid.New(),
	)
	if err != nil {
		t.Fatalf("list project files: %v", err)
	}

	if projectFiles == nil {
		t.Fatal(
			"expected empty non-nil project file slice",
		)
	}

	if len(projectFiles) != 0 {
		t.Fatalf(
			"expected 0 project files, got %d",
			len(projectFiles),
		)
	}
}

func TestListRejectsInvalidInput(
	t *testing.T,
) {
	tests := []struct {
		name      string
		ownerID   uuid.UUID
		projectID uuid.UUID
		want      error
	}{
		{
			name:      "missing owner",
			projectID: uuid.New(),
			want:      ErrOwnerRequired,
		},
		{
			name:    "missing project ID",
			ownerID: uuid.New(),
			want:    ErrProjectIDRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := &fakeProjectReader{}
			files := &fakeFileStore{}

			service := NewService(projects, files)

			_, err := service.List(
				context.Background(),
				test.ownerID,
				test.projectID,
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

			if files.listCalled {
				t.Fatal(
					"expected file store not to be called",
				)
			}
		})
	}
}

func TestListMapsMissingProjectToNotFound(
	t *testing.T,
) {
	projects := &fakeProjectReader{
		err: pgx.ErrNoRows,
	}
	files := &fakeFileStore{}

	service := NewService(projects, files)

	_, err := service.List(
		context.Background(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf(
			"expected ErrProjectNotFound, got %v",
			err,
		)
	}

	if files.listCalled {
		t.Fatal(
			"expected project file list not to be called",
		)
	}
}

func TestListWrapsProjectLookupError(
	t *testing.T,
) {
	databaseErr := errors.New("database unavailable")

	projects := &fakeProjectReader{
		err: databaseErr,
	}
	files := &fakeFileStore{}

	service := NewService(projects, files)

	_, err := service.List(
		context.Background(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped project lookup error, got %v",
			err,
		)
	}

	if files.listCalled {
		t.Fatal(
			"expected project file list not to be called",
		)
	}
}

func TestListWrapsFileStoreError(
	t *testing.T,
) {
	databaseErr := errors.New("database unavailable")

	service := NewService(
		&fakeProjectReader{},
		&fakeFileStore{
			listErr: databaseErr,
		},
	)

	_, err := service.List(
		context.Background(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped file store error, got %v",
			err,
		)
	}
}
