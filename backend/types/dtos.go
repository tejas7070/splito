package types

import "time"

type SendOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type VerifyOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
	OTP   string `json:"otp" binding:"required,len=6"`
}

type RegisterOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type RegisterRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	OTP      string `json:"otp" binding:"required,len=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type ForgotPasswordOTPRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Email       string `json:"email" binding:"required,email"`
	OTP         string `json:"otp" binding:"required,len=6"`
	NewPassword string `json:"new_password" binding:"required,min=6"`
}

type AuthResponse struct {
	AccessToken string `json:"access_token"`
	User        any    `json:"user"`
}

type UpdateProfileRequest struct {
	Name  string  `json:"name" binding:"required"`
	Phone *string `json:"phone"`
}

type ChangePasswordRequest struct {
	CurrentPassword *string `json:"current_password"`
	OTP             *string `json:"otp"`
	NewPassword     string  `json:"new_password" binding:"required,min=6"`
}

type RequestPasswordChangeOTPResponse struct {
	Message string `json:"message"`
	Target  string `json:"target"` // masked phone or email
}

type GoogleAuthRequest struct {
	Credential string `json:"credential" binding:"required"`
}

type CreateTripRequest struct {
	Title           string          `json:"title" binding:"required"`
	StartPoint      string          `json:"start_point" binding:"required"`
	StartLatitude   *float64        `json:"start_latitude"`
	StartLongitude  *float64        `json:"start_longitude"`
	Destination     string          `json:"destination" binding:"required"`
	DestLatitude    *float64        `json:"destination_latitude"`
	DestLongitude   *float64        `json:"destination_longitude"`
	StartAt         time.Time       `json:"start_at" binding:"required"`
	EndAt           time.Time       `json:"end_at" binding:"required"`
	EstimatedBudget float64         `json:"estimated_budget"`
	Stops           []TripStopInput `json:"stops"`
}

type UpdateTripRequest struct {
	TripID          int64           `json:"trip_id" binding:"required"`
	Title           string          `json:"title" binding:"required"`
	StartPoint      string          `json:"start_point" binding:"required"`
	StartLatitude   *float64        `json:"start_latitude"`
	StartLongitude  *float64        `json:"start_longitude"`
	Destination     string          `json:"destination" binding:"required"`
	DestLatitude    *float64        `json:"destination_latitude"`
	DestLongitude   *float64        `json:"destination_longitude"`
	StartAt         time.Time       `json:"start_at" binding:"required"`
	EndAt           time.Time       `json:"end_at" binding:"required"`
	EstimatedBudget float64         `json:"estimated_budget"`
	Stops           []TripStopInput `json:"stops"`
}

type TripStopInput struct {
	Name      string   `json:"name" binding:"required"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type TripDetailRequest struct {
	TripID int64 `json:"trip_id" binding:"required"`
}

type JoinTripRequest struct {
	TripID int64 `json:"trip_id" binding:"required"`
}

type InvitePeerRequest struct {
	TripID int64  `json:"trip_id" binding:"required"`
	Email  string `json:"email" binding:"required,email"`
	Role   string `json:"role" binding:"omitempty,oneof=munimji co_reviewer participant"`
}

type SearchEligibleUsersRequest struct {
	TripID int64  `json:"trip_id" binding:"required"`
	Query  string `json:"query"`
}

type EligibleUserResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type RespondInvitationRequest struct {
	TripID int64  `json:"trip_id" binding:"required"`
	Action string `json:"action" binding:"required,oneof=accept reject"`
}

type UpdateMemberRoleRequest struct {
	TripID int64  `json:"trip_id" binding:"required"`
	UserID int64  `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required,oneof=co_reviewer participant"`
}

type CreateClaimRequest struct {
	TripID      int64     `json:"trip_id" binding:"required"`
	Amount      float64   `json:"amount"`
	AmountPaise *int64    `json:"amount_paise"`
	Title       string    `json:"title"`
	Category    string    `json:"category" binding:"required"`
	Reason      string    `json:"reason" binding:"required"`
	ExpenseDate time.Time `json:"expense_date" binding:"required"`
	ReceiptPath *string   `json:"receipt_path"`
	ClientUUID  *string   `json:"client_uuid"`
}

type ClaimDecisionRequest struct {
	TripID  int64  `json:"trip_id" binding:"required"`
	ClaimID int64  `json:"claim_id" binding:"required"`
	Reason  string `json:"reason"`
}

type CreateContributionRequest struct {
	TripID        int64     `json:"trip_id" binding:"required"`
	UserID        int64     `json:"user_id" binding:"required"`
	Amount        float64   `json:"amount" binding:"required,gt=0"`
	PaymentMethod string    `json:"payment_method"`
	ReceivedAt    time.Time `json:"received_at" binding:"required"`
	Notes         *string   `json:"notes"`
	ClientUUID    *string   `json:"client_uuid"`
}

type CreateExpenseRequest struct {
	TripID      int64     `json:"trip_id" binding:"required"`
	Title       string    `json:"title" binding:"required"`
	Category    string    `json:"category" binding:"required"`
	Amount      float64   `json:"amount" binding:"required,gt=0"`
	PaidFrom    string    `json:"paid_from"`
	ExpenseDate time.Time `json:"expense_date" binding:"required"`
	Notes       *string   `json:"notes"`
	ClientUUID  *string   `json:"client_uuid"`
}

type GenerateReportRequest struct {
	TripID int64 `json:"trip_id" binding:"required"`
}

type UpdateTripStatusRequest struct {
	TripID int64  `json:"trip_id" binding:"required"`
	Status string `json:"status" binding:"required,oneof=planned active completed cancelled archived"`
}

type SettlementTransfer struct {
	FromUserID   int64   `json:"from_user_id"`
	FromUserName string  `json:"from_user_name"`
	ToUserID     int64   `json:"to_user_id"`
	ToUserName   string  `json:"to_user_name"`
	Amount       float64 `json:"amount"`
	AmountPaise  int64   `json:"amount_paise"`
	TransferType string  `json:"transfer_type"` // "group_claim", "pool_refund", "peer_fair_share"
	Description  string  `json:"description"`
}

type GroupClaimSettlement struct {
	ClaimID         int64   `json:"claim_id"`
	ClaimantID      int64   `json:"claimant_id"`
	ClaimantName    string  `json:"claimant_name"`
	Title           string  `json:"title"`
	Category        string  `json:"category"`
	Amount          float64 `json:"amount"`
	AmountPaise     int64   `json:"amount_paise"`
	OwedBy          string  `json:"owed_by"`
	PayableFromPool bool    `json:"payable_from_pool"`
}

type TreasurySettlementSummary struct {
	TotalContributionsPaise int64 `json:"total_contributions_paise"`
	GroupSpendPaise         int64 `json:"group_spend_paise"`
	AvailableCashPaise      int64 `json:"available_cash_paise"`
	ApprovedClaimsPaise     int64 `json:"approved_claims_paise"`
	NetSurplusPaise         int64 `json:"net_surplus_paise"`
}

type ParticipantBalance struct {
	UserID          int64   `json:"user_id"`
	UserName        string  `json:"user_name"`
	Email           string  `json:"email"`
	Contributed     float64 `json:"contributed"`
	Claimed         float64 `json:"claimed"`
	TotalPaid       float64 `json:"total_paid"`
	EqualShare      float64 `json:"equal_share"`
	NetBalance      float64 `json:"net_balance"` // Positive = should receive, Negative = owes pool/peers
	NetBalancePaise int64   `json:"net_balance_paise"`
	PoolRefund      float64 `json:"pool_refund"`
	ClaimPayout     float64 `json:"claim_payout"`
	PeerDues        float64 `json:"peer_dues"`
}

type TripSettlementResponse struct {
	TripID              int64                     `json:"trip_id"`
	TotalTripCost       float64                   `json:"total_trip_cost"`
	TotalTripCostPaise  int64                     `json:"total_trip_cost_paise"`
	GroupSpendPaise     int64                     `json:"group_spend_paise"`
	ApprovedClaimsPaise int64                     `json:"approved_claims_paise"`
	EqualSharePerUser   float64                   `json:"equal_share_per_user"`
	MemberCount         int                       `json:"member_count"`
	TreasurySummary     TreasurySettlementSummary `json:"treasury_summary"`
	GroupClaims         []GroupClaimSettlement    `json:"group_claims"`
	Balances            []ParticipantBalance      `json:"balances"`
	Transfers           []SettlementTransfer      `json:"transfers"`
}
