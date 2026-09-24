//go:build integration

package provider_test

import (
	"context"
	"errors"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const repoBookingID = domain.BookingID("b-1001")

type repoResources struct {
	Bookings ports.Repository[domain.BookingID, domain.BookingSnapshot]
}

func bindRepo(tx *postgres.Tx) repoResources {
	return repoResources{Bookings: provider.NewBookingRepository(tx)}
}

func snap(quantity int) domain.BookingSnapshot {
	return domain.BookingSnapshot{
		ID:         repoBookingID,
		ResourceID: "r-200",
		Quantity:   quantity,
		Status:     domain.BookingReservedStatus,
		ReservedAt: 1755432000,
	}
}

func withRepo(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context, repo ports.Repository[domain.BookingID, domain.BookingSnapshot]) error) error {
	t.Helper()
	uow := postgres.NewUnitOfWork(pool, bindRepo)
	return uow.Within(withExecution(t, context.Background()), func(ctx context.Context, res repoResources) error {
		return fn(ctx, res.Bookings)
	})
}

func seed(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.BookingID, domain.BookingSnapshot]) error {
		return repo.Save(ctx, repoBookingID, snap(5), 0)
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func loadFromPool(t *testing.T, pool *pgxpool.Pool) (domain.BookingSnapshot, ports.Version) {
	t.Helper()
	reader := provider.NewBookingReader(postgres.NewReadPool(pool))
	s, v, err := reader.Load(withExecution(t, context.Background()), repoBookingID)
	if err != nil {
		t.Fatalf("reader.Load() = %v", err)
	}
	return s, v
}

func TestSaveCreatesAndLoadReturnsIt(t *testing.T) {
	pool := pg.OpenPool(t, "bookings_booking", "bookings_resource")

	if err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.BookingID, domain.BookingSnapshot]) error {
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
	pool := pg.OpenPool(t, "bookings_booking", "bookings_resource")
	seed(t, pool)

	updated := snap(10)
	updated.Status = domain.BookingCancelled
	if err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.BookingID, domain.BookingSnapshot]) error {
		return repo.Save(ctx, repoBookingID, updated, 1)
	}); err != nil {
		t.Fatalf("Save(update) = %v", err)
	}

	got, version := loadFromPool(t, pool)
	if version != 2 {
		t.Fatalf("version = %d, want 2", version)
	}
	if got.Status != domain.BookingCancelled {
		t.Fatalf("Status = %v, want BookingCancelled", got.Status)
	}
}

func TestSaveConflictsOnStaleVersion(t *testing.T) {
	pool := pg.OpenPool(t, "bookings_booking", "bookings_resource")
	seed(t, pool)

	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.BookingID, domain.BookingSnapshot]) error {
		return repo.Save(ctx, repoBookingID, snap(7), 0)
	})
	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("Save(stale) = %v, want ErrVersionConflict", err)
	}
}

func TestLoadReturnsNotFoundForAbsentBooking(t *testing.T) {
	pool := pg.OpenPool(t, "bookings_booking", "bookings_resource")

	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.BookingID, domain.BookingSnapshot]) error {
		_, _, err := repo.Load(ctx, "nonexistent")
		return err
	})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load(absent) = %v, want ErrNotFound", err)
	}
}
