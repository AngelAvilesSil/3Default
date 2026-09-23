package versioning

import (
	"context"
	"errors"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestListBranchesReturnsBranchesForOwnedProject(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()

	expected := []dbgen.ProjectBranch{
		{
			ID:        uuid.New(),
			ProjectID: projectID,
			Name:      "feature",
		},
		{
			ID:        uuid.New(),
			ProjectID: projectID,
			Name:      "main",
		},
	}

	projects := &fakeProjectReader{}
	revisions := &fakeRevisionStore{
		branches: expected,
	}

	service := NewService(projects, revisions)

	branches, err := service.ListBranches(
		context.Background(),
		ownerID,
		projectID,
	)
	if err != nil {
		t.Fatalf("list branches: %v", err)
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

	if !revisions.listCalled {
		t.Fatal("expected branch listing store call")
	}

	if revisions.listProjectID != projectID {
		t.Fatalf(
			"expected branch-list project ID %s, got %s",
			projectID,
			revisions.listProjectID,
		)
	}

	if len(branches) != len(expected) {
		t.Fatalf(
			"expected %d branches, got %d",
			len(expected),
			len(branches),
		)
	}

	for index := range expected {
		if branches[index].ID != expected[index].ID {
			t.Fatalf(
				"expected branch %d ID %s, got %s",
				index,
				expected[index].ID,
				branches[index].ID,
			)
		}
	}
}

func TestListBranchesReturnsEmptySliceForNilStoreResult(
	t *testing.T,
) {
	service := NewService(
		&fakeProjectReader{},
		&fakeRevisionStore{},
	)

	branches, err := service.ListBranches(
		context.Background(),
		uuid.New(),
		uuid.New(),
	)
	if err != nil {
		t.Fatalf("list branches: %v", err)
	}

	if branches == nil {
		t.Fatal("expected empty branch slice, got nil")
	}

	if len(branches) != 0 {
		t.Fatalf(
			"expected 0 branches, got %d",
			len(branches),
		)
	}
}

func TestListBranchesRejectsInvalidInput(t *testing.T) {
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
			revisions := &fakeRevisionStore{}

			service := NewService(projects, revisions)

			_, err := service.ListBranches(
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

			if revisions.listCalled {
				t.Fatal(
					"expected branch store not to be called",
				)
			}
		})
	}
}

func TestListBranchesMapsMissingProjectToNotFound(t *testing.T) {
	projects := &fakeProjectReader{
		err: pgx.ErrNoRows,
	}
	revisions := &fakeRevisionStore{}

	service := NewService(projects, revisions)

	_, err := service.ListBranches(
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

	if revisions.listCalled {
		t.Fatal(
			"expected branch store not to be called",
		)
	}
}

func TestListBranchesWrapsProjectLookupError(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	service := NewService(
		&fakeProjectReader{
			err: databaseErr,
		},
		&fakeRevisionStore{},
	)

	_, err := service.ListBranches(
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

func TestListBranchesWrapsStoreError(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	service := NewService(
		&fakeProjectReader{},
		&fakeRevisionStore{
			listErr: databaseErr,
		},
	)

	_, err := service.ListBranches(
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

func TestGetRevisionReturnsRevisionForOwnedProject(t *testing.T) {
	ownerID := uuid.New()
	projectID := uuid.New()
	revisionID := uuid.New()

	projects := &fakeProjectReader{}
	revisions := &fakeRevisionStore{
		getRevision: dbgen.ProjectRevision{
			ID:           revisionID,
			ProjectID:    projectID,
			AuthorUserID: ownerID,
			Message:      "Initial revision",
		},
	}

	service := NewService(projects, revisions)

	revision, err := service.GetRevision(
		context.Background(),
		ownerID,
		projectID,
		revisionID,
	)
	if err != nil {
		t.Fatalf("get revision: %v", err)
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

	if !revisions.getCalled {
		t.Fatal("expected revision store lookup")
	}

	if revisions.getParams.ProjectID != projectID {
		t.Fatalf(
			"expected revision project ID %s, got %s",
			projectID,
			revisions.getParams.ProjectID,
		)
	}

	if revisions.getParams.RevisionID != revisionID {
		t.Fatalf(
			"expected revision ID %s, got %s",
			revisionID,
			revisions.getParams.RevisionID,
		)
	}

	if revision.ID != revisionID {
		t.Fatalf(
			"expected returned revision ID %s, got %s",
			revisionID,
			revision.ID,
		)
	}
}

func TestGetRevisionRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		ownerID    uuid.UUID
		projectID  uuid.UUID
		revisionID uuid.UUID
		want       error
	}{
		{
			name:       "missing owner",
			projectID:  uuid.New(),
			revisionID: uuid.New(),
			want:       ErrOwnerRequired,
		},
		{
			name:       "missing project ID",
			ownerID:    uuid.New(),
			revisionID: uuid.New(),
			want:       ErrProjectIDRequired,
		},
		{
			name:      "missing revision ID",
			ownerID:   uuid.New(),
			projectID: uuid.New(),
			want:      ErrRevisionIDRequired,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projects := &fakeProjectReader{}
			revisions := &fakeRevisionStore{}

			service := NewService(projects, revisions)

			_, err := service.GetRevision(
				context.Background(),
				test.ownerID,
				test.projectID,
				test.revisionID,
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
					"expected revision store not to be called",
				)
			}
		})
	}
}

func TestGetRevisionMapsMissingProjectToNotFound(t *testing.T) {
	projects := &fakeProjectReader{
		err: pgx.ErrNoRows,
	}
	revisions := &fakeRevisionStore{}

	service := NewService(projects, revisions)

	_, err := service.GetRevision(
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

	if revisions.getCalled {
		t.Fatal(
			"expected revision store not to be called",
		)
	}
}

func TestGetRevisionMapsMissingRevisionToNotFound(t *testing.T) {
	service := NewService(
		&fakeProjectReader{},
		&fakeRevisionStore{
			getErr: pgx.ErrNoRows,
		},
	)

	_, err := service.GetRevision(
		context.Background(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	)
	if !errors.Is(err, ErrRevisionNotFound) {
		t.Fatalf(
			"expected ErrRevisionNotFound, got %v",
			err,
		)
	}
}

func TestGetRevisionWrapsProjectLookupError(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	service := NewService(
		&fakeProjectReader{
			err: databaseErr,
		},
		&fakeRevisionStore{},
	)

	_, err := service.GetRevision(
		context.Background(),
		uuid.New(),
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

func TestGetRevisionWrapsStoreError(t *testing.T) {
	databaseErr := errors.New("database unavailable")

	service := NewService(
		&fakeProjectReader{},
		&fakeRevisionStore{
			getErr: databaseErr,
		},
	)

	_, err := service.GetRevision(
		context.Background(),
		uuid.New(),
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
