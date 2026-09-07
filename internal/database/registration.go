package database

import (
	"context"
	"fmt"

	"github.com/AngelAvilesSil/3Default/internal/database/dbgen"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RegistrationStore struct {
	pool *pgxpool.Pool
}

func NewRegistrationStore(
	pool *pgxpool.Pool,
) *RegistrationStore {
	return &RegistrationStore{
		pool: pool,
	}
}

func (s *RegistrationStore) CreateUserWithPassword(
	ctx context.Context,
	userParams dbgen.CreateUserParams,
	passwordHash string,
) (dbgen.User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return dbgen.User{}, fmt.Errorf(
			"begin registration transaction: %w",
			err,
		)
	}

	defer func() {
		_ = tx.Rollback(context.Background())
	}()

	queries := dbgen.New(tx)

	user, err := queries.CreateUser(
		ctx,
		userParams,
	)
	if err != nil {
		return dbgen.User{}, fmt.Errorf(
			"create registration user: %w",
			err,
		)
	}

	_, err = queries.CreatePasswordCredential(
		ctx,
		dbgen.CreatePasswordCredentialParams{
			UserID:       user.ID,
			PasswordHash: passwordHash,
		},
	)
	if err != nil {
		return dbgen.User{}, fmt.Errorf(
			"create registration password credential: %w",
			err,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return dbgen.User{}, fmt.Errorf(
			"commit registration transaction: %w",
			err,
		)
	}

	return user, nil
}
