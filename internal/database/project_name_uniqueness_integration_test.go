//go:build integration

package database_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/projects"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProjectStoreRejectsCaseInsensitiveDuplicateCreate(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	queries := dbgen.New(pool)

	owner, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
		Email:       "project-name-" + uuid.NewString() + "@example.com",
		DisplayName: "Project Name Test Owner",
	})
	if err != nil {
		t.Fatalf("create test owner: %v", err)
	}

	// Registered after pool.Close so this cleanup executes first.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cleanupCancel()

		if _, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM projects WHERE owner_user_id = $1",
			owner.ID,
		); err != nil {
			t.Errorf("clean up test projects: %v", err)
		}

		if _, err := pool.Exec(
			cleanupCtx,
			"DELETE FROM users WHERE id = $1",
			owner.ID,
		); err != nil {
			t.Errorf("clean up test owner: %v", err)
		}
	})

	store := database.NewProjectStore(pool)

	_, err = store.CreateProject(ctx, dbgen.CreateProjectParams{
		OwnerUserID: owner.ID,
		Name:        "Robot Gripper",
	})
	if err != nil {
		t.Fatalf("create original project: %v", err)
	}

	_, err = store.CreateProject(ctx, dbgen.CreateProjectParams{
		OwnerUserID: owner.ID,
		Name:        "robot gripper",
	})
	if !errors.Is(err, projects.ErrNameConflict) {
		t.Fatalf(
			"duplicate creation error = %v, want ErrNameConflict",
			err,
		)
	}

	var projectCount int
	var branchCount int

	err = pool.QueryRow(ctx, `
        SELECT
            (
                SELECT count(*)
                FROM projects
                WHERE owner_user_id = $1
            ),
            (
                SELECT count(*)
                FROM project_branches AS b
                JOIN projects AS p ON p.id = b.project_id
                WHERE p.owner_user_id = $1
            )
    `, owner.ID).Scan(&projectCount, &branchCount)
	if err != nil {
		t.Fatalf("count projects and branches: %v", err)
	}

	if projectCount != 1 {
		t.Fatalf("project count = %d, want 1", projectCount)
	}
	if branchCount != 1 {
		t.Fatalf("default branch count = %d, want 1", branchCount)
	}
}

func TestProjectStoreEnforcesNameUniquenessDuringRename(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	queries := dbgen.New(pool)
	store := database.NewProjectStore(pool)

	createOwner := func(displayName string) uuid.UUID {
		t.Helper()

		user, err := queries.CreateUser(ctx, dbgen.CreateUserParams{
			Email:       "project-rename-" + uuid.NewString() + "@example.com",
			DisplayName: displayName,
		})
		if err != nil {
			t.Fatalf("create test owner: %v", err)
		}

		t.Cleanup(func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(
				context.Background(),
				10*time.Second,
			)
			defer cleanupCancel()

			_, err := pool.Exec(
				cleanupCtx,
				"DELETE FROM projects WHERE owner_user_id = $1",
				user.ID,
			)
			if err != nil {
				t.Errorf("clean up test projects: %v", err)
			}

			_, err = pool.Exec(
				cleanupCtx,
				"DELETE FROM users WHERE id = $1",
				user.ID,
			)
			if err != nil {
				t.Errorf("clean up test owner: %v", err)
			}
		})

		return user.ID
	}

	createProject := func(ownerID uuid.UUID, name string) dbgen.Project {
		t.Helper()

		project, err := store.CreateProject(
			ctx,
			dbgen.CreateProjectParams{
				OwnerUserID: ownerID,
				Name:        name,
			},
		)
		if err != nil {
			t.Fatalf("create project %q: %v", name, err)
		}

		return project
	}

	firstOwner := createOwner("First Project Owner")
	secondOwner := createOwner("Second Project Owner")

	gripper := createProject(firstOwner, "Robot Gripper")
	gearbox := createProject(firstOwner, "Gearbox")

	// Identical names are allowed across different owners.
	otherOwnersGripper := createProject(secondOwner, "Robot Gripper")
	if otherOwnersGripper.OwnerUserID != secondOwner {
		t.Fatal("project was created for the wrong owner")
	}

	// A project may change only the capitalization of its own name.
	renamed, err := store.UpdateProjectMetadataByIDAndOwner(
		ctx,
		dbgen.UpdateProjectMetadataByIDAndOwnerParams{
			ProjectID:   gripper.ID,
			OwnerUserID: firstOwner,
			NameSet:     true,
			Name:        "ROBOT GRIPPER",
		},
	)
	if err != nil {
		t.Fatalf("rename project with different capitalization: %v", err)
	}
	if renamed.Name != "ROBOT GRIPPER" {
		t.Fatalf("renamed project name = %q, want ROBOT GRIPPER", renamed.Name)
	}

	// Updating only the description must preserve the current name.
	description := "Mechanical assembly development"

	updated, err := store.UpdateProjectMetadataByIDAndOwner(
		ctx,
		dbgen.UpdateProjectMetadataByIDAndOwnerParams{
			ProjectID:      gripper.ID,
			OwnerUserID:    firstOwner,
			DescriptionSet: true,
			Description:    &description,
		},
	)
	if err != nil {
		t.Fatalf("update project description: %v", err)
	}
	if updated.Name != "ROBOT GRIPPER" {
		t.Fatalf("description update changed project name to %q", updated.Name)
	}

	// Owner scoping must still protect projects from other users.
	_, err = store.UpdateProjectMetadataByIDAndOwner(
		ctx,
		dbgen.UpdateProjectMetadataByIDAndOwnerParams{
			ProjectID:   gripper.ID,
			OwnerUserID: secondOwner,
			NameSet:     true,
			Name:        "Unauthorized Rename",
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("cross-owner update error = %v, want pgx.ErrNoRows", err)
	}

	// A different project of the same owner cannot take an existing name.
	_, err = store.UpdateProjectMetadataByIDAndOwner(
		ctx,
		dbgen.UpdateProjectMetadataByIDAndOwnerParams{
			ProjectID:   gearbox.ID,
			OwnerUserID: firstOwner,
			NameSet:     true,
			Name:        "robot gripper",
		},
	)
	if !errors.Is(err, projects.ErrNameConflict) {
		t.Fatalf(
			"duplicate rename error = %v, want ErrNameConflict",
			err,
		)
	}

	// The failed rename must leave the original project unchanged.
	unchanged, err := store.GetProjectByIDAndOwner(
		ctx,
		dbgen.GetProjectByIDAndOwnerParams{
			ProjectID:   gearbox.ID,
			OwnerUserID: firstOwner,
		},
	)
	if err != nil {
		t.Fatalf("retrieve project after failed rename: %v", err)
	}
	if unchanged.Name != "Gearbox" {
		t.Fatalf("project name after failed rename = %q, want Gearbox", unchanged.Name)
	}
}
