package auth

import (
	"context"
	"errors"
	"fmt"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	passwordMinCharacters = 15
	passwordMaxCharacters = 128
)

var (
	ErrPasswordInvalidUTF8 = errors.New(
		"password contains invalid UTF-8",
	)
	ErrPasswordTooShort = errors.New(
		"password must contain at least 15 characters",
	)
	ErrPasswordTooLong = errors.New(
		"password must contain at most 128 characters",
	)
	ErrPasswordBlocked = errors.New(
		"password is too common, expected, or compromised",
	)
)

type PasswordBlocklist interface {
	IsBlocked(
		ctx context.Context,
		password string,
		email string,
	) (bool, error)
}

type PasswordPolicy struct {
	blocklist PasswordBlocklist
}

func NewPasswordPolicy(
	blocklist PasswordBlocklist,
) *PasswordPolicy {
	return &PasswordPolicy{
		blocklist: blocklist,
	}
}

func (p *PasswordPolicy) NormalizeAndValidate(
	ctx context.Context,
	password string,
	email string,
) (string, error) {
	if !utf8.ValidString(password) {
		return "", ErrPasswordInvalidUTF8
	}

	normalized := norm.NFC.String(password)

	characterCount := utf8.RuneCountInString(normalized)

	if characterCount < passwordMinCharacters {
		return "", ErrPasswordTooShort
	}

	if characterCount > passwordMaxCharacters {
		return "", ErrPasswordTooLong
	}

	blocked, err := p.blocklist.IsBlocked(
		ctx,
		normalized,
		email,
	)
	if err != nil {
		return "", fmt.Errorf(
			"check password blocklist: %w",
			err,
		)
	}

	if blocked {
		return "", ErrPasswordBlocked
	}

	return normalized, nil
}

var _ PasswordValidator = (*PasswordPolicy)(nil)
