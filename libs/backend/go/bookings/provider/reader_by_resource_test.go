//go:build integration

package bookingspostgres_test

import (
	"context"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
	bookingspostgres "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/provider"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

const (
	queriedResource = bookingsdomain.ResourceID("r-query")
	otherResource   = bookingsdomain.ResourceID("r-other")
)

func saveBooking(t *testing.T, pool *pgxpool.Pool, id bookingsdomain.BookingID, resource bookingsdomain.ResourceID) {
	t.Helper()
	err := withRepo(t, pool, func(ctx context.Context, repo dmpfports.Repository[bookingsdomain.BookingID, bookingsdomain.BookingSnapshot]) error {
		return repo.Save(ctx, id, bookingsdomain.BookingSnapshot{
			ID:         id,
			ResourceID: resource,
			Quantity:   3,
			Status:     bookingsdomain.BookingReservedStatus,
			ReservedAt: 1755432000,
		}, 0)
	})
	if err != nil {
		t.Fatalf("save %s on %s: %v", id, resource, err)
	}
}

func ids(snapshots []bookingsdomain.BookingSnapshot) []string {
	out := make([]string, 0, len(snapshots))
	for _, s := range snapshots {
		out = append(out, string(s.ID))
	}
	sort.Strings(out)
	return out
}

// The reader takes the pool, never a Tx: a query traverses a relation and runs
// outside any unit of work, so it must not be able to join a caller's
// transaction. Constructing it from the pool is what makes that structural.
func TestLoadByResourceReturnsOnlyTheBookingsOfThatResource(t *testing.T) {
	pool := openPool(t)

	saveBooking(t, pool, "b-query-1", queriedResource)
	saveBooking(t, pool, "b-query-2", queriedResource)
	saveBooking(t, pool, "b-other-1", otherResource)

	reader := bookingspostgres.NewBookingsByResourceReader(pool)

	found, err := reader.LoadByResource(context.Background(), queriedResource)
	if err != nil {
		t.Fatalf("LoadByResource(%s) = %v, want nil", queriedResource, err)
	}
	if got, want := ids(found), []string{"b-query-1", "b-query-2"}; !equal(got, want) {
		t.Errorf("LoadByResource(%s) = %v, want %v", queriedResource, got, want)
	}

	for _, s := range found {
		if s.ResourceID != queriedResource {
			t.Errorf("booking %s has resource %s, want %s", s.ID, s.ResourceID, queriedResource)
		}
	}

	// The booking of the other resource is still there: the filter narrows the
	// result; it does not shrink the table.
	others, err := reader.LoadByResource(context.Background(), otherResource)
	if err != nil {
		t.Fatalf("LoadByResource(%s) = %v, want nil", otherResource, err)
	}
	if got, want := ids(others), []string{"b-other-1"}; !equal(got, want) {
		t.Errorf("LoadByResource(%s) = %v, want %v", otherResource, got, want)
	}
}

func TestLoadByResourceReturnsEmptyForUnknownResource(t *testing.T) {
	pool := openPool(t)

	saveBooking(t, pool, "b-query-1", queriedResource)

	found, err := bookingspostgres.NewBookingsByResourceReader(pool).
		LoadByResource(context.Background(), "r-absent")
	if err != nil {
		t.Fatalf("LoadByResource(r-absent) = %v, want nil", err)
	}
	if len(found) != 0 {
		t.Errorf("LoadByResource(r-absent) returned %d booking(s), want 0", len(found))
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
