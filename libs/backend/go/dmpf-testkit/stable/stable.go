package stable

import (
	"cmp"
	"slices"
	"sync"
)

// SortStrings returns a sorted copy; the input is left as it was so a test can
// still assert on the order it observed.
func SortStrings(s []string) []string {
	out := slices.Clone(s)
	slices.Sort(out)
	return out
}

// SortBy returns a copy ordered by key, keeping the input order among equal
// keys (KIT-08): a collection drawn from a map compares the same on every run.
func SortBy[T any, K cmp.Ordered](s []T, key func(T) K) []T {
	out := slices.Clone(s)
	slices.SortStableFunc(out, func(a, b T) int { return cmp.Compare(key(a), key(b)) })
	return out
}

// Sequence is an append-only collection that hands each item its position, so
// a ledger of gestures can be compared by order without a clock. The zero value
// is ready to use and safe for concurrent Append.
type Sequence[T any] struct {
	mu    sync.Mutex
	items []T
}

func (s *Sequence[T]) Append(item T) (position int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	return len(s.items) - 1
}

func (s *Sequence[T]) Items() []T {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.items)
}

func (s *Sequence[T]) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.items)
}
