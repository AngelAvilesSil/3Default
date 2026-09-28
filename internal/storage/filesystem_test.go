package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestNewFilesystemStoreRequiresRoot(t *testing.T) {
	_, err := NewFilesystemStore("")
	if !errors.Is(err, ErrRootRequired) {
		t.Fatalf(
			"expected ErrRootRequired, got %v",
			err,
		)
	}
}

func TestFilesystemStorePutRequiresSource(t *testing.T) {
	store, err := NewFilesystemStore(t.TempDir())
	if err != nil {
		t.Fatalf("create filesystem store: %v", err)
	}

	_, err = store.Put(context.Background(), nil)
	if !errors.Is(err, ErrSourceRequired) {
		t.Fatalf(
			"expected ErrSourceRequired, got %v",
			err,
		)
	}
}

func TestFilesystemStorePutStoresContentBySHA256(t *testing.T) {
	root := t.TempDir()

	store, err := NewFilesystemStore(root)
	if err != nil {
		t.Fatalf("create filesystem store: %v", err)
	}

	content := []byte("authoritative CAD source bytes")

	expectedDigest := sha256.Sum256(content)
	expectedSHA256 := hex.EncodeToString(expectedDigest[:])

	result, err := store.Put(
		context.Background(),
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatalf("put content: %v", err)
	}

	if result.SHA256 != expectedSHA256 {
		t.Fatalf(
			"expected SHA-256 %q, got %q",
			expectedSHA256,
			result.SHA256,
		)
	}

	if result.SizeBytes != int64(len(content)) {
		t.Fatalf(
			"expected size %d, got %d",
			len(content),
			result.SizeBytes,
		)
	}

	finalPath := objectPath(root, expectedSHA256)

	stored, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("read stored object: %v", err)
	}

	if !bytes.Equal(stored, content) {
		t.Fatalf(
			"expected stored content %q, got %q",
			content,
			stored,
		)
	}

	assertTemporaryDirectoryEmpty(t, root)
}

func TestFilesystemStorePutDeduplicatesExistingContent(
	t *testing.T,
) {
	root := t.TempDir()

	store, err := NewFilesystemStore(root)
	if err != nil {
		t.Fatalf("create filesystem store: %v", err)
	}

	content := []byte("same immutable CAD bytes")

	first, err := store.Put(
		context.Background(),
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatalf("put first content object: %v", err)
	}

	finalPath := objectPath(root, first.SHA256)

	if err := os.Chmod(finalPath, 0o400); err != nil {
		t.Fatalf("mark existing object read-only: %v", err)
	}

	second, err := store.Put(
		context.Background(),
		bytes.NewReader(content),
	)
	if err != nil {
		t.Fatalf("put duplicate content object: %v", err)
	}

	if second != first {
		t.Fatalf(
			"expected duplicate result %+v, got %+v",
			first,
			second,
		)
	}

	info, err := os.Stat(finalPath)
	if err != nil {
		t.Fatalf("stat deduplicated object: %v", err)
	}

	if info.Mode().Perm() != 0o400 {
		t.Fatalf(
			"expected existing object permissions to remain 0400, got %04o",
			info.Mode().Perm(),
		)
	}

	stored, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("read deduplicated object: %v", err)
	}

	if !bytes.Equal(stored, content) {
		t.Fatalf(
			"expected stored content %q, got %q",
			content,
			stored,
		)
	}

	assertTemporaryDirectoryEmpty(t, root)
}

func TestFilesystemStorePutHandlesConcurrentDuplicateContent(
	t *testing.T,
) {
	root := t.TempDir()

	store, err := NewFilesystemStore(root)
	if err != nil {
		t.Fatalf("create filesystem store: %v", err)
	}

	content := []byte(
		"concurrently uploaded authoritative CAD source",
	)

	const writers = 12

	results := make(chan PutResult, writers)
	errs := make(chan error, writers)

	var waitGroup sync.WaitGroup
	waitGroup.Add(writers)

	for range writers {
		go func() {
			defer waitGroup.Done()

			result, err := store.Put(
				context.Background(),
				bytes.NewReader(content),
			)
			if err != nil {
				errs <- err
				return
			}

			results <- result
		}()
	}

	waitGroup.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Errorf("put concurrent duplicate content: %v", err)
	}

	var expected *PutResult
	for result := range results {
		if expected == nil {
			current := result
			expected = &current
			continue
		}

		if result != *expected {
			t.Errorf(
				"expected concurrent result %+v, got %+v",
				*expected,
				result,
			)
		}
	}

	if expected == nil {
		t.Fatal("expected at least one successful put result")
	}

	objectCount := 0

	err = filepath.WalkDir(
		filepath.Join(root, "objects"),
		func(
			path string,
			entry os.DirEntry,
			walkErr error,
		) error {
			if walkErr != nil {
				return walkErr
			}

			if entry.Type().IsRegular() {
				objectCount++
			}

			return nil
		},
	)
	if err != nil {
		t.Fatalf("walk content objects: %v", err)
	}

	if objectCount != 1 {
		t.Fatalf(
			"expected exactly 1 stored content object, got %d",
			objectCount,
		)
	}

	assertTemporaryDirectoryEmpty(t, root)
}

func TestFilesystemStorePutRejectsExistingObjectWithWrongSize(
	t *testing.T,
) {
	root := t.TempDir()

	store, err := NewFilesystemStore(root)
	if err != nil {
		t.Fatalf("create filesystem store: %v", err)
	}

	content := []byte("expected authoritative bytes")

	digest := sha256.Sum256(content)
	sha256Hex := hex.EncodeToString(digest[:])
	finalPath := objectPath(root, sha256Hex)

	if err := os.MkdirAll(
		filepath.Dir(finalPath),
		0o750,
	); err != nil {
		t.Fatalf("create content object directory: %v", err)
	}

	invalidExisting := []byte("x")

	if err := os.WriteFile(
		finalPath,
		invalidExisting,
		0o600,
	); err != nil {
		t.Fatalf("create invalid existing object: %v", err)
	}

	_, err = store.Put(
		context.Background(),
		bytes.NewReader(content),
	)
	if !errors.Is(err, ErrContentIntegrity) {
		t.Fatalf(
			"expected ErrContentIntegrity, got %v",
			err,
		)
	}

	stored, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("read invalid existing object: %v", err)
	}

	if !bytes.Equal(stored, invalidExisting) {
		t.Fatalf(
			"expected existing object to remain unchanged, got %q",
			stored,
		)
	}

	assertTemporaryDirectoryEmpty(t, root)
}

func TestFilesystemStorePutRejectsExistingObjectWithWrongHash(
	t *testing.T,
) {
	root := t.TempDir()

	store, err := NewFilesystemStore(root)
	if err != nil {
		t.Fatalf("create filesystem store: %v", err)
	}

	content := []byte("expected authoritative bytes")

	digest := sha256.Sum256(content)
	sha256Hex := hex.EncodeToString(digest[:])
	finalPath := objectPath(root, sha256Hex)

	if err := os.MkdirAll(
		filepath.Dir(finalPath),
		0o750,
	); err != nil {
		t.Fatalf("create content object directory: %v", err)
	}

	invalidExisting := append([]byte(nil), content...)
	invalidExisting[0] ^= 0xff

	if err := os.WriteFile(
		finalPath,
		invalidExisting,
		0o600,
	); err != nil {
		t.Fatalf("create same-size invalid existing object: %v", err)
	}

	_, err = store.Put(
		context.Background(),
		bytes.NewReader(content),
	)
	if !errors.Is(err, ErrContentIntegrity) {
		t.Fatalf(
			"expected ErrContentIntegrity, got %v",
			err,
		)
	}

	stored, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("read invalid existing object: %v", err)
	}

	if !bytes.Equal(stored, invalidExisting) {
		t.Fatalf(
			"expected existing object to remain unchanged, got %q",
			stored,
		)
	}

	assertTemporaryDirectoryEmpty(t, root)
}

func TestFilesystemStorePutRejectsNonRegularExistingObject(
	t *testing.T,
) {
	root := t.TempDir()

	store, err := NewFilesystemStore(root)
	if err != nil {
		t.Fatalf("create filesystem store: %v", err)
	}

	content := []byte("authoritative bytes")

	digest := sha256.Sum256(content)
	sha256Hex := hex.EncodeToString(digest[:])
	finalPath := objectPath(root, sha256Hex)

	if err := os.MkdirAll(
		filepath.Dir(finalPath),
		0o750,
	); err != nil {
		t.Fatalf("create content object directory: %v", err)
	}

	if err := os.Mkdir(finalPath, 0o750); err != nil {
		t.Fatalf("create non-regular existing object: %v", err)
	}

	_, err = store.Put(
		context.Background(),
		bytes.NewReader(content),
	)
	if !errors.Is(err, ErrContentIntegrity) {
		t.Fatalf(
			"expected ErrContentIntegrity, got %v",
			err,
		)
	}

	info, err := os.Lstat(finalPath)
	if err != nil {
		t.Fatalf("inspect existing object: %v", err)
	}

	if !info.IsDir() {
		t.Fatalf(
			"expected existing directory to remain unchanged, got mode %v",
			info.Mode(),
		)
	}

	assertTemporaryDirectoryEmpty(t, root)
}

func TestFilesystemStorePutHonorsCanceledContext(t *testing.T) {
	root := t.TempDir()

	store, err := NewFilesystemStore(root)
	if err != nil {
		t.Fatalf("create filesystem store: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = store.Put(
		ctx,
		bytes.NewReader([]byte("unused")),
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context.Canceled, got %v",
			err,
		)
	}

	if _, err := os.Stat(filepath.Join(root, "objects")); !os.IsNotExist(err) {
		t.Fatalf(
			"expected no object directory after canceled put, got %v",
			err,
		)
	}
}

func TestFilesystemStorePutCleansUpAfterReaderFailure(
	t *testing.T,
) {
	root := t.TempDir()

	store, err := NewFilesystemStore(root)
	if err != nil {
		t.Fatalf("create filesystem store: %v", err)
	}

	expectedErr := errors.New("source read failed")

	_, err = store.Put(
		context.Background(),
		&failingReader{
			content: []byte("partial content"),
			err:     expectedErr,
		},
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf(
			"expected source error %v, got %v",
			expectedErr,
			err,
		)
	}

	if _, err := os.Stat(filepath.Join(root, "objects")); !os.IsNotExist(err) {
		t.Fatalf(
			"expected no published object after reader failure, got %v",
			err,
		)
	}

	assertTemporaryDirectoryEmpty(t, root)
}

func objectPath(root, sha256Hex string) string {
	return filepath.Join(
		root,
		"objects",
		"sha256",
		sha256Hex[:2],
		sha256Hex[2:4],
		sha256Hex,
	)
}

func assertTemporaryDirectoryEmpty(
	t *testing.T,
	root string,
) {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join(root, "tmp"))
	if err != nil {
		t.Fatalf("read storage temporary directory: %v", err)
	}

	if len(entries) != 0 {
		t.Fatalf(
			"expected empty storage temporary directory, got %d entries",
			len(entries),
		)
	}
}

type failingReader struct {
	content []byte
	err     error
	read    bool
}

func (r *failingReader) Read(buffer []byte) (int, error) {
	if r.read {
		return 0, r.err
	}

	r.read = true
	return copy(buffer, r.content), nil
}

var _ io.Reader = (*failingReader)(nil)
