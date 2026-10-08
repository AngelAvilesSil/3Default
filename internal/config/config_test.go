package config

import (
	"strings"
	"testing"
)

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("STORAGE_ROOT", "/var/lib/3default/storage")

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing DATABASE_URL to fail")
	}

	if !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf(
			"expected DATABASE_URL error, got %v",
			err,
		)
	}
}

func TestLoadRequiresStorageRoot(t *testing.T) {
	t.Setenv(
		"DATABASE_URL",
		"postgres://example.invalid/database",
	)
	t.Setenv("STORAGE_ROOT", "")

	_, err := Load()
	if err == nil {
		t.Fatal("expected missing STORAGE_ROOT to fail")
	}

	if !strings.Contains(err.Error(), "STORAGE_ROOT") {
		t.Fatalf(
			"expected STORAGE_ROOT error, got %v",
			err,
		)
	}
}

func TestLoadReturnsConfiguredValues(t *testing.T) {
	databaseURL := "postgres://example.invalid/database"
	storageRoot := "/var/lib/3default/storage"

	t.Setenv("DATABASE_URL", databaseURL)
	t.Setenv("STORAGE_ROOT", storageRoot)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.DatabaseURL != databaseURL {
		t.Fatalf(
			"expected database URL %q, got %q",
			databaseURL,
			cfg.DatabaseURL,
		)
	}

	if cfg.StorageRoot != storageRoot {
		t.Fatalf(
			"expected storage root %q, got %q",
			storageRoot,
			cfg.StorageRoot,
		)
	}
}

func TestLoadReturnsOptionalMayoExecutable(t *testing.T) {
	t.Setenv(
		"DATABASE_URL",
		"postgres://example.invalid/database",
	)
	t.Setenv("STORAGE_ROOT", "/tmp/3default-storage")
	t.Setenv("MAYO_EXECUTABLE", "/opt/mayo/AppRun")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.MayoExecutable != "/opt/mayo/AppRun" {
		t.Fatalf(
			"unexpected Mayo executable: %q",
			cfg.MayoExecutable,
		)
	}
}

func TestLoadAllowsMayoToBeUnconfigured(t *testing.T) {
	t.Setenv(
		"DATABASE_URL",
		"postgres://example.invalid/database",
	)
	t.Setenv("STORAGE_ROOT", "/tmp/3default-storage")
	t.Setenv("MAYO_EXECUTABLE", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.MayoExecutable != "" {
		t.Fatalf(
			"expected no Mayo executable, got %q",
			cfg.MayoExecutable,
		)
	}
}
