package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
)

type registrationStoreStub struct {
	createUserWithPassword func(
		ctx context.Context,
		userParams dbgen.CreateUserParams,
		passwordHash string,
	) (dbgen.User, error)
}

func (s registrationStoreStub) CreateUserWithPassword(
	ctx context.Context,
	userParams dbgen.CreateUserParams,
	passwordHash string,
) (dbgen.User, error) {
	return s.createUserWithPassword(
		ctx,
		userParams,
		passwordHash,
	)
}

type newPasswordPolicyStub struct {
	normalizeAndValidate func(
		ctx context.Context,
		password string,
		email string,
	) (string, error)
}

func (p newPasswordPolicyStub) NormalizeAndValidate(
	ctx context.Context,
	password string,
	email string,
) (string, error) {
	return p.normalizeAndValidate(
		ctx,
		password,
		email,
	)
}

func TestRegistrationServiceRegister(t *testing.T) {
	expectedUser := dbgen.User{
		ID:          uuid.New(),
		Email:       "person@example.com",
		DisplayName: "Person",
	}

	store := registrationStoreStub{
		createUserWithPassword: func(
			_ context.Context,
			userParams dbgen.CreateUserParams,
			passwordHash string,
		) (dbgen.User, error) {
			if userParams.Email != "person@example.com" {
				t.Fatalf(
					"Email = %q, want %q",
					userParams.Email,
					"person@example.com",
				)
			}

			if userParams.DisplayName != "Person" {
				t.Fatalf(
					"DisplayName = %q, want %q",
					userParams.DisplayName,
					"Person",
				)
			}

			if passwordHash != "encoded-password-hash" {
				t.Fatalf(
					"passwordHash = %q, want %q",
					passwordHash,
					"encoded-password-hash",
				)
			}

			return expectedUser, nil
		},
	}

	policy := newPasswordPolicyStub{
		normalizeAndValidate: func(
			_ context.Context,
			password string,
			email string,
		) (string, error) {
			if password != "raw password" {
				t.Fatalf(
					"password = %q, want %q",
					password,
					"raw password",
				)
			}

			if email != "person@example.com" {
				t.Fatalf(
					"email = %q, want %q",
					email,
					"person@example.com",
				)
			}

			return "normalized password", nil
		},
	}

	service := NewRegistrationService(
		store,
		policy,
	)

	service.hashPassword = func(
		password string,
	) (string, error) {
		if password != "normalized password" {
			t.Fatalf(
				"password = %q, want %q",
				password,
				"normalized password",
			)
		}

		return "encoded-password-hash", nil
	}

	user, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:       "  Person@Example.COM  ",
			DisplayName: "  Person  ",
			Password:    "raw password",
		},
	)
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if user != expectedUser {
		t.Fatalf(
			"Register() user = %#v, want %#v",
			user,
			expectedUser,
		)
	}
}

func TestRegistrationServiceRequiresEmail(t *testing.T) {
	service := NewRegistrationService(
		registrationStoreStub{
			createUserWithPassword: func(
				context.Context,
				dbgen.CreateUserParams,
				string,
			) (dbgen.User, error) {
				t.Fatal("registration store should not be called")
				return dbgen.User{}, nil
			},
		},
		newPasswordPolicyStub{
			normalizeAndValidate: func(
				context.Context,
				string,
				string,
			) (string, error) {
				t.Fatal("password policy should not be called")
				return "", nil
			},
		},
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:       "   ",
			DisplayName: "Person",
			Password:    "password",
		},
	)
	if !errors.Is(err, ErrEmailRequired) {
		t.Fatalf(
			"Register() error = %v, want ErrEmailRequired",
			err,
		)
	}
}

func TestRegistrationServiceRequiresDisplayName(t *testing.T) {
	service := NewRegistrationService(
		registrationStoreStub{
			createUserWithPassword: func(
				context.Context,
				dbgen.CreateUserParams,
				string,
			) (dbgen.User, error) {
				t.Fatal("registration store should not be called")
				return dbgen.User{}, nil
			},
		},
		newPasswordPolicyStub{
			normalizeAndValidate: func(
				context.Context,
				string,
				string,
			) (string, error) {
				t.Fatal("password policy should not be called")
				return "", nil
			},
		},
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:       "person@example.com",
			DisplayName: "   ",
			Password:    "password",
		},
	)
	if !errors.Is(err, ErrDisplayNameRequired) {
		t.Fatalf(
			"Register() error = %v, want ErrDisplayNameRequired",
			err,
		)
	}
}

func TestRegistrationServiceReturnsPasswordPolicyError(t *testing.T) {
	policyErr := errors.New("password rejected")

	service := NewRegistrationService(
		registrationStoreStub{
			createUserWithPassword: func(
				context.Context,
				dbgen.CreateUserParams,
				string,
			) (dbgen.User, error) {
				t.Fatal("registration store should not be called")
				return dbgen.User{}, nil
			},
		},
		newPasswordPolicyStub{
			normalizeAndValidate: func(
				context.Context,
				string,
				string,
			) (string, error) {
				return "", policyErr
			},
		},
	)

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:       "person@example.com",
			DisplayName: "Person",
			Password:    "password",
		},
	)
	if !errors.Is(err, policyErr) {
		t.Fatalf(
			"Register() error = %v, want password policy error",
			err,
		)
	}
}

func TestRegistrationServiceReturnsPasswordHashError(t *testing.T) {
	hashErr := errors.New("hash failed")

	service := NewRegistrationService(
		registrationStoreStub{
			createUserWithPassword: func(
				context.Context,
				dbgen.CreateUserParams,
				string,
			) (dbgen.User, error) {
				t.Fatal("registration store should not be called")
				return dbgen.User{}, nil
			},
		},
		newPasswordPolicyStub{
			normalizeAndValidate: func(
				context.Context,
				string,
				string,
			) (string, error) {
				return "normalized password", nil
			},
		},
	)

	service.hashPassword = func(
		string,
	) (string, error) {
		return "", hashErr
	}

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:       "person@example.com",
			DisplayName: "Person",
			Password:    "password",
		},
	)
	if !errors.Is(err, hashErr) {
		t.Fatalf(
			"Register() error = %v, want password hash error",
			err,
		)
	}
}

func TestRegistrationServiceReturnsStoreError(t *testing.T) {
	storeErr := errors.New("store failed")

	service := NewRegistrationService(
		registrationStoreStub{
			createUserWithPassword: func(
				context.Context,
				dbgen.CreateUserParams,
				string,
			) (dbgen.User, error) {
				return dbgen.User{}, storeErr
			},
		},
		newPasswordPolicyStub{
			normalizeAndValidate: func(
				context.Context,
				string,
				string,
			) (string, error) {
				return "normalized password", nil
			},
		},
	)

	service.hashPassword = func(
		string,
	) (string, error) {
		return "encoded-password-hash", nil
	}

	_, err := service.Register(
		context.Background(),
		RegisterInput{
			Email:       "person@example.com",
			DisplayName: "Person",
			Password:    "password",
		},
	)
	if !errors.Is(err, storeErr) {
		t.Fatalf(
			"Register() error = %v, want store error",
			err,
		)
	}
}
