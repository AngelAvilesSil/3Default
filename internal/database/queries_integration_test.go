//go:build integration

package database_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
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
}
