package ports_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestInstantUnix(t *testing.T) {
	tests := []struct {
		name string
		in   ports.Instant
		want int64
	}{
		{name: "epoch", in: 0, want: 0},
		{name: "whole second", in: 1_755_432_000_000_000_000, want: 1_755_432_000},
		{name: "sub-second is truncated", in: 1_755_432_000_999_999_999, want: 1_755_432_000},
		{name: "one nanosecond", in: 1, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.Unix(); got != tt.want {
				t.Fatalf("Unix() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestInstantOrdersChronologically(t *testing.T) {
	earlier := ports.Instant(1_755_432_000_000_000_000)
	later := ports.Instant(1_755_432_000_000_000_001)

	if earlier >= later {
		t.Fatalf("Instant must order chronologically: %d < %d is false", earlier, later)
	}

	got := []ports.Instant{later, earlier}
	slices.Sort(got)
	if want := []ports.Instant{earlier, later}; !slices.Equal(got, want) {
		t.Fatalf("slices.Sort = %v, want %v", got, want)
	}
}

func TestMessageIDIsComparableAndUsableAsKey(t *testing.T) {
	var absent ports.MessageID
	if absent != "" {
		t.Fatalf("the zero MessageID must be the empty (invalid) identifier, got %q", absent)
	}

	seen := map[ports.MessageID]int{}
	seen["m-000001"]++
	seen["m-000001"]++
	if seen["m-000001"] != 2 {
		t.Fatalf("MessageID must be usable as a map key, got %d occurrences", seen["m-000001"])
	}
}

func TestVersionZeroMeansNotPersisted(t *testing.T) {
	var notPersisted ports.Version
	if notPersisted != 0 {
		t.Fatalf("the zero Version must be 0, got %d", notPersisted)
	}
	if got := notPersisted + 1; got != 1 {
		t.Fatalf("the version written for expected=0 must be 1, got %d", got)
	}
}
