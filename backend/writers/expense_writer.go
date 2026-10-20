package writers

import (
	"context"
	"database/sql"
	"fmt"

	"tripledger/models"
)

type ExpenseWriter struct {
	db *sql.DB
}

func NewExpenseWriter(db *sql.DB) *ExpenseWriter {
	return &ExpenseWriter{db: db}
}

func (w *ExpenseWriter) CreateClaimTx(ctx context.Context, tx *sql.Tx, claim *models.ExpenseClaim) (int64, error) {
	if claim.ClientUUID != nil && *claim.ClientUUID != "" {
		// Idempotency check: if already inserted with same client_uuid, return existing ID
		var existingID int64
		err := tx.QueryRowContext(ctx, "SELECT id FROM expense_claims WHERE client_uuid = ?", *claim.ClientUUID).Scan(&existingID)
		if err == nil && existingID > 0 {
			return existingID, nil
		}
	}

	query := `
		INSERT INTO expense_claims (trip_id, claimant_id, title, amount, category, reason, receipt_path, status, reimbursement_status, client_uuid)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'pending', 'unpaid', ?)
	`
	res, err := tx.ExecContext(ctx, query,
		claim.TripID, claim.ClaimantID, claim.Title, claim.Amount,
		claim.Category, claim.Reason, claim.ReceiptPath, claim.ClientUUID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert claim: %w", err)
	}
	return res.LastInsertId()
}

func (w *ExpenseWriter) UpdateClaimDecisionAtomicTx(ctx context.Context, tx *sql.Tx, claimID int64, tripID int64, status models.ClaimStatus, reviewerID int64, reason string) (int64, error) {
	query := `
		UPDATE expense_claims 
		SET status = ?, reviewed_by = ?, rejection_reason = ?, reviewed_at = NOW()
		WHERE id = ? AND trip_id = ? AND status = 'pending' AND claimant_id <> ?
	`
	res, err := tx.ExecContext(ctx, query, status, reviewerID, reason, claimID, tripID, reviewerID)
	if err != nil {
		return 0, fmt.Errorf("failed to update claim decision: %w", err)
	}
	return res.RowsAffected()
}

func (w *ExpenseWriter) UpdateClaimDecisionTx(ctx context.Context, tx *sql.Tx, claimID int64, status models.ClaimStatus, reviewerID int64, reason string) error {
	query := `
		UPDATE expense_claims 
		SET status = ?, reviewed_by = ?, rejection_reason = ?, reviewed_at = NOW()
		WHERE id = ?
	`
	_, err := tx.ExecContext(ctx, query, status, reviewerID, reason, claimID)
	if err != nil {
		return fmt.Errorf("failed to update claim decision: %w", err)
	}
	return nil
}

func (w *ExpenseWriter) RecordContributionTx(ctx context.Context, tx *sql.Tx, c *models.Contribution) (int64, error) {
	if c.ClientUUID != nil && *c.ClientUUID != "" {
		// Idempotency check: if already inserted with same client_uuid, return existing ID
		var existingID int64
		err := tx.QueryRowContext(ctx, "SELECT id FROM contributions WHERE client_uuid = ?", *c.ClientUUID).Scan(&existingID)
		if err == nil && existingID > 0 {
			return existingID, nil
		}
	}

	query := `
		INSERT INTO contributions (trip_id, user_id, amount, received_at, payment_method, notes, recorded_by, client_uuid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := tx.ExecContext(ctx, query,
		c.TripID, c.UserID, c.Amount, c.ReceivedAt, c.PaymentMethod, c.Notes, c.RecordedBy, c.ClientUUID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert contribution: %w", err)
	}
	return res.LastInsertId()
}

func (w *ExpenseWriter) RecordExpenseTx(ctx context.Context, tx *sql.Tx, e *models.Expense) (int64, error) {
	if e.ClientUUID != nil && *e.ClientUUID != "" {
		// Idempotency check: if already inserted with same client_uuid, return existing ID
		var existingID int64
		err := tx.QueryRowContext(ctx, "SELECT id FROM expenses WHERE client_uuid = ?", *e.ClientUUID).Scan(&existingID)
		if err == nil && existingID > 0 {
			return existingID, nil
		}
	}

	query := `
		INSERT INTO expenses (trip_id, title, category, amount, paid_from, paid_by, recorded_by, expense_date, notes, receipt_path, client_uuid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	res, err := tx.ExecContext(ctx, query,
		e.TripID, e.Title, e.Category, e.Amount, e.PaidFrom, e.PaidBy, e.RecordedBy, e.ExpenseDate, e.Notes, e.ReceiptPath, e.ClientUUID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert expense: %w", err)
	}
	return res.LastInsertId()
}
