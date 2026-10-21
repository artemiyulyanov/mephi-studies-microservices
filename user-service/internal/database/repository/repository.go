package repository

import (
	"context"
	"errors"
	"fmt"

	"user-service/internal/database"

	"github.com/jackc/pgx/v5"

	"common/models"
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

func (repo *UserRepository) GetUserByID(ctx context.Context, id int64) (models.User, error) {
	user, err := database.Read(ctx, repo.db, func(q database.Querier) (models.User, error) {
		return scanUser(
			q.QueryRow(
				ctx,
				`SELECT `+userColumns+` FROM users WHERE id = $1`,
				id,
			),
		)
	})

	return user, err
}

func (repo *UserRepository) GetUserByIDFresh(ctx context.Context, id int64) (models.User, error) {
	user, err := scanUser(
		repo.db.Writer().QueryRow(
			ctx,
			`SELECT `+userColumns+` FROM users WHERE id = $1`,
			id,
		),
	)

	return user, err
}

func scanUser(row pgx.Row) (models.User, error) {
	var user models.User

	err := row.Scan(&user.ID, &user.Role, &user.FirstName, &user.LastName, &user.TelegramId, &user.CreatedAt)

	return user, err
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
