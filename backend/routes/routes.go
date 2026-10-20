package routes

import (
	"github.com/gin-gonic/gin"
	"tripledger/config"
	"tripledger/handlers"
	"tripledger/middlewares"
)

func RegisterRoutes(
	r *gin.Engine,
	cfg *config.Config,
	authHandler *handlers.AuthHandler,
	tripHandler *handlers.TripHandler,
	ledgerHandler *handlers.LedgerHandler,
) {
	// CORS with exact origin allowlist (never '*') and credentials allowed
	r.Use(middlewares.CORSMiddleware(cfg))

	// CSRF Protection on all mutating endpoints
	r.Use(middlewares.CSRFProtectionMiddleware(cfg))

	// Public auth routes
	auth := r.Group("/auth")
	{
		auth.POST("/register/otp", authHandler.RequestRegisterOTP)
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/google", authHandler.GoogleAuth)
		auth.POST("/forgot-password/otp", authHandler.RequestForgotPasswordOTP)
		auth.POST("/forgot-password/reset", authHandler.ResetPassword)

		// Legacy
		auth.POST("/otp/request", authHandler.RequestOTP)
		auth.POST("/otp/verify", authHandler.VerifyOTP)
	}

	// Protected routes (All IDs passed strictly in request body JSON)
	protected := r.Group("")
	protected.Use(middlewares.AuthMiddleware(cfg))
	{
		// User & Profile
		userAuth := protected.Group("/auth")
		{
			userAuth.GET("/me", authHandler.GetCurrentUser)
			userAuth.POST("/me", authHandler.GetCurrentUser)
			userAuth.POST("/profile/update", authHandler.UpdateProfile)
			userAuth.POST("/password/otp", authHandler.RequestPasswordChangeOTP)
			userAuth.POST("/password/change", authHandler.ChangePassword)
			userAuth.POST("/logout", authHandler.Logout)
		}

		// Trips
		trips := protected.Group("/trips")
		{
			trips.POST("/create", tripHandler.CreateTrip)
			trips.POST("/list", tripHandler.ListTrips)
			trips.POST("/detail", tripHandler.GetTrip)
			trips.POST("/join", tripHandler.JoinTrip)
			trips.POST("/dashboard", ledgerHandler.GetDashboard)
			trips.POST("/members/list", tripHandler.ListMembers)
			trips.POST("/members/invite", tripHandler.InviteMember)
			trips.POST("/members/eligible", tripHandler.ListEligibleUsers)
			trips.POST("/members/role/update", tripHandler.UpdateMemberRole)
			trips.POST("/members/remove", tripHandler.RemoveMember)
			trips.POST("/reports/generate", tripHandler.GenerateReport)
			trips.POST("/status/update", tripHandler.UpdateTripStatus)
			trips.POST("/delete", tripHandler.DeleteTrip)
			trips.POST("/settlement/calculate", tripHandler.GetSettlement)
			trips.POST("/invitation/respond", tripHandler.RespondInvitation)
			trips.POST("/invitations/list", tripHandler.ListInvitations)
			trips.POST("/invitations/my-pending", tripHandler.ListMyPendingInvitations)
		}


		// Claims
		claims := protected.Group("/claims")
		{
			claims.POST("/create", ledgerHandler.CreateClaim)
			claims.POST("/list", ledgerHandler.ListClaims)
			claims.POST("/approve", ledgerHandler.ApproveClaim)
			claims.POST("/reject", ledgerHandler.RejectClaim)
		}

		// Accounting (Contributions & Group Expenses)
		accounting := protected.Group("/accounting")
		{
			accounting.POST("/contributions/record", ledgerHandler.RecordContribution)
			accounting.POST("/contributions/list", ledgerHandler.ListContributions)
			accounting.POST("/expenses/record", ledgerHandler.RecordExpense)
			accounting.POST("/expenses/list", ledgerHandler.ListExpenses)
		}

		// File uploads for receipts
		protected.POST("/uploads/receipt", ledgerHandler.UploadReceipt)
	}

	// Serve uploaded receipts statically
	uploadDir := "./uploads"
	if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
		if _, err := os.Stat("backend/uploads"); err == nil {
			uploadDir = "backend/uploads"
		}
	}
	r.Static("/uploads", uploadDir)
}
