package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type fakeSessionStore struct {
	createCalled bool
	createParams dbgen.CreateSessionParams
	createResult dbgen.Session
	createErr    error

	getCalled bool
	getHash   []byte
	getResult dbgen.GetActiveSessionByTokenHashRow
	getErr    error

	deleteCalled bool
	deleteHash   []byte
	deleteErr    error
}

type failingReader struct {
	err error
}

func (r failingReader) Read(_ []byte) (int, error) {
	return 0, r.err
}

func (f *fakeSessionStore) CreateSession(
	_ context.Context,
	arg dbgen.CreateSessionParams,
) (dbgen.Session, error) {
	f.createCalled = true
	f.createParams = arg

	return f.createResult, f.createErr
}

func (f *fakeSessionStore) GetActiveSessionByTokenHash(
	_ context.Context,
	tokenHash []byte,
) (dbgen.GetActiveSessionByTokenHashRow, error) {
	f.getCalled = true
	f.getHash = append([]byte(nil), tokenHash...)

	return f.getResult, f.getErr
}

func (f *fakeSessionStore) DeleteSession(
	_ context.Context,
	tokenHash []byte,
) error {
	f.deleteCalled = true
	f.deleteHash = append([]byte(nil), tokenHash...)

	return f.deleteErr
}

func TestCreateGeneratesTokenAndStoresOnlyHash(t *testing.T) {
	userID := uuid.New()
	sessionID := uuid.New()

	now := time.Date(
		2026,
		time.September,
		4,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	lifetime := 24 * time.Hour

	rawToken := make([]byte, sessionTokenSize)
	for i := range rawToken {
		rawToken[i] = byte(i)
	}

	store := &fakeSessionStore{
		createResult: dbgen.Session{
			ID:        sessionID,
			UserID:    userID,
			CreatedAt: now,
			ExpiresAt: now.Add(lifetime),
		},
	}

	service, err := NewSessionService(store, lifetime)
	if err != nil {
		t.Fatalf("create session service: %v", err)
	}

	service.entropy = bytes.NewReader(rawToken)
	service.now = func() time.Time {
		return now
	}

	created, err := service.Create(
		context.Background(),
		userID,
	)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	expectedToken := base64.RawURLEncoding.EncodeToString(rawToken)

	if created.Token != expectedToken {
		t.Fatalf(
			"expected token %q, got %q",
			expectedToken,
			created.Token,
		)
	}

	expectedHash := sha256.Sum256(rawToken)

	if !bytes.Equal(
		store.createParams.TokenHash,
		expectedHash[:],
	) {
		t.Fatal("expected SHA-256 token hash to be stored")
	}

	if bytes.Equal(
		store.createParams.TokenHash,
		rawToken,
	) {
		t.Fatal("expected raw token not to be stored")
	}

	if store.createParams.UserID != userID {
		t.Fatalf(
			"expected user ID %s, got %s",
			userID,
			store.createParams.UserID,
		)
	}

	if !store.createParams.ExpiresAt.Equal(
		now.Add(lifetime),
	) {
		t.Fatalf(
			"expected expiry %s, got %s",
			now.Add(lifetime),
			store.createParams.ExpiresAt,
		)
	}

	if created.Session.ID != sessionID {
		t.Fatalf(
			"expected session ID %s, got %s",
			sessionID,
			created.Session.ID,
		)
	}
}

func TestCreateRejectsMissingUser(t *testing.T) {
	store := &fakeSessionStore{}

	service, err := NewSessionService(
		store,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create session service: %v", err)
	}

	_, err = service.Create(
		context.Background(),
		uuid.Nil,
	)

	if !errors.Is(err, ErrSessionUserRequired) {
		t.Fatalf(
			"expected ErrSessionUserRequired, got %v",
			err,
		)
	}

	if store.createCalled {
		t.Fatal("expected session store not to be called")
	}
}

func TestCreateFailsWhenEntropySourceFails(t *testing.T) {
	entropyErr := errors.New("entropy unavailable")
	store := &fakeSessionStore{}

	service, err := NewSessionService(
		store,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create session service: %v", err)
	}

	service.entropy = failingReader{
		err: entropyErr,
	}

	_, err = service.Create(
		context.Background(),
		uuid.New(),
	)

	if !errors.Is(err, entropyErr) {
		t.Fatalf(
			"expected entropy error, got %v",
			err,
		)
	}

	if store.createCalled {
		t.Fatal("expected session store not to be called")
	}
}

func TestResolveHashesTokenAndReturnsSession(t *testing.T) {
	rawToken := bytes.Repeat(
		[]byte{0x42},
		sessionTokenSize,
	)

	token := base64.RawURLEncoding.EncodeToString(rawToken)
	expectedHash := sha256.Sum256(rawToken)

	sessionID := uuid.New()
	userID := uuid.New()
	now := time.Now()

	store := &fakeSessionStore{
		getResult: dbgen.GetActiveSessionByTokenHashRow{
			ID:        sessionID,
			UserID:    userID,
			CreatedAt: now,
			ExpiresAt: now.Add(time.Hour),
		},
	}

	service, err := NewSessionService(
		store,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create session service: %v", err)
	}

	session, err := service.Resolve(
		context.Background(),
		token,
	)
	if err != nil {
		t.Fatalf("resolve session: %v", err)
	}

	if !bytes.Equal(store.getHash, expectedHash[:]) {
		t.Fatal("expected token to be hashed before lookup")
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
}

func TestResolveRejectsMalformedTokenWithoutDatabaseLookup(
	t *testing.T,
) {
	store := &fakeSessionStore{}

	service, err := NewSessionService(
		store,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create session service: %v", err)
	}

	_, err = service.Resolve(
		context.Background(),
		"not-a-valid-session-token",
	)

	if !errors.Is(err, ErrInvalidSessionToken) {
		t.Fatalf(
			"expected ErrInvalidSessionToken, got %v",
			err,
		)
	}

	if store.getCalled {
		t.Fatal("expected session store not to be called")
	}
}

func TestResolveMapsMissingSession(t *testing.T) {
	rawToken := bytes.Repeat(
		[]byte{0x24},
		sessionTokenSize,
	)

	token := base64.RawURLEncoding.EncodeToString(rawToken)

	store := &fakeSessionStore{
		getErr: pgx.ErrNoRows,
	}

	service, err := NewSessionService(
		store,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create session service: %v", err)
	}

	_, err = service.Resolve(
		context.Background(),
		token,
	)

	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf(
			"expected ErrSessionNotFound, got %v",
			err,
		)
	}
}

func TestRevokeHashesTokenBeforeDeletion(t *testing.T) {
	rawToken := bytes.Repeat(
		[]byte{0x7a},
		sessionTokenSize,
	)

	token := base64.RawURLEncoding.EncodeToString(rawToken)
	expectedHash := sha256.Sum256(rawToken)

	store := &fakeSessionStore{}

	service, err := NewSessionService(
		store,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create session service: %v", err)
	}

	if err := service.Revoke(
		context.Background(),
		token,
	); err != nil {
		t.Fatalf("revoke session: %v", err)
	}

	if !store.deleteCalled {
		t.Fatal("expected session store to be called")
	}

	if !bytes.Equal(
		store.deleteHash,
		expectedHash[:],
	) {
		t.Fatal("expected token to be hashed before deletion")
	}
}

func TestNewSessionServiceRejectsInvalidLifetime(
	t *testing.T,
) {
	store := &fakeSessionStore{}

	_, err := NewSessionService(store, 0)

	if !errors.Is(err, ErrInvalidSessionLifetime) {
		t.Fatalf(
			"expected ErrInvalidSessionLifetime, got %v",
			err,
		)
	}
}
