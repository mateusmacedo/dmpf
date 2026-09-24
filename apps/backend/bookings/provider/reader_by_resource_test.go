//go:build integration

package provider_test

import (
	"context"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const (
	queriedResource = domain.ResourceID("r-query")
	otherResource   = domain.ResourceID("r-other")
)

func saveBooking(t *testing.T, pool *pgxpool.Pool, id domain.BookingID, resource domain.ResourceID) {
	t.Helper()
	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.BookingID, domain.BookingSnapshot]) error {
		return repo.Save(ctx, id, domain.BookingSnapshot{
			ID:         id,
			ResourceID: resource,
			Quantity:   3,
			Status:     domain.BookingReservedStatus,
			ReservedAt: 1755432000,
		}, 0)
	})
	if err != nil {
		t.Fatalf("save %s on %s: %v", id, resource, err)
	}
}

func ids(snapshots []domain.BookingSnapshot) []string {
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
	pool := pg.OpenPool(t, "bookings_booking", "bookings_resource")

	saveBooking(t, pool, "b-query-1", queriedResource)
	saveBooking(t, pool, "b-query-2", queriedResource)
	saveBooking(t, pool, "b-other-1", otherResource)

	reader := provider.NewBookingsByResourceReader(postgres.NewReadPool(pool))

	found, err := reader.LoadByResource(withExecution(t, context.Background()), queriedResource)
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
	others, err := reader.LoadByResource(withExecution(t, context.Background()), otherResource)
	if err != nil {
		t.Fatalf("LoadByResource(%s) = %v, want nil", otherResource, err)
	}
	if got, want := ids(others), []string{"b-other-1"}; !equal(got, want) {
		t.Errorf("LoadByResource(%s) = %v, want %v", otherResource, got, want)
	}
}

func TestLoadByResourceReturnsEmptyForUnknownResource(t *testing.T) {
	pool := pg.OpenPool(t, "bookings_booking", "bookings_resource")

	saveBooking(t, pool, "b-query-1", queriedResource)

	found, err := provider.NewBookingsByResourceReader(postgres.NewReadPool(pool)).
		LoadByResource(withExecution(t, context.Background()), "r-absent")
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
