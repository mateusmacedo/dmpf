//go:build integration

package bookingspostgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
	bookingspostgres "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/provider"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"
)

const repoBookingID = bookingsdomain.BookingID("b-1001")

type repoResources struct {
	Bookings dmpfports.Repository[bookingsdomain.BookingID, bookingsdomain.BookingSnapshot]
}

func bindRepo(tx *dmpfpostgres.Tx) repoResources {
	return repoResources{Bookings: bookingspostgres.NewBookingRepository(tx)}
}

func snap(quantity int) bookingsdomain.BookingSnapshot {
	return bookingsdomain.BookingSnapshot{
		ID:         repoBookingID,
		ResourceID: "r-200",
		Quantity:   quantity,
		Status:     bookingsdomain.BookingReservedStatus,
		ReservedAt: 1755432000,
	}
}

func withRepo(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context, repo dmpfports.Repository[bookingsdomain.BookingID, bookingsdomain.BookingSnapshot]) error) error {
	t.Helper()
	uow := dmpfpostgres.NewUnitOfWork(pool, bindRepo)
	return uow.Within(context.Background(), func(ctx context.Context, res repoResources) error {
		return fn(ctx, res.Bookings)
	})
}

func seed(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if err := withRepo(t, pool, func(ctx context.Context, repo dmpfports.Repository[bookingsdomain.BookingID, bookingsdomain.BookingSnapshot]) error {
		return repo.Save(ctx, repoBookingID, snap(5), 0)
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func loadFromPool(t *testing.T, pool *pgxpool.Pool) (bookingsdomain.BookingSnapshot, dmpfports.Version) {
	t.Helper()
	reader := bookingspostgres.NewBookingReader(pool)
	s, v, err := reader.Load(context.Background(), repoBookingID)
	if err != nil {
		t.Fatalf("reader.Load() = %v", err)
	}
	return s, v
}

func TestSaveCreatesAndLoadReturnsIt(t *testing.T) {
	pool := openPool(t)

	if err := withRepo(t, pool, func(ctx context.Context, repo dmpfports.Repository[bookingsdomain.BookingID, bookingsdomain.BookingSnapshot]) error {
		return repo.Save(ctx, repoBookingID, snap(5), 0)
	}); err != nil {
		t.Fatalf("Save(create) = %v", err)
	}

	got, version := loadFromPool(t, pool)
	if version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}
	want := snap(5)
	if !got.Equal(want) {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

func TestSaveUpdatesWithCorrectVersion(t *testing.T) {
	pool := openPool(t)
	seed(t, pool)

	updated := snap(10)
	updated.Status = bookingsdomain.BookingCancelled
	if err := withRepo(t, pool, func(ctx context.Context, repo dmpfports.Repository[bookingsdomain.BookingID, bookingsdomain.BookingSnapshot]) error {
		return repo.Save(ctx, repoBookingID, updated, 1)
	}); err != nil {
		t.Fatalf("Save(update) = %v", err)
	}

	got, version := loadFromPool(t, pool)
	if version != 2 {
		t.Fatalf("version = %d, want 2", version)
	}
	if got.Status != bookingsdomain.BookingCancelled {
		t.Fatalf("Status = %v, want BookingCancelled", got.Status)
	}
}

func TestSaveConflictsOnStaleVersion(t *testing.T) {
	pool := openPool(t)
	seed(t, pool)

	err := withRepo(t, pool, func(ctx context.Context, repo dmpfports.Repository[bookingsdomain.BookingID, bookingsdomain.BookingSnapshot]) error {
		return repo.Save(ctx, repoBookingID, snap(7), 0)
	})
	if !errors.Is(err, dmpfports.ErrVersionConflict) {
		t.Fatalf("Save(stale) = %v, want ErrVersionConflict", err)
	}
}

func TestLoadReturnsNotFoundForAbsentBooking(t *testing.T) {
	pool := openPool(t)

	err := withRepo(t, pool, func(ctx context.Context, repo dmpfports.Repository[bookingsdomain.BookingID, bookingsdomain.BookingSnapshot]) error {
		_, _, err := repo.Load(ctx, "nonexistent")
		return err
	})
	if !errors.Is(err, dmpfports.ErrNotFound) {
		t.Fatalf("Load(absent) = %v, want ErrNotFound", err)
	}
}
