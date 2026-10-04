//go:build integration

package database_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/conversionjobs"
	"github.com/AngelAvilesSil/3Default/internal/database"
	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestConversionJobStoreFinalizesOutputAndSuccessAtomically(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"output-success",
	)

	job := createConversionJob(
		t,
		fixture,
		projectFile.ID,
	)

	claimed, err := store.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if err != nil {
		t.Fatalf("claim conversion job: %v", err)
	}

	if claimed.ID != job.ID {
		t.Fatalf(
			"expected claimed job %s, got %s",
			job.ID,
			claimed.ID,
		)
	}

	contentSHA256 := newConversionOutputSHA256(
		"successful-output",
	)
	defer cleanupConversionOutputContent(
		t,
		fixture,
		job.ID,
		contentSHA256,
	)

	output, err := store.FinalizeConversionJobSuccess(
		fixture.ctx,
		conversionjobs.FinalizeSuccessInput{
			ConversionJobID: job.ID,
			ContentSHA256:   contentSHA256,
			SizeBytes:       4096,
			MediaType:       "model/gltf-binary",
		},
	)
	if err != nil {
		t.Fatalf(
			"finalize conversion success: %v",
			err,
		)
	}

	if output.ConversionJobID != job.ID {
		t.Fatalf(
			"expected output job ID %s, got %s",
			job.ID,
			output.ConversionJobID,
		)
	}

	if output.ContentSha256 != contentSHA256 {
		t.Fatalf(
			"expected output SHA-256 %q, got %q",
			contentSHA256,
			output.ContentSha256,
		)
	}

	if output.MediaType != "model/gltf-binary" {
		t.Fatalf(
			"expected GLB media type, got %q",
			output.MediaType,
		)
	}

	contentObject, err :=
		fixture.queries.GetContentObjectBySHA256(
			fixture.ctx,
			contentSHA256,
		)
	if err != nil {
		t.Fatalf(
			"get conversion output content object: %v",
			err,
		)
	}

	if contentObject.SizeBytes != 4096 {
		t.Fatalf(
			"expected output size 4096, got %d",
			contentObject.SizeBytes,
		)
	}

	storedOutput, err :=
		fixture.queries.GetConversionJobOutputByJobID(
			fixture.ctx,
			job.ID,
		)
	if err != nil {
		t.Fatalf(
			"get conversion job output: %v",
			err,
		)
	}

	if storedOutput.ContentSha256 != contentSHA256 {
		t.Fatalf(
			"expected stored output SHA-256 %q, got %q",
			contentSHA256,
			storedOutput.ContentSha256,
		)
	}

	latest, err :=
		fixture.queries.GetLatestConversionJobOutputByProjectFile(
			fixture.ctx,
			dbgen.GetLatestConversionJobOutputByProjectFileParams{
				ProjectID:     fixture.projectID,
				ProjectFileID: projectFile.ID,
			},
		)
	if err != nil {
		t.Fatalf(
			"get latest conversion output: %v",
			err,
		)
	}

	if latest.ConversionJobID != job.ID {
		t.Fatalf(
			"expected latest output job ID %s, got %s",
			job.ID,
			latest.ConversionJobID,
		)
	}

	finalJob, err :=
		store.GetConversionJobByIDAndProject(
			fixture.ctx,
			dbgen.GetConversionJobByIDAndProjectParams{
				ConversionJobID: job.ID,
				ProjectID:       fixture.projectID,
			},
		)
	if err != nil {
		t.Fatalf(
			"get finalized conversion job: %v",
			err,
		)
	}

	if finalJob.Status != "succeeded" {
		t.Fatalf(
			"expected succeeded status, got %q",
			finalJob.Status,
		)
	}

	if !finalJob.FinishedAt.Valid {
		t.Fatal(
			"expected succeeded job finished_at to be set",
		)
	}
}

func TestConversionJobStoreReusesMatchingOutputContentObject(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"output-reuse",
	)

	job := createConversionJob(
		t,
		fixture,
		projectFile.ID,
	)

	_, err := store.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if err != nil {
		t.Fatalf("claim conversion job: %v", err)
	}

	contentSHA256 := newConversionOutputSHA256(
		"reused-output-content",
	)
	defer cleanupConversionOutputContent(
		t,
		fixture,
		job.ID,
		contentSHA256,
	)

	_, err = fixture.queries.EnsureContentObject(
		fixture.ctx,
		dbgen.EnsureContentObjectParams{
			Sha256:    contentSHA256,
			SizeBytes: 2048,
		},
	)
	if err != nil {
		t.Fatalf(
			"create existing output content object: %v",
			err,
		)
	}

	_, err = store.FinalizeConversionJobSuccess(
		fixture.ctx,
		conversionjobs.FinalizeSuccessInput{
			ConversionJobID: job.ID,
			ContentSHA256:   contentSHA256,
			SizeBytes:       2048,
			MediaType:       "model/gltf-binary",
		},
	)
	if err != nil {
		t.Fatalf(
			"finalize with existing content object: %v",
			err,
		)
	}

	var count int
	err = fixture.pool.QueryRow(
		fixture.ctx,
		`SELECT count(*)
		 FROM content_objects
		 WHERE sha256 = $1`,
		contentSHA256,
	).Scan(&count)
	if err != nil {
		t.Fatalf(
			"count reused content object: %v",
			err,
		)
	}

	if count != 1 {
		t.Fatalf(
			"expected one reused content object, got %d",
			count,
		)
	}
}

func TestConversionJobStoreMapsOutputContentSizeConflict(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"output-size-conflict",
	)

	job := createConversionJob(
		t,
		fixture,
		projectFile.ID,
	)

	_, err := store.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if err != nil {
		t.Fatalf("claim conversion job: %v", err)
	}

	contentSHA256 := newConversionOutputSHA256(
		"output-size-conflict",
	)
	defer cleanupConversionOutputContent(
		t,
		fixture,
		job.ID,
		contentSHA256,
	)

	_, err = fixture.queries.EnsureContentObject(
		fixture.ctx,
		dbgen.EnsureContentObjectParams{
			Sha256:    contentSHA256,
			SizeBytes: 1024,
		},
	)
	if err != nil {
		t.Fatalf(
			"create existing content object: %v",
			err,
		)
	}

	_, err = store.FinalizeConversionJobSuccess(
		fixture.ctx,
		conversionjobs.FinalizeSuccessInput{
			ConversionJobID: job.ID,
			ContentSHA256:   contentSHA256,
			SizeBytes:       1025,
			MediaType:       "model/gltf-binary",
		},
	)
	if !errors.Is(
		err,
		conversionjobs.ErrOutputContentSizeConflict,
	) {
		t.Fatalf(
			"expected ErrOutputContentSizeConflict, got %v",
			err,
		)
	}

	_, err = fixture.queries.GetConversionJobOutputByJobID(
		fixture.ctx,
		job.ID,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected no output after size conflict, got %v",
			err,
		)
	}

	finalJob, err :=
		store.GetConversionJobByIDAndProject(
			fixture.ctx,
			dbgen.GetConversionJobByIDAndProjectParams{
				ConversionJobID: job.ID,
				ProjectID:       fixture.projectID,
			},
		)
	if err != nil {
		t.Fatalf(
			"get job after size conflict: %v",
			err,
		)
	}

	if finalJob.Status != "running" {
		t.Fatalf(
			"expected job to remain running, got %q",
			finalJob.Status,
		)
	}
}

func TestConversionJobStoreRollsBackContentForNonRunningJob(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"output-pending-job",
	)

	job := createConversionJob(
		t,
		fixture,
		projectFile.ID,
	)

	contentSHA256 := newConversionOutputSHA256(
		"rolled-back-pending-output",
	)
	defer cleanupConversionOutputContent(
		t,
		fixture,
		job.ID,
		contentSHA256,
	)

	_, err := store.FinalizeConversionJobSuccess(
		fixture.ctx,
		conversionjobs.FinalizeSuccessInput{
			ConversionJobID: job.ID,
			ContentSHA256:   contentSHA256,
			SizeBytes:       512,
			MediaType:       "model/gltf-binary",
		},
	)
	if !errors.Is(
		err,
		conversionjobs.ErrConversionJobNotRunning,
	) {
		t.Fatalf(
			"expected ErrConversionJobNotRunning, got %v",
			err,
		)
	}

	_, err = fixture.queries.GetContentObjectBySHA256(
		fixture.ctx,
		contentSHA256,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected content object rollback, got %v",
			err,
		)
	}

	_, err = fixture.queries.GetConversionJobOutputByJobID(
		fixture.ctx,
		job.ID,
	)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf(
			"expected no output row, got %v",
			err,
		)
	}

	finalJob, err :=
		store.GetConversionJobByIDAndProject(
			fixture.ctx,
			dbgen.GetConversionJobByIDAndProjectParams{
				ConversionJobID: job.ID,
				ProjectID:       fixture.projectID,
			},
		)
	if err != nil {
		t.Fatalf(
			"get pending conversion job: %v",
			err,
		)
	}

	if finalJob.Status != "pending" {
		t.Fatalf(
			"expected job to remain pending, got %q",
			finalJob.Status,
		)
	}
}

func TestConversionJobStoreRollsBackOutputWhenSuccessTransitionFails(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"output-success-rollback",
	)

	job := createConversionJob(
		t,
		fixture,
		projectFile.ID,
	)

	claimed, err := store.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if err != nil {
		t.Fatalf("claim conversion job: %v", err)
	}

	future := time.Now().UTC().Add(24 * time.Hour)

	_, err = fixture.pool.Exec(
		fixture.ctx,
		`UPDATE conversion_jobs
		 SET
		    created_at = $1,
		    started_at = $1,
		    updated_at = $1
		 WHERE id = $2`,
		future,
		claimed.ID,
	)
	if err != nil {
		t.Fatalf(
			"prepare success-transition failure: %v",
			err,
		)
	}

	contentSHA256 := newConversionOutputSHA256(
		"rolled-back-success-output",
	)
	defer cleanupConversionOutputContent(
		t,
		fixture,
		job.ID,
		contentSHA256,
	)

	_, err = store.FinalizeConversionJobSuccess(
		fixture.ctx,
		conversionjobs.FinalizeSuccessInput{
			ConversionJobID: job.ID,
			ContentSHA256:   contentSHA256,
			SizeBytes:       8192,
			MediaType:       "model/gltf-binary",
		},
	)
	if err == nil {
		t.Fatal(
			"expected success transition to fail",
		)
	}

	if !strings.Contains(
		err.Error(),
		"mark conversion job succeeded",
	) {
		t.Fatalf(
			"expected success-transition error, got %v",
			err,
		)
	}

	_, outputErr :=
		fixture.queries.GetConversionJobOutputByJobID(
			fixture.ctx,
			job.ID,
		)
	if !errors.Is(outputErr, pgx.ErrNoRows) {
		t.Fatalf(
			"expected output insertion rollback, got %v",
			outputErr,
		)
	}

	_, contentErr :=
		fixture.queries.GetContentObjectBySHA256(
			fixture.ctx,
			contentSHA256,
		)
	if !errors.Is(contentErr, pgx.ErrNoRows) {
		t.Fatalf(
			"expected content-object rollback, got %v",
			contentErr,
		)
	}

	finalJob, getErr :=
		store.GetConversionJobByIDAndProject(
			fixture.ctx,
			dbgen.GetConversionJobByIDAndProjectParams{
				ConversionJobID: job.ID,
				ProjectID:       fixture.projectID,
			},
		)
	if getErr != nil {
		t.Fatalf(
			"get job after rolled-back finalization: %v",
			getErr,
		)
	}

	if finalJob.Status != "running" {
		t.Fatalf(
			"expected job to remain running, got %q",
			finalJob.Status,
		)
	}
}

func cleanupConversionOutputContent(
	t *testing.T,
	fixture *conversionJobFixture,
	conversionJobID uuid.UUID,
	contentSHA256 string,
) {
	t.Helper()

	_, err := fixture.pool.Exec(
		fixture.ctx,
		`DELETE FROM conversion_job_outputs
		 WHERE conversion_job_id = $1`,
		conversionJobID,
	)
	if err != nil {
		t.Errorf(
			"delete conversion output for job %s: %v",
			conversionJobID,
			err,
		)
	}

	_, err = fixture.pool.Exec(
		fixture.ctx,
		`DELETE FROM content_objects
		 WHERE sha256 = $1`,
		contentSHA256,
	)
	if err != nil {
		t.Errorf(
			"delete conversion output content %q: %v",
			contentSHA256,
			err,
		)
	}
}

func newConversionOutputSHA256(
	label string,
) string {
	digest := sha256.Sum256(
		[]byte(label + ":" + uuid.New().String()),
	)

	return hex.EncodeToString(digest[:])
}

func TestConversionJobOutputQueriesReturnLatestSuccessfulReconversion(
	t *testing.T,
) {
	fixture := setupConversionJobTest(t)
	store := database.NewConversionJobStore(fixture.pool)

	projectFile := fixture.createProjectFile(
		fixture.projectID,
		"output-reconversion",
	)

	firstJob := createConversionJob(
		t,
		fixture,
		projectFile.ID,
	)

	_, err := store.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if err != nil {
		t.Fatalf(
			"claim first conversion job: %v",
			err,
		)
	}

	firstSHA256 := newConversionOutputSHA256(
		"first-reconversion-output",
	)
	defer cleanupConversionOutputContent(
		t,
		fixture,
		firstJob.ID,
		firstSHA256,
	)

	_, err = store.FinalizeConversionJobSuccess(
		fixture.ctx,
		conversionjobs.FinalizeSuccessInput{
			ConversionJobID: firstJob.ID,
			ContentSHA256:   firstSHA256,
			SizeBytes:       1000,
			MediaType:       "model/gltf-binary",
		},
	)
	if err != nil {
		t.Fatalf(
			"finalize first conversion: %v",
			err,
		)
	}

	firstStored, err :=
		fixture.queries.GetConversionJobOutputByJobID(
			fixture.ctx,
			firstJob.ID,
		)
	if err != nil {
		t.Fatalf(
			"get first conversion output: %v",
			err,
		)
	}

	if firstStored.ContentSha256 != firstSHA256 {
		t.Fatalf(
			"expected first output SHA-256 %q, got %q",
			firstSHA256,
			firstStored.ContentSha256,
		)
	}

	secondJob := createConversionJob(
		t,
		fixture,
		projectFile.ID,
	)

	_, err = store.ClaimNextPendingConversionJob(
		fixture.ctx,
	)
	if err != nil {
		t.Fatalf(
			"claim second conversion job: %v",
			err,
		)
	}

	time.Sleep(5 * time.Millisecond)

	secondSHA256 := newConversionOutputSHA256(
		"second-reconversion-output",
	)
	defer cleanupConversionOutputContent(
		t,
		fixture,
		secondJob.ID,
		secondSHA256,
	)

	_, err = store.FinalizeConversionJobSuccess(
		fixture.ctx,
		conversionjobs.FinalizeSuccessInput{
			ConversionJobID: secondJob.ID,
			ContentSHA256:   secondSHA256,
			SizeBytes:       2000,
			MediaType:       "model/gltf-binary",
		},
	)
	if err != nil {
		t.Fatalf(
			"finalize second conversion: %v",
			err,
		)
	}

	latest, err :=
		fixture.queries.GetLatestConversionJobOutputByProjectFile(
			fixture.ctx,
			dbgen.GetLatestConversionJobOutputByProjectFileParams{
				ProjectID:     fixture.projectID,
				ProjectFileID: projectFile.ID,
			},
		)
	if err != nil {
		t.Fatalf(
			"get latest reconversion output: %v",
			err,
		)
	}

	if latest.ConversionJobID != secondJob.ID {
		t.Fatalf(
			"expected latest output from job %s, got %s",
			secondJob.ID,
			latest.ConversionJobID,
		)
	}

	if latest.ContentSha256 != secondSHA256 {
		t.Fatalf(
			"expected latest output SHA-256 %q, got %q",
			secondSHA256,
			latest.ContentSha256,
		)
	}
}
