package auth

import (
	"context"
	"testing"
)

func TestLocalPasswordBlocklistAllowsUnrelatedPassword(
	t *testing.T,
) {
	blocklist := NewLocalPasswordBlocklist(
		"3default",
		"3default.com",
	)

	blocked, err := blocklist.IsBlocked(
		context.Background(),
		"cobalt river lantern meadow 72",
		"person@example.com",
	)
	if err != nil {
		t.Fatalf(
			"IsBlocked() error = %v",
			err,
		)
	}

	if blocked {
		t.Fatal(
			"IsBlocked() = true, want false",
		)
	}
}

func TestLocalPasswordBlocklistBlocksBaselinePassword(
	t *testing.T,
) {
	blocklist := NewLocalPasswordBlocklist()

	blocked, err := blocklist.IsBlocked(
		context.Background(),
		"PasswordPassword",
		"person@example.com",
	)
	if err != nil {
		t.Fatalf(
			"IsBlocked() error = %v",
			err,
		)
	}

	if !blocked {
		t.Fatal(
			"IsBlocked() = false, want true",
		)
	}
}

func TestLocalPasswordBlocklistBlocksBaselineSuffix(
	t *testing.T,
) {
	blocklist := NewLocalPasswordBlocklist()

	blocked, err := blocklist.IsBlocked(
		context.Background(),
		"administrator123456",
		"person@example.com",
	)
	if err != nil {
		t.Fatalf(
			"IsBlocked() error = %v",
			err,
		)
	}

	if !blocked {
		t.Fatal(
			"IsBlocked() = false, want true",
		)
	}
}

func TestLocalPasswordBlocklistBlocksServiceTerm(
	t *testing.T,
) {
	blocklist := NewLocalPasswordBlocklist(
		"3default",
		"3default.com",
	)

	blocked, err := blocklist.IsBlocked(
		context.Background(),
		"3DEFAULT3DEFAULT",
		"person@example.com",
	)
	if err != nil {
		t.Fatalf(
			"IsBlocked() error = %v",
			err,
		)
	}

	if !blocked {
		t.Fatal(
			"IsBlocked() = false, want true",
		)
	}
}

func TestLocalPasswordBlocklistBlocksEmail(
	t *testing.T,
) {
	blocklist := NewLocalPasswordBlocklist()

	blocked, err := blocklist.IsBlocked(
		context.Background(),
		"Person@Example.com",
		"person@example.com",
	)
	if err != nil {
		t.Fatalf(
			"IsBlocked() error = %v",
			err,
		)
	}

	if !blocked {
		t.Fatal(
			"IsBlocked() = false, want true",
		)
	}
}

func TestLocalPasswordBlocklistBlocksEmailLocalPartConstruction(
	t *testing.T,
) {
	blocklist := NewLocalPasswordBlocklist()

	blocked, err := blocklist.IsBlocked(
		context.Background(),
		"verylongusername123456",
		"verylongusername@example.com",
	)
	if err != nil {
		t.Fatalf(
			"IsBlocked() error = %v",
			err,
		)
	}

	if !blocked {
		t.Fatal(
			"IsBlocked() = false, want true",
		)
	}
}

func TestLocalPasswordBlocklistDoesNotMatchSubstring(
	t *testing.T,
) {
	blocklist := NewLocalPasswordBlocklist(
		"3default",
	)

	blocked, err := blocklist.IsBlocked(
		context.Background(),
		"my long 3default engineering passphrase",
		"person@example.com",
	)
	if err != nil {
		t.Fatalf(
			"IsBlocked() error = %v",
			err,
		)
	}

	if blocked {
		t.Fatal(
			"IsBlocked() = true, want false",
		)
	}
}

func TestLocalPasswordBlocklistDoesNotTrimPassword(
	t *testing.T,
) {
	blocklist := NewLocalPasswordBlocklist()

	blocked, err := blocklist.IsBlocked(
		context.Background(),
		" passwordpassword ",
		"person@example.com",
	)
	if err != nil {
		t.Fatalf(
			"IsBlocked() error = %v",
			err,
		)
	}

	if blocked {
		t.Fatal(
			"IsBlocked() = true, want false",
		)
	}
}

func TestLocalPasswordBlocklistIgnoresBlankServiceTerms(
	t *testing.T,
) {
	blocklist := NewLocalPasswordBlocklist(
		"",
		"   ",
	)

	blocked, err := blocklist.IsBlocked(
		context.Background(),
		"unrelated secure passphrase",
		"person@example.com",
	)
	if err != nil {
		t.Fatalf(
			"IsBlocked() error = %v",
			err,
		)
	}

	if blocked {
		t.Fatal(
			"IsBlocked() = true, want false",
		)
	}
}
