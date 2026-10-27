package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	api "github.com/AngelAvilesSil/3Default/internal/api"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/versioning"
	"github.com/google/uuid"
)

func authorRevisionHandler(
	service VersioningService,
	reader CurrentUserReader,
) http.Handler {
	return NewHandler(
		NewServerWithVersioning(
			fakeDatabase{},
			nil,
			service,
			nil,
			nil,
			nil,
			reader,
		),
		nil,
	)
}

func authorRevisionRequest(
	ownerID uuid.UUID,
	projectID uuid.UUID,
	revisionID uuid.UUID,
) *http.Request {
	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+projectID.String()+
			"/revisions/"+revisionID.String(),
		nil,
	)
	return requestWithSession(request, ownerID)
}

func authorRevisionFixture(
	projectID uuid.UUID,
	revisionID uuid.UUID,
	authorID uuid.UUID,
) *fakeVersioningService {
	return &fakeVersioningService{
		revision: dbgen.ProjectRevision{
			ID:           revisionID,
			ProjectID:    projectID,
			AuthorUserID: authorID,
			Message:      "Existing engineering revision",
			CreatedAt:    time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC),
		},
	}
}

func TestGetProjectRevisionResolvesRecordedAuthorNotSessionUser(t *testing.T) {
	ownerID := uuid.New()
	authorID := uuid.New()
	projectID := uuid.New()
	revisionID := uuid.New()

	service := authorRevisionFixture(projectID, revisionID, authorID)
	reader := &fakeCurrentUserReader{
		user: dbgen.User{
			ID:          authorID,
			DisplayName: "  Engineering Author  ",
		},
	}

	response := httptest.NewRecorder()
	authorRevisionHandler(service, reader).ServeHTTP(
		response,
		authorRevisionRequest(ownerID, projectID, revisionID),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body)
	}

	if !reader.called || reader.userID != authorID {
		t.Fatal("author lookup must use the recorded author ID")
	}
	if service.getOwnerUserID != ownerID {
		t.Fatal("revision authorization must use the session owner")
	}

	var body api.ProjectRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if body.AuthorDisplayName == nil ||
		*body.AuthorDisplayName != "Engineering Author" {
		t.Fatalf("unexpected author display name: %v", body.AuthorDisplayName)
	}

	if uuid.UUID(body.AuthorUserId) != authorID {
		t.Fatal("immutable recorded author ID was changed")
	}
}

func TestGetProjectRevisionAuthorLookupFailureKeepsRevisionReadable(t *testing.T) {
	ownerID := uuid.New()
	authorID := uuid.New()
	projectID := uuid.New()
	revisionID := uuid.New()

	service := authorRevisionFixture(projectID, revisionID, authorID)
	reader := &fakeCurrentUserReader{
		err: errors.New("author lookup unavailable"),
	}

	response := httptest.NewRecorder()
	authorRevisionHandler(service, reader).ServeHTTP(
		response,
		authorRevisionRequest(ownerID, projectID, revisionID),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body)
	}

	var body api.ProjectRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if body.AuthorDisplayName != nil {
		t.Fatal("unresolved author must not have an invented display name")
	}

	if uuid.UUID(body.AuthorUserId) != authorID {
		t.Fatal("recorded author ID must be preserved")
	}
}

func TestGetProjectRevisionRejectsMismatchedAuthorLookupIdentity(t *testing.T) {
	ownerID := uuid.New()
	authorID := uuid.New()
	projectID := uuid.New()
	revisionID := uuid.New()

	service := authorRevisionFixture(projectID, revisionID, authorID)
	reader := &fakeCurrentUserReader{
		user: dbgen.User{
			ID:          uuid.New(),
			DisplayName: "Unrelated Person",
		},
	}

	response := httptest.NewRecorder()
	authorRevisionHandler(service, reader).ServeHTTP(
		response,
		authorRevisionRequest(ownerID, projectID, revisionID),
	)

	if response.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", response.Code, response.Body)
	}

	var body api.ProjectRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}

	if body.AuthorDisplayName != nil {
		t.Fatal("an unrelated user's name must not be displayed")
	}
}

func TestGetProjectRevisionDoesNotLookupAuthorBeforeAuthorization(t *testing.T) {
	ownerID := uuid.New()
	authorID := uuid.New()
	projectID := uuid.New()
	revisionID := uuid.New()

	service := authorRevisionFixture(projectID, revisionID, authorID)
	service.getErr = versioning.ErrProjectNotFound

	reader := &fakeCurrentUserReader{
		user: dbgen.User{
			ID:          authorID,
			DisplayName: "Private Author",
		},
	}

	response := httptest.NewRecorder()
	authorRevisionHandler(service, reader).ServeHTTP(
		response,
		authorRevisionRequest(ownerID, projectID, revisionID),
	)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", response.Code, response.Body)
	}

	if reader.called {
		t.Fatal("author must not be looked up for inaccessible revisions")
	}
}
