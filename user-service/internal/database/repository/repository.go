package repository

import (
	"errors"
	"fmt"

	"user-service/internal/database"

	"github.com/jackc/pgx/v5"
)

var (
	ErrNotFound = errors.New("user not found")
)

const (
	userColumns = `id, role, first_name, last_name, telegram_id, created_at`
)

type UserRepository struct {
	db *database.Cluster
}

func NewUserRepository(db *database.Cluster) *UserRepository {
	return &UserRepository{db: db}
}

func mapErr(op string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound
	default:
		return fmt.Errorf("%s: %w", op, err)
	}
}
