package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrRootRequired          = errors.New("storage root is required")
	ErrSourceRequired        = errors.New("content source is required")
	ErrContentSHA256Required = errors.New("content SHA-256 is required")
	ErrContentSHA256Invalid  = errors.New("content SHA-256 is invalid")
	ErrContentNotFound       = errors.New("stored content object not found")
	ErrContentIntegrity      = errors.New("stored content object failed integrity check")
)

type PutResult struct {
	SHA256    string
	SizeBytes int64
}

type FilesystemStore struct {
	root string
}

func NewFilesystemStore(root string) (*FilesystemStore, error) {
	if root == "" {
		return nil, ErrRootRequired
	}

	return &FilesystemStore{
		root: filepath.Clean(root),
	}, nil
}

func (s *FilesystemStore) Put(
	ctx context.Context,
	source io.Reader,
) (PutResult, error) {
	if source == nil {
		return PutResult{}, ErrSourceRequired
	}

	if err := ctx.Err(); err != nil {
		return PutResult{}, err
	}

	tempDir := filepath.Join(s.root, "tmp")
	if err := os.MkdirAll(tempDir, 0o750); err != nil {
		return PutResult{}, fmt.Errorf(
			"create storage temporary directory: %w",
			err,
		)
	}

	tempFile, err := os.CreateTemp(tempDir, "put-*")
	if err != nil {
		return PutResult{}, fmt.Errorf(
			"create temporary content object: %w",
			err,
		)
	}

	tempPath := tempFile.Name()
	tempClosed := false

	defer func() {
		if !tempClosed {
			_ = tempFile.Close()
		}
		_ = os.Remove(tempPath)
	}()

	hasher := sha256.New()

	sizeBytes, err := io.Copy(
		io.MultiWriter(tempFile, hasher),
		contextReader{
			ctx:    ctx,
			reader: source,
		},
	)
	if err != nil {
		return PutResult{}, fmt.Errorf(
			"write temporary content object: %w",
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		return PutResult{}, err
	}

	if err := tempFile.Sync(); err != nil {
		return PutResult{}, fmt.Errorf(
			"sync temporary content object: %w",
			err,
		)
	}

	if err := tempFile.Close(); err != nil {
		return PutResult{}, fmt.Errorf(
			"close temporary content object: %w",
			err,
		)
	}
	tempClosed = true

	sha256Hex := hex.EncodeToString(hasher.Sum(nil))

	result := PutResult{
		SHA256:    sha256Hex,
		SizeBytes: sizeBytes,
	}

	finalDir := filepath.Join(
		s.root,
		"objects",
		"sha256",
		sha256Hex[:2],
		sha256Hex[2:4],
	)
	if err := os.MkdirAll(finalDir, 0o750); err != nil {
		return PutResult{}, fmt.Errorf(
			"create content object directory: %w",
			err,
		)
	}

	finalPath := filepath.Join(finalDir, sha256Hex)

	if err := os.Link(tempPath, finalPath); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return PutResult{}, fmt.Errorf(
				"publish content object: %w",
				err,
			)
		}

		if err := validateExistingObject(
			ctx,
			finalPath,
			sha256Hex,
			sizeBytes,
		); err != nil {
			return PutResult{}, err
		}
	}

	if err := syncDirectory(finalDir); err != nil {
		return PutResult{}, fmt.Errorf(
			"sync content object directory: %w",
			err,
		)
	}

	return result, nil
}

func (s *FilesystemStore) Open(
	ctx context.Context,
	contentSHA256 string,
) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	normalizedSHA256, err := normalizeContentSHA256(
		contentSHA256,
	)
	if err != nil {
		return nil, err
	}

	path := filepath.Join(
		s.root,
		"objects",
		"sha256",
		normalizedSHA256[:2],
		normalizedSHA256[2:4],
		normalizedSHA256,
	)

	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrContentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf(
			"inspect stored content object: %w",
			err,
		)
	}

	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf(
			"%w: stored object is not a regular file",
			ErrContentIntegrity,
		)
	}

	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrContentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf(
			"open stored content object: %w",
			err,
		)
	}

	hasher := sha256.New()

	_, err = io.Copy(
		hasher,
		contextReader{
			ctx:    ctx,
			reader: file,
		},
	)
	if err != nil {
		_ = file.Close()

		if contextErr := ctx.Err(); contextErr != nil {
			return nil, contextErr
		}

		return nil, fmt.Errorf(
			"%w: hash stored content object: %v",
			ErrContentIntegrity,
			err,
		)
	}

	actualSHA256 := hex.EncodeToString(hasher.Sum(nil))
	if actualSHA256 != normalizedSHA256 {
		_ = file.Close()

		return nil, fmt.Errorf(
			"%w: expected SHA-256 %s, got %s",
			ErrContentIntegrity,
			normalizedSHA256,
			actualSHA256,
		)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		_ = file.Close()

		return nil, fmt.Errorf(
			"rewind stored content object: %w",
			err,
		)
	}

	if err := ctx.Err(); err != nil {
		_ = file.Close()
		return nil, err
	}

	return file, nil
}

func normalizeContentSHA256(
	value string,
) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return "", ErrContentSHA256Required
	}

	if len(normalized) != 64 {
		return "", ErrContentSHA256Invalid
	}

	decoded, err := hex.DecodeString(normalized)
	if err != nil || len(decoded) != sha256.Size {
		return "", ErrContentSHA256Invalid
	}

	return normalized, nil
}

func validateExistingObject(
	ctx context.Context,
	path string,
	expectedSHA256 string,
	expectedSize int64,
) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf(
			"%w: inspect existing object: %v",
			ErrContentIntegrity,
			err,
		)
	}

	if !info.Mode().IsRegular() {
		return fmt.Errorf(
			"%w: existing object is not a regular file",
			ErrContentIntegrity,
		)
	}

	if info.Size() != expectedSize {
		return fmt.Errorf(
			"%w: expected size %d, got %d",
			ErrContentIntegrity,
			expectedSize,
			info.Size(),
		)
	}

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf(
			"%w: open existing object: %v",
			ErrContentIntegrity,
			err,
		)
	}
	defer file.Close()

	hasher := sha256.New()

	sizeBytes, err := io.Copy(
		hasher,
		contextReader{
			ctx:    ctx,
			reader: file,
		},
	)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return contextErr
		}

		return fmt.Errorf(
			"%w: hash existing object: %v",
			ErrContentIntegrity,
			err,
		)
	}

	if sizeBytes != expectedSize {
		return fmt.Errorf(
			"%w: expected size %d, got %d",
			ErrContentIntegrity,
			expectedSize,
			sizeBytes,
		)
	}

	actualSHA256 := hex.EncodeToString(hasher.Sum(nil))
	if actualSHA256 != expectedSHA256 {
		return fmt.Errorf(
			"%w: expected SHA-256 %s, got %s",
			ErrContentIntegrity,
			expectedSHA256,
			actualSHA256,
		)
	}

	return nil
}

func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()

	return directory.Sync()
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}

	n, err := r.reader.Read(buffer)
	if err != nil {
		return n, err
	}

	if contextErr := r.ctx.Err(); contextErr != nil {
		return n, contextErr
	}

	return n, nil
}
