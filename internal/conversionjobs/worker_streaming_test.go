package conversionjobs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/google/uuid"
)

var errReadAfterSourceClose = errors.New(
	"read attempted after source close",
)

type closeAwareReadCloser struct {
	reader io.Reader
	closed bool
}

func (r *closeAwareReadCloser) Read(
	buffer []byte,
) (int, error) {
	if r.closed {
		return 0, errReadAfterSourceClose
	}

	return r.reader.Read(buffer)
}

func (r *closeAwareReadCloser) Close() error {
	r.closed = true
	return nil
}

type sourceBackedConverter struct{}

func (c *sourceBackedConverter) Convert(
	_ context.Context,
	_ dbgen.ProjectFile,
	source io.Reader,
) (ConversionResult, error) {
	return ConversionResult{
		Content:   io.NopCloser(source),
		MediaType: GLBMediaType,
	}, nil
}

func TestRunOnceStoresSourceBackedConverterOutputBeforeClosingSource(
	t *testing.T,
) {
	jobID := uuid.New()
	source := &closeAwareReadCloser{
		reader: bytes.NewReader([]byte("GLB")),
	}

	store := &fakeWorkerStore{
		claimJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
			Status:        "running",
			AttemptCount:  1,
		},
		projectFile: dbgen.ProjectFile{
			ContentSha256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	content := &fakeWorkerContentReader{
		content: source,
	}
	converter := &sourceBackedConverter{}

	worker, err := NewWorker(
		store,
		content,
		converter,
		time.Second,
	)
	if err != nil {
		t.Fatalf("create worker: %v", err)
	}

	processed, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run worker once: %v", err)
	}

	if !processed {
		t.Fatal("expected job to be processed")
	}

	if !content.putCalled {
		t.Fatal("expected source-backed output storage")
	}

	if !bytes.Equal(
		content.putContent,
		[]byte("GLB"),
	) {
		t.Fatalf(
			"expected stored source-backed bytes %q, got %q",
			[]byte("GLB"),
			content.putContent,
		)
	}

	if !source.closed {
		t.Fatal("expected source to close after output storage")
	}

	if !store.succeedCalled {
		t.Fatal("expected successful finalization")
	}
}
