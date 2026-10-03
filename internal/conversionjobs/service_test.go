package conversionjobs

import (
	"context"
	"errors"
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

type fakeProjectFileReader struct {
	called      bool
	params      dbgen.GetProjectFileByIDAndProjectParams
	projectFile dbgen.ProjectFile
	err         error
}

func (f *fakeProjectFileReader) GetProjectFileByIDAndProject(
	_ context.Context,
	arg dbgen.GetProjectFileByIDAndProjectParams,
) (dbgen.ProjectFile, error) {
	f.called = true
	f.params = arg
	return f.projectFile, f.err
}

type fakeStore struct {
	createCalled bool
	createParams dbgen.CreateConversionJobParams
	createJob    dbgen.ConversionJob
	createErr    error

	getCalled bool
	getParams dbgen.GetConversionJobByIDAndProjectParams
	getJob    dbgen.ConversionJob
	getErr    error

	listCalled bool
	listParams dbgen.ListConversionJobsByProjectFileParams
	listJobs   []dbgen.ConversionJob
	listErr    error
}

func (f *fakeStore) CreateConversionJob(
	_ context.Context,
	arg dbgen.CreateConversionJobParams,
) (dbgen.ConversionJob, error) {
	f.createCalled = true
	f.createParams = arg
	return f.createJob, f.createErr
}

func (f *fakeStore) GetConversionJobByIDAndProject(
	_ context.Context,
	arg dbgen.GetConversionJobByIDAndProjectParams,
) (dbgen.ConversionJob, error) {
	f.getCalled = true
	f.getParams = arg
	return f.getJob, f.getErr
}

func (f *fakeStore) ListConversionJobsByProjectFile(
	_ context.Context,
	arg dbgen.ListConversionJobsByProjectFileParams,
) ([]dbgen.ConversionJob, error) {
	f.listCalled = true
	f.listParams = arg
	return f.listJobs, f.listErr
}

func TestCreateCreatesJobForOwnedProjectFile(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()
	jobID := uuid.New()

	projects := &fakeProjectReader{
		project: dbgen.Project{
			ID:          projectID,
			OwnerUserID: ownerID,
		},
	}
	files := &fakeProjectFileReader{
		projectFile: dbgen.ProjectFile{
			ID:        projectFileID,
			ProjectID: projectID,
		},
	}
	jobs := &fakeStore{
		createJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     projectID,
			ProjectFileID: projectFileID,
			Status:        "pending",
		},
	}

	service := NewService(projects, files, jobs)

	job, err := service.Create(
		context.Background(),
		CreateInput{
			OwnerUserID:   ownerID,
			ProjectID:     projectID,
			ProjectFileID: projectFileID,
		},
	)
	if err != nil {
		t.Fatalf("create conversion job: %v", err)
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

	if !files.called {
		t.Fatal("expected project file lookup")
	}
	if files.params.ProjectID != projectID ||
		files.params.ProjectFileID != projectFileID {
		t.Fatalf(
			"unexpected project file lookup params: %+v",
			files.params,
		)
	}

	if !jobs.createCalled {
		t.Fatal("expected conversion job store call")
	}
	if jobs.createParams.ProjectID != projectID ||
		jobs.createParams.ProjectFileID != projectFileID {
		t.Fatalf(
			"unexpected conversion job create params: %+v",
			jobs.createParams,
		)
	}

	if job.ID != jobID {
		t.Fatalf(
			"expected conversion job ID %s, got %s",
			jobID,
			job.ID,
		)
	}
}

func TestCreateRejectsInvalidInput(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()

	tests := []struct {
		name  string
		input CreateInput
		want  error
	}{
		{
			name: "missing owner",
			input: CreateInput{
				ProjectID:     projectID,
				ProjectFileID: projectFileID,
			},
			want: ErrOwnerRequired,
		},
		{
			name: "missing project",
			input: CreateInput{
				OwnerUserID:   ownerID,
				ProjectFileID: projectFileID,
			},
			want: ErrProjectIDRequired,
		},
		{
			name: "missing project file",
			input: CreateInput{
				OwnerUserID: ownerID,
				ProjectID:   projectID,
			},
			want: ErrProjectFileIDRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := &fakeProjectReader{}
			files := &fakeProjectFileReader{}
			jobs := &fakeStore{}

			service := NewService(projects, files, jobs)

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

			if projects.called ||
				files.called ||
				jobs.createCalled {
				t.Fatal(
					"expected no dependency calls for invalid input",
				)
			}
		})
	}
}

func TestCreateMapsMissingProjectToNotFound(t *testing.T) {
	projects := &fakeProjectReader{
		err: pgx.ErrNoRows,
	}
	files := &fakeProjectFileReader{}
	jobs := &fakeStore{}

	service := NewService(projects, files, jobs)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			OwnerUserID:   uuid.New(),
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
		},
	)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf(
			"expected ErrProjectNotFound, got %v",
			err,
		)
	}

	if files.called {
		t.Fatal(
			"expected project file lookup not to run",
		)
	}
	if jobs.createCalled {
		t.Fatal(
			"expected job creation not to run",
		)
	}
}

func TestCreateMapsMissingProjectFileToNotFound(t *testing.T) {
	projects := &fakeProjectReader{}
	files := &fakeProjectFileReader{
		err: pgx.ErrNoRows,
	}
	jobs := &fakeStore{}

	service := NewService(projects, files, jobs)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			OwnerUserID:   uuid.New(),
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
		},
	)
	if !errors.Is(err, ErrProjectFileNotFound) {
		t.Fatalf(
			"expected ErrProjectFileNotFound, got %v",
			err,
		)
	}

	if jobs.createCalled {
		t.Fatal(
			"expected job creation not to run",
		)
	}
}

func TestCreatePreservesActiveJobConflict(t *testing.T) {
	projects := &fakeProjectReader{}
	files := &fakeProjectFileReader{}
	jobs := &fakeStore{
		createErr: ErrActiveConversionJobExists,
	}

	service := NewService(projects, files, jobs)

	_, err := service.Create(
		context.Background(),
		CreateInput{
			OwnerUserID:   uuid.New(),
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
		},
	)
	if !errors.Is(err, ErrActiveConversionJobExists) {
		t.Fatalf(
			"expected ErrActiveConversionJobExists, got %v",
			err,
		)
	}
}

func TestCreateWrapsDependencyErrors(t *testing.T) {
	projectErr := errors.New("project lookup failed")
	fileErr := errors.New("file lookup failed")
	storeErr := errors.New("job store failed")

	tests := []struct {
		name     string
		projects *fakeProjectReader
		files    *fakeProjectFileReader
		jobs     *fakeStore
		want     error
	}{
		{
			name: "project lookup",
			projects: &fakeProjectReader{
				err: projectErr,
			},
			files: &fakeProjectFileReader{},
			jobs:  &fakeStore{},
			want:  projectErr,
		},
		{
			name:     "project file lookup",
			projects: &fakeProjectReader{},
			files: &fakeProjectFileReader{
				err: fileErr,
			},
			jobs: &fakeStore{},
			want: fileErr,
		},
		{
			name:     "job creation",
			projects: &fakeProjectReader{},
			files:    &fakeProjectFileReader{},
			jobs: &fakeStore{
				createErr: storeErr,
			},
			want: storeErr,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(
				test.projects,
				test.files,
				test.jobs,
			)

			_, err := service.Create(
				context.Background(),
				CreateInput{
					OwnerUserID:   uuid.New(),
					ProjectID:     uuid.New(),
					ProjectFileID: uuid.New(),
				},
			)
			if !errors.Is(err, test.want) {
				t.Fatalf(
					"expected wrapped %v, got %v",
					test.want,
					err,
				)
			}
		})
	}
}

func TestGetReturnsOwnedProjectJob(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()

	projects := &fakeProjectReader{}
	files := &fakeProjectFileReader{}
	jobs := &fakeStore{
		getJob: dbgen.ConversionJob{
			ID:        jobID,
			ProjectID: projectID,
			Status:    "pending",
		},
	}

	service := NewService(projects, files, jobs)

	job, err := service.Get(
		context.Background(),
		ownerID,
		projectID,
		jobID,
	)
	if err != nil {
		t.Fatalf("get conversion job: %v", err)
	}

	if !projects.called {
		t.Fatal("expected project ownership lookup")
	}

	if files.called {
		t.Fatal(
			"did not expect project file lookup for job get",
		)
	}

	if !jobs.getCalled {
		t.Fatal("expected conversion job lookup")
	}

	if jobs.getParams.ProjectID != projectID ||
		jobs.getParams.ConversionJobID != jobID {
		t.Fatalf(
			"unexpected conversion job lookup params: %+v",
			jobs.getParams,
		)
	}

	if job.ID != jobID {
		t.Fatalf(
			"expected conversion job ID %s, got %s",
			jobID,
			job.ID,
		)
	}
}

func TestGetRejectsInvalidInput(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	jobID := uuid.New()

	tests := []struct {
		name      string
		ownerID   uuid.UUID
		projectID uuid.UUID
		jobID     uuid.UUID
		want      error
	}{
		{
			name:      "missing owner",
			projectID: projectID,
			jobID:     jobID,
			want:      ErrOwnerRequired,
		},
		{
			name:    "missing project",
			ownerID: ownerID,
			jobID:   jobID,
			want:    ErrProjectIDRequired,
		},
		{
			name:      "missing conversion job",
			ownerID:   ownerID,
			projectID: projectID,
			want:      ErrConversionJobIDRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := &fakeProjectReader{}
			files := &fakeProjectFileReader{}
			jobs := &fakeStore{}

			service := NewService(projects, files, jobs)

			_, err := service.Get(
				context.Background(),
				test.ownerID,
				test.projectID,
				test.jobID,
			)
			if !errors.Is(err, test.want) {
				t.Fatalf(
					"expected %v, got %v",
					test.want,
					err,
				)
			}

			if projects.called || jobs.getCalled {
				t.Fatal(
					"expected no dependency calls for invalid input",
				)
			}
		})
	}
}

func TestGetMapsMissingJobToNotFound(t *testing.T) {
	projects := &fakeProjectReader{}
	files := &fakeProjectFileReader{}
	jobs := &fakeStore{
		getErr: pgx.ErrNoRows,
	}

	service := NewService(projects, files, jobs)

	_, err := service.Get(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, ErrConversionJobNotFound) {
		t.Fatalf(
			"expected ErrConversionJobNotFound, got %v",
			err,
		)
	}
}

func TestGetMapsMissingProjectBeforeJobLookup(t *testing.T) {
	projects := &fakeProjectReader{
		err: pgx.ErrNoRows,
	}
	files := &fakeProjectFileReader{}
	jobs := &fakeStore{}

	service := NewService(projects, files, jobs)

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

	if jobs.getCalled {
		t.Fatal(
			"expected job lookup not to run for missing project",
		)
	}
}

func TestGetWrapsStoreError(t *testing.T) {
	storeErr := errors.New("get job failed")

	projects := &fakeProjectReader{}
	files := &fakeProjectFileReader{}
	jobs := &fakeStore{
		getErr: storeErr,
	}

	service := NewService(projects, files, jobs)

	_, err := service.Get(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, storeErr) {
		t.Fatalf(
			"expected wrapped store error, got %v",
			err,
		)
	}
}

func TestListForProjectFileReturnsJobs(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()

	expected := []dbgen.ConversionJob{
		{
			ID:            uuid.New(),
			ProjectID:     projectID,
			ProjectFileID: projectFileID,
			Status:        "failed",
		},
		{
			ID:            uuid.New(),
			ProjectID:     projectID,
			ProjectFileID: projectFileID,
			Status:        "pending",
		},
	}

	projects := &fakeProjectReader{}
	files := &fakeProjectFileReader{}
	jobs := &fakeStore{
		listJobs: expected,
	}

	service := NewService(projects, files, jobs)

	actual, err := service.ListForProjectFile(
		context.Background(),
		ownerID,
		projectID,
		projectFileID,
	)
	if err != nil {
		t.Fatalf(
			"list project file conversion jobs: %v",
			err,
		)
	}

	if !projects.called {
		t.Fatal("expected project ownership lookup")
	}
	if !files.called {
		t.Fatal("expected project file lookup")
	}
	if !jobs.listCalled {
		t.Fatal("expected conversion job list call")
	}

	if jobs.listParams.ProjectID != projectID ||
		jobs.listParams.ProjectFileID != projectFileID {
		t.Fatalf(
			"unexpected conversion job list params: %+v",
			jobs.listParams,
		)
	}

	if len(actual) != len(expected) {
		t.Fatalf(
			"expected %d jobs, got %d",
			len(expected),
			len(actual),
		)
	}
}

func TestListForProjectFileReturnsEmptySliceForNilStoreResult(
	t *testing.T,
) {
	service := NewService(
		&fakeProjectReader{},
		&fakeProjectFileReader{},
		&fakeStore{},
	)

	jobs, err := service.ListForProjectFile(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)
	if err != nil {
		t.Fatalf(
			"list project file conversion jobs: %v",
			err,
		)
	}

	if jobs == nil {
		t.Fatal("expected non-nil empty conversion job slice")
	}

	if len(jobs) != 0 {
		t.Fatalf(
			"expected 0 jobs, got %d",
			len(jobs),
		)
	}
}

func TestListForProjectFileRejectsInvalidInput(t *testing.T) {
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
			name:          "missing project",
			ownerID:       ownerID,
			projectFileID: projectFileID,
			want:          ErrProjectIDRequired,
		},
		{
			name:      "missing project file",
			ownerID:   ownerID,
			projectID: projectID,
			want:      ErrProjectFileIDRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := &fakeProjectReader{}
			files := &fakeProjectFileReader{}
			jobs := &fakeStore{}

			service := NewService(projects, files, jobs)

			_, err := service.ListForProjectFile(
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

			if projects.called ||
				files.called ||
				jobs.listCalled {
				t.Fatal(
					"expected no dependency calls for invalid input",
				)
			}
		})
	}
}

func TestListForProjectFileMapsMissingFileToNotFound(
	t *testing.T,
) {
	projects := &fakeProjectReader{}
	files := &fakeProjectFileReader{
		err: pgx.ErrNoRows,
	}
	jobs := &fakeStore{}

	service := NewService(projects, files, jobs)

	_, err := service.ListForProjectFile(
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

	if jobs.listCalled {
		t.Fatal(
			"expected conversion job listing not to run",
		)
	}
}

func TestListForProjectFileWrapsStoreError(t *testing.T) {
	storeErr := errors.New("list jobs failed")

	service := NewService(
		&fakeProjectReader{},
		&fakeProjectFileReader{},
		&fakeStore{
			listErr: storeErr,
		},
	)

	_, err := service.ListForProjectFile(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, storeErr) {
		t.Fatalf(
			"expected wrapped store error, got %v",
			err,
		)
	}
}
