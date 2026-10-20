package money

import (
	"errors"
	"fmt"
	"sort"
)

var (
	ErrPoolSpendExceedsContributions = errors.New("pool spending exceeds contributions: record the difference as a claim or contribution")
	ErrInvariantViolated             = errors.New("internal error: sum of balances does not equal zero")
	ErrTreasurerNotInMembers         = errors.New("treasurerID must be present in memberIDs")
)

type SettlementInput struct {
	MemberIDs     []int64         // ascending
	TreasurerID   int64           // the Munimji; must be in MemberIDs
	Contributions map[int64]int64 // confirmed inflows per member, paise
	Claims        map[int64]int64 // APPROVED claims per member, paise
	PoolSpend     int64           // total group outflows, paise
}

type Balance struct {
	UserID int64
	Paise  int64 // positive = should receive, negative = should pay
}

type Transfer struct {
	FromUserID int64
	ToUserID   int64
	Paise      int64
}

// ComputeBalances calculates the exact net balance for each member in paise.
// Net = Contributions[i] + Claims[i] - FairShare[i]
// Treasurer physically holds unspent cash, so: Net[Treasurer] -= Cash
// Sum of all Net balances must equal 0 exactly.
func ComputeBalances(in SettlementInput) ([]Balance, error) {
	if len(in.MemberIDs) == 0 {
		return nil, ErrEmptyMembers
	}

	// Validate Treasurer is in MemberIDs
	treasurerFound := false
	for _, m := range in.MemberIDs {
		if m == in.TreasurerID {
			treasurerFound = true
			break
		}
	}
	if !treasurerFound {
		return nil, ErrTreasurerNotInMembers
	}

	// 1. Total claims & total contributions
	var totalClaims int64
	for _, c := range in.Claims {
		totalClaims += c
	}

	var totalContributions int64
	for _, c := range in.Contributions {
		totalContributions += c
	}

	// 2. Total cost & fair share
	totalCost := in.PoolSpend + totalClaims
	shares, err := SplitEvenly(totalCost, in.MemberIDs)
	if err != nil {
		return nil, err
	}

	// 3. Central Treasury Cash in Hand
	cash := totalContributions - in.PoolSpend
	if cash < 0 {
		return nil, ErrPoolSpendExceedsContributions
	}

	// 4. Net per member
	sortedIDs := make([]int64, len(in.MemberIDs))
	copy(sortedIDs, in.MemberIDs)
	sort.Slice(sortedIDs, func(i, j int) bool { return sortedIDs[i] < sortedIDs[j] })

	balances := make([]Balance, len(sortedIDs))
	var sumNets int64

	for i, uid := range sortedIDs {
		contrib := in.Contributions[uid]
		claim := in.Claims[uid]
		share := shares[uid]

		net := contrib + claim - share

		// 5. Treasurer holds the physical cash in hand (liability)
		if uid == in.TreasurerID {
			net -= cash
		}

		balances[i] = Balance{
			UserID: uid,
			Paise:  net,
		}
		sumNets += net
	}

	// 6. Invariant guard: sum of all net balances must equal 0
	if sumNets != 0 {
		return nil, fmt.Errorf("%w: sum is %d paise", ErrInvariantViolated, sumNets)
	}

	return balances, nil
}

// ComputeTransfers greedily matches debtors and creditors deterministically,
// resulting in at most N - 1 transfers that bring every member's balance to 0.
func ComputeTransfers(b []Balance) ([]Transfer, error) {
	type party struct {
		userID int64
		paise  int64
	}

	var creditors []party
	var debtors []party

	for _, bal := range b {
		if bal.Paise > 0 {
			creditors = append(creditors, party{userID: bal.UserID, paise: bal.Paise})
		} else if bal.Paise < 0 {
			debtors = append(debtors, party{userID: bal.UserID, paise: -bal.Paise}) // store positive debt
		}
	}

	// Sort creditors and debtors descending by amount, ties by userID ascending
	sortParties := func(list []party) {
		sort.Slice(list, func(i, j int) bool {
			if list[i].paise != list[j].paise {
				return list[i].paise > list[j].paise
			}
			return list[i].userID < list[j].userID
		})
	}

	sortParties(creditors)
	sortParties(debtors)

	var transfers []Transfer
	ci, di := 0, 0

	for ci < len(creditors) && di < len(debtors) {
		c := &creditors[ci]
		d := &debtors[di]

		amount := c.paise
		if d.paise < amount {
			amount = d.paise
		}

		if amount > 0 {
			transfers = append(transfers, Transfer{
				FromUserID: d.userID,
				ToUserID:   c.userID,
				Paise:      amount,
			})
			c.paise -= amount
			d.paise -= amount
		}

		if c.paise == 0 {
			ci++
		}
		if d.paise == 0 {
			di++
		}
	}

	return transfers, nil
}
