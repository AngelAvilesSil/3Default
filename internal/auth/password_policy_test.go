package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type passwordBlocklistStub struct {
	isBlocked func(
		ctx context.Context,
		password string,
		email string,
	) (bool, error)
}

func (b passwordBlocklistStub) IsBlocked(
	ctx context.Context,
	password string,
	email string,
) (bool, error) {
	return b.isBlocked(
		ctx,
		password,
		email,
	)
}

func allowAllPasswordsBlocklist() passwordBlocklistStub {
	return passwordBlocklistStub{
		isBlocked: func(
			context.Context,
			string,
			string,
		) (bool, error) {
			return false, nil
		},
	}
}

func TestPasswordPolicyAcceptsMinimumLength(t *testing.T) {
	policy := NewPasswordPolicy(
		allowAllPasswordsBlocklist(),
	)

	password := "abcdefghijklmno"

	got, err := policy.NormalizeAndValidate(
		context.Background(),
		password,
		"person@example.com",
	)
	if err != nil {
		t.Fatalf(
			"NormalizeAndValidate() error = %v",
			err,
		)
	}

	if got != password {
		t.Fatalf(
			"NormalizeAndValidate() = %q, want %q",
			got,
			password,
		)
	}
}

func TestPasswordPolicyRejectsTooShortPassword(t *testing.T) {
	policy := NewPasswordPolicy(
		allowAllPasswordsBlocklist(),
	)

	_, err := policy.NormalizeAndValidate(
		context.Background(),
		"abcdefghijklmn",
		"person@example.com",
	)
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf(
			"NormalizeAndValidate() error = %v, want ErrPasswordTooShort",
			err,
		)
	}
}

func TestPasswordPolicyAcceptsMaximumLength(t *testing.T) {
	policy := NewPasswordPolicy(
		allowAllPasswordsBlocklist(),
	)

	password := strings.Repeat(
		"a",
		passwordMaxCharacters,
	)

	_, err := policy.NormalizeAndValidate(
		context.Background(),
		password,
		"person@example.com",
	)
	if err != nil {
		t.Fatalf(
			"NormalizeAndValidate() error = %v",
			err,
		)
	}
}

func TestPasswordPolicyRejectsTooLongPassword(t *testing.T) {
	policy := NewPasswordPolicy(
		allowAllPasswordsBlocklist(),
	)

	password := strings.Repeat(
		"a",
		passwordMaxCharacters+1,
	)

	_, err := policy.NormalizeAndValidate(
		context.Background(),
		password,
		"person@example.com",
	)
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf(
			"NormalizeAndValidate() error = %v, want ErrPasswordTooLong",
			err,
		)
	}
}

func TestPasswordPolicyNormalizesBeforeCounting(t *testing.T) {
	policy := NewPasswordPolicy(
		allowAllPasswordsBlocklist(),
	)

	password := strings.Repeat(
		"e\u0301",
		passwordMaxCharacters,
	)

	expected := strings.Repeat(
		"é",
		passwordMaxCharacters,
	)

	got, err := policy.NormalizeAndValidate(
		context.Background(),
		password,
		"person@example.com",
	)
	if err != nil {
		t.Fatalf(
			"NormalizeAndValidate() error = %v",
			err,
		)
	}

	if got != expected {
		t.Fatalf(
			"NormalizeAndValidate() did not return NFC-normalized password",
		)
	}
}

func TestPasswordPolicyRejectsTooShortAfterNormalization(
	t *testing.T,
) {
	policy := NewPasswordPolicy(
		allowAllPasswordsBlocklist(),
	)

	password := strings.Repeat(
		"e\u0301",
		passwordMinCharacters-1,
	)

	_, err := policy.NormalizeAndValidate(
		context.Background(),
		password,
		"person@example.com",
	)
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf(
			"NormalizeAndValidate() error = %v, want ErrPasswordTooShort",
			err,
		)
	}
}

func TestPasswordPolicyRejectsInvalidUTF8(t *testing.T) {
	policy := NewPasswordPolicy(
		allowAllPasswordsBlocklist(),
	)

	password := string([]byte{
		0xff,
		0xfe,
	})

	_, err := policy.NormalizeAndValidate(
		context.Background(),
		password,
		"person@example.com",
	)
	if !errors.Is(err, ErrPasswordInvalidUTF8) {
		t.Fatalf(
			"NormalizeAndValidate() error = %v, want ErrPasswordInvalidUTF8",
			err,
		)
	}
}

func TestPasswordPolicyRejectsBlockedPassword(t *testing.T) {
	policy := NewPasswordPolicy(
		passwordBlocklistStub{
			isBlocked: func(
				_ context.Context,
				password string,
				email string,
			) (bool, error) {
				if password != "this password is blocked" {
					t.Fatalf(
						"password = %q, want blocked password",
						password,
					)
				}

				if email != "person@example.com" {
					t.Fatalf(
						"email = %q, want %q",
						email,
						"person@example.com",
					)
				}

				return true, nil
			},
		},
	)

	_, err := policy.NormalizeAndValidate(
		context.Background(),
		"this password is blocked",
		"person@example.com",
	)
	if !errors.Is(err, ErrPasswordBlocked) {
		t.Fatalf(
			"NormalizeAndValidate() error = %v, want ErrPasswordBlocked",
			err,
		)
	}
}

func TestPasswordPolicyReturnsBlocklistError(t *testing.T) {
	blocklistErr := errors.New("blocklist unavailable")

	policy := NewPasswordPolicy(
		passwordBlocklistStub{
			isBlocked: func(
				context.Context,
				string,
				string,
			) (bool, error) {
				return false, blocklistErr
			},
		},
	)

	_, err := policy.NormalizeAndValidate(
		context.Background(),
		"valid password phrase",
		"person@example.com",
	)
	if !errors.Is(err, blocklistErr) {
		t.Fatalf(
			"NormalizeAndValidate() error = %v, want blocklist error",
			err,
		)
	}
}
