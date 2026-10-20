package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"tripledger/config"
	"tripledger/loaders"
	"tripledger/models"
	"tripledger/types"
	"tripledger/writers"
)

type LedgerService struct {
	db            *sql.DB
	cfg           *config.Config
	tripLoader    *loaders.TripLoader
	expenseLoader *loaders.ExpenseLoader
	expenseWriter *writers.ExpenseWriter
}

func NewLedgerService(
	db *sql.DB,
	cfg *config.Config,
	tl *loaders.TripLoader,
	el *loaders.ExpenseLoader,
	ew *writers.ExpenseWriter,
) *LedgerService {
	return &LedgerService{
		db:            db,
		cfg:           cfg,
		tripLoader:    tl,
		expenseLoader: el,
		expenseWriter: ew,
	}
}

// CreateClaim submits a peer out-of-pocket expense claim
func (s *LedgerService) CreateClaim(ctx context.Context, userID int64, req *types.CreateClaimRequest) (*models.ExpenseClaim, error) {
	membership, err := s.tripLoader.GetUserMembership(ctx, req.TripID, userID)
	if err != nil || membership == nil || membership.Status != models.MembershipActive {
		return nil, errors.New("must be an active member of the trip to submit claims")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	title := req.Title
	if title == "" {
		title = req.Category
	}

	amountVal := req.Amount
	if amountVal <= 0 && req.AmountPaise != nil && *req.AmountPaise > 0 {
		amountVal = float64(*req.AmountPaise) / 100.0
	}
	if amountVal <= 0 {
		return nil, errors.New("claim amount must be greater than zero")
	}

	claim := &models.ExpenseClaim{
		TripID:              req.TripID,
		ClaimantID:          userID,
		Title:               title,
		Amount:              amountVal,
		Category:            req.Category,
		Reason:              req.Reason,
		ReceiptPath:         req.ReceiptPath,
		ClientUUID:          req.ClientUUID,
		Status:              models.ClaimStatusPending,
		ReimbursementStatus: models.ReimbursementUnpaid,
	}

	claimID, err := s.expenseWriter.CreateClaimTx(ctx, tx, claim)
	if err != nil {
		return nil, err
	}
	claim.ID = claimID

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return claim, nil
}

var (
	ErrClaimConflict     = errors.New("claim already reviewed or not allowed")
	ErrRejectionReason   = errors.New("rejection reason must be between 3 and 500 characters")
	ErrTripCompletedNoOp = errors.New("cannot review claims on a completed trip")
	ErrNotReviewer       = errors.New("forbidden: only Munimji or Co-Reviewer can review claims")
)

// DecideClaim approves or rejects a peer claim inside an atomic transaction
func (s *LedgerService) DecideClaim(ctx context.Context, reviewerID int64, req *types.ClaimDecisionRequest, approve bool) error {
	trimmedReason := strings.TrimSpace(req.Reason)
	if !approve {
		if len(trimmedReason) < 3 || len(trimmedReason) > 500 {
			return ErrRejectionReason
		}
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Verify caller's role in this trip inside tx with FOR SHARE
	var role models.Role
	var membershipStatus models.MembershipStatus
	err = tx.QueryRowContext(ctx, `
		SELECT role, status 
		FROM trip_members 
		WHERE trip_id = ? AND user_id = ? 
		FOR SHARE
	`, req.TripID, reviewerID).Scan(&role, &membershipStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errors.New("reviewer is not a member of this trip")
		}
		return err
	}
	if membershipStatus != models.MembershipActive {
		return errors.New("reviewer is not an active member")
	}
	if role != models.RoleMunimji && role != models.RoleCoReviewer {
		return ErrNotReviewer
	}

	// 2. Reject the call if trip status is completed
	var tripStatus models.TripStatus
	err = tx.QueryRowContext(ctx, `
		SELECT status 
		FROM trips 
		WHERE id = ? 
		FOR SHARE
	`, req.TripID).Scan(&tripStatus)
	if err != nil {
		return err
	}
	if tripStatus == models.TripStatusCompleted {
		return ErrTripCompletedNoOp
	}

	// 3. Atomically update claim status
	newStatus := models.ClaimStatusApproved
	if !approve {
		newStatus = models.ClaimStatusRejected
	}

	rowsAffected, err := s.expenseWriter.UpdateClaimDecisionAtomicTx(ctx, tx, req.ClaimID, req.TripID, newStatus, reviewerID, trimmedReason)
	if err != nil {
		return err
	}
	if rowsAffected != 1 {
		return ErrClaimConflict
	}

	// 4. Commit transaction
	if err := tx.Commit(); err != nil {
		return err
	}

	// Load claim data for async notification
	claim, _ := s.expenseLoader.GetClaimByID(ctx, req.ClaimID)
	if claim != nil {
		s.sendClaimDecisionEmail(claim, req.TripID, reviewerID, role, approve, trimmedReason)
	}

	return nil
}

// RecordContribution registers an incoming cash or UPI pool contribution
func (s *LedgerService) RecordContribution(ctx context.Context, recorderID int64, req *types.CreateContributionRequest) (*models.Contribution, error) {
	membership, err := s.tripLoader.GetUserMembership(ctx, req.TripID, recorderID)
	if err != nil || membership == nil || membership.Role != models.RoleMunimji {
		return nil, errors.New("only Munimji can record contributions")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	contrib := &models.Contribution{
		TripID:        req.TripID,
		UserID:        req.UserID,
		Amount:        req.Amount,
		ReceivedAt:    req.ReceivedAt,
		PaymentMethod: req.PaymentMethod,
		Notes:         req.Notes,
		RecordedBy:    recorderID,
		ClientUUID:    req.ClientUUID,
	}

	id, err := s.expenseWriter.RecordContributionTx(ctx, tx, contrib)
	if err != nil {
		return nil, err
	}
	contrib.ID = id

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return contrib, nil
}

// RecordExpense registers common group spending
func (s *LedgerService) RecordExpense(ctx context.Context, recorderID int64, req *types.CreateExpenseRequest) (*models.Expense, error) {
	membership, err := s.tripLoader.GetUserMembership(ctx, req.TripID, recorderID)
	if err != nil || membership == nil || membership.Role != models.RoleMunimji {
		return nil, errors.New("only Munimji can record expenses")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	paidFrom := req.PaidFrom
	if paidFrom == "" {
		paidFrom = "group_funds"
	}

	expense := &models.Expense{
		TripID:      req.TripID,
		Title:       req.Title,
		Category:    req.Category,
		Amount:      req.Amount,
		PaidFrom:    paidFrom,
		PaidBy:      recorderID,
		RecordedBy:  recorderID,
		ExpenseDate: req.ExpenseDate,
		Notes:       req.Notes,
		ClientUUID:  req.ClientUUID,
	}

	id, err := s.expenseWriter.RecordExpenseTx(ctx, tx, expense)
	if err != nil {
		return nil, err
	}
	expense.ID = id

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return expense, nil
}

// GetDashboard calculates treasury metrics
func (s *LedgerService) GetDashboard(ctx context.Context, tripID int64) (map[string]interface{}, error) {
	contribs, expenses, claims, err := s.expenseLoader.GetTripFinancialSummary(ctx, tripID)
	if err != nil {
		return nil, err
	}

	availableCash := contribs - expenses

	return map[string]interface{}{
		"total_contributions_paise": int64(contribs * 100),
		"group_funded_spend_paise":  int64(expenses * 100),
		"available_cash_paise":      int64(availableCash * 100),
		"approved_claims_paise":     int64(claims * 100),
	}, nil
}

func (s *LedgerService) ListClaims(ctx context.Context, tripID int64) ([]models.ExpenseClaim, error) {
	return s.expenseLoader.ListTripClaims(ctx, tripID)
}

func (s *LedgerService) ListExpenses(ctx context.Context, tripID int64) ([]models.Expense, error) {
	return s.expenseLoader.ListTripExpenses(ctx, tripID)
}

func (s *LedgerService) ListContributions(ctx context.Context, tripID int64) ([]models.Contribution, error) {
	return s.expenseLoader.ListTripContributions(ctx, tripID)
}

func (s *LedgerService) sendClaimDecisionEmail(
	claim *models.ExpenseClaim,
	tripID int64,
	reviewerID int64,
	reviewerRole models.Role,
	approve bool,
	decisionReason string,
) {
	if s.cfg == nil || s.cfg.SMTPHost == "" || s.cfg.SMTPUser == "" {
		fmt.Printf("[CLAIM EMAIL SKIPPED] SMTP not configured. Decision: approve=%v, claimID=%d\n", approve, claim.ID)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var claimantEmail, claimantName string
		err := s.db.QueryRowContext(ctx, "SELECT email, name FROM user_details WHERE id = ?", claim.ClaimantID).Scan(&claimantEmail, &claimantName)
		if err != nil || claimantEmail == "" {
			fmt.Printf("[SPLITO CLAIM EMAIL] Could not find claimant email for user_id=%d: %v\n", claim.ClaimantID, err)
			return
		}

		var reviewerName string
		_ = s.db.QueryRowContext(ctx, "SELECT name FROM user_details WHERE id = ?", reviewerID).Scan(&reviewerName)
		if reviewerName == "" {
			reviewerName = "Munimji"
		}

		reviewerRoleFormatted := "Munimji"
		if reviewerRole == models.RoleCoReviewer {
			reviewerRoleFormatted = "Co-Reviewer"
		}

		var tripName, startLoc, dest string
		_ = s.db.QueryRowContext(ctx, "SELECT name, start_location, destination FROM trips WHERE id = ?", tripID).Scan(&tripName, &startLoc, &dest)
		if tripName == "" {
			tripName = "Expedition"
		}

		claimTitle := claim.Title
		if claimTitle == "" {
			claimTitle = claim.Category
		}

		var subject, badgeText, badgeBg, badgeColor, badgeBorder, headerTitle, explanationHtml string
		if approve {
			subject = fmt.Sprintf("✅ Claim Approved: ₹%.2f for \"%s\" (%s)", claim.Amount, claimTitle, tripName)
			badgeText = "CLAIM APPROVED"
			badgeBg = "#ecfdf5"
			badgeColor = "#059669"
			badgeBorder = "#a7f3d0"
			headerTitle = "Expense Claim Approved"
			explanationHtml = fmt.Sprintf(`
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#ecfdf5;border:1px solid #a7f3d0;border-radius:12px;padding:16px 20px;margin-bottom:24px;">
                <tr>
                  <td style="font-size:13px;color:#065f46;line-height:1.6;">
                    <strong style="color:#047857;font-size:14px;">✅ Group Treasury Approval:</strong><br>
                    This out-of-pocket expense is now validated as an approved liability of the group treasury pool. It will be accounted for in the trip ledger and reimbursed or offset during final expedition settlement.
                  </td>
                </tr>
              </table>`)

			if strings.TrimSpace(decisionReason) != "" && strings.TrimSpace(decisionReason) != "Approved by Munimji" {
				explanationHtml += fmt.Sprintf(`
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:12px;padding:14px 20px;margin-bottom:24px;">
                <tr>
                  <td>
                    <div style="font-size:11px;font-weight:700;color:#475569;text-transform:uppercase;letter-spacing:0.5px;">Reviewer Note</div>
                    <div style="font-size:14px;color:#1e293b;margin:4px 0 0 0;">%s</div>
                  </td>
                </tr>
              </table>`, strings.TrimSpace(decisionReason))
			}
		} else {
			subject = fmt.Sprintf("❌ Claim Declined: ₹%.2f for \"%s\" (%s)", claim.Amount, claimTitle, tripName)
			badgeText = "CLAIM DECLINED"
			badgeBg = "#fef2f2"
			badgeColor = "#dc2626"
			badgeBorder = "#fecaca"
			headerTitle = "Expense Claim Declined"

			reasonText := strings.TrimSpace(decisionReason)
			if reasonText == "" {
				reasonText = "No specific reason provided by reviewer."
			}

			explanationHtml = fmt.Sprintf(`
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#fef2f2;border:1px solid #fecaca;border-radius:12px;padding:18px 20px;margin-bottom:24px;">
                <tr>
                  <td>
                    <div style="font-size:11px;font-weight:700;color:#991b1b;text-transform:uppercase;letter-spacing:0.5px;">Reason for Rejection</div>
                    <div style="font-size:15px;font-weight:600;color:#7f1d1d;margin:6px 0 0 0;line-height:1.5;">%s</div>
                  </td>
                </tr>
              </table>
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:12px;padding:16px 20px;margin-bottom:24px;">
                <tr>
                  <td style="font-size:12px;color:#64748b;line-height:1.6;">
                    <strong style="color:#334155;">Treasury Protocol Note:</strong><br>
                    Because this claim was declined, it will not be reimbursed from the expedition treasury pool. It remains recorded as your personal out-of-pocket expense and will not be shared across group members.
                  </td>
                </tr>
              </table>`, reasonText)
		}

		htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>%s</title>
</head>
<body style="margin:0;padding:0;background-color:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#0f172a;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background-color:#f1f5f9;padding:40px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" style="max-width:540px;background-color:#ffffff;border-radius:16px;border:1px solid #e2e8f0;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.06);">
          <tr>
            <td style="background-color:#0f172a;padding:26px 32px;text-align:left;">
              <div style="display:inline-block;">
                <span style="color:#ffffff;font-size:24px;font-weight:800;letter-spacing:-0.5px;">Splito</span>
                <span style="color:%s;font-size:11px;margin-left:10px;font-weight:700;text-transform:uppercase;letter-spacing:1px;background:%s;border:1px solid %s;padding:3px 8px;border-radius:6px;">%s</span>
              </div>
              <p style="margin:6px 0 0 0;color:#94a3b8;font-size:12px;">Expedition Treasury Protocol • %s</p>
            </td>
          </tr>
          <tr>
            <td style="padding:32px 32px 24px 32px;">
              <h1 style="margin:0 0 10px 0;font-size:20px;font-weight:700;color:#0f172a;">%s</h1>
              <p style="margin:0 0 20px 0;font-size:14px;line-height:1.6;color:#475569;">
                Hello <strong>%s</strong>,<br><br>
                Your expense claim for <strong>%s</strong> in <strong>%s</strong> has been reviewed by <strong>%s</strong> (%s).
              </p>

              <!-- Claim Summary Table -->
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:12px;padding:16px 20px;margin-bottom:20px;">
                <tr>
                  <td style="padding:6px 0;font-size:13px;color:#64748b;width:40%%;">Claim Item:</td>
                  <td style="padding:6px 0;font-size:13px;font-weight:600;color:#0f172a;width:60%%;">%s</td>
                </tr>
                <tr>
                  <td style="padding:6px 0;font-size:13px;color:#64748b;">Amount:</td>
                  <td style="padding:6px 0;font-size:16px;font-weight:700;color:%s;">₹%.2f</td>
                </tr>
                <tr>
                  <td style="padding:6px 0;font-size:13px;color:#64748b;">Category:</td>
                  <td style="padding:6px 0;font-size:13px;font-weight:600;color:#0f172a;text-transform:capitalize;">%s</td>
                </tr>
                <tr>
                  <td style="padding:6px 0;font-size:13px;color:#64748b;">Reviewed By:</td>
                  <td style="padding:6px 0;font-size:13px;font-weight:600;color:#0f172a;">%s (%s)</td>
                </tr>
              </table>

              %s

              <p style="margin:0;font-size:12px;color:#94a3b8;line-height:1.5;">
                To view the complete audit log and updated expedition settlement, visit your Splito dashboard.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color:#f8fafc;border-top:1px solid #f1f5f9;padding:16px 32px;text-align:center;">
              <p style="margin:0;font-size:11px;color:#94a3b8;">
                Splito • Group expenses, split with zero drama.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
			headerTitle,
			badgeColor, badgeBg, badgeBorder, badgeText,
			tripName,
			headerTitle,
			claimantName,
			claimTitle,
			tripName,
			reviewerName, reviewerRoleFormatted,
			claimTitle,
			badgeColor, claim.Amount,
			claim.Category,
			reviewerName, reviewerRoleFormatted,
			explanationHtml,
		)

		from := s.cfg.SMTPFrom
		if from == "" {
			from = s.cfg.SMTPUser
		}

		auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)
		addr := fmt.Sprintf("%s:%s", s.cfg.SMTPHost, s.cfg.SMTPPort)

		msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s", claimantEmail, subject, htmlContent))

		if err := smtp.SendMail(addr, auth, from, []string{claimantEmail}, msg); err != nil {
			fmt.Printf("[SPLITO CLAIM EMAIL ERROR] Failed to send decision email to %s: %v\n", claimantEmail, err)
		} else {
			fmt.Printf("[SPLITO CLAIM EMAIL SUCCESS] Successfully sent %s email to %s for claim '%s' (₹%.2f)\n", badgeText, claimantEmail, claimTitle, claim.Amount)
		}
	}()
}
