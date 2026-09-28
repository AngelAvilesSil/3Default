//go:build integration

package database_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGeneratedQueries(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}

	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	queries := dbgen.New(tx)

	user, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "Angel@example.com",
		DisplayName: "Angel",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	foundUser, err := queries.GetUserByEmail(ctx, "angel@example.com")
	if err != nil {
		t.Fatalf("get user by email: %v", err)
	}

	if foundUser.ID != user.ID {
		t.Fatalf("expected user ID %s, got %s", user.ID, foundUser.ID)
	}

	if foundUser.Email != "Angel@example.com" {
		t.Fatalf(
			"expected preserved email casing %q, got %q",
			"Angel@example.com",
			foundUser.Email,
		)
	}

	project, err := queries.CreateProject(ctx, dbgen.CreateProjectParams{
		OwnerUserID: user.ID,
		Name:        "Integration Test Project",
		Description: nil,
	})
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	if project.OwnerUserID != user.ID {
		t.Fatalf(
			"expected owner ID %s, got %s",
			user.ID,
			project.OwnerUserID,
		)
	}

	if project.Visibility != "private" {
		t.Fatalf(
			"expected default visibility %q, got %q",
			"private",
			project.Visibility,
		)
	}

	if project.Description != nil {
		t.Fatalf("expected nil description, got %q", *project.Description)
	}

	projects, err := queries.ListProjectsByOwner(ctx, user.ID)
	if err != nil {
		t.Fatalf("list projects by owner: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	if projects[0].ID != project.ID {
		t.Fatalf(
			"expected project ID %s, got %s",
			project.ID,
			projects[0].ID,
		)
	}
	foundProject, err := queries.GetProjectByIDAndOwner(
		ctx,
		dbgen.GetProjectByIDAndOwnerParams{
			ProjectID:   project.ID,
			OwnerUserID: user.ID,
		},
	)
	if err != nil {
		t.Fatalf("get project by ID and owner: %v", err)
	}

	if foundProject.ID != project.ID {
		t.Fatalf(
			"expected project ID %s, got %s",
			project.ID,
			foundProject.ID,
		)
	}

	staleUpdatedAt := project.UpdatedAt.Add(-time.Hour)

	setStaleProjectUpdatedAt := func() {
		result, err := tx.Exec(
			ctx,
			"UPDATE projects SET updated_at = $1 WHERE id = $2",
			staleUpdatedAt,
			project.ID,
		)
		if err != nil {
			t.Fatalf("set stale project updated_at: %v", err)
		}

		if result.RowsAffected() != 1 {
			t.Fatalf(
				"expected 1 project row when setting stale updated_at, got %d",
				result.RowsAffected(),
			)
		}
	}

	updatedDescription := "Updated project description"
	setStaleProjectUpdatedAt()

	descriptionUpdatedProject, err := queries.UpdateProjectMetadataByIDAndOwner(
		ctx,
		dbgen.UpdateProjectMetadataByIDAndOwnerParams{
			NameSet:        false,
			Name:           "ignored name",
			DescriptionSet: true,
			Description:    &updatedDescription,
			ProjectID:      project.ID,
			OwnerUserID:    user.ID,
		},
	)
	if err != nil {
		t.Fatalf("update project description: %v", err)
	}

	if descriptionUpdatedProject.Name != project.Name {
		t.Fatalf(
			"expected unchanged name %q, got %q",
			project.Name,
			descriptionUpdatedProject.Name,
		)
	}

	if descriptionUpdatedProject.Description == nil ||
		*descriptionUpdatedProject.Description != updatedDescription {
		t.Fatalf(
			"expected description %q, got %v",
			updatedDescription,
			descriptionUpdatedProject.Description,
		)
	}

	if !descriptionUpdatedProject.UpdatedAt.After(staleUpdatedAt) {
		t.Fatalf(
			"expected updated_at after stale value %s, got %s",
			staleUpdatedAt,
			descriptionUpdatedProject.UpdatedAt,
		)
	}

	ignoredDescription := "ignored description"
	updatedName := "Renamed Integration Test Project"
	setStaleProjectUpdatedAt()

	nameUpdatedProject, err := queries.UpdateProjectMetadataByIDAndOwner(
		ctx,
		dbgen.UpdateProjectMetadataByIDAndOwnerParams{
			NameSet:        true,
			Name:           updatedName,
			DescriptionSet: false,
			Description:    &ignoredDescription,
			ProjectID:      project.ID,
			OwnerUserID:    user.ID,
		},
	)
	if err != nil {
		t.Fatalf("update project name: %v", err)
	}

	if nameUpdatedProject.Name != updatedName {
		t.Fatalf(
			"expected name %q, got %q",
			updatedName,
			nameUpdatedProject.Name,
		)
	}

	if nameUpdatedProject.Description == nil ||
		*nameUpdatedProject.Description != updatedDescription {
		t.Fatalf(
			"expected preserved description %q, got %v",
			updatedDescription,
			nameUpdatedProject.Description,
		)
	}

	if !nameUpdatedProject.UpdatedAt.After(staleUpdatedAt) {
		t.Fatalf(
			"expected updated_at after stale value %s, got %s",
			staleUpdatedAt,
			nameUpdatedProject.UpdatedAt,
		)
	}

	setStaleProjectUpdatedAt()

	clearedDescriptionProject, err := queries.UpdateProjectMetadataByIDAndOwner(
		ctx,
		dbgen.UpdateProjectMetadataByIDAndOwnerParams{
			NameSet:        false,
			Name:           "ignored name",
			DescriptionSet: true,
			Description:    nil,
			ProjectID:      project.ID,
			OwnerUserID:    user.ID,
		},
	)
	if err != nil {
		t.Fatalf("clear project description: %v", err)
	}

	if clearedDescriptionProject.Name != updatedName {
		t.Fatalf(
			"expected preserved name %q, got %q",
			updatedName,
			clearedDescriptionProject.Name,
		)
	}

	if clearedDescriptionProject.Description != nil {
		t.Fatalf(
			"expected nil description, got %q",
			*clearedDescriptionProject.Description,
		)
	}

	if !clearedDescriptionProject.UpdatedAt.After(staleUpdatedAt) {
		t.Fatalf(
			"expected updated_at after stale value %s, got %s",
			staleUpdatedAt,
			clearedDescriptionProject.UpdatedAt,
		)
	}

	otherUser, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "other-project-owner@example.com",
		DisplayName: "Other Project Owner",
	})
	if err != nil {
		t.Fatalf("create other project owner: %v", err)
	}

	_, err = queries.UpdateProjectMetadataByIDAndOwner(
		ctx,
		dbgen.UpdateProjectMetadataByIDAndOwnerParams{
			NameSet:        true,
			Name:           "Unauthorized Rename",
			DescriptionSet: false,
			Description:    nil,
			ProjectID:      project.ID,
			OwnerUserID:    otherUser.ID,
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected another owner's project update to return pgx.ErrNoRows, got %v",
			err,
		)
	}

	projectAfterUnauthorizedUpdate, err := queries.GetProjectByIDAndOwner(
		ctx,
		dbgen.GetProjectByIDAndOwnerParams{
			ProjectID:   project.ID,
			OwnerUserID: user.ID,
		},
	)
	if err != nil {
		t.Fatalf("get project after unauthorized update: %v", err)
	}

	if projectAfterUnauthorizedUpdate.Name != updatedName {
		t.Fatalf(
			"expected unauthorized update to preserve name %q, got %q",
			updatedName,
			projectAfterUnauthorizedUpdate.Name,
		)
	}

	_, err = queries.UpdateProjectMetadataByIDAndOwner(
		ctx,
		dbgen.UpdateProjectMetadataByIDAndOwnerParams{
			NameSet:        true,
			Name:           "Missing Project",
			DescriptionSet: false,
			Description:    nil,
			ProjectID:      uuid.New(),
			OwnerUserID:    user.ID,
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected missing project update to return pgx.ErrNoRows, got %v",
			err,
		)
	}

	_, err = queries.GetProjectByIDAndOwner(
		ctx,
		dbgen.GetProjectByIDAndOwnerParams{
			ProjectID:   project.ID,
			OwnerUserID: otherUser.ID,
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected another owner's project lookup to return pgx.ErrNoRows, got %v",
			err,
		)
	}

	_, err = queries.GetProjectByIDAndOwner(
		ctx,
		dbgen.GetProjectByIDAndOwnerParams{
			ProjectID:   uuid.New(),
			OwnerUserID: user.ID,
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected missing project lookup to return pgx.ErrNoRows, got %v",
			err,
		)
	}
}

func TestProjectStoreCreatesProjectWithMainBranch(
	t *testing.T,
) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	queries := dbgen.New(pool)

	user, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "atomic-project@example.com",
		DisplayName: "Atomic Project User",
	})
	if err != nil {
		t.Fatalf("create project owner: %v", err)
	}

	var projectID uuid.UUID

	defer func() {
		if projectID != uuid.Nil {
			_, err := pool.Exec(
				context.Background(),
				"DELETE FROM projects WHERE id = $1",
				projectID,
			)
			if err != nil {
				t.Errorf("delete project store test project: %v", err)
			}
		}

		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM users WHERE id = $1",
			user.ID,
		)
		if err != nil {
			t.Errorf("delete project store test user: %v", err)
		}
	}()

	store := database.NewProjectStore(pool)

	project, err := store.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: user.ID,
			Name:        "Atomic Project",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create project with default branch: %v", err)
	}

	projectID = project.ID

	var branchCount int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_branches
		 WHERE project_id = $1`,
		project.ID,
	).Scan(&branchCount); err != nil {
		t.Fatalf("count project branches: %v", err)
	}

	if branchCount != 1 {
		t.Fatalf(
			"expected 1 default branch, got %d",
			branchCount,
		)
	}

	var branchName string
	var headRevisionID *uuid.UUID

	if err := pool.QueryRow(
		ctx,
		`SELECT name, head_revision_id
		 FROM project_branches
		 WHERE project_id = $1`,
		project.ID,
	).Scan(
		&branchName,
		&headRevisionID,
	); err != nil {
		t.Fatalf("get default project branch: %v", err)
	}

	if branchName != "main" {
		t.Fatalf(
			"expected default branch name %q, got %q",
			"main",
			branchName,
		)
	}

	if headRevisionID != nil {
		t.Fatalf(
			"expected default branch head to be nil, got %s",
			*headRevisionID,
		)
	}
}

func TestProjectVersioningSchemaConstraints(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}

	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	queries := dbgen.New(tx)

	firstUser, err := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{
			Email:       "versioning-one@example.com",
			DisplayName: "Versioning User One",
		},
	)
	if err != nil {
		t.Fatalf("create first versioning user: %v", err)
	}

	secondUser, err := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{
			Email:       "versioning-two@example.com",
			DisplayName: "Versioning User Two",
		},
	)
	if err != nil {
		t.Fatalf("create second versioning user: %v", err)
	}

	firstProject, err := queries.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: firstUser.ID,
			Name:        "Versioning Project One",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create first versioning project: %v", err)
	}

	secondProject, err := queries.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: secondUser.ID,
			Name:        "Versioning Project Two",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create second versioning project: %v", err)
	}

	insertRevision := func(
		id uuid.UUID,
		projectID uuid.UUID,
		authorUserID uuid.UUID,
		message string,
		parentRevisionID *uuid.UUID,
		mergeParentRevisionID *uuid.UUID,
	) error {
		_, err := tx.Exec(
			ctx,
			`INSERT INTO project_revisions (
				id,
				project_id,
				author_user_id,
				message,
				parent_revision_id,
				merge_parent_revision_id
			)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			id,
			projectID,
			authorUserID,
			message,
			parentRevisionID,
			mergeParentRevisionID,
		)
		return err
	}

	firstRootID := uuid.New()
	if err := insertRevision(
		firstRootID,
		firstProject.ID,
		firstUser.ID,
		"Initial revision",
		nil,
		nil,
	); err != nil {
		t.Fatalf("create first root revision: %v", err)
	}

	firstNormalID := uuid.New()
	if err := insertRevision(
		firstNormalID,
		firstProject.ID,
		firstUser.ID,
		"Normal revision",
		&firstRootID,
		nil,
	); err != nil {
		t.Fatalf("create normal revision: %v", err)
	}

	firstSideID := uuid.New()
	if err := insertRevision(
		firstSideID,
		firstProject.ID,
		firstUser.ID,
		"Side revision",
		&firstRootID,
		nil,
	); err != nil {
		t.Fatalf("create side revision: %v", err)
	}

	firstMergeID := uuid.New()
	if err := insertRevision(
		firstMergeID,
		firstProject.ID,
		firstUser.ID,
		"Merge revision",
		&firstNormalID,
		&firstSideID,
	); err != nil {
		t.Fatalf("create valid merge revision: %v", err)
	}

	secondRootID := uuid.New()
	if err := insertRevision(
		secondRootID,
		secondProject.ID,
		secondUser.ID,
		"Second project root",
		nil,
		nil,
	); err != nil {
		t.Fatalf("create second root revision: %v", err)
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO project_branches (
			project_id,
			name,
			head_revision_id
		)
		VALUES ($1, 'main', $2)`,
		firstProject.ID,
		firstMergeID,
	); err != nil {
		t.Fatalf("create first valid main branch: %v", err)
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO project_branches (
			project_id,
			name,
			head_revision_id
		)
		VALUES ($1, 'main', $2)`,
		secondProject.ID,
		secondRootID,
	); err != nil {
		t.Fatalf(
			"expected same branch name in different project to succeed: %v",
			err,
		)
	}

	assertConstraintViolation := func(
		t *testing.T,
		expectedCode string,
		expectedConstraint string,
		run func(pgx.Tx) error,
	) {
		t.Helper()

		savepoint, err := tx.Begin(ctx)
		if err != nil {
			t.Fatalf("begin constraint-test savepoint: %v", err)
		}

		err = run(savepoint)

		if rollbackErr := savepoint.Rollback(ctx); rollbackErr != nil {
			t.Fatalf(
				"rollback constraint-test savepoint: %v",
				rollbackErr,
			)
		}

		if err == nil {
			t.Fatalf(
				"expected PostgreSQL constraint %q to fail",
				expectedConstraint,
			)
		}

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) {
			t.Fatalf(
				"expected PostgreSQL error for constraint %q, got %v",
				expectedConstraint,
				err,
			)
		}

		if pgErr.Code != expectedCode {
			t.Fatalf(
				"expected PostgreSQL code %q, got %q",
				expectedCode,
				pgErr.Code,
			)
		}

		if pgErr.ConstraintName != expectedConstraint {
			t.Fatalf(
				"expected constraint %q, got %q",
				expectedConstraint,
				pgErr.ConstraintName,
			)
		}
	}

	t.Run("rejects cross-project primary parent", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23503",
			"project_revisions_parent_same_project",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_revisions (
						id,
						project_id,
						author_user_id,
						message,
						parent_revision_id
					)
					VALUES ($1, $2, $3, $4, $5)`,
					uuid.New(),
					firstProject.ID,
					firstUser.ID,
					"Invalid cross-project parent",
					secondRootID,
				)
				return err
			},
		)
	})

	t.Run("rejects cross-project merge parent", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23503",
			"project_revisions_merge_parent_same_project",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_revisions (
						id,
						project_id,
						author_user_id,
						message,
						parent_revision_id,
						merge_parent_revision_id
					)
					VALUES ($1, $2, $3, $4, $5, $6)`,
					uuid.New(),
					firstProject.ID,
					firstUser.ID,
					"Invalid cross-project merge parent",
					firstNormalID,
					secondRootID,
				)
				return err
			},
		)
	})

	t.Run("requires primary parent for merge parent", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23514",
			"project_revisions_merge_parent_requires_parent",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_revisions (
						id,
						project_id,
						author_user_id,
						message,
						merge_parent_revision_id
					)
					VALUES ($1, $2, $3, $4, $5)`,
					uuid.New(),
					firstProject.ID,
					firstUser.ID,
					"Invalid second-parent-only revision",
					firstSideID,
				)
				return err
			},
		)
	})

	t.Run("rejects identical parents", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23514",
			"project_revisions_parents_distinct",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_revisions (
						id,
						project_id,
						author_user_id,
						message,
						parent_revision_id,
						merge_parent_revision_id
					)
					VALUES ($1, $2, $3, $4, $5, $6)`,
					uuid.New(),
					firstProject.ID,
					firstUser.ID,
					"Invalid duplicate parents",
					firstRootID,
					firstRootID,
				)
				return err
			},
		)
	})

	t.Run("rejects self parent", func(t *testing.T) {
		revisionID := uuid.New()

		assertConstraintViolation(
			t,
			"23514",
			"project_revisions_parent_not_self",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_revisions (
						id,
						project_id,
						author_user_id,
						message,
						parent_revision_id
					)
					VALUES ($1, $2, $3, $4, $5)`,
					revisionID,
					firstProject.ID,
					firstUser.ID,
					"Invalid self parent",
					revisionID,
				)
				return err
			},
		)
	})

	t.Run("rejects cross-project branch head", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23503",
			"project_branches_head_same_project",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_branches (
						project_id,
						name,
						head_revision_id
					)
					VALUES ($1, $2, $3)`,
					secondProject.ID,
					"invalid-cross-project-head",
					firstRootID,
				)
				return err
			},
		)
	})

	t.Run("rejects duplicate branch name within project", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23505",
			"project_branches_project_name_unique",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_branches (
						project_id,
						name
					)
					VALUES ($1, 'main')`,
					firstProject.ID,
				)
				return err
			},
		)
	})
}

func TestFileStorageSchemaConstraints(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}

	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	queries := dbgen.New(tx)

	firstUser, err := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{
			Email:       "storage-one@example.com",
			DisplayName: "Storage User One",
		},
	)
	if err != nil {
		t.Fatalf("create first storage user: %v", err)
	}

	secondUser, err := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{
			Email:       "storage-two@example.com",
			DisplayName: "Storage User Two",
		},
	)
	if err != nil {
		t.Fatalf("create second storage user: %v", err)
	}

	firstProject, err := queries.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: firstUser.ID,
			Name:        "Storage Project One",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create first storage project: %v", err)
	}

	secondProject, err := queries.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: secondUser.ID,
			Name:        "Storage Project Two",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create second storage project: %v", err)
	}

	firstRevisionID := uuid.New()
	if _, err := tx.Exec(
		ctx,
		`INSERT INTO project_revisions (
			id,
			project_id,
			author_user_id,
			message
		)
		VALUES ($1, $2, $3, $4)`,
		firstRevisionID,
		firstProject.ID,
		firstUser.ID,
		"Storage revision one",
	); err != nil {
		t.Fatalf("create first storage revision: %v", err)
	}

	secondRevisionID := uuid.New()
	if _, err := tx.Exec(
		ctx,
		`INSERT INTO project_revisions (
			id,
			project_id,
			author_user_id,
			message
		)
		VALUES ($1, $2, $3, $4)`,
		secondRevisionID,
		secondProject.ID,
		secondUser.ID,
		"Storage revision two",
	); err != nil {
		t.Fatalf("create second storage revision: %v", err)
	}

	firstOnlyHash := strings.Repeat("a", 64)
	sharedHash := strings.Repeat("b", 64)

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO content_objects (
			sha256,
			size_bytes
		)
		VALUES ($1, $2), ($3, $4)`,
		firstOnlyHash,
		int64(0),
		sharedHash,
		int64(128),
	); err != nil {
		t.Fatalf("create valid content objects: %v", err)
	}

	firstFileID := uuid.New()
	if _, err := tx.Exec(
		ctx,
		`INSERT INTO project_files (
			id,
			project_id,
			uploaded_by_user_id,
			content_sha256,
			original_filename,
			media_type
		)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		firstFileID,
		firstProject.ID,
		firstUser.ID,
		firstOnlyHash,
		"gripper.step",
		nil,
	); err != nil {
		t.Fatalf("create valid project file: %v", err)
	}

	firstSharedFileID := uuid.New()
	if _, err := tx.Exec(
		ctx,
		`INSERT INTO project_files (
			id,
			project_id,
			uploaded_by_user_id,
			content_sha256,
			original_filename,
			media_type
		)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		firstSharedFileID,
		firstProject.ID,
		firstUser.ID,
		sharedHash,
		"shared.step",
		"model/step",
	); err != nil {
		t.Fatalf("create first shared project file: %v", err)
	}

	secondFileID := uuid.New()
	if _, err := tx.Exec(
		ctx,
		`INSERT INTO project_files (
			id,
			project_id,
			uploaded_by_user_id,
			content_sha256,
			original_filename,
			media_type
		)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		secondFileID,
		secondProject.ID,
		secondUser.ID,
		sharedHash,
		"same-content.step",
		"application/octet-stream",
	); err != nil {
		t.Fatalf("create second shared project file: %v", err)
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO project_revision_files (
			project_id,
			revision_id,
			project_file_id
		)
		VALUES ($1, $2, $3)`,
		firstProject.ID,
		firstRevisionID,
		firstFileID,
	); err != nil {
		t.Fatalf("create valid revision file reference: %v", err)
	}

	assertConstraintViolation := func(
		t *testing.T,
		expectedCode string,
		expectedConstraint string,
		run func(pgx.Tx) error,
	) {
		t.Helper()

		savepoint, err := tx.Begin(ctx)
		if err != nil {
			t.Fatalf("begin constraint-test savepoint: %v", err)
		}

		err = run(savepoint)

		if rollbackErr := savepoint.Rollback(ctx); rollbackErr != nil {
			t.Fatalf(
				"rollback constraint-test savepoint: %v",
				rollbackErr,
			)
		}

		if err == nil {
			t.Fatalf(
				"expected PostgreSQL constraint %q to fail",
				expectedConstraint,
			)
		}

		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) {
			t.Fatalf(
				"expected PostgreSQL error for constraint %q, got %v",
				expectedConstraint,
				err,
			)
		}

		if pgErr.Code != expectedCode {
			t.Fatalf(
				"expected PostgreSQL code %q, got %q",
				expectedCode,
				pgErr.Code,
			)
		}

		if pgErr.ConstraintName != expectedConstraint {
			t.Fatalf(
				"expected constraint %q, got %q",
				expectedConstraint,
				pgErr.ConstraintName,
			)
		}
	}

	t.Run("rejects short content hash", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23514",
			"content_objects_sha256_valid",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO content_objects (
						sha256,
						size_bytes
					)
					VALUES ($1, $2)`,
					strings.Repeat("c", 63),
					int64(1),
				)
				return err
			},
		)
	})

	t.Run("rejects uppercase content hash", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23514",
			"content_objects_sha256_valid",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO content_objects (
						sha256,
						size_bytes
					)
					VALUES ($1, $2)`,
					strings.Repeat("A", 64),
					int64(1),
				)
				return err
			},
		)
	})

	t.Run("rejects nonhex content hash", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23514",
			"content_objects_sha256_valid",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO content_objects (
						sha256,
						size_bytes
					)
					VALUES ($1, $2)`,
					strings.Repeat("g", 64),
					int64(1),
				)
				return err
			},
		)
	})

	t.Run("rejects negative content size", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23514",
			"content_objects_size_nonnegative",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO content_objects (
						sha256,
						size_bytes
					)
					VALUES ($1, $2)`,
					strings.Repeat("c", 64),
					int64(-1),
				)
				return err
			},
		)
	})

	t.Run("rejects duplicate content hash", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23505",
			"content_objects_pkey",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO content_objects (
						sha256,
						size_bytes
					)
					VALUES ($1, $2)`,
					firstOnlyHash,
					int64(0),
				)
				return err
			},
		)
	})

	t.Run("requires existing project", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23503",
			"project_files_project_exists",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_files (
						project_id,
						uploaded_by_user_id,
						content_sha256,
						original_filename
					)
					VALUES ($1, $2, $3, $4)`,
					uuid.New(),
					firstUser.ID,
					firstOnlyHash,
					"missing-project.step",
				)
				return err
			},
		)
	})

	t.Run("requires existing uploader", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23503",
			"project_files_uploader_exists",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_files (
						project_id,
						uploaded_by_user_id,
						content_sha256,
						original_filename
					)
					VALUES ($1, $2, $3, $4)`,
					firstProject.ID,
					uuid.New(),
					firstOnlyHash,
					"missing-uploader.step",
				)
				return err
			},
		)
	})

	t.Run("requires existing content object", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23503",
			"project_files_content_exists",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_files (
						project_id,
						uploaded_by_user_id,
						content_sha256,
						original_filename
					)
					VALUES ($1, $2, $3, $4)`,
					firstProject.ID,
					firstUser.ID,
					strings.Repeat("c", 64),
					"missing-content.step",
				)
				return err
			},
		)
	})

	t.Run("rejects blank original filename", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23514",
			"project_files_original_filename_not_blank",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_files (
						project_id,
						uploaded_by_user_id,
						content_sha256,
						original_filename
					)
					VALUES ($1, $2, $3, '')`,
					firstProject.ID,
					firstUser.ID,
					firstOnlyHash,
				)
				return err
			},
		)
	})

	t.Run("rejects untrimmed original filename", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23514",
			"project_files_original_filename_trimmed",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_files (
						project_id,
						uploaded_by_user_id,
						content_sha256,
						original_filename
					)
					VALUES ($1, $2, $3, $4)`,
					firstProject.ID,
					firstUser.ID,
					firstOnlyHash,
					" gripper.step ",
				)
				return err
			},
		)
	})

	t.Run("rejects blank media type", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23514",
			"project_files_media_type_not_blank",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_files (
						project_id,
						uploaded_by_user_id,
						content_sha256,
						original_filename,
						media_type
					)
					VALUES ($1, $2, $3, $4, '')`,
					firstProject.ID,
					firstUser.ID,
					firstOnlyHash,
					"blank-media.step",
				)
				return err
			},
		)
	})

	t.Run("rejects untrimmed media type", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23514",
			"project_files_media_type_trimmed",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_files (
						project_id,
						uploaded_by_user_id,
						content_sha256,
						original_filename,
						media_type
					)
					VALUES ($1, $2, $3, $4, $5)`,
					firstProject.ID,
					firstUser.ID,
					firstOnlyHash,
					"untrimmed-media.step",
					" model/step ",
				)
				return err
			},
		)
	})

	t.Run("rejects revision from another project", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23503",
			"project_revision_files_revision_same_project",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_revision_files (
						project_id,
						revision_id,
						project_file_id
					)
					VALUES ($1, $2, $3)`,
					firstProject.ID,
					secondRevisionID,
					firstSharedFileID,
				)
				return err
			},
		)
	})

	t.Run("rejects file from another project", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23503",
			"project_revision_files_file_same_project",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_revision_files (
						project_id,
						revision_id,
						project_file_id
					)
					VALUES ($1, $2, $3)`,
					firstProject.ID,
					firstRevisionID,
					secondFileID,
				)
				return err
			},
		)
	})

	t.Run("rejects duplicate revision file reference", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23505",
			"project_revision_files_primary_key",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`INSERT INTO project_revision_files (
						project_id,
						revision_id,
						project_file_id
					)
					VALUES ($1, $2, $3)`,
					firstProject.ID,
					firstRevisionID,
					firstFileID,
				)
				return err
			},
		)
	})

	t.Run("protects project file referenced by revision", func(t *testing.T) {
		assertConstraintViolation(
			t,
			"23503",
			"project_revision_files_file_same_project",
			func(savepoint pgx.Tx) error {
				_, err := savepoint.Exec(
					ctx,
					`DELETE FROM project_files
					WHERE id = $1`,
					firstFileID,
				)
				return err
			},
		)
	})

	t.Run("project deletion preserves global content objects", func(t *testing.T) {
		savepoint, err := tx.Begin(ctx)
		if err != nil {
			t.Fatalf("begin project deletion savepoint: %v", err)
		}

		if _, err := savepoint.Exec(
			ctx,
			`DELETE FROM projects
			WHERE id = $1`,
			firstProject.ID,
		); err != nil {
			_ = savepoint.Rollback(ctx)
			t.Fatalf("delete project with file references: %v", err)
		}

		var revisionCount int
		if err := savepoint.QueryRow(
			ctx,
			`SELECT count(*)
			FROM project_revisions
			WHERE project_id = $1`,
			firstProject.ID,
		).Scan(&revisionCount); err != nil {
			_ = savepoint.Rollback(ctx)
			t.Fatalf("count revisions after project deletion: %v", err)
		}

		if revisionCount != 0 {
			_ = savepoint.Rollback(ctx)
			t.Fatalf(
				"expected project revisions to be deleted, got %d",
				revisionCount,
			)
		}

		var projectFileCount int
		if err := savepoint.QueryRow(
			ctx,
			`SELECT count(*)
			FROM project_files
			WHERE project_id = $1`,
			firstProject.ID,
		).Scan(&projectFileCount); err != nil {
			_ = savepoint.Rollback(ctx)
			t.Fatalf("count project files after deletion: %v", err)
		}

		if projectFileCount != 0 {
			_ = savepoint.Rollback(ctx)
			t.Fatalf(
				"expected project files to be deleted, got %d",
				projectFileCount,
			)
		}

		var revisionFileCount int
		if err := savepoint.QueryRow(
			ctx,
			`SELECT count(*)
			FROM project_revision_files
			WHERE project_id = $1`,
			firstProject.ID,
		).Scan(&revisionFileCount); err != nil {
			_ = savepoint.Rollback(ctx)
			t.Fatalf(
				"count revision files after project deletion: %v",
				err,
			)
		}

		if revisionFileCount != 0 {
			_ = savepoint.Rollback(ctx)
			t.Fatalf(
				"expected revision file references to be deleted, got %d",
				revisionFileCount,
			)
		}

		var orphanObjectCount int
		if err := savepoint.QueryRow(
			ctx,
			`SELECT count(*)
			FROM content_objects
			WHERE sha256 = $1`,
			firstOnlyHash,
		).Scan(&orphanObjectCount); err != nil {
			_ = savepoint.Rollback(ctx)
			t.Fatalf("count orphan content object: %v", err)
		}

		if orphanObjectCount != 1 {
			_ = savepoint.Rollback(ctx)
			t.Fatalf(
				"expected orphan content object to remain, got %d",
				orphanObjectCount,
			)
		}

		if err := savepoint.Rollback(ctx); err != nil {
			t.Fatalf("rollback project deletion savepoint: %v", err)
		}
	})
}

func TestFileStorageQueries(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}

	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	queries := dbgen.New(tx)

	firstUser, err := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{
			Email:       "storage-queries-one@example.com",
			DisplayName: "Storage Queries User One",
		},
	)
	if err != nil {
		t.Fatalf("create first storage query user: %v", err)
	}

	secondUser, err := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{
			Email:       "storage-queries-two@example.com",
			DisplayName: "Storage Queries User Two",
		},
	)
	if err != nil {
		t.Fatalf("create second storage query user: %v", err)
	}

	firstProject, err := queries.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: firstUser.ID,
			Name:        "Storage Queries Project One",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create first storage query project: %v", err)
	}

	secondProject, err := queries.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: secondUser.ID,
			Name:        "Storage Queries Project Two",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create second storage query project: %v", err)
	}

	firstHash := strings.Repeat("1", 64)
	secondHash := strings.Repeat("2", 64)
	thirdHash := strings.Repeat("3", 64)

	firstObject, err := queries.EnsureContentObject(
		ctx,
		dbgen.EnsureContentObjectParams{
			Sha256:    firstHash,
			SizeBytes: 512,
		},
	)
	if err != nil {
		t.Fatalf("ensure first content object: %v", err)
	}

	if firstObject.Sha256 != firstHash {
		t.Fatalf(
			"expected content hash %q, got %q",
			firstHash,
			firstObject.Sha256,
		)
	}

	if firstObject.SizeBytes != 512 {
		t.Fatalf(
			"expected content size %d, got %d",
			512,
			firstObject.SizeBytes,
		)
	}

	sameObject, err := queries.EnsureContentObject(
		ctx,
		dbgen.EnsureContentObjectParams{
			Sha256:    firstHash,
			SizeBytes: 512,
		},
	)
	if err != nil {
		t.Fatalf("ensure existing matching content object: %v", err)
	}

	if sameObject.Sha256 != firstObject.Sha256 ||
		sameObject.SizeBytes != firstObject.SizeBytes ||
		!sameObject.CreatedAt.Equal(firstObject.CreatedAt) {
		t.Fatalf(
			"expected idempotent ensure to return existing object, got %+v",
			sameObject,
		)
	}

	_, err = queries.EnsureContentObject(
		ctx,
		dbgen.EnsureContentObjectParams{
			Sha256:    firstHash,
			SizeBytes: 513,
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected size mismatch to return pgx.ErrNoRows, got %v",
			err,
		)
	}

	foundObject, err := queries.GetContentObjectBySHA256(
		ctx,
		firstHash,
	)
	if err != nil {
		t.Fatalf("get content object by hash: %v", err)
	}

	if foundObject.Sha256 != firstObject.Sha256 ||
		foundObject.SizeBytes != firstObject.SizeBytes ||
		!foundObject.CreatedAt.Equal(firstObject.CreatedAt) {
		t.Fatalf(
			"expected persisted content object %+v, got %+v",
			firstObject,
			foundObject,
		)
	}

	for _, object := range []dbgen.EnsureContentObjectParams{
		{
			Sha256:    secondHash,
			SizeBytes: 1024,
		},
		{
			Sha256:    thirdHash,
			SizeBytes: 2048,
		},
	} {
		if _, err := queries.EnsureContentObject(ctx, object); err != nil {
			t.Fatalf(
				"ensure additional content object %q: %v",
				object.Sha256,
				err,
			)
		}
	}

	firstFile, err := queries.CreateProjectFile(
		ctx,
		dbgen.CreateProjectFileParams{
			ProjectID:        firstProject.ID,
			UploadedByUserID: firstUser.ID,
			ContentSha256:    firstHash,
			OriginalFilename: "gripper.step",
			MediaType:        nil,
		},
	)
	if err != nil {
		t.Fatalf("create first project file: %v", err)
	}

	if firstFile.MediaType != nil {
		t.Fatalf(
			"expected nil media type, got %q",
			*firstFile.MediaType,
		)
	}

	stepMediaType := "model/step"

	secondFile, err := queries.CreateProjectFile(
		ctx,
		dbgen.CreateProjectFileParams{
			ProjectID:        firstProject.ID,
			UploadedByUserID: firstUser.ID,
			ContentSha256:    secondHash,
			OriginalFilename: "jaw.step",
			MediaType:        &stepMediaType,
		},
	)
	if err != nil {
		t.Fatalf("create second project file: %v", err)
	}

	if secondFile.MediaType == nil ||
		*secondFile.MediaType != stepMediaType {
		t.Fatalf(
			"expected media type %q, got %v",
			stepMediaType,
			secondFile.MediaType,
		)
	}

	thirdFile, err := queries.CreateProjectFile(
		ctx,
		dbgen.CreateProjectFileParams{
			ProjectID:        firstProject.ID,
			UploadedByUserID: firstUser.ID,
			ContentSha256:    thirdHash,
			OriginalFilename: "mount.step",
			MediaType:        &stepMediaType,
		},
	)
	if err != nil {
		t.Fatalf("create third project file: %v", err)
	}

	secondProjectFile, err := queries.CreateProjectFile(
		ctx,
		dbgen.CreateProjectFileParams{
			ProjectID:        secondProject.ID,
			UploadedByUserID: secondUser.ID,
			ContentSha256:    firstHash,
			OriginalFilename: "same-content.step",
			MediaType:        nil,
		},
	)
	if err != nil {
		t.Fatalf("create second-project file: %v", err)
	}

	if secondProjectFile.ContentSha256 != firstHash {
		t.Fatalf(
			"expected shared content hash %q, got %q",
			firstHash,
			secondProjectFile.ContentSha256,
		)
	}

	foundFile, err := queries.GetProjectFileByIDAndProject(
		ctx,
		dbgen.GetProjectFileByIDAndProjectParams{
			ProjectFileID: firstFile.ID,
			ProjectID:     firstProject.ID,
		},
	)
	if err != nil {
		t.Fatalf("get project file by ID and project: %v", err)
	}

	if foundFile.ID != firstFile.ID ||
		foundFile.ProjectID != firstProject.ID ||
		foundFile.UploadedByUserID != firstUser.ID ||
		foundFile.ContentSha256 != firstHash ||
		foundFile.OriginalFilename != "gripper.step" ||
		foundFile.MediaType != nil {
		t.Fatalf(
			"unexpected project file returned: %+v",
			foundFile,
		)
	}

	_, err = queries.GetProjectFileByIDAndProject(
		ctx,
		dbgen.GetProjectFileByIDAndProjectParams{
			ProjectFileID: firstFile.ID,
			ProjectID:     secondProject.ID,
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected cross-project file lookup to return pgx.ErrNoRows, got %v",
			err,
		)
	}

	olderTime := time.Date(
		2026,
		time.January,
		1,
		12,
		0,
		0,
		0,
		time.UTC,
	)
	newerTime := olderTime.Add(time.Hour)

	if _, err := tx.Exec(
		ctx,
		`UPDATE project_files
		SET created_at = $1
		WHERE id = $2`,
		olderTime,
		firstFile.ID,
	); err != nil {
		t.Fatalf("set first project file timestamp: %v", err)
	}

	for _, fileID := range []uuid.UUID{
		secondFile.ID,
		thirdFile.ID,
	} {
		if _, err := tx.Exec(
			ctx,
			`UPDATE project_files
			SET created_at = $1
			WHERE id = $2`,
			newerTime,
			fileID,
		); err != nil {
			t.Fatalf(
				"set project file %s timestamp: %v",
				fileID,
				err,
			)
		}
	}

	files, err := queries.ListProjectFilesByProject(
		ctx,
		firstProject.ID,
	)
	if err != nil {
		t.Fatalf("list project files: %v", err)
	}

	if len(files) != 3 {
		t.Fatalf(
			"expected 3 first-project files, got %d",
			len(files),
		)
	}

	expectedNewestFirst := secondFile.ID
	expectedNewestSecond := thirdFile.ID

	if secondFile.ID.String() < thirdFile.ID.String() {
		expectedNewestFirst = thirdFile.ID
		expectedNewestSecond = secondFile.ID
	}

	expectedOrder := []uuid.UUID{
		expectedNewestFirst,
		expectedNewestSecond,
		firstFile.ID,
	}

	for i, expectedID := range expectedOrder {
		if files[i].ID != expectedID {
			t.Fatalf(
				"expected project file %d to be %s, got %s",
				i,
				expectedID,
				files[i].ID,
			)
		}

		if files[i].ProjectID != firstProject.ID {
			t.Fatalf(
				"expected listed file project %s, got %s",
				firstProject.ID,
				files[i].ProjectID,
			)
		}
	}
}

func TestSessionQueries(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}

	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	queries := dbgen.New(tx)

	user, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "sessions@example.com",
		DisplayName: "Session Test User",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	activeTokenHash := []byte("active-session-token-hash")

	activeSession, err := queries.CreateSession(ctx, dbgen.CreateSessionParams{
		UserID:    user.ID,
		TokenHash: activeTokenHash,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create active session: %v", err)
	}

	if !bytes.Equal(activeSession.TokenHash, activeTokenHash) {
		t.Fatalf(
			"expected token hash %q, got %q",
			activeTokenHash,
			activeSession.TokenHash,
		)
	}

	resolvedSession, err := queries.GetActiveSessionByTokenHash(
		ctx,
		activeTokenHash,
	)
	if err != nil {
		t.Fatalf("get active session by token hash: %v", err)
	}

	if resolvedSession.ID != activeSession.ID {
		t.Fatalf(
			"expected session ID %s, got %s",
			activeSession.ID,
			resolvedSession.ID,
		)
	}

	if resolvedSession.UserID != user.ID {
		t.Fatalf(
			"expected user ID %s, got %s",
			user.ID,
			resolvedSession.UserID,
		)
	}

	expiredTokenHash := []byte("expired-session-token-hash")

	expiredSession, err := queries.CreateSession(ctx, dbgen.CreateSessionParams{
		UserID:    user.ID,
		TokenHash: expiredTokenHash,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create session for expiration test: %v", err)
	}

	_, err = tx.Exec(
		ctx,
		`
			UPDATE sessions
			SET
				created_at = now() - interval '2 hours',
				expires_at = now() - interval '1 hour'
			WHERE id = $1
		`,
		expiredSession.ID,
	)
	if err != nil {
		t.Fatalf("expire session: %v", err)
	}

	_, err = queries.GetActiveSessionByTokenHash(ctx, expiredTokenHash)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected expired session lookup to return pgx.ErrNoRows, got %v",
			err,
		)
	}

	deleteTokenHash := []byte("delete-session-token-hash")

	_, err = queries.CreateSession(ctx, dbgen.CreateSessionParams{
		UserID:    user.ID,
		TokenHash: deleteTokenHash,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create session for deletion test: %v", err)
	}

	if err := queries.DeleteSession(ctx, deleteTokenHash); err != nil {
		t.Fatalf("delete session: %v", err)
	}

	_, err = queries.GetActiveSessionByTokenHash(ctx, deleteTokenHash)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected deleted session lookup to return pgx.ErrNoRows, got %v",
			err,
		)
	}

	deletedExpiredSessions, err := queries.DeleteExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("delete expired sessions: %v", err)
	}

	if deletedExpiredSessions != 1 {
		t.Fatalf(
			"expected 1 expired session to be deleted, got %d",
			deletedExpiredSessions,
		)
	}

	cascadeUser, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "cascade@example.com",
		DisplayName: "Cascade Test User",
	})
	if err != nil {
		t.Fatalf("create cascade test user: %v", err)
	}

	cascadeTokenHash := []byte("cascade-session-token-hash")

	cascadeSession, err := queries.CreateSession(ctx, dbgen.CreateSessionParams{
		UserID:    cascadeUser.ID,
		TokenHash: cascadeTokenHash,
		ExpiresAt: time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create cascade test session: %v", err)
	}

	if _, err := tx.Exec(
		ctx,
		"DELETE FROM users WHERE id = $1",
		cascadeUser.ID,
	); err != nil {
		t.Fatalf("delete cascade test user: %v", err)
	}

	var cascadeSessionCount int

	if err := tx.QueryRow(
		ctx,
		"SELECT count(*) FROM sessions WHERE id = $1",
		cascadeSession.ID,
	).Scan(&cascadeSessionCount); err != nil {
		t.Fatalf("count cascade test sessions: %v", err)
	}

	if cascadeSessionCount != 0 {
		t.Fatalf(
			"expected user deletion to remove session, got %d sessions",
			cascadeSessionCount,
		)
	}
}

func TestPasswordCredentialQueries(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}

	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	queries := dbgen.New(tx)

	user, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "credentials@example.com",
		DisplayName: "Credential Test User",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	const passwordHash = "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA"

	credential, err := queries.CreatePasswordCredential(
		ctx,
		dbgen.CreatePasswordCredentialParams{
			UserID:       user.ID,
			PasswordHash: passwordHash,
		},
	)
	if err != nil {
		t.Fatalf("create password credential: %v", err)
	}

	if credential.UserID != user.ID {
		t.Fatalf(
			"expected credential user ID %s, got %s",
			user.ID,
			credential.UserID,
		)
	}

	if credential.PasswordHash != passwordHash {
		t.Fatalf(
			"expected password hash %q, got %q",
			passwordHash,
			credential.PasswordHash,
		)
	}

	if credential.CreatedAt.IsZero() {
		t.Fatal("expected credential created_at to be set")
	}

	if credential.UpdatedAt.IsZero() {
		t.Fatal("expected credential updated_at to be set")
	}

	foundCredential, err := queries.GetPasswordCredentialByUserID(
		ctx,
		user.ID,
	)
	if err != nil {
		t.Fatalf("get password credential by user ID: %v", err)
	}

	if foundCredential.UserID != credential.UserID {
		t.Fatalf(
			"expected credential user ID %s, got %s",
			credential.UserID,
			foundCredential.UserID,
		)
	}

	if foundCredential.PasswordHash != passwordHash {
		t.Fatalf(
			"expected password hash %q, got %q",
			passwordHash,
			foundCredential.PasswordHash,
		)
	}

	cascadeUser, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "credential-cascade@example.com",
		DisplayName: "Credential Cascade User",
	})
	if err != nil {
		t.Fatalf("create cascade user: %v", err)
	}

	_, err = queries.CreatePasswordCredential(
		ctx,
		dbgen.CreatePasswordCredentialParams{
			UserID:       cascadeUser.ID,
			PasswordHash: passwordHash,
		},
	)
	if err != nil {
		t.Fatalf("create cascade password credential: %v", err)
	}

	if _, err := tx.Exec(
		ctx,
		"DELETE FROM users WHERE id = $1",
		cascadeUser.ID,
	); err != nil {
		t.Fatalf("delete cascade user: %v", err)
	}

	_, err = queries.GetPasswordCredentialByUserID(
		ctx,
		cascadeUser.ID,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected deleted user's credential lookup to return pgx.ErrNoRows, got %v",
			err,
		)
	}

	_, err = queries.CreatePasswordCredential(
		ctx,
		dbgen.CreatePasswordCredentialParams{
			UserID:       user.ID,
			PasswordHash: passwordHash,
		},
	)

	var pgErr *pgconn.PgError

	if !errors.As(err, &pgErr) {
		t.Fatalf(
			"expected duplicate credential to return PostgreSQL error, got %v",
			err,
		)
	}

	if pgErr.Code != "23505" {
		t.Fatalf(
			"expected unique violation code %q, got %q",
			"23505",
			pgErr.Code,
		)
	}

	if pgErr.ConstraintName != "password_credentials_pkey" {
		t.Fatalf(
			"expected constraint %q, got %q",
			"password_credentials_pkey",
			pgErr.ConstraintName,
		)
	}

	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback transaction: %v", err)
	}

	var credentialCount int

	if err := pool.QueryRow(
		ctx,
		`
			SELECT count(*)
			FROM password_credentials
			WHERE user_id = $1
		`,
		user.ID,
	).Scan(&credentialCount); err != nil {
		t.Fatalf("count credentials after rollback: %v", err)
	}

	if credentialCount != 0 {
		t.Fatalf(
			"expected rollback to remove credential, got %d credentials",
			credentialCount,
		)
	}
}

func TestRegistrationStoreCreatesUserAndCredentialAtomically(
	t *testing.T,
) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	store := database.NewRegistrationStore(pool)

	const passwordHash = "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA"

	user, err := store.CreateUserWithPassword(
		ctx,
		dbgen.CreateUserParams{
			Email:       "atomic-registration@example.com",
			DisplayName: "Atomic Registration User",
		},
		passwordHash,
	)
	if err != nil {
		t.Fatalf("create user with password: %v", err)
	}

	defer func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM users WHERE id = $1",
			user.ID,
		)
		if err != nil {
			t.Errorf("delete registration test user: %v", err)
		}
	}()

	queries := dbgen.New(pool)

	foundUser, err := queries.GetUserByEmail(
		ctx,
		"atomic-registration@example.com",
	)
	if err != nil {
		t.Fatalf("get registered user: %v", err)
	}

	if foundUser.ID != user.ID {
		t.Fatalf(
			"expected user ID %s, got %s",
			user.ID,
			foundUser.ID,
		)
	}

	credential, err := queries.GetPasswordCredentialByUserID(
		ctx,
		user.ID,
	)
	if err != nil {
		t.Fatalf("get registered password credential: %v", err)
	}

	if credential.PasswordHash != passwordHash {
		t.Fatalf(
			"expected password hash %q, got %q",
			passwordHash,
			credential.PasswordHash,
		)
	}
}

func TestRegistrationStoreRollsBackUserWhenCredentialFails(
	t *testing.T,
) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	store := database.NewRegistrationStore(pool)

	const email = "rollback-registration@example.com"

	_, err = store.CreateUserWithPassword(
		ctx,
		dbgen.CreateUserParams{
			Email:       email,
			DisplayName: "Rollback Registration User",
		},
		"",
	)
	if err == nil {
		t.Fatal(
			"expected blank password hash to fail registration",
		)
	}

	queries := dbgen.New(pool)

	_, err = queries.GetUserByEmail(ctx, email)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected failed registration to leave no user, got %v",
			err,
		)
	}
}
