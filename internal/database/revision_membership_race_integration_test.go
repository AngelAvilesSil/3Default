//go:build integration

package database_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestConcurrentInsertCannotModifyNewlyFinalizedRevision(t *testing.T) {
	ctx, pool, _, queries, userID, projectID, _ :=
		setupVersioningStoreTest(t)

	projectFile := createVersioningProjectFile(
		t, ctx, pool, queries, userID, projectID,
		"concurrent-membership",
	)

	// Transaction A creates and finalizes a revision but has not committed.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin revision transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	revisionID := uuid.New()
	_, err = tx.Exec(
		ctx,
		`INSERT INTO project_revisions
			(id, project_id, author_user_id, message)
		 VALUES ($1, $2, $3, $4)`,
		revisionID, projectID, userID, "Concurrent finalization",
	)
	if err != nil {
		t.Fatalf("insert revision: %v", err)
	}

	_, err = tx.Exec(
		ctx,
		`UPDATE project_revisions
		 SET membership_finalized = TRUE
		 WHERE project_id = $1 AND id = $2`,
		projectID, revisionID,
	)
	if err != nil {
		t.Fatalf("finalize revision: %v", err)
	}

	// Transaction B attempts to add a file before A commits.
	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire concurrent connection: %v", err)
	}
	defer conn.Release()

	var backendPID int32
	if err := conn.QueryRow(
		ctx, "SELECT pg_backend_pid()",
	).Scan(&backendPID); err != nil {
		t.Fatalf("get backend PID: %v", err)
	}

	insertCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	result := make(chan error, 1)
	go func() {
		_, insertErr := conn.Exec(
			insertCtx,
			`INSERT INTO project_revision_files
				(project_id, revision_id, project_file_id)
			 VALUES ($1, $2, $3)`,
			projectID, revisionID, projectFile.ID,
		)
		result <- insertErr
	}()

	var insertErr error
	completed := false
	blocked := false
	deadline := time.Now().Add(5 * time.Second)

	// Determine whether the concurrent statement has completed or
	// reached a PostgreSQL lock wait before committing A.
	for time.Now().Before(deadline) {
		select {
		case insertErr = <-result:
			completed = true
		default:
		}
		if completed {
			break
		}

		if err := pool.QueryRow(
			ctx,
			`SELECT COALESCE(wait_event_type = 'Lock', FALSE)
			 FROM pg_stat_activity WHERE pid = $1`,
			backendPID,
		).Scan(&blocked); err != nil {
			t.Fatalf("inspect concurrent statement: %v", err)
		}
		if blocked {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	if !completed && !blocked {
		t.Fatal("could not establish concurrent insertion timing")
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit finalized revision: %v", err)
	}

	if !completed {
		insertErr = <-result
	}

	// An insertion racing finalization must never succeed.
	var pgErr *pgconn.PgError
	if !errors.As(insertErr, &pgErr) ||
		(pgErr.Code != "P0001" && pgErr.Code != "23503") {
		t.Fatalf(
			"expected concurrent insertion rejection, got %v",
			insertErr,
		)
	}

	var count int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*) FROM project_revision_files
		 WHERE project_id = $1 AND revision_id = $2`,
		projectID, revisionID,
	).Scan(&count); err != nil {
		t.Fatalf("count revision membership: %v", err)
	}
	if count != 0 {
		t.Fatalf("concurrent insertion changed finalized snapshot")
	}
}
