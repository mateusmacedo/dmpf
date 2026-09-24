package ports_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/ports"
)

// readerFunc realizes BookingsByResourceReader over a function, so the contract
// is exercised without reaching the provider block: the port declares the
// boundary and holds no realization (FND-04 §3.3).
type readerFunc func(context.Context, domain.ResourceID) ([]domain.BookingSnapshot, error)

func (f readerFunc) LoadByResource(ctx context.Context, resourceID domain.ResourceID) ([]domain.BookingSnapshot, error) {
	return f(ctx, resourceID)
}

func TestLoadByResourceAnswersWithEveryBookingHeldByTheResource(t *testing.T) {
	held := []domain.BookingSnapshot{
		{ID: "booking-1", ResourceID: "room-1", Quantity: 2, Status: domain.BookingReservedStatus, ReservedAt: 1},
		{ID: "booking-2", ResourceID: "room-1", Quantity: 1, Status: domain.BookingReservedStatus, ReservedAt: 2},
	}
	var asked domain.ResourceID
	var reader ports.BookingsByResourceReader = readerFunc(
		func(_ context.Context, resourceID domain.ResourceID) ([]domain.BookingSnapshot, error) {
			asked = resourceID
			return held, nil
		})

	got, err := reader.LoadByResource(context.Background(), "room-1")

	if err != nil {
		t.Fatalf("LoadByResource returned %v, want no error", err)
	}
	if asked != "room-1" {
		t.Fatalf("the reader was keyed on %q, want %q: the query traverses the relation from the resource", asked, "room-1")
	}
	if len(got) != len(held) {
		t.Fatalf("LoadByResource returned %d bookings, want %d: the relation answers with a collection, not one aggregate", len(got), len(held))
	}
	for i, snapshot := range got {
		if snapshot.ID != held[i].ID || snapshot.ResourceID != held[i].ResourceID {
			t.Fatalf("booking %d = %+v, want %+v", i, snapshot, held[i])
		}
	}
}

func TestLoadByResourceAnswersWithNoBookingWithoutFailing(t *testing.T) {
	var reader ports.BookingsByResourceReader = readerFunc(
		func(context.Context, domain.ResourceID) ([]domain.BookingSnapshot, error) {
			return nil, nil
		})

	got, err := reader.LoadByResource(context.Background(), "room-without-bookings")

	if err != nil {
		t.Fatalf("LoadByResource returned %v, want no error: an unheld resource is an empty answer, not a failure", err)
	}
	if len(got) != 0 {
		t.Fatalf("LoadByResource returned %d bookings, want none", len(got))
	}
}

func TestLoadByResourceKeepsTheCallerContext(t *testing.T) {
	type key struct{}
	parent := context.WithValue(context.Background(), key{}, "marker")
	var seen context.Context
	var reader ports.BookingsByResourceReader = readerFunc(
		func(ctx context.Context, _ domain.ResourceID) ([]domain.BookingSnapshot, error) {
			seen = ctx
			return nil, nil
		})

	if _, err := reader.LoadByResource(parent, "room-1"); err != nil {
		t.Fatalf("LoadByResource returned %v, want no error", err)
	}

	if seen.Value(key{}) != "marker" {
		t.Fatal("LoadByResource dropped the caller context; deadline and correlation must reach the provider")
	}
}

func TestLoadByResourceSurfacesTheProviderErrorUntranslated(t *testing.T) {
	cause := errors.New("connection reset by peer")
	var reader ports.BookingsByResourceReader = readerFunc(
		func(context.Context, domain.ResourceID) ([]domain.BookingSnapshot, error) {
			return nil, cause
		})

	got, err := reader.LoadByResource(context.Background(), "room-1")

	if !errors.Is(err, cause) {
		t.Fatalf("LoadByResource returned %v, want an error carrying %v: the port declares no taxonomy of its own", err, cause)
	}
	if got != nil {
		t.Fatalf("LoadByResource returned %+v alongside the error, want no bookings", got)
	}
}
