package loaders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"tripledger/models"
)

type TripLoader struct {
	db *sql.DB
}

func NewTripLoader(db *sql.DB) *TripLoader {
	return &TripLoader{db: db}
}

func (l *TripLoader) GetByID(ctx context.Context, id int64) (*models.Trip, error) {
	query := `
		SELECT id, name, start_location, start_latitude, start_longitude,
		       destination, destination_latitude, destination_longitude,
		       start_date, end_date, estimated_budget, created_by, status, created_at, updated_at
		FROM trips WHERE id = ?
	`
	row := l.db.QueryRowContext(ctx, query, id)

	trip := &models.Trip{}
	err := row.Scan(
		&trip.ID, &trip.Name, &trip.StartLocation, &trip.StartLatitude, &trip.StartLongitude,
		&trip.Destination, &trip.DestLatitude, &trip.DestLongitude,
		&trip.StartDate, &trip.EndDate, &trip.EstimatedBudget,
		&trip.CreatedBy, &trip.Status, &trip.CreatedAt, &trip.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get trip: %w", err)
	}
	trip.Title = trip.Name
	trip.StartPoint = trip.StartLocation
	trip.StartAt = trip.StartDate
	trip.EndAt = trip.EndDate
	stops, err := l.ListTripStops(ctx, id)
	if err != nil {
		return nil, err
	}
	trip.Stops = stops
	return trip, nil
}

func (l *TripLoader) ListTripStops(ctx context.Context, tripID int64) ([]models.TripStop, error) {
	rows, err := l.db.QueryContext(ctx, `
		SELECT id, trip_id, position, name, latitude, longitude
		FROM trip_stops
		WHERE trip_id = ?
		ORDER BY position ASC
	`, tripID)
	if err != nil {
		return nil, fmt.Errorf("failed to list trip stops: %w", err)
	}
	defer rows.Close()

	stops := make([]models.TripStop, 0)
	for rows.Next() {
		var stop models.TripStop
		if err := rows.Scan(&stop.ID, &stop.TripID, &stop.Position, &stop.Name, &stop.Latitude, &stop.Longitude); err != nil {
			return nil, fmt.Errorf("failed to scan trip stop: %w", err)
		}
		stops = append(stops, stop)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read trip stops: %w", err)
	}
	return stops, nil
}

func (l *TripLoader) ListUserTrips(ctx context.Context, userID int64, email string) ([]models.Trip, error) {
	query := `
		SELECT t.id, t.name, t.start_location, t.destination, t.start_date, t.end_date,
		       t.estimated_budget, t.created_by, t.status, t.created_at, t.updated_at,
		       COALESCE(tm.role, ti.role, 'participant') AS user_role,
		       COALESCE(tm.status, IF(ti.status = 'pending', 'invited', ti.status), 'invited') AS user_status
		FROM trips t
		LEFT JOIN trip_members tm ON tm.trip_id = t.id AND tm.user_id = ? AND tm.status = 'active'
		LEFT JOIN trip_invitations ti ON ti.trip_id = t.id AND LOWER(ti.invitee_email) = LOWER(?) AND ti.status = 'pending'
		WHERE tm.user_id IS NOT NULL OR ti.id IS NOT NULL
		GROUP BY t.id, t.name, t.start_location, t.destination, t.start_date, t.end_date,
		         t.estimated_budget, t.created_by, t.status, t.created_at, t.updated_at,
		         COALESCE(tm.role, ti.role, 'participant'),
		         COALESCE(tm.status, IF(ti.status = 'pending', 'invited', ti.status), 'invited')
		ORDER BY t.created_at DESC
	`
	rows, err := l.db.QueryContext(ctx, query, userID, email)
	if err != nil {
		return nil, fmt.Errorf("failed to list user trips: %w", err)
	}
	defer rows.Close()

	var trips []models.Trip
	for rows.Next() {
		var t models.Trip
		err := rows.Scan(
			&t.ID, &t.Name, &t.StartLocation, &t.Destination,
			&t.StartDate, &t.EndDate, &t.EstimatedBudget,
			&t.CreatedBy, &t.Status, &t.CreatedAt, &t.UpdatedAt,
			&t.UserRole, &t.UserStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan trip: %w", err)
		}
		t.Title = t.Name
		t.StartPoint = t.StartLocation
		t.StartAt = t.StartDate
		t.EndAt = t.EndDate
		trips = append(trips, t)
	}
	return trips, nil
}

func (l *TripLoader) GetUserMembership(ctx context.Context, tripID, userID int64) (*models.TripMember, error) {
	query := `
		SELECT id, trip_id, user_id, role, status, joined_at, created_at
		FROM trip_members
		WHERE trip_id = ? AND user_id = ?
	`
	row := l.db.QueryRowContext(ctx, query, tripID, userID)

	m := &models.TripMember{}
	err := row.Scan(&m.ID, &m.TripID, &m.UserID, &m.Role, &m.Status, &m.JoinedAt, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get membership: %w", err)
	}
	return m, nil
}

func (l *TripLoader) ListTripMembers(ctx context.Context, tripID int64) ([]models.TripMember, error) {
	query := `
		SELECT tm.id, tm.trip_id, tm.user_id, COALESCE(u.name, 'Member'), COALESCE(u.email, ''),
		       tm.role, tm.status, tm.joined_at, tm.created_at
		FROM trip_members tm
		LEFT JOIN user_details u ON u.id = tm.user_id
		WHERE tm.trip_id = ? AND tm.status = 'active'
		ORDER BY tm.created_at ASC
	`
	rows, err := l.db.QueryContext(ctx, query, tripID)
	if err != nil {
		return nil, fmt.Errorf("failed to list trip members: %w", err)
	}
	defer rows.Close()

	var members []models.TripMember
	for rows.Next() {
		var m models.TripMember
		if err := rows.Scan(&m.ID, &m.TripID, &m.UserID, &m.UserName, &m.Email, &m.Role, &m.Status, &m.JoinedAt, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan member: %w", err)
		}
		members = append(members, m)
	}
	return members, nil
}

func (l *TripLoader) ListTripInvitations(ctx context.Context, tripID int64) ([]models.TripInvitation, error) {
	query := `
		SELECT ti.id, ti.trip_id, ti.inviter_user_id, COALESCE(u.name, 'Munimji'),
		       ti.invitee_email, ti.role, ti.token, ti.status, ti.expires_at, ti.responded_at, ti.created_at, ti.updated_at
		FROM trip_invitations ti
		LEFT JOIN user_details u ON u.id = ti.inviter_user_id
		WHERE ti.trip_id = ?
		  AND ti.invitee_email NOT IN (
		      SELECT ud.email 
		      FROM trip_members tm 
		      JOIN user_details ud ON ud.id = tm.user_id 
		      WHERE tm.trip_id = ti.trip_id AND tm.status = 'removed'
		  )
		ORDER BY ti.created_at DESC
	`
	rows, err := l.db.QueryContext(ctx, query, tripID)
	if err != nil {
		return nil, fmt.Errorf("failed to list trip invitations: %w", err)
	}
	defer rows.Close()

	var invitations []models.TripInvitation
	for rows.Next() {
		var inv models.TripInvitation
		var expiresAt, respondedAt sql.NullTime
		if err := rows.Scan(
			&inv.ID, &inv.TripID, &inv.InviterUserID, &inv.InviterName,
			&inv.InviteeEmail, &inv.Role, &inv.Token, &inv.Status,
			&expiresAt, &respondedAt, &inv.CreatedAt, &inv.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan trip invitation: %w", err)
		}
		if expiresAt.Valid {
			inv.ExpiresAt = &expiresAt.Time
		}
		if respondedAt.Valid {
			inv.RespondedAt = &respondedAt.Time
		}
		invitations = append(invitations, inv)
	}
	return invitations, nil
}

func (l *TripLoader) ListUserPendingInvitations(ctx context.Context, email string) ([]models.TripInvitation, error) {
	query := `
		SELECT ti.id, ti.trip_id, COALESCE(t.name, 'Trip'), COALESCE(t.start_location, ''), COALESCE(t.destination, ''),
		       ti.inviter_user_id, COALESCE(u.name, 'Munimji'),
		       ti.invitee_email, ti.role, ti.token, ti.status, ti.expires_at, ti.responded_at, ti.created_at, ti.updated_at
		FROM trip_invitations ti
		JOIN trips t ON t.id = ti.trip_id
		LEFT JOIN user_details u ON u.id = ti.inviter_user_id
		WHERE LOWER(ti.invitee_email) = LOWER(?)
		  AND ti.status = 'pending'
		  AND (ti.expires_at IS NULL OR ti.expires_at > NOW())
		ORDER BY ti.created_at DESC
	`
	rows, err := l.db.QueryContext(ctx, query, email)
	if err != nil {
		return nil, fmt.Errorf("failed to list user pending invitations: %w", err)
	}
	defer rows.Close()

	var invitations []models.TripInvitation
	for rows.Next() {
		var inv models.TripInvitation
		var expiresAt, respondedAt sql.NullTime
		if err := rows.Scan(
			&inv.ID, &inv.TripID, &inv.TripTitle, &inv.StartLocation, &inv.Destination,
			&inv.InviterUserID, &inv.InviterName,
			&inv.InviteeEmail, &inv.Role, &inv.Token, &inv.Status,
			&expiresAt, &respondedAt, &inv.CreatedAt, &inv.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan pending invitation: %w", err)
		}
		if expiresAt.Valid {
			inv.ExpiresAt = &expiresAt.Time
		}
		if respondedAt.Valid {
			inv.RespondedAt = &respondedAt.Time
		}
		invitations = append(invitations, inv)
	}
	return invitations, nil
}
