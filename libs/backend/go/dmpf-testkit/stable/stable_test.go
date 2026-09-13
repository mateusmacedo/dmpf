package stable_test

import (
	"slices"
	"sync"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/stable"
)

func TestSortStringsReturnsASortedCopy(t *testing.T) {
	in := []string{"b", "c", "a"}
	got := stable.SortStrings(in)
	if !slices.Equal(got, []string{"a", "b", "c"}) {
		t.Fatalf("SortStrings = %v", got)
	}
	if !slices.Equal(in, []string{"b", "c", "a"}) {
		t.Fatalf("SortStrings mutated its input: %v", in)
	}
}

func TestSortByIsStableForEqualKeys(t *testing.T) {
	type row struct {
		key  int
		name string
	}
	in := []row{{2, "first-2"}, {1, "first-1"}, {2, "second-2"}, {1, "second-1"}}
	got := stable.SortBy(in, func(r row) int { return r.key })
	want := []row{{1, "first-1"}, {1, "second-1"}, {2, "first-2"}, {2, "second-2"}}
	if !slices.Equal(got, want) {
		t.Fatalf("SortBy = %v, want %v", got, want)
	}
	if in[0].name != "first-2" {
		t.Fatalf("SortBy mutated its input: %v", in)
	}
}

func TestSortByFromMapIsDeterministic(t *testing.T) {
	m := map[string]int{"z": 1, "a": 2, "m": 3}
	var first []string
	for range 20 {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		got := stable.SortBy(keys, func(k string) string { return k })
		if first == nil {
			first = got
			continue
		}
		if !slices.Equal(got, first) {
			t.Fatalf("iteration order leaked: %v vs %v", got, first)
		}
	}
}

func TestSequenceKeepsInsertionOrderAndPositions(t *testing.T) {
	var s stable.Sequence[string]
	if p := s.Append("open"); p != 0 {
		t.Fatalf("first position = %d, want 0", p)
	}
	if p := s.Append("save"); p != 1 {
		t.Fatalf("second position = %d, want 1", p)
	}
	if p := s.Append("commit"); p != 2 {
		t.Fatalf("third position = %d, want 2", p)
	}
	if got := s.Items(); !slices.Equal(got, []string{"open", "save", "commit"}) {
		t.Fatalf("Items = %v", got)
	}
	if s.Len() != 3 {
		t.Fatalf("Len = %d", s.Len())
	}
	items := s.Items()
	items[0] = "mutated"
	if s.Items()[0] != "open" {
		t.Fatal("Items exposed the internal slice")
	}
}

func TestSequenceIsSafeForConcurrentAppend(t *testing.T) {
	var s stable.Sequence[int]
	const n = 500
	var wg sync.WaitGroup
	positions := make(chan int, n)
	for i := range n {
		wg.Go(func() { positions <- s.Append(i) })
	}
	wg.Wait()
	close(positions)
	seen := make([]bool, n)
	for p := range positions {
		if seen[p] {
			t.Fatalf("position %d handed out twice", p)
		}
		seen[p] = true
	}
	if s.Len() != n {
		t.Fatalf("Len = %d, want %d", s.Len(), n)
	}
}
