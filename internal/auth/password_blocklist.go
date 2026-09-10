package auth

import (
	"context"
	"strings"
)

var baselineWeakPasswordTerms = []string{
	"password",
	"qwerty",
	"qwertyuiop",
	"123456",
	"12345678",
	"123456789",
	"letmein",
	"welcome",
	"admin",
	"administrator",
	"iloveyou",
	"correct horse battery staple",
}

var weakPasswordSuffixes = []string{
	"",
	"1",
	"12",
	"123",
	"1234",
	"123456",
	"!",
	"password",
}

type LocalPasswordBlocklist struct {
	serviceTerms []string
}

func NewLocalPasswordBlocklist(
	serviceTerms ...string,
) *LocalPasswordBlocklist {
	normalizedTerms := make(
		[]string,
		0,
		len(serviceTerms),
	)

	for _, term := range serviceTerms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}

		normalizedTerms = append(
			normalizedTerms,
			term,
		)
	}

	return &LocalPasswordBlocklist{
		serviceTerms: normalizedTerms,
	}
}

func (b *LocalPasswordBlocklist) IsBlocked(
	_ context.Context,
	password string,
	email string,
) (bool, error) {
	for _, term := range baselineWeakPasswordTerms {
		if matchesWeakPasswordConstruction(
			password,
			term,
		) {
			return true, nil
		}
	}

	for _, term := range b.serviceTerms {
		if matchesWeakPasswordConstruction(
			password,
			term,
		) {
			return true, nil
		}
	}

	for _, term := range passwordEmailTerms(email) {
		if matchesWeakPasswordConstruction(
			password,
			term,
		) {
			return true, nil
		}
	}

	return false, nil
}

func matchesWeakPasswordConstruction(
	password string,
	term string,
) bool {
	if term == "" {
		return false
	}

	if strings.EqualFold(
		password,
		term+term,
	) {
		return true
	}

	if strings.EqualFold(
		password,
		"password"+term,
	) {
		return true
	}

	for _, suffix := range weakPasswordSuffixes {
		if strings.EqualFold(
			password,
			term+suffix,
		) {
			return true
		}
	}

	return false
}

func passwordEmailTerms(
	email string,
) []string {
	email = strings.TrimSpace(email)
	if email == "" {
		return nil
	}

	terms := []string{
		email,
	}

	localPart, _, found := strings.Cut(
		email,
		"@",
	)
	if found && localPart != "" {
		terms = append(
			terms,
			localPart,
		)
	}

	return terms
}

var _ PasswordBlocklist = (*LocalPasswordBlocklist)(nil)
