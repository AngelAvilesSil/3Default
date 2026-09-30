//go:build integration

package database_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/versioning"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProjectStoreCreatesRevisionAndAdvancesBranchHead(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	firstRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               branchID,
			AuthorUserID:           userID,
			Message:                "Initial revision",
			ExpectedHeadRevisionID: nil,
			MergeParentRevisionID:  nil,
		},
	)
	if err != nil {
		t.Fatalf("create initial revision: %v", err)
	}

	if firstRevision.ParentRevisionID.Valid {
		t.Fatalf(
			"expected initial revision parent to be null, got %s",
			uuid.UUID(firstRevision.ParentRevisionID.Bytes),
		)
	}

	if firstRevision.MergeParentRevisionID.Valid {
		t.Fatalf(
			"expected initial merge parent to be null, got %s",
			uuid.UUID(firstRevision.MergeParentRevisionID.Bytes),
		)
	}

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		branchID,
		firstRevision.ID,
	)

	secondRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               branchID,
			AuthorUserID:           userID,
			Message:                "Second revision",
			ExpectedHeadRevisionID: &firstRevision.ID,
			MergeParentRevisionID:  nil,
		},
	)
	if err != nil {
		t.Fatalf("create second revision: %v", err)
	}

	if !secondRevision.ParentRevisionID.Valid {
		t.Fatal("expected second revision parent to be set")
	}

	if uuid.UUID(secondRevision.ParentRevisionID.Bytes) != firstRevision.ID {
		t.Fatalf(
			"expected second revision parent %s, got %s",
			firstRevision.ID,
			uuid.UUID(secondRevision.ParentRevisionID.Bytes),
		)
	}

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		branchID,
		secondRevision.ID,
	)

	var revisionCount int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revisions
		 WHERE project_id = $1`,
		projectID,
	).Scan(&revisionCount); err != nil {
		t.Fatalf("count project revisions: %v", err)
	}

	if revisionCount != 2 {
		t.Fatalf(
			"expected 2 project revisions, got %d",
			revisionCount,
		)
	}
}

func TestProjectStoreRollsBackRevisionOnBranchHeadConflict(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	firstRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     branchID,
			AuthorUserID: userID,
			Message:      "Initial revision",
		},
	)
	if err != nil {
		t.Fatalf("create initial revision: %v", err)
	}

	secondRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               branchID,
			AuthorUserID:           userID,
			Message:                "Second revision",
			ExpectedHeadRevisionID: &firstRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create second revision: %v", err)
	}

	var revisionsBefore int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revisions
		 WHERE project_id = $1`,
		projectID,
	).Scan(&revisionsBefore); err != nil {
		t.Fatalf("count revisions before conflict: %v", err)
	}

	_, err = store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               branchID,
			AuthorUserID:           userID,
			Message:                "Stale candidate revision",
			ExpectedHeadRevisionID: &firstRevision.ID,
		},
	)
	if !errors.Is(err, versioning.ErrBranchHeadConflict) {
		t.Fatalf(
			"expected branch head conflict, got %v",
			err,
		)
	}

	var revisionsAfter int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revisions
		 WHERE project_id = $1`,
		projectID,
	).Scan(&revisionsAfter); err != nil {
		t.Fatalf("count revisions after conflict: %v", err)
	}

	if revisionsAfter != revisionsBefore {
		t.Fatalf(
			"expected failed revision to roll back; count changed from %d to %d",
			revisionsBefore,
			revisionsAfter,
		)
	}

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		branchID,
		secondRevision.ID,
	)
}

func TestProjectStoreRollsBackRevisionWhenBranchIsMissing(
	t *testing.T,
) {
	ctx, pool, store, _, userID, projectID, _ :=
		setupVersioningStoreTest(t)

	var revisionsBefore int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revisions
		 WHERE project_id = $1`,
		projectID,
	).Scan(&revisionsBefore); err != nil {
		t.Fatalf("count revisions before missing branch: %v", err)
	}

	_, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     uuid.New(),
			AuthorUserID: userID,
			Message:      "Missing branch candidate",
		},
	)
	if !errors.Is(err, versioning.ErrBranchNotFound) {
		t.Fatalf(
			"expected project branch not found, got %v",
			err,
		)
	}

	var revisionsAfter int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revisions
		 WHERE project_id = $1`,
		projectID,
	).Scan(&revisionsAfter); err != nil {
		t.Fatalf("count revisions after missing branch: %v", err)
	}

	if revisionsAfter != revisionsBefore {
		t.Fatalf(
			"expected missing-branch revision to roll back; count changed from %d to %d",
			revisionsBefore,
			revisionsAfter,
		)
	}
}

func TestProjectStoreCreatesMergeRevisionAndAdvancesTargetBranch(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, mainBranchID :=
		setupVersioningStoreTest(t)

	rootRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     mainBranchID,
			AuthorUserID: userID,
			Message:      "Initial revision",
		},
	)
	if err != nil {
		t.Fatalf("create root revision: %v", err)
	}

	var sideBranchID uuid.UUID
	if err := pool.QueryRow(
		ctx,
		`INSERT INTO project_branches (
			project_id,
			name,
			head_revision_id
		)
		VALUES ($1, 'feature', $2)
		RETURNING id`,
		projectID,
		rootRevision.ID,
	).Scan(&sideBranchID); err != nil {
		t.Fatalf("create side branch: %v", err)
	}

	mainRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               mainBranchID,
			AuthorUserID:           userID,
			Message:                "Main branch revision",
			ExpectedHeadRevisionID: &rootRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create main branch revision: %v", err)
	}

	sideRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               sideBranchID,
			AuthorUserID:           userID,
			Message:                "Feature branch revision",
			ExpectedHeadRevisionID: &rootRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create side branch revision: %v", err)
	}

	mergeRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               mainBranchID,
			AuthorUserID:           userID,
			Message:                "Merge feature branch",
			ExpectedHeadRevisionID: &mainRevision.ID,
			MergeParentRevisionID:  &sideRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create merge revision: %v", err)
	}

	if !mergeRevision.ParentRevisionID.Valid {
		t.Fatal("expected merge revision primary parent to be set")
	}

	actualPrimaryParent :=
		uuid.UUID(mergeRevision.ParentRevisionID.Bytes)

	if actualPrimaryParent != mainRevision.ID {
		t.Fatalf(
			"expected merge primary parent %s, got %s",
			mainRevision.ID,
			actualPrimaryParent,
		)
	}

	if !mergeRevision.MergeParentRevisionID.Valid {
		t.Fatal("expected merge revision second parent to be set")
	}

	actualMergeParent :=
		uuid.UUID(mergeRevision.MergeParentRevisionID.Bytes)

	if actualMergeParent != sideRevision.ID {
		t.Fatalf(
			"expected merge second parent %s, got %s",
			sideRevision.ID,
			actualMergeParent,
		)
	}

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		mainBranchID,
		mergeRevision.ID,
	)

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		sideBranchID,
		sideRevision.ID,
	)
}

func TestProjectStoreCreatesEmptyBranch(
	t *testing.T,
) {
	ctx, _, store, queries, _, projectID, _ :=
		setupVersioningStoreTest(t)

	branch, err := store.CreateBranch(
		ctx,
		versioning.CreateBranchParams{
			ProjectID: projectID,
			Name:      "empty-feature",
		},
	)
	if err != nil {
		t.Fatalf("create empty project branch: %v", err)
	}

	if branch.ProjectID != projectID {
		t.Fatalf(
			"expected branch project ID %s, got %s",
			projectID,
			branch.ProjectID,
		)
	}

	if branch.Name != "empty-feature" {
		t.Fatalf(
			"expected branch name %q, got %q",
			"empty-feature",
			branch.Name,
		)
	}

	if branch.HeadRevisionID.Valid {
		t.Fatalf(
			"expected empty branch head to be null, got %s",
			uuid.UUID(branch.HeadRevisionID.Bytes),
		)
	}

	persisted, err := queries.GetProjectBranchByIDAndProject(
		ctx,
		dbgen.GetProjectBranchByIDAndProjectParams{
			BranchID:  branch.ID,
			ProjectID: projectID,
		},
	)
	if err != nil {
		t.Fatalf("get created empty branch: %v", err)
	}

	if persisted.ID != branch.ID {
		t.Fatalf(
			"expected persisted branch ID %s, got %s",
			branch.ID,
			persisted.ID,
		)
	}

	if persisted.HeadRevisionID.Valid {
		t.Fatalf(
			"expected persisted empty branch head to be null, got %s",
			uuid.UUID(persisted.HeadRevisionID.Bytes),
		)
	}
}

func TestProjectStoreCreatesBranchAtExactRevision(
	t *testing.T,
) {
	ctx, _, store, queries, userID, projectID, mainBranchID :=
		setupVersioningStoreTest(t)

	rootRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     mainBranchID,
			AuthorUserID: userID,
			Message:      "Initial revision",
		},
	)
	if err != nil {
		t.Fatalf("create root revision: %v", err)
	}

	branch, err := store.CreateBranch(
		ctx,
		versioning.CreateBranchParams{
			ProjectID:      projectID,
			Name:           "feature-from-root",
			HeadRevisionID: &rootRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create branch at revision: %v", err)
	}

	if !branch.HeadRevisionID.Valid {
		t.Fatal("expected created branch head to be set")
	}

	actualHead := uuid.UUID(branch.HeadRevisionID.Bytes)
	if actualHead != rootRevision.ID {
		t.Fatalf(
			"expected created branch head %s, got %s",
			rootRevision.ID,
			actualHead,
		)
	}

	persisted, err := queries.GetProjectBranchByIDAndProject(
		ctx,
		dbgen.GetProjectBranchByIDAndProjectParams{
			BranchID:  branch.ID,
			ProjectID: projectID,
		},
	)
	if err != nil {
		t.Fatalf("get created branch: %v", err)
	}

	if !persisted.HeadRevisionID.Valid {
		t.Fatal("expected persisted branch head to be set")
	}

	persistedHead := uuid.UUID(persisted.HeadRevisionID.Bytes)
	if persistedHead != rootRevision.ID {
		t.Fatalf(
			"expected persisted branch head %s, got %s",
			rootRevision.ID,
			persistedHead,
		)
	}
}

func TestProjectStoreMapsDuplicateBranchNameToConflict(
	t *testing.T,
) {
	ctx, _, store, _, _, projectID, _ :=
		setupVersioningStoreTest(t)

	_, err := store.CreateBranch(
		ctx,
		versioning.CreateBranchParams{
			ProjectID: projectID,
			Name:      "main",
		},
	)
	if !errors.Is(err, versioning.ErrBranchNameConflict) {
		t.Fatalf(
			"expected branch name conflict, got %v",
			err,
		)
	}
}

func TestProjectStoreRenamesBranchAndPreservesBranchState(
	t *testing.T,
) {
	ctx, _, store, queries, userID, projectID, mainBranchID :=
		setupVersioningStoreTest(t)

	rootRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     mainBranchID,
			AuthorUserID: userID,
			Message:      "Initial revision",
		},
	)
	if err != nil {
		t.Fatalf("create root revision: %v", err)
	}

	branch, err := store.CreateBranch(
		ctx,
		versioning.CreateBranchParams{
			ProjectID:      projectID,
			Name:           "feature",
			HeadRevisionID: &rootRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create project branch: %v", err)
	}

	renamed, err := store.RenameBranch(
		ctx,
		versioning.RenameBranchParams{
			ProjectID: projectID,
			BranchID:  branch.ID,
			Name:      "feature-renamed",
		},
	)
	if err != nil {
		t.Fatalf("rename project branch: %v", err)
	}

	if renamed.ID != branch.ID {
		t.Fatalf(
			"expected branch ID %s, got %s",
			branch.ID,
			renamed.ID,
		)
	}

	if renamed.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			renamed.ProjectID,
		)
	}

	if renamed.Name != "feature-renamed" {
		t.Fatalf(
			"expected renamed branch name %q, got %q",
			"feature-renamed",
			renamed.Name,
		)
	}

	if !renamed.HeadRevisionID.Valid {
		t.Fatal("expected renamed branch head to remain set")
	}

	actualHead := uuid.UUID(renamed.HeadRevisionID.Bytes)
	if actualHead != rootRevision.ID {
		t.Fatalf(
			"expected branch head %s, got %s",
			rootRevision.ID,
			actualHead,
		)
	}

	if !renamed.CreatedAt.Equal(branch.CreatedAt) {
		t.Fatalf(
			"expected created_at %s to remain unchanged, got %s",
			branch.CreatedAt,
			renamed.CreatedAt,
		)
	}

	if renamed.UpdatedAt.Before(branch.UpdatedAt) {
		t.Fatalf(
			"expected updated_at not before %s, got %s",
			branch.UpdatedAt,
			renamed.UpdatedAt,
		)
	}

	persisted, err := queries.GetProjectBranchByIDAndProject(
		ctx,
		dbgen.GetProjectBranchByIDAndProjectParams{
			BranchID:  branch.ID,
			ProjectID: projectID,
		},
	)
	if err != nil {
		t.Fatalf("get renamed project branch: %v", err)
	}

	if persisted.Name != "feature-renamed" {
		t.Fatalf(
			"expected persisted branch name %q, got %q",
			"feature-renamed",
			persisted.Name,
		)
	}

	if !persisted.HeadRevisionID.Valid ||
		uuid.UUID(persisted.HeadRevisionID.Bytes) != rootRevision.ID {
		t.Fatal("expected persisted branch head to remain unchanged")
	}
}

func TestProjectStoreRenameBranchAllowsSameName(
	t *testing.T,
) {
	ctx, _, store, _, _, projectID, _ :=
		setupVersioningStoreTest(t)

	branch, err := store.CreateBranch(
		ctx,
		versioning.CreateBranchParams{
			ProjectID: projectID,
			Name:      "feature",
		},
	)
	if err != nil {
		t.Fatalf("create project branch: %v", err)
	}

	renamed, err := store.RenameBranch(
		ctx,
		versioning.RenameBranchParams{
			ProjectID: projectID,
			BranchID:  branch.ID,
			Name:      "feature",
		},
	)
	if err != nil {
		t.Fatalf("rename branch to same name: %v", err)
	}

	if renamed.ID != branch.ID {
		t.Fatalf(
			"expected branch ID %s, got %s",
			branch.ID,
			renamed.ID,
		)
	}

	if renamed.Name != "feature" {
		t.Fatalf(
			"expected branch name %q, got %q",
			"feature",
			renamed.Name,
		)
	}
}

func TestProjectStoreRenameBranchMapsMissingOrWrongProjectToNotFound(
	t *testing.T,
) {
	ctx, _, store, _, _, projectID, _ :=
		setupVersioningStoreTest(t)

	branch, err := store.CreateBranch(
		ctx,
		versioning.CreateBranchParams{
			ProjectID: projectID,
			Name:      "feature",
		},
	)
	if err != nil {
		t.Fatalf("create project branch: %v", err)
	}

	tests := []struct {
		name      string
		projectID uuid.UUID
		branchID  uuid.UUID
	}{
		{
			name:      "missing branch",
			projectID: projectID,
			branchID:  uuid.New(),
		},
		{
			name:      "wrong project",
			projectID: uuid.New(),
			branchID:  branch.ID,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := store.RenameBranch(
				ctx,
				versioning.RenameBranchParams{
					ProjectID: test.projectID,
					BranchID:  test.branchID,
					Name:      "renamed",
				},
			)
			if !errors.Is(err, versioning.ErrBranchNotFound) {
				t.Fatalf(
					"expected ErrBranchNotFound, got %v",
					err,
				)
			}
		})
	}
}

func TestProjectStoreRenameBranchMapsDuplicateNameToConflict(
	t *testing.T,
) {
	ctx, _, store, _, _, projectID, _ :=
		setupVersioningStoreTest(t)

	branch, err := store.CreateBranch(
		ctx,
		versioning.CreateBranchParams{
			ProjectID: projectID,
			Name:      "feature",
		},
	)
	if err != nil {
		t.Fatalf("create project branch: %v", err)
	}

	_, err = store.RenameBranch(
		ctx,
		versioning.RenameBranchParams{
			ProjectID: projectID,
			BranchID:  branch.ID,
			Name:      "main",
		},
	)
	if !errors.Is(err, versioning.ErrBranchNameConflict) {
		t.Fatalf(
			"expected ErrBranchNameConflict, got %v",
			err,
		)
	}
}

func TestProjectStoreDeleteBranchRemovesPointerAndPreservesRevision(
	t *testing.T,
) {
	ctx, _, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	revision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     branchID,
			AuthorUserID: userID,
			Message:      "Revision before branch deletion",
		},
	)
	if err != nil {
		t.Fatalf("create project revision: %v", err)
	}

	err = store.DeleteBranch(
		ctx,
		versioning.DeleteBranchParams{
			ProjectID: projectID,
			BranchID:  branchID,
		},
	)
	if err != nil {
		t.Fatalf("delete project branch: %v", err)
	}

	branches, err := queries.ListProjectBranchesByProject(
		ctx,
		projectID,
	)
	if err != nil {
		t.Fatalf("list project branches after deletion: %v", err)
	}

	if len(branches) != 0 {
		t.Fatalf(
			"expected project to have 0 branches after deleting its last branch, got %d",
			len(branches),
		)
	}

	persistedRevision, err :=
		queries.GetProjectRevisionByIDAndProject(
			ctx,
			dbgen.GetProjectRevisionByIDAndProjectParams{
				RevisionID: revision.ID,
				ProjectID:  projectID,
			},
		)
	if err != nil {
		t.Fatalf(
			"get revision after deleting branch: %v",
			err,
		)
	}

	if persistedRevision.ID != revision.ID {
		t.Fatalf(
			"expected revision ID %s, got %s",
			revision.ID,
			persistedRevision.ID,
		)
	}

	if persistedRevision.ProjectID != projectID {
		t.Fatalf(
			"expected revision project ID %s, got %s",
			projectID,
			persistedRevision.ProjectID,
		)
	}
}

func TestProjectStoreDeleteBranchMapsMissingOrWrongProjectToNotFound(
	t *testing.T,
) {
	ctx, _, store, queries, _, projectID, branchID :=
		setupVersioningStoreTest(t)

	tests := []struct {
		name      string
		projectID uuid.UUID
		branchID  uuid.UUID
	}{
		{
			name:      "missing branch",
			projectID: projectID,
			branchID:  uuid.New(),
		},
		{
			name:      "wrong project",
			projectID: uuid.New(),
			branchID:  branchID,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := store.DeleteBranch(
				ctx,
				versioning.DeleteBranchParams{
					ProjectID: test.projectID,
					BranchID:  test.branchID,
				},
			)
			if !errors.Is(err, versioning.ErrBranchNotFound) {
				t.Fatalf(
					"expected ErrBranchNotFound, got %v",
					err,
				)
			}
		})
	}

	branch, err := queries.GetProjectBranchByIDAndProject(
		ctx,
		dbgen.GetProjectBranchByIDAndProjectParams{
			BranchID:  branchID,
			ProjectID: projectID,
		},
	)
	if err != nil {
		t.Fatalf(
			"get branch after rejected deletions: %v",
			err,
		)
	}

	if branch.ID != branchID {
		t.Fatalf(
			"expected branch ID %s, got %s",
			branchID,
			branch.ID,
		)
	}
}

func TestListProjectBranchesByProjectScopesAndOrdersBranches(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, mainBranchID :=
		setupVersioningStoreTest(t)

	rootRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     mainBranchID,
			AuthorUserID: userID,
			Message:      "Initial revision",
		},
	)
	if err != nil {
		t.Fatalf("create root revision: %v", err)
	}

	_, err = pool.Exec(
		ctx,
		`INSERT INTO project_branches (
			project_id,
			name,
			head_revision_id
		)
		VALUES
			($1, 'feature', $2),
			($1, 'alpha', NULL)`,
		projectID,
		rootRevision.ID,
	)
	if err != nil {
		t.Fatalf("create additional project branches: %v", err)
	}

	otherProject, err := store.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: userID,
			Name:        "Other Versioning Project",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create other project: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM projects WHERE id = $1",
			otherProject.ID,
		)
		if err != nil {
			t.Errorf("delete other versioning project: %v", err)
		}
	})

	branches, err := queries.ListProjectBranchesByProject(
		ctx,
		projectID,
	)
	if err != nil {
		t.Fatalf("list project branches: %v", err)
	}

	if len(branches) != 3 {
		t.Fatalf(
			"expected 3 project branches, got %d",
			len(branches),
		)
	}

	expectedNames := []string{
		"alpha",
		"feature",
		"main",
	}

	for index, expectedName := range expectedNames {
		branch := branches[index]

		if branch.ProjectID != projectID {
			t.Fatalf(
				"expected branch project ID %s, got %s",
				projectID,
				branch.ProjectID,
			)
		}

		if branch.Name != expectedName {
			t.Fatalf(
				"expected branch %d name %q, got %q",
				index,
				expectedName,
				branch.Name,
			)
		}
	}

	if branches[0].HeadRevisionID.Valid {
		t.Fatalf(
			"expected alpha branch head to be null, got %s",
			uuid.UUID(branches[0].HeadRevisionID.Bytes),
		)
	}

	for _, index := range []int{1, 2} {
		if !branches[index].HeadRevisionID.Valid {
			t.Fatalf(
				"expected %s branch head to be set",
				branches[index].Name,
			)
		}

		actualHead :=
			uuid.UUID(branches[index].HeadRevisionID.Bytes)

		if actualHead != rootRevision.ID {
			t.Fatalf(
				"expected %s branch head %s, got %s",
				branches[index].Name,
				rootRevision.ID,
				actualHead,
			)
		}
	}

	otherBranches, err := queries.ListProjectBranchesByProject(
		ctx,
		otherProject.ID,
	)
	if err != nil {
		t.Fatalf("list other project branches: %v", err)
	}

	if len(otherBranches) != 1 {
		t.Fatalf(
			"expected 1 other-project branch, got %d",
			len(otherBranches),
		)
	}

	if otherBranches[0].ProjectID != otherProject.ID {
		t.Fatalf(
			"expected other branch project ID %s, got %s",
			otherProject.ID,
			otherBranches[0].ProjectID,
		)
	}

	if otherBranches[0].Name != "main" {
		t.Fatalf(
			"expected other project branch name %q, got %q",
			"main",
			otherBranches[0].Name,
		)
	}
}

func TestGetProjectRevisionByIDAndProjectScopesRevision(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	revision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     branchID,
			AuthorUserID: userID,
			Message:      "Initial revision",
		},
	)
	if err != nil {
		t.Fatalf("create project revision: %v", err)
	}

	otherProject, err := store.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: userID,
			Name:        "Other Revision Project",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create other project: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM projects WHERE id = $1",
			otherProject.ID,
		)
		if err != nil {
			t.Errorf("delete other revision project: %v", err)
		}
	})

	foundRevision, err :=
		queries.GetProjectRevisionByIDAndProject(
			ctx,
			dbgen.GetProjectRevisionByIDAndProjectParams{
				RevisionID: revision.ID,
				ProjectID:  projectID,
			},
		)
	if err != nil {
		t.Fatalf("get project revision: %v", err)
	}

	if foundRevision.ID != revision.ID {
		t.Fatalf(
			"expected revision ID %s, got %s",
			revision.ID,
			foundRevision.ID,
		)
	}

	if foundRevision.ProjectID != projectID {
		t.Fatalf(
			"expected project ID %s, got %s",
			projectID,
			foundRevision.ProjectID,
		)
	}

	if foundRevision.AuthorUserID != userID {
		t.Fatalf(
			"expected author ID %s, got %s",
			userID,
			foundRevision.AuthorUserID,
		)
	}

	if foundRevision.Message != "Initial revision" {
		t.Fatalf(
			"expected revision message %q, got %q",
			"Initial revision",
			foundRevision.Message,
		)
	}

	_, err = queries.GetProjectRevisionByIDAndProject(
		ctx,
		dbgen.GetProjectRevisionByIDAndProjectParams{
			RevisionID: revision.ID,
			ProjectID:  otherProject.ID,
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected cross-project revision lookup to return pgx.ErrNoRows, got %v",
			err,
		)
	}

	_, err = queries.GetProjectRevisionByIDAndProject(
		ctx,
		dbgen.GetProjectRevisionByIDAndProjectParams{
			RevisionID: uuid.New(),
			ProjectID:  projectID,
		},
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected missing revision lookup to return pgx.ErrNoRows, got %v",
			err,
		)
	}
}

func TestListReachableProjectRevisionsFromRevisionTraversesMergeDAG(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, mainBranchID :=
		setupVersioningStoreTest(t)

	rootRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     mainBranchID,
			AuthorUserID: userID,
			Message:      "Root revision",
		},
	)
	if err != nil {
		t.Fatalf("create root revision: %v", err)
	}

	sideBranch, err := store.CreateBranch(
		ctx,
		versioning.CreateBranchParams{
			ProjectID:      projectID,
			Name:           "feature",
			HeadRevisionID: &rootRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create side branch: %v", err)
	}

	disconnectedBranch, err := store.CreateBranch(
		ctx,
		versioning.CreateBranchParams{
			ProjectID: projectID,
			Name:      "disconnected",
		},
	)
	if err != nil {
		t.Fatalf("create disconnected branch: %v", err)
	}

	mainRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               mainBranchID,
			AuthorUserID:           userID,
			Message:                "Main revision",
			ExpectedHeadRevisionID: &rootRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create main revision: %v", err)
	}

	sideRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               sideBranch.ID,
			AuthorUserID:           userID,
			Message:                "Side revision",
			ExpectedHeadRevisionID: &rootRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create side revision: %v", err)
	}

	mainHeadRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               mainBranchID,
			AuthorUserID:           userID,
			Message:                "Main head revision",
			ExpectedHeadRevisionID: &mainRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create main head revision: %v", err)
	}

	disconnectedRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     disconnectedBranch.ID,
			AuthorUserID: userID,
			Message:      "Disconnected revision",
		},
	)
	if err != nil {
		t.Fatalf("create disconnected revision: %v", err)
	}

	mergeRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               mainBranchID,
			AuthorUserID:           userID,
			Message:                "Merge revision",
			ExpectedHeadRevisionID: &mainHeadRevision.ID,
			MergeParentRevisionID:  &sideRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create merge revision: %v", err)
	}

	otherProject, err := store.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: userID,
			Name:        "Other Traversal Project",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create other project: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM projects WHERE id = $1",
			otherProject.ID,
		)
		if err != nil {
			t.Errorf("delete other traversal project: %v", err)
		}
	})

	var otherMainBranchID uuid.UUID
	err = pool.QueryRow(
		ctx,
		`SELECT id
		 FROM project_branches
		 WHERE project_id = $1
		   AND name = 'main'`,
		otherProject.ID,
	).Scan(&otherMainBranchID)
	if err != nil {
		t.Fatalf("get other project main branch: %v", err)
	}

	otherRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    otherProject.ID,
			BranchID:     otherMainBranchID,
			AuthorUserID: userID,
			Message:      "Other project revision",
		},
	)
	if err != nil {
		t.Fatalf("create other project revision: %v", err)
	}

	revisions, err :=
		queries.ListReachableProjectRevisionsFromRevision(
			ctx,
			dbgen.ListReachableProjectRevisionsFromRevisionParams{
				ProjectID:       projectID,
				StartRevisionID: mergeRevision.ID,
			},
		)
	if err != nil {
		t.Fatalf("list reachable project revisions: %v", err)
	}

	expectedIDs := map[uuid.UUID]bool{
		rootRevision.ID:     true,
		mainRevision.ID:     true,
		sideRevision.ID:     true,
		mainHeadRevision.ID: true,
		mergeRevision.ID:    true,
	}

	if len(revisions) != len(expectedIDs) {
		t.Fatalf(
			"expected %d reachable revisions, got %d",
			len(expectedIDs),
			len(revisions),
		)
	}

	seen := make(map[uuid.UUID]bool, len(revisions))

	for index, revision := range revisions {
		if revision.ProjectID != projectID {
			t.Fatalf(
				"expected revision %s to belong to project %s, got %s",
				revision.ID,
				projectID,
				revision.ProjectID,
			)
		}

		if !expectedIDs[revision.ID] {
			t.Fatalf(
				"unexpected reachable revision %s",
				revision.ID,
			)
		}

		if seen[revision.ID] {
			t.Fatalf(
				"reachable revision %s returned more than once",
				revision.ID,
			)
		}
		seen[revision.ID] = true

		if index == 0 {
			continue
		}

		previous := revisions[index-1]

		if previous.CreatedAt.Before(revision.CreatedAt) {
			t.Fatalf(
				"revisions are not ordered by created_at descending: "+
					"%s precedes %s",
				previous.ID,
				revision.ID,
			)
		}

		if previous.CreatedAt.Equal(revision.CreatedAt) &&
			previous.ID.String() > revision.ID.String() {
			t.Fatalf(
				"equal-time revisions are not ordered by ID ascending: "+
					"%s precedes %s",
				previous.ID,
				revision.ID,
			)
		}
	}

	for expectedID := range expectedIDs {
		if !seen[expectedID] {
			t.Fatalf(
				"expected reachable revision %s was not returned",
				expectedID,
			)
		}
	}

	if seen[disconnectedRevision.ID] {
		t.Fatalf(
			"disconnected revision %s was returned",
			disconnectedRevision.ID,
		)
	}

	if seen[otherRevision.ID] {
		t.Fatalf(
			"other-project revision %s was returned",
			otherRevision.ID,
		)
	}

	crossProjectStart, err :=
		queries.ListReachableProjectRevisionsFromRevision(
			ctx,
			dbgen.ListReachableProjectRevisionsFromRevisionParams{
				ProjectID:       projectID,
				StartRevisionID: otherRevision.ID,
			},
		)
	if err != nil {
		t.Fatalf(
			"list reachable revisions from cross-project start: %v",
			err,
		)
	}

	if len(crossProjectStart) != 0 {
		t.Fatalf(
			"expected cross-project start to return 0 revisions, got %d",
			len(crossProjectStart),
		)
	}
}

func TestProjectStoreCreatesRevisionWithExactProjectFileSnapshot(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	firstFile := createVersioningProjectFile(
		t,
		ctx,
		pool,
		queries,
		userID,
		projectID,
		"first-source",
	)
	secondFile := createVersioningProjectFile(
		t,
		ctx,
		pool,
		queries,
		userID,
		projectID,
		"second-source",
	)

	revision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     branchID,
			AuthorUserID: userID,
			Message:      "Revision with exact file snapshot",
			ProjectFileIDs: []uuid.UUID{
				secondFile.ID,
				firstFile.ID,
			},
		},
	)
	if err != nil {
		t.Fatalf("create revision with project files: %v", err)
	}

	files, err := queries.ListProjectFilesByRevision(
		ctx,
		dbgen.ListProjectFilesByRevisionParams{
			ProjectID:  projectID,
			RevisionID: revision.ID,
		},
	)
	if err != nil {
		t.Fatalf("list project files by revision: %v", err)
	}

	if len(files) != 2 {
		t.Fatalf(
			"expected 2 revision files, got %d",
			len(files),
		)
	}

	expectedIDs := map[uuid.UUID]bool{
		firstFile.ID:  true,
		secondFile.ID: true,
	}
	seenIDs := make(map[uuid.UUID]bool, len(files))

	for index, projectFile := range files {
		if projectFile.ProjectID != projectID {
			t.Fatalf(
				"expected project file %s to belong to project %s, got %s",
				projectFile.ID,
				projectID,
				projectFile.ProjectID,
			)
		}

		if !expectedIDs[projectFile.ID] {
			t.Fatalf(
				"unexpected project file %s in revision snapshot",
				projectFile.ID,
			)
		}

		if seenIDs[projectFile.ID] {
			t.Fatalf(
				"project file %s returned more than once",
				projectFile.ID,
			)
		}

		seenIDs[projectFile.ID] = true

		if index == 0 {
			continue
		}

		previous := files[index-1]

		if previous.CreatedAt.After(projectFile.CreatedAt) {
			t.Fatalf(
				"revision files are not ordered by created_at ascending: "+
					"%s precedes %s",
				previous.ID,
				projectFile.ID,
			)
		}

		if previous.CreatedAt.Equal(projectFile.CreatedAt) &&
			previous.ID.String() > projectFile.ID.String() {
			t.Fatalf(
				"equal-time revision files are not ordered by ID ascending: "+
					"%s precedes %s",
				previous.ID,
				projectFile.ID,
			)
		}
	}

	scopedOut, err := queries.ListProjectFilesByRevision(
		ctx,
		dbgen.ListProjectFilesByRevisionParams{
			ProjectID:  uuid.New(),
			RevisionID: revision.ID,
		},
	)
	if err != nil {
		t.Fatalf(
			"list revision files through wrong project scope: %v",
			err,
		)
	}

	if len(scopedOut) != 0 {
		t.Fatalf(
			"expected wrong-project revision-file lookup to return 0 files, got %d",
			len(scopedOut),
		)
	}

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		branchID,
		revision.ID,
	)
}

func TestProjectStoreCreatesRevisionWithEmptyProjectFileSnapshot(
	t *testing.T,
) {
	ctx, _, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	revision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:      projectID,
			BranchID:       branchID,
			AuthorUserID:   userID,
			Message:        "Revision with empty file snapshot",
			ProjectFileIDs: []uuid.UUID{},
		},
	)
	if err != nil {
		t.Fatalf("create revision with empty file snapshot: %v", err)
	}

	files, err := queries.ListProjectFilesByRevision(
		ctx,
		dbgen.ListProjectFilesByRevisionParams{
			ProjectID:  projectID,
			RevisionID: revision.ID,
		},
	)
	if err != nil {
		t.Fatalf("list empty revision file snapshot: %v", err)
	}

	if len(files) != 0 {
		t.Fatalf(
			"expected empty revision file snapshot, got %d files",
			len(files),
		)
	}

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		branchID,
		revision.ID,
	)
}

func TestProjectStoreRollsBackRevisionForMissingOrWrongProjectFile(
	t *testing.T,
) {
	t.Run("missing project file", func(t *testing.T) {
		ctx, pool, store, queries, userID, projectID, branchID :=
			setupVersioningStoreTest(t)

		_, err := store.CreateRevisionOnBranch(
			ctx,
			versioning.CreateRevisionOnBranchParams{
				ProjectID:    projectID,
				BranchID:     branchID,
				AuthorUserID: userID,
				Message:      "Revision with missing project file",
				ProjectFileIDs: []uuid.UUID{
					uuid.New(),
				},
			},
		)
		if !errors.Is(err, versioning.ErrProjectFileNotFound) {
			t.Fatalf(
				"expected ErrProjectFileNotFound, got %v",
				err,
			)
		}

		assertEmptyRevisionCreationState(
			t,
			ctx,
			pool,
			queries,
			projectID,
			branchID,
		)
	})

	t.Run("wrong project file", func(t *testing.T) {
		ctx, pool, store, queries, userID, projectID, branchID :=
			setupVersioningStoreTest(t)

		otherProject, err := store.CreateProject(
			ctx,
			dbgen.CreateProjectParams{
				OwnerUserID: userID,
				Name:        "Other Revision File Project",
				Description: nil,
			},
		)
		if err != nil {
			t.Fatalf("create other project: %v", err)
		}

		t.Cleanup(func() {
			_, err := pool.Exec(
				context.Background(),
				"DELETE FROM projects WHERE id = $1",
				otherProject.ID,
			)
			if err != nil {
				t.Errorf(
					"delete other revision-file project: %v",
					err,
				)
			}
		})

		otherProjectFile := createVersioningProjectFile(
			t,
			ctx,
			pool,
			queries,
			userID,
			otherProject.ID,
			"wrong-project-source",
		)

		_, err = store.CreateRevisionOnBranch(
			ctx,
			versioning.CreateRevisionOnBranchParams{
				ProjectID:    projectID,
				BranchID:     branchID,
				AuthorUserID: userID,
				Message:      "Revision with cross-project file",
				ProjectFileIDs: []uuid.UUID{
					otherProjectFile.ID,
				},
			},
		)
		if !errors.Is(err, versioning.ErrProjectFileNotFound) {
			t.Fatalf(
				"expected ErrProjectFileNotFound, got %v",
				err,
			)
		}

		assertEmptyRevisionCreationState(
			t,
			ctx,
			pool,
			queries,
			projectID,
			branchID,
		)
	})
}

func TestProjectStoreRollsBackRevisionForDuplicateProjectFileID(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	projectFile := createVersioningProjectFile(
		t,
		ctx,
		pool,
		queries,
		userID,
		projectID,
		"duplicate-source",
	)

	_, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     branchID,
			AuthorUserID: userID,
			Message:      "Revision with duplicate project file",
			ProjectFileIDs: []uuid.UUID{
				projectFile.ID,
				projectFile.ID,
			},
		},
	)
	if !errors.Is(err, versioning.ErrDuplicateProjectFileID) {
		t.Fatalf(
			"expected ErrDuplicateProjectFileID, got %v",
			err,
		)
	}

	assertEmptyRevisionCreationState(
		t,
		ctx,
		pool,
		queries,
		projectID,
		branchID,
	)
}

func TestProjectStoreRollsBackRevisionFilesOnBranchHeadConflict(
	t *testing.T,
) {
	ctx, pool, store, queries, userID, projectID, branchID :=
		setupVersioningStoreTest(t)

	rootRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:    projectID,
			BranchID:     branchID,
			AuthorUserID: userID,
			Message:      "Root revision",
		},
	)
	if err != nil {
		t.Fatalf("create root revision: %v", err)
	}

	currentRevision, err := store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               branchID,
			AuthorUserID:           userID,
			Message:                "Current revision",
			ExpectedHeadRevisionID: &rootRevision.ID,
		},
	)
	if err != nil {
		t.Fatalf("create current revision: %v", err)
	}

	projectFile := createVersioningProjectFile(
		t,
		ctx,
		pool,
		queries,
		userID,
		projectID,
		"stale-candidate-source",
	)

	_, err = store.CreateRevisionOnBranch(
		ctx,
		versioning.CreateRevisionOnBranchParams{
			ProjectID:              projectID,
			BranchID:               branchID,
			AuthorUserID:           userID,
			Message:                "Stale candidate revision",
			ExpectedHeadRevisionID: &rootRevision.ID,
			ProjectFileIDs: []uuid.UUID{
				projectFile.ID,
			},
		},
	)
	if !errors.Is(err, versioning.ErrBranchHeadConflict) {
		t.Fatalf(
			"expected ErrBranchHeadConflict, got %v",
			err,
		)
	}

	var revisionCount int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revisions
		 WHERE project_id = $1`,
		projectID,
	).Scan(&revisionCount); err != nil {
		t.Fatalf(
			"count revisions after branch-head conflict: %v",
			err,
		)
	}

	if revisionCount != 2 {
		t.Fatalf(
			"expected 2 committed revisions after conflict, got %d",
			revisionCount,
		)
	}

	var referenceCount int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revision_files
		 WHERE project_id = $1
		   AND project_file_id = $2`,
		projectID,
		projectFile.ID,
	).Scan(&referenceCount); err != nil {
		t.Fatalf(
			"count revision-file references after branch-head conflict: %v",
			err,
		)
	}

	if referenceCount != 0 {
		t.Fatalf(
			"expected branch-head conflict to roll back revision-file references, got %d",
			referenceCount,
		)
	}

	assertBranchHead(
		t,
		ctx,
		queries,
		projectID,
		branchID,
		currentRevision.ID,
	)
}

func createVersioningProjectFile(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	queries *dbgen.Queries,
	userID uuid.UUID,
	projectID uuid.UUID,
	label string,
) dbgen.ProjectFile {
	t.Helper()

	contentSHA256 := newVersioningProjectFileSHA256(label)

	_, err := queries.EnsureContentObject(
		ctx,
		dbgen.EnsureContentObjectParams{
			Sha256:    contentSHA256,
			SizeBytes: int64(len(label)),
		},
	)
	if err != nil {
		t.Fatalf(
			"ensure content object for %q: %v",
			label,
			err,
		)
	}

	projectFile, err := queries.CreateProjectFile(
		ctx,
		dbgen.CreateProjectFileParams{
			ProjectID:        projectID,
			UploadedByUserID: userID,
			ContentSha256:    contentSHA256,
			OriginalFilename: label + ".step",
			MediaType:        nil,
		},
	)
	if err != nil {
		t.Fatalf(
			"create project file for %q: %v",
			label,
			err,
		)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			context.Background(),
			`DELETE FROM project_revision_files
			 WHERE project_id = $1
			   AND project_file_id = $2`,
			projectID,
			projectFile.ID,
		)
		if err != nil {
			t.Errorf(
				"delete revision-file references for %s: %v",
				projectFile.ID,
				err,
			)
		}

		_, err = pool.Exec(
			context.Background(),
			`DELETE FROM project_files
			 WHERE project_id = $1
			   AND id = $2`,
			projectID,
			projectFile.ID,
		)
		if err != nil {
			t.Errorf(
				"delete versioning project file %s: %v",
				projectFile.ID,
				err,
			)
		}

		_, err = pool.Exec(
			context.Background(),
			"DELETE FROM content_objects WHERE sha256 = $1",
			contentSHA256,
		)
		if err != nil {
			t.Errorf(
				"delete versioning content object %q: %v",
				contentSHA256,
				err,
			)
		}
	})

	return projectFile
}

func newVersioningProjectFileSHA256(
	label string,
) string {
	digest := sha256.Sum256(
		[]byte(label + ":" + uuid.New().String()),
	)

	return hex.EncodeToString(digest[:])
}

func assertEmptyRevisionCreationState(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	queries *dbgen.Queries,
	projectID uuid.UUID,
	branchID uuid.UUID,
) {
	t.Helper()

	var revisionCount int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revisions
		 WHERE project_id = $1`,
		projectID,
	).Scan(&revisionCount); err != nil {
		t.Fatalf(
			"count project revisions after rollback: %v",
			err,
		)
	}

	if revisionCount != 0 {
		t.Fatalf(
			"expected rollback to leave 0 revisions, got %d",
			revisionCount,
		)
	}

	var referenceCount int
	if err := pool.QueryRow(
		ctx,
		`SELECT count(*)
		 FROM project_revision_files
		 WHERE project_id = $1`,
		projectID,
	).Scan(&referenceCount); err != nil {
		t.Fatalf(
			"count revision-file references after rollback: %v",
			err,
		)
	}

	if referenceCount != 0 {
		t.Fatalf(
			"expected rollback to leave 0 revision-file references, got %d",
			referenceCount,
		)
	}

	branch, err := queries.GetProjectBranchByIDAndProject(
		ctx,
		dbgen.GetProjectBranchByIDAndProjectParams{
			BranchID:  branchID,
			ProjectID: projectID,
		},
	)
	if err != nil {
		t.Fatalf(
			"get project branch after revision rollback: %v",
			err,
		)
	}

	if branch.HeadRevisionID.Valid {
		t.Fatalf(
			"expected rollback to preserve empty branch head, got %s",
			uuid.UUID(branch.HeadRevisionID.Bytes),
		)
	}
}

func setupVersioningStoreTest(
	t *testing.T,
) (
	context.Context,
	*pgxpool.Pool,
	*database.ProjectStore,
	*dbgen.Queries,
	uuid.UUID,
	uuid.UUID,
	uuid.UUID,
) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for database integration tests")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	t.Cleanup(cancel)

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}

	queries := dbgen.New(pool)

	testID := uuid.New().String()

	user, err := queries.CreateUser(
		ctx,
		dbgen.CreateUserParams{
			Email:       testID + "@versioning.example.com",
			DisplayName: "Versioning Store User",
		},
	)
	if err != nil {
		t.Fatalf("create versioning store user: %v", err)
	}

	store := database.NewProjectStore(pool)

	project, err := store.CreateProject(
		ctx,
		dbgen.CreateProjectParams{
			OwnerUserID: user.ID,
			Name:        "Versioning Store Project",
			Description: nil,
		},
	)
	if err != nil {
		t.Fatalf("create versioning store project: %v", err)
	}

	t.Cleanup(func() {
		_, err := pool.Exec(
			context.Background(),
			"DELETE FROM projects WHERE id = $1",
			project.ID,
		)
		if err != nil {
			t.Errorf("delete versioning store project: %v", err)
		}

		_, err = pool.Exec(
			context.Background(),
			"DELETE FROM users WHERE id = $1",
			user.ID,
		)
		if err != nil {
			t.Errorf("delete versioning store user: %v", err)
		}
	})

	var branchID uuid.UUID
	if err := pool.QueryRow(
		ctx,
		`SELECT id
		 FROM project_branches
		 WHERE project_id = $1
		   AND name = 'main'`,
		project.ID,
	).Scan(&branchID); err != nil {
		t.Fatalf("get default project branch: %v", err)
	}

	return ctx, pool, store, queries, user.ID, project.ID, branchID
}

func assertBranchHead(
	t *testing.T,
	ctx context.Context,
	queries *dbgen.Queries,
	projectID uuid.UUID,
	branchID uuid.UUID,
	expectedRevisionID uuid.UUID,
) {
	t.Helper()

	branch, err := queries.GetProjectBranchByIDAndProject(
		ctx,
		dbgen.GetProjectBranchByIDAndProjectParams{
			BranchID:  branchID,
			ProjectID: projectID,
		},
	)
	if err != nil {
		t.Fatalf("get project branch: %v", err)
	}

	if !branch.HeadRevisionID.Valid {
		t.Fatal("expected project branch head to be set")
	}

	actualRevisionID := uuid.UUID(branch.HeadRevisionID.Bytes)
	if actualRevisionID != expectedRevisionID {
		t.Fatalf(
			"expected branch head %s, got %s",
			expectedRevisionID,
			actualRevisionID,
		)
	}
}
