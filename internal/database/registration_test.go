package database

import (
	"errors"
	"fmt"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/auth"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapCreateRegistrationUserErrorMapsEmailConflict(
	t *testing.T,
) {
	pgErr := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "users_email_case_insensitive_unique",
	}

	err := mapCreateRegistrationUserError(
		fmt.Errorf("insert user: %w", pgErr),
	)

	if !errors.Is(err, auth.ErrEmailAlreadyRegistered) {
		t.Fatalf(
			"error = %v, want ErrEmailAlreadyRegistered",
			err,
		)
	}
}

func TestMapCreateRegistrationUserErrorDoesNotMapOtherUniqueConstraint(
	t *testing.T,
) {
	pgErr := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "some_other_unique_constraint",
	}

	err := mapCreateRegistrationUserError(pgErr)

	if errors.Is(err, auth.ErrEmailAlreadyRegistered) {
		t.Fatalf(
			"error = %v, did not want ErrEmailAlreadyRegistered",
			err,
		)
	}

	if !errors.Is(err, pgErr) {
		t.Fatalf(
			"error = %v, want original PostgreSQL error preserved",
			err,
		)
	}
}

func TestMapCreateRegistrationUserErrorPreservesUnexpectedError(
	t *testing.T,
) {
	originalErr := errors.New("database unavailable")

	err := mapCreateRegistrationUserError(originalErr)

	if !errors.Is(err, originalErr) {
		t.Fatalf(
			"error = %v, want original error preserved",
			err,
		)
	}

	if errors.Is(err, auth.ErrEmailAlreadyRegistered) {
		t.Fatalf(
			"error = %v, did not want ErrEmailAlreadyRegistered",
			err,
		)
	}
}
