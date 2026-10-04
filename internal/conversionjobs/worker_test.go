package conversionjobs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/AngelAvilesSil/3Default/internal/storage"
	"github.com/google/uuid"
)

type fakeWorkerStore struct {
	recoverCalled bool
	recoverErr    error

	claimCalled bool
	claimJob    dbgen.ConversionJob
	claimErr    error
	claimSignal chan struct{}

	requeueCalled bool
	requeueID     uuid.UUID
	requeueJob    dbgen.ConversionJob
	requeueErr    error

	getFileCalled bool
	getFileParams dbgen.GetProjectFileByIDAndProjectParams
	projectFile   dbgen.ProjectFile
	getFileErr    error

	succeedCalled  bool
	succeedID      uuid.UUID
	succeedInput   FinalizeSuccessInput
	finalizeOutput dbgen.ConversionJobOutput
	succeedErr     error

	failCalled bool
	failParams dbgen.MarkConversionJobFailedParams
	failJob    dbgen.ConversionJob
	failErr    error
}

func (f *fakeWorkerStore) RequeueRunningConversionJobs(
	_ context.Context,
) error {
	f.recoverCalled = true
	return f.recoverErr
}

func (f *fakeWorkerStore) ClaimNextPendingConversionJob(
	_ context.Context,
) (dbgen.ConversionJob, error) {
	f.claimCalled = true

	if f.claimSignal != nil {
		select {
		case f.claimSignal <- struct{}{}:
		default:
		}
	}

	return f.claimJob, f.claimErr
}

func (f *fakeWorkerStore) RequeueConversionJob(
	_ context.Context,
	conversionJobID uuid.UUID,
) (dbgen.ConversionJob, error) {
	f.requeueCalled = true
	f.requeueID = conversionJobID
	return f.requeueJob, f.requeueErr
}

func (f *fakeWorkerStore) GetProjectFileByIDAndProject(
	_ context.Context,
	arg dbgen.GetProjectFileByIDAndProjectParams,
) (dbgen.ProjectFile, error) {
	f.getFileCalled = true
	f.getFileParams = arg
	return f.projectFile, f.getFileErr
}

func (f *fakeWorkerStore) FinalizeConversionJobSuccess(
	_ context.Context,
	input FinalizeSuccessInput,
) (dbgen.ConversionJobOutput, error) {
	f.succeedCalled = true
	f.succeedID = input.ConversionJobID
	f.succeedInput = input
	return f.finalizeOutput, f.succeedErr
}

func (f *fakeWorkerStore) MarkConversionJobFailed(
	_ context.Context,
	arg dbgen.MarkConversionJobFailedParams,
) (dbgen.ConversionJob, error) {
	f.failCalled = true
	f.failParams = arg
	return f.failJob, f.failErr
}

type fakeWorkerContentReader struct {
	called        bool
	contentSHA256 string
	content       io.ReadCloser
	err           error

	putCalled              bool
	putContent             []byte
	putResult              storage.PutResult
	putErr                 error
	putStarted             chan struct{}
	putWaitForCancellation bool
}

func (f *fakeWorkerContentReader) Open(
	_ context.Context,
	contentSHA256 string,
) (io.ReadCloser, error) {
	f.called = true
	f.contentSHA256 = contentSHA256
	return f.content, f.err
}

func (f *fakeWorkerContentReader) Put(
	ctx context.Context,
	source io.Reader,
) (storage.PutResult, error) {
	f.putCalled = true

	if f.putStarted != nil {
		close(f.putStarted)
	}

	if f.putWaitForCancellation {
		<-ctx.Done()
		return storage.PutResult{}, ctx.Err()
	}

	content, err := io.ReadAll(source)
	if err != nil {
		return storage.PutResult{}, err
	}
	f.putContent = content

	if f.putErr != nil {
		return storage.PutResult{}, f.putErr
	}

	result := f.putResult
	if result.SHA256 == "" {
		result.SHA256 = strings.Repeat("9", 64)
		result.SizeBytes = int64(len(content))
	}

	return result, nil
}

type recordingConverter struct {
	called         bool
	projectFile    dbgen.ProjectFile
	content        []byte
	output         []byte
	mediaType      string
	outputCloseErr error
	produced       *trackingReadCloser
	err            error
}

func (c *recordingConverter) Convert(
	_ context.Context,
	projectFile dbgen.ProjectFile,
	source io.Reader,
) (ConversionResult, error) {
	c.called = true
	c.projectFile = projectFile

	content, err := io.ReadAll(source)
	if err != nil {
		return ConversionResult{}, err
	}

	c.content = content

	if c.err != nil {
		return ConversionResult{}, c.err
	}

	output := c.output
	if output == nil {
		output = []byte("derived GLB")
	}

	mediaType := c.mediaType
	if mediaType == "" {
		mediaType = "model/gltf-binary"
	}

	c.produced = &trackingReadCloser{
		reader:   bytes.NewReader(output),
		closeErr: c.outputCloseErr,
	}

	return ConversionResult{
		Content:   c.produced,
		MediaType: mediaType,
	}, nil
}

type blockingConverter struct {
	started chan struct{}
}

func (c *blockingConverter) Convert(
	ctx context.Context,
	_ dbgen.ProjectFile,
	_ io.Reader,
) (ConversionResult, error) {
	close(c.started)
	<-ctx.Done()
	return ConversionResult{}, ctx.Err()
}

type trackingReadCloser struct {
	reader   io.Reader
	closed   bool
	closeErr error
}

func (r *trackingReadCloser) Read(
	buffer []byte,
) (int, error) {
	return r.reader.Read(buffer)
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return r.closeErr
}

func TestNewWorkerRejectsInvalidDependencies(t *testing.T) {
	validStore := &fakeWorkerStore{}
	validContent := &fakeWorkerContentReader{}
	validConverter := &recordingConverter{}

	tests := []struct {
		name      string
		store     WorkerStore
		content   ContentStore
		converter Converter
		idleDelay time.Duration
		want      error
	}{
		{
			name:      "missing store",
			content:   validContent,
			converter: validConverter,
			idleDelay: time.Second,
			want:      ErrWorkerStoreRequired,
		},
		{
			name:      "missing content store",
			store:     validStore,
			converter: validConverter,
			idleDelay: time.Second,
			want:      ErrContentStoreRequired,
		},
		{
			name:      "missing converter",
			store:     validStore,
			content:   validContent,
			idleDelay: time.Second,
			want:      ErrConverterRequired,
		},
		{
			name:      "zero idle delay",
			store:     validStore,
			content:   validContent,
			converter: validConverter,
			want:      ErrIdleDelayInvalid,
		},
		{
			name:      "negative idle delay",
			store:     validStore,
			content:   validContent,
			converter: validConverter,
			idleDelay: -time.Second,
			want:      ErrIdleDelayInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			worker, err := NewWorker(
				test.store,
				test.content,
				test.converter,
				test.idleDelay,
			)

			if worker != nil {
				t.Fatal("expected no worker")
			}

			if !errors.Is(err, test.want) {
				t.Fatalf(
					"expected %v, got %v",
					test.want,
					err,
				)
			}
		})
	}
}

func TestRunOnceTreatsEmptyQueueAsIdle(t *testing.T) {
	store := &fakeWorkerStore{
		claimErr: ErrNoPendingConversionJob,
	}
	content := &fakeWorkerContentReader{}
	converter := &recordingConverter{}

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

	if processed {
		t.Fatal("expected no job to be processed")
	}

	if !store.claimCalled {
		t.Fatal("expected pending-job claim")
	}

	if store.getFileCalled {
		t.Fatal("did not expect source metadata lookup")
	}

	if content.called {
		t.Fatal("did not expect content lookup")
	}

	if converter.called {
		t.Fatal("did not expect converter call")
	}
}

func TestRunOnceConvertsSourceAndMarksJobSucceeded(
	t *testing.T,
) {
	jobID := uuid.New()
	projectID := uuid.New()
	projectFileID := uuid.New()
	contentSHA256 := strings.Repeat("a", 64)
	sourceBytes := []byte("authoritative CAD source")

	source := &trackingReadCloser{
		reader: bytes.NewReader(sourceBytes),
	}

	store := &fakeWorkerStore{
		claimJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     projectID,
			ProjectFileID: projectFileID,
			Status:        "running",
			AttemptCount:  1,
		},
		projectFile: dbgen.ProjectFile{
			ID:               projectFileID,
			ProjectID:        projectID,
			ContentSha256:    contentSHA256,
			OriginalFilename: "assembly.step",
		},
	}
	content := &fakeWorkerContentReader{
		content: source,
	}
	converter := &recordingConverter{}

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
		t.Fatal("expected conversion job to be processed")
	}

	if !store.getFileCalled {
		t.Fatal("expected source metadata lookup")
	}

	if store.getFileParams.ProjectID != projectID ||
		store.getFileParams.ProjectFileID != projectFileID {
		t.Fatalf(
			"unexpected source metadata params: %+v",
			store.getFileParams,
		)
	}

	if !content.called {
		t.Fatal("expected source content lookup")
	}

	if content.contentSHA256 != contentSHA256 {
		t.Fatalf(
			"expected content SHA-256 %q, got %q",
			contentSHA256,
			content.contentSHA256,
		)
	}

	if !converter.called {
		t.Fatal("expected converter call")
	}

	if converter.projectFile.ID != projectFileID {
		t.Fatalf(
			"expected converter project file %s, got %s",
			projectFileID,
			converter.projectFile.ID,
		)
	}

	if !bytes.Equal(converter.content, sourceBytes) {
		t.Fatalf(
			"expected converter source %q, got %q",
			sourceBytes,
			converter.content,
		)
	}

	if !content.putCalled {
		t.Fatal("expected converted output to be stored")
	}

	if !bytes.Equal(
		content.putContent,
		[]byte("derived GLB"),
	) {
		t.Fatalf(
			"expected stored output %q, got %q",
			[]byte("derived GLB"),
			content.putContent,
		)
	}

	if converter.produced == nil ||
		!converter.produced.closed {
		t.Fatal("expected conversion output to be closed")
	}

	if !source.closed {
		t.Fatal("expected source content to be closed")
	}

	if !store.succeedCalled {
		t.Fatal("expected conversion job success transition")
	}

	if store.succeedID != jobID {
		t.Fatalf(
			"expected success job ID %s, got %s",
			jobID,
			store.succeedID,
		)
	}

	if store.succeedInput.ContentSHA256 !=
		strings.Repeat("9", 64) {
		t.Fatalf(
			"unexpected finalized SHA-256 %q",
			store.succeedInput.ContentSHA256,
		)
	}

	if store.succeedInput.SizeBytes !=
		int64(len("derived GLB")) {
		t.Fatalf(
			"unexpected finalized size %d",
			store.succeedInput.SizeBytes,
		)
	}

	if store.succeedInput.MediaType !=
		"model/gltf-binary" {
		t.Fatalf(
			"unexpected finalized media type %q",
			store.succeedInput.MediaType,
		)
	}

	if store.failCalled {
		t.Fatal("did not expect failure transition")
	}

	if store.requeueCalled {
		t.Fatal("did not expect requeue")
	}
}

func TestRunOnceRecordsConverterFailure(t *testing.T) {
	jobID := uuid.New()
	converterErr := errors.New("converter exploded")

	source := &trackingReadCloser{
		reader: bytes.NewReader([]byte("CAD")),
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
			ContentSha256: strings.Repeat("b", 64),
		},
	}
	content := &fakeWorkerContentReader{
		content: source,
	}
	converter := &recordingConverter{
		err: converterErr,
	}

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

	if !source.closed {
		t.Fatal("expected source content to be closed")
	}

	if !store.failCalled {
		t.Fatal("expected failed conversion transition")
	}

	if store.failParams.ConversionJobID != jobID {
		t.Fatalf(
			"expected failed job ID %s, got %s",
			jobID,
			store.failParams.ConversionJobID,
		)
	}

	if store.failParams.LastError == nil {
		t.Fatal("expected persisted failure message")
	}

	if !strings.Contains(
		*store.failParams.LastError,
		converterErr.Error(),
	) {
		t.Fatalf(
			"expected failure message to contain %q, got %q",
			converterErr,
			*store.failParams.LastError,
		)
	}

	if store.succeedCalled {
		t.Fatal("did not expect success transition")
	}
}

func TestRunOnceRecordsSourceOpenFailure(t *testing.T) {
	jobID := uuid.New()
	openErr := errors.New("source object unavailable")

	store := &fakeWorkerStore{
		claimJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
			Status:        "running",
			AttemptCount:  1,
		},
		projectFile: dbgen.ProjectFile{
			ContentSha256: strings.Repeat("c", 64),
		},
	}
	content := &fakeWorkerContentReader{
		err: openErr,
	}
	converter := &recordingConverter{}

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

	if converter.called {
		t.Fatal("did not expect converter after source-open failure")
	}

	if !store.failCalled {
		t.Fatal("expected failed conversion transition")
	}

	if store.failParams.LastError == nil ||
		!strings.Contains(
			*store.failParams.LastError,
			openErr.Error(),
		) {
		t.Fatalf(
			"expected source-open failure to be persisted, got %+v",
			store.failParams.LastError,
		)
	}
}

func TestRunOnceRecordsMetadataLookupFailure(t *testing.T) {
	jobID := uuid.New()
	metadataErr := errors.New("project file metadata missing")

	store := &fakeWorkerStore{
		claimJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
			Status:        "running",
			AttemptCount:  1,
		},
		getFileErr: metadataErr,
	}
	content := &fakeWorkerContentReader{}
	converter := &recordingConverter{}

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

	if content.called {
		t.Fatal("did not expect content lookup")
	}

	if converter.called {
		t.Fatal("did not expect converter call")
	}

	if !store.failCalled {
		t.Fatal("expected failed conversion transition")
	}

	if store.failParams.LastError == nil ||
		!strings.Contains(
			*store.failParams.LastError,
			metadataErr.Error(),
		) {
		t.Fatalf(
			"expected metadata failure to be persisted, got %+v",
			store.failParams.LastError,
		)
	}
}

func TestRunOnceRequeuesJobWhenConversionIsCanceled(
	t *testing.T,
) {
	jobID := uuid.New()

	store := &fakeWorkerStore{
		claimJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
			Status:        "running",
			AttemptCount:  1,
		},
		projectFile: dbgen.ProjectFile{
			ContentSha256: strings.Repeat("d", 64),
		},
	}
	content := &fakeWorkerContentReader{
		content: &trackingReadCloser{
			reader: bytes.NewReader([]byte("CAD")),
		},
	}
	converter := &blockingConverter{
		started: make(chan struct{}),
	}

	worker, err := NewWorker(
		store,
		content,
		converter,
		time.Second,
	)
	if err != nil {
		t.Fatalf("create worker: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		processed bool
		err       error
	}

	resultChannel := make(chan result, 1)

	go func() {
		processed, err := worker.RunOnce(ctx)
		resultChannel <- result{
			processed: processed,
			err:       err,
		}
	}()

	select {
	case <-converter.started:
	case <-time.After(2 * time.Second):
		t.Fatal("converter did not start")
	}

	cancel()

	select {
	case result := <-resultChannel:
		if !result.processed {
			t.Fatal("expected claimed job to count as processed")
		}

		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf(
				"expected context.Canceled, got %v",
				result.err,
			)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("worker did not return after cancellation")
	}

	if !store.requeueCalled {
		t.Fatal("expected canceled job to be requeued")
	}

	if store.requeueID != jobID {
		t.Fatalf(
			"expected requeued job ID %s, got %s",
			jobID,
			store.requeueID,
		)
	}

	if store.failCalled {
		t.Fatal("did not expect canceled job to be failed")
	}

	if store.succeedCalled {
		t.Fatal("did not expect canceled job to succeed")
	}
}

func TestRunOnceSurfacesSuccessPersistenceFailure(
	t *testing.T,
) {
	persistErr := errors.New("success persistence failed")

	store := &fakeWorkerStore{
		claimJob: dbgen.ConversionJob{
			ID:            uuid.New(),
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
			Status:        "running",
		},
		projectFile: dbgen.ProjectFile{
			ContentSha256: strings.Repeat("e", 64),
		},
		succeedErr: persistErr,
	}
	content := &fakeWorkerContentReader{
		content: io.NopCloser(
			bytes.NewReader([]byte("CAD")),
		),
	}
	converter := &recordingConverter{}

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

	if !processed {
		t.Fatal("expected job to be processed")
	}

	if !errors.Is(err, persistErr) {
		t.Fatalf(
			"expected wrapped persistence error %v, got %v",
			persistErr,
			err,
		)
	}
}

func TestRunOnceSurfacesFailurePersistenceFailure(
	t *testing.T,
) {
	persistErr := errors.New("failure persistence failed")

	store := &fakeWorkerStore{
		claimJob: dbgen.ConversionJob{
			ID:            uuid.New(),
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
			Status:        "running",
		},
		projectFile: dbgen.ProjectFile{
			ContentSha256: strings.Repeat("f", 64),
		},
		failErr: persistErr,
	}
	content := &fakeWorkerContentReader{
		content: io.NopCloser(
			bytes.NewReader([]byte("CAD")),
		),
	}
	converter := &recordingConverter{
		err: errors.New("conversion failed"),
	}

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

	if !processed {
		t.Fatal("expected job to be processed")
	}

	if !errors.Is(err, persistErr) {
		t.Fatalf(
			"expected wrapped persistence error %v, got %v",
			persistErr,
			err,
		)
	}
}

func TestRunRecoversRunningJobsBeforePolling(
	t *testing.T,
) {
	claimSignal := make(chan struct{}, 1)

	store := &fakeWorkerStore{
		claimErr:    ErrNoPendingConversionJob,
		claimSignal: claimSignal,
	}
	content := &fakeWorkerContentReader{}
	converter := &recordingConverter{}

	worker, err := NewWorker(
		store,
		content,
		converter,
		time.Hour,
	)
	if err != nil {
		t.Fatalf("create worker: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultChannel := make(chan error, 1)

	go func() {
		resultChannel <- worker.Run(ctx)
	}()

	select {
	case <-claimSignal:
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not poll after recovery")
	}

	if !store.recoverCalled {
		t.Fatal("expected startup recovery before polling")
	}

	cancel()

	select {
	case err := <-resultChannel:
		if err != nil {
			t.Fatalf("run worker: %v", err)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}

func TestRunStopsWhenStartupRecoveryFails(t *testing.T) {
	recoveryErr := errors.New("recovery failed")

	store := &fakeWorkerStore{
		recoverErr: recoveryErr,
	}

	worker, err := NewWorker(
		store,
		&fakeWorkerContentReader{},
		&recordingConverter{},
		time.Second,
	)
	if err != nil {
		t.Fatalf("create worker: %v", err)
	}

	err = worker.Run(context.Background())
	if !errors.Is(err, recoveryErr) {
		t.Fatalf(
			"expected wrapped recovery error %v, got %v",
			recoveryErr,
			err,
		)
	}

	if store.claimCalled {
		t.Fatal(
			"expected no queue polling after recovery failure",
		)
	}
}

func TestRunOnceRecordsSourceCloseFailure(t *testing.T) {
	jobID := uuid.New()
	closeErr := errors.New("source close failed")

	source := &trackingReadCloser{
		reader:   bytes.NewReader([]byte("CAD")),
		closeErr: closeErr,
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
			ContentSha256: strings.Repeat("1", 64),
		},
	}
	content := &fakeWorkerContentReader{
		content: source,
	}
	converter := &recordingConverter{}

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

	if !source.closed {
		t.Fatal("expected source close to be attempted")
	}

	if !store.failCalled {
		t.Fatal("expected source-close failure to fail job")
	}

	if store.failParams.LastError == nil ||
		!strings.Contains(
			*store.failParams.LastError,
			closeErr.Error(),
		) {
		t.Fatalf(
			"expected persisted close failure, got %+v",
			store.failParams.LastError,
		)
	}

	if store.succeedCalled {
		t.Fatal(
			"did not expect success after source-close failure",
		)
	}
}

func TestRunOnceSurfacesCancellationRequeueFailure(
	t *testing.T,
) {
	jobID := uuid.New()
	requeueErr := errors.New("requeue persistence failed")

	store := &fakeWorkerStore{
		claimJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
			Status:        "running",
			AttemptCount:  1,
		},
		projectFile: dbgen.ProjectFile{
			ContentSha256: strings.Repeat("2", 64),
		},
		requeueErr: requeueErr,
	}
	content := &fakeWorkerContentReader{
		content: io.NopCloser(
			bytes.NewReader([]byte("CAD")),
		),
	}
	converter := &blockingConverter{
		started: make(chan struct{}),
	}

	worker, err := NewWorker(
		store,
		content,
		converter,
		time.Second,
	)
	if err != nil {
		t.Fatalf("create worker: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resultChannel := make(chan error, 1)

	go func() {
		_, err := worker.RunOnce(ctx)
		resultChannel <- err
	}()

	select {
	case <-converter.started:
	case <-time.After(2 * time.Second):
		t.Fatal("converter did not start")
	}

	cancel()

	select {
	case err := <-resultChannel:
		if !errors.Is(err, requeueErr) {
			t.Fatalf(
				"expected wrapped requeue error %v, got %v",
				requeueErr,
				err,
			)
		}

	case <-time.After(2 * time.Second):
		t.Fatal("worker did not return after cancellation")
	}

	if !store.requeueCalled {
		t.Fatal("expected cancellation requeue attempt")
	}

	if store.failCalled {
		t.Fatal(
			"did not expect canceled job to be marked failed",
		)
	}
}

func TestRunOnceRecordsOutputStorageFailure(
	t *testing.T,
) {
	jobID := uuid.New()
	putErr := errors.New("output storage failed")

	store := &fakeWorkerStore{
		claimJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
			Status:        "running",
			AttemptCount:  1,
		},
		projectFile: dbgen.ProjectFile{
			ContentSha256: strings.Repeat("3", 64),
		},
	}
	content := &fakeWorkerContentReader{
		content: io.NopCloser(
			bytes.NewReader([]byte("CAD")),
		),
		putErr: putErr,
	}
	converter := &recordingConverter{}

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
		t.Fatal("expected output storage attempt")
	}

	if converter.produced == nil ||
		!converter.produced.closed {
		t.Fatal("expected failed output storage stream to close")
	}

	if !store.failCalled {
		t.Fatal("expected output storage failure to fail job")
	}

	if store.failParams.LastError == nil ||
		!strings.Contains(
			*store.failParams.LastError,
			putErr.Error(),
		) {
		t.Fatalf(
			"expected persisted output storage failure, got %+v",
			store.failParams.LastError,
		)
	}

	if store.succeedCalled {
		t.Fatal("did not expect success finalization")
	}
}

func TestRunOnceRecordsOutputCloseFailure(
	t *testing.T,
) {
	jobID := uuid.New()
	closeErr := errors.New("output close failed")

	store := &fakeWorkerStore{
		claimJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
			Status:        "running",
			AttemptCount:  1,
		},
		projectFile: dbgen.ProjectFile{
			ContentSha256: strings.Repeat("4", 64),
		},
	}
	content := &fakeWorkerContentReader{
		content: io.NopCloser(
			bytes.NewReader([]byte("CAD")),
		),
	}
	converter := &recordingConverter{
		outputCloseErr: closeErr,
	}

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

	if !store.failCalled {
		t.Fatal("expected output-close failure to fail job")
	}

	if store.failParams.LastError == nil ||
		!strings.Contains(
			*store.failParams.LastError,
			closeErr.Error(),
		) {
		t.Fatalf(
			"expected persisted output close failure, got %+v",
			store.failParams.LastError,
		)
	}

	if store.succeedCalled {
		t.Fatal("did not expect success finalization")
	}
}

type fixedResultConverter struct {
	result ConversionResult
	err    error
}

func (c *fixedResultConverter) Convert(
	_ context.Context,
	_ dbgen.ProjectFile,
	_ io.Reader,
) (ConversionResult, error) {
	return c.result, c.err
}

func TestRunOnceRecordsMissingOutputContent(
	t *testing.T,
) {
	jobID := uuid.New()

	store := &fakeWorkerStore{
		claimJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
			Status:        "running",
			AttemptCount:  1,
		},
		projectFile: dbgen.ProjectFile{
			ContentSha256: strings.Repeat("5", 64),
		},
	}
	content := &fakeWorkerContentReader{
		content: io.NopCloser(
			bytes.NewReader([]byte("CAD")),
		),
	}
	converter := &fixedResultConverter{
		result: ConversionResult{
			MediaType: "model/gltf-binary",
		},
	}

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

	if content.putCalled {
		t.Fatal("did not expect storage for missing output content")
	}

	if !store.failCalled {
		t.Fatal("expected missing output content to fail job")
	}

	if store.failParams.LastError == nil ||
		!strings.Contains(
			*store.failParams.LastError,
			ErrOutputContentRequired.Error(),
		) {
		t.Fatalf(
			"expected persisted missing-content error, got %+v",
			store.failParams.LastError,
		)
	}

	if store.succeedCalled {
		t.Fatal("did not expect success finalization")
	}
}

func TestRunOnceRecordsBlankOutputMediaType(
	t *testing.T,
) {
	jobID := uuid.New()
	output := &trackingReadCloser{
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
			ContentSha256: strings.Repeat("6", 64),
		},
	}
	content := &fakeWorkerContentReader{
		content: io.NopCloser(
			bytes.NewReader([]byte("CAD")),
		),
	}
	converter := &fixedResultConverter{
		result: ConversionResult{
			Content:   output,
			MediaType: "   ",
		},
	}

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

	if !output.closed {
		t.Fatal("expected rejected output stream to close")
	}

	if content.putCalled {
		t.Fatal("did not expect storage for blank media type")
	}

	if !store.failCalled {
		t.Fatal("expected blank media type to fail job")
	}

	if store.failParams.LastError == nil ||
		!strings.Contains(
			*store.failParams.LastError,
			ErrOutputMediaTypeRequired.Error(),
		) {
		t.Fatalf(
			"expected persisted blank-media-type error, got %+v",
			store.failParams.LastError,
		)
	}

	if store.succeedCalled {
		t.Fatal("did not expect success finalization")
	}
}

func TestRunOnceRecordsInvalidStoredOutputMetadata(
	t *testing.T,
) {
	tests := []struct {
		name      string
		putResult storage.PutResult
		want      error
	}{
		{
			name: "invalid SHA-256",
			putResult: storage.PutResult{
				SHA256:    "not-a-sha256",
				SizeBytes: 3,
			},
			want: ErrStoredOutputSHA256Invalid,
		},
		{
			name: "negative size",
			putResult: storage.PutResult{
				SHA256:    strings.Repeat("7", 64),
				SizeBytes: -1,
			},
			want: ErrStoredOutputSizeInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeWorkerStore{
				claimJob: dbgen.ConversionJob{
					ID:            uuid.New(),
					ProjectID:     uuid.New(),
					ProjectFileID: uuid.New(),
					Status:        "running",
					AttemptCount:  1,
				},
				projectFile: dbgen.ProjectFile{
					ContentSha256: strings.Repeat("8", 64),
				},
			}
			content := &fakeWorkerContentReader{
				content: io.NopCloser(
					bytes.NewReader([]byte("CAD")),
				),
				putResult: test.putResult,
			}
			converter := &recordingConverter{}

			worker, err := NewWorker(
				store,
				content,
				converter,
				time.Second,
			)
			if err != nil {
				t.Fatalf("create worker: %v", err)
			}

			processed, err := worker.RunOnce(
				context.Background(),
			)
			if err != nil {
				t.Fatalf("run worker once: %v", err)
			}

			if !processed {
				t.Fatal("expected job to be processed")
			}

			if !store.failCalled {
				t.Fatal(
					"expected invalid stored output metadata to fail job",
				)
			}

			if store.failParams.LastError == nil ||
				!strings.Contains(
					*store.failParams.LastError,
					test.want.Error(),
				) {
				t.Fatalf(
					"expected persisted error %v, got %+v",
					test.want,
					store.failParams.LastError,
				)
			}

			if store.succeedCalled {
				t.Fatal(
					"did not expect success finalization",
				)
			}
		})
	}
}

func TestRunOnceRequeuesJobWhenOutputStorageIsCanceled(
	t *testing.T,
) {
	jobID := uuid.New()
	putStarted := make(chan struct{})

	store := &fakeWorkerStore{
		claimJob: dbgen.ConversionJob{
			ID:            jobID,
			ProjectID:     uuid.New(),
			ProjectFileID: uuid.New(),
			Status:        "running",
			AttemptCount:  1,
		},
		projectFile: dbgen.ProjectFile{
			ContentSha256: strings.Repeat("a", 64),
		},
	}
	content := &fakeWorkerContentReader{
		content: io.NopCloser(
			bytes.NewReader([]byte("CAD")),
		),
		putStarted:             putStarted,
		putWaitForCancellation: true,
	}
	converter := &recordingConverter{}

	worker, err := NewWorker(
		store,
		content,
		converter,
		time.Second,
	)
	if err != nil {
		t.Fatalf("create worker: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	type result struct {
		processed bool
		err       error
	}

	resultChannel := make(chan result, 1)

	go func() {
		processed, err := worker.RunOnce(ctx)
		resultChannel <- result{
			processed: processed,
			err:       err,
		}
	}()

	select {
	case <-putStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("output storage did not start")
	}

	cancel()

	select {
	case result := <-resultChannel:
		if !result.processed {
			t.Fatal(
				"expected claimed job to count as processed",
			)
		}

		if !errors.Is(result.err, context.Canceled) {
			t.Fatalf(
				"expected context.Canceled, got %v",
				result.err,
			)
		}

	case <-time.After(2 * time.Second):
		t.Fatal(
			"worker did not return after output-storage cancellation",
		)
	}

	if !store.requeueCalled {
		t.Fatal("expected canceled job to be requeued")
	}

	if store.requeueID != jobID {
		t.Fatalf(
			"expected requeued job ID %s, got %s",
			jobID,
			store.requeueID,
		)
	}

	if converter.produced == nil ||
		!converter.produced.closed {
		t.Fatal(
			"expected canceled output stream to close",
		)
	}

	if store.failCalled {
		t.Fatal(
			"did not expect canceled job to be marked failed",
		)
	}

	if store.succeedCalled {
		t.Fatal(
			"did not expect canceled job to succeed",
		)
	}
}
