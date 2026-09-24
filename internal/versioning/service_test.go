package versioning

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

type fakeRevisionStore struct {
	called   bool
	params   CreateRevisionOnBranchParams
	revision dbgen.ProjectRevision
	err      error

	createBranchCalled bool
	createBranchParams CreateBranchParams
	createdBranch      dbgen.ProjectBranch
	createBranchErr    error

	listCalled    bool
	listProjectID uuid.UUID
	branches      []dbgen.ProjectBranch
	listErr       error

	getCalled   bool
	getParams   dbgen.GetProjectRevisionByIDAndProjectParams
	getRevision dbgen.ProjectRevision
	getErr      error
}

func (f *fakeRevisionStore) CreateBranch(
	_ context.Context,
	arg CreateBranchParams,
) (dbgen.ProjectBranch, error) {
	f.createBranchCalled = true
	f.createBranchParams = arg

	return f.createdBranch, f.createBranchErr
}

func (f *fakeRevisionStore) CreateRevisionOnBranch(
	_ context.Context,
	arg CreateRevisionOnBranchParams,
) (dbgen.ProjectRevision, error) {
	f.called = true
	f.params = arg

	return f.revision, f.err
}

func (f *fakeRevisionStore) GetProjectRevisionByIDAndProject(
	_ context.Context,
	arg dbgen.GetProjectRevisionByIDAndProjectParams,
) (dbgen.ProjectRevision, error) {
	f.getCalled = true
	f.getParams = arg

	return f.getRevision, f.getErr
}

func (f *fakeRevisionStore) ListProjectBranchesByProject(
	_ context.Context,
	projectID uuid.UUID,
) ([]dbgen.ProjectBranch, error) {
	f.listCalled = true
	f.listProjectID = projectID

	return f.branches, f.listErr
}

func TestCreateBranchNormalizesInputAndCreatesBranch(
	t *testing.T,
) {
	ownerID := uuid.New()
	projectID := uuid.New()
	headRevisionID := uuid.New()
	branchID := uuid.New()

	projects := &fakeProjectReader{
		project: dbgen.Project{
			ID:          projectID,
			OwnerUserID: ownerID,
		},
	}
	revisions := &fakeRevisionStore{
		getRevision: dbgen.ProjectRevision{
			ID:        headRevisionID,
			ProjectID: projectID,
		},
		createdBranch: dbgen.ProjectBranch{
			ID:        branchID,
			ProjectID: projectID,
			Name:      "feature-gripper",
		},
	}

	service := NewService(projects, revisions)

	branch, err := service.CreateBranch(
		context.Background(),
		CreateBranchInput{
			OwnerUserID:    ownerID,
			ProjectID:      projectID,
			Name:           "  feature-gripper  ",
			HeadRevisionID: &headRevisionID,
		},
	)
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}

	if !projects.called {
		t.Fatal("expected project ownership lookup")
	}

	if projects.params.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			projects.params.ProjectID,
		)
	}

	if projects.params.OwnerUserID != ownerID {
		t.Fatalf(
			"expected owner ID %s, got %s",
			ownerID,
			projects.params.OwnerUserID,
		)
	}

	if !revisions.getCalled {
		t.Fatal("expected branch head revision lookup")
	}

	if revisions.getParams.ProjectID != projectID {
		t.Fatalf(
			"expected revision project ID %s, got %s",
			projectID,
			revisions.getParams.ProjectID,
		)
	}

	if revisions.getParams.RevisionID != headRevisionID {
		t.Fatalf(
			"expected head revision ID %s, got %s",
			headRevisionID,
			revisions.getParams.RevisionID,
		)
	}

	if !revisions.createBranchCalled {
		t.Fatal("expected branch store to be called")
	}

	if revisions.createBranchParams.ProjectID != projectID {
		t.Fatalf(
			"expected branch project ID %s, got %s",
			projectID,
			revisions.createBranchParams.ProjectID,
		)
	}

	if revisions.createBranchParams.Name != "feature-gripper" {
		t.Fatalf(
			"expected normalized branch name %q, got %q",
			"feature-gripper",
			revisions.createBranchParams.Name,
		)
	}

	if revisions.createBranchParams.HeadRevisionID == nil ||
		*revisions.createBranchParams.HeadRevisionID != headRevisionID {
		t.Fatalf(
			"expected branch head revision ID %s",
			headRevisionID,
		)
	}

	if branch.ID != branchID {
		t.Fatalf(
			"expected branch ID %s, got %s",
			branchID,
			branch.ID,
		)
	}
}

func TestCreateBranchAllowsEmptyHead(
	t *testing.T,
) {
	projects := &fakeProjectReader{}
	revisions := &fakeRevisionStore{
		createdBranch: dbgen.ProjectBranch{
			ID: uuid.New(),
		},
	}

	service := NewService(projects, revisions)

	_, err := service.CreateBranch(
		context.Background(),
		CreateBranchInput{
			OwnerUserID: uuid.New(),
			ProjectID:   uuid.New(),
			Name:        "empty-feature",
		},
	)
	if err != nil {
		t.Fatalf("create empty branch: %v", err)
	}

	if revisions.getCalled {
		t.Fatal(
			"expected no revision lookup for empty branch head",
		)
	}

	if !revisions.createBranchCalled {
		t.Fatal("expected branch store to be called")
	}

	if revisions.createBranchParams.HeadRevisionID != nil {
		t.Fatalf(
			"expected nil branch head, got %s",
			*revisions.createBranchParams.HeadRevisionID,
		)
	}
}

func TestCreateBranchRejectsInvalidInput(t *testing.T) {
	zeroID := uuid.Nil

	tests := []struct {
		name  string
		input CreateBranchInput
		want  error
	}{
		{
			name: "missing owner",
			input: CreateBranchInput{
				ProjectID: uuid.New(),
				Name:      "feature",
			},
			want: ErrOwnerRequired,
		},
		{
			name: "missing project ID",
			input: CreateBranchInput{
				OwnerUserID: uuid.New(),
				Name:        "feature",
			},
			want: ErrProjectIDRequired,
		},
		{
			name: "blank branch name",
			input: CreateBranchInput{
				OwnerUserID: uuid.New(),
				ProjectID:   uuid.New(),
				Name:        "   ",
			},
			want: ErrBranchNameRequired,
		},
		{
			name: "zero head revision ID",
			input: CreateBranchInput{
				OwnerUserID:    uuid.New(),
				ProjectID:      uuid.New(),
				Name:           "feature",
				HeadRevisionID: &zeroID,
			},
			want: ErrHeadRevisionIDInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := &fakeProjectReader{}
			revisions := &fakeRevisionStore{}

			service := NewService(projects, revisions)

			_, err := service.CreateBranch(
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

			if revisions.getCalled {
				t.Fatal(
					"expected revision store not to be read",
				)
			}

			if revisions.createBranchCalled {
				t.Fatal(
					"expected branch store not to be called",
				)
			}
		})
	}
}

func TestCreateBranchMapsMissingProjectToNotFound(
	t *testing.T,
) {
	projects := &fakeProjectReader{
		err: pgx.ErrNoRows,
	}
	revisions := &fakeRevisionStore{}

	service := NewService(projects, revisions)

	_, err := service.CreateBranch(
		context.Background(),
		validCreateBranchInput(),
	)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf(
			"expected ErrProjectNotFound, got %v",
			err,
		)
	}

	if revisions.getCalled {
		t.Fatal(
			"expected revision lookup not to be called",
		)
	}

	if revisions.createBranchCalled {
		t.Fatal(
			"expected branch store not to be called",
		)
	}
}

func TestCreateBranchWrapsProjectLookupError(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	projects := &fakeProjectReader{
		err: databaseErr,
	}
	revisions := &fakeRevisionStore{}

	service := NewService(projects, revisions)

	_, err := service.CreateBranch(
		context.Background(),
		validCreateBranchInput(),
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped database error, got %v",
			err,
		)
	}

	if revisions.createBranchCalled {
		t.Fatal(
			"expected branch store not to be called",
		)
	}
}

func TestCreateBranchMapsMissingHeadRevisionToNotFound(
	t *testing.T,
) {
	headRevisionID := uuid.New()

	projects := &fakeProjectReader{}
	revisions := &fakeRevisionStore{
		getErr: pgx.ErrNoRows,
	}

	service := NewService(projects, revisions)

	input := validCreateBranchInput()
	input.HeadRevisionID = &headRevisionID

	_, err := service.CreateBranch(
		context.Background(),
		input,
	)
	if !errors.Is(err, ErrRevisionNotFound) {
		t.Fatalf(
			"expected ErrRevisionNotFound, got %v",
			err,
		)
	}

	if revisions.createBranchCalled {
		t.Fatal(
			"expected branch store not to be called",
		)
	}
}

func TestCreateBranchWrapsHeadRevisionLookupError(
	t *testing.T,
) {
	databaseErr := errors.New("database unavailable")
	headRevisionID := uuid.New()

	projects := &fakeProjectReader{}
	revisions := &fakeRevisionStore{
		getErr: databaseErr,
	}

	service := NewService(projects, revisions)

	input := validCreateBranchInput()
	input.HeadRevisionID = &headRevisionID

	_, err := service.CreateBranch(
		context.Background(),
		input,
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped database error, got %v",
			err,
		)
	}

	if revisions.createBranchCalled {
		t.Fatal(
			"expected branch store not to be called",
		)
	}
}

func TestCreateBranchPreservesNameConflict(
	t *testing.T,
) {
	projects := &fakeProjectReader{}
	revisions := &fakeRevisionStore{
		createBranchErr: ErrBranchNameConflict,
	}

	service := NewService(projects, revisions)

	_, err := service.CreateBranch(
		context.Background(),
		validCreateBranchInput(),
	)
	if !errors.Is(err, ErrBranchNameConflict) {
		t.Fatalf(
			"expected ErrBranchNameConflict, got %v",
			err,
		)
	}
}

func TestCreateBranchWrapsStoreError(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	projects := &fakeProjectReader{}
	revisions := &fakeRevisionStore{
		createBranchErr: databaseErr,
	}

	service := NewService(projects, revisions)

	_, err := service.CreateBranch(
		context.Background(),
		validCreateBranchInput(),
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped database error, got %v",
			err,
		)
	}
}

func validCreateBranchInput() CreateBranchInput {
	return CreateBranchInput{
		OwnerUserID: uuid.New(),
		ProjectID:   uuid.New(),
		Name:        "feature",
	}
}

func TestCreateRevisionNormalizesInputAndCreatesRevision(
	t *testing.T,
) {
	ownerID := uuid.New()
	projectID := uuid.New()
	branchID := uuid.New()
	expectedHeadID := uuid.New()
	mergeParentID := uuid.New()
	revisionID := uuid.New()

	projects := &fakeProjectReader{
		project: dbgen.Project{
			ID:          projectID,
			OwnerUserID: ownerID,
		},
	}
	revisions := &fakeRevisionStore{
		revision: dbgen.ProjectRevision{
			ID:           revisionID,
			ProjectID:    projectID,
			AuthorUserID: ownerID,
			Message:      "Merge feature branch",
		},
	}

	service := NewService(projects, revisions)

	revision, err := service.CreateRevision(
		context.Background(),
		CreateRevisionInput{
			OwnerUserID:            ownerID,
			ProjectID:              projectID,
			BranchID:               branchID,
			Message:                "  Merge feature branch  ",
			ExpectedHeadRevisionID: &expectedHeadID,
			MergeParentRevisionID:  &mergeParentID,
		},
	)
	if err != nil {
		t.Fatalf("create revision: %v", err)
	}

	if !projects.called {
		t.Fatal("expected project ownership lookup")
	}

	if projects.params.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			projects.params.ProjectID,
		)
	}

	if projects.params.OwnerUserID != ownerID {
		t.Fatalf(
			"expected owner ID %s, got %s",
			ownerID,
			projects.params.OwnerUserID,
		)
	}

	if !revisions.called {
		t.Fatal("expected revision store to be called")
	}

	if revisions.params.ProjectID != projectID {
		t.Fatalf(
			"expected revision project ID %s, got %s",
			projectID,
			revisions.params.ProjectID,
		)
	}

	if revisions.params.BranchID != branchID {
		t.Fatalf(
			"expected branch ID %s, got %s",
			branchID,
			revisions.params.BranchID,
		)
	}

	if revisions.params.AuthorUserID != ownerID {
		t.Fatalf(
			"expected author ID %s, got %s",
			ownerID,
			revisions.params.AuthorUserID,
		)
	}

	if revisions.params.Message != "Merge feature branch" {
		t.Fatalf(
			"expected normalized message %q, got %q",
			"Merge feature branch",
			revisions.params.Message,
		)
	}

	if revisions.params.ExpectedHeadRevisionID == nil ||
		*revisions.params.ExpectedHeadRevisionID != expectedHeadID {
		t.Fatalf(
			"expected head revision ID %s",
			expectedHeadID,
		)
	}

	if revisions.params.MergeParentRevisionID == nil ||
		*revisions.params.MergeParentRevisionID != mergeParentID {
		t.Fatalf(
			"expected merge parent revision ID %s",
			mergeParentID,
		)
	}

	if revision.ID != revisionID {
		t.Fatalf(
			"expected revision ID %s, got %s",
			revisionID,
			revision.ID,
		)
	}
}

func TestCreateRevisionRejectsInvalidInput(t *testing.T) {
	zeroID := uuid.Nil
	parentID := uuid.New()

	tests := []struct {
		name  string
		input CreateRevisionInput
		want  error
	}{
		{
			name: "missing owner",
			input: CreateRevisionInput{
				ProjectID: uuid.New(),
				BranchID:  uuid.New(),
				Message:   "Revision",
			},
			want: ErrOwnerRequired,
		},
		{
			name: "missing project ID",
			input: CreateRevisionInput{
				OwnerUserID: uuid.New(),
				BranchID:    uuid.New(),
				Message:     "Revision",
			},
			want: ErrProjectIDRequired,
		},
		{
			name: "missing branch ID",
			input: CreateRevisionInput{
				OwnerUserID: uuid.New(),
				ProjectID:   uuid.New(),
				Message:     "Revision",
			},
			want: ErrBranchIDRequired,
		},
		{
			name: "blank message",
			input: CreateRevisionInput{
				OwnerUserID: uuid.New(),
				ProjectID:   uuid.New(),
				BranchID:    uuid.New(),
				Message:     "   ",
			},
			want: ErrMessageRequired,
		},
		{
			name: "zero expected head revision ID",
			input: CreateRevisionInput{
				OwnerUserID:            uuid.New(),
				ProjectID:              uuid.New(),
				BranchID:               uuid.New(),
				Message:                "Revision",
				ExpectedHeadRevisionID: &zeroID,
			},
			want: ErrExpectedHeadRevisionIDInvalid,
		},
		{
			name: "zero merge parent revision ID",
			input: CreateRevisionInput{
				OwnerUserID:           uuid.New(),
				ProjectID:             uuid.New(),
				BranchID:              uuid.New(),
				Message:               "Revision",
				MergeParentRevisionID: &zeroID,
			},
			want: ErrMergeParentRevisionIDInvalid,
		},
		{
			name: "merge without expected head",
			input: CreateRevisionInput{
				OwnerUserID:           uuid.New(),
				ProjectID:             uuid.New(),
				BranchID:              uuid.New(),
				Message:               "Merge",
				MergeParentRevisionID: &parentID,
			},
			want: ErrMergeRequiresHead,
		},
		{
			name: "identical revision parents",
			input: CreateRevisionInput{
				OwnerUserID:            uuid.New(),
				ProjectID:              uuid.New(),
				BranchID:               uuid.New(),
				Message:                "Merge",
				ExpectedHeadRevisionID: &parentID,
				MergeParentRevisionID:  &parentID,
			},
			want: ErrRevisionParentsMustDiffer,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := &fakeProjectReader{}
			revisions := &fakeRevisionStore{}

			service := NewService(projects, revisions)

			_, err := service.CreateRevision(
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

			if revisions.called {
				t.Fatal(
					"expected revision store not to be called",
				)
			}
		})
	}
}

func TestCreateRevisionMapsMissingProjectToNotFound(
	t *testing.T,
) {
	projects := &fakeProjectReader{
		err: pgx.ErrNoRows,
	}
	revisions := &fakeRevisionStore{}

	service := NewService(projects, revisions)

	_, err := service.CreateRevision(
		context.Background(),
		validCreateRevisionInput(),
	)
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf(
			"expected ErrProjectNotFound, got %v",
			err,
		)
	}

	if revisions.called {
		t.Fatal(
			"expected revision store not to be called",
		)
	}
}

func TestCreateRevisionWrapsProjectLookupError(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	projects := &fakeProjectReader{
		err: databaseErr,
	}
	revisions := &fakeRevisionStore{}

	service := NewService(projects, revisions)

	_, err := service.CreateRevision(
		context.Background(),
		validCreateRevisionInput(),
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped database error, got %v",
			err,
		)
	}

	if revisions.called {
		t.Fatal(
			"expected revision store not to be called",
		)
	}
}

func TestCreateRevisionPreservesVersioningStoreErrors(
	t *testing.T,
) {
	tests := []struct {
		name string
		err  error
	}{
		{
			name: "branch not found",
			err:  ErrBranchNotFound,
		},
		{
			name: "branch head conflict",
			err:  ErrBranchHeadConflict,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := &fakeProjectReader{}
			revisions := &fakeRevisionStore{
				err: test.err,
			}

			service := NewService(projects, revisions)

			_, err := service.CreateRevision(
				context.Background(),
				validCreateRevisionInput(),
			)
			if !errors.Is(err, test.err) {
				t.Fatalf(
					"expected %v, got %v",
					test.err,
					err,
				)
			}
		})
	}
}

func TestCreateRevisionWrapsRevisionStoreError(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	projects := &fakeProjectReader{}
	revisions := &fakeRevisionStore{
		err: databaseErr,
	}

	service := NewService(projects, revisions)

	_, err := service.CreateRevision(
		context.Background(),
		validCreateRevisionInput(),
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped database error, got %v",
			err,
		)
	}
}

func validCreateRevisionInput() CreateRevisionInput {
	return CreateRevisionInput{
		OwnerUserID: uuid.New(),
		ProjectID:   uuid.New(),
		BranchID:    uuid.New(),
		Message:     "Revision",
	}
}
