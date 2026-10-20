package services

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"tripledger/config"
	"tripledger/loaders"
	"tripledger/types"
	"tripledger/writers"
)

func setupAuthMock(t *testing.T) (*sql.DB, sqlmock.Sqlmock, *AuthService) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to open sqlmock: %v", err)
	}

	cfg := &config.Config{
		JWTSecret: "test-jwt-secret-key-that-is-at-least-32-chars-long",
	}
	ul := loaders.NewUserLoader(db)
	uw := writers.NewUserWriter(db)
	as := NewAuthService(db, cfg, ul, uw)

	return db, mock, as
}

func TestOTP_MaxVerifyAttemptsInvalidation(t *testing.T) {
	db, _, as := setupAuthMock(t)
	defer db.Close()

	email := "test_throttle@example.com"
	key := "register:" + email

	// Store an OTP hashed
	otpCode := "123456"
	localOTPs.mu.Lock()
	localOTPs.otps[key] = &otpItem{
		hashedCode:     as.hashOTP(otpCode),
		expiresAt:      time.Now().Add(10 * time.Minute),
		verifyAttempts: 0,
	}
	localOTPs.mu.Unlock()

	ctx := context.Background()

	// 5 wrong attempts should invalidate
	for i := 1; i <= 4; i++ {
		req := &types.RegisterRequest{
			Email:    email,
			Name:     "Test User",
			Password: "password123",
			OTP:      "000000",
		}
		_, err := as.Register(ctx, req)
		if err == nil {
			t.Fatalf("attempt %d: expected error for wrong OTP", i)
		}
	}

	// 5th wrong attempt -> OTP gets deleted/invalidated
	req := &types.RegisterRequest{
		Email:    email,
		Name:     "Test User",
		Password: "password123",
		OTP:      "000000",
	}
	_, err := as.Register(ctx, req)
	if err == nil || err.Error() != "too many incorrect attempts. This OTP has been invalidated, please request a new code" {
		t.Fatalf("5th attempt: expected invalidation error, got: %v", err)
	}

	// 6th attempt (even with right OTP) must fail because OTP was invalidated
	req.OTP = otpCode
	_, err = as.Register(ctx, req)
	if err == nil || err.Error() != "no verification code found or it has expired, please request a new one" {
		t.Fatalf("6th attempt: expected code not found/expired, got: %v", err)
	}
}

func TestOTP_ExpiredFails(t *testing.T) {
	db, _, as := setupAuthMock(t)
	defer db.Close()

	email := "expired@example.com"
	key := "register:" + email

	localOTPs.mu.Lock()
	localOTPs.otps[key] = &otpItem{
		hashedCode: as.hashOTP("999999"),
		expiresAt:  time.Now().Add(-1 * time.Minute), // Expired 1 min ago
	}
	localOTPs.mu.Unlock()

	ctx := context.Background()
	req := &types.RegisterRequest{
		Email:    email,
		Name:     "Expired User",
		Password: "password123",
		OTP:      "999999",
	}
	_, err := as.Register(ctx, req)
	if err == nil || err.Error() != "verification code has expired, please request a new one" {
		t.Fatalf("expected expired error, got: %v", err)
	}
}

func TestLogin_ThrottlingAfter5Failures(t *testing.T) {
	db, _, as := setupAuthMock(t)
	defer db.Close()

	testEmail := "bruteforce@example.com"
	key := "login:email:" + testEmail

	// Reset any existing
	as.ResetLoginFailure(key)

	// Record 4 failures -> should still be ok
	for i := 1; i <= 4; i++ {
		ok, _ := as.CheckLoginRateLimit(key)
		if !ok {
			t.Fatalf("attempt %d should be allowed", i)
		}
		as.RecordLoginFailure(key)
	}

	// 5th failure recorded
	as.RecordLoginFailure(key)

	// Now check rate limit -> must be locked out
	ok, waitSec := as.CheckLoginRateLimit(key)
	if ok {
		t.Fatalf("expected login to be throttled/locked after 5 failures")
	}
	if waitSec <= 0 {
		t.Fatalf("expected positive wait time, got %d", waitSec)
	}
}

func TestForgotPassword_GenericResponse(t *testing.T) {
	db, mock, as := setupAuthMock(t)
	defer db.Close()

	// Email does NOT exist in DB (sql.ErrNoRows)
	mock.ExpectQuery("SELECT id, name, email, password_hash, phone, created_at, updated_at FROM user_details WHERE email = \\?").
		WithArgs("nonexistent@example.com").
		WillReturnError(sql.ErrNoRows)

	ctx := context.Background()
	err := as.RequestForgotPasswordOTP(ctx, "nonexistent@example.com")
	// Must return nil (generic success response to prevent email enumeration)
	if err != nil {
		t.Fatalf("expected generic nil response for non-existent email, got: %v", err)
	}
}
