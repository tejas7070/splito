package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"tripledger/services"
	"tripledger/types"
)

type AuthHandler struct {
	authService *services.AuthService
}

func NewAuthHandler(as *services.AuthService) *AuthHandler {
	return &AuthHandler{authService: as}
}

// setAuthCookie configures the session cookie dynamically (SameSite=None for HTTPS cross-site, Lax for local dev)
func (h *AuthHandler) setAuthCookie(c *gin.Context, token string, maxAge int) {
	secure := c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https"
	if secure {
		c.SetSameSite(http.SameSiteNoneMode)
	} else {
		c.SetSameSite(http.SameSiteLaxMode)
	}
	c.SetCookie("auth_token", token, maxAge, "/", "", secure, true)
}

// RequestRegisterOTP dispatches OTP to verify email for new user registration
func (h *AuthHandler) RequestRegisterOTP(c *gin.Context) {
	var req types.RegisterOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authService.RequestRegisterOTP(c.Request.Context(), req.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Registration verification code dispatched to your email."})
}

// Register verifies OTP and creates user with name and password
func (h *AuthHandler) Register(c *gin.Context) {
	var req types.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.Register(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set HttpOnly cookie (SameSite=None when HTTPS for cross-domain Vercel frontend)
	h.setAuthCookie(c, resp.AccessToken, 28800)
	c.JSON(http.StatusCreated, resp)
}

// Login verifies email and password and issues JWT
func (h *AuthHandler) Login(c *gin.Context) {
	var req types.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	clientIP := c.ClientIP()
	cleanEmail := strings.ToLower(strings.TrimSpace(req.Email))

	// Check rate limits for both email and IP
	okEmail, waitEmail := h.authService.CheckLoginRateLimit("login:email:" + cleanEmail)
	if !okEmail {
		c.Header("Retry-After", strconv.Itoa(waitEmail))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many failed login attempts. Please try again later."})
		return
	}

	okIP, waitIP := h.authService.CheckLoginRateLimit("login:ip:" + clientIP)
	if !okIP {
		c.Header("Retry-After", strconv.Itoa(waitIP))
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Too many failed login attempts from this network. Please try again later."})
		return
	}

	resp, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		h.authService.RecordLoginFailure("login:email:" + cleanEmail)
		h.authService.RecordLoginFailure("login:ip:" + clientIP)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Reset failure counters on successful login
	h.authService.ResetLoginFailure("login:email:" + cleanEmail)
	h.authService.ResetLoginFailure("login:ip:" + clientIP)

	// Set HttpOnly cookie (SameSite=None when HTTPS for cross-domain Vercel frontend)
	h.setAuthCookie(c, resp.AccessToken, 28800)
	c.JSON(http.StatusOK, resp)
}

// GoogleAuth handles Google ID Token authentication & automatic registration
func (h *AuthHandler) GoogleAuth(c *gin.Context) {
	var req types.GoogleAuthRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Google credential token is required"})
		return
	}

	resp, err := h.authService.GoogleAuth(c.Request.Context(), req.Credential)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set HttpOnly cookie (SameSite=None when HTTPS for cross-domain Vercel frontend)
	h.setAuthCookie(c, resp.AccessToken, 28800)
	c.JSON(http.StatusOK, resp)
}

// GetCurrentUser returns the profile of the authenticated user
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	user, err := h.authService.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateProfile updates the authenticated user's name and phone
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req types.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.authService.UpdateProfile(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// RequestPasswordChangeOTP dispatches an OTP to user's registered phone / email for secure password change
func (h *AuthHandler) RequestPasswordChangeOTP(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	resp, err := h.authService.RequestPasswordChangeOTP(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ChangePassword changes the authenticated user's password
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req types.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authService.ChangePassword(c.Request.Context(), userID, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

// Logout clears the auth cookie
func (h *AuthHandler) Logout(c *gin.Context) {
	h.setAuthCookie(c, "", -1)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

// RequestForgotPasswordOTP sends OTP for password reset
func (h *AuthHandler) RequestForgotPasswordOTP(c *gin.Context) {
	var req types.ForgotPasswordOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authService.RequestForgotPasswordOTP(c.Request.Context(), req.Email); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password reset verification code dispatched to your email."})
}

// ResetPassword verifies OTP and updates password
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req types.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.ResetPassword(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Password reset successfully.",
		"access_token": resp.AccessToken,
		"user":         resp.User,
	})
}

// Legacy RequestOTP
func (h *AuthHandler) RequestOTP(c *gin.Context) {
	var req types.SendOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.authService.RequestOTP(c.Request.Context(), req.Email); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OTP sent successfully"})
}

// Legacy VerifyOTP
func (h *AuthHandler) VerifyOTP(c *gin.Context) {
	var req types.VerifyOTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.authService.VerifyOTP(c.Request.Context(), req.Email, req.OTP)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
