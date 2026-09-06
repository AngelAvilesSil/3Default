package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/auth"
	"github.com/google/uuid"
)

type fakeSessionResolver struct {
	called  bool
	token   string
	session auth.Session
	err     error
}

func (f *fakeSessionResolver) Resolve(
	_ context.Context,
	token string,
) (auth.Session, error) {
	f.called = true
	f.token = token

	return f.session, f.err
}

func TestNewSessionCookieUsesSecureHostOnlySettings(
	t *testing.T,
) {
	expiresAt := time.Date(
		2026,
		time.September,
		6,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	cookie := NewSessionCookie(
		"session-token",
		expiresAt,
	)

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

	if cookie.Domain != "" {
		t.Fatalf(
			"expected no cookie domain, got %q",
			cookie.Domain,
		)
	}

	if !cookie.HttpOnly {
		t.Fatal("expected HttpOnly cookie")
	}

	if !cookie.Secure {
		t.Fatal("expected Secure cookie")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf(
			"expected SameSite=Lax, got %v",
			cookie.SameSite,
		)
	}

	if !cookie.Expires.Equal(expiresAt) {
		t.Fatalf(
			"expected expiry %s, got %s",
			expiresAt,
			cookie.Expires,
		)
	}
}

func TestNewExpiredSessionCookieDeletesCookie(
	t *testing.T,
) {
	cookie := NewExpiredSessionCookie()

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

	if cookie.MaxAge != -1 {
		t.Fatalf(
			"expected MaxAge -1, got %d",
			cookie.MaxAge,
		)
	}

	if cookie.Path != "/" {
		t.Fatalf(
			"expected cookie path %q, got %q",
			"/",
			cookie.Path,
		)
	}

	if !cookie.HttpOnly {
		t.Fatal("expected HttpOnly cookie")
	}

	if !cookie.Secure {
		t.Fatal("expected Secure cookie")
	}

	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf(
			"expected SameSite=Lax, got %v",
			cookie.SameSite,
		)
	}
}

func TestSessionContextMiddlewareAllowsRequestWithoutCookie(
	t *testing.T,
) {
	resolver := &fakeSessionResolver{}

	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if _, ok := SessionFromContext(r.Context()); ok {
			t.Fatal("expected no authenticated session")
		}

		if err := SessionResolutionError(r.Context()); err != nil {
			t.Fatalf(
				"expected no session resolution error, got %v",
				err,
			)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	handler := SessionContextMiddleware(resolver)(next)

	request := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	)

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if resolver.called {
		t.Fatal("expected resolver not to be called")
	}

	if response.Code != http.StatusNoContent {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNoContent,
			response.Code,
		)
	}
}

func TestSessionContextMiddlewareAddsResolvedSession(
	t *testing.T,
) {
	sessionID := uuid.New()
	userID := uuid.New()

	resolver := &fakeSessionResolver{
		session: auth.Session{
			ID:     sessionID,
			UserID: userID,
		},
	}

	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		session, ok := SessionFromContext(r.Context())
		if !ok {
			t.Fatal("expected authenticated session")
		}

		if session.ID != sessionID {
			t.Fatalf(
				"expected session ID %s, got %s",
				sessionID,
				session.ID,
			)
		}

		if session.UserID != userID {
			t.Fatalf(
				"expected user ID %s, got %s",
				userID,
				session.UserID,
			)
		}

		if err := SessionResolutionError(r.Context()); err != nil {
			t.Fatalf(
				"expected no session resolution error, got %v",
				err,
			)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	handler := SessionContextMiddleware(resolver)(next)

	request := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	)

	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if !resolver.called {
		t.Fatal("expected resolver to be called")
	}

	if resolver.token != "session-token" {
		t.Fatalf(
			"expected token %q, got %q",
			"session-token",
			resolver.token,
		)
	}
}

func TestSessionContextMiddlewareTreatsInvalidSessionAsUnauthenticated(
	t *testing.T,
) {
	testCases := []struct {
		name string
		err  error
	}{
		{
			name: "invalid token",
			err:  auth.ErrInvalidSessionToken,
		},
		{
			name: "session not found",
			err:  auth.ErrSessionNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			resolver := &fakeSessionResolver{
				err: testCase.err,
			}

			next := http.HandlerFunc(func(
				w http.ResponseWriter,
				r *http.Request,
			) {
				if _, ok := SessionFromContext(r.Context()); ok {
					t.Fatal("expected no authenticated session")
				}

				if err := SessionResolutionError(r.Context()); err != nil {
					t.Fatalf(
						"expected no resolution error, got %v",
						err,
					)
				}

				w.WriteHeader(http.StatusNoContent)
			})

			handler := SessionContextMiddleware(resolver)(next)

			request := httptest.NewRequest(
				http.MethodGet,
				"/",
				nil,
			)

			request.AddCookie(&http.Cookie{
				Name:  sessionCookieName,
				Value: "session-token",
			})

			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)
		})
	}
}

func TestSessionContextMiddlewarePreservesUnexpectedResolutionError(
	t *testing.T,
) {
	resolutionErr := errors.New("database unavailable")

	resolver := &fakeSessionResolver{
		err: resolutionErr,
	}

	next := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if _, ok := SessionFromContext(r.Context()); ok {
			t.Fatal("expected no authenticated session")
		}

		err := SessionResolutionError(r.Context())
		if !errors.Is(err, resolutionErr) {
			t.Fatalf(
				"expected resolution error, got %v",
				err,
			)
		}

		w.WriteHeader(http.StatusNoContent)
	})

	handler := SessionContextMiddleware(resolver)(next)

	request := httptest.NewRequest(
		http.MethodGet,
		"/",
		nil,
	)

	request.AddCookie(&http.Cookie{
		Name:  sessionCookieName,
		Value: "session-token",
	})

	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)
}
