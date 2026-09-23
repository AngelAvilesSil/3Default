package httpapi

import (
	"context"
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
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeVersioningService struct {
	listCalled      bool
	listOwnerUserID uuid.UUID
	listProjectID   uuid.UUID
	branches        []dbgen.ProjectBranch
	listErr         error

	getCalled      bool
	getOwnerUserID uuid.UUID
	getProjectID   uuid.UUID
	getRevisionID  uuid.UUID
	revision       dbgen.ProjectRevision
	getErr         error
}

func (f *fakeVersioningService) ListBranches(
	_ context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
) ([]dbgen.ProjectBranch, error) {
	f.listCalled = true
	f.listOwnerUserID = ownerUserID
	f.listProjectID = projectID

	return f.branches, f.listErr
}

func (f *fakeVersioningService) GetRevision(
	_ context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	revisionID uuid.UUID,
) (dbgen.ProjectRevision, error) {
	f.getCalled = true
	f.getOwnerUserID = ownerUserID
	f.getProjectID = projectID
	f.getRevisionID = revisionID

	return f.revision, f.getErr
}

func newVersioningHandler(
	service VersioningService,
) http.Handler {
	return NewHandler(
		NewServerWithVersioning(
			fakeDatabase{},
			nil,
			service,
			nil,
			nil,
			nil,
			nil,
		),
		nil,
	)
}

func TestListProjectBranchesRequiresAuthentication(t *testing.T) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+uuid.New().String()+"/branches",
		nil,
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			response.Code,
		)
	}

	if service.listCalled {
		t.Fatal("expected versioning service not to be called")
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "authentication required" {
		t.Fatalf(
			"expected authentication error, got %q",
			body.Error,
		)
	}
}

func TestListProjectBranchesRejectsSessionResolutionFailure(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+uuid.New().String()+"/branches",
		nil,
	)
	request = requestWithSessionResolutionError(
		request,
		errors.New("database unavailable"),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			response.Code,
		)
	}

	if service.listCalled {
		t.Fatal("expected versioning service not to be called")
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "unable to authenticate request" {
		t.Fatalf(
			"expected authentication failure error, got %q",
			body.Error,
		)
	}
}

func TestListProjectBranchesRejectsMalformedProjectID(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/not-a-uuid/branches",
		nil,
	)
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}

	if service.listCalled {
		t.Fatal("expected versioning service not to be called")
	}
}

func TestListProjectBranchesMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name        string
		projectID   uuid.UUID
		serviceErr  error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing project ID",
			projectID:   uuid.Nil,
			serviceErr:  versioning.ErrProjectIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project ID is required",
		},
		{
			name:        "project not found",
			projectID:   uuid.New(),
			serviceErr:  versioning.ErrProjectNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project not found",
		},
		{
			name:        "unexpected service error",
			projectID:   uuid.New(),
			serviceErr:  errors.New("database unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to list project branches",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userID := uuid.New()
			service := &fakeVersioningService{
				listErr: test.serviceErr,
			}
			handler := newVersioningHandler(service)

			request := httptest.NewRequest(
				http.MethodGet,
				"/api/projects/"+
					test.projectID.String()+
					"/branches",
				nil,
			)
			request = requestWithSession(request, userID)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf(
					"expected status %d, got %d",
					test.wantStatus,
					response.Code,
				)
			}

			if !service.listCalled {
				t.Fatal("expected versioning service to be called")
			}

			if service.listOwnerUserID != userID {
				t.Fatalf(
					"expected owner %s, got %s",
					userID,
					service.listOwnerUserID,
				)
			}

			if service.listProjectID != test.projectID {
				t.Fatalf(
					"expected project ID %s, got %s",
					test.projectID,
					service.listProjectID,
				)
			}

			body := decodeErrorResponse(t, response)
			if body.Error != test.wantMessage {
				t.Fatalf(
					"expected error %q, got %q",
					test.wantMessage,
					body.Error,
				)
			}
		})
	}
}

func TestListProjectBranchesRejectsMissingVersioningDependency(
	t *testing.T,
) {
	projectID := uuid.New()

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			nil,
			nil,
			nil,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+projectID.String()+"/branches",
		nil,
	)
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			response.Code,
		)
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "unable to list project branches" {
		t.Fatalf(
			"expected missing dependency error, got %q",
			body.Error,
		)
	}
}

func TestListProjectBranchesReturnsBranchesForAuthenticatedUser(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	headRevisionID := uuid.New()

	firstCreatedAt := time.Date(
		2026,
		time.September,
		22,
		14,
		0,
		0,
		0,
		time.UTC,
	)
	secondCreatedAt := firstCreatedAt.Add(time.Minute)

	service := &fakeVersioningService{
		branches: []dbgen.ProjectBranch{
			{
				ID:        uuid.New(),
				ProjectID: projectID,
				Name:      "alpha",
				CreatedAt: firstCreatedAt,
				UpdatedAt: firstCreatedAt,
			},
			{
				ID:        uuid.New(),
				ProjectID: projectID,
				Name:      "main",
				HeadRevisionID: pgtype.UUID{
					Bytes: headRevisionID,
					Valid: true,
				},
				CreatedAt: secondCreatedAt,
				UpdatedAt: secondCreatedAt.Add(time.Minute),
			},
		},
	}

	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+projectID.String()+"/branches",
		nil,
	)
	request = requestWithSession(request, userID)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}

	if !service.listCalled {
		t.Fatal("expected versioning service to be called")
	}

	if service.listOwnerUserID != userID {
		t.Fatalf(
			"expected owner %s, got %s",
			userID,
			service.listOwnerUserID,
		)
	}

	if service.listProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			service.listProjectID,
		)
	}

	var body []api.ProjectBranchResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode branch response: %v", err)
	}

	if len(body) != 2 {
		t.Fatalf("expected 2 branches, got %d", len(body))
	}

	if uuid.UUID(body[0].Id) != service.branches[0].ID {
		t.Fatalf(
			"expected first branch ID %s, got %s",
			service.branches[0].ID,
			body[0].Id,
		)
	}

	if uuid.UUID(body[0].ProjectId) != projectID {
		t.Fatalf(
			"expected first project ID %s, got %s",
			projectID,
			body[0].ProjectId,
		)
	}

	if body[0].Name != "alpha" {
		t.Fatalf(
			"expected first branch name %q, got %q",
			"alpha",
			body[0].Name,
		)
	}

	if body[0].HeadRevisionId != nil {
		t.Fatalf(
			"expected first branch head to be null, got %s",
			*body[0].HeadRevisionId,
		)
	}

	if !body[0].CreatedAt.Equal(firstCreatedAt) {
		t.Fatalf(
			"expected first created time %s, got %s",
			firstCreatedAt,
			body[0].CreatedAt,
		)
	}

	if body[1].HeadRevisionId == nil {
		t.Fatal("expected main branch head revision")
	}

	if uuid.UUID(*body[1].HeadRevisionId) != headRevisionID {
		t.Fatalf(
			"expected head revision ID %s, got %s",
			headRevisionID,
			*body[1].HeadRevisionId,
		)
	}

	if !body[1].UpdatedAt.Equal(
		secondCreatedAt.Add(time.Minute),
	) {
		t.Fatalf(
			"expected second updated time %s, got %s",
			secondCreatedAt.Add(time.Minute),
			body[1].UpdatedAt,
		)
	}
}

func TestListProjectBranchesReturnsEmptyArray(t *testing.T) {
	projectID := uuid.New()

	handler := newVersioningHandler(
		&fakeVersioningService{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+projectID.String()+"/branches",
		nil,
	)
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}

	var body []api.ProjectBranchResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode branch response: %v", err)
	}

	if body == nil {
		t.Fatal("expected empty array, got null")
	}

	if len(body) != 0 {
		t.Fatalf("expected 0 branches, got %d", len(body))
	}
}

func TestGetProjectRevisionRequiresAuthentication(t *testing.T) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			uuid.New().String()+
			"/revisions/"+
			uuid.New().String(),
		nil,
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			response.Code,
		)
	}

	if service.getCalled {
		t.Fatal("expected versioning service not to be called")
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "authentication required" {
		t.Fatalf(
			"expected authentication error, got %q",
			body.Error,
		)
	}
}

func TestGetProjectRevisionRejectsSessionResolutionFailure(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			uuid.New().String()+
			"/revisions/"+
			uuid.New().String(),
		nil,
	)
	request = requestWithSessionResolutionError(
		request,
		errors.New("database unavailable"),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			response.Code,
		)
	}

	if service.getCalled {
		t.Fatal("expected versioning service not to be called")
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "unable to authenticate request" {
		t.Fatalf(
			"expected authentication failure error, got %q",
			body.Error,
		)
	}
}

func TestGetProjectRevisionRejectsMalformedIDs(t *testing.T) {
	tests := []struct {
		name       string
		projectID  string
		revisionID string
	}{
		{
			name:       "malformed project ID",
			projectID:  "not-a-uuid",
			revisionID: uuid.New().String(),
		},
		{
			name:       "malformed revision ID",
			projectID:  uuid.New().String(),
			revisionID: "not-a-uuid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeVersioningService{}
			handler := newVersioningHandler(service)

			request := httptest.NewRequest(
				http.MethodGet,
				"/api/projects/"+
					test.projectID+
					"/revisions/"+
					test.revisionID,
				nil,
			)
			request = requestWithSession(request, uuid.New())

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected status %d, got %d",
					http.StatusBadRequest,
					response.Code,
				)
			}

			if service.getCalled {
				t.Fatal(
					"expected versioning service not to be called",
				)
			}
		})
	}
}

func TestGetProjectRevisionMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name        string
		projectID   uuid.UUID
		revisionID  uuid.UUID
		serviceErr  error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing project ID",
			projectID:   uuid.Nil,
			revisionID:  uuid.New(),
			serviceErr:  versioning.ErrProjectIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project ID is required",
		},
		{
			name:        "missing revision ID",
			projectID:   uuid.New(),
			revisionID:  uuid.Nil,
			serviceErr:  versioning.ErrRevisionIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "revision ID is required",
		},
		{
			name:        "project not found",
			projectID:   uuid.New(),
			revisionID:  uuid.New(),
			serviceErr:  versioning.ErrProjectNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project not found",
		},
		{
			name:        "revision not found",
			projectID:   uuid.New(),
			revisionID:  uuid.New(),
			serviceErr:  versioning.ErrRevisionNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "revision not found",
		},
		{
			name:        "unexpected service error",
			projectID:   uuid.New(),
			revisionID:  uuid.New(),
			serviceErr:  errors.New("database unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to get project revision",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userID := uuid.New()
			service := &fakeVersioningService{
				getErr: test.serviceErr,
			}
			handler := newVersioningHandler(service)

			request := httptest.NewRequest(
				http.MethodGet,
				"/api/projects/"+
					test.projectID.String()+
					"/revisions/"+
					test.revisionID.String(),
				nil,
			)
			request = requestWithSession(request, userID)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf(
					"expected status %d, got %d",
					test.wantStatus,
					response.Code,
				)
			}

			if !service.getCalled {
				t.Fatal("expected versioning service to be called")
			}

			if service.getOwnerUserID != userID {
				t.Fatalf(
					"expected owner %s, got %s",
					userID,
					service.getOwnerUserID,
				)
			}

			if service.getProjectID != test.projectID {
				t.Fatalf(
					"expected project ID %s, got %s",
					test.projectID,
					service.getProjectID,
				)
			}

			if service.getRevisionID != test.revisionID {
				t.Fatalf(
					"expected revision ID %s, got %s",
					test.revisionID,
					service.getRevisionID,
				)
			}

			body := decodeErrorResponse(t, response)
			if body.Error != test.wantMessage {
				t.Fatalf(
					"expected error %q, got %q",
					test.wantMessage,
					body.Error,
				)
			}
		})
	}
}

func TestGetProjectRevisionRejectsMissingVersioningDependency(
	t *testing.T,
) {
	projectID := uuid.New()
	revisionID := uuid.New()

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			nil,
			nil,
			nil,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			projectID.String()+
			"/revisions/"+
			revisionID.String(),
		nil,
	)
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			response.Code,
		)
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "unable to get project revision" {
		t.Fatalf(
			"expected missing dependency error, got %q",
			body.Error,
		)
	}
}

func TestGetProjectRevisionReturnsRevisionForAuthenticatedUser(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	revisionID := uuid.New()
	parentID := uuid.New()
	mergeParentID := uuid.New()

	createdAt := time.Date(
		2026,
		time.September,
		22,
		15,
		0,
		0,
		0,
		time.UTC,
	)

	service := &fakeVersioningService{
		revision: dbgen.ProjectRevision{
			ID:           revisionID,
			ProjectID:    projectID,
			AuthorUserID: userID,
			Message:      "Merge feature into main",
			ParentRevisionID: pgtype.UUID{
				Bytes: parentID,
				Valid: true,
			},
			MergeParentRevisionID: pgtype.UUID{
				Bytes: mergeParentID,
				Valid: true,
			},
			CreatedAt: createdAt,
		},
	}

	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			projectID.String()+
			"/revisions/"+
			revisionID.String(),
		nil,
	)
	request = requestWithSession(request, userID)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}

	if !service.getCalled {
		t.Fatal("expected versioning service to be called")
	}

	if service.getOwnerUserID != userID {
		t.Fatalf(
			"expected owner %s, got %s",
			userID,
			service.getOwnerUserID,
		)
	}

	if service.getProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			service.getProjectID,
		)
	}

	if service.getRevisionID != revisionID {
		t.Fatalf(
			"expected revision ID %s, got %s",
			revisionID,
			service.getRevisionID,
		)
	}

	var body api.ProjectRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode revision response: %v", err)
	}

	if uuid.UUID(body.Id) != revisionID {
		t.Fatalf(
			"expected revision ID %s, got %s",
			revisionID,
			body.Id,
		)
	}

	if uuid.UUID(body.ProjectId) != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			body.ProjectId,
		)
	}

	if uuid.UUID(body.AuthorUserId) != userID {
		t.Fatalf(
			"expected author ID %s, got %s",
			userID,
			body.AuthorUserId,
		)
	}

	if body.Message != "Merge feature into main" {
		t.Fatalf(
			"expected message %q, got %q",
			"Merge feature into main",
			body.Message,
		)
	}

	if body.ParentRevisionId == nil {
		t.Fatal("expected primary parent revision")
	}

	if uuid.UUID(*body.ParentRevisionId) != parentID {
		t.Fatalf(
			"expected parent ID %s, got %s",
			parentID,
			*body.ParentRevisionId,
		)
	}

	if body.MergeParentRevisionId == nil {
		t.Fatal("expected merge parent revision")
	}

	if uuid.UUID(*body.MergeParentRevisionId) != mergeParentID {
		t.Fatalf(
			"expected merge parent ID %s, got %s",
			mergeParentID,
			*body.MergeParentRevisionId,
		)
	}

	if !body.CreatedAt.Equal(createdAt) {
		t.Fatalf(
			"expected created time %s, got %s",
			createdAt,
			body.CreatedAt,
		)
	}
}

func TestGetProjectRevisionReturnsNullParentsForRootRevision(
	t *testing.T,
) {
	projectID := uuid.New()
	revisionID := uuid.New()

	service := &fakeVersioningService{
		revision: dbgen.ProjectRevision{
			ID:           revisionID,
			ProjectID:    projectID,
			AuthorUserID: uuid.New(),
			Message:      "Initial revision",
		},
	}

	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			projectID.String()+
			"/revisions/"+
			revisionID.String(),
		nil,
	)
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}

	var body api.ProjectRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode revision response: %v", err)
	}

	if body.ParentRevisionId != nil {
		t.Fatalf(
			"expected primary parent to be null, got %s",
			*body.ParentRevisionId,
		)
	}

	if body.MergeParentRevisionId != nil {
		t.Fatalf(
			"expected merge parent to be null, got %s",
			*body.MergeParentRevisionId,
		)
	}
}
