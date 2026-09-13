package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/auth"
)

type fakeSessionRevoker struct {
	called bool
	token  string
	err    error
}

func (f *fakeSessionRevoker) Revoke(
	_ context.Context,
	token string,
) error {
	f.called = true
	f.token = token

	return f.err
}

func TestLogoutUserRevokesSessionAndExpiresCookie(
	t *testing.T,
) {
	revoker := &fakeSessionRevoker{}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			nil,
			revoker,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/logout",
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNoContent,
			response.Code,
		)
	}

	if !revoker.called {
		t.Fatal("expected session revoker to be called")
	}

	if revoker.token != "session-token" {
		t.Fatalf(
			"expected token %q, got %q",
			"session-token",
			revoker.token,
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

	if cookie.Value != "" {
		t.Fatalf(
			"expected empty cookie value, got %q",
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

	if cookie.MaxAge != -1 {
		t.Fatalf(
			"expected MaxAge -1, got %d",
			cookie.MaxAge,
		)
	}

	expectedExpiry := time.Unix(1, 0).UTC()
	if !cookie.Expires.Equal(expectedExpiry) {
		t.Fatalf(
			"expected expiry %s, got %s",
			expectedExpiry,
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

	if response.Body.Len() != 0 {
		t.Fatalf(
			"expected empty response body, got %q",
			response.Body.String(),
		)
	}
}

func TestLogoutUserWithoutCookieIsIdempotent(
	t *testing.T,
) {
	revoker := &fakeSessionRevoker{}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			nil,
			revoker,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/logout",
		nil,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNoContent,
			response.Code,
		)
	}

	if revoker.called {
		t.Fatal("expected session revoker not to be called")
	}

	if values := response.Header().Values("Set-Cookie"); len(values) != 1 {
		t.Fatalf(
			"expected 1 Set-Cookie header, got %v",
			values,
		)
	}
}

func TestLogoutUserTreatsInvalidTokenAsLoggedOut(
	t *testing.T,
) {
	revoker := &fakeSessionRevoker{
		err: auth.ErrInvalidSessionToken,
	}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			nil,
			revoker,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/logout",
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "malformed-token",
	})

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNoContent,
			response.Code,
		)
	}

	if !revoker.called {
		t.Fatal("expected session revoker to be called")
	}

	if values := response.Header().Values("Set-Cookie"); len(values) != 1 {
		t.Fatalf(
			"expected 1 Set-Cookie header, got %v",
			values,
		)
	}
}

func TestLogoutUserMapsRevocationFailure(
	t *testing.T,
) {
	revokeErr := errors.New("session store unavailable")

	revoker := &fakeSessionRevoker{
		err: revokeErr,
	}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			nil,
			revoker,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/logout",
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

	if !revoker.called {
		t.Fatal("expected session revoker to be called")
	}

	if values := response.Header().Values("Set-Cookie"); len(values) != 0 {
		t.Fatalf(
			"expected no Set-Cookie header, got %v",
			values,
		)
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "unable to log out" {
		t.Fatalf(
			"expected logout error, got %q",
			body.Error,
		)
	}
}

func TestLogoutUserRejectsCrossOriginBrowserRequest(
	t *testing.T,
) {
	revoker := &fakeSessionRevoker{}

	handler := NewHandler(
		NewServer(
			fakeDatabase{},
			nil,
			nil,
			nil,
			revoker,
		),
		nil,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/auth/logout",
		nil,
	)
	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})
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

	if revoker.called {
		t.Fatal("expected session revoker not to be called")
	}

	body := decodeErrorResponse(t, response)

	if body.Error != "cross-origin request denied" {
		t.Fatalf(
			"expected cross-origin error, got %q",
			body.Error,
		)
	}
}
