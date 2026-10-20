package money

import (
	"math/rand"
	"testing"
	"time"
)

func TestSplitEvenly_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		total         int64
		memberIDs     []int64
		expectedErr   error
		expectedShare map[int64]int64
	}{
		{
			name:        "100000 paise among 3 members",
			total:       100000,
			memberIDs:   []int64{1, 2, 3},
			expectedErr: nil,
			expectedShare: map[int64]int64{
				1: 33334,
				2: 33333,
				3: 33333,
			},
		},
		{
			name:        "total = 0 among 3 members",
			total:       0,
			memberIDs:   []int64{10, 20, 30},
			expectedErr: nil,
			expectedShare: map[int64]int64{
				10: 0,
				20: 0,
				30: 0,
			},
		},
		{
			name:        "n = 1 single member takes all",
			total:       54321,
			memberIDs:   []int64{42},
			expectedErr: nil,
			expectedShare: map[int64]int64{
				42: 54321,
			},
		},
		{
			name:        "total < n (2 paise among 5 members)",
			total:       2,
			memberIDs:   []int64{1, 2, 3, 4, 5},
			expectedErr: nil,
			expectedShare: map[int64]int64{
				1: 1,
				2: 1,
				3: 0,
				4: 0,
				5: 0,
			},
		},
		{
			name:        "negative total returns error",
			total:       -100,
			memberIDs:   []int64{1, 2},
			expectedErr: ErrNegativeTotal,
		},
		{
			name:        "empty members returns error",
			total:       1000,
			memberIDs:   []int64{},
			expectedErr: ErrEmptyMembers,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := SplitEvenly(tc.total, tc.memberIDs)
			if tc.expectedErr != nil {
				if err != tc.expectedErr {
					t.Fatalf("expected error %v, got %v", tc.expectedErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Verify exact share map match
			for id, expectedVal := range tc.expectedShare {
				if res[id] != expectedVal {
					t.Errorf("member %d: expected %d, got %d", id, expectedVal, res[id])
				}
			}

			// Verify sum matches total exactly
			var sum int64
			for _, v := range res {
				sum += v
			}
			if sum != tc.total {
				t.Errorf("expected total sum %d, got %d", tc.total, sum)
			}
		})
	}
}

func TestSplitEvenly_Property1000Random(t *testing.T) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for iter := 0; iter < 1000; iter++ {
		total := rng.Int63n(100000000) // up to 10 Lakh Rupees (100,000,000 paise)
		numMembers := rng.Intn(50) + 1  // 1 to 50 members

		memberIDs := make([]int64, numMembers)
		idSet := make(map[int64]bool)
		for i := 0; i < numMembers; i++ {
			var id int64
			for {
				id = int64(rng.Intn(10000) + 1)
				if !idSet[id] {
					idSet[id] = true
					break
				}
			}
			memberIDs[i] = id
		}

		res, err := SplitEvenly(total, memberIDs)
		if err != nil {
			t.Fatalf("iteration %d failed with error: %v", iter, err)
		}

		var sum int64
		var minShare, maxShare int64 = 1<<62 - 1, -1

		for _, share := range res {
			sum += share
			if share < minShare {
				minShare = share
			}
			if share > maxShare {
				maxShare = share
			}
		}

		// Invariant 1: Sum of shares must equal total exactly
		if sum != total {
			t.Fatalf("iteration %d: total %d != sum %d", iter, total, sum)
		}

		// Invariant 2: No two shares differ by more than 1 paise
		if maxShare-minShare > 1 {
			t.Fatalf("iteration %d: share discrepancy > 1 (min: %d, max: %d)", iter, minShare, maxShare)
		}
	}
}
