package tests

import (
	"testing"

	"tripledger/BLs"
	"tripledger/models"
)

func TestCalculateDashboardSummary(t *testing.T) {
	contributions := []models.Contribution{
		{AmountPaise: 1000000, Status: "recorded"}, // ₹10,000
		{AmountPaise: 500000, Status: "recorded"},  // ₹5,000
		{AmountPaise: 200000, Status: "reversed"},  // Should be ignored
	}

	expenses := []models.Expense{
		{AmountPaise: 300000, PaymentSource: "group_fund", Status: "recorded"}, // ₹3,000 group spend
		{AmountPaise: 150000, PaymentSource: "group_fund", Status: "recorded"}, // ₹1,500 group spend
		{AmountPaise: 50000, PaymentSource: "group_fund", Status: "reversed"},  // Should be ignored
	}

	claims := []models.ExpenseClaim{
		{AmountPaise: 100000, Status: models.ClaimStatusApproved}, // ₹1,000 approved out-of-pocket
		{AmountPaise: 50000, Status: models.ClaimStatusPending},   // ₹500 pending
		{AmountPaise: 20000, Status: models.ClaimStatusRejected},  // Ignored
	}

	summary := BLs.CalculateDashboardSummary(contributions, expenses, claims)

	// Contributions: 10,000 + 5,000 = 15,000 (1,500,000 paise)
	if summary.TotalContributionsPaise != 1500000 {
		t.Errorf("expected 1500000 paise contributions, got %d", summary.TotalContributionsPaise)
	}

	// Group spend: 3,000 + 1,500 = 4,500 (450,000 paise)
	if summary.GroupFundedSpendPaise != 450000 {
		t.Errorf("expected 450000 paise group spend, got %d", summary.GroupFundedSpendPaise)
	}

	// Available cash: 15,000 - 4,500 = 10,500 (1,050,000 paise)
	if summary.AvailableCashPaise != 1050000 {
		t.Errorf("expected 1050000 paise available cash, got %d", summary.AvailableCashPaise)
	}

	// Approved claims liability: ₹1,000 (100,000 paise)
	if summary.ApprovedClaimsPaise != 100000 {
		t.Errorf("expected 100000 paise approved claims, got %d", summary.ApprovedClaimsPaise)
	}

	// Pending claims: ₹500 (50,000 paise)
	if summary.PendingClaimsPaise != 50000 {
		t.Errorf("expected 50000 paise pending claims, got %d", summary.PendingClaimsPaise)
	}
}
