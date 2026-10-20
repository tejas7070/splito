package money

import (
	"errors"
	"sort"
)

var (
	ErrNegativeTotal = errors.New("total paise cannot be negative")
	ErrEmptyMembers  = errors.New("memberIDs cannot be empty")
)

// SplitEvenly divides total paise among members deterministically.
// memberIDs must be sorted ascending by user ID by the caller.
// The first (total % n) members receive (base + 1), and remaining members receive base.
func SplitEvenly(total int64, memberIDs []int64) (map[int64]int64, error) {
	if total < 0 {
		return nil, ErrNegativeTotal
	}
	n := int64(len(memberIDs))
	if n == 0 {
		return nil, ErrEmptyMembers
	}

	// Verify or ensure IDs are sorted ascending
	ids := make([]int64, len(memberIDs))
	copy(ids, memberIDs)
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	base := total / n
	rem := total % n

	result := make(map[int64]int64, n)
	for i, id := range ids {
		share := base
		if int64(i) < rem {
			share++
		}
		result[id] = share
	}

	return result, nil
}
