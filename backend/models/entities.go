package models

import "time"

type Role string

const (
	RoleMunimji     Role = "munimji"
	RoleCoReviewer  Role = "co_reviewer"
	RoleParticipant Role = "participant"
)

type MembershipStatus string

const (
	MembershipInvited  MembershipStatus = "invited"
	MembershipActive   MembershipStatus = "active"
	MembershipRejected MembershipStatus = "rejected"
	MembershipLeft     MembershipStatus = "left"
	MembershipRemoved  MembershipStatus = "removed"
	MembershipRequested MembershipStatus = "requested"
)

type InvitationStatus string

const (
	InvitationPending   InvitationStatus = "pending"
	InvitationAccepted  InvitationStatus = "accepted"
	InvitationRejected  InvitationStatus = "rejected"
	InvitationExpired   InvitationStatus = "expired"
	InvitationCancelled InvitationStatus = "cancelled"
)

type TripStatus string

const (
	TripStatusPlanned   TripStatus = "planned"
	TripStatusActive    TripStatus = "active"
	TripStatusCompleted TripStatus = "completed"
	TripStatusCancelled TripStatus = "cancelled"
	TripStatusArchived  TripStatus = "archived"
)

type ClaimStatus string

const (
	ClaimStatusPending                ClaimStatus = "pending"
	ClaimStatusClarificationRequested ClaimStatus = "clarification_requested"
	ClaimStatusApproved               ClaimStatus = "approved"
	ClaimStatusRejected               ClaimStatus = "rejected"
)

type ReimbursementStatus string

const (
	ReimbursementNotApplicable ReimbursementStatus = "not_applicable"
	ReimbursementUnpaid        ReimbursementStatus = "unpaid"
	ReimbursementPaid          ReimbursementStatus = "paid"
)

// Matches user_details table in splito_db
type User struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Phone        *string   `json:"phone,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Matches trips table in splito_db
type Trip struct {
	ID              int64            `json:"id"`
	Name            string           `json:"name"`
	Title           string           `json:"title"` // alias for frontend compatibility
	StartLocation   string           `json:"start_location"`
	StartPoint      string           `json:"start_point"` // alias for frontend compatibility
	StartLatitude   *float64         `json:"start_latitude,omitempty"`
	StartLongitude  *float64         `json:"start_longitude,omitempty"`
	Destination     string           `json:"destination"`
	DestLatitude    *float64         `json:"destination_latitude,omitempty"`
	DestLongitude   *float64         `json:"destination_longitude,omitempty"`
	StartDate       time.Time        `json:"start_date"`
	StartAt         time.Time        `json:"start_at"` // alias for frontend compatibility
	EndDate         time.Time        `json:"end_date"`
	EndAt           time.Time        `json:"end_at"` // alias for frontend compatibility
	EstimatedBudget float64          `json:"estimated_budget"`
	Stops           []TripStop       `json:"stops"`
	CreatedBy       int64            `json:"created_by"`
	Status          TripStatus       `json:"status"`
	UserRole        Role             `json:"user_role,omitempty"`
	UserStatus      MembershipStatus `json:"user_status,omitempty"`
	InviteCode      string           `json:"invite_code,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
	UpdatedAt       time.Time        `json:"updated_at"`
}

type TripStop struct {
	ID        int64   `json:"id,omitempty"`
	TripID    int64   `json:"trip_id,omitempty"`
	Position  int     `json:"position"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// Matches trip_members table in splito_db
type TripMember struct {
	ID            int64            `json:"id"`
	TripID        int64            `json:"trip_id"`
	UserID        int64            `json:"user_id"`
	UserName      string           `json:"user_name"`
	Email         string           `json:"email"`
	Role          Role             `json:"role"`
	Status        MembershipStatus `json:"status"`
	JoinedAt      *time.Time       `json:"joined_at,omitempty"`
	RemovalReason *string          `json:"removal_reason,omitempty"`
	RemovedBy     *int64           `json:"removed_by,omitempty"`
	RemovedAt     *time.Time       `json:"removed_at,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
}

// Matches trip_invitations table in splito_db
type TripInvitation struct {
	ID            int64            `json:"id"`
	TripID        int64            `json:"trip_id"`
	TripTitle     string           `json:"trip_title,omitempty"`
	StartLocation string           `json:"start_location,omitempty"`
	Destination   string           `json:"destination,omitempty"`
	InviterUserID int64            `json:"inviter_user_id"`
	InviterName   string           `json:"inviter_name,omitempty"`
	InviteeEmail  string           `json:"invitee_email"`
	Role          Role             `json:"role"`
	Token         string           `json:"token"`
	Status        InvitationStatus `json:"status"`
	ExpiresAt     *time.Time       `json:"expires_at,omitempty"`
	RespondedAt   *time.Time       `json:"responded_at,omitempty"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
}

// Matches contributions table in splito_db
type Contribution struct {
	ID            int64     `json:"id"`
	TripID        int64     `json:"trip_id"`
	UserID        int64     `json:"user_id"`
	UserName      string    `json:"user_name"`
	Amount        float64   `json:"amount"`
	AmountPaise   int64     `json:"amount_paise"`
	ReceivedAt    time.Time `json:"received_at"`
	PaymentMethod string    `json:"payment_method"`
	Notes         *string   `json:"notes,omitempty"`
	RecordedBy    int64     `json:"recorded_by"`
	ClientUUID    *string   `json:"client_uuid,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// Matches expenses table in splito_db
type Expense struct {
	ID          int64     `json:"id"`
	TripID      int64     `json:"trip_id"`
	Title       string    `json:"title"`
	Category    string    `json:"category"`
	Amount      float64   `json:"amount"`
	AmountPaise int64     `json:"amount_paise"`
	PaidFrom    string    `json:"paid_from"` // group_funds, personal
	PaidBy      int64     `json:"paid_by"`
	PaidByName  string    `json:"paid_by_name"`
	RecordedBy  int64     `json:"recorded_by"`
	ExpenseDate time.Time `json:"expense_date"`
	Notes       *string   `json:"notes,omitempty"`
	ReceiptPath *string   `json:"receipt_path,omitempty"`
	ClientUUID  *string   `json:"client_uuid,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Matches expense_claims table in splito_db
type ExpenseClaim struct {
	ID                  int64               `json:"id"`
	TripID              int64               `json:"trip_id"`
	ClaimantID          int64               `json:"claimant_id"`
	ClaimantName        string              `json:"claimant_name"`
	Title               string              `json:"title"`
	Amount              float64             `json:"amount"`
	AmountPaise         int64               `json:"amount_paise"` // alias for frontend compatibility
	Category            string              `json:"category"`
	Reason              string              `json:"reason"`
	ReceiptPath         *string             `json:"receipt_path,omitempty"`
	Status              ClaimStatus         `json:"status"`
	ReimbursementStatus ReimbursementStatus `json:"reimbursement_status"`
	ReviewedBy          *int64              `json:"reviewed_by,omitempty"`
	ReviewedAt          *time.Time          `json:"reviewed_at,omitempty"`
	RejectionReason     *string             `json:"rejection_reason,omitempty"`
	ClientUUID          *string             `json:"client_uuid,omitempty"`
	CreatedAt           time.Time           `json:"created_at"`
}

// Report job and deliveries
type TripReportJob struct {
	ID                 int64      `json:"id"`
	TripID             int64      `json:"trip_id"`
	RequestedBy        int64      `json:"requested_by"`
	JobStatus          string     `json:"job_status"` // queued, processing, completed, failed
	ReportType         string     `json:"report_type"`
	FilePath           *string    `json:"file_path,omitempty"`
	FileName           *string    `json:"file_name,omitempty"`
	FileSizeBytes      *int64     `json:"file_size_bytes,omitempty"`
	TotalSpend         float64    `json:"total_spend"`
	TotalContributions float64    `json:"total_contributions"`
	ErrorMessage       *string    `json:"error_message,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
}

type TripReportDelivery struct {
	ID              int64      `json:"id"`
	JobID           int64      `json:"job_id"`
	RecipientUserID int64      `json:"recipient_user_id"`
	RecipientEmail  string     `json:"recipient_email"`
	DeliveryStatus  string     `json:"delivery_status"` // pending, sending, sent, failed
	Attempts        int        `json:"attempts"`
	MaxAttempts     int        `json:"max_attempts"`
	NextRetryAt     *time.Time `json:"next_retry_at,omitempty"`
	SentAt          *time.Time `json:"sent_at,omitempty"`
	LastError       *string    `json:"last_error,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type RemoveMemberRequest struct {
	TripID int64  `json:"trip_id" binding:"required"`
	UserID int64  `json:"user_id" binding:"required"`
	Reason string `json:"reason" binding:"required"`
}
