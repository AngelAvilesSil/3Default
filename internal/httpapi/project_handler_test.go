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
	"github.com/AngelAvilesSil/3Default/internal/auth"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/projects"
	"github.com/google/uuid"
)

type fakeProjectCreator struct {
	called  bool
	input   projects.CreateInput
	project dbgen.Project
	err     error
}

func (f *fakeProjectCreator) Create(
	_ context.Context,
	input projects.CreateInput,
) (dbgen.Project, error) {
	f.called = true
	f.input = input

	return f.project, f.err
}

func requestWithSession(
	request *http.Request,
	userID uuid.UUID,
) *http.Request {
	ctx := context.WithValue(
		request.Context(),
		sessionContextKey{},
		sessionContextState{
			session: auth.Session{
				UserID: userID,
			},
			hasSession: true,
		},
	)

	return request.WithContext(ctx)
}

func requestWithSessionResolutionError(
	request *http.Request,
	err error,
) *http.Request {
	ctx := context.WithValue(
		request.Context(),
		sessionContextKey{},
		sessionContextState{
			err: err,
		},
	)

	return request.WithContext(ctx)
}

func decodeErrorResponse(
	t *testing.T,
	response *httptest.ResponseRecorder,
) api.ErrorResponse {
	t.Helper()

	var body api.ErrorResponse

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}

	return body
}

func TestCreateProjectRequiresAuthentication(t *testing.T) {
	projectCreator := &fakeProjectCreator{}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			projectCreator,
			nil,
			nil,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects",
		strings.NewReader(`{"name":"Robot Gripper"}`),
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

	if projectCreator.called {
		t.Fatal("expected project creator not to be called")
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "authentication required" {
		t.Fatalf(
			"expected authentication error, got %q",
			body.Error,
		)
	}
}

func TestCreateProjectRejectsSessionResolutionFailure(
	t *testing.T,
) {
	projectCreator := &fakeProjectCreator{}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			projectCreator,
			nil,
			nil,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects",
		strings.NewReader(`{"name":"Robot Gripper"}`),
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

	if projectCreator.called {
		t.Fatal("expected project creator not to be called")
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "unable to authenticate request" {
		t.Fatalf(
			"expected authentication failure error, got %q",
			body.Error,
		)
	}
}

func TestCreateProjectMapsNameValidationError(t *testing.T) {
	userID := uuid.New()

	projectCreator := &fakeProjectCreator{
		err: projects.ErrNameRequired,
	}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			projectCreator,
			nil,
			nil,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects",
		strings.NewReader(`{"name":"   "}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request = requestWithSession(request, userID)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}

	if !projectCreator.called {
		t.Fatal("expected project creator to be called")
	}

	if projectCreator.input.OwnerUserID != userID {
		t.Fatalf(
			"expected owner %s, got %s",
			userID,
			projectCreator.input.OwnerUserID,
		)
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "project name is required" {
		t.Fatalf(
			"expected project name error, got %q",
			body.Error,
		)
	}
}

func TestCreateProjectMapsUnexpectedServiceError(t *testing.T) {
	projectCreator := &fakeProjectCreator{
		err: errors.New("database unavailable"),
	}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			projectCreator,
			nil,
			nil,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects",
		strings.NewReader(`{"name":"Robot Gripper"}`),
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

	if body.Error != "unable to create project" {
		t.Fatalf(
			"expected generic creation error, got %q",
			body.Error,
		)
	}
}

func TestCreateProjectRejectsCrossOriginBrowserRequest(
	t *testing.T,
) {
	resolver := &fakeSessionResolver{
		session: auth.Session{
			UserID: uuid.New(),
		},
	}

	projectCreator := &fakeProjectCreator{}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			projectCreator,
			nil,
			nil,
		),
		resolver,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects",
		strings.NewReader(`{"name":"Robot Gripper"}`),
	)

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Sec-Fetch-Site", "cross-site")

	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusForbidden,
			response.Code,
		)
	}

	if resolver.called {
		t.Fatal("expected session resolver not to be called")
	}

	if projectCreator.called {
		t.Fatal("expected project creator not to be called")
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "cross-origin request denied" {
		t.Fatalf(
			"expected cross-origin error, got %q",
			body.Error,
		)
	}
}

func TestCreateProjectResolvesSessionCookie(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()

	createdAt := time.Date(
		2026,
		time.September,
		6,
		3,
		30,
		0,
		0,
		time.UTC,
	)

	resolver := &fakeSessionResolver{
		session: auth.Session{
			UserID: userID,
		},
	}

	projectCreator := &fakeProjectCreator{
		project: dbgen.Project{
			ID:          projectID,
			OwnerUserID: userID,
			Name:        "Robot Gripper",
			Visibility:  "private",
			CreatedAt:   createdAt,
			UpdatedAt:   createdAt,
		},
	}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			projectCreator,
			nil,
			nil,
		),
		resolver,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects",
		strings.NewReader(`{"name":"Robot Gripper"}`),
	)

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Sec-Fetch-Site", "same-origin")

	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.Code,
		)
	}

	if !resolver.called {
		t.Fatal("expected session resolver to be called")
	}

	if resolver.token != "session-token" {
		t.Fatalf(
			"expected session token %q, got %q",
			"session-token",
			resolver.token,
		)
	}

	if !projectCreator.called {
		t.Fatal("expected project creator to be called")
	}

	if projectCreator.input.OwnerUserID != userID {
		t.Fatalf(
			"expected owner %s, got %s",
			userID,
			projectCreator.input.OwnerUserID,
		)
	}
}

func TestCreateProjectUsesAuthenticatedUserAsOwner(
	t *testing.T,
) {
	userID := uuid.New()
	projectID := uuid.New()

	description := "Mechanical gripper assembly"

	createdAt := time.Date(
		2026,
		time.September,
		5,
		20,
		0,
		0,
		0,
		time.UTC,
	)

	updatedAt := createdAt.Add(time.Minute)

	projectCreator := &fakeProjectCreator{
		project: dbgen.Project{
			ID:          projectID,
			OwnerUserID: userID,
			Name:        "Robot Gripper",
			Description: &description,
			Visibility:  "private",
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		},
	}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			projectCreator,
			nil,
			nil,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects",
		strings.NewReader(
			`{"name":"Robot Gripper","description":"Mechanical gripper assembly"}`,
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

	if !projectCreator.called {
		t.Fatal("expected project creator to be called")
	}

	if projectCreator.input.OwnerUserID != userID {
		t.Fatalf(
			"expected owner %s, got %s",
			userID,
			projectCreator.input.OwnerUserID,
		)
	}

	if projectCreator.input.Name != "Robot Gripper" {
		t.Fatalf(
			"expected project name %q, got %q",
			"Robot Gripper",
			projectCreator.input.Name,
		)
	}

	if projectCreator.input.Description == nil {
		t.Fatal("expected project description")
	}

	if *projectCreator.input.Description !=
		"Mechanical gripper assembly" {
		t.Fatalf(
			"unexpected project description %q",
			*projectCreator.input.Description,
		)
	}

	var body struct {
		ID          string    `json:"id"`
		Name        string    `json:"name"`
		Description *string   `json:"description"`
		Visibility  string    `json:"visibility"`
		CreatedAt   time.Time `json:"createdAt"`
		UpdatedAt   time.Time `json:"updatedAt"`
	}

	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode project response: %v", err)
	}

	if body.ID != projectID.String() {
		t.Fatalf(
			"expected project ID %q, got %q",
			projectID.String(),
			body.ID,
		)
	}

	if body.Name != "Robot Gripper" {
		t.Fatalf(
			"expected project name %q, got %q",
			"Robot Gripper",
			body.Name,
		)
	}

	if body.Description == nil ||
		*body.Description != description {
		t.Fatalf(
			"expected description %q, got %v",
			description,
			body.Description,
		)
	}

	if body.Visibility != "private" {
		t.Fatalf(
			"expected visibility %q, got %q",
			"private",
			body.Visibility,
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

func TestCreateProjectRejectsMalformedJSON(t *testing.T) {
	projectCreator := &fakeProjectCreator{}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			projectCreator,
			nil,
			nil,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/projects",
		strings.NewReader(`{"name":`),
	)
	request.Header.Set("Content-Type", "application/json")

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}

	if projectCreator.called {
		t.Fatal("expected project creator not to be called")
	}

	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf(
			"expected application/json content type, got %q",
			contentType,
		)
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "invalid request" {
		t.Fatalf(
			"expected invalid request error, got %q",
			body.Error,
		)
	}
}
