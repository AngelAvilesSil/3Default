package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type loginCredentialStoreStub struct {
	getLoginCredentialByEmail func(
		ctx context.Context,
		email string,
	) (dbgen.GetLoginCredentialByEmailRow, error)
}

func (s loginCredentialStoreStub) GetLoginCredentialByEmail(
	ctx context.Context,
	email string,
) (dbgen.GetLoginCredentialByEmailRow, error) {
	return s.getLoginCredentialByEmail(
		ctx,
		email,
	)
}

type loginSessionCreatorStub struct {
	create func(
		ctx context.Context,
		userID uuid.UUID,
	) (CreatedSession, error)
}

func (s loginSessionCreatorStub) Create(
	ctx context.Context,
	userID uuid.UUID,
) (CreatedSession, error) {
	return s.create(
		ctx,
		userID,
	)
}

func TestLoginServiceLogin(t *testing.T) {
	userID := uuid.New()

	createdAt := time.Date(
		2026,
		time.September,
		10,
		12,
		0,
		0,
		0,
		time.UTC,
	)

	updatedAt := createdAt.Add(time.Minute)

	expectedSession := CreatedSession{
		Token: "session-token",
		Session: Session{
			ID:        uuid.New(),
			UserID:    userID,
			CreatedAt: createdAt,
			ExpiresAt: createdAt.Add(time.Hour),
		},
	}

	credentials := loginCredentialStoreStub{
		getLoginCredentialByEmail: func(
			_ context.Context,
			email string,
		) (dbgen.GetLoginCredentialByEmailRow, error) {
			if email != "person@example.com" {
				t.Fatalf(
					"email = %q, want %q",
					email,
					"person@example.com",
				)
			}

			return dbgen.GetLoginCredentialByEmailRow{
				ID:           userID,
				Email:        "person@example.com",
				DisplayName:  "Person",
				CreatedAt:    createdAt,
				UpdatedAt:    updatedAt,
				PasswordHash: "stored-password-hash",
			}, nil
		},
	}

	sessions := loginSessionCreatorStub{
		create: func(
			_ context.Context,
			gotUserID uuid.UUID,
		) (CreatedSession, error) {
			if gotUserID != userID {
				t.Fatalf(
					"userID = %s, want %s",
					gotUserID,
					userID,
				)
			}

			return expectedSession, nil
		},
	}

	service := &LoginService{
		credentials:       credentials,
		sessions:          sessions,
		dummyPasswordHash: "dummy-password-hash",
		verifyPassword: func(
			password string,
			encodedHash string,
		) (bool, error) {
			if password != "éx" {
				t.Fatalf(
					"password = %q, want NFC-normalized %q",
					password,
					"éx",
				)
			}

			if encodedHash != "stored-password-hash" {
				t.Fatalf(
					"encodedHash = %q, want %q",
					encodedHash,
					"stored-password-hash",
				)
			}

			return true, nil
		},
	}

	result, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "  Person@Example.COM  ",
			Password: "e\u0301x",
		},
	)
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if result.User.ID != userID {
		t.Fatalf(
			"User.ID = %s, want %s",
			result.User.ID,
			userID,
		)
	}

	if result.User.Email != "person@example.com" {
		t.Fatalf(
			"User.Email = %q, want %q",
			result.User.Email,
			"person@example.com",
		)
	}

	if result.User.DisplayName != "Person" {
		t.Fatalf(
			"User.DisplayName = %q, want %q",
			result.User.DisplayName,
			"Person",
		)
	}

	if !result.User.CreatedAt.Equal(createdAt) {
		t.Fatalf(
			"User.CreatedAt = %s, want %s",
			result.User.CreatedAt,
			createdAt,
		)
	}

	if !result.User.UpdatedAt.Equal(updatedAt) {
		t.Fatalf(
			"User.UpdatedAt = %s, want %s",
			result.User.UpdatedAt,
			updatedAt,
		)
	}

	if result.Session.Token != expectedSession.Token {
		t.Fatalf(
			"Session.Token = %q, want %q",
			result.Session.Token,
			expectedSession.Token,
		)
	}

	if result.Session.Session != expectedSession.Session {
		t.Fatalf(
			"Session.Session = %#v, want %#v",
			result.Session.Session,
			expectedSession.Session,
		)
	}
}

func TestLoginServiceRejectsWrongPassword(t *testing.T) {
	credentials := loginCredentialStoreStub{
		getLoginCredentialByEmail: func(
			context.Context,
			string,
		) (dbgen.GetLoginCredentialByEmailRow, error) {
			return dbgen.GetLoginCredentialByEmailRow{
				ID:           uuid.New(),
				PasswordHash: "stored-password-hash",
			}, nil
		},
	}

	sessions := loginSessionCreatorStub{
		create: func(
			context.Context,
			uuid.UUID,
		) (CreatedSession, error) {
			t.Fatal("session creator should not be called")
			return CreatedSession{}, nil
		},
	}

	service := &LoginService{
		credentials:       credentials,
		sessions:          sessions,
		dummyPasswordHash: "dummy-password-hash",
		verifyPassword: func(
			password string,
			encodedHash string,
		) (bool, error) {
			if password != "wrong password" {
				t.Fatalf(
					"password = %q, want %q",
					password,
					"wrong password",
				)
			}

			if encodedHash != "stored-password-hash" {
				t.Fatalf(
					"encodedHash = %q, want stored hash",
					encodedHash,
				)
			}

			return false, nil
		},
	}

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "person@example.com",
			Password: "wrong password",
		},
	)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf(
			"Login() error = %v, want ErrInvalidCredentials",
			err,
		)
	}
}

func TestLoginServiceUsesDummyHashForMissingUser(
	t *testing.T,
) {
	credentials := loginCredentialStoreStub{
		getLoginCredentialByEmail: func(
			_ context.Context,
			email string,
		) (dbgen.GetLoginCredentialByEmailRow, error) {
			if email != "missing@example.com" {
				t.Fatalf(
					"email = %q, want %q",
					email,
					"missing@example.com",
				)
			}

			return dbgen.GetLoginCredentialByEmailRow{},
				pgx.ErrNoRows
		},
	}

	sessions := loginSessionCreatorStub{
		create: func(
			context.Context,
			uuid.UUID,
		) (CreatedSession, error) {
			t.Fatal("session creator should not be called")
			return CreatedSession{}, nil
		},
	}

	service := &LoginService{
		credentials:       credentials,
		sessions:          sessions,
		dummyPasswordHash: "dummy-password-hash",
		verifyPassword: func(
			password string,
			encodedHash string,
		) (bool, error) {
			if password != "submitted password" {
				t.Fatalf(
					"password = %q, want %q",
					password,
					"submitted password",
				)
			}

			if encodedHash != "dummy-password-hash" {
				t.Fatalf(
					"encodedHash = %q, want dummy hash",
					encodedHash,
				)
			}

			return false, nil
		},
	}

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "missing@example.com",
			Password: "submitted password",
		},
	)
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf(
			"Login() error = %v, want ErrInvalidCredentials",
			err,
		)
	}
}

func TestLoginServiceReturnsCredentialStoreError(
	t *testing.T,
) {
	storeErr := errors.New("database unavailable")

	credentials := loginCredentialStoreStub{
		getLoginCredentialByEmail: func(
			context.Context,
			string,
		) (dbgen.GetLoginCredentialByEmailRow, error) {
			return dbgen.GetLoginCredentialByEmailRow{},
				storeErr
		},
	}

	sessions := loginSessionCreatorStub{
		create: func(
			context.Context,
			uuid.UUID,
		) (CreatedSession, error) {
			t.Fatal("session creator should not be called")
			return CreatedSession{}, nil
		},
	}

	service := &LoginService{
		credentials:       credentials,
		sessions:          sessions,
		dummyPasswordHash: "dummy-password-hash",
		verifyPassword: func(
			string,
			string,
		) (bool, error) {
			t.Fatal("password verifier should not be called")
			return false, nil
		},
	}

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "person@example.com",
			Password: "password",
		},
	)
	if !errors.Is(err, storeErr) {
		t.Fatalf(
			"Login() error = %v, want store error",
			err,
		)
	}
}

func TestLoginServiceReturnsPasswordVerificationError(
	t *testing.T,
) {
	verifyErr := errors.New("password hash invalid")

	credentials := loginCredentialStoreStub{
		getLoginCredentialByEmail: func(
			context.Context,
			string,
		) (dbgen.GetLoginCredentialByEmailRow, error) {
			return dbgen.GetLoginCredentialByEmailRow{
				ID:           uuid.New(),
				PasswordHash: "stored-password-hash",
			}, nil
		},
	}

	sessions := loginSessionCreatorStub{
		create: func(
			context.Context,
			uuid.UUID,
		) (CreatedSession, error) {
			t.Fatal("session creator should not be called")
			return CreatedSession{}, nil
		},
	}

	service := &LoginService{
		credentials:       credentials,
		sessions:          sessions,
		dummyPasswordHash: "dummy-password-hash",
		verifyPassword: func(
			string,
			string,
		) (bool, error) {
			return false, verifyErr
		},
	}

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "person@example.com",
			Password: "password",
		},
	)
	if !errors.Is(err, verifyErr) {
		t.Fatalf(
			"Login() error = %v, want verification error",
			err,
		)
	}
}

func TestLoginServiceReturnsSessionCreationError(
	t *testing.T,
) {
	sessionErr := errors.New("session store unavailable")
	userID := uuid.New()

	credentials := loginCredentialStoreStub{
		getLoginCredentialByEmail: func(
			context.Context,
			string,
		) (dbgen.GetLoginCredentialByEmailRow, error) {
			return dbgen.GetLoginCredentialByEmailRow{
				ID:           userID,
				PasswordHash: "stored-password-hash",
			}, nil
		},
	}

	sessions := loginSessionCreatorStub{
		create: func(
			_ context.Context,
			gotUserID uuid.UUID,
		) (CreatedSession, error) {
			if gotUserID != userID {
				t.Fatalf(
					"userID = %s, want %s",
					gotUserID,
					userID,
				)
			}

			return CreatedSession{}, sessionErr
		},
	}

	service := &LoginService{
		credentials:       credentials,
		sessions:          sessions,
		dummyPasswordHash: "dummy-password-hash",
		verifyPassword: func(
			string,
			string,
		) (bool, error) {
			return true, nil
		},
	}

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "person@example.com",
			Password: "password",
		},
	)
	if !errors.Is(err, sessionErr) {
		t.Fatalf(
			"Login() error = %v, want session error",
			err,
		)
	}
}

func TestLoginServiceReturnsDummyPasswordVerificationError(
	t *testing.T,
) {
	verifyErr := errors.New("dummy password hash invalid")

	credentials := loginCredentialStoreStub{
		getLoginCredentialByEmail: func(
			context.Context,
			string,
		) (dbgen.GetLoginCredentialByEmailRow, error) {
			return dbgen.GetLoginCredentialByEmailRow{},
				pgx.ErrNoRows
		},
	}

	sessions := loginSessionCreatorStub{
		create: func(
			context.Context,
			uuid.UUID,
		) (CreatedSession, error) {
			t.Fatal("session creator should not be called")
			return CreatedSession{}, nil
		},
	}

	service := &LoginService{
		credentials:       credentials,
		sessions:          sessions,
		dummyPasswordHash: "dummy-password-hash",
		verifyPassword: func(
			_ string,
			encodedHash string,
		) (bool, error) {
			if encodedHash != "dummy-password-hash" {
				t.Fatalf(
					"encodedHash = %q, want dummy hash",
					encodedHash,
				)
			}

			return false, verifyErr
		},
	}

	_, err := service.Login(
		context.Background(),
		LoginInput{
			Email:    "missing@example.com",
			Password: "submitted password",
		},
	)
	if !errors.Is(err, verifyErr) {
		t.Fatalf(
			"Login() error = %v, want dummy verification error",
			err,
		)
	}
}
