//go:build integration

package provider_test

import (
	"context"
	"errors"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/provider"
)

const repoOrderID = domain.OrderID("order-1")

type repoResources struct {
	Reservations ports.Repository[domain.OrderID, domain.Snapshot]
}

func bindRepo(tx *postgres.Tx) repoResources {
	return repoResources{Reservations: provider.NewRepository(tx)}
}

func snapshot(items int) domain.Snapshot {
	return domain.Snapshot{
		Order:  repoOrderID,
		Items:  items,
		Status: domain.Confirmed,
	}
}

func withRepo(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context, repo ports.Repository[domain.OrderID, domain.Snapshot]) error) error {
	t.Helper()
	uow := postgres.NewUnitOfWork(pool, bindRepo)
	return uow.Within(context.Background(), func(ctx context.Context, res repoResources) error {
		return fn(ctx, res.Reservations)
	})
}

func seed(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.OrderID, domain.Snapshot]) error {
		return repo.Save(ctx, repoOrderID, snapshot(5), 0)
	}); err != nil {
		t.Fatalf("seed = %v", err)
	}
}

func load(t *testing.T, pool *pgxpool.Pool) (domain.Snapshot, ports.Version) {
	t.Helper()
	var (
		s domain.Snapshot
		v ports.Version
	)
	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.OrderID, domain.Snapshot]) error {
		var loadErr error
		s, v, loadErr = repo.Load(ctx, repoOrderID)
		return loadErr
	})
	if err != nil {
		t.Fatalf("load = %v", err)
	}
	return s, v
}

func TestRepositoryLoadAbsent(t *testing.T) {
	pool := pg.OpenPool(t)

	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.OrderID, domain.Snapshot]) error {
		_, _, err := repo.Load(ctx, repoOrderID)
		return err
	})
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound", err)
	}
}

func TestRepositorySaveCreate(t *testing.T) {
	pool := pg.OpenPool(t)
	seed(t, pool)

	s, v := load(t, pool)
	if v != 1 {
		t.Errorf("version = %d, want 1", v)
	}
	if !s.Equal(snapshot(5)) {
		t.Errorf("snapshot = %+v, want %+v", s, snapshot(5))
	}
}

func TestRepositorySaveUpdate(t *testing.T) {
	pool := pg.OpenPool(t)
	seed(t, pool)

	if err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.OrderID, domain.Snapshot]) error {
		return repo.Save(ctx, repoOrderID, snapshot(10), 1)
	}); err != nil {
		t.Fatalf("update = %v", err)
	}

	s, v := load(t, pool)
	if v != 2 {
		t.Errorf("version = %d, want 2", v)
	}
	if s.Items != 10 {
		t.Errorf("Items = %d, want 10", s.Items)
	}
}

func TestRepositoryVersionConflict(t *testing.T) {
	pool := pg.OpenPool(t)
	seed(t, pool)

	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.OrderID, domain.Snapshot]) error {
		return repo.Save(ctx, repoOrderID, snapshot(20), 99)
	})
	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("Save(stale version) = %v, want ErrVersionConflict", err)
	}
}

func TestRepositoryCreateOverExistingConflicts(t *testing.T) {
	pool := pg.OpenPool(t)
	seed(t, pool)

	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.OrderID, domain.Snapshot]) error {
		return repo.Save(ctx, repoOrderID, snapshot(30), 0)
	})
	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("Save(create over existing) = %v, want ErrVersionConflict (GAR-10)", err)
	}
}

func TestConcurrentSaveReservation(t *testing.T) {
	pool := pg.OpenPool(t)
	seed(t, pool)

	const writers = 2
	var (
		loaded  sync.WaitGroup
		release = make(chan struct{})
		results = make(chan error, writers)
		running sync.WaitGroup
	)
	loaded.Add(writers)
	running.Add(writers)

	for i := range writers {
		go func(quantity int) {
			defer running.Done()
			results <- withRepo(t, pool, func(ctx context.Context, repo ports.Repository[domain.OrderID, domain.Snapshot]) error {
				_, version, err := repo.Load(ctx, repoOrderID)
				if err != nil {
					return err
				}
				loaded.Done()
				<-release
				return repo.Save(ctx, repoOrderID, snapshot(quantity), version)
			})
		}(i + 10)
	}

	loaded.Wait()
	close(release)
	running.Wait()
	close(results)

	var committed, conflicted int
	for err := range results {
		switch {
		case err == nil:
			committed++
		case errors.Is(err, ports.ErrVersionConflict):
			conflicted++
		default:
			t.Fatalf("Within() = %v, want nil or ErrVersionConflict", err)
		}
	}

	if committed != 1 || conflicted != writers-1 {
		t.Fatalf("%d committed and %d conflicted, want 1 and %d",
			committed, conflicted, writers-1)
	}
}
