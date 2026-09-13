package httpapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/auth"
)

const sessionCookieName = "__Host-3default_session"

type SessionResolver interface {
	Resolve(
		ctx context.Context,
		token string,
	) (auth.Session, error)
}

type sessionContextKey struct{}

type sessionTokenContextKey struct{}

type sessionContextState struct {
	session    auth.Session
	hasSession bool
	err        error
}

func NewSessionCookie(
	token string,
	expiresAt time.Time,
) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

func NewExpiredSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(1, 0).UTC(),
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

func SessionFromContext(
	ctx context.Context,
) (auth.Session, bool) {
	state, ok := ctx.Value(sessionContextKey{}).(sessionContextState)
	if !ok || !state.hasSession {
		return auth.Session{}, false
	}

	return state.session, true
}

func SessionResolutionError(ctx context.Context) error {
	state, ok := ctx.Value(sessionContextKey{}).(sessionContextState)
	if !ok {
		return nil
	}

	return state.err
}

func SessionTokenFromContext(
	ctx context.Context,
) (string, bool) {
	token, ok := ctx.Value(sessionTokenContextKey{}).(string)
	if !ok {
		return "", false
	}

	return token, true
}

func SessionTokenContextMiddleware(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		ctx := context.WithValue(
			r.Context(),
			sessionTokenContextKey{},
			cookie.Value,
		)

		next.ServeHTTP(
			w,
			r.WithContext(ctx),
		)
	})
}

func LogoutSessionTokenMiddleware(
	next http.Handler,
) http.Handler {
	protected := SessionTokenContextMiddleware(next)

	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		if r.Method == http.MethodPost &&
			r.URL.Path == "/api/auth/logout" {
			protected.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func SessionContextMiddleware(
	resolver SessionResolver,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			cookie, err := r.Cookie(sessionCookieName)
			if errors.Is(err, http.ErrNoCookie) {
				next.ServeHTTP(w, r)
				return
			}
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			session, err := resolver.Resolve(
				r.Context(),
				cookie.Value,
			)

			if errors.Is(err, auth.ErrInvalidSessionToken) ||
				errors.Is(err, auth.ErrSessionNotFound) {
				next.ServeHTTP(w, r)
				return
			}

			state := sessionContextState{}

			if err != nil {
				state.err = err
			} else {
				state.session = session
				state.hasSession = true
			}

			ctx := context.WithValue(
				r.Context(),
				sessionContextKey{},
				state,
			)

			next.ServeHTTP(
				w,
				r.WithContext(ctx),
			)
		})
	}
}

func AuthenticatedSessionContextMiddleware(
	resolver SessionResolver,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		protected := SessionContextMiddleware(resolver)(next)

		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if r.Method == http.MethodPost &&
				r.URL.Path == "/api/projects" {
				protected.ServeHTTP(w, r)
				return
			}

			if r.Method == http.MethodGet &&
				r.URL.Path == "/api/auth/me" {
				protected.ServeHTTP(w, r)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

var _ SessionResolver = (*auth.SessionService)(nil)
