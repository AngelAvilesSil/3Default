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
	"github.com/AngelAvilesSil/3Default/internal/conversionjobs"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeConversionJobService struct {
	createCalled bool
	createInput  conversionjobs.CreateInput
	createJob    dbgen.ConversionJob
	createErr    error

	getCalled      bool
	getOwnerUserID uuid.UUID
	getProjectID   uuid.UUID
	getJobID       uuid.UUID
	getJob         dbgen.ConversionJob
	getErr         error

	listCalled        bool
	listOwnerUserID   uuid.UUID
	listProjectID     uuid.UUID
	listProjectFileID uuid.UUID
	jobs              []dbgen.ConversionJob
	listErr           error
}

func (f *fakeConversionJobService) Create(
	_ context.Context,
	input conversionjobs.CreateInput,
) (dbgen.ConversionJob, error) {
	f.createCalled = true
	f.createInput = input

	return f.createJob, f.createErr
}

func (f *fakeConversionJobService) Get(
	_ context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	conversionJobID uuid.UUID,
) (dbgen.ConversionJob, error) {
	f.getCalled = true
	f.getOwnerUserID = ownerUserID
	f.getProjectID = projectID
	f.getJobID = conversionJobID

	return f.getJob, f.getErr
}

func (f *fakeConversionJobService) ListForProjectFile(
	_ context.Context,
	ownerUserID uuid.UUID,
	projectID uuid.UUID,
	projectFileID uuid.UUID,
) ([]dbgen.ConversionJob, error) {
	f.listCalled = true
	f.listOwnerUserID = ownerUserID
	f.listProjectID = projectID
	f.listProjectFileID = projectFileID

	return f.jobs, f.listErr
}

func newConversionJobHandler(
	service ConversionJobService,
) http.Handler {
	return NewHandler(
		NewServerWithConversionJobs(
			fakeDatabase{},
			nil,
			nil,
			nil,
			nil,
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

func newConversionJobHandlerWithResolver(
	service ConversionJobService,
	resolver SessionResolver,
) http.Handler {
	return NewHandler(
		NewServerWithConversionJobs(
			fakeDatabase{},
			nil,
			nil,
			nil,
			nil,
			nil,
			service,
			nil,
			nil,
			nil,
			nil,
		),
		resolver,
	)
}

func TestConversionJobEndpointsRequireAuthentication(
	t *testing.T,
) {
	projectID := uuid.New()
	projectFileID := uuid.New()
	jobID := uuid.New()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path: "/api/projects/" +
				projectID.String() +
				"/files/" +
				projectFileID.String() +
				"/conversion-jobs",
		},
		{
			name:   "get",
			method: http.MethodGet,
			path: "/api/projects/" +
				projectID.String() +
				"/conversion-jobs/" +
				jobID.String(),
		},
		{
			name:   "list",
			method: http.MethodGet,
			path: "/api/projects/" +
				projectID.String() +
				"/files/" +
				projectFileID.String() +
				"/conversion-jobs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeConversionJobService{}
			handler := newConversionJobHandler(service)

			request := httptest.NewRequest(
				test.method,
				test.path,
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

			if service.createCalled ||
				service.getCalled ||
				service.listCalled {
				t.Fatal(
					"expected conversion service not to be called",
				)
			}

			body := decodeErrorResponse(t, response)
			if body.Error != "authentication required" {
				t.Fatalf(
					"expected authentication error, got %q",
					body.Error,
				)
			}
		})
	}
}

func TestConversionJobEndpointsRejectSessionResolutionFailure(
	t *testing.T,
) {
	projectID := uuid.New()
	projectFileID := uuid.New()
	jobID := uuid.New()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path: "/api/projects/" +
				projectID.String() +
				"/files/" +
				projectFileID.String() +
				"/conversion-jobs",
		},
		{
			name:   "get",
			method: http.MethodGet,
			path: "/api/projects/" +
				projectID.String() +
				"/conversion-jobs/" +
				jobID.String(),
		},
		{
			name:   "list",
			method: http.MethodGet,
			path: "/api/projects/" +
				projectID.String() +
				"/files/" +
				projectFileID.String() +
				"/conversion-jobs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeConversionJobService{}
			resolver := &fakeSessionResolver{
				err: errors.New("session database unavailable"),
			}

			handler := newConversionJobHandlerWithResolver(
				service,
				resolver,
			)

			request := httptest.NewRequest(
				test.method,
				test.path,
				nil,
			)
			request.AddCookie(&http.Cookie{
				Name:  sessionCookieName,
				Value: "session-token",
			})

			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusInternalServerError {
				t.Fatalf(
					"expected status %d, got %d",
					http.StatusInternalServerError,
					response.Code,
				)
			}

			if !resolver.called {
				t.Fatal("expected session resolver call")
			}

			if service.createCalled ||
				service.getCalled ||
				service.listCalled {
				t.Fatal(
					"expected conversion service not to be called",
				)
			}

			body := decodeErrorResponse(t, response)
			if body.Error != "unable to authenticate request" {
				t.Fatalf(
					"expected authentication failure, got %q",
					body.Error,
				)
			}
		})
	}
}

func TestConversionJobEndpointsRejectMalformedIDs(
	t *testing.T,
) {
	projectID := uuid.New()
	projectFileID := uuid.New()
	jobID := uuid.New()

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{
			name:   "create malformed project",
			method: http.MethodPost,
			path: "/api/projects/not-a-uuid/files/" +
				projectFileID.String() +
				"/conversion-jobs",
		},
		{
			name:   "create malformed project file",
			method: http.MethodPost,
			path: "/api/projects/" +
				projectID.String() +
				"/files/not-a-uuid/conversion-jobs",
		},
		{
			name:   "get malformed project",
			method: http.MethodGet,
			path: "/api/projects/not-a-uuid/conversion-jobs/" +
				jobID.String(),
		},
		{
			name:   "get malformed job",
			method: http.MethodGet,
			path: "/api/projects/" +
				projectID.String() +
				"/conversion-jobs/not-a-uuid",
		},
		{
			name:   "list malformed project",
			method: http.MethodGet,
			path: "/api/projects/not-a-uuid/files/" +
				projectFileID.String() +
				"/conversion-jobs",
		},
		{
			name:   "list malformed project file",
			method: http.MethodGet,
			path: "/api/projects/" +
				projectID.String() +
				"/files/not-a-uuid/conversion-jobs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeConversionJobService{}
			handler := newConversionJobHandler(service)

			request := httptest.NewRequest(
				test.method,
				test.path,
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

			if service.createCalled ||
				service.getCalled ||
				service.listCalled {
				t.Fatal(
					"expected conversion service not to be called",
				)
			}

		})
	}
}

func TestCreateProjectFileConversionJobRejectsCrossOriginRequest(
	t *testing.T,
) {
	service := &fakeConversionJobService{}
	handler := newConversionJobHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+
			uuid.New().String()+
			"/files/"+
			uuid.New().String()+
			"/conversion-jobs",
		nil,
	)
	request.Header.Set(
		"Origin",
		"https://attacker.example",
	)

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
		t.Fatal(
			"expected conversion service not to be called",
		)
	}

	body := decodeErrorResponse(t, response)
	if body.Error != "cross-origin request denied" {
		t.Fatalf(
			"expected cross-origin error, got %q",
			body.Error,
		)
	}
}

func TestConversionJobEndpointsRequireDependency(
	t *testing.T,
) {
	projectID := uuid.New()
	projectFileID := uuid.New()
	jobID := uuid.New()

	tests := []struct {
		name        string
		method      string
		path        string
		wantMessage string
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path: "/api/projects/" +
				projectID.String() +
				"/files/" +
				projectFileID.String() +
				"/conversion-jobs",
			wantMessage: "unable to create conversion job",
		},
		{
			name:   "get",
			method: http.MethodGet,
			path: "/api/projects/" +
				projectID.String() +
				"/conversion-jobs/" +
				jobID.String(),
			wantMessage: "unable to get conversion job",
		},
		{
			name:   "list",
			method: http.MethodGet,
			path: "/api/projects/" +
				projectID.String() +
				"/files/" +
				projectFileID.String() +
				"/conversion-jobs",
			wantMessage: "unable to list conversion jobs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := newConversionJobHandler(nil)

			request := httptest.NewRequest(
				test.method,
				test.path,
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

func TestCreateProjectFileConversionJobMapsServiceErrors(
	t *testing.T,
) {
	tests := []struct {
		name        string
		serviceErr  error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing project ID",
			serviceErr:  conversionjobs.ErrProjectIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project ID is required",
		},
		{
			name:        "missing project file ID",
			serviceErr:  conversionjobs.ErrProjectFileIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project file ID is required",
		},
		{
			name:        "project not found",
			serviceErr:  conversionjobs.ErrProjectNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project not found",
		},
		{
			name:        "project file not found",
			serviceErr:  conversionjobs.ErrProjectFileNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project file not found",
		},
		{
			name: "active job conflict",
			serviceErr: conversionjobs.
				ErrActiveConversionJobExists,
			wantStatus:  http.StatusConflict,
			wantMessage: "active conversion job already exists",
		},
		{
			name:        "unexpected error",
			serviceErr:  errors.New("database unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to create conversion job",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeConversionJobService{
				createErr: test.serviceErr,
			}
			handler := newConversionJobHandler(service)

			request := httptest.NewRequest(
				http.MethodPost,
				"/api/projects/"+
					uuid.New().String()+
					"/files/"+
					uuid.New().String()+
					"/conversion-jobs",
				nil,
			)
			request = requestWithSession(
				request,
				uuid.New(),
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

func TestCreateProjectFileConversionJobForwardsAuthenticatedRequest(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()
	jobID := uuid.New()

	createdAt := time.Date(
		2026,
		time.October,
		2,
		20,
		0,
		0,
		0,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Second)

	service := &fakeConversionJobService{
		createJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     projectID,
			ProjectFileID: projectFileID,
			Status:        "pending",
			AttemptCount:  0,
			CreatedAt:     createdAt,
			UpdatedAt:     updatedAt,
		},
	}
	handler := newConversionJobHandler(service)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects/"+
			projectID.String()+
			"/files/"+
			projectFileID.String()+
			"/conversion-jobs",
		nil,
	)
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
		t.Fatal("expected conversion service call")
	}

	if service.createInput.OwnerUserID != userID ||
		service.createInput.ProjectID != projectID ||
		service.createInput.ProjectFileID != projectFileID {
		t.Fatalf(
			"unexpected create input: %+v",
			service.createInput,
		)
	}

	var body api.ConversionJobResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf(
			"decode conversion job response: %v",
			err,
		)
	}

	if uuid.UUID(body.Id) != jobID ||
		uuid.UUID(body.ProjectId) != projectID ||
		uuid.UUID(body.ProjectFileId) != projectFileID ||
		body.Status != api.Pending ||
		body.AttemptCount != 0 ||
		body.LastError != nil ||
		body.StartedAt != nil ||
		body.FinishedAt != nil ||
		!body.CreatedAt.Equal(createdAt) ||
		!body.UpdatedAt.Equal(updatedAt) {
		t.Fatalf(
			"unexpected conversion job response: %+v",
			body,
		)
	}
}

func TestGetProjectConversionJobMapsServiceErrors(
	t *testing.T,
) {
	tests := []struct {
		name        string
		serviceErr  error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing project ID",
			serviceErr:  conversionjobs.ErrProjectIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project ID is required",
		},
		{
			name: "missing conversion job ID",
			serviceErr: conversionjobs.
				ErrConversionJobIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "conversion job ID is required",
		},
		{
			name:        "project not found",
			serviceErr:  conversionjobs.ErrProjectNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project not found",
		},
		{
			name: "conversion job not found",
			serviceErr: conversionjobs.
				ErrConversionJobNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "conversion job not found",
		},
		{
			name:        "unexpected error",
			serviceErr:  errors.New("database unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to get conversion job",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeConversionJobService{
				getErr: test.serviceErr,
			}
			handler := newConversionJobHandler(service)

			request := httptest.NewRequest(
				http.MethodGet,
				"/api/projects/"+
					uuid.New().String()+
					"/conversion-jobs/"+
					uuid.New().String(),
				nil,
			)
			request = requestWithSession(
				request,
				uuid.New(),
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

func TestGetProjectConversionJobReturnsRunningJob(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()
	jobID := uuid.New()

	createdAt := time.Date(
		2026,
		time.October,
		2,
		20,
		10,
		0,
		0,
		time.UTC,
	)
	startedAt := createdAt.Add(time.Second)
	updatedAt := startedAt.Add(time.Second)

	service := &fakeConversionJobService{
		getJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     projectID,
			ProjectFileID: projectFileID,
			Status:        "running",
			AttemptCount:  2,
			CreatedAt:     createdAt,
			StartedAt: pgtype.Timestamptz{
				Time:  startedAt,
				Valid: true,
			},
			UpdatedAt: updatedAt,
		},
	}
	handler := newConversionJobHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			projectID.String()+
			"/conversion-jobs/"+
			jobID.String(),
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

	if service.getOwnerUserID != userID ||
		service.getProjectID != projectID ||
		service.getJobID != jobID {
		t.Fatalf(
			"unexpected get arguments: owner=%s project=%s job=%s",
			service.getOwnerUserID,
			service.getProjectID,
			service.getJobID,
		)
	}

	var body api.ConversionJobResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf(
			"decode conversion job response: %v",
			err,
		)
	}

	if uuid.UUID(body.Id) != jobID ||
		uuid.UUID(body.ProjectId) != projectID ||
		uuid.UUID(body.ProjectFileId) != projectFileID ||
		body.Status != api.Running ||
		body.AttemptCount != 2 ||
		body.LastError != nil ||
		body.StartedAt == nil ||
		!body.StartedAt.Equal(startedAt) ||
		body.FinishedAt != nil ||
		!body.CreatedAt.Equal(createdAt) ||
		!body.UpdatedAt.Equal(updatedAt) {
		t.Fatalf(
			"unexpected conversion job response: %+v",
			body,
		)
	}
}

func TestListProjectFileConversionJobsMapsServiceErrors(
	t *testing.T,
) {
	tests := []struct {
		name        string
		serviceErr  error
		wantStatus  int
		wantMessage string
	}{
		{
			name:        "missing project ID",
			serviceErr:  conversionjobs.ErrProjectIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project ID is required",
		},
		{
			name: "missing project file ID",
			serviceErr: conversionjobs.
				ErrProjectFileIDRequired,
			wantStatus:  http.StatusBadRequest,
			wantMessage: "project file ID is required",
		},
		{
			name:        "project not found",
			serviceErr:  conversionjobs.ErrProjectNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project not found",
		},
		{
			name: "project file not found",
			serviceErr: conversionjobs.
				ErrProjectFileNotFound,
			wantStatus:  http.StatusNotFound,
			wantMessage: "project file not found",
		},
		{
			name:        "unexpected error",
			serviceErr:  errors.New("database unavailable"),
			wantStatus:  http.StatusInternalServerError,
			wantMessage: "unable to list conversion jobs",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := &fakeConversionJobService{
				listErr: test.serviceErr,
			}
			handler := newConversionJobHandler(service)

			request := httptest.NewRequest(
				http.MethodGet,
				"/api/projects/"+
					uuid.New().String()+
					"/files/"+
					uuid.New().String()+
					"/conversion-jobs",
				nil,
			)
			request = requestWithSession(
				request,
				uuid.New(),
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

func TestListProjectFileConversionJobsReturnsHistory(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()

	firstJobID := uuid.New()
	secondJobID := uuid.New()

	baseTime := time.Date(
		2026,
		time.October,
		2,
		20,
		20,
		0,
		0,
		time.UTC,
	)

	firstStartedAt := baseTime.Add(time.Second)
	firstFinishedAt := baseTime.Add(2 * time.Second)
	firstError := "converter failed"

	secondStartedAt := baseTime.Add(-2 * time.Minute)
	secondFinishedAt := baseTime.Add(-time.Minute)

	service := &fakeConversionJobService{
		jobs: []dbgen.ConversionJob{
			{
				ID:            firstJobID,
				ProjectID:     projectID,
				ProjectFileID: projectFileID,
				Status:        "failed",
				AttemptCount:  2,
				LastError:     &firstError,
				CreatedAt:     baseTime,
				StartedAt: pgtype.Timestamptz{
					Time:  firstStartedAt,
					Valid: true,
				},
				FinishedAt: pgtype.Timestamptz{
					Time:  firstFinishedAt,
					Valid: true,
				},
				UpdatedAt: firstFinishedAt,
			},
			{
				ID:            secondJobID,
				ProjectID:     projectID,
				ProjectFileID: projectFileID,
				Status:        "succeeded",
				AttemptCount:  1,
				CreatedAt:     baseTime.Add(-3 * time.Minute),
				StartedAt: pgtype.Timestamptz{
					Time:  secondStartedAt,
					Valid: true,
				},
				FinishedAt: pgtype.Timestamptz{
					Time:  secondFinishedAt,
					Valid: true,
				},
				UpdatedAt: secondFinishedAt,
			},
		},
	}
	handler := newConversionJobHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			projectID.String()+
			"/files/"+
			projectFileID.String()+
			"/conversion-jobs",
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

	if service.listOwnerUserID != userID ||
		service.listProjectID != projectID ||
		service.listProjectFileID != projectFileID {
		t.Fatalf(
			"unexpected list arguments: owner=%s project=%s file=%s",
			service.listOwnerUserID,
			service.listProjectID,
			service.listProjectFileID,
		)
	}

	var body []api.ConversionJobResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf(
			"decode conversion job history: %v",
			err,
		)
	}

	if len(body) != 2 {
		t.Fatalf(
			"expected 2 conversion jobs, got %d",
			len(body),
		)
	}

	if uuid.UUID(body[0].Id) != firstJobID ||
		body[0].Status != api.Failed ||
		body[0].AttemptCount != 2 ||
		body[0].LastError == nil ||
		*body[0].LastError != firstError ||
		body[0].StartedAt == nil ||
		!body[0].StartedAt.Equal(firstStartedAt) ||
		body[0].FinishedAt == nil ||
		!body[0].FinishedAt.Equal(firstFinishedAt) {
		t.Fatalf(
			"unexpected first conversion job: %+v",
			body[0],
		)
	}

	if uuid.UUID(body[1].Id) != secondJobID ||
		body[1].Status != api.Succeeded ||
		body[1].AttemptCount != 1 ||
		body[1].LastError != nil ||
		body[1].StartedAt == nil ||
		!body[1].StartedAt.Equal(secondStartedAt) ||
		body[1].FinishedAt == nil ||
		!body[1].FinishedAt.Equal(secondFinishedAt) {
		t.Fatalf(
			"unexpected second conversion job: %+v",
			body[1],
		)
	}
}

func TestListProjectFileConversionJobsReturnsEmptyArray(
	t *testing.T,
) {
	service := &fakeConversionJobService{
		jobs: nil,
	}
	handler := newConversionJobHandler(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/projects/"+
			uuid.New().String()+
			"/files/"+
			uuid.New().String()+
			"/conversion-jobs",
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

	var raw any
	if err := json.NewDecoder(response.Body).Decode(&raw); err != nil {
		t.Fatalf(
			"decode conversion job list: %v",
			err,
		)
	}

	array, ok := raw.([]any)
	if !ok {
		t.Fatalf(
			"expected JSON array, got %T (%v)",
			raw,
			raw,
		)
	}

	if len(array) != 0 {
		t.Fatalf(
			"expected empty conversion job array, got %d entries",
			len(array),
		)
	}
}
