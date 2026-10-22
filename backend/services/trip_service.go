package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"tripledger/BLs"
	"tripledger/config"
	"tripledger/loaders"
	"tripledger/models"
	"tripledger/types"
	"tripledger/writers"
)

type TripService struct {
	db            *sql.DB
	cfg           *config.Config
	tripLoader    *loaders.TripLoader
	tripWriter    *writers.TripWriter
	userWriter    *writers.UserWriter
	expenseLoader *loaders.ExpenseLoader
}

func NewTripService(
	db *sql.DB,
	cfg *config.Config,
	tl *loaders.TripLoader,
	tw *writers.TripWriter,
	uw *writers.UserWriter,
	el *loaders.ExpenseLoader,
) *TripService {
	return &TripService{
		db:            db,
		cfg:           cfg,
		tripLoader:    tl,
		tripWriter:    tw,
		userWriter:    uw,
		expenseLoader: el,
	}
}

func (s *TripService) CreateTrip(ctx context.Context, creatorUserID int64, req *types.CreateTripRequest) (*models.Trip, error) {
	if req.StartAt.After(req.EndAt) {
		return nil, errors.New("trip start_at must be before or equal to end_at")
	}
	if err := validateTripLocations(req.StartPoint, req.StartLatitude, req.StartLongitude, req.Destination, req.DestLatitude, req.DestLongitude, req.Stops); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := BLs.CheckTripDateOverlap(ctx, tx, creatorUserID, req.StartAt, req.EndAt); err != nil {
		return nil, err
	}

	trip := &models.Trip{
		Name:            req.Title,
		Title:           req.Title,
		StartLocation:   req.StartPoint,
		StartPoint:      req.StartPoint,
		StartLatitude:   req.StartLatitude,
		StartLongitude:  req.StartLongitude,
		Destination:     req.Destination,
		DestLatitude:    req.DestLatitude,
		DestLongitude:   req.DestLongitude,
		StartDate:       req.StartAt,
		StartAt:         req.StartAt,
		EndDate:         req.EndAt,
		EndAt:           req.EndAt,
		EstimatedBudget: req.EstimatedBudget,
		Status:          models.TripStatusPlanned,
		CreatedBy:       creatorUserID,
		Stops:           make([]models.TripStop, len(req.Stops)),
	}
	for i, stop := range req.Stops {
		trip.Stops[i] = models.TripStop{
			TripID: trip.ID, Position: i, Name: stop.Name,
			Latitude: *stop.Latitude, Longitude: *stop.Longitude,
		}
	}

	tripID, err := s.tripWriter.CreateTripTx(ctx, tx, trip)
	if err != nil {
		return nil, err
	}
	trip.ID = tripID
	for i := range trip.Stops {
		trip.Stops[i].TripID = tripID
	}
	if err := s.tripWriter.SaveStopsTx(ctx, tx, tripID, trip.Stops); err != nil {
		return nil, err
	}

	err = s.tripWriter.AddMemberTx(ctx, tx, tripID, creatorUserID, models.RoleMunimji, models.MembershipActive)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return trip, nil
}

func validateTripLocations(startName string, startLatitude, startLongitude *float64, destinationName string, destinationLatitude, destinationLongitude *float64, stops []types.TripStopInput) error {
	if len(stops) > 20 {
		return errors.New("a trip can have at most 20 stops")
	}
	if err := validateLocation(startName, startLatitude, startLongitude); err != nil {
		return fmt.Errorf("starting point: %w", err)
	}
	if err := validateLocation(destinationName, destinationLatitude, destinationLongitude); err != nil {
		return fmt.Errorf("destination: %w", err)
	}
	for i, stop := range stops {
		if err := validateLocation(stop.Name, stop.Latitude, stop.Longitude); err != nil {
			return fmt.Errorf("stop %d: %w", i+1, err)
		}
	}
	return nil
}

func validateLocation(name string, latitude, longitude *float64) error {
	if strings.TrimSpace(name) == "" || len(name) > 255 {
		return errors.New("name must be between 1 and 255 characters")
	}
	if latitude == nil || longitude == nil ||
		math.IsNaN(*latitude) || math.IsInf(*latitude, 0) ||
		math.IsNaN(*longitude) || math.IsInf(*longitude, 0) ||
		*latitude < -90 || *latitude > 90 || *longitude < -180 || *longitude > 180 {
		return errors.New("select a valid map location")
	}
	return nil
}

func (s *TripService) UpdateTrip(ctx context.Context, userID int64, req *types.UpdateTripRequest) (*models.Trip, error) {
	if req.StartAt.After(req.EndAt) {
		return nil, errors.New("trip start_at must be before or equal to end_at")
	}
	if err := validateTripLocations(req.StartPoint, req.StartLatitude, req.StartLongitude, req.Destination, req.DestLatitude, req.DestLongitude, req.Stops); err != nil {
		return nil, err
	}

	trip, membership, err := s.GetTripDetail(ctx, req.TripID, userID)
	if err != nil {
		return nil, err
	}
	if trip == nil {
		return nil, errors.New("trip not found")
	}
	if membership == nil || membership.Status != models.MembershipActive || membership.Role != models.RoleMunimji {
		return nil, errors.New("only an active Munimji can edit a trip")
	}
	if trip.Status != models.TripStatusPlanned && trip.Status != models.TripStatusActive {
		return nil, errors.New("only planned or active trips can be edited")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := BLs.CheckTripDateOverlapExcept(ctx, tx, userID, req.TripID, req.StartAt, req.EndAt); err != nil {
		return nil, err
	}

	trip.Name = req.Title
	trip.Title = req.Title
	trip.StartLocation = req.StartPoint
	trip.StartPoint = req.StartPoint
	trip.StartLatitude = req.StartLatitude
	trip.StartLongitude = req.StartLongitude
	trip.Destination = req.Destination
	trip.DestLatitude = req.DestLatitude
	trip.DestLongitude = req.DestLongitude
	trip.StartDate = req.StartAt
	trip.StartAt = req.StartAt
	trip.EndDate = req.EndAt
	trip.EndAt = req.EndAt
	trip.EstimatedBudget = req.EstimatedBudget
	trip.Stops = make([]models.TripStop, len(req.Stops))
	for i, stop := range req.Stops {
		trip.Stops[i] = models.TripStop{
			TripID: req.TripID, Position: i, Name: stop.Name,
			Latitude: *stop.Latitude, Longitude: *stop.Longitude,
		}
	}

	if err := s.tripWriter.UpdateTripTx(ctx, tx, trip, trip.Stops); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit trip update: %w", err)
	}
	return trip, nil
}

func (s *TripService) ListUserTrips(ctx context.Context, userID int64) ([]models.Trip, error) {
	userLoader := loaders.NewUserLoader(s.db)
	user, err := userLoader.GetByID(ctx, userID)
	email := ""
	if err == nil && user != nil {
		email = user.Email
	}
	return s.tripLoader.ListUserTrips(ctx, userID, email)
}

func (s *TripService) GetTripDetail(ctx context.Context, tripID, userID int64) (*models.Trip, *models.TripMember, error) {
	trip, err := s.tripLoader.GetByID(ctx, tripID)
	if err != nil {
		return nil, nil, err
	}
	if trip == nil {
		return nil, nil, errors.New("trip not found")
	}

	member, err := s.tripLoader.GetUserMembership(ctx, tripID, userID)
	if err != nil {
		return nil, nil, err
	}

	return trip, member, nil
}

func (s *TripService) InviteMember(ctx context.Context, inviterID int64, req *types.InvitePeerRequest, userLoader *loaders.UserLoader) error {
	trip, member, err := s.GetTripDetail(ctx, req.TripID, inviterID)
	if err != nil {
		return err
	}
	if trip == nil || member == nil || member.Role != models.RoleMunimji {
		return errors.New("only Munimji can invite members")
	}

	role := models.RoleParticipant
	if req.Role == "munimji" {
		role = models.RoleMunimji
	} else if req.Role == "co_reviewer" {
		role = models.RoleCoReviewer
	}

	// 1. Check if user already exists and is already an active member of this trip
	invitedUser, _ := userLoader.GetByEmail(ctx, req.Email)
	if invitedUser != nil {
		existingMember, err := s.tripLoader.GetUserMembership(ctx, req.TripID, invitedUser.ID)
		if err == nil && existingMember != nil && existingMember.Status == models.MembershipActive {
			return errors.New("user is already an active participant in this expedition")
		}
	}

	// 2. Generate secure invitation token
	tokenBytes := make([]byte, 16)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("failed to generate invitation token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	// 3. Check if there's already a pending invitation for this email on this trip
	var pendingID int64
	err = s.db.QueryRowContext(ctx, `
		SELECT id FROM trip_invitations 
		WHERE trip_id = ? AND LOWER(invitee_email) = LOWER(?) AND status = 'pending'
		LIMIT 1
	`, req.TripID, strings.TrimSpace(req.Email)).Scan(&pendingID)

	if err == nil && pendingID > 0 {
		// Update existing pending invitation with updated role and fresh token
		_, err = s.db.ExecContext(ctx, `
			UPDATE trip_invitations 
			SET role = ?, token = ?, updated_at = NOW() 
			WHERE id = ?
		`, role, token, pendingID)
		if err != nil {
			return fmt.Errorf("failed to update pending invitation: %w", err)
		}
	} else {
		// 4. Save new invitation record
		invitation := &models.TripInvitation{
			TripID:        req.TripID,
			InviterUserID: inviterID,
			InviteeEmail:  strings.TrimSpace(req.Email),
			Role:          role,
			Token:         token,
			Status:        models.InvitationPending,
		}

		if _, err := s.tripWriter.CreateInvitation(ctx, invitation); err != nil {
			return fmt.Errorf("failed to record invitation: %w", err)
		}
	}

	// 5. Send Invitation Email via Email Service (Brevo / Resend / SMTP)
	roleTitle := "Participant"
	if role == models.RoleCoReviewer {
		roleTitle = "Co-Reviewer (Auditor)"
	} else if role == models.RoleMunimji {
		roleTitle = "Munimji"
	}

	appBaseURL := "https://splito-alpha.vercel.app"
	if s.cfg.AppEnv == "development" {
		appBaseURL = "http://localhost:5173"
	}
	inviteURL := fmt.Sprintf("%s/?invite_trip=%d&invite_token=%s", appBaseURL, req.TripID, token)
	subject := fmt.Sprintf("You're invited to join trip \"%s\" as %s on Splito", trip.Name, roleTitle)

	htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Trip Invitation</title>
</head>
<body style="margin:0;padding:0;background-color:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#0f172a;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background-color:#f1f5f9;padding:40px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" style="max-width:520px;background-color:#ffffff;border-radius:16px;border:1px solid #e2e8f0;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.06);">
          <tr>
            <td style="background-color:#0f172a;padding:28px 32px;text-align:left;">
              <div style="display:inline-block;">
                <span style="color:#ffffff;font-size:24px;font-weight:800;letter-spacing:-0.5px;">Splito</span>
                <span style="color:#38bdf8;font-size:12px;margin-left:10px;font-weight:700;text-transform:uppercase;letter-spacing:1px;background:rgba(56,189,248,0.18);padding:3px 8px;border-radius:6px;">Invitation</span>
              </div>
              <p style="margin:6px 0 0 0;color:#94a3b8;font-size:12px;">Expedition Treasury Protocol</p>
            </td>
          </tr>
          <tr>
            <td style="padding:32px 32px 24px 32px;">
              <h1 style="margin:0 0 10px 0;font-size:20px;font-weight:700;color:#0f172a;">You've been invited!</h1>
              <p style="margin:0 0 20px 0;font-size:14px;line-height:1.6;color:#475569;">
                Munimji has invited you to join the shared trip <strong>%s</strong> as a <strong>%s</strong>.
              </p>

              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:12px;padding:18px 20px;margin-bottom:24px;">
                <tr>
                  <td>
                    <div style="font-size:11px;font-weight:700;color:#64748b;text-transform:uppercase;letter-spacing:0.5px;">Expedition Details</div>
                    <div style="font-size:17px;font-weight:700;color:#0f172a;margin:4px 0 8px 0;">%s</div>
                    <div style="font-size:13px;color:#475569;">Route: <strong>%s</strong> &rarr; <strong>%s</strong></div>
                    <div style="font-size:13px;color:#2563eb;font-weight:600;margin-top:6px;">Assigned Role: %s</div>
                  </td>
                </tr>
              </table>

              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="margin:0 0 24px 0;">
                <tr>
                  <td align="center">
                    <a href="%s" style="display:inline-block;background-color:#2563eb;color:#ffffff;font-size:14px;font-weight:700;text-decoration:none;padding:14px 28px;border-radius:10px;box-shadow:0 4px 12px rgba(37,99,235,0.25);">
                      Review & Respond to Invitation &rarr;
                    </a>
                  </td>
                </tr>
              </table>

              <p style="margin:0;font-size:12px;color:#64748b;line-height:1.5;">
                <em>Note:</em> Clicking the button will take you directly to this trip. If you are not signed in yet, sign in with your email to accept or decline.
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
</html>`, trip.Name, roleTitle, trip.Name, trip.StartLocation, trip.Destination, roleTitle, inviteURL)

	DispatchEmail(s.cfg, req.Email, subject, htmlContent, "SPLITO INVITATION")

	return nil
}

func (s *TripService) RespondInvitation(ctx context.Context, userID, tripID int64, action string) error {
	userLoader := loaders.NewUserLoader(s.db)
	user, err := userLoader.GetByID(ctx, userID)
	if err != nil || user == nil {
		return errors.New("user profile not found")
	}

	if action != "accept" && action != "reject" {
		return errors.New("invalid action, must be 'accept' or 'reject'")
	}

	return s.tripWriter.RespondInvitation(ctx, tripID, userID, user.Email, action)
}

func (s *TripService) ListTripInvitations(ctx context.Context, tripID int64) ([]models.TripInvitation, error) {
	return s.tripLoader.ListTripInvitations(ctx, tripID)
}

func (s *TripService) ListMembers(ctx context.Context, tripID int64) ([]models.TripMember, error) {
	return s.tripLoader.ListTripMembers(ctx, tripID)
}

func (s *TripService) UpdateTripStatus(ctx context.Context, userID, tripID int64, status string) error {
	trip, member, err := s.GetTripDetail(ctx, tripID, userID)
	if err != nil {
		return err
	}
	if trip == nil || member == nil || member.Role != models.RoleMunimji {
		return errors.New("only Munimji can change trip status")
	}

	validStatus := models.TripStatus(status)
	return s.tripWriter.UpdateTripStatus(ctx, tripID, validStatus)
}

func (s *TripService) DeleteTrip(ctx context.Context, userID, tripID int64) error {
	trip, member, err := s.GetTripDetail(ctx, tripID, userID)
	if err != nil {
		return err
	}
	if trip == nil || member == nil || member.Role != models.RoleMunimji {
		return errors.New("only Munimji can delete an expedition")
	}

	return s.tripWriter.DeleteTrip(ctx, tripID)
}

func (s *TripService) GetSettlement(ctx context.Context, tripID int64) (*types.TripSettlementResponse, error) {
	members, err := s.tripLoader.ListTripMembers(ctx, tripID)
	if err != nil {
		return nil, err
	}
	contribs, err := s.expenseLoader.ListTripContributions(ctx, tripID)
	if err != nil {
		return nil, err
	}
	expenses, err := s.expenseLoader.ListTripExpenses(ctx, tripID)
	if err != nil {
		return nil, err
	}
	claims, err := s.expenseLoader.ListTripClaims(ctx, tripID)
	if err != nil {
		return nil, err
	}

	return BLs.CalculateSettlement(tripID, members, contribs, expenses, claims), nil
}

// GenerateTripReport generates an executive, high-aesthetic PDF settlement report
func (s *TripService) GenerateTripReport(ctx context.Context, tripID int64) (string, []byte, error) {
	trip, err := s.tripLoader.GetByID(ctx, tripID)
	if err != nil {
		return "", nil, err
	}
	if trip == nil {
		return "", nil, errors.New("trip not found")
	}

	members, err := s.tripLoader.ListTripMembers(ctx, tripID)
	if err != nil {
		return "", nil, err
	}

	contribs, err := s.expenseLoader.ListTripContributions(ctx, tripID)
	if err != nil {
		return "", nil, err
	}

	expenses, err := s.expenseLoader.ListTripExpenses(ctx, tripID)
	if err != nil {
		return "", nil, err
	}

	claims, err := s.expenseLoader.ListTripClaims(ctx, tripID)
	if err != nil {
		return "", nil, err
	}

	totalContribs, groupSpend, approvedClaims, err := s.expenseLoader.GetTripFinancialSummary(ctx, tripID)
	if err != nil {
		return "", nil, err
	}
	availCash := totalContribs - groupSpend

	// Setup Clean High-Aesthetic PDF
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()

	// Header Banner: Midnight Slate Background
	pdf.SetFillColor(25, 28, 32)
	pdf.Rect(15, 15, 180, 26, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetXY(20, 20)
	pdf.Cell(110, 8, "SPLITO TREASURY LEDGER")

	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(59, 130, 246) // Blue accent
	pdf.SetXY(135, 20)
	pdf.CellFormat(55, 8, "AUDITED STATEMENT", "", 0, "R", false, 0, "")

	pdf.SetTextColor(180, 185, 195)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetXY(20, 29)
	pdf.Cell(150, 6, fmt.Sprintf("%s  |  Route: %s -> %s", trip.Name, trip.StartLocation, trip.Destination))

	pdf.SetXY(15, 48)

	// Summary KPI Grid (4 Cards)
	kpiWidth := 43.0
	kpiHeight := 20.0
	pdf.SetFillColor(240, 242, 245)
	pdf.SetDrawColor(215, 220, 228)

	// 1. Total Inflow
	pdf.Rect(15, 46, kpiWidth, kpiHeight, "FD")
	pdf.SetXY(17, 49)
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetTextColor(100, 110, 125)
	pdf.Cell(kpiWidth-4, 4, "TOTAL CONTRIBUTIONS")
	pdf.SetXY(17, 54)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(25, 28, 32)
	pdf.Cell(kpiWidth-4, 7, fmt.Sprintf("Rs. %.2f", totalContribs))

	// 2. Total Outflow
	pdf.Rect(60, 46, kpiWidth, kpiHeight, "FD")
	pdf.SetXY(62, 49)
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetTextColor(100, 110, 125)
	pdf.Cell(kpiWidth-4, 4, "GROUP SPEND")
	pdf.SetXY(62, 54)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(25, 28, 32)
	pdf.Cell(kpiWidth-4, 7, fmt.Sprintf("Rs. %.2f", groupSpend))

	// 3. Available Cash (Focal)
	pdf.SetFillColor(224, 238, 255)
	pdf.Rect(105, 46, kpiWidth, kpiHeight, "FD")
	pdf.SetXY(107, 49)
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetTextColor(37, 99, 235)
	pdf.Cell(kpiWidth-4, 4, "CASH IN HAND")
	pdf.SetXY(107, 54)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(25, 28, 32)
	pdf.Cell(kpiWidth-4, 7, fmt.Sprintf("Rs. %.2f", availCash))

	// 4. Approved Claims Liability
	pdf.SetFillColor(240, 242, 245)
	pdf.Rect(150, 46, kpiWidth, kpiHeight, "FD")
	pdf.SetXY(152, 49)
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetTextColor(100, 110, 125)
	pdf.Cell(kpiWidth-4, 4, "APPROVED CLAIMS")
	pdf.SetXY(152, 54)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(25, 28, 32)
	pdf.Cell(kpiWidth-4, 7, fmt.Sprintf("Rs. %.2f", approvedClaims))

	pdf.Ln(26)

	// Section 1: Member Pool Inflow Ledger
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(25, 28, 32)
	pdf.Cell(0, 7, fmt.Sprintf("1. Member Contributions Pool (%d Records)", len(contribs)))
	pdf.Ln(8)

	// Table Header
	pdf.SetFillColor(230, 234, 240)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetTextColor(50, 55, 65)
	pdf.CellFormat(40, 6, "Member", "1", 0, "L", true, 0, "")
	pdf.CellFormat(65, 6, "Method / Reference", "1", 0, "L", true, 0, "")
	pdf.CellFormat(42, 6, "Received At", "1", 0, "L", true, 0, "")
	pdf.CellFormat(33, 6, "Amount (INR)", "1", 1, "R", true, 0, "")

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(30, 30, 30)
	if len(contribs) == 0 {
		pdf.CellFormat(180, 6, "No member contributions recorded yet.", "1", 1, "C", false, 0, "")
	} else {
		for _, c := range contribs {
			method := c.PaymentMethod
			if c.Notes != nil && *c.Notes != "" {
				method = fmt.Sprintf("%s (%s)", method, *c.Notes)
			}
			pdf.CellFormat(40, 6, truncatePDFText(c.UserName, 22), "1", 0, "L", false, 0, "")
			pdf.CellFormat(65, 6, truncatePDFText(method, 40), "1", 0, "L", false, 0, "")
			pdf.CellFormat(42, 6, c.ReceivedAt.Format("02-Jan-2006 15:04"), "1", 0, "L", false, 0, "")
			pdf.CellFormat(33, 6, fmt.Sprintf("Rs. %.2f", c.Amount), "1", 1, "R", false, 0, "")
		}
	}
	pdf.Ln(6)

	// Section 2: Group Expenses Outflow
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(25, 28, 32)
	pdf.Cell(0, 7, fmt.Sprintf("2. Group-Funded Outflows (%d Records)", len(expenses)))
	pdf.Ln(8)

	pdf.SetFillColor(230, 234, 240)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetTextColor(50, 55, 65)
	pdf.CellFormat(68, 6, "Title / Description", "1", 0, "L", true, 0, "")
	pdf.CellFormat(35, 6, "Category", "1", 0, "L", true, 0, "")
	pdf.CellFormat(44, 6, "Expense Date", "1", 0, "L", true, 0, "")
	pdf.CellFormat(33, 6, "Amount (INR)", "1", 1, "R", true, 0, "")

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(30, 30, 30)
	if len(expenses) == 0 {
		pdf.CellFormat(180, 6, "No group-funded expenses logged.", "1", 1, "C", false, 0, "")
	} else {
		for _, e := range expenses {
			pdf.CellFormat(68, 6, truncatePDFText(e.Title, 42), "1", 0, "L", false, 0, "")
			pdf.CellFormat(35, 6, truncatePDFText(e.Category, 20), "1", 0, "L", false, 0, "")
			pdf.CellFormat(44, 6, e.ExpenseDate.Format("02-Jan-2006 15:04"), "1", 0, "L", false, 0, "")
			pdf.CellFormat(33, 6, fmt.Sprintf("Rs. %.2f", e.Amount), "1", 1, "R", false, 0, "")
		}
	}
	pdf.Ln(6)

	// Section 3: Peer Claims Protocol
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(25, 28, 32)
	pdf.Cell(0, 7, fmt.Sprintf("3. Peer Expense Claims & Approvals (%d Records)", len(claims)))
	pdf.Ln(8)

	pdf.SetFillColor(230, 234, 240)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetTextColor(50, 55, 65)
	pdf.CellFormat(36, 6, "Claimant", "1", 0, "L", true, 0, "")
	pdf.CellFormat(73, 6, "Reason / Category", "1", 0, "L", true, 0, "")
	pdf.CellFormat(38, 6, "Status", "1", 0, "L", true, 0, "")
	pdf.CellFormat(33, 6, "Amount (INR)", "1", 1, "R", true, 0, "")

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(30, 30, 30)
	if len(claims) == 0 {
		pdf.CellFormat(180, 6, "No peer claims submitted.", "1", 1, "C", false, 0, "")
	} else {
		for _, cl := range claims {
			statusText := fmt.Sprintf("%s (%s)", cl.Status, cl.ReimbursementStatus)
			reasonCategory := fmt.Sprintf("%s [%s]", cl.Reason, cl.Category)
			pdf.CellFormat(36, 6, truncatePDFText(cl.ClaimantName, 20), "1", 0, "L", false, 0, "")
			pdf.CellFormat(73, 6, truncatePDFText(reasonCategory, 46), "1", 0, "L", false, 0, "")
			pdf.CellFormat(38, 6, truncatePDFText(statusText, 22), "1", 0, "L", false, 0, "")
			pdf.CellFormat(33, 6, fmt.Sprintf("Rs. %.2f", cl.Amount), "1", 1, "R", false, 0, "")
		}
	}

	// Footer Metadata
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "I", 7)
	pdf.SetTextColor(120, 125, 135)
	pdf.Cell(0, 4, fmt.Sprintf("Generated on %s by Splito | Munimji Treasury Management | Participants: %d",
		time.Now().Format(time.RFC1123), len(members)))

	fileName := fmt.Sprintf("Splito_Report_%s_%d.pdf", trip.Name, trip.ID)
	var buf []byte
	err = pdf.Output(&outputBuffer{buf: &buf})
	if err != nil {
		return "", nil, fmt.Errorf("failed to output pdf: %w", err)
	}

	return fileName, buf, nil
}

func truncatePDFText(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

type outputBuffer struct {
	buf *[]byte
}

func (b *outputBuffer) Write(p []byte) (n int, err error) {
	*b.buf = append(*b.buf, p...)
	return len(p), nil
}

func (s *TripService) RemoveMember(ctx context.Context, munimjiID int64, req models.RemoveMemberRequest) error {
	// 1. Verify caller is Munimji of this trip
	membership, err := s.tripLoader.GetUserMembership(ctx, req.TripID, munimjiID)
	if err != nil {
		return fmt.Errorf("failed to verify permissions: %w", err)
	}
	if membership == nil || membership.Role != models.RoleMunimji {
		return errors.New("unauthorized: only Munimji can remove participants from the expedition")
	}

	// 2. Cannot remove self
	if req.UserID == munimjiID {
		return errors.New("Munimji cannot remove themselves from the expedition")
	}

	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		return errors.New("removal reason is mandatory")
	}

	// 3. Fetch target user and trip details
	var targetEmail, targetName string
	err = s.db.QueryRowContext(ctx, "SELECT email, name FROM user_details WHERE id = ?", req.UserID).Scan(&targetEmail, &targetName)
	if err != nil {
		return fmt.Errorf("target participant not found: %w", err)
	}

	trip, err := s.tripLoader.GetByID(ctx, req.TripID)
	if err != nil || trip == nil {
		return errors.New("trip not found")
	}

	var munimjiName string
	_ = s.db.QueryRowContext(ctx, "SELECT name FROM user_details WHERE id = ?", munimjiID).Scan(&munimjiName)
	if munimjiName == "" {
		munimjiName = "Munimji"
	}

	// 4. Update status in DB
	if err := s.tripWriter.RemoveMember(ctx, req.TripID, req.UserID, munimjiID, reason); err != nil {
		return err
	}

	// 5. Also completely delete any invitation records for this email on this trip
	_ = s.tripWriter.DeleteInvitationForEmail(ctx, req.TripID, targetEmail)

	// 6. Send notification email via Email Service (Brevo / Resend / SMTP)
	if targetEmail != "" {
		subject := fmt.Sprintf("Notice: You have been removed from trip \"%s\"", trip.Title)

		htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <title>Expedition Membership Update</title>
</head>
<body style="margin:0;padding:0;background-color:#f1f5f9;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;color:#0f172a;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background-color:#f1f5f9;padding:40px 16px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" style="max-width:520px;background-color:#ffffff;border-radius:16px;border:1px solid #e2e8f0;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.06);">
          <tr>
            <td style="background-color:#0f172a;padding:28px 32px;text-align:left;">
              <div style="display:inline-block;">
                <span style="color:#ffffff;font-size:24px;font-weight:800;letter-spacing:-0.5px;">Splito</span>
                <span style="color:#f87171;font-size:12px;margin-left:10px;font-weight:700;text-transform:uppercase;letter-spacing:1px;background:rgba(248,113,113,0.18);padding:3px 8px;border-radius:6px;">Membership Notice</span>
              </div>
              <p style="margin:6px 0 0 0;color:#94a3b8;font-size:12px;">Expedition Treasury Protocol</p>
            </td>
          </tr>
          <tr>
            <td style="padding:32px 32px 24px 32px;">
              <h1 style="margin:0 0 10px 0;font-size:20px;font-weight:700;color:#0f172a;">Expedition Membership Revoked</h1>
              <p style="margin:0 0 20px 0;font-size:14px;line-height:1.6;color:#475569;">
                Hello <strong>%s</strong>,<br><br>
                Munimji (<strong>%s</strong>) has removed your account from the shared expedition <strong>%s</strong>.
              </p>

              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#fef2f2;border:1px solid #fecaca;border-radius:12px;padding:18px 20px;margin-bottom:24px;">
                <tr>
                  <td>
                    <div style="font-size:11px;font-weight:700;color:#991b1b;text-transform:uppercase;letter-spacing:0.5px;">Removal Reason Provided</div>
                    <div style="font-size:15px;font-weight:600;color:#7f1d1d;margin:6px 0 0 0;line-height:1.5;">%s</div>
                  </td>
                </tr>
              </table>

              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#f8fafc;border:1px solid #e2e8f0;border-radius:12px;padding:16px 20px;margin-bottom:24px;">
                <tr>
                  <td style="font-size:12px;color:#64748b;line-height:1.6;">
                    <strong style="color:#334155;">Treasury & Ledger Note:</strong><br>
                    Your past approved contributions and verified expenses remain recorded in the trip ledger. You will no longer have access to participate or file claims under this trip.
                  </td>
                </tr>
              </table>

              <p style="margin:0;font-size:12px;color:#94a3b8;line-height:1.5;">
                If you have questions regarding this decision or settlement transfers, please contact the expedition Munimji directly.
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
</html>`, targetName, munimjiName, trip.Title, reason)

		DispatchEmail(s.cfg, targetEmail, subject, htmlContent, "SPLITO MEMBER REMOVED")
	}

	return nil
}

func (s *TripService) ListUserPendingInvitations(ctx context.Context, email string) ([]models.TripInvitation, error) {
	return s.tripLoader.ListUserPendingInvitations(ctx, email)
}

func (s *TripService) UpdateMemberRole(ctx context.Context, munimjiID int64, req *types.UpdateMemberRoleRequest) error {
	membership, err := s.tripLoader.GetUserMembership(ctx, req.TripID, munimjiID)
	if err != nil || membership == nil || membership.Role != models.RoleMunimji {
		return errors.New("only Munimji can configure member roles")
	}

	if req.UserID == munimjiID {
		return errors.New("Munimji role is the expedition creator role and cannot be changed")
	}

	targetMem, err := s.tripLoader.GetUserMembership(ctx, req.TripID, req.UserID)
	if err != nil || targetMem == nil || targetMem.Status != models.MembershipActive {
		return errors.New("target participant not found or is not active")
	}

	var targetRole models.Role
	switch req.Role {
	case "co_reviewer":
		targetRole = models.RoleCoReviewer
	case "participant":
		targetRole = models.RoleParticipant
	default:
		return fmt.Errorf("invalid role '%s', must be 'co_reviewer' or 'participant'", req.Role)
	}

	if targetMem.Role == targetRole {
		return nil // already that role
	}

	return s.tripWriter.UpdateMemberRole(ctx, req.TripID, req.UserID, targetRole)
}
