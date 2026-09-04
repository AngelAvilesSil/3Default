package projects

import (
	"context"
	"errors"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
)

type fakeProjectCreator struct {
	called  bool
	params  dbgen.CreateProjectParams
	project dbgen.Project
	err     error
}

func (f *fakeProjectCreator) CreateProject(
	_ context.Context,
	arg dbgen.CreateProjectParams,
) (dbgen.Project, error) {
	f.called = true
	f.params = arg

	return f.project, f.err
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

func stringPointer(value string) *string {
	return &value
}
