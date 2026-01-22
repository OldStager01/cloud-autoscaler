package queries

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID           int
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	query := `SELECT id, username, password_hash, created_at FROM users WHERE username = $1`

	var user User
	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int) (*User, error) {
	query := `SELECT id, username, password_hash, created_at FROM users WHERE id = $1`

	var user User
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) Create(ctx context.Context, username, passwordHash string) (*User, error) {
	query := `INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id, created_at`

	var user User
	user.Username = username
	user.PasswordHash = passwordHash

	err := r.db.QueryRowContext(ctx, query, username, passwordHash).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &user, nil
}