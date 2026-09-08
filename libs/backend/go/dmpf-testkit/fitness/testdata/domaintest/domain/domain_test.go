package domain_test

import (
	"testing"

	"exemplo.test/domaintest/domain"
	"exemplo.test/domaintest/port"
)

// A double of a port inside a domain test: the vector V29/V30 must name it.
type fakeStore struct{ n int }

func (f fakeStore) Load() int { return f.n }

func TestPositiveThroughADouble(t *testing.T) {
	var s port.Store = fakeStore{n: 1}
	if !domain.Positive(s.Load()) {
		t.Fatal("1 is positive")
	}
}
