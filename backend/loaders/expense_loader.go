package loaders

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"tripledger/models"
)

type ExpenseLoader struct {
	db *sql.DB
}

func NewExpenseLoader(db *sql.DB) *ExpenseLoader {
	return &ExpenseLoader{db: db}
}

func (l *ExpenseLoader) GetClaimByID(ctx context.Context, claimID int64) (*models.ExpenseClaim, error) {
	query := `
		SELECT id, trip_id, claimant_id, title, amount, category, reason, receipt_path,
		       status, reimbursement_status, reviewed_by, reviewed_at, rejection_reason, created_at
		FROM expense_claims WHERE id = ?
	`
	row := l.db.QueryRowContext(ctx, query, claimID)

	claim := &models.ExpenseClaim{}
	err := row.Scan(
		&claim.ID, &claim.TripID, &claim.ClaimantID, &claim.Title, &claim.Amount,
		&claim.Category, &claim.Reason, &claim.ReceiptPath, &claim.Status,
		&claim.ReimbursementStatus, &claim.ReviewedBy, &claim.ReviewedAt,
		&claim.RejectionReason, &claim.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get claim: %w", err)
	}
	return claim, nil
}

func (l *ExpenseLoader) ListTripClaims(ctx context.Context, tripID int64) ([]models.ExpenseClaim, error) {
	query := `
		SELECT c.id, c.trip_id, c.claimant_id, COALESCE(u.name, 'Member'), c.title, c.amount,
		       c.category, c.reason, c.receipt_path, c.status, c.reimbursement_status,
		       c.reviewed_by, c.reviewed_at, c.rejection_reason, c.created_at
		FROM expense_claims c
		LEFT JOIN user_details u ON u.id = c.claimant_id
		WHERE c.trip_id = ?
		ORDER BY c.created_at DESC
	`
	rows, err := l.db.QueryContext(ctx, query, tripID)
	if err != nil {
		return nil, fmt.Errorf("failed to list claims: %w", err)
	}
	defer rows.Close()

	var claims []models.ExpenseClaim
	for rows.Next() {
		var c models.ExpenseClaim
		err := rows.Scan(
			&c.ID, &c.TripID, &c.ClaimantID, &c.ClaimantName, &c.Title, &c.Amount,
			&c.Category, &c.Reason, &c.ReceiptPath, &c.Status,
			&c.ReimbursementStatus, &c.ReviewedBy, &c.ReviewedAt,
			&c.RejectionReason, &c.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan claim: %w", err)
		}
		c.AmountPaise = int64(c.Amount * 100)
		claims = append(claims, c)
	}
	return claims, nil
}

func (l *ExpenseLoader) ListTripExpenses(ctx context.Context, tripID int64) ([]models.Expense, error) {
	query := `
		SELECT e.id, e.trip_id, e.title, e.category, e.amount, e.paid_from, e.paid_by,
		       COALESCE(u.name, 'Munimji'), e.recorded_by, e.expense_date, e.notes, e.receipt_path, e.created_at
		FROM expenses e
		LEFT JOIN user_details u ON u.id = e.paid_by
		WHERE e.trip_id = ?
		ORDER BY e.expense_date DESC
	`
	rows, err := l.db.QueryContext(ctx, query, tripID)
	if err != nil {
		return nil, fmt.Errorf("failed to list expenses: %w", err)
	}
	defer rows.Close()

	var expenses []models.Expense
	for rows.Next() {
		var e models.Expense
		err := rows.Scan(
			&e.ID, &e.TripID, &e.Title, &e.Category, &e.Amount, &e.PaidFrom, &e.PaidBy,
			&e.PaidByName, &e.RecordedBy, &e.ExpenseDate, &e.Notes, &e.ReceiptPath, &e.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan expense: %w", err)
		}
		e.AmountPaise = int64(e.Amount * 100)
		expenses = append(expenses, e)
	}
	return expenses, nil
}

func (l *ExpenseLoader) ListTripContributions(ctx context.Context, tripID int64) ([]models.Contribution, error) {
	query := `
		SELECT c.id, c.trip_id, c.user_id, COALESCE(u.name, 'Member'), c.amount,
		       c.received_at, c.payment_method, c.notes, c.recorded_by, c.created_at
		FROM contributions c
		LEFT JOIN user_details u ON u.id = c.user_id
		WHERE c.trip_id = ?
		ORDER BY c.received_at DESC
	`
	rows, err := l.db.QueryContext(ctx, query, tripID)
	if err != nil {
		return nil, fmt.Errorf("failed to list contributions: %w", err)
	}
	defer rows.Close()

	var contributions []models.Contribution
	for rows.Next() {
		var c models.Contribution
		err := rows.Scan(
			&c.ID, &c.TripID, &c.UserID, &c.UserName, &c.Amount, &c.ReceivedAt,
			&c.PaymentMethod, &c.Notes, &c.RecordedBy, &c.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan contribution: %w", err)
		}
		c.AmountPaise = int64(c.Amount * 100)
		contributions = append(contributions, c)
	}
	return contributions, nil
}

func (l *ExpenseLoader) GetTripFinancialSummary(ctx context.Context, tripID int64) (totalContributions float64, totalExpenses float64, approvedClaims float64, err error) {
	// Total contributions
	rowContrib := l.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0) FROM contributions WHERE trip_id = ?`, tripID)
	if err = rowContrib.Scan(&totalContributions); err != nil {
		return
	}

	// Total group spend
	rowExp := l.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE trip_id = ? AND paid_from = 'group_funds'`, tripID)
	if err = rowExp.Scan(&totalExpenses); err != nil {
		return
	}

	// Total approved claims
	rowClaim := l.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount), 0) FROM expense_claims WHERE trip_id = ? AND status = 'approved'`, tripID)
	if err = rowClaim.Scan(&approvedClaims); err != nil {
		return
	}

	return
}
