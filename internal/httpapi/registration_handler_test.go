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

type fakeUserRegistrar struct {
	called bool
	input  auth.RegisterInput
	user   dbgen.User
	err    error
}

func (f *fakeUserRegistrar) Register(
	_ context.Context,
	input auth.RegisterInput,
) (dbgen.User, error) {
	f.called = true
	f.input = input

	return f.user, f.err
}

func TestRegisterUserCreatesUser(t *testing.T) {
	userID := googleuuid.New()

	createdAt := time.Date(
		2026,
		time.September,
		9,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	updatedAt := createdAt.Add(time.Minute)

	registrar := &fakeUserRegistrar{
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
			registrar,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/register",
		strings.NewReader(
			`{"email":"Person@Example.com","displayName":"Person","password":"a sufficiently long password"}`,
		),
	)
	request.Header.Set(
		"Content-Type",
		"application/json",
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.Code,
		)
	}

	if !registrar.called {
		t.Fatal("expected registrar to be called")
	}

	if registrar.input.Email != "Person@Example.com" {
		t.Fatalf(
			"expected email %q, got %q",
			"Person@Example.com",
			registrar.input.Email,
		)
	}

	if registrar.input.DisplayName != "Person" {
		t.Fatalf(
			"expected display name %q, got %q",
			"Person",
			registrar.input.DisplayName,
		)
	}

	if registrar.input.Password != "a sufficiently long password" {
		t.Fatal("expected password to be passed to registrar unchanged")
	}

	if response.Header().Get("Set-Cookie") != "" {
		t.Fatal("expected registration not to create a session cookie")
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

func TestRegisterUserRequiresBody(t *testing.T) {
	server := NewServer(
		fakeDatabase{},
		nil,
		&fakeUserRegistrar{},
	)

	response, err := server.RegisterUser(
		context.Background(),
		api.RegisterUserRequestObject{},
	)
	if err != nil {
		t.Fatalf(
			"RegisterUser() error = %v",
			err,
		)
	}

	body, ok := response.(api.RegisterUser400JSONResponse)
	if !ok {
		t.Fatalf(
			"response type = %T, want RegisterUser400JSONResponse",
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

func TestRegisterUserMapsValidationErrors(
	t *testing.T,
) {
	testCases := []struct {
		name    string
		err     error
		message string
	}{
		{
			name:    "email required",
			err:     auth.ErrEmailRequired,
			message: "email is required",
		},
		{
			name:    "display name required",
			err:     auth.ErrDisplayNameRequired,
			message: "display name is required",
		},
		{
			name:    "invalid UTF-8",
			err:     auth.ErrPasswordInvalidUTF8,
			message: "password contains invalid UTF-8",
		},
		{
			name:    "password too short",
			err:     auth.ErrPasswordTooShort,
			message: "password must contain at least 15 characters",
		},
		{
			name:    "password too long",
			err:     auth.ErrPasswordTooLong,
			message: "password must contain at most 128 characters",
		},
		{
			name:    "password blocked",
			err:     auth.ErrPasswordBlocked,
			message: "password is too common, expected, or compromised",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			registrar := &fakeUserRegistrar{
				err: testCase.err,
			}

			server := NewServer(
				fakeDatabase{},
				nil,
				registrar,
			)

			response, err := server.RegisterUser(
				context.Background(),
				api.RegisterUserRequestObject{
					Body: &api.RegisterUserJSONRequestBody{
						Email:       "person@example.com",
						DisplayName: "Person",
						Password:    "valid password phrase",
					},
				},
			)
			if err != nil {
				t.Fatalf(
					"RegisterUser() error = %v",
					err,
				)
			}

			body, ok := response.(api.RegisterUser400JSONResponse)
			if !ok {
				t.Fatalf(
					"response type = %T, want RegisterUser400JSONResponse",
					response,
				)
			}

			if body.Error != testCase.message {
				t.Fatalf(
					"error = %q, want %q",
					body.Error,
					testCase.message,
				)
			}
		})
	}
}

func TestRegisterUserMapsEmailConflict(t *testing.T) {
	registrar := &fakeUserRegistrar{
		err: auth.ErrEmailAlreadyRegistered,
	}

	server := NewServer(
		fakeDatabase{},
		nil,
		registrar,
	)

	response, err := server.RegisterUser(
		context.Background(),
		api.RegisterUserRequestObject{
			Body: &api.RegisterUserJSONRequestBody{
				Email:       "person@example.com",
				DisplayName: "Person",
				Password:    "valid password phrase",
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"RegisterUser() error = %v",
			err,
		)
	}

	body, ok := response.(api.RegisterUser409JSONResponse)
	if !ok {
		t.Fatalf(
			"response type = %T, want RegisterUser409JSONResponse",
			response,
		)
	}

	if body.Error != "email is already registered" {
		t.Fatalf(
			"error = %q, want %q",
			body.Error,
			"email is already registered",
		)
	}
}

func TestRegisterUserMapsUnexpectedError(t *testing.T) {
	registrar := &fakeUserRegistrar{
		err: errors.New("database unavailable"),
	}

	server := NewServer(
		fakeDatabase{},
		nil,
		registrar,
	)

	response, err := server.RegisterUser(
		context.Background(),
		api.RegisterUserRequestObject{
			Body: &api.RegisterUserJSONRequestBody{
				Email:       "person@example.com",
				DisplayName: "Person",
				Password:    "valid password phrase",
			},
		},
	)
	if err != nil {
		t.Fatalf(
			"RegisterUser() error = %v",
			err,
		)
	}

	body, ok := response.(api.RegisterUser500JSONResponse)
	if !ok {
		t.Fatalf(
			"response type = %T, want RegisterUser500JSONResponse",
			response,
		)
	}

	if body.Error != "unable to register user" {
		t.Fatalf(
			"error = %q, want %q",
			body.Error,
			"unable to register user",
		)
	}
}

func TestRegisterUserRejectsCrossOriginBrowserRequest(
	t *testing.T,
) {
	registrar := &fakeUserRegistrar{}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			registrar,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/register",
		strings.NewReader(
			`{"email":"person@example.com","displayName":"Person","password":"valid password phrase"}`,
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

	if registrar.called {
		t.Fatal("expected registrar not to be called")
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "cross-origin request denied" {
		t.Fatalf(
			"expected cross-origin error, got %q",
			body.Error,
		)
	}
}
