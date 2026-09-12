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
	googleuuid "github.com/google/uuid"
	apiuuid "uuid"
)

type fakeUserAuthenticator struct {
	called bool
	input  auth.LoginInput
	result auth.LoginResult
	err    error
}

func (f *fakeUserAuthenticator) Login(
	_ context.Context,
	input auth.LoginInput,
) (auth.LoginResult, error) {
	f.called = true
	f.input = input

	return f.result, f.err
}

func TestLoginUserCreatesSessionCookie(t *testing.T) {
	userID := googleuuid.New()

	createdAt := time.Date(
		2026,
		time.September,
		11,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	updatedAt := createdAt.Add(time.Minute)
	expiresAt := createdAt.Add(7 * 24 * time.Hour)

	authenticator := &fakeUserAuthenticator{
		result: auth.LoginResult{
			User: dbgen.User{
				ID:          userID,
				Email:       "person@example.com",
				DisplayName: "Person",
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
			},
			Session: auth.CreatedSession{
				Token: "session-token",
				Session: auth.Session{
					ID:        googleuuid.New(),
					UserID:    userID,
					CreatedAt: createdAt,
					ExpiresAt: expiresAt,
				},
			},
		},
	}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			authenticator,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/login",
		strings.NewReader(
			`{"email":"Person@Example.com","password":"submitted password"}`,
		),
	)
	request.Header.Set(
		"Content-Type",
		"application/json",
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

	if !authenticator.called {
		t.Fatal("expected authenticator to be called")
	}

	if authenticator.input.Email != "Person@Example.com" {
		t.Fatalf(
			"expected email %q, got %q",
			"Person@Example.com",
			authenticator.input.Email,
		)
	}

	if authenticator.input.Password != "submitted password" {
		t.Fatal(
			"expected password to be passed to authenticator unchanged",
		)
	}

	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf(
			"expected 1 response cookie, got %d",
			len(cookies),
		)
	}

	cookie := cookies[0]

	if cookie.Name != sessionCookieName {
		t.Fatalf(
			"expected cookie name %q, got %q",
			sessionCookieName,
			cookie.Name,
		)
	}

	if cookie.Value != "session-token" {
		t.Fatalf(
			"expected cookie value %q, got %q",
			"session-token",
			cookie.Value,
		)
	}

	if cookie.Path != "/" {
		t.Fatalf(
			"expected cookie path %q, got %q",
			"/",
			cookie.Path,
		)
	}

	if !cookie.Expires.Equal(expiresAt) {
		t.Fatalf(
			"expected cookie expiry %s, got %s",
			expiresAt,
			cookie.Expires,
		)
	}

	if !cookie.HttpOnly {
		t.Fatal("expected cookie to be HttpOnly")
	}

	if !cookie.Secure {
		t.Fatal("expected cookie to be Secure")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf(
			"expected SameSite %d, got %d",
			http.SameSiteLaxMode,
			cookie.SameSite,
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
			"expected createdAt %s, got %s",
			createdAt,
			body.CreatedAt,
		)
	}

	if !body.UpdatedAt.Equal(updatedAt) {
		t.Fatalf(
			"expected updatedAt %s, got %s",
			updatedAt,
			body.UpdatedAt,
		)
	}
}

func TestLoginUserRequiresBody(t *testing.T) {
	server := NewServer(
		fakeDatabase{},
		nil,
		nil,
		&fakeUserAuthenticator{},
	)

	response, err := server.LoginUser(
		context.Background(),
		api.LoginUserRequestObject{},
	)
	if err != nil {
		t.Fatalf(
			"LoginUser() error = %v",
			err,
		)
	}

	body, ok := response.(api.LoginUser400JSONResponse)
	if !ok {
		t.Fatalf(
			"response type = %T, want LoginUser400JSONResponse",
			response,
		)
	}

	if body.Error != "request body is required" {
		t.Fatalf(
			"error = %q, want %q",
			body.Error,
			"request body is required",
		)
	}
}

func TestLoginUserMapsInvalidCredentials(t *testing.T) {
	authenticator := &fakeUserAuthenticator{
		err: auth.ErrInvalidCredentials,
	}

	server := NewServer(
		fakeDatabase{},
		nil,
		nil,
		authenticator,
	)

	response, err := server.LoginUser(
		context.Background(),
		api.LoginUserRequestObject{
			Body: &api.LoginUserJSONRequestBody{
				Email:    "person@example.com",
				Password: "wrong password",
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"LoginUser() error = %v",
			err,
		)
	}

	body, ok := response.(api.LoginUser401JSONResponse)
	if !ok {
		t.Fatalf(
			"response type = %T, want LoginUser401JSONResponse",
			response,
		)
	}

	if body.Error != "invalid email or password" {
		t.Fatalf(
			"error = %q, want %q",
			body.Error,
			"invalid email or password",
		)
	}
}

func TestLoginUserMapsUnexpectedError(t *testing.T) {
	authenticator := &fakeUserAuthenticator{
		err: errors.New("session store unavailable"),
	}

	server := NewServer(
		fakeDatabase{},
		nil,
		nil,
		authenticator,
	)

	response, err := server.LoginUser(
		context.Background(),
		api.LoginUserRequestObject{
			Body: &api.LoginUserJSONRequestBody{
				Email:    "person@example.com",
				Password: "password",
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"LoginUser() error = %v",
			err,
		)
	}

	body, ok := response.(api.LoginUser500JSONResponse)
	if !ok {
		t.Fatalf(
			"response type = %T, want LoginUser500JSONResponse",
			response,
		)
	}

	if body.Error != "unable to log in" {
		t.Fatalf(
			"error = %q, want %q",
			body.Error,
			"unable to log in",
		)
	}
}

func TestLoginUserRejectsCrossOriginBrowserRequest(
	t *testing.T,
) {
	authenticator := &fakeUserAuthenticator{}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			authenticator,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/login",
		strings.NewReader(
			`{"email":"person@example.com","password":"password"}`,
		),
	)
	request.Header.Set(
		"Content-Type",
		"application/json",
	)
	request.Header.Set(
		"Sec-Fetch-Site",
		"cross-site",
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

	if authenticator.called {
		t.Fatal(
			"expected authenticator not to be called",
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

func TestLoginUserInvalidCredentialsDoNotSetCookie(
	t *testing.T,
) {
	authenticator := &fakeUserAuthenticator{
		err: auth.ErrInvalidCredentials,
	}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			authenticator,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/login",
		strings.NewReader(
			`{"email":"person@example.com","password":"wrong password"}`,
		),
	)
	request.Header.Set(
		"Content-Type",
		"application/json",
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

	if !authenticator.called {
		t.Fatal("expected authenticator to be called")
	}

	if values := response.Header().Values("Set-Cookie"); len(values) != 0 {
		t.Fatalf(
			"expected no Set-Cookie header, got %v",
			values,
		)
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "invalid email or password" {
		t.Fatalf(
			"expected invalid credentials error, got %q",
			body.Error,
		)
	}
}
