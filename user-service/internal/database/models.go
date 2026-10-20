package database

import "time"

type UserRole string

type User struct {
	ID         int64    `json:"id" db:"id"`
	Role       UserRole `json:"role" db:"role"`
	FirstName  string   `json:"first_name" db:"first_name" validate:"required,max=100"`
	LastName   string   `json:"last_name" db:"last_name" validate:"required,max=100"`
	TelegramId int64    `json:"telegram_id" db:"telegram_id" validate:"required,max=16"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
