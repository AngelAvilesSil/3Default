package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const sessionTokenSize = 32

var (
	ErrSessionUserRequired    = errors.New("session user is required")
	ErrInvalidSessionToken    = errors.New("invalid session token")
	ErrSessionNotFound        = errors.New("session not found")
	ErrInvalidSessionLifetime = errors.New("session lifetime must be positive")
)

type SessionStore interface {
	CreateSession(
		ctx context.Context,
		arg dbgen.CreateSessionParams,
	) (dbgen.Session, error)

	GetActiveSessionByTokenHash(
		ctx context.Context,
		tokenHash []byte,
	) (dbgen.GetActiveSessionByTokenHashRow, error)

	DeleteSession(
		ctx context.Context,
		tokenHash []byte,
	) error
}

type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	CreatedAt time.Time
	ExpiresAt time.Time
}

type CreatedSession struct {
	Token   string
	Session Session
}

type SessionService struct {
	sessions SessionStore
	lifetime time.Duration
	entropy  io.Reader
	now      func() time.Time
}

func NewSessionService(
	sessions SessionStore,
	lifetime time.Duration,
) (*SessionService, error) {
	if lifetime <= 0 {
		return nil, ErrInvalidSessionLifetime
	}

	return &SessionService{
		sessions: sessions,
		lifetime: lifetime,
		entropy:  rand.Reader,
		now:      time.Now,
	}, nil
}

func (s *SessionService) Create(
	ctx context.Context,
	userID uuid.UUID,
) (CreatedSession, error) {
	if userID == uuid.Nil {
		return CreatedSession{}, ErrSessionUserRequired
	}

	rawToken := make([]byte, sessionTokenSize)

	if _, err := io.ReadFull(s.entropy, rawToken); err != nil {
		return CreatedSession{}, fmt.Errorf(
			"generate session token: %w",
			err,
		)
	}

	token := base64.RawURLEncoding.EncodeToString(rawToken)
	tokenHash := sha256.Sum256(rawToken)

	session, err := s.sessions.CreateSession(
		ctx,
		dbgen.CreateSessionParams{
			UserID:    userID,
			TokenHash: tokenHash[:],
			ExpiresAt: s.now().Add(s.lifetime),
		},
	)
	if err != nil {
		return CreatedSession{}, fmt.Errorf(
			"create session: %w",
			err,
		)
	}

	return CreatedSession{
		Token: token,
		Session: Session{
			ID:        session.ID,
			UserID:    session.UserID,
			CreatedAt: session.CreatedAt,
			ExpiresAt: session.ExpiresAt,
		},
	}, nil
}

func (s *SessionService) Resolve(
	ctx context.Context,
	token string,
) (Session, error) {
	tokenHash, err := hashSessionToken(token)
	if err != nil {
		return Session{}, err
	}

	session, err := s.sessions.GetActiveSessionByTokenHash(
		ctx,
		tokenHash,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, fmt.Errorf(
			"resolve session: %w",
			err,
		)
	}

	return Session{
		ID:        session.ID,
		UserID:    session.UserID,
		CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt,
	}, nil
}

func (s *SessionService) Revoke(
	ctx context.Context,
	token string,
) error {
	tokenHash, err := hashSessionToken(token)
	if err != nil {
		return err
	}

	if err := s.sessions.DeleteSession(ctx, tokenHash); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	return nil
}

func hashSessionToken(token string) ([]byte, error) {
	rawToken, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(rawToken) != sessionTokenSize {
		return nil, ErrInvalidSessionToken
	}

	tokenHash := sha256.Sum256(rawToken)

	return tokenHash[:], nil
}

var _ SessionStore = (*dbgen.Queries)(nil)
