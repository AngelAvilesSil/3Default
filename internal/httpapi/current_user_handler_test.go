package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	apiuuid "uuid"

	api "github.com/AngelAvilesSil/3Default/internal/api"
	"github.com/AngelAvilesSil/3Default/internal/auth"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	googleuuid "github.com/google/uuid"
)

type fakeCurrentUserReader struct {
	called bool
	userID googleuuid.UUID
	user   dbgen.User
	err    error
}

func (f *fakeCurrentUserReader) GetUserByID(
	_ context.Context,
	userID googleuuid.UUID,
) (dbgen.User, error) {
	f.called = true
	f.userID = userID

	return f.user, f.err
}

func TestGetCurrentUserReturnsAuthenticatedUser(
	t *testing.T,
) {
	userID := googleuuid.New()

	createdAt := time.Date(
		2026,
		time.September,
		12,
		12,
		0,
		0,
		0,
		time.UTC,
	)
	updatedAt := createdAt.Add(time.Minute)

	resolver := &fakeSessionResolver{
		session: auth.Session{
			ID:     googleuuid.New(),
			UserID: userID,
		},
	}

	reader := &fakeCurrentUserReader{
		user: dbgen.User{
			ID:          userID,
			Email:       "person@example.com",
			DisplayName: "Person",
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
		},
	}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			nil,
			nil,
			reader,
		),
		resolver,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/auth/me",
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
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

	if !reader.called {
		t.Fatal("expected current user reader to be called")
	}

	if reader.userID != userID {
		t.Fatalf(
			"expected user ID %s, got %s",
			userID,
			reader.userID,
		)
	}

	var body api.UserResponse

	if err := json.NewDecoder(
		response.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode response: %v",
			err,
		)
	}

	if body.Id != apiuuid.UUID(userID) {
		t.Fatalf(
			"expected user ID %s, got %s",
			userID,
			body.Id,
		)
	}

	if body.Email != "person@example.com" {
		t.Fatalf(
			"expected email %q, got %q",
			"person@example.com",
			body.Email,
		)
	}

	if body.DisplayName != "Person" {
		t.Fatalf(
			"expected display name %q, got %q",
			"Person",
			body.DisplayName,
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

func TestGetCurrentUserRequiresAuthentication(
	t *testing.T,
) {
	resolver := &fakeSessionResolver{}
	reader := &fakeCurrentUserReader{}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			nil,
			nil,
			reader,
		),
		resolver,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/auth/me",
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

	if resolver.called {
		t.Fatal("expected session resolver not to be called")
	}

	if reader.called {
		t.Fatal("expected current user reader not to be called")
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "authentication required" {
		t.Fatalf(
			"expected authentication error, got %q",
			body.Error,
		)
	}
}

func TestGetCurrentUserTreatsMissingSessionAsUnauthenticated(
	t *testing.T,
) {
	resolver := &fakeSessionResolver{
		err: auth.ErrSessionNotFound,
	}
	reader := &fakeCurrentUserReader{}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			nil,
			nil,
			reader,
		),
		resolver,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/auth/me",
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusUnauthorized,
			response.Code,
		)
	}

	if !resolver.called {
		t.Fatal("expected session resolver to be called")
	}

	if reader.called {
		t.Fatal("expected current user reader not to be called")
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "authentication required" {
		t.Fatalf(
			"expected authentication error, got %q",
			body.Error,
		)
	}
}

func TestGetCurrentUserRejectsSessionResolutionFailure(
	t *testing.T,
) {
	resolutionErr := errors.New("database unavailable")

	resolver := &fakeSessionResolver{
		err: resolutionErr,
	}
	reader := &fakeCurrentUserReader{}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			nil,
			nil,
			reader,
		),
		resolver,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/auth/me",
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
		t.Fatal("expected session resolver to be called")
	}

	if reader.called {
		t.Fatal("expected current user reader not to be called")
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "unable to authenticate request" {
		t.Fatalf(
			"expected authentication failure error, got %q",
			body.Error,
		)
	}
}

func TestGetCurrentUserMapsUserLookupFailure(
	t *testing.T,
) {
	userID := googleuuid.New()

	resolver := &fakeSessionResolver{
		session: auth.Session{
			ID:     googleuuid.New(),
			UserID: userID,
		},
	}

	reader := &fakeCurrentUserReader{
		err: errors.New("user lookup failed"),
	}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			nil,
			nil,
			reader,
		),
		resolver,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/api/auth/me",
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

	if !reader.called {
		t.Fatal("expected current user reader to be called")
	}

	if reader.userID != userID {
		t.Fatalf(
			"expected user ID %s, got %s",
			userID,
			reader.userID,
		)
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "unable to get current user" {
		t.Fatalf(
			"expected current user lookup error, got %q",
			body.Error,
		)
	}
}
