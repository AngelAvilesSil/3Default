package config

import (
	"errors"
	"os"
)

type Config struct {
	DatabaseURL string
	StorageRoot string
}

func Load() (Config, error) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}

	storageRoot := os.Getenv("STORAGE_ROOT")
	if storageRoot == "" {
		return Config{}, errors.New("STORAGE_ROOT is required")
	}

	return Config{
		DatabaseURL: databaseURL,
		StorageRoot: storageRoot,
	}, nil
}
