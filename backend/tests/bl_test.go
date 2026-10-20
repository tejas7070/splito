package tests

import (
	"testing"
	"time"

	"tripledger/BLs"
	"tripledger/models"
)

func TestValidateClaimDecision(t *testing.T) {
	munimjiID := int64(1)
	coReviewerID := int64(2)
	peerID := int64(3)

	// Test 1: Self-approval forbidden
	claim1 := &models.ExpenseClaim{ClaimantID: peerID, Status: models.ClaimStatusPending}
	err := BLs.ValidateClaimDecision(claim1, peerID, models.RoleParticipant)
	if err != BLs.ErrSelfApproval {
		t.Errorf("expected ErrSelfApproval, got %v", err)
	}

	// Test 2: Munimji self-approval forbidden
	claim2 := &models.ExpenseClaim{ClaimantID: munimjiID, Status: models.ClaimStatusPending}
	err = BLs.ValidateClaimDecision(claim2, munimjiID, models.RoleMunimji)
	if err != BLs.ErrSelfApproval {
		t.Errorf("expected ErrSelfApproval for munimji self claim, got %v", err)
	}

	// Test 3: Peer claim approved by Munimji (valid)
	claim3 := &models.ExpenseClaim{ClaimantID: peerID, Status: models.ClaimStatusPending}
	err = BLs.ValidateClaimDecision(claim3, munimjiID, models.RoleMunimji)
	if err != nil {
		t.Errorf("expected valid approval by munimji, got %v", err)
	}

	// Test 4: Munimji claim reviewed by Co-Reviewer (valid)
	claim4 := &models.ExpenseClaim{ClaimantID: munimjiID, Status: models.ClaimStatusPending}
	err = BLs.ValidateClaimDecision(claim4, coReviewerID, models.RoleCoReviewer)
	if err != nil {
		t.Errorf("expected co-reviewer to be able to approve Munimji claim, got %v", err)
	}

	// Test 5: Munimji claim reviewed by ordinary peer (forbidden)
	err = BLs.ValidateClaimDecision(claim4, peerID, models.RoleParticipant)
	if err != BLs.ErrUnauthorizedMunim {
		t.Errorf("expected ErrUnauthorizedMunim when ordinary peer tries to review, got %v", err)
	}
}

func TestDateStrictIntervals(t *testing.T) {
	// Trip A: 10:00:00 to 14:00:00
	aStart := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
	aEnd := time.Date(2026, 10, 11, 14, 0, 0, 0, time.UTC)

	// Trip B starts immediately when Trip A ends (14:00:01) -> Valid, no overlap
	bStart := time.Date(2026, 10, 11, 14, 0, 1, 0, time.UTC)
	bEnd := time.Date(2026, 10, 11, 18, 0, 0, 0, time.UTC)

	// Strict condition: bStart < aEnd AND bEnd > aStart
	overlaps := bStart.Before(aEnd) && bEnd.After(aStart)
	if overlaps {
		t.Errorf("trip B starting at 14:00:01 should NOT overlap with Trip A ending at 14:00:00")
	}

	// Trip C: starts at 13:59:59 (1 second before A ends) -> Must overlap
	cStart := time.Date(2026, 10, 11, 13, 59, 59, 0, time.UTC)
	cEnd := time.Date(2026, 10, 11, 18, 0, 0, 0, time.UTC)
	overlapsC := cStart.Before(aEnd) && cEnd.After(aStart)
	if !overlapsC {
		t.Errorf("trip C starting at 13:59:59 SHOULD overlap with Trip A")
	}
}
