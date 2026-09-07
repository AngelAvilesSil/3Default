package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	passwordHashMemory      uint32 = 19 * 1024
	passwordHashIterations  uint32 = 2
	passwordHashParallelism uint8  = 1
	passwordHashSaltSize           = 16
	passwordHashKeySize            = 32

	passwordHashMaxMemory      uint32 = 64 * 1024
	passwordHashMaxIterations  uint32 = 5
	passwordHashMaxParallelism uint8  = 4
	passwordHashMaxSaltSize           = 64
	passwordHashMaxKeySize            = 64
)

var ErrInvalidPasswordHash = errors.New("invalid password hash")

func HashPassword(password string) (string, error) {
	salt := make([]byte, passwordHashSaltSize)

	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}

	hash := argon2.IDKey(
		[]byte(password),
		salt,
		passwordHashIterations,
		passwordHashMemory,
		passwordHashParallelism,
		passwordHashKeySize,
	)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		passwordHashMemory,
		passwordHashIterations,
		passwordHashParallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func VerifyPassword(
	password string,
	encodedHash string,
) (bool, error) {
	params, salt, expectedHash, err := parsePasswordHash(encodedHash)
	if err != nil {
		return false, err
	}

	actualHash := argon2.IDKey(
		[]byte(password),
		salt,
		params.iterations,
		params.memory,
		params.parallelism,
		uint32(len(expectedHash)),
	)

	return subtle.ConstantTimeCompare(
		actualHash,
		expectedHash,
	) == 1, nil
}

type passwordHashParams struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
}

func parsePasswordHash(
	encodedHash string,
) (passwordHashParams, []byte, []byte, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 ||
		parts[0] != "" ||
		parts[1] != "argon2id" {
		return passwordHashParams{}, nil, nil, ErrInvalidPasswordHash
	}

	version, err := parsePasswordHashValue(
		parts[2],
		"v=",
	)
	if err != nil || version != uint64(argon2.Version) {
		return passwordHashParams{}, nil, nil, ErrInvalidPasswordHash
	}

	paramParts := strings.Split(parts[3], ",")
	if len(paramParts) != 3 {
		return passwordHashParams{}, nil, nil, ErrInvalidPasswordHash
	}

	memory, err := parsePasswordHashValue(
		paramParts[0],
		"m=",
	)
	if err != nil ||
		memory == 0 ||
		memory > uint64(passwordHashMaxMemory) {
		return passwordHashParams{}, nil, nil, ErrInvalidPasswordHash
	}

	iterations, err := parsePasswordHashValue(
		paramParts[1],
		"t=",
	)
	if err != nil ||
		iterations == 0 ||
		iterations > uint64(passwordHashMaxIterations) {
		return passwordHashParams{}, nil, nil, ErrInvalidPasswordHash
	}

	parallelism, err := parsePasswordHashValue(
		paramParts[2],
		"p=",
	)
	if err != nil ||
		parallelism == 0 ||
		parallelism > uint64(passwordHashMaxParallelism) {
		return passwordHashParams{}, nil, nil, ErrInvalidPasswordHash
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil ||
		len(salt) == 0 ||
		len(salt) > passwordHashMaxSaltSize {
		return passwordHashParams{}, nil, nil, ErrInvalidPasswordHash
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil ||
		len(expectedHash) == 0 ||
		len(expectedHash) > passwordHashMaxKeySize {
		return passwordHashParams{}, nil, nil, ErrInvalidPasswordHash
	}

	return passwordHashParams{
		memory:      uint32(memory),
		iterations:  uint32(iterations),
		parallelism: uint8(parallelism),
	}, salt, expectedHash, nil
}

func parsePasswordHashValue(
	value string,
	prefix string,
) (uint64, error) {
	raw, ok := strings.CutPrefix(value, prefix)
	if !ok || raw == "" {
		return 0, ErrInvalidPasswordHash
	}

	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, ErrInvalidPasswordHash
	}

	return parsed, nil
}
