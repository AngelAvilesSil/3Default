package projects

import (
	"context"
	"errors"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeProjectCreator struct {
	called        bool
	params        dbgen.CreateProjectParams
	project       dbgen.Project
	err           error
	listCalled    bool
	ownerUserID   uuid.UUID
	projects      []dbgen.Project
	listErr       error
	getCalled     bool
	getParams     dbgen.GetProjectByIDAndOwnerParams
	getProject    dbgen.Project
	getErr        error
	updateCalled  bool
	updateParams  dbgen.UpdateProjectMetadataByIDAndOwnerParams
	updateProject dbgen.Project
	updateErr     error
}

func (f *fakeProjectCreator) CreateProject(
	_ context.Context,
	arg dbgen.CreateProjectParams,
) (dbgen.Project, error) {
	f.called = true
	f.params = arg

	return f.project, f.err
}

func (f *fakeProjectCreator) ListProjectsByOwner(
	_ context.Context,
	ownerUserID uuid.UUID,
) ([]dbgen.Project, error) {
	f.listCalled = true
	f.ownerUserID = ownerUserID

	return f.projects, f.listErr
}

func (f *fakeProjectCreator) GetProjectByIDAndOwner(
	_ context.Context,
	arg dbgen.GetProjectByIDAndOwnerParams,
) (dbgen.Project, error) {
	f.getCalled = true
	f.getParams = arg

	return f.getProject, f.getErr
}

func (f *fakeProjectCreator) UpdateProjectMetadataByIDAndOwner(
	_ context.Context,
	arg dbgen.UpdateProjectMetadataByIDAndOwnerParams,
) (dbgen.Project, error) {
	f.updateCalled = true
	f.updateParams = arg

	return f.updateProject, f.updateErr
}

func TestCreateNormalizesInputAndCreatesProject(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	description := "  Main CAD assembly  "

	store := &fakeProjectCreator{
		project: dbgen.Project{
			ID:          projectID,
			OwnerUserID: ownerID,
			Name:        "My Project",
			Description: stringPointer("Main CAD assembly"),
			Visibility:  "private",
		},
	}

	service := NewService(store)

	project, err := service.Create(context.Background(), CreateInput{
		OwnerUserID: ownerID,
		Name:        "  My Project  ",
		Description: &description,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if !store.called {
		t.Fatal("expected CreateProject to be called")
	}

	if store.params.OwnerUserID != ownerID {
		t.Fatalf(
			"expected owner ID %s, got %s",
			ownerID,
			store.params.OwnerUserID,
		)
	}

	if store.params.Name != "My Project" {
		t.Fatalf(
			"expected normalized name %q, got %q",
			"My Project",
			store.params.Name,
		)
	}

	if store.params.Description == nil {
		t.Fatal("expected description, got nil")
	}

	if *store.params.Description != "Main CAD assembly" {
		t.Fatalf(
			"expected normalized description %q, got %q",
			"Main CAD assembly",
			*store.params.Description,
		)
	}

	if project.ID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			project.ID,
		)
	}
}

func TestCreateConvertsBlankDescriptionToNil(t *testing.T) {
	description := "   "

	store := &fakeProjectCreator{}

	service := NewService(store)

	_, err := service.Create(context.Background(), CreateInput{
		OwnerUserID: uuid.New(),
		Name:        "My Project",
		Description: &description,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if store.params.Description != nil {
		t.Fatalf(
			"expected nil description, got %q",
			*store.params.Description,
		)
	}
}

func TestCreateRejectsMissingOwner(t *testing.T) {
	store := &fakeProjectCreator{}

	service := NewService(store)

	_, err := service.Create(context.Background(), CreateInput{
		Name: "My Project",
	})
	if !errors.Is(err, ErrOwnerRequired) {
		t.Fatalf(
			"expected ErrOwnerRequired, got %v",
			err,
		)
	}

	if store.called {
		t.Fatal("expected CreateProject not to be called")
	}
}

func TestCreateRejectsBlankName(t *testing.T) {
	store := &fakeProjectCreator{}

	service := NewService(store)

	_, err := service.Create(context.Background(), CreateInput{
		OwnerUserID: uuid.New(),
		Name:        "   ",
	})
	if !errors.Is(err, ErrNameRequired) {
		t.Fatalf(
			"expected ErrNameRequired, got %v",
			err,
		)
	}

	if store.called {
		t.Fatal("expected CreateProject not to be called")
	}
}

func TestCreateWrapsDatabaseError(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	store := &fakeProjectCreator{
		err: databaseErr,
	}

	service := NewService(store)

	_, err := service.Create(context.Background(), CreateInput{
		OwnerUserID: uuid.New(),
		Name:        "My Project",
	})
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped database error, got %v",
			err,
		)
	}
}

func TestGetReturnsProjectForOwner(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()

	store := &fakeProjectCreator{
		getProject: dbgen.Project{
			ID:          projectID,
			OwnerUserID: ownerID,
			Name:        "Robot Gripper",
			Visibility:  "private",
		},
	}

	service := NewService(store)

	project, err := service.Get(
		context.Background(),
		ownerID,
		projectID,
	)
	if err != nil {
		t.Fatalf("get project: %v", err)
	}

	if !store.getCalled {
		t.Fatal("expected GetProjectByIDAndOwner to be called")
	}

	if store.getParams.OwnerUserID != ownerID {
		t.Fatalf(
			"expected owner ID %s, got %s",
			ownerID,
			store.getParams.OwnerUserID,
		)
	}

	if store.getParams.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			store.getParams.ProjectID,
		)
	}

	if project.ID != projectID {
		t.Fatalf(
			"expected returned project ID %s, got %s",
			projectID,
			project.ID,
		)
	}
}

func TestGetRejectsMissingOwner(t *testing.T) {
	store := &fakeProjectCreator{}

	service := NewService(store)

	_, err := service.Get(
		context.Background(),
		uuid.Nil,
		uuid.New(),
	)
	if !errors.Is(err, ErrOwnerRequired) {
		t.Fatalf(
			"expected ErrOwnerRequired, got %v",
			err,
		)
	}

	if store.getCalled {
		t.Fatal("expected GetProjectByIDAndOwner not to be called")
	}
}

func TestGetRejectsMissingProjectID(t *testing.T) {
	store := &fakeProjectCreator{}

	service := NewService(store)

	_, err := service.Get(
		context.Background(),
		uuid.New(),
		uuid.Nil,
	)
	if !errors.Is(err, ErrProjectIDRequired) {
		t.Fatalf(
			"expected ErrProjectIDRequired, got %v",
			err,
		)
	}

	if store.getCalled {
		t.Fatal("expected GetProjectByIDAndOwner not to be called")
	}
}

func TestGetMapsMissingProjectToNotFound(t *testing.T) {
	store := &fakeProjectCreator{
		getErr: pgx.ErrNoRows,
	}

	service := NewService(store)

	_, err := service.Get(
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
}

func TestGetWrapsDatabaseError(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	store := &fakeProjectCreator{
		getErr: databaseErr,
	}

	service := NewService(store)

	_, err := service.Get(
		context.Background(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped database error, got %v",
			err,
		)
	}
}

func TestListReturnsProjectsForOwner(t *testing.T) {
	ownerID := uuid.New()
	firstProjectID := uuid.New()
	secondProjectID := uuid.New()

	store := &fakeProjectCreator{
		projects: []dbgen.Project{
			{
				ID:          firstProjectID,
				OwnerUserID: ownerID,
				Name:        "First Project",
			},
			{
				ID:          secondProjectID,
				OwnerUserID: ownerID,
				Name:        "Second Project",
			},
		},
	}

	service := NewService(store)

	projects, err := service.List(
		context.Background(),
		ownerID,
	)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}

	if !store.listCalled {
		t.Fatal("expected ListProjectsByOwner to be called")
	}

	if store.ownerUserID != ownerID {
		t.Fatalf(
			"expected owner ID %s, got %s",
			ownerID,
			store.ownerUserID,
		)
	}

	if len(projects) != 2 {
		t.Fatalf(
			"expected 2 projects, got %d",
			len(projects),
		)
	}

	if projects[0].ID != firstProjectID {
		t.Fatalf(
			"expected first project ID %s, got %s",
			firstProjectID,
			projects[0].ID,
		)
	}

	if projects[1].ID != secondProjectID {
		t.Fatalf(
			"expected second project ID %s, got %s",
			secondProjectID,
			projects[1].ID,
		)
	}
}

func TestListReturnsEmptySliceWhenStoreReturnsNil(t *testing.T) {
	store := &fakeProjectCreator{}

	service := NewService(store)

	projects, err := service.List(
		context.Background(),
		uuid.New(),
	)
	if err != nil {
		t.Fatalf("list projects: %v", err)
	}

	if projects == nil {
		t.Fatal("expected non-nil empty project slice")
	}

	if len(projects) != 0 {
		t.Fatalf(
			"expected no projects, got %d",
			len(projects),
		)
	}
}

func TestListRejectsMissingOwner(t *testing.T) {
	store := &fakeProjectCreator{}

	service := NewService(store)

	_, err := service.List(
		context.Background(),
		uuid.Nil,
	)
	if !errors.Is(err, ErrOwnerRequired) {
		t.Fatalf(
			"expected ErrOwnerRequired, got %v",
			err,
		)
	}

	if store.listCalled {
		t.Fatal(
			"expected ListProjectsByOwner not to be called",
		)
	}
}

func TestListWrapsDatabaseError(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	store := &fakeProjectCreator{
		listErr: databaseErr,
	}

	service := NewService(store)

	_, err := service.List(
		context.Background(),
		uuid.New(),
	)
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped database error, got %v",
			err,
		)
	}
}

func stringPointer(value string) *string {
	return &value
}

func TestUpdateNormalizesInputAndUpdatesProject(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	name := "  Robot Arm  "
	description := "  Main CAD assembly  "

	store := &fakeProjectCreator{
		updateProject: dbgen.Project{
			ID:          projectID,
			OwnerUserID: ownerID,
			Name:        "Robot Arm",
			Description: stringPointer("Main CAD assembly"),
			Visibility:  "private",
		},
	}

	service := NewService(store)

	project, err := service.Update(context.Background(), UpdateInput{
		OwnerUserID:    ownerID,
		ProjectID:      projectID,
		NameSet:        true,
		Name:           &name,
		DescriptionSet: true,
		Description:    &description,
	})
	if err != nil {
		t.Fatalf("update project: %v", err)
	}

	if !store.updateCalled {
		t.Fatal("expected UpdateProjectMetadataByIDAndOwner to be called")
	}

	if store.updateParams.OwnerUserID != ownerID {
		t.Fatalf(
			"expected owner ID %s, got %s",
			ownerID,
			store.updateParams.OwnerUserID,
		)
	}

	if store.updateParams.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			store.updateParams.ProjectID,
		)
	}

	if !store.updateParams.NameSet {
		t.Fatal("expected NameSet to be true")
	}

	if store.updateParams.Name != "Robot Arm" {
		t.Fatalf(
			"expected normalized name %q, got %q",
			"Robot Arm",
			store.updateParams.Name,
		)
	}

	if !store.updateParams.DescriptionSet {
		t.Fatal("expected DescriptionSet to be true")
	}

	if store.updateParams.Description == nil {
		t.Fatal("expected description, got nil")
	}

	if *store.updateParams.Description != "Main CAD assembly" {
		t.Fatalf(
			"expected normalized description %q, got %q",
			"Main CAD assembly",
			*store.updateParams.Description,
		)
	}

	if project.ID != projectID {
		t.Fatalf(
			"expected returned project ID %s, got %s",
			projectID,
			project.ID,
		)
	}
}

func TestUpdateClearsDescription(t *testing.T) {
	ignoredName := "ignored name"

	store := &fakeProjectCreator{}
	service := NewService(store)

	_, err := service.Update(context.Background(), UpdateInput{
		OwnerUserID:    uuid.New(),
		ProjectID:      uuid.New(),
		NameSet:        false,
		Name:           &ignoredName,
		DescriptionSet: true,
		Description:    nil,
	})
	if err != nil {
		t.Fatalf("update project: %v", err)
	}

	if !store.updateCalled {
		t.Fatal("expected UpdateProjectMetadataByIDAndOwner to be called")
	}

	if store.updateParams.NameSet {
		t.Fatal("expected NameSet to be false")
	}

	if store.updateParams.Name != "" {
		t.Fatalf(
			"expected omitted name value to be empty, got %q",
			store.updateParams.Name,
		)
	}

	if !store.updateParams.DescriptionSet {
		t.Fatal("expected DescriptionSet to be true")
	}

	if store.updateParams.Description != nil {
		t.Fatalf(
			"expected nil description, got %q",
			*store.updateParams.Description,
		)
	}
}

func TestUpdateConvertsBlankDescriptionToNil(t *testing.T) {
	description := "   "

	store := &fakeProjectCreator{}
	service := NewService(store)

	_, err := service.Update(context.Background(), UpdateInput{
		OwnerUserID:    uuid.New(),
		ProjectID:      uuid.New(),
		DescriptionSet: true,
		Description:    &description,
	})
	if err != nil {
		t.Fatalf("update project: %v", err)
	}

	if store.updateParams.Description != nil {
		t.Fatalf(
			"expected nil description, got %q",
			*store.updateParams.Description,
		)
	}
}

func TestUpdateRejectsMissingOwner(t *testing.T) {
	name := "Robot Arm"
	store := &fakeProjectCreator{}
	service := NewService(store)

	_, err := service.Update(context.Background(), UpdateInput{
		ProjectID: uuid.New(),
		NameSet:   true,
		Name:      &name,
	})
	if !errors.Is(err, ErrOwnerRequired) {
		t.Fatalf(
			"expected ErrOwnerRequired, got %v",
			err,
		)
	}

	if store.updateCalled {
		t.Fatal("expected update store not to be called")
	}
}

func TestUpdateRejectsMissingProjectID(t *testing.T) {
	name := "Robot Arm"
	store := &fakeProjectCreator{}
	service := NewService(store)

	_, err := service.Update(context.Background(), UpdateInput{
		OwnerUserID: uuid.New(),
		NameSet:     true,
		Name:        &name,
	})
	if !errors.Is(err, ErrProjectIDRequired) {
		t.Fatalf(
			"expected ErrProjectIDRequired, got %v",
			err,
		)
	}

	if store.updateCalled {
		t.Fatal("expected update store not to be called")
	}
}

func TestUpdateRejectsNoMetadataChanges(t *testing.T) {
	store := &fakeProjectCreator{}
	service := NewService(store)

	_, err := service.Update(context.Background(), UpdateInput{
		OwnerUserID: uuid.New(),
		ProjectID:   uuid.New(),
	})
	if !errors.Is(err, ErrNoMetadataChanges) {
		t.Fatalf(
			"expected ErrNoMetadataChanges, got %v",
			err,
		)
	}

	if store.updateCalled {
		t.Fatal("expected update store not to be called")
	}
}

func TestUpdateRejectsNullName(t *testing.T) {
	store := &fakeProjectCreator{}
	service := NewService(store)

	_, err := service.Update(context.Background(), UpdateInput{
		OwnerUserID: uuid.New(),
		ProjectID:   uuid.New(),
		NameSet:     true,
		Name:        nil,
	})
	if !errors.Is(err, ErrNameRequired) {
		t.Fatalf(
			"expected ErrNameRequired, got %v",
			err,
		)
	}

	if store.updateCalled {
		t.Fatal("expected update store not to be called")
	}
}

func TestUpdateRejectsBlankName(t *testing.T) {
	name := "   "
	store := &fakeProjectCreator{}
	service := NewService(store)

	_, err := service.Update(context.Background(), UpdateInput{
		OwnerUserID: uuid.New(),
		ProjectID:   uuid.New(),
		NameSet:     true,
		Name:        &name,
	})
	if !errors.Is(err, ErrNameRequired) {
		t.Fatalf(
			"expected ErrNameRequired, got %v",
			err,
		)
	}

	if store.updateCalled {
		t.Fatal("expected update store not to be called")
	}
}

func TestUpdateMapsMissingProjectToNotFound(t *testing.T) {
	name := "Robot Arm"

	store := &fakeProjectCreator{
		updateErr: pgx.ErrNoRows,
	}

	service := NewService(store)

	_, err := service.Update(context.Background(), UpdateInput{
		OwnerUserID: uuid.New(),
		ProjectID:   uuid.New(),
		NameSet:     true,
		Name:        &name,
	})
	if !errors.Is(err, ErrProjectNotFound) {
		t.Fatalf(
			"expected ErrProjectNotFound, got %v",
			err,
		)
	}
}

func TestUpdateWrapsDatabaseError(t *testing.T) {
	databaseErr := errors.New("database unavailable")
	name := "Robot Arm"

	store := &fakeProjectCreator{
		updateErr: databaseErr,
	}

	service := NewService(store)

	_, err := service.Update(context.Background(), UpdateInput{
		OwnerUserID: uuid.New(),
		ProjectID:   uuid.New(),
		NameSet:     true,
		Name:        &name,
	})
	if !errors.Is(err, databaseErr) {
		t.Fatalf(
			"expected wrapped database error, got %v",
			err,
		)
	}

	if err.Error() != "update project: database unavailable" {
		t.Fatalf(
			"expected wrapped update error, got %q",
			err.Error(),
		)
	}
}
