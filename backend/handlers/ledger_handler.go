package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"tripledger/services"
	"tripledger/types"
)

type LedgerHandler struct {
	ledgerService *services.LedgerService
}

func NewLedgerHandler(ls *services.LedgerService) *LedgerHandler {
	return &LedgerHandler{ledgerService: ls}
}

func (h *LedgerHandler) CreateClaim(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.CreateClaimRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	claim, err := h.ledgerService.CreateClaim(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, claim)
}

func (h *LedgerHandler) ListClaims(c *gin.Context) {
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

	if err := h.ledgerService.EnsureActiveMembership(c.Request.Context(), req.TripID, userID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: you are not an active member of this expedition"})
		return
	}

	claims, err := h.ledgerService.ListClaims(c.Request.Context(), req.TripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, claims)
}

func (h *LedgerHandler) ApproveClaim(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.ClaimDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.ledgerService.DecideClaim(c.Request.Context(), userID, &req, true); err != nil {
		if errors.Is(err, services.ErrClaimConflict) || errors.Is(err, services.ErrTripCompletedNoOp) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, services.ErrNotReviewer) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Claim approved successfully"})
}

func (h *LedgerHandler) RejectClaim(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.ClaimDecisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.ledgerService.DecideClaim(c.Request.Context(), userID, &req, false); err != nil {
		if errors.Is(err, services.ErrClaimConflict) || errors.Is(err, services.ErrTripCompletedNoOp) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, services.ErrNotReviewer) {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Claim rejected successfully"})
}

func (h *LedgerHandler) RecordContribution(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.CreateContributionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	contrib, err := h.ledgerService.RecordContribution(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, contrib)
}

func (h *LedgerHandler) RecordExpense(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := userIDVal.(int64)

	var req types.CreateExpenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	exp, err := h.ledgerService.RecordExpense(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, exp)
}

func (h *LedgerHandler) GetDashboard(c *gin.Context) {
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

	if err := h.ledgerService.EnsureActiveMembership(c.Request.Context(), req.TripID, userID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: you are not an active member of this expedition"})
		return
	}

	summary, err := h.ledgerService.GetDashboard(c.Request.Context(), req.TripID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}

func (h *LedgerHandler) ListContributions(c *gin.Context) {
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

	if err := h.ledgerService.EnsureActiveMembership(c.Request.Context(), req.TripID, userID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: you are not an active member of this expedition"})
		return
	}

	contribs, err := h.ledgerService.ListContributions(c.Request.Context(), req.TripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, contribs)
}

func (h *LedgerHandler) ListExpenses(c *gin.Context) {
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

	if err := h.ledgerService.EnsureActiveMembership(c.Request.Context(), req.TripID, userID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied: you are not an active member of this expedition"})
		return
	}

	expenses, err := h.ledgerService.ListExpenses(c.Request.Context(), req.TripID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, expenses)
}

func (h *LedgerHandler) UploadReceipt(c *gin.Context) {
	file, err := c.FormFile("receipt")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No receipt file provided"})
		return
	}

	uploadDir := "./uploads"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare upload folder"})
		return
	}

	ext := filepath.Ext(file.Filename)
	safeName := fmt.Sprintf("receipt_%d%s", time.Now().UnixNano(), ext)
	dst := filepath.Join(uploadDir, safeName)

	if err := c.SaveUploadedFile(file, dst); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"receipt_path": fmt.Sprintf("/uploads/%s", safeName),
		"filename":     file.Filename,
	})
}

