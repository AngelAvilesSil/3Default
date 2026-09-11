package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"golang.org/x/text/unicode/norm"
)

const dummyLoginPassword = "3default-dummy-login-password"

var ErrInvalidCredentials = errors.New(
	"invalid email or password",
)

type LoginCredentialStore interface {
	GetLoginCredentialByEmail(
		ctx context.Context,
		email string,
	) (dbgen.GetLoginCredentialByEmailRow, error)
}

type LoginSessionCreator interface {
	Create(
		ctx context.Context,
		userID uuid.UUID,
	) (CreatedSession, error)
}

type LoginService struct {
	credentials       LoginCredentialStore
	sessions          LoginSessionCreator
	dummyPasswordHash string
	verifyPassword    func(string, string) (bool, error)
}

type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	User    dbgen.User
	Session CreatedSession
}

func NewLoginService(
	credentials LoginCredentialStore,
	sessions LoginSessionCreator,
) (*LoginService, error) {
	dummyPasswordHash, err := HashPassword(
		dummyLoginPassword,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create dummy login password hash: %w",
			err,
		)
	}

	return &LoginService{
		credentials:       credentials,
		sessions:          sessions,
		dummyPasswordHash: dummyPasswordHash,
		verifyPassword:    VerifyPassword,
	}, nil
}

func (s *LoginService) Login(
	ctx context.Context,
	input LoginInput,
) (LoginResult, error) {
	email := strings.ToLower(
		strings.TrimSpace(input.Email),
	)

	password := input.Password
	if utf8.ValidString(password) {
		password = norm.NFC.String(password)
	}

	credential, err := s.credentials.GetLoginCredentialByEmail(
		ctx,
		email,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		_, verifyErr := s.verifyPassword(
			password,
			s.dummyPasswordHash,
		)
		if verifyErr != nil {
			return LoginResult{}, fmt.Errorf(
				"verify dummy login password: %w",
				verifyErr,
			)
		}

		return LoginResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return LoginResult{}, fmt.Errorf(
			"get login credential: %w",
			err,
		)
	}

	matches, err := s.verifyPassword(
		password,
		credential.PasswordHash,
	)
	if err != nil {
		return LoginResult{}, fmt.Errorf(
			"verify login password: %w",
			err,
		)
	}

	if !matches {
		return LoginResult{}, ErrInvalidCredentials
	}

	session, err := s.sessions.Create(
		ctx,
		credential.ID,
	)
	if err != nil {
		return LoginResult{}, fmt.Errorf(
			"create login session: %w",
			err,
		)
	}

	return LoginResult{
		User: dbgen.User{
			ID:          credential.ID,
			Email:       credential.Email,
			DisplayName: credential.DisplayName,
			CreatedAt:   credential.CreatedAt,
			UpdatedAt:   credential.UpdatedAt,
		},
		Session: session,
	}, nil
}

var _ LoginCredentialStore = (*dbgen.Queries)(nil)
var _ LoginSessionCreator = (*SessionService)(nil)
