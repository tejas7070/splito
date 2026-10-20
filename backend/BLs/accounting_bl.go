package BLs

import (
	"fmt"
	"tripledger/models"
	"tripledger/types"
)

type DashboardSummary struct {
	TotalContributionsPaise int64 `json:"total_contributions_paise"`
	GroupFundedSpendPaise   int64 `json:"group_funded_spend_paise"`
	AvailableCashPaise      int64 `json:"available_cash_paise"`
	ApprovedClaimsPaise     int64 `json:"approved_claims_paise"`
	PendingClaimsPaise      int64 `json:"pending_claims_paise"`
}

// CalculateDashboardSummary aggregates authoritative financial metrics from splito_db models
func CalculateDashboardSummary(
	contributions []models.Contribution,
	expenses []models.Expense,
	claims []models.ExpenseClaim,
) DashboardSummary {
	var totalContributions float64
	for _, c := range contributions {
		totalContributions += c.Amount
	}

	var groupSpend float64
	for _, e := range expenses {
		if e.PaidFrom == "group_funds" {
			groupSpend += e.Amount
		}
	}

	var approvedClaims float64
	var pendingClaims float64
	for _, cl := range claims {
		if cl.Status == models.ClaimStatusApproved {
			approvedClaims += cl.Amount
		} else if cl.Status == models.ClaimStatusPending {
			pendingClaims += cl.Amount
		}
	}

	availableCash := totalContributions - groupSpend
	if availableCash < 0 {
		availableCash = 0
	}

	return DashboardSummary{
		TotalContributionsPaise: int64(totalContributions * 100),
		GroupFundedSpendPaise:   int64(groupSpend * 100),
		AvailableCashPaise:      int64(availableCash * 100),
		ApprovedClaimsPaise:     int64(approvedClaims * 100),
		PendingClaimsPaise:      int64(pendingClaims * 100),
	}
}

// CalculateSettlement calculates the comprehensive financial reconciliation for an expedition:
// 1. Group-funded spend vs personal out-of-pocket claims
// 2. Personal approved claims are liabilities of the GROUP TREASURY (not owed by individual reviewers)
// 3. Central Treasury cash is used to settle approved claims and refund surplus advance contributions
// 4. Equal fair share cost reconciliation among all active participants
func CalculateSettlement(
	tripID int64,
	members []models.TripMember,
	contributions []models.Contribution,
	expenses []models.Expense,
	claims []models.ExpenseClaim,
) *types.TripSettlementResponse {
	n := len(members)
	if n == 0 {
		return &types.TripSettlementResponse{TripID: tripID}
	}

	// 1. Group Outflows (expenses paid directly from common treasury pool)
	var groupSpend float64
	for _, e := range expenses {
		if e.PaidFrom == "group_funds" {
			groupSpend += e.Amount
		}
	}

	// 2. Personal Out-of-Pocket Claims
	// Note: Only Approved personal claims become liabilities of the expedition group!
	var approvedClaimsTotal float64
	var groupClaims []types.GroupClaimSettlement
	claimByUser := make(map[int64]float64)

	// User Name lookup map
	userNameMap := make(map[int64]string)
	for _, m := range members {
		userNameMap[m.UserID] = m.UserName
	}

	for _, cl := range claims {
		if cl.Status == models.ClaimStatusApproved {
			approvedClaimsTotal += cl.Amount
			claimByUser[cl.ClaimantID] += cl.Amount

			claimantName := userNameMap[cl.ClaimantID]
			if claimantName == "" {
				claimantName = "Participant"
			}

			groupClaims = append(groupClaims, types.GroupClaimSettlement{
				ClaimID:         cl.ID,
				ClaimantID:      cl.ClaimantID,
				ClaimantName:    claimantName,
				Title:           cl.Title,
				Category:        cl.Category,
				Amount:          cl.Amount,
				AmountPaise:     int64(cl.Amount * 100),
				OwedBy:          "Expedition Group Treasury",
				PayableFromPool: false, // updated below once available cash is computed
			})
		}
	}

	// 3. Total Expedition Cost = Group-Funded Outflows + Approved Personal Claims
	totalTripCost := groupSpend + approvedClaimsTotal
	equalShare := totalTripCost / float64(n)

	// 4. Advance Pool Contributions
	var totalContributions float64
	contribByUser := make(map[int64]float64)
	for _, c := range contributions {
		contribByUser[c.UserID] += c.Amount
		totalContributions += c.Amount
	}

	// 5. Central Treasury Reconciliation
	availableCash := totalContributions - groupSpend
	if availableCash < 0 {
		availableCash = 0
	}

	// Can Treasury cash cover the approved personal claims?
	claimsCoveredByPool := availableCash
	if claimsCoveredByPool > approvedClaimsTotal {
		claimsCoveredByPool = approvedClaimsTotal
	}

	payableFromPool := availableCash >= approvedClaimsTotal
	for i := range groupClaims {
		groupClaims[i].PayableFromPool = payableFromPool
	}

	// Remaining unspent treasury surplus after reimbursing claims from cash
	netTreasurySurplus := availableCash - claimsCoveredByPool
	if netTreasurySurplus < 0 {
		netTreasurySurplus = 0
	}

	treasurySummary := types.TreasurySettlementSummary{
		TotalContributionsPaise: int64(totalContributions * 100),
		GroupSpendPaise:         int64(groupSpend * 100),
		AvailableCashPaise:      int64(availableCash * 100),
		ApprovedClaimsPaise:     int64(approvedClaimsTotal * 100),
		NetSurplusPaise:         int64(netTreasurySurplus * 100),
	}

	// 6. Member Balances Calculation
	type balanceItem struct {
		UserID   int64
		UserName string
		Net      float64
	}
	var debtors []balanceItem
	var creditors []balanceItem
	var balances []types.ParticipantBalance

	for _, m := range members {
		contributed := contribByUser[m.UserID]
		claimed := claimByUser[m.UserID]
		totalPaid := contributed + claimed
		net := totalPaid - equalShare

		var poolRefund float64
		if totalContributions > 0 && netTreasurySurplus > 0 {
			poolRefund = (contributed / totalContributions) * netTreasurySurplus
		}

		claimPayout := claimed

		var peerDues float64
		if net < -0.01 {
			peerDues = -net
		}

		balances = append(balances, types.ParticipantBalance{
			UserID:          m.UserID,
			UserName:        m.UserName,
			Email:           m.Email,
			Contributed:     contributed,
			Claimed:         claimed,
			TotalPaid:       totalPaid,
			EqualShare:      equalShare,
			NetBalance:      net,
			NetBalancePaise: int64(net * 100),
			PoolRefund:      poolRefund,
			ClaimPayout:     claimPayout,
			PeerDues:        peerDues,
		})

		if net < -0.01 {
			debtors = append(debtors, balanceItem{UserID: m.UserID, UserName: m.UserName, Net: -net})
		} else if net > 0.01 {
			creditors = append(creditors, balanceItem{UserID: m.UserID, UserName: m.UserName, Net: net})
		}
	}

	// 7. Settlement Actions & Transfers:
	var transfers []types.SettlementTransfer

	// A. Approved Personal Claims Reimbursement:
	// Owed by the GROUP TREASURY to each claimant!
	// (Reviewer/acceptor does NOT personally owe this; it's funded by group pool)
	for _, m := range members {
		claimed := claimByUser[m.UserID]
		if claimed > 0.01 {
			sourceName := "Expedition Group Treasury (Pool Cash)"
			if availableCash < claimed {
				sourceName = "Expedition Group Treasury (Deficit)"
			}
			transfers = append(transfers, types.SettlementTransfer{
				FromUserID:   0,
				FromUserName: sourceName,
				ToUserID:     m.UserID,
				ToUserName:   m.UserName,
				Amount:       claimed,
				AmountPaise:  int64(claimed * 100),
				TransferType: "group_claim",
				Description:  "Personal Out-of-Pocket Expense Claim Reimbursement (Owed by Group)",
			})
		}
	}

	// B. Unspent Advance Pool Contribution Refund:
	// If Treasury has surplus cash after claims, refund to contributor(s):
	if netTreasurySurplus > 0.01 && totalContributions > 0 {
		for _, m := range members {
			contributed := contribByUser[m.UserID]
			if contributed > 0 {
				refund := (contributed / totalContributions) * netTreasurySurplus
				if refund > 0.01 {
					pct := (contributed / totalContributions) * 100.0
					desc := fmt.Sprintf("Unspent Pool Advance Refund: Returns %d%% of leftover pool cash to contributor %s (deposited ₹%.0f of ₹%.0f pool total). Refunded based on advance contribution share, not role.", int(pct), m.UserName, contributed, totalContributions)
					transfers = append(transfers, types.SettlementTransfer{
						FromUserID:   0,
						FromUserName: "Expedition Group Treasury (Surplus Cash)",
						ToUserID:     m.UserID,
						ToUserName:   m.UserName,
						Amount:       refund,
						AmountPaise:  int64(refund * 100),
						TransferType: "pool_refund",
						Description:  desc,
					})
				}
			}
		}
	}

	// C. Member Fair Share Equalization:
	// Members who underpaid their equal share pay towards the pool or over-contributing peers
	dIdx, cIdx := 0, 0
	for dIdx < len(debtors) && cIdx < len(creditors) {
		debtor := &debtors[dIdx]
		creditor := &creditors[cIdx]

		settleAmount := debtor.Net
		if creditor.Net < settleAmount {
			settleAmount = creditor.Net
		}

		if settleAmount > 0.01 {
			transfers = append(transfers, types.SettlementTransfer{
				FromUserID:   debtor.UserID,
				FromUserName: debtor.UserName,
				ToUserID:     creditor.UserID,
				ToUserName:   creditor.UserName,
				Amount:       settleAmount,
				AmountPaise:  int64(settleAmount * 100),
				TransferType: "peer_fair_share",
				Description:  "Equal Fair Share Cost Settlement (Group Spend & Claims)",
			})
		}

		debtor.Net -= settleAmount
		creditor.Net -= settleAmount

		if debtor.Net <= 0.01 {
			dIdx++
		}
		if creditor.Net <= 0.01 {
			cIdx++
		}
	}

	return &types.TripSettlementResponse{
		TripID:              tripID,
		TotalTripCost:       totalTripCost,
		TotalTripCostPaise:  int64(totalTripCost * 100),
		GroupSpendPaise:     int64(groupSpend * 100),
		ApprovedClaimsPaise: int64(approvedClaimsTotal * 100),
		EqualSharePerUser:   equalShare,
		MemberCount:         n,
		TreasurySummary:     treasurySummary,
		GroupClaims:         groupClaims,
		Balances:            balances,
		Transfers:           transfers,
	}
}
