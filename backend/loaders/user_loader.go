package loaders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"tripledger/models"
	"tripledger/types"
)

type UserLoader struct {
	db *sql.DB
}

func NewUserLoader(db *sql.DB) *UserLoader {
	return &UserLoader{db: db}
}

func (l *UserLoader) GetByID(ctx context.Context, id int64) (*models.User, error) {
	query := `SELECT id, name, email, password_hash, phone, created_at, updated_at FROM user_details WHERE id = ?`
	row := l.db.QueryRowContext(ctx, query, id)

	user := &models.User{}
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.Phone, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}
	return user, nil
}

func (l *UserLoader) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `SELECT id, name, email, password_hash, phone, created_at, updated_at FROM user_details WHERE email = ?`
	row := l.db.QueryRowContext(ctx, query, email)

	user := &models.User{}
	err := row.Scan(&user.ID, &user.Name, &user.Email, &user.PasswordHash, &user.Phone, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return user, nil
}

func (l *UserLoader) ListEligibleUsersForTrip(ctx context.Context, tripID int64, query string) ([]types.EligibleUserResponse, error) {
	trimmedQuery := strings.TrimSpace(query)
	searchPattern := fmt.Sprintf("%%%s%%", trimmedQuery)

	sqlQuery := `
		SELECT ud.id, ud.name, ud.email
		FROM user_details ud
		WHERE ud.id NOT IN (
			SELECT tm.user_id 
			FROM trip_members tm 
			WHERE tm.trip_id = ? AND tm.status = 'active'
		)
		AND (? = '' OR ud.name LIKE ? OR ud.email LIKE ?)
		ORDER BY ud.name ASC, ud.email ASC
		LIMIT 50
	`

	rows, err := l.db.QueryContext(ctx, sqlQuery, tripID, trimmedQuery, searchPattern, searchPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to query eligible users: %w", err)
	}
	defer rows.Close()

	users := make([]types.EligibleUserResponse, 0)
	for rows.Next() {
		var u types.EligibleUserResponse
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			return nil, fmt.Errorf("failed to scan eligible user: %w", err)
		}
		users = append(users, u)
	}
	return users, nil
}

