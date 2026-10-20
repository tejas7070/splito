package money

import (
	"math/rand"
	"testing"
	"time"
)

func TestSettlement_KonkanFixture(t *testing.T) {
	// Reference fixture (Konkan trip, amounts in paise):
	// Treasurer is A (ID: 1).
	// Member IDs: 1 (A), 2 (B), 3 (C), 4 (D), 5 (E)
	memberIDs := []int64{1, 2, 3, 4, 5}
	treasurerID := int64(1)

	contributions := map[int64]int64{
		1: 1010700, // A
		2: 1000000, // B
		3: 611500,  // C
		4: 500000,  // D
		5: 1500000, // E
	}

	claims := map[int64]int64{
		1: 30800,  // A
		2: 130000, // B
		3: 60800,  // C
		4: 10000,  // D
		5: 0,      // E
	}

	poolSpend := int64(2500000)

	in := SettlementInput{
		MemberIDs:     memberIDs,
		TreasurerID:   treasurerID,
		Contributions: contributions,
		Claims:        claims,
		PoolSpend:     poolSpend,
	}

	balances, err := ComputeBalances(in)
	if err != nil {
		t.Fatalf("unexpected ComputeBalances error: %v", err)
	}

	// Verify exact net balances after treasurer adjustment
	// Total cost = 2500000 + 231600 = 2731600
	// 2731600 / 5 = 546320 (rem = 0)
	// Cash = (1010700+1000000+611500+500000+1500000) - 2500000 = 2122200
	// A: 1010700 + 30800 - 546320 - 2122200 = -1627020
	// B: 1000000 + 130000 - 546320 = +583680
	// C: 611500 + 60800 - 546320 = +125980
	// D: 500000 + 10000 - 546320 = -36320
	// E: 1500000 + 0 - 546320 = +953680
	expectedBalances := map[int64]int64{
		1: -1627020,
		2: 583680,
		3: 125980,
		4: -36320,
		5: 953680,
	}

	for _, b := range balances {
		expected := expectedBalances[b.UserID]
		if b.Paise != expected {
			t.Errorf("user %d: expected balance %d, got %d", b.UserID, expected, b.Paise)
		}
	}

	// Verify invariant: sum of balances == 0
	var sumBal int64
	for _, b := range balances {
		sumBal += b.Paise
	}
	if sumBal != 0 {
		t.Errorf("sum of balances must be 0, got %d", sumBal)
	}

	transfers, err := ComputeTransfers(balances)
	if err != nil {
		t.Fatalf("unexpected ComputeTransfers error: %v", err)
	}

	// Exactly 4 transfers:
	// 1. A pays E 953680
	// 2. A pays B 583680
	// 3. A pays C 89660
	// 4. D pays C 36320
	expectedTransfers := []Transfer{
		{FromUserID: 1, ToUserID: 5, Paise: 953680},
		{FromUserID: 1, ToUserID: 2, Paise: 583680},
		{FromUserID: 1, ToUserID: 3, Paise: 89660},
		{FromUserID: 4, ToUserID: 3, Paise: 36320},
	}

	if len(transfers) != len(expectedTransfers) {
		t.Fatalf("expected %d transfers, got %d: %+v", len(expectedTransfers), len(transfers), transfers)
	}

	for i, tr := range transfers {
		exp := expectedTransfers[i]
		if tr.FromUserID != exp.FromUserID || tr.ToUserID != exp.ToUserID || tr.Paise != exp.Paise {
			t.Errorf("transfer %d: expected %+v, got %+v", i, exp, tr)
		}
	}
}

func TestSettlement_Property1000Random(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for iter := 0; iter < 1000; iter++ {
		n := rng.Intn(20) + 2 // 2 to 21 members
		memberIDs := make([]int64, n)
		for i := 0; i < n; i++ {
			memberIDs[i] = int64(i + 1)
		}
		treasurerID := memberIDs[rng.Intn(n)]

		contributions := make(map[int64]int64)
		claims := make(map[int64]int64)

		var totalContrib int64
		for _, id := range memberIDs {
			c := rng.Int63n(10000000) // up to 100,000 INR
			cl := rng.Int63n(5000000)
			contributions[id] = c
			claims[id] = cl
			totalContrib += c
		}

		// Pool spend <= total contributions
		var poolSpend int64
		if totalContrib > 0 {
			poolSpend = rng.Int63n(totalContrib + 1)
		}

		in := SettlementInput{
			MemberIDs:     memberIDs,
			TreasurerID:   treasurerID,
			Contributions: contributions,
			Claims:        claims,
			PoolSpend:     poolSpend,
		}

		balances, err := ComputeBalances(in)
		if err != nil {
			t.Fatalf("iter %d: ComputeBalances failed: %v", iter, err)
		}

		var sumBal int64
		for _, b := range balances {
			sumBal += b.Paise
		}
		if sumBal != 0 {
			t.Fatalf("iter %d: sum of balances %d != 0", iter, sumBal)
		}

		transfers, err := ComputeTransfers(balances)
		if err != nil {
			t.Fatalf("iter %d: ComputeTransfers failed: %v", iter, err)
		}

		// Invariant 1: Count of transfers <= N - 1
		if len(transfers) > n-1 {
			t.Fatalf("iter %d: transfers count %d > n-1 (%d)", iter, len(transfers), n-1)
		}

		// Invariant 2: All transfer amounts > 0
		for _, tr := range transfers {
			if tr.Paise <= 0 {
				t.Fatalf("iter %d: transfer amount <= 0: %+v", iter, tr)
			}
		}

		// Invariant 3: Applying transfers zeroes all balances exactly
		balMap := make(map[int64]int64)
		for _, b := range balances {
			balMap[b.UserID] = b.Paise
		}

		for _, tr := range transfers {
			balMap[tr.FromUserID] += tr.Paise // debtor pays, reducing negative balance towards 0
			balMap[tr.ToUserID] -= tr.Paise   // creditor receives, reducing positive balance towards 0
		}

		for uid, finalBal := range balMap {
			if finalBal != 0 {
				t.Fatalf("iter %d: user %d residual balance %d != 0 after transfers", iter, uid, finalBal)
			}
		}
	}
}

func TestSettlement_Deterministic(t *testing.T) {
	in := SettlementInput{
		MemberIDs:   []int64{10, 20, 30, 40},
		TreasurerID: 20,
		Contributions: map[int64]int64{
			10: 200000,
			20: 500000,
			30: 100000,
			40: 300000,
		},
		Claims: map[int64]int64{
			10: 50000,
			20: 0,
			30: 80000,
			40: 20000,
		},
		PoolSpend: 600000,
	}

	b1, err := ComputeBalances(in)
	if err != nil {
		t.Fatalf("run 1 failed: %v", err)
	}
	tr1, err := ComputeTransfers(b1)
	if err != nil {
		t.Fatalf("run 1 transfers failed: %v", err)
	}

	b2, err := ComputeBalances(in)
	if err != nil {
		t.Fatalf("run 2 failed: %v", err)
	}
	tr2, err := ComputeTransfers(b2)
	if err != nil {
		t.Fatalf("run 2 transfers failed: %v", err)
	}

	if len(tr1) != len(tr2) {
		t.Fatalf("deterministic check failed: len %d != %d", len(tr1), len(tr2))
	}

	for i := range tr1 {
		if tr1[i] != tr2[i] {
			t.Fatalf("deterministic check failed at index %d: %+v != %+v", i, tr1[i], tr2[i])
		}
	}
}

func TestSettlement_PoolOverspend(t *testing.T) {
	in := SettlementInput{
		MemberIDs:   []int64{1, 2},
		TreasurerID: 1,
		Contributions: map[int64]int64{
			1: 1000,
			2: 2000,
		},
		Claims:    map[int64]int64{},
		PoolSpend: 5000, // Total contrib is 3000, spend is 5000 -> overspend
	}

	_, err := ComputeBalances(in)
	if err != ErrPoolSpendExceedsContributions {
		t.Fatalf("expected ErrPoolSpendExceedsContributions, got: %v", err)
	}
}
