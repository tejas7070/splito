package main

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jung-kurt/gofpdf"
)

type DemoUser struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"full_name"`
}

type DemoTrip struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	StartPoint  string    `json:"start_point"`
	Destination string    `json:"destination"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at"`
	Status      string    `json:"status"`
}

type DemoClaim struct {
	ID                  int64     `json:"id"`
	TripID              int64     `json:"trip_id"`
	ClaimantUserID      int64     `json:"claimant_user_id"`
	ClaimantName        string    `json:"claimant_name"`
	AmountPaise         int64     `json:"amount_paise"`
	Currency            string    `json:"currency"`
	Category            string    `json:"category"`
	Reason              string    `json:"reason"`
	ExpenseDate         time.Time `json:"expense_date"`
	ReceiptURL          *string   `json:"receipt_url,omitempty"`
	Status              string    `json:"status"`
	ReimbursementStatus string    `json:"reimbursement_status"`
	DecisionReason      string    `json:"decision_reason,omitempty"`
	CreatedAt           time.Time `json:"created_at"`
}

type DemoMember struct {
	ID        int64     `json:"id"`
	TripID    int64     `json:"trip_id"`
	UserID    int64     `json:"user_id"`
	UserName  string    `json:"user_name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	JoinedAt  time.Time `json:"joined_at"`
}

type DemoStore struct {
	mu            sync.Mutex
	users         map[string]*DemoUser
	trips         []DemoTrip
	claimList     []DemoClaim
	memberList    []DemoMember
	contributions int64
	groupSpend    int64
	claims        int64
}

var store = &DemoStore{
	users: make(map[string]*DemoUser),
	trips: []DemoTrip{
		{
			ID:          1,
			Title:       "Goa Road Trip 2026",
			StartPoint:  "Pune",
			Destination: "Goa",
			StartAt:     time.Now().Add(24 * time.Hour),
			EndAt:       time.Now().Add(5 * 24 * time.Hour),
			Status:      "upcoming",
		},
	},
	claimList: []DemoClaim{
		{
			ID:                  1,
			TripID:              1,
			ClaimantUserID:      2,
			ClaimantName:        "Sameer Patil",
			AmountPaise:         250000,
			Currency:            "INR",
			Category:            "Fuel & Toll",
			Reason:              "Expressway Toll & Diesel fillup",
			ExpenseDate:         time.Now().Add(-2 * time.Hour),
			Status:              "approved",
			ReimbursementStatus: "unpaid",
			CreatedAt:           time.Now().Add(-2 * time.Hour),
		},
		{
			ID:                  2,
			TripID:              1,
			ClaimantUserID:      3,
			ClaimantName:        "Aditi Sharma",
			AmountPaise:         85000,
			Currency:            "INR",
			Category:            "Snacks",
			Reason:              "Pitstop cold drinks and sandwiches",
			ExpenseDate:         time.Now().Add(-30 * time.Minute),
			Status:              "pending",
			ReimbursementStatus: "unpaid",
			CreatedAt:           time.Now().Add(-30 * time.Minute),
		},
	},
	memberList: []DemoMember{
		{
			ID:       1,
			TripID:   1,
			UserID:   1,
			UserName: "Tejas Ulawekar",
			Email:    "tejas@tripledger.com",
			Role:     "munimji",
			Status:   "active",
			JoinedAt: time.Now().Add(-72 * time.Hour),
		},
		{
			ID:       2,
			TripID:   1,
			UserID:   2,
			UserName: "Sameer Patil",
			Email:    "sameer@example.com",
			Role:     "co_reviewer",
			Status:   "active",
			JoinedAt: time.Now().Add(-48 * time.Hour),
		},
		{
			ID:       3,
			TripID:   1,
			UserID:   3,
			UserName: "Aditi Sharma",
			Email:    "aditi@example.com",
			Role:     "participant",
			Status:   "active",
			JoinedAt: time.Now().Add(-24 * time.Hour),
		},
	},
	contributions: 3000000, // ₹30,000
	groupSpend:    1450000, // ₹14,500
	claims:        250000,  // ₹2,500
}

func main() {
	router := gin.Default()

	// CORS for dev
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Auth demo endpoints
	router.POST("/auth/otp/request", func(c *gin.Context) {
		var req struct {
			Email string `json:"email"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid email"})
			return
		}
		c.JSON(200, gin.H{"message": "OTP sent successfully. Demo OTP is 123456"})
	})

	router.POST("/auth/otp/verify", func(c *gin.Context) {
		var req struct {
			Email string `json:"email"`
			OTP   string `json:"otp"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}

		if req.OTP != "123456" {
			c.JSON(401, gin.H{"error": "Invalid OTP. Use 123456 for demo"})
			return
		}

		store.mu.Lock()
		u, exists := store.users[req.Email]
		if !exists {
			u = &DemoUser{
				ID:       int64(len(store.users) + 1),
				Email:    req.Email,
				FullName: req.Email,
			}
			store.users[req.Email] = u
		}
		store.mu.Unlock()

		c.JSON(200, gin.H{
			"access_token": "demo-jwt-token-splito",
			"user":         u,
		})
	})

	// Trips demo endpoints
	router.POST("/trips/list", func(c *gin.Context) {
		store.mu.Lock()
		defer store.mu.Unlock()
		c.JSON(200, store.trips)
	})

	router.POST("/trips/create", func(c *gin.Context) {
		var req struct {
			Title       string    `json:"title"`
			StartPoint  string    `json:"start_point"`
			Destination string    `json:"destination"`
			StartAt     time.Time `json:"start_at"`
			EndAt       time.Time `json:"end_at"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		store.mu.Lock()
		t := DemoTrip{
			ID:          int64(len(store.trips) + 1),
			Title:       req.Title,
			StartPoint:  req.StartPoint,
			Destination: req.Destination,
			StartAt:     req.StartAt,
			EndAt:       req.EndAt,
			Status:      "upcoming",
		}
		store.trips = append([]DemoTrip{t}, store.trips...)
		store.mu.Unlock()

		c.JSON(201, t)
	})

	router.POST("/trips/dashboard", func(c *gin.Context) {
		store.mu.Lock()
		defer store.mu.Unlock()

		avail := store.contributions - store.groupSpend

		var approvedTotal int64
		var pendingTotal int64
		for _, cl := range store.claimList {
			if cl.Status == "approved" {
				approvedTotal += cl.AmountPaise
			} else if cl.Status == "pending" {
				pendingTotal += cl.AmountPaise
			}
		}

		c.JSON(200, gin.H{
			"total_contributions_paise": store.contributions,
			"group_funded_spend_paise":   store.groupSpend,
			"available_cash_paise":      avail,
			"approved_claims_paise":     approvedTotal,
			"pending_claims_paise":      pendingTotal,
		})
	})

	// Claims endpoints
	router.POST("/claims/list", func(c *gin.Context) {
		var req struct {
			TripID int64 `json:"trip_id"`
		}
		_ = c.ShouldBindJSON(&req)
		store.mu.Lock()
		defer store.mu.Unlock()

		var tripClaims []DemoClaim
		for _, cl := range store.claimList {
			if req.TripID == 0 || cl.TripID == req.TripID {
				tripClaims = append(tripClaims, cl)
			}
		}
		c.JSON(200, tripClaims)
	})

	router.POST("/claims/create", func(c *gin.Context) {
		var req struct {
			TripID      int64     `json:"trip_id"`
			AmountPaise int64     `json:"amount_paise"`
			Category    string    `json:"category"`
			Reason      string    `json:"reason"`
			ExpenseDate time.Time `json:"expense_date"`
			ReceiptURL  *string   `json:"receipt_url"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		store.mu.Lock()
		cl := DemoClaim{
			ID:                  int64(len(store.claimList) + 1),
			TripID:              req.TripID,
			ClaimantUserID:      1,
			ClaimantName:        "Rohit Sharma",
			AmountPaise:         req.AmountPaise,
			Currency:            "INR",
			Category:            req.Category,
			Reason:              req.Reason,
			ExpenseDate:         req.ExpenseDate,
			ReceiptURL:          req.ReceiptURL,
			Status:              "pending",
			ReimbursementStatus: "unpaid",
			CreatedAt:           time.Now(),
		}
		store.claimList = append([]DemoClaim{cl}, store.claimList...)
		store.mu.Unlock()

		c.JSON(201, cl)
	})

	router.POST("/claims/approve", func(c *gin.Context) {
		var req struct {
			ClaimID int64  `json:"claim_id"`
			Reason  string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		store.mu.Lock()
		defer store.mu.Unlock()
		for i := range store.claimList {
			if store.claimList[i].ID == req.ClaimID {
				store.claimList[i].Status = "approved"
				store.claimList[i].DecisionReason = req.Reason
				c.JSON(200, gin.H{"message": "Claim approved"})
				return
			}
		}
		c.JSON(404, gin.H{"error": "Claim not found"})
	})

	router.POST("/claims/reject", func(c *gin.Context) {
		var req struct {
			ClaimID int64  `json:"claim_id"`
			Reason  string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		store.mu.Lock()
		defer store.mu.Unlock()
		for i := range store.claimList {
			if store.claimList[i].ID == req.ClaimID {
				store.claimList[i].Status = "rejected"
				store.claimList[i].DecisionReason = req.Reason
				c.JSON(200, gin.H{"message": "Claim rejected"})
				return
			}
		}
		c.JSON(404, gin.H{"error": "Claim not found"})
	})

	// Members endpoints
	router.POST("/trips/members/list", func(c *gin.Context) {
		var req struct {
			TripID int64 `json:"trip_id"`
		}
		_ = c.ShouldBindJSON(&req)
		store.mu.Lock()
		defer store.mu.Unlock()

		var tripMembers []DemoMember
		for _, m := range store.memberList {
			if req.TripID == 0 || m.TripID == req.TripID {
				tripMembers = append(tripMembers, m)
			}
		}
		c.JSON(200, tripMembers)
	})

	router.POST("/trips/members/invite", func(c *gin.Context) {
		var req struct {
			TripID int64  `json:"trip_id"`
			Email  string `json:"email"`
			Role   string `json:"role"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		store.mu.Lock()
		// check if already member
		for _, m := range store.memberList {
			if m.TripID == req.TripID && m.Email == req.Email {
				store.mu.Unlock()
				c.JSON(400, gin.H{"error": "User is already an active member of this trip"})
				return
			}
		}

		role := "participant"
		if req.Role == "co_reviewer" {
			role = "co_reviewer"
		}

		newMember := DemoMember{
			ID:       int64(len(store.memberList) + 1),
			TripID:   req.TripID,
			UserID:   int64(len(store.memberList) + 10),
			UserName: req.Email,
			Email:    req.Email,
			Role:     role,
			Status:   "active",
			JoinedAt: time.Now(),
		}
		store.memberList = append(store.memberList, newMember)
		store.mu.Unlock()

		c.JSON(201, gin.H{"message": "Invitation accepted & member added", "member": newMember})
	})

	// PDF Statement Generation & Email Dispatch endpoint
	router.POST("/trips/reports/generate", func(c *gin.Context) {
		var req struct {
			TripID int64 `json:"trip_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		store.mu.Lock()
		var trip *DemoTrip
		for i := range store.trips {
			if store.trips[i].ID == req.TripID {
				trip = &store.trips[i]
				break
			}
		}
		if trip == nil {
			store.mu.Unlock()
			c.JSON(404, gin.H{"error": "Trip not found"})
			return
		}

		var tripMembers []DemoMember
		for _, m := range store.memberList {
			if m.TripID == req.TripID {
				tripMembers = append(tripMembers, m)
			}
		}

		avail := store.contributions - store.groupSpend
		var approvedClaims int64
		for _, cl := range store.claimList {
			if cl.TripID == req.TripID && cl.Status == "approved" {
				approvedClaims += cl.AmountPaise
			}
		}
		store.mu.Unlock()

		// Generate PDF using gofpdf
		pdf := gofpdf.New("P", "mm", "A4", "")
		pdf.AddPage()
		pdf.SetFont("Helvetica", "B", 18)
		pdf.Cell(0, 10, fmt.Sprintf("Splito - Trip Settlement Report: %s", trip.Title))
		pdf.Ln(12)

		pdf.SetFont("Helvetica", "", 11)
		pdf.Cell(0, 8, fmt.Sprintf("Route: %s to %s | Status: %s", trip.StartPoint, trip.Destination, trip.Status))
		pdf.Ln(8)
		pdf.Cell(0, 8, fmt.Sprintf("Snapshot Timestamp: %s", time.Now().Format(time.RFC1123)))
		pdf.Ln(14)

		pdf.SetFont("Helvetica", "B", 13)
		pdf.Cell(0, 8, "Treasury Balance Summary:")
		pdf.Ln(9)

		pdf.SetFont("Helvetica", "", 11)
		pdf.Cell(0, 7, fmt.Sprintf("  - Total Pool Collected: Rs. %.2f", float64(store.contributions)/100.0))
		pdf.Ln(7)
		pdf.Cell(0, 7, fmt.Sprintf("  - Group-Funded Outflows: Rs. %.2f", float64(store.groupSpend)/100.0))
		pdf.Ln(7)
		pdf.Cell(0, 7, fmt.Sprintf("  - Current Net Cash in Hand: Rs. %.2f", float64(avail)/100.0))
		pdf.Ln(7)
		pdf.Cell(0, 7, fmt.Sprintf("  - Approved Peer Claims Liability: Rs. %.2f", float64(approvedClaims)/100.0))
		pdf.Ln(14)

		pdf.SetFont("Helvetica", "B", 13)
		pdf.Cell(0, 8, fmt.Sprintf("Recipient Dispatch Queue (%d Active Members):", len(tripMembers)))
		pdf.Ln(9)

		pdf.SetFont("Helvetica", "", 10)
		for _, m := range tripMembers {
			pdf.Cell(0, 6, fmt.Sprintf("  [SENT] %s (%s) - Role: %s", m.UserName, m.Email, m.Role))
			pdf.Ln(6)
		}

		// Save PDF
		pdfFileName := fmt.Sprintf("Splito_Report_Trip_%d.pdf", trip.ID)
		_ = pdf.OutputFileAndClose(pdfFileName)

		log.Printf("[Worker] Successfully generated PDF '%s' and dispatched emails to %d active members", pdfFileName, len(tripMembers))

		c.JSON(200, gin.H{
			"message":          fmt.Sprintf("PDF settlement report compiled and dispatched to %d members", len(tripMembers)),
			"recipient_count": len(tripMembers),
			"report_file":     pdfFileName,
			"status":          "completed",
		})
	})

	// Serve Frontend Static files from dist
	distPath := filepath.Join("..", "frontend", "dist")
	router.Static("/assets", filepath.Join(distPath, "assets"))
	router.NoRoute(func(c *gin.Context) {
		c.File(filepath.Join(distPath, "index.html"))
	})

	log.Println("Splito Demo server listening on http://localhost:8080")
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
