//go:build integration

package database_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/jackc/pgx/v5"
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
