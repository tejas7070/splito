package writers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"tripledger/models"
)

type TripWriter struct {
	db *sql.DB
}

func NewTripWriter(db *sql.DB) *TripWriter {
	return &TripWriter{db: db}
}

func (w *TripWriter) CreateTripTx(ctx context.Context, tx *sql.Tx, trip *models.Trip) (int64, error) {
	query := `
		INSERT INTO trips (name, start_location, start_latitude, start_longitude, destination, destination_latitude, destination_longitude, start_date, end_date, estimated_budget, created_by, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := tx.ExecContext(ctx, query,
		trip.Name, trip.StartLocation, trip.StartLatitude, trip.StartLongitude,
		trip.Destination, trip.DestLatitude, trip.DestLongitude, trip.StartDate, trip.EndDate,
		trip.EstimatedBudget, trip.CreatedBy, trip.Status,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert trip: %w", err)
	}
	return res.LastInsertId()
}

func (w *TripWriter) SaveStopsTx(ctx context.Context, tx *sql.Tx, tripID int64, stops []models.TripStop) error {
	for position, stop := range stops {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO trip_stops (trip_id, position, name, latitude, longitude)
			VALUES (?, ?, ?, ?, ?)
		`, tripID, position, stop.Name, stop.Latitude, stop.Longitude)
		if err != nil {
			return fmt.Errorf("failed to save trip stop %d: %w", position+1, err)
		}
	}
	return nil
}

func (w *TripWriter) UpdateTripTx(ctx context.Context, tx *sql.Tx, trip *models.Trip, stops []models.TripStop) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE trips
		SET name = ?, start_location = ?, start_latitude = ?, start_longitude = ?,
		    destination = ?, destination_latitude = ?, destination_longitude = ?,
		    start_date = ?, end_date = ?, estimated_budget = ?
		WHERE id = ?
	`, trip.Name, trip.StartLocation, trip.StartLatitude, trip.StartLongitude,
		trip.Destination, trip.DestLatitude, trip.DestLongitude,
		trip.StartDate, trip.EndDate, trip.EstimatedBudget, trip.ID)
	if err != nil {
		return fmt.Errorf("failed to update trip: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM trip_stops WHERE trip_id = ?", trip.ID); err != nil {
		return fmt.Errorf("failed to replace existing trip stops: %w", err)
	}
	return w.SaveStopsTx(ctx, tx, trip.ID, stops)
}

func (w *TripWriter) AddMemberTx(ctx context.Context, tx *sql.Tx, tripID, userID int64, role models.Role, status models.MembershipStatus) error {
	var joinedAt any = time.Now()
	if status == models.MembershipInvited {
		joinedAt = nil
	}
	query := `
		INSERT INTO trip_members (trip_id, user_id, role, status, joined_at)
		VALUES (?, ?, ?, ?, ?)
	`
	_, err := tx.ExecContext(ctx, query, tripID, userID, role, status, joinedAt)
	if err != nil {
		return fmt.Errorf("failed to insert member: %w", err)
	}
	return nil
}

func (w *TripWriter) CreateInvitation(ctx context.Context, inv *models.TripInvitation) (int64, error) {
	query := `
		INSERT INTO trip_invitations (trip_id, inviter_user_id, invitee_email, role, token, status, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`
	res, err := w.db.ExecContext(ctx, query,
		inv.TripID, inv.InviterUserID, inv.InviteeEmail, inv.Role, inv.Token, inv.Status, inv.ExpiresAt,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert trip invitation: %w", err)
	}
	return res.LastInsertId()
}

func (w *TripWriter) RespondInvitation(ctx context.Context, tripID, userID int64, email, action string) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to start tx: %w", err)
	}
	defer tx.Rollback()

	// Check if there is a pending invitation for this trip & email
	var invitationID int64
	var role models.Role
	err = tx.QueryRowContext(ctx, `
		SELECT id, role FROM trip_invitations 
		WHERE trip_id = ? AND LOWER(invitee_email) = LOWER(?) AND status = 'pending'
		LIMIT 1 FOR UPDATE
	`, tripID, email).Scan(&invitationID, &role)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("no pending invitation found for your account on this expedition")
		}
		return fmt.Errorf("failed to query pending invitation: %w", err)
	}

	newStatus := models.InvitationRejected
	if action == "accept" {
		newStatus = models.InvitationAccepted
	}

	// Update invitation record
	_, err = tx.ExecContext(ctx, `
		UPDATE trip_invitations 
		SET status = ?, responded_at = NOW() 
		WHERE id = ?
	`, newStatus, invitationID)
	if err != nil {
		return fmt.Errorf("failed to update invitation: %w", err)
	}

	// If accepted, add user to trip_members with status 'active' and joined_at NOW()
	if action == "accept" {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO trip_members (trip_id, user_id, role, status, joined_at)
			VALUES (?, ?, ?, 'active', NOW())
			ON DUPLICATE KEY UPDATE status = 'active', role = VALUES(role), joined_at = NOW(), removal_reason = NULL, removed_by = NULL, removed_at = NULL
		`, tripID, userID, role)
		if err != nil {
			return fmt.Errorf("failed to add member to trip: %w", err)
		}
	}

	return tx.Commit()
}

func (w *TripWriter) UpdateMemberStatusTx(ctx context.Context, tx *sql.Tx, tripID, userID int64, status models.MembershipStatus) error {
	query := `
		UPDATE trip_members 
		SET status = ?
		WHERE trip_id = ? AND user_id = ?
	`
	_, err := tx.ExecContext(ctx, query, status, tripID, userID)
	if err != nil {
		return fmt.Errorf("failed to update member status: %w", err)
	}
	return nil
}

func (w *TripWriter) UpdateTripStatus(ctx context.Context, tripID int64, status models.TripStatus) error {
	query := `
		UPDATE trips 
		SET status = ?
		WHERE id = ?
	`
	_, err := w.db.ExecContext(ctx, query, status, tripID)
	if err != nil {
		return fmt.Errorf("failed to update trip status: %w", err)
	}
	return nil
}

func (w *TripWriter) RemoveMember(ctx context.Context, tripID, userID, removedBy int64, reason string) error {
	query := `
		UPDATE trip_members 
		SET status = 'removed',
		    removal_reason = ?,
		    removed_by = ?,
		    removed_at = NOW()
		WHERE trip_id = ? AND user_id = ? AND status = 'active'
	`
	res, err := w.db.ExecContext(ctx, query, reason, removedBy, tripID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove member from trip: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}
	if rows == 0 {
		return errors.New("no active member found to remove")
	}
	return nil
}

func (w *TripWriter) CancelInvitation(ctx context.Context, tripID int64, email string) error {
	query := `
		UPDATE trip_invitations 
		SET status = 'cancelled', responded_at = NOW() 
		WHERE trip_id = ? AND LOWER(invitee_email) = LOWER(?) AND status = 'pending'
	`
	_, err := w.db.ExecContext(ctx, query, tripID, email)
	return err
}

func (w *TripWriter) DeleteInvitationForEmail(ctx context.Context, tripID int64, email string) error {
	query := `
		DELETE FROM trip_invitations 
		WHERE trip_id = ? AND LOWER(invitee_email) = LOWER(?)
	`
	_, err := w.db.ExecContext(ctx, query, tripID, email)
	return err
}

func (w *TripWriter) UpdateMemberRole(ctx context.Context, tripID, userID int64, newRole models.Role) error {
	query := `
		UPDATE trip_members 
		SET role = ?
		WHERE trip_id = ? AND user_id = ? AND status = 'active'
	`
	res, err := w.db.ExecContext(ctx, query, newRole, tripID, userID)
	if err != nil {
		return fmt.Errorf("failed to update member role: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}
	if rows == 0 {
		return errors.New("no active member found to update role")
	}
	return nil
}

func (w *TripWriter) DeleteTrip(ctx context.Context, tripID int64) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin delete transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete claims and claim comments/receipts if any
	_, _ = tx.ExecContext(ctx, "DELETE FROM claim_audit_log WHERE claim_id IN (SELECT id FROM expense_claims WHERE trip_id = ?)", tripID)
	_, _ = tx.ExecContext(ctx, "DELETE FROM expense_claims WHERE trip_id = ?", tripID)

	// Delete expenses
	_, _ = tx.ExecContext(ctx, "DELETE FROM expenses WHERE trip_id = ?", tripID)

	// Delete contributions
	_, _ = tx.ExecContext(ctx, "DELETE FROM contributions WHERE trip_id = ?", tripID)

	// Delete invitations
	_, _ = tx.ExecContext(ctx, "DELETE FROM trip_invitations WHERE trip_id = ?", tripID)

	// Delete members
	_, _ = tx.ExecContext(ctx, "DELETE FROM trip_members WHERE trip_id = ?", tripID)

	// Delete trip
	res, err := tx.ExecContext(ctx, "DELETE FROM trips WHERE id = ?", tripID)
	if err != nil {
		return fmt.Errorf("failed to delete trip: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check affected rows: %w", err)
	}
	if rows == 0 {
		return errors.New("trip not found")
	}

	return tx.Commit()
}
