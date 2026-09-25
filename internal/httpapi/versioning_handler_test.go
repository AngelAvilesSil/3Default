package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	api "github.com/AngelAvilesSil/3Default/internal/api"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/versioning"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeVersioningService struct {
	createCalled bool
	createInput  versioning.CreateRevisionInput
	createResult dbgen.ProjectRevision
	createErr    error

	createBranchCalled bool
	createBranchInput  versioning.CreateBranchInput
	createBranchResult dbgen.ProjectBranch
	createBranchErr    error

	listCalled      bool
	listOwnerUserID uuid.UUID
	listProjectID   uuid.UUID
	branches        []dbgen.ProjectBranch
	listErr         error

	historyCalled      bool
	historyOwnerUserID uuid.UUID
	historyProjectID   uuid.UUID
	historyBranchID    uuid.UUID
	history            []dbgen.ProjectRevision
	historyErr         error

	getCalled      bool
	getOwnerUserID uuid.UUID
	getProjectID   uuid.UUID
	getRevisionID  uuid.UUID
	revision       dbgen.ProjectRevision
	getErr         error
}

func (f *fakeVersioningService) CreateBranch(
	_ context.Context,
	input versioning.CreateBranchInput,
) (dbgen.ProjectBranch, error) {
	f.createBranchCalled = true
	f.createBranchInput = input

	return f.createBranchResult, f.createBranchErr
}

func (f *fakeVersioningService) CreateRevision(
	_ context.Context,
	input versioning.CreateRevisionInput,
) (dbgen.ProjectRevision, error) {
	f.createCalled = true
	f.createInput = input

	return f.createResult, f.createErr
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

func (f *fakeVersioningService) ListBranchHistory(
	_ context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	branchID uuid.UUID,
) ([]dbgen.ProjectRevision, error) {
	f.historyCalled = true
	f.historyOwnerUserID = ownerUserID
	f.historyProjectID = projectID
	f.historyBranchID = branchID

	return f.history, f.historyErr
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

func TestCreateProjectBranchRequiresAuthentication(t *testing.T) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+uuid.New().String()+"/branches",
		strings.NewReader(
			`{"name":"feature","headRevisionId":null}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			response.Code,
		)
	}

	if service.createBranchCalled {
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

func TestCreateProjectBranchRejectsSessionResolutionFailure(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+uuid.New().String()+"/branches",
		strings.NewReader(
			`{"name":"feature","headRevisionId":null}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
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

	if service.createBranchCalled {
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

func TestCreateProjectBranchRejectsCrossOriginBrowserRequest(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+uuid.New().String()+"/branches",
		strings.NewReader(
			`{"name":"feature","headRevisionId":null}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusForbidden,
			response.Code,
		)
	}

	if service.createBranchCalled {
		t.Fatal("expected versioning service not to be called")
	}
}

func TestCreateProjectBranchRejectsMalformedProjectID(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/not-a-uuid/branches",
		strings.NewReader(
			`{"name":"feature","headRevisionId":null}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
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

	if service.createBranchCalled {
		t.Fatal("expected versioning service not to be called")
	}
}

func TestCreateProjectBranchRejectsInvalidJSON(t *testing.T) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+uuid.New().String()+"/branches",
		strings.NewReader(`{"name":`),
	)
	request.Header.Set("Content-Type", "application/json")
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

	if service.createBranchCalled {
		t.Fatal("expected versioning service not to be called")
	}
}

func TestCreateProjectBranchRequiresHeadRevisionField(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+uuid.New().String()+"/branches",
		strings.NewReader(`{"name":"feature"}`),
	)
	request.Header.Set("Content-Type", "application/json")
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

	if service.createBranchCalled {
		t.Fatal("expected versioning service not to be called")
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "head revision ID is required" {
		t.Fatalf(
			"expected missing head-revision error, got %q",
			body.Error,
		)
	}
}

func TestCreateProjectBranchRejectsMissingVersioningDependency(
	t *testing.T,
) {
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
		http.MethodPost,
		"/api/projects/"+uuid.New().String()+"/branches",
		strings.NewReader(
			`{"name":"feature","headRevisionId":null}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
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
	if body.Error != "unable to create project branch" {
		t.Fatalf(
			"expected missing dependency error, got %q",
			body.Error,
		)
	}
}

func TestCreateProjectBranchMapsServiceErrors(t *testing.T) {
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
			name:        "branch name required",
			projectID:   uuid.New(),
			serviceErr:  versioning.ErrBranchNameRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "branch name is required",
		},
		{
			name:        "invalid head revision ID",
			projectID:   uuid.New(),
			serviceErr:  versioning.ErrHeadRevisionIDInvalid,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "branch head revision ID is invalid",
		},
		{
			name:        "project not found",
			projectID:   uuid.New(),
			serviceErr:  versioning.ErrProjectNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project not found",
		},
		{
			name:        "head revision not found",
			projectID:   uuid.New(),
			serviceErr:  versioning.ErrRevisionNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "head revision not found",
		},
		{
			name:        "branch name conflict",
			projectID:   uuid.New(),
			serviceErr:  versioning.ErrBranchNameConflict,
			wantStatus:  http.StatusConflict,
			wantMessage: "branch name already exists",
		},
		{
			name:        "unexpected service error",
			projectID:   uuid.New(),
			serviceErr:  errors.New("database unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to create project branch",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userID := uuid.New()
			service := &fakeVersioningService{
				createBranchErr: test.serviceErr,
			}
			handler := newVersioningHandler(service)

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/projects/"+
					test.projectID.String()+
					"/branches",
				strings.NewReader(
					`{"name":"feature","headRevisionId":null}`,
				),
			)
			request.Header.Set(
				"Content-Type",
				"application/json",
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

			if !service.createBranchCalled {
				t.Fatal(
					"expected versioning service to be called",
				)
			}

			if service.createBranchInput.OwnerUserID != userID {
				t.Fatalf(
					"expected owner ID %s, got %s",
					userID,
					service.createBranchInput.OwnerUserID,
				)
			}

			if service.createBranchInput.ProjectID != test.projectID {
				t.Fatalf(
					"expected project ID %s, got %s",
					test.projectID,
					service.createBranchInput.ProjectID,
				)
			}

			if service.createBranchInput.Name != "feature" {
				t.Fatalf(
					"expected branch name %q, got %q",
					"feature",
					service.createBranchInput.Name,
				)
			}

			if service.createBranchInput.HeadRevisionID != nil {
				t.Fatalf(
					"expected nil head revision, got %s",
					*service.createBranchInput.HeadRevisionID,
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

func TestCreateProjectBranchAcceptsExplicitNullHead(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	branchID := uuid.New()

	createdAt := time.Date(
		2026,
		time.September,
		24,
		20,
		0,
		0,
		0,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Minute)

	service := &fakeVersioningService{
		createBranchResult: dbgen.ProjectBranch{
			ID:        branchID,
			ProjectID: projectID,
			Name:      "empty-feature",
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}

	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+projectID.String()+"/branches",
		strings.NewReader(
			`{"name":"empty-feature","headRevisionId":null}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
	request = requestWithSession(request, userID)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.Code,
		)
	}

	if !service.createBranchCalled {
		t.Fatal("expected versioning service to be called")
	}

	if service.createBranchInput.OwnerUserID != userID {
		t.Fatalf(
			"expected owner ID %s, got %s",
			userID,
			service.createBranchInput.OwnerUserID,
		)
	}

	if service.createBranchInput.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			service.createBranchInput.ProjectID,
		)
	}

	if service.createBranchInput.Name != "empty-feature" {
		t.Fatalf(
			"expected branch name %q, got %q",
			"empty-feature",
			service.createBranchInput.Name,
		)
	}

	if service.createBranchInput.HeadRevisionID != nil {
		t.Fatalf(
			"expected nil branch head, got %s",
			*service.createBranchInput.HeadRevisionID,
		)
	}

	var body api.ProjectBranchResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode branch response: %v", err)
	}

	if uuid.UUID(body.Id) != branchID {
		t.Fatalf(
			"expected branch ID %s, got %s",
			branchID,
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

	if body.Name != "empty-feature" {
		t.Fatalf(
			"expected branch name %q, got %q",
			"empty-feature",
			body.Name,
		)
	}

	if body.HeadRevisionId != nil {
		t.Fatalf(
			"expected response head revision to be null, got %s",
			*body.HeadRevisionId,
		)
	}

	if !body.CreatedAt.Equal(createdAt) {
		t.Fatalf(
			"expected created time %s, got %s",
			createdAt,
			body.CreatedAt,
		)
	}

	if !body.UpdatedAt.Equal(updatedAt) {
		t.Fatalf(
			"expected updated time %s, got %s",
			updatedAt,
			body.UpdatedAt,
		)
	}
}

func TestCreateProjectBranchForwardsExactHeadRevision(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	branchID := uuid.New()
	headRevisionID := uuid.New()

	createdAt := time.Date(
		2026,
		time.September,
		24,
		21,
		0,
		0,
		0,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Minute)

	service := &fakeVersioningService{
		createBranchResult: dbgen.ProjectBranch{
			ID:        branchID,
			ProjectID: projectID,
			Name:      "feature-gripper",
			HeadRevisionID: pgtype.UUID{
				Bytes: headRevisionID,
				Valid: true,
			},
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		},
	}

	handler := newVersioningHandler(service)

	requestBody :=
		`{"name":"feature-gripper","headRevisionId":"` +
			headRevisionID.String() +
			`"}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+projectID.String()+"/branches",
		strings.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = requestWithSession(request, userID)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.Code,
		)
	}

	if !service.createBranchCalled {
		t.Fatal("expected versioning service to be called")
	}

	if service.createBranchInput.HeadRevisionID == nil {
		t.Fatal("expected branch head revision ID")
	}

	if *service.createBranchInput.HeadRevisionID != headRevisionID {
		t.Fatalf(
			"expected head revision ID %s, got %s",
			headRevisionID,
			*service.createBranchInput.HeadRevisionID,
		)
	}

	var body api.ProjectBranchResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode branch response: %v", err)
	}

	if body.HeadRevisionId == nil {
		t.Fatal("expected head revision in response")
	}

	if uuid.UUID(*body.HeadRevisionId) != headRevisionID {
		t.Fatalf(
			"expected response head ID %s, got %s",
			headRevisionID,
			*body.HeadRevisionId,
		)
	}
}

func TestCreateProjectBranchRejectsMalformedHeadRevisionUUID(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+uuid.New().String()+"/branches",
		strings.NewReader(
			`{"name":"feature","headRevisionId":"not-a-uuid"}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
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

	if service.createBranchCalled {
		t.Fatal("expected versioning service not to be called")
	}
}

func TestCreateProjectRevisionRequiresAuthentication(t *testing.T) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	projectID := uuid.New()
	branchID := uuid.New()

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+
			projectID.String()+
			"/branches/"+
			branchID.String()+
			"/revisions",
		strings.NewReader(
			`{"message":"Initial revision","expectedHeadRevisionId":null}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			response.Code,
		)
	}

	if service.createCalled {
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

func TestCreateProjectRevisionRejectsSessionResolutionFailure(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+
			uuid.New().String()+
			"/branches/"+
			uuid.New().String()+
			"/revisions",
		strings.NewReader(
			`{"message":"Initial revision","expectedHeadRevisionId":null}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
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

	if service.createCalled {
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

func TestCreateProjectRevisionRejectsCrossOriginBrowserRequest(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+
			uuid.New().String()+
			"/branches/"+
			uuid.New().String()+
			"/revisions",
		strings.NewReader(
			`{"message":"Initial revision","expectedHeadRevisionId":null}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	request = requestWithSession(request, uuid.New())

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusForbidden,
			response.Code,
		)
	}

	if service.createCalled {
		t.Fatal("expected versioning service not to be called")
	}
}

func TestCreateProjectRevisionRejectsMalformedPathIDs(t *testing.T) {
	tests := []struct {
		name      string
		projectID string
		branchID  string
	}{
		{
			name:      "malformed project ID",
			projectID: "not-a-uuid",
			branchID:  uuid.New().String(),
		},
		{
			name:      "malformed branch ID",
			projectID: uuid.New().String(),
			branchID:  "not-a-uuid",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeVersioningService{}
			handler := newVersioningHandler(service)

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/projects/"+
					test.projectID+
					"/branches/"+
					test.branchID+
					"/revisions",
				strings.NewReader(
					`{"message":"Revision","expectedHeadRevisionId":null}`,
				),
			)
			request.Header.Set(
				"Content-Type",
				"application/json",
			)
			request = requestWithSession(
				request,
				uuid.New(),
			)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected status %d, got %d",
					http.StatusBadRequest,
					response.Code,
				)
			}

			if service.createCalled {
				t.Fatal(
					"expected versioning service not to be called",
				)
			}
		})
	}
}

func TestCreateProjectRevisionRejectsInvalidJSON(t *testing.T) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+
			uuid.New().String()+
			"/branches/"+
			uuid.New().String()+
			"/revisions",
		strings.NewReader(`{"message":`),
	)
	request.Header.Set("Content-Type", "application/json")
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

	if service.createCalled {
		t.Fatal("expected versioning service not to be called")
	}
}

func TestCreateProjectRevisionRequiresExpectedHeadField(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+
			uuid.New().String()+
			"/branches/"+
			uuid.New().String()+
			"/revisions",
		strings.NewReader(`{"message":"Initial revision"}`),
	)
	request.Header.Set("Content-Type", "application/json")
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

	if service.createCalled {
		t.Fatal("expected versioning service not to be called")
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "expected head revision ID is required" {
		t.Fatalf(
			"expected missing expected-head error, got %q",
			body.Error,
		)
	}
}

func TestCreateProjectRevisionRejectsMissingVersioningDependency(
	t *testing.T,
) {
	projectID := uuid.New()
	branchID := uuid.New()

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
		http.MethodPost,
		"/api/projects/"+
			projectID.String()+
			"/branches/"+
			branchID.String()+
			"/revisions",
		strings.NewReader(
			`{"message":"Initial revision","expectedHeadRevisionId":null}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
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
	if body.Error != "unable to create project revision" {
		t.Fatalf(
			"expected missing dependency error, got %q",
			body.Error,
		)
	}
}

func TestCreateProjectRevisionMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name        string
		projectID   uuid.UUID
		branchID    uuid.UUID
		serviceErr  error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing project ID",
			projectID:   uuid.Nil,
			branchID:    uuid.New(),
			serviceErr:  versioning.ErrProjectIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project ID is required",
		},
		{
			name:        "missing branch ID",
			projectID:   uuid.New(),
			branchID:    uuid.Nil,
			serviceErr:  versioning.ErrBranchIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "branch ID is required",
		},
		{
			name:        "message required",
			projectID:   uuid.New(),
			branchID:    uuid.New(),
			serviceErr:  versioning.ErrMessageRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "revision message is required",
		},
		{
			name:        "invalid expected head",
			projectID:   uuid.New(),
			branchID:    uuid.New(),
			serviceErr:  versioning.ErrExpectedHeadRevisionIDInvalid,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "expected head revision ID is invalid",
		},
		{
			name:        "invalid merge parent",
			projectID:   uuid.New(),
			branchID:    uuid.New(),
			serviceErr:  versioning.ErrMergeParentRevisionIDInvalid,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "merge parent revision ID is invalid",
		},
		{
			name:        "merge requires head",
			projectID:   uuid.New(),
			branchID:    uuid.New(),
			serviceErr:  versioning.ErrMergeRequiresHead,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "merge revision requires an expected branch head",
		},
		{
			name:        "parents must differ",
			projectID:   uuid.New(),
			branchID:    uuid.New(),
			serviceErr:  versioning.ErrRevisionParentsMustDiffer,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "revision parents must differ",
		},
		{
			name:        "project not found",
			projectID:   uuid.New(),
			branchID:    uuid.New(),
			serviceErr:  versioning.ErrProjectNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project not found",
		},
		{
			name:        "branch not found",
			projectID:   uuid.New(),
			branchID:    uuid.New(),
			serviceErr:  versioning.ErrBranchNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "branch not found",
		},
		{
			name:        "branch head changed",
			projectID:   uuid.New(),
			branchID:    uuid.New(),
			serviceErr:  versioning.ErrBranchHeadConflict,
			wantStatus:  http.StatusConflict,
			wantMessage: "branch head changed",
		},
		{
			name:        "unexpected service error",
			projectID:   uuid.New(),
			branchID:    uuid.New(),
			serviceErr:  errors.New("database unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to create project revision",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userID := uuid.New()
			service := &fakeVersioningService{
				createErr: test.serviceErr,
			}
			handler := newVersioningHandler(service)

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/projects/"+
					test.projectID.String()+
					"/branches/"+
					test.branchID.String()+
					"/revisions",
				strings.NewReader(
					`{"message":"Revision","expectedHeadRevisionId":null}`,
				),
			)
			request.Header.Set(
				"Content-Type",
				"application/json",
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

			if !service.createCalled {
				t.Fatal("expected versioning service to be called")
			}

			if service.createInput.OwnerUserID != userID {
				t.Fatalf(
					"expected owner ID %s, got %s",
					userID,
					service.createInput.OwnerUserID,
				)
			}

			if service.createInput.ProjectID != test.projectID {
				t.Fatalf(
					"expected project ID %s, got %s",
					test.projectID,
					service.createInput.ProjectID,
				)
			}

			if service.createInput.BranchID != test.branchID {
				t.Fatalf(
					"expected branch ID %s, got %s",
					test.branchID,
					service.createInput.BranchID,
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

func TestCreateProjectRevisionAcceptsExplicitNullExpectedHead(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	branchID := uuid.New()
	revisionID := uuid.New()

	createdAt := time.Date(
		2026,
		time.September,
		22,
		20,
		0,
		0,
		0,
		time.UTC,
	)

	service := &fakeVersioningService{
		createResult: dbgen.ProjectRevision{
			ID:           revisionID,
			ProjectID:    projectID,
			AuthorUserID: userID,
			Message:      "Initial revision",
			CreatedAt:    createdAt,
		},
	}

	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+
			projectID.String()+
			"/branches/"+
			branchID.String()+
			"/revisions",
		strings.NewReader(
			`{"message":"Initial revision","expectedHeadRevisionId":null}`,
		),
	)
	request.Header.Set("Content-Type", "application/json")
	request = requestWithSession(request, userID)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.Code,
		)
	}

	if !service.createCalled {
		t.Fatal("expected versioning service to be called")
	}

	if service.createInput.OwnerUserID != userID {
		t.Fatalf(
			"expected owner ID %s, got %s",
			userID,
			service.createInput.OwnerUserID,
		)
	}

	if service.createInput.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			service.createInput.ProjectID,
		)
	}

	if service.createInput.BranchID != branchID {
		t.Fatalf(
			"expected branch ID %s, got %s",
			branchID,
			service.createInput.BranchID,
		)
	}

	if service.createInput.Message != "Initial revision" {
		t.Fatalf(
			"expected message %q, got %q",
			"Initial revision",
			service.createInput.Message,
		)
	}

	if service.createInput.ExpectedHeadRevisionID != nil {
		t.Fatalf(
			"expected nil expected head, got %s",
			*service.createInput.ExpectedHeadRevisionID,
		)
	}

	if service.createInput.MergeParentRevisionID != nil {
		t.Fatalf(
			"expected nil merge parent, got %s",
			*service.createInput.MergeParentRevisionID,
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

	if body.Message != "Initial revision" {
		t.Fatalf(
			"expected message %q, got %q",
			"Initial revision",
			body.Message,
		)
	}

	if body.ParentRevisionId != nil {
		t.Fatalf(
			"expected parent revision to be null, got %s",
			*body.ParentRevisionId,
		)
	}

	if body.MergeParentRevisionId != nil {
		t.Fatalf(
			"expected merge parent revision to be null, got %s",
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

func TestCreateProjectRevisionForwardsExpectedHeadAndMergeParent(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	branchID := uuid.New()
	revisionID := uuid.New()
	expectedHeadID := uuid.New()
	mergeParentID := uuid.New()

	createdAt := time.Date(
		2026,
		time.September,
		22,
		21,
		0,
		0,
		0,
		time.UTC,
	)

	service := &fakeVersioningService{
		createResult: dbgen.ProjectRevision{
			ID:           revisionID,
			ProjectID:    projectID,
			AuthorUserID: userID,
			Message:      "Merge feature branch",
			ParentRevisionID: pgtype.UUID{
				Bytes: expectedHeadID,
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

	requestBody :=
		`{"message":"Merge feature branch",` +
			`"expectedHeadRevisionId":"` +
			expectedHeadID.String() +
			`","mergeParentRevisionId":"` +
			mergeParentID.String() +
			`"}`

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+
			projectID.String()+
			"/branches/"+
			branchID.String()+
			"/revisions",
		strings.NewReader(requestBody),
	)
	request.Header.Set("Content-Type", "application/json")
	request = requestWithSession(request, userID)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.Code,
		)
	}

	if !service.createCalled {
		t.Fatal("expected versioning service to be called")
	}

	if service.createInput.ExpectedHeadRevisionID == nil {
		t.Fatal("expected branch head revision ID")
	}

	if *service.createInput.ExpectedHeadRevisionID != expectedHeadID {
		t.Fatalf(
			"expected head revision ID %s, got %s",
			expectedHeadID,
			*service.createInput.ExpectedHeadRevisionID,
		)
	}

	if service.createInput.MergeParentRevisionID == nil {
		t.Fatal("expected merge parent revision ID")
	}

	if *service.createInput.MergeParentRevisionID != mergeParentID {
		t.Fatalf(
			"expected merge parent revision ID %s, got %s",
			mergeParentID,
			*service.createInput.MergeParentRevisionID,
		)
	}

	var body api.ProjectRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode revision response: %v", err)
	}

	if body.ParentRevisionId == nil {
		t.Fatal("expected parent revision in response")
	}

	if uuid.UUID(*body.ParentRevisionId) != expectedHeadID {
		t.Fatalf(
			"expected response parent ID %s, got %s",
			expectedHeadID,
			*body.ParentRevisionId,
		)
	}

	if body.MergeParentRevisionId == nil {
		t.Fatal("expected merge parent revision in response")
	}

	if uuid.UUID(*body.MergeParentRevisionId) != mergeParentID {
		t.Fatalf(
			"expected response merge parent ID %s, got %s",
			mergeParentID,
			*body.MergeParentRevisionId,
		)
	}
}

func TestCreateProjectRevisionRejectsMalformedBodyUUIDs(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "malformed expected head revision ID",
			body: `{
				"message":"Revision",
				"expectedHeadRevisionId":"not-a-uuid"
			}`,
		},
		{
			name: "malformed merge parent revision ID",
			body: `{
				"message":"Merge revision",
				"expectedHeadRevisionId":null,
				"mergeParentRevisionId":"not-a-uuid"
			}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeVersioningService{}
			handler := newVersioningHandler(service)

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/projects/"+
					uuid.New().String()+
					"/branches/"+
					uuid.New().String()+
					"/revisions",
				strings.NewReader(test.body),
			)
			request.Header.Set(
				"Content-Type",
				"application/json",
			)
			request = requestWithSession(
				request,
				uuid.New(),
			)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected status %d, got %d",
					http.StatusBadRequest,
					response.Code,
				)
			}

			if service.createCalled {
				t.Fatal(
					"expected versioning service not to be called",
				)
			}
		})
	}
}

func TestListProjectBranchHistoryRequiresAuthentication(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			uuid.New().String()+
			"/branches/"+
			uuid.New().String()+
			"/history",
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

	if service.historyCalled {
		t.Fatal(
			"expected versioning service not to be called",
		)
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "authentication required" {
		t.Fatalf(
			"expected authentication error, got %q",
			body.Error,
		)
	}
}

func TestListProjectBranchHistoryRejectsSessionResolutionFailure(
	t *testing.T,
) {
	service := &fakeVersioningService{}
	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			uuid.New().String()+
			"/branches/"+
			uuid.New().String()+
			"/history",
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

	if service.historyCalled {
		t.Fatal(
			"expected versioning service not to be called",
		)
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "unable to authenticate request" {
		t.Fatalf(
			"expected authentication failure error, got %q",
			body.Error,
		)
	}
}

func TestListProjectBranchHistoryRejectsMalformedIDs(
	t *testing.T,
) {
	tests := []struct {
		name      string
		projectID string
		branchID  string
	}{
		{
			name:      "malformed project ID",
			projectID: "not-a-uuid",
			branchID:  uuid.New().String(),
		},
		{
			name:      "malformed branch ID",
			projectID: uuid.New().String(),
			branchID:  "not-a-uuid",
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
					"/branches/"+
					test.branchID+
					"/history",
				nil,
			)
			request = requestWithSession(
				request,
				uuid.New(),
			)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected status %d, got %d",
					http.StatusBadRequest,
					response.Code,
				)
			}

			if service.historyCalled {
				t.Fatal(
					"expected versioning service not to be called",
				)
			}
		})
	}
}

func TestListProjectBranchHistoryMapsServiceErrors(
	t *testing.T,
) {
	tests := []struct {
		name        string
		projectID   uuid.UUID
		branchID    uuid.UUID
		serviceErr  error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing project ID",
			projectID:   uuid.Nil,
			branchID:    uuid.New(),
			serviceErr:  versioning.ErrProjectIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project ID is required",
		},
		{
			name:        "missing branch ID",
			projectID:   uuid.New(),
			branchID:    uuid.Nil,
			serviceErr:  versioning.ErrBranchIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "branch ID is required",
		},
		{
			name:        "project not found",
			projectID:   uuid.New(),
			branchID:    uuid.New(),
			serviceErr:  versioning.ErrProjectNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project not found",
		},
		{
			name:        "branch not found",
			projectID:   uuid.New(),
			branchID:    uuid.New(),
			serviceErr:  versioning.ErrBranchNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "branch not found",
		},
		{
			name:        "unexpected service error",
			projectID:   uuid.New(),
			branchID:    uuid.New(),
			serviceErr:  errors.New("database unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to list project branch history",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			userID := uuid.New()
			service := &fakeVersioningService{
				historyErr: test.serviceErr,
			}
			handler := newVersioningHandler(service)

			request := httptest.NewRequest(
				http.MethodGet,
				"/api/projects/"+
					test.projectID.String()+
					"/branches/"+
					test.branchID.String()+
					"/history",
				nil,
			)
			request = requestWithSession(
				request,
				userID,
			)

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != test.wantStatus {
				t.Fatalf(
					"expected status %d, got %d",
					test.wantStatus,
					response.Code,
				)
			}

			if !service.historyCalled {
				t.Fatal(
					"expected versioning service to be called",
				)
			}

			if service.historyOwnerUserID != userID {
				t.Fatalf(
					"expected owner %s, got %s",
					userID,
					service.historyOwnerUserID,
				)
			}

			if service.historyProjectID != test.projectID {
				t.Fatalf(
					"expected project ID %s, got %s",
					test.projectID,
					service.historyProjectID,
				)
			}

			if service.historyBranchID != test.branchID {
				t.Fatalf(
					"expected branch ID %s, got %s",
					test.branchID,
					service.historyBranchID,
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

func TestListProjectBranchHistoryRejectsMissingVersioningDependency(
	t *testing.T,
) {
	projectID := uuid.New()
	branchID := uuid.New()

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
			"/branches/"+
			branchID.String()+
			"/history",
		nil,
	)
	request = requestWithSession(
		request,
		uuid.New(),
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

	body := decodeErrorResponse(t, response)
	if body.Error != "unable to list project branch history" {
		t.Fatalf(
			"expected missing dependency error, got %q",
			body.Error,
		)
	}
}

func TestListProjectBranchHistoryReturnsRevisionsForAuthenticatedUser(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	branchID := uuid.New()

	rootRevisionID := uuid.New()
	mainRevisionID := uuid.New()
	sideRevisionID := uuid.New()
	mergeRevisionID := uuid.New()

	rootCreatedAt := time.Date(
		2026,
		time.September,
		24,
		12,
		0,
		0,
		0,
		time.UTC,
	)
	mergeCreatedAt := rootCreatedAt.Add(time.Hour)

	service := &fakeVersioningService{
		history: []dbgen.ProjectRevision{
			{
				ID:           mergeRevisionID,
				ProjectID:    projectID,
				AuthorUserID: userID,
				Message:      "Merge feature into main",
				ParentRevisionID: pgtype.UUID{
					Bytes: mainRevisionID,
					Valid: true,
				},
				MergeParentRevisionID: pgtype.UUID{
					Bytes: sideRevisionID,
					Valid: true,
				},
				CreatedAt: mergeCreatedAt,
			},
			{
				ID:           rootRevisionID,
				ProjectID:    projectID,
				AuthorUserID: userID,
				Message:      "Initial revision",
				CreatedAt:    rootCreatedAt,
			},
		},
	}

	handler := newVersioningHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			projectID.String()+
			"/branches/"+
			branchID.String()+
			"/history",
		nil,
	)
	request = requestWithSession(
		request,
		userID,
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}

	if !service.historyCalled {
		t.Fatal(
			"expected versioning service to be called",
		)
	}

	if service.historyOwnerUserID != userID {
		t.Fatalf(
			"expected owner %s, got %s",
			userID,
			service.historyOwnerUserID,
		)
	}

	if service.historyProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			service.historyProjectID,
		)
	}

	if service.historyBranchID != branchID {
		t.Fatalf(
			"expected branch ID %s, got %s",
			branchID,
			service.historyBranchID,
		)
	}

	var body []api.ProjectRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf(
			"decode branch history response: %v",
			err,
		)
	}

	if len(body) != 2 {
		t.Fatalf(
			"expected 2 revisions, got %d",
			len(body),
		)
	}

	if uuid.UUID(body[0].Id) != mergeRevisionID {
		t.Fatalf(
			"expected first revision ID %s, got %s",
			mergeRevisionID,
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

	if uuid.UUID(body[0].AuthorUserId) != userID {
		t.Fatalf(
			"expected first author ID %s, got %s",
			userID,
			body[0].AuthorUserId,
		)
	}

	if body[0].Message != "Merge feature into main" {
		t.Fatalf(
			"expected merge message, got %q",
			body[0].Message,
		)
	}

	if body[0].ParentRevisionId == nil {
		t.Fatal(
			"expected primary parent revision",
		)
	}

	if uuid.UUID(*body[0].ParentRevisionId) != mainRevisionID {
		t.Fatalf(
			"expected primary parent %s, got %s",
			mainRevisionID,
			*body[0].ParentRevisionId,
		)
	}

	if body[0].MergeParentRevisionId == nil {
		t.Fatal(
			"expected merge parent revision",
		)
	}

	if uuid.UUID(*body[0].MergeParentRevisionId) != sideRevisionID {
		t.Fatalf(
			"expected merge parent %s, got %s",
			sideRevisionID,
			*body[0].MergeParentRevisionId,
		)
	}

	if !body[0].CreatedAt.Equal(mergeCreatedAt) {
		t.Fatalf(
			"expected merge created time %s, got %s",
			mergeCreatedAt,
			body[0].CreatedAt,
		)
	}

	if uuid.UUID(body[1].Id) != rootRevisionID {
		t.Fatalf(
			"expected second revision ID %s, got %s",
			rootRevisionID,
			body[1].Id,
		)
	}

	if body[1].ParentRevisionId != nil {
		t.Fatalf(
			"expected root primary parent to be null, got %s",
			*body[1].ParentRevisionId,
		)
	}

	if body[1].MergeParentRevisionId != nil {
		t.Fatalf(
			"expected root merge parent to be null, got %s",
			*body[1].MergeParentRevisionId,
		)
	}
}

func TestListProjectBranchHistoryReturnsEmptyArray(
	t *testing.T,
) {
	projectID := uuid.New()
	branchID := uuid.New()

	handler := newVersioningHandler(
		&fakeVersioningService{},
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			projectID.String()+
			"/branches/"+
			branchID.String()+
			"/history",
		nil,
	)
	request = requestWithSession(
		request,
		uuid.New(),
	)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}

	var body []api.ProjectRevisionResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf(
			"decode branch history response: %v",
			err,
		)
	}

	if body == nil {
		t.Fatal(
			"expected empty array, got null",
		)
	}

	if len(body) != 0 {
		t.Fatalf(
			"expected 0 revisions, got %d",
			len(body),
		)
	}
}
