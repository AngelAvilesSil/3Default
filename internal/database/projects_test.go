package database

import (
	"errors"
	"fmt"
	"testing"

	"github.com/AngelAvilesSil/3Default/internal/projects"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapProjectNameError(t *testing.T) {
	const indexName = "projects_owner_name_case_insensitive_unique"

	nameConflict := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: indexName,
	}
	otherConstraint := &pgconn.PgError{
		Code:           "23505",
		ConstraintName: "some_other_unique_constraint",
	}
	differentCode := &pgconn.PgError{
		Code:           "23514",
		ConstraintName: indexName,
	}
	databaseUnavailable := errors.New("database unavailable")

	tests := []struct {
		name         string
		input        error
		wantConflict bool
	}{
		{
			name:         "maps matching project-name violation",
			input:        nameConflict,
			wantConflict: true,
		},
		{
			name:         "maps wrapped project-name violation",
			input:        fmt.Errorf("insert project: %w", nameConflict),
			wantConflict: true,
		},
		{
			name:  "preserves another unique-constraint violation",
			input: otherConstraint,
		},
		{
			name:  "preserves another PostgreSQL error code",
			input: differentCode,
		},
		{
			name:  "preserves ordinary database error",
			input: databaseUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapProjectNameError(tt.input)

			if tt.wantConflict {
				if !errors.Is(got, projects.ErrNameConflict) {
					t.Fatalf(
						"error = %v, want ErrNameConflict",
						got,
					)
				}
				return
			}

			if errors.Is(got, projects.ErrNameConflict) {
				t.Fatalf(
					"error = %v, did not want ErrNameConflict",
					got,
				)
			}

			if !errors.Is(got, tt.input) {
				t.Fatalf(
					"error = %v, want original error preserved",
					got,
				)
			}
		})
	}
}
