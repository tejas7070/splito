package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"tripledger/loaders"
	"tripledger/models"
	"tripledger/services"
	"tripledger/types"
)

type TripHandler struct {
	tripService *services.TripService
	userLoader  *loaders.UserLoader
}

func NewTripHandler(ts *services.TripService, ul *loaders.UserLoader) *TripHandler {
	return &TripHandler{
		tripService: ts,
		userLoader:  ul,
	}
}

func (h *TripHandler) CreateTrip(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.CreateTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Server-side strict sanitization & validation (block SQL injection / XSS patterns)
	dangerousPattern := func(s string) bool {
		for _, ch := range []string{"<", ">", ";", "'", "\"", "--", "/*", "*/"} {
			if strings.Contains(s, ch) {
				return true
			}
		}
		return false
	}

	if dangerousPattern(req.Title) || dangerousPattern(req.StartPoint) || dangerousPattern(req.Destination) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Inputs contain disallowed special characters (< > ; ' \" -- /* */)"})
		return
	}

	trip, err := h.tripService.CreateTrip(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, trip)
}

func (h *TripHandler) ListTrips(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	trips, err := h.tripService.ListUserTrips(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, trips)
}

func (h *TripHandler) JoinTrip(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Trip membership handled via invitations"})
}

func (h *TripHandler) GetTrip(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.TripDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	trip, membership, err := h.tripService.GetTripDetail(c.Request.Context(), req.TripID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if trip == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Trip not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"trip":       trip,
		"membership": membership,
	})
}

func (h *TripHandler) ListMembers(c *gin.Context) {
	var req types.TripDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	members, err := h.tripService.ListMembers(c.Request.Context(), req.TripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, members)
}

func (h *TripHandler) InviteMember(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.InvitePeerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.tripService.InviteMember(c.Request.Context(), userID, &req, h.userLoader); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member invited successfully"})
}

func (h *TripHandler) ListEligibleUsers(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.SearchEligibleUsersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Verify requester has access to trip
	trip, member, err := h.tripService.GetTripDetail(c.Request.Context(), req.TripID, userID)
	if err != nil || trip == nil || member == nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to expedition"})
		return
	}

	users, err := h.userLoader.ListEligibleUsersForTrip(c.Request.Context(), req.TripID, req.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, users)
}

func (h *TripHandler) GenerateReport(c *gin.Context) {
	var req types.GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	fileName, pdfBytes, err := h.tripService.GenerateTripReport(c.Request.Context(), req.TripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	c.Data(http.StatusOK, "application/pdf", pdfBytes)
}

func (h *TripHandler) UpdateTripStatus(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.UpdateTripStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.tripService.UpdateTripStatus(c.Request.Context(), userID, req.TripID, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Trip status updated successfully"})
}

func (h *TripHandler) DeleteTrip(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.TripDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.tripService.DeleteTrip(c.Request.Context(), userID, req.TripID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Expedition deleted successfully"})
}

func (h *TripHandler) GetSettlement(c *gin.Context) {
	var req types.TripDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	settlement, err := h.tripService.GetSettlement(c.Request.Context(), req.TripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, settlement)
}

func (h *TripHandler) RespondInvitation(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.RespondInvitationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.tripService.RespondInvitation(c.Request.Context(), userID, req.TripID, req.Action); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg := "Invitation accepted! Welcome to the expedition."
	if req.Action == "reject" {
		msg = "Invitation declined."
	}
	c.JSON(http.StatusOK, gin.H{"message": msg})
}

func (h *TripHandler) ListInvitations(c *gin.Context) {
	var req types.TripDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	invitations, err := h.tripService.ListTripInvitations(c.Request.Context(), req.TripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if invitations == nil {
		invitations = []models.TripInvitation{}
	}

	c.JSON(http.StatusOK, invitations)
}

func (h *TripHandler) RemoveMember(c *gin.Context) {
	munimjiIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	munimjiID, ok := munimjiIDVal.(int64)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user ID"})
		return
	}

	var req models.RemoveMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.tripService.RemoveMember(c.Request.Context(), munimjiID, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Participant removed and notification email sent."})
}

func (h *TripHandler) ListMyPendingInvitations(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	user, err := h.userLoader.GetByID(c.Request.Context(), userID)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	invitations, err := h.tripService.ListUserPendingInvitations(c.Request.Context(), user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if invitations == nil {
		invitations = []models.TripInvitation{}
	}

	c.JSON(http.StatusOK, invitations)
}

func (h *TripHandler) UpdateMemberRole(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.UpdateMemberRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.tripService.UpdateMemberRole(c.Request.Context(), userID, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	roleName := "Co-Reviewer (Auditor)"
	if req.Role == "participant" {
		roleName = "Participant"
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Member role successfully updated to %s", roleName),
		"role":    req.Role,
	})
}


