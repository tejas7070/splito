package BLs

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"tripledger/models"
)

var (
	ErrOverlappingTrip   = errors.New("cannot join trip: overlapping active trip exists during this timeframe")
	ErrSelfApproval      = errors.New("forbidden: users cannot approve or reject their own claims")
	ErrUnauthorizedMunim = errors.New("forbidden: only Munimji can perform this action")
	ErrInvalidStatus     = errors.New("invalid status transition")
)

// CheckTripDateOverlap checks if a user has any conflicting active trips using strict date boundaries:
// user_details row is locked using SELECT FOR UPDATE to serialize checks
func CheckTripDateOverlap(ctx context.Context, tx *sql.Tx, userID int64, newStart, newEnd time.Time) error {
	var lockedUserID int64
	err := tx.QueryRowContext(ctx, "SELECT id FROM user_details WHERE id = ? FOR UPDATE", userID).Scan(&lockedUserID)
	if err != nil {
		return fmt.Errorf("failed to acquire user lock: %w", err)
	}

	query := `
		SELECT t.id, t.name, t.start_date, t.end_date
		FROM trip_members tm
		JOIN trips t ON tm.trip_id = t.id
		WHERE tm.user_id = ?
		  AND tm.status = 'active'
		  AND t.status IN ('planned', 'active')
		  AND t.start_date <= ?
		  AND t.end_date >= ?
		LIMIT 1
	`

	var conflictID int64
	var name string
	var start, end time.Time

	err = tx.QueryRowContext(ctx, query, userID, newEnd, newStart).Scan(&conflictID, &name, &start, &end)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // No conflict
	}
	if err != nil {
		return fmt.Errorf("error querying overlapping trips: %w", err)
	}

	return fmt.Errorf("%w: conflicts with trip '%s' (%s to %s)", ErrOverlappingTrip, name, start.Format("2006-01-02"), end.Format("2006-01-02"))
}

// ValidateClaimDecision verifies permissions for approving/rejecting a claim:
// - Claimant cannot decide their own claim (even if Munimji)
// - Munimji and Co-Reviewers can decide claims
func ValidateClaimDecision(claim *models.ExpenseClaim, reviewerID int64, reviewerRole models.Role) error {
	if claim.ClaimantID == reviewerID {
		return ErrSelfApproval
	}

	if reviewerRole != models.RoleMunimji && reviewerRole != models.RoleCoReviewer {
		return ErrUnauthorizedMunim
	}

	if claim.Status != models.ClaimStatusPending && claim.Status != models.ClaimStatusClarificationRequested {
		return fmt.Errorf("%w: cannot transition claim in status %s", ErrInvalidStatus, claim.Status)
	}

	return nil
}
