package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestHashPasswordUsesArgon2idAndUniqueSalt(
	t *testing.T,
) {
	firstHash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash first password: %v", err)
	}

	secondHash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash second password: %v", err)
	}

	expectedPrefix := "$argon2id$v=19$m=19456,t=2,p=1$"

	if !strings.HasPrefix(firstHash, expectedPrefix) {
		t.Fatalf(
			"expected Argon2id hash prefix %q, got %q",
			expectedPrefix,
			firstHash,
		)
	}

	if firstHash == secondHash {
		t.Fatal("expected different hashes from unique salts")
	}
}

func TestVerifyPasswordAcceptsCorrectPassword(t *testing.T) {
	encodedHash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	matches, err := VerifyPassword(
		"correct horse battery staple",
		encodedHash,
	)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}

	if !matches {
		t.Fatal("expected password to match")
	}
}

func TestVerifyPasswordRejectsIncorrectPassword(t *testing.T) {
	encodedHash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	matches, err := VerifyPassword(
		"incorrect password",
		encodedHash,
	)
	if err != nil {
		t.Fatalf("verify password: %v", err)
	}

	if matches {
		t.Fatal("expected password not to match")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	testCases := []string{
		"",
		"not-a-password-hash",
		"$argon2i$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=18$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=0,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=19456,t=0,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=19456,t=2,p=0$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=19456,t=2,p=1$%%%$aGFzaA",
		"$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$%%%",
		"$argon2id$v=19$m=65537,t=2,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=19456,t=6,p=1$c2FsdA$aGFzaA",
		"$argon2id$v=19$m=19456,t=2,p=5$c2FsdA$aGFzaA",
	}

	for _, encodedHash := range testCases {
		t.Run(encodedHash, func(t *testing.T) {
			matches, err := VerifyPassword(
				"password",
				encodedHash,
			)

			if matches {
				t.Fatal("expected malformed hash not to match")
			}

			if !errors.Is(err, ErrInvalidPasswordHash) {
				t.Fatalf(
					"expected ErrInvalidPasswordHash, got %v",
					err,
				)
			}
		})
	}
}
