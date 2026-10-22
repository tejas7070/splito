package services

import (
	"bytes"
	"io"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"tripledger/config"
	"tripledger/loaders"
	"tripledger/middlewares"
	"tripledger/models"
	"tripledger/types"
	"tripledger/writers"
)

type otpItem struct {
	hashedCode     string
	expiresAt      time.Time
	verifyAttempts int
}

type rateLimitEntry struct {
	lastSentAt  time.Time
	requests    []time.Time
	lockedUntil time.Time
}

type otpStore struct {
	mu         sync.Mutex
	otps       map[string]*otpItem        // key e.g. "purpose:identifier"
	rateLimits map[string]*rateLimitEntry // key e.g. "register:email" or "forgot:email"
	loginFails map[string]*rateLimitEntry // key e.g. "login:email" or "login:ip"
}

var localOTPs = &otpStore{
	otps:       make(map[string]*otpItem),
	rateLimits: make(map[string]*rateLimitEntry),
	loginFails: make(map[string]*rateLimitEntry),
}

type AuthService struct {
	db         *sql.DB
	cfg        *config.Config
	userLoader *loaders.UserLoader
	userWriter *writers.UserWriter
}

func NewAuthService(db *sql.DB, cfg *config.Config, ul *loaders.UserLoader, uw *writers.UserWriter) *AuthService {
	return &AuthService{
		db:         db,
		cfg:        cfg,
		userLoader: ul,
		userWriter: uw,
	}
}

// getIntParam retrieves a dynamic system parameter from custom_parameters table with a default fallback
func (s *AuthService) getIntParam(key string, defaultVal int) int {
	if s.db == nil {
		return defaultVal
	}
	var valStr string
	err := s.db.QueryRow("SELECT param_value FROM custom_parameters WHERE param_key = ?", key).Scan(&valStr)
	if err != nil {
		return defaultVal
	}
	val, err := strconv.Atoi(strings.TrimSpace(valStr))
	if err != nil || val <= 0 {
		return defaultVal
	}
	return val
}

// checkRateLimit enforces CERT-In compliant rate limiting:
// 1. Minimum 30s cooldown between requests
// 2. Max 5 attempts within 15 minutes. If exceeded, locks out for 15 minutes.
func (s *AuthService) checkRateLimit(key string) error {
	cooldownSec := s.getIntParam("otp_cooldown_seconds", 30)
	maxAttempts := s.getIntParam("otp_max_attempts", 5)
	windowMinutes := s.getIntParam("otp_window_minutes", 15)
	lockoutMinutes := s.getIntParam("otp_lockout_minutes", 15)

	localOTPs.mu.Lock()
	defer localOTPs.mu.Unlock()

	now := time.Now()
	entry, exists := localOTPs.rateLimits[key]
	if !exists {
		entry = &rateLimitEntry{}
		localOTPs.rateLimits[key] = entry
	}

	// 1. Check if currently locked out
	if now.Before(entry.lockedUntil) {
		remaining := time.Until(entry.lockedUntil).Round(time.Second)
		mins := int(remaining.Minutes())
		secs := int(remaining.Seconds()) % 60
		return fmt.Errorf("too many OTP requests. Account locked for %dm %ds", mins, secs)
	}

	// 2. Check 30-second cooldown
	if !entry.lastSentAt.IsZero() {
		cooldownDuration := time.Duration(cooldownSec) * time.Second
		if time.Since(entry.lastSentAt) < cooldownDuration {
			waitSecs := int((cooldownDuration - time.Since(entry.lastSentAt)).Seconds()) + 1
			return fmt.Errorf("please wait %d seconds before requesting another OTP", waitSecs)
		}
	}

	// 3. Sliding window check (5 attempts in 15 mins)
	windowDuration := time.Duration(windowMinutes) * time.Minute
	var validRequests []time.Time
	for _, t := range entry.requests {
		if now.Sub(t) < windowDuration {
			validRequests = append(validRequests, t)
		}
	}

	if len(validRequests) >= maxAttempts {
		entry.lockedUntil = now.Add(time.Duration(lockoutMinutes) * time.Minute)
		entry.requests = nil
		return fmt.Errorf("maximum OTP limit reached (5 in 15 mins). Service locked for 15 minutes")
	}

	// Record new attempt
	validRequests = append(validRequests, now)
	entry.requests = validRequests
	entry.lastSentAt = now

	return nil
}

func (s *AuthService) hashOTP(code string) string {
	secret := s.cfg.JWTSecret
	if secret == "" {
		secret = "splito-default-otp-secret-key"
	}
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(code))
	return hex.EncodeToString(h.Sum(nil))
}

func (s *AuthService) compareOTP(submittedCode, expectedHash string) bool {
	computed := s.hashOTP(submittedCode)
	return hmac.Equal([]byte(computed), []byte(expectedHash))
}

func generateOTP() (string, error) {
	codeNum, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return "", fmt.Errorf("failed to generate secure OTP: %w", err)
	}
	return fmt.Sprintf("%06d", codeNum.Int64()+100000), nil
}

func (s *AuthService) generateToken(user *models.User) (string, error) {
	claims := middlewares.UserClaims{
		UserID: user.ID,
		Email:  user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *AuthService) dispatchOTPEmail(toEmail, code, subject, title, badge, description string) {
	if s.cfg.ResendAPIKey == "" && s.cfg.BrevoAPIKey == "" && (s.cfg.SMTPHost == "" || s.cfg.SMTPUser == "") {
		return
	}
	go func() {
		from := s.cfg.SMTPFrom
		if from == "" {
			from = s.cfg.SMTPUser
		}

		auth := smtp.PlainAuth("", s.cfg.SMTPUser, s.cfg.SMTPPass, s.cfg.SMTPHost)
		addr := fmt.Sprintf("%s:%s", s.cfg.SMTPHost, s.cfg.SMTPPort)

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
        <table role="presentation" width="100%%" style="max-width:500px;background-color:#ffffff;border-radius:16px;border:1px solid #e2e8f0;overflow:hidden;box-shadow:0 4px 20px rgba(0,0,0,0.06);">
          <tr>
            <td style="background-color:#0f172a;padding:28px 32px;text-align:left;">
              <div style="display:inline-block;">
                <span style="color:#ffffff;font-size:24px;font-weight:800;letter-spacing:-0.5px;">Splito</span>
                <span style="color:#38bdf8;font-size:12px;margin-left:10px;font-weight:700;text-transform:uppercase;letter-spacing:1px;background:rgba(56,189,248,0.18);padding:3px 8px;border-radius:6px;">%s</span>
              </div>
              <p style="margin:6px 0 0 0;color:#94a3b8;font-size:12px;letter-spacing:0.3px;">Expedition Treasury Protocol</p>
            </td>
          </tr>
          <tr>
            <td style="padding:32px 32px 24px 32px;">
              <h1 style="margin:0 0 10px 0;font-size:19px;font-weight:700;color:#0f172a;">%s</h1>
              <p style="margin:0 0 22px 0;font-size:14px;line-height:1.6;color:#475569;">
                %s
              </p>
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="margin:0 0 22px 0;">
                <tr>
                  <td align="center" style="background:#f8fafc;border:2px dashed #cbd5e1;border-radius:12px;padding:20px 16px;">
                    <div style="font-family:'SF Pro Mono','Geist Mono',Consolas,Monaco,monospace;font-size:36px;font-weight:800;letter-spacing:8px;color:#0f172a;text-indent:8px;">
                      %s
                    </div>
                    <div style="margin-top:8px;font-size:12px;font-weight:600;color:#64748b;">
                      Valid for 10 minutes
                    </div>
                  </td>
                </tr>
              </table>
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#f8fafc;border-radius:8px;padding:12px 16px;margin-bottom:20px;">
                <tr>
                  <td style="font-size:12px;color:#64748b;line-height:1.5;">
                    <strong style="color:#334155;">Security Notice:</strong> Do not share this code with anyone. Splito support or Munimjis will never ask for your verification code.
                  </td>
                </tr>
              </table>
              <p style="margin:0;font-size:12px;color:#94a3b8;line-height:1.5;">
                If you did not request this code, you can safely disregard this email.
              </p>
            </td>
          </tr>
          <tr>
            <td style="background-color:#f8fafc;border-top:1px solid #f1f5f9;padding:16px 32px;text-align:center;">
              <p style="margin:0;font-size:11px;color:#94a3b8;">
                Splito • Trip expenses, split with zero drama.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, title, badge, title, description, code)

		headers := fmt.Sprintf("MIME-Version: 1.0\r\n"+
			"From: Splito <%s>\r\n"+
			"To: %s\r\n"+
			"Subject: %s\r\n"+
			"Content-Type: text/html; charset=UTF-8\r\n\r\n", from, toEmail, subject)

		// 1. Try Brevo HTTP API (Port 443 - 100% unblocked on Render Free tier, sends to ANY recipient without custom domain)
		cleanBrevoKey := strings.TrimSpace(s.cfg.BrevoAPIKey)
		if cleanBrevoKey != "" {
			brevoSender := strings.TrimSpace(s.cfg.SMTPFrom)
			if brevoSender == "" || strings.Contains(brevoSender, "resend.dev") {
				brevoSender = strings.TrimSpace(s.cfg.SMTPUser)
			}
			if brevoSender == "" {
				brevoSender = "tejasulawekar@gmail.com"
			}

			brevoPayload := map[string]interface{}{
				"sender": map[string]string{
					"name":  "Splito",
					"email": brevoSender,
				},
				"to": []map[string]string{
					{"email": toEmail},
				},
				"subject":     subject,
				"htmlContent": htmlContent,
			}
			jsonBytes, err := json.Marshal(brevoPayload)
			if err == nil {
				req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", bytes.NewBuffer(jsonBytes))
				if err == nil {
					req.Header.Set("accept", "application/json")
					req.Header.Set("api-key", cleanBrevoKey)
					req.Header.Set("Content-Type", "application/json")
					client := &http.Client{Timeout: 10 * time.Second}
					resp, err := client.Do(req)
					if err == nil {
						bodyBytes, _ := io.ReadAll(resp.Body)
						resp.Body.Close()
						if resp.StatusCode >= 200 && resp.StatusCode < 300 {
							fmt.Printf("[SPLITO EMAIL SUCCESS] Dispatched OTP to %s via Brevo API\n", toEmail)
							return
						}
						fmt.Printf("[SPLITO BREVO ERROR] HTTP %d: %s\n", resp.StatusCode, string(bodyBytes))
					} else {
						fmt.Printf("[SPLITO BREVO ERROR] %v\n", err)
					}
				}
			}
		}

		// 2. Try Resend HTTP API (Port 443 - free sandbox delivers strictly to account owner)
		cleanResendKey := strings.TrimSpace(s.cfg.ResendAPIKey)
		if cleanResendKey != "" {
			// Resend requires onboarding@resend.dev unless a custom verified domain is used (never accepts @gmail.com)
			sender := "Splito <onboarding@resend.dev>"
			cleanFrom := strings.ToLower(strings.TrimSpace(s.cfg.SMTPFrom))
			if cleanFrom != "" && strings.Contains(cleanFrom, "@") && !strings.HasSuffix(cleanFrom, "@gmail.com") && !strings.HasSuffix(cleanFrom, "@yahoo.com") && !strings.Contains(cleanFrom, "resend.dev") {
				sender = s.cfg.SMTPFrom
			}
			resendPayload := map[string]interface{}{
				"from":    sender,
				"to":      []string{toEmail},
				"subject": subject,
				"html":    htmlContent,
			}
			jsonBytes, err := json.Marshal(resendPayload)
			if err == nil {
				req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewBuffer(jsonBytes))
				if err == nil {
					req.Header.Set("Authorization", "Bearer "+cleanResendKey)
					req.Header.Set("Content-Type", "application/json")
					client := &http.Client{Timeout: 10 * time.Second}
					resp, err := client.Do(req)
					if err == nil {
						bodyBytes, _ := io.ReadAll(resp.Body)
						resp.Body.Close()
						if resp.StatusCode >= 200 && resp.StatusCode < 300 {
							fmt.Printf("[SPLITO EMAIL SUCCESS] Dispatched OTP to %s via Resend API\n", toEmail)
							return
						}
						fmt.Printf("[SPLITO RESEND ERROR] HTTP %d: %s\n", resp.StatusCode, string(bodyBytes))
					} else {
						fmt.Printf("[SPLITO RESEND ERROR] %v\n", err)
					}
				}
			}
		}

		// 3. Fallback to standard SMTP (Port 587 - works on localhost)
		if s.cfg.SMTPHost != "" && s.cfg.SMTPUser != "" {
			msg := []byte(headers + htmlContent)
			if err := smtp.SendMail(addr, auth, from, []string{toEmail}, msg); err != nil {
				fmt.Printf("[SPLITO SMTP ERROR] Failed to send email to %s: %v (Note: Render Free Tier blocks outbound ports 25, 465, and 587; set RESEND_API_KEY or BREVO_API_KEY on Render for instant HTTPS delivery)\n", toEmail, err)
			} else {
				fmt.Printf("[SPLITO SMTP SUCCESS] Verification code dispatched to %s (%s)\n", toEmail, badge)
			}
		}
	}()
}

// RequestRegisterOTP checks if email is free, enforces dynamic rate limiting, generates OTP and sends email
func (s *AuthService) RequestRegisterOTP(ctx context.Context, email string) error {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	if cleanEmail == "" {
		return errors.New("email is required")
	}

	// 1. Enforce rate limiting from custom_parameters
	if err := s.checkRateLimit("reg:" + cleanEmail); err != nil {
		return err
	}

	existing, err := s.userLoader.GetByEmail(ctx, cleanEmail)
	if err != nil {
		return err
	}
	if existing != nil {
		return errors.New("an account with this email already exists, please sign in")
	}

	code, err := generateOTP()
	if err != nil {
		return err
	}

	key := "register:" + cleanEmail
	expiryMin := s.getIntParam("otp_expiry_minutes", 10)
	localOTPs.mu.Lock()
	localOTPs.otps[key] = &otpItem{
		hashedCode: s.hashOTP(code),
		expiresAt:  time.Now().Add(time.Duration(expiryMin) * time.Minute),
	}
	localOTPs.mu.Unlock()

	fmt.Printf("\n========================================\n[SPLITO REGISTER OTP] Email: %s -> OTP: %s (Expires: 10m)\n========================================\n\n", cleanEmail, code)

	s.dispatchOTPEmail(
		cleanEmail, code,
		fmt.Sprintf("%s is your Splito registration code", code),
		"Verify your email address",
		"Registration",
		"Welcome to Splito! Please use the 6-digit verification code below to verify your email and complete your account registration:",
	)

	return nil
}

// Register verifies registration OTP, hashes password with bcrypt, creates user and issues JWT
func (s *AuthService) Register(ctx context.Context, req *types.RegisterRequest) (*types.AuthResponse, error) {
	cleanEmail := strings.ToLower(strings.TrimSpace(req.Email))
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	if len(req.Password) < 6 {
		return nil, errors.New("password must be at least 6 characters")
	}

	key := "register:" + cleanEmail
	localOTPs.mu.Lock()
	item, exists := localOTPs.otps[key]
	if !exists {
		localOTPs.mu.Unlock()
		return nil, errors.New("no verification code found or it has expired, please request a new one")
	}

	if time.Now().After(item.expiresAt) {
		delete(localOTPs.otps, key)
		localOTPs.mu.Unlock()
		return nil, errors.New("verification code has expired, please request a new one")
	}

	maxVerifyAttempts := s.getIntParam("otp_max_verify_attempts", 5)
	if !s.compareOTP(req.OTP, item.hashedCode) {
		item.verifyAttempts++
		if item.verifyAttempts >= maxVerifyAttempts {
			delete(localOTPs.otps, key)
			localOTPs.mu.Unlock()
			return nil, errors.New("too many incorrect attempts. This OTP has been invalidated, please request a new code")
		}
		remaining := maxVerifyAttempts - item.verifyAttempts
		localOTPs.mu.Unlock()
		return nil, fmt.Errorf("invalid verification code (%d attempt(s) remaining)", remaining)
	}

	// Code matched! Delete OTP
	delete(localOTPs.otps, key)
	localOTPs.mu.Unlock()

	existing, err := s.userLoader.GetByEmail(ctx, cleanEmail)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errors.New("an account with this email already exists, please sign in")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := s.userWriter.CreateUser(ctx, cleanEmail, name, string(hash))
	if err != nil {
		return nil, err
	}

	tokenString, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	return &types.AuthResponse{
		AccessToken: tokenString,
		User:        user,
	}, nil
}

// CheckLoginRateLimit checks if login attempts for email or IP exceed limits
func (s *AuthService) CheckLoginRateLimit(key string) (bool, int) {
	maxFailures := s.getIntParam("login_max_failures", 5)
	lockoutMin := s.getIntParam("login_lockout_minutes", 15)

	localOTPs.mu.Lock()
	defer localOTPs.mu.Unlock()

	entry, exists := localOTPs.loginFails[key]
	if !exists {
		return true, 0
	}

	now := time.Now()
	if !entry.lockedUntil.IsZero() && now.Before(entry.lockedUntil) {
		remainingSec := int(entry.lockedUntil.Sub(now).Seconds())
		return false, remainingSec
	}

	windowDuration := time.Duration(lockoutMin) * time.Minute
	var validAttempts []time.Time
	for _, t := range entry.requests {
		if now.Sub(t) < windowDuration {
			validAttempts = append(validAttempts, t)
		}
	}
	entry.requests = validAttempts

	if len(validAttempts) >= maxFailures {
		entry.lockedUntil = now.Add(windowDuration)
		entry.requests = nil
		return false, lockoutMin * 60
	}

	return true, 0
}

func (s *AuthService) RecordLoginFailure(key string) {
	localOTPs.mu.Lock()
	defer localOTPs.mu.Unlock()

	entry, exists := localOTPs.loginFails[key]
	if !exists {
		entry = &rateLimitEntry{}
		localOTPs.loginFails[key] = entry
	}
	entry.requests = append(entry.requests, time.Now())
}

func (s *AuthService) ResetLoginFailure(key string) {
	localOTPs.mu.Lock()
	defer localOTPs.mu.Unlock()
	delete(localOTPs.loginFails, key)
}

// Login verifies email and password against bcrypt hash and issues JWT
func (s *AuthService) Login(ctx context.Context, email, password string) (*types.AuthResponse, error) {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	user, err := s.userLoader.GetByEmail(ctx, cleanEmail)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("invalid email or password")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		// Migration helper for placeholder demo accounts
		if user.PasswordHash == "argon2_demo_placeholder" && (password == "password123" || password == "admin123") {
			newHash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			_ = s.userWriter.UpdatePassword(ctx, user.ID, string(newHash))
		} else {
			return nil, errors.New("invalid email or password")
		}
	}

	tokenString, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	return &types.AuthResponse{
		AccessToken: tokenString,
		User:        user,
	}, nil
}

// RequestForgotPasswordOTP checks if user exists, generates OTP and sends reset email
func (s *AuthService) RequestForgotPasswordOTP(ctx context.Context, email string) error {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))

	// 1. Enforce rate limiting from custom_parameters
	if err := s.checkRateLimit("forgot:" + cleanEmail); err != nil {
		return err
	}

	user, err := s.userLoader.GetByEmail(ctx, cleanEmail)
	if err != nil {
		return err
	}
	if user == nil {
		// Generic response: do not reveal whether email exists
		return nil
	}

	code, err := generateOTP()
	if err != nil {
		return err
	}

	key := "forgot_password:" + cleanEmail
	expiryMin := s.getIntParam("otp_expiry_minutes", 10)
	localOTPs.mu.Lock()
	localOTPs.otps[key] = &otpItem{
		hashedCode: s.hashOTP(code),
		expiresAt:  time.Now().Add(time.Duration(expiryMin) * time.Minute),
	}
	localOTPs.mu.Unlock()

	fmt.Printf("\n========================================\n[SPLITO FORGOT PASSWORD OTP] Email: %s -> OTP: %s (Expires: 10m)\n========================================\n\n", cleanEmail, code)

	s.dispatchOTPEmail(
		cleanEmail, code,
		fmt.Sprintf("%s is your Splito password reset code", code),
		"Reset your password",
		"Security",
		"We received a request to reset your Splito password. Use the verification code below to set a new password for your account:",
	)

	return nil
}

// ResetPassword verifies forgot-password OTP, hashes new password with bcrypt, updates DB and issues JWT
func (s *AuthService) ResetPassword(ctx context.Context, req *types.ResetPasswordRequest) (*types.AuthResponse, error) {
	cleanEmail := strings.ToLower(strings.TrimSpace(req.Email))
	if len(req.NewPassword) < 6 {
		return nil, errors.New("new password must be at least 6 characters")
	}

	key := "forgot_password:" + cleanEmail
	localOTPs.mu.Lock()
	item, exists := localOTPs.otps[key]
	if !exists {
		localOTPs.mu.Unlock()
		return nil, errors.New("no password reset code found or it has expired, please request a new one")
	}

	if time.Now().After(item.expiresAt) {
		delete(localOTPs.otps, key)
		localOTPs.mu.Unlock()
		return nil, errors.New("password reset code has expired, please request a new one")
	}

	maxVerifyAttempts := s.getIntParam("otp_max_verify_attempts", 5)
	if !s.compareOTP(req.OTP, item.hashedCode) {
		item.verifyAttempts++
		if item.verifyAttempts >= maxVerifyAttempts {
			delete(localOTPs.otps, key)
			localOTPs.mu.Unlock()
			return nil, errors.New("too many incorrect attempts. This OTP has been invalidated, please request a new code")
		}
		remaining := maxVerifyAttempts - item.verifyAttempts
		localOTPs.mu.Unlock()
		return nil, fmt.Errorf("invalid verification code (%d attempt(s) remaining)", remaining)
	}

	// Code matched! Delete OTP
	delete(localOTPs.otps, key)
	localOTPs.mu.Unlock()

	user, err := s.userLoader.GetByEmail(ctx, cleanEmail)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.userWriter.UpdatePassword(ctx, user.ID, string(hash)); err != nil {
		return nil, err
	}

	tokenString, err := s.generateToken(user)
	if err != nil {
		return nil, err
	}

	return &types.AuthResponse{
		AccessToken: tokenString,
		User:        user,
	}, nil
}

// Legacy RequestOTP generates OTP and sends email
func (s *AuthService) RequestOTP(ctx context.Context, email string) error {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	code, err := generateOTP()
	if err != nil {
		return err
	}

	localOTPs.mu.Lock()
	localOTPs.otps[cleanEmail] = &otpItem{
		hashedCode: s.hashOTP(code),
		expiresAt:  time.Now().Add(10 * time.Minute),
	}
	localOTPs.mu.Unlock()

	fmt.Printf("\n========================================\n[SPLITO AUTH] Email: %s -> OTP: %s (Expires: 10m)\n========================================\n\n", cleanEmail, code)

	s.dispatchOTPEmail(
		cleanEmail, code,
		fmt.Sprintf("%s is your Splito verification code", code),
		"Sign in to your account",
		"Auth",
		"Here is your single-use verification code to securely access your trip manifest and ledger:",
	)

	return nil
}

// Legacy VerifyOTP checks code, creates user if not existing, and issues JWT
func (s *AuthService) VerifyOTP(ctx context.Context, email, code string) (*types.AuthResponse, error) {
	cleanEmail := strings.ToLower(strings.TrimSpace(email))
	localOTPs.mu.Lock()
	item, exists := localOTPs.otps[cleanEmail]
	if !exists {
		localOTPs.mu.Unlock()
		return nil, fmt.Errorf("invalid verification code")
	}

	if time.Now().After(item.expiresAt) {
		delete(localOTPs.otps, cleanEmail)
		localOTPs.mu.Unlock()
		return nil, fmt.Errorf("verification code has expired, please request a new one")
	}

	if !s.compareOTP(code, item.hashedCode) {
		localOTPs.mu.Unlock()
		return nil, fmt.Errorf("invalid verification code")
	}

	delete(localOTPs.otps, cleanEmail)
	localOTPs.mu.Unlock()

	user, err := s.userLoader.GetByEmail(ctx, cleanEmail)
	if err != nil {
		return nil, err
	}

	if user == nil {
		defaultName := strings.Split(cleanEmail, "@")[0]
		user, err = s.userWriter.CreateUser(ctx, cleanEmail, defaultName, "argon2_demo_placeholder")
		if err != nil {
			return nil, err
		}
	}

	tokenString, err := s.generateToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	return &types.AuthResponse{
		AccessToken: tokenString,
		User:        user,
	}, nil
}

type GoogleTokenInfo struct {
	Iss           string `json:"iss"`
	Sub           string `json:"sub"`
	Aud           string `json:"aud"`
	Email         string `json:"email"`
	EmailVerified any    `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	ErrorDesc     string `json:"error_description"`
}

// GoogleAuth verifies a Google credential, registers the user if new, and issues an access token
func (s *AuthService) GoogleAuth(ctx context.Context, credential string) (*types.AuthResponse, error) {
	trimmedCred := strings.TrimSpace(credential)
	if trimmedCred == "" {
		return nil, errors.New("google credential is required")
	}

	// 1. Dev / local simulation fallback
	if strings.HasPrefix(trimmedCred, "dev_google:") {
		payload := strings.TrimPrefix(trimmedCred, "dev_google:")
		var devInfo struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		}
		if err := json.Unmarshal([]byte(payload), &devInfo); err == nil && devInfo.Email != "" {
			cleanEmail := strings.ToLower(strings.TrimSpace(devInfo.Email))
			name := strings.TrimSpace(devInfo.Name)
			if name == "" {
				name = strings.Split(cleanEmail, "@")[0]
			}
			return s.findOrCreateGoogleUser(ctx, cleanEmail, name)
		}
	}

	// 2. Official Google tokeninfo / userinfo verification
	var tokenInfo GoogleTokenInfo
	var verified bool

	// Try ID token verification first
	verifyURL := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", url.QueryEscape(trimmedCred))
	req, err := http.NewRequestWithContext(ctx, "GET", verifyURL, nil)
	if err == nil {
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err == nil && tokenInfo.Email != "" {
				verified = true
			}
			resp.Body.Close()
		} else if resp != nil {
			resp.Body.Close()
		}
	}

	// If not an ID token, verify as Google OAuth2 Access Token via userinfo endpoint
	if !verified {
		userinfoURL := "https://www.googleapis.com/oauth2/v3/userinfo"
		req, err := http.NewRequestWithContext(ctx, "GET", userinfoURL, nil)
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+trimmedCred)
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err == nil && resp.StatusCode == http.StatusOK {
				if err := json.NewDecoder(resp.Body).Decode(&tokenInfo); err == nil && tokenInfo.Email != "" {
					verified = true
				}
				resp.Body.Close()
			} else if resp != nil {
				resp.Body.Close()
			}
		}
	}

	if !verified || tokenInfo.Email == "" {
		return nil, errors.New("invalid or expired Google authentication credentials")
	}

	if s.cfg.GoogleClientID != "" && tokenInfo.Aud != "" && tokenInfo.Aud != s.cfg.GoogleClientID {
		return nil, errors.New("Google token audience does not match configured Client ID")
	}

	cleanEmail := strings.ToLower(strings.TrimSpace(tokenInfo.Email))
	name := strings.TrimSpace(tokenInfo.Name)
	if name == "" {
		name = strings.Split(cleanEmail, "@")[0]
	}

	return s.findOrCreateGoogleUser(ctx, cleanEmail, name)
}

func (s *AuthService) findOrCreateGoogleUser(ctx context.Context, email, name string) (*types.AuthResponse, error) {
	user, err := s.userLoader.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	if user == nil {
		// Auto-register newly authenticated Google user with unguessable bcrypt password hash
		randomBytes := make([]byte, 24)
		if _, err := rand.Read(randomBytes); err != nil {
			return nil, fmt.Errorf("failed to generate secure credential: %w", err)
		}
		hash, err := bcrypt.GenerateFromPassword(randomBytes, bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash credential: %w", err)
		}

		user, err = s.userWriter.CreateUser(ctx, email, name, string(hash))
		if err != nil {
			return nil, fmt.Errorf("failed to register Google user: %w", err)
		}
	}

	tokenString, err := s.generateToken(user)
	if err != nil {
		return nil, fmt.Errorf("failed to issue session token: %w", err)
	}

	return &types.AuthResponse{
		AccessToken: tokenString,
		User:        user,
	}, nil
}

// GetCurrentUser fetches the authenticated user's profile directly from the DB using token-verified userID
func (s *AuthService) GetCurrentUser(ctx context.Context, userID int64) (*models.User, error) {
	return s.userLoader.GetByID(ctx, userID)
}

// UpdateProfile updates name and phone for the token-verified user
func (s *AuthService) UpdateProfile(ctx context.Context, userID int64, req *types.UpdateProfileRequest) (*models.User, error) {
	cleanName := strings.TrimSpace(req.Name)
	if cleanName == "" {
		return nil, errors.New("name cannot be empty")
	}

	var cleanPhone *string
	if req.Phone != nil && strings.TrimSpace(*req.Phone) != "" {
		trimmed := strings.TrimSpace(*req.Phone)
		cleanPhone = &trimmed
	}

	if err := s.userWriter.UpdateProfile(ctx, userID, cleanName, cleanPhone); err != nil {
		return nil, err
	}

	return s.userLoader.GetByID(ctx, userID)
}

// RequestPasswordChangeOTP dispatches an OTP to user's registered phone via SMS (or email fallback)
func (s *AuthService) RequestPasswordChangeOTP(ctx context.Context, userID int64) (*types.RequestPasswordChangeOTPResponse, error) {
	user, err := s.userLoader.GetByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}

	// Rate limit check
	rateKey := fmt.Sprintf("change_password_otp:%d", userID)
	if err := s.checkRateLimit(rateKey); err != nil {
		return nil, err
	}

	code, err := generateOTP()
	if err != nil {
		return nil, err
	}

	otpKey := fmt.Sprintf("change_password:%d", userID)
	expiryMin := s.getIntParam("otp_expiry_minutes", 10)
	localOTPs.mu.Lock()
	localOTPs.otps[otpKey] = &otpItem{
		hashedCode: s.hashOTP(code),
		expiresAt:  time.Now().Add(time.Duration(expiryMin) * time.Minute),
	}
	localOTPs.mu.Unlock()

	targetDesc := ""
	if user.Phone != nil && strings.TrimSpace(*user.Phone) != "" {
		phone := strings.TrimSpace(*user.Phone)
		maskedPhone := phone
		if len(phone) >= 4 {
			maskedPhone = strings.Repeat("*", len(phone)-4) + phone[len(phone)-4:]
		}
		targetDesc = fmt.Sprintf("registered mobile (%s)", maskedPhone)

		// Dispatch SMS simulated / logged securely
		fmt.Printf("\n======================================================\n"+
			"[SPLITO SMS GATEWAY] Dispatching to Registered Number: %s\n"+
			"Message: <%s> is your Splito password authorization OTP. Valid for 10 minutes.\n"+
			"======================================================\n\n", phone, code)
	} else {
		targetDesc = fmt.Sprintf("registered email (%s)", user.Email)
	}

	// Also dispatch to email as reliable channel
	s.dispatchOTPEmail(
		user.Email, code,
		fmt.Sprintf("%s is your Splito password update authorization code", code),
		"Password Update Authorization",
		"Security",
		"We received a request to update your account password. If you initiated this, enter the 6-digit authorization code below:",
	)

	return &types.RequestPasswordChangeOTPResponse{
		Message: fmt.Sprintf("Authorization code sent to your %s", targetDesc),
		Target:  targetDesc,
	}, nil
}

// ChangePassword verifies either Current Password or OTP sent to registered phone/email
func (s *AuthService) ChangePassword(ctx context.Context, userID int64, req *types.ChangePasswordRequest) error {
	if len(req.NewPassword) < 6 {
		return errors.New("new password must be at least 6 characters")
	}

	user, err := s.userLoader.GetByID(ctx, userID)
	if err != nil || user == nil {
		return errors.New("user not found")
	}

	// 1. Verify via OTP if provided
	if req.OTP != nil && strings.TrimSpace(*req.OTP) != "" {
		otpKey := fmt.Sprintf("change_password:%d", userID)
		submittedOTP := strings.TrimSpace(*req.OTP)

		localOTPs.mu.Lock()
		item, exists := localOTPs.otps[otpKey]
		if !exists {
			localOTPs.mu.Unlock()
			return errors.New("no authorization code requested or code has expired")
		}

		if time.Now().After(item.expiresAt) {
			delete(localOTPs.otps, otpKey)
			localOTPs.mu.Unlock()
			return errors.New("authorization code has expired. Please request a new one")
		}

		maxVerifyAttempts := s.getIntParam("otp_max_verify_attempts", 5)
		if !s.compareOTP(submittedOTP, item.hashedCode) {
			item.verifyAttempts++
			if item.verifyAttempts >= maxVerifyAttempts {
				delete(localOTPs.otps, otpKey)
				localOTPs.mu.Unlock()
				return errors.New("too many incorrect code attempts. OTP invalidated")
			}
			remaining := maxVerifyAttempts - item.verifyAttempts
			localOTPs.mu.Unlock()
			return fmt.Errorf("invalid authorization code. %d attempt(s) remaining", remaining)
		}

		delete(localOTPs.otps, otpKey)
		localOTPs.mu.Unlock()
	} else if req.CurrentPassword != nil && strings.TrimSpace(*req.CurrentPassword) != "" {
		// 2. Verify via Current Password
		currPass := strings.TrimSpace(*req.CurrentPassword)
		// Handle legacy/placeholder password users gracefully
		if user.PasswordHash == "argon2_demo_placeholder" || !strings.HasPrefix(user.PasswordHash, "$2") {
			// Account created without initial password (e.g. OTP-only or Google)
			// Allow initial password setup
		} else {
			if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currPass)); err != nil {
				return errors.New("incorrect current password!. Please try again")
			}
		}
	} else {
		return errors.New("either current password or verification OTP is required")
	}

	// Hash and update to new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	return s.userWriter.UpdatePassword(ctx, userID, string(newHash))
}

