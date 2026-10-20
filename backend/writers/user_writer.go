package writers

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"tripledger/models"
)

type UserWriter struct {
	db *sql.DB
}

func NewUserWriter(db *sql.DB) *UserWriter {
	return &UserWriter{db: db}
}

func (w *UserWriter) CreateUser(ctx context.Context, email, name, passwordHash string) (*models.User, error) {
	query := `INSERT INTO user_details (name, email, password_hash) VALUES (?, ?, ?)`
	res, err := w.db.ExecContext(ctx, query, name, email, passwordHash)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return &models.User{
		ID:           id,
		Email:        email,
		Name:         name,
		PasswordHash: passwordHash,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}, nil
}

func (w *UserWriter) UpdatePassword(ctx context.Context, userID int64, passwordHash string) error {
	query := `UPDATE user_details SET password_hash = ?, updated_at = NOW() WHERE id = ?`
	_, err := w.db.ExecContext(ctx, query, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("failed to update user password: %w", err)
	}
	return nil
}

func (w *UserWriter) UpdateProfile(ctx context.Context, userID int64, name string, phone *string) error {
	query := `UPDATE user_details SET name = ?, phone = ?, updated_at = NOW() WHERE id = ?`
	_, err := w.db.ExecContext(ctx, query, name, phone, userID)
	if err != nil {
		return fmt.Errorf("failed to update user profile: %w", err)
	}
	return nil
}
