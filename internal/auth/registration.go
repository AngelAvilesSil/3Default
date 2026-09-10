package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
)

var (
	ErrEmailRequired = errors.New(
		"email is required",
	)
	ErrDisplayNameRequired = errors.New(
		"display name is required",
	)
	ErrEmailAlreadyRegistered = errors.New(
		"email is already registered",
	)
)

type RegistrationStore interface {
	CreateUserWithPassword(
		ctx context.Context,
		userParams dbgen.CreateUserParams,
		passwordHash string,
	) (dbgen.User, error)
}

type PasswordValidator interface {
	NormalizeAndValidate(
		ctx context.Context,
		password string,
		email string,
	) (string, error)
}

type RegistrationService struct {
	registrations RegistrationStore
	passwords     PasswordValidator
	hashPassword  func(string) (string, error)
}

type RegisterInput struct {
	Email       string
	DisplayName string
	Password    string
}

func NewRegistrationService(
	registrations RegistrationStore,
	passwords PasswordValidator,
) *RegistrationService {
	return &RegistrationService{
		registrations: registrations,
		passwords:     passwords,
		hashPassword:  HashPassword,
	}
}

func (s *RegistrationService) Register(
	ctx context.Context,
	input RegisterInput,
) (dbgen.User, error) {
	email := strings.ToLower(strings.TrimSpace(input.Email))
	if email == "" {
		return dbgen.User{}, ErrEmailRequired
	}

	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		return dbgen.User{}, ErrDisplayNameRequired
	}

	password, err := s.passwords.NormalizeAndValidate(
		ctx,
		input.Password,
		email,
	)
	if err != nil {
		return dbgen.User{}, fmt.Errorf(
			"validate registration password: %w",
			err,
		)
	}

	passwordHash, err := s.hashPassword(password)
	if err != nil {
		return dbgen.User{}, fmt.Errorf(
			"hash registration password: %w",
			err,
		)
	}

	user, err := s.registrations.CreateUserWithPassword(
		ctx,
		dbgen.CreateUserParams{
			Email:       email,
			DisplayName: displayName,
		},
		passwordHash,
	)
	if err != nil {
		return dbgen.User{}, fmt.Errorf(
			"persist registration: %w",
			err,
		)
	}

	return user, nil
}
