//go:build integration

package orderspg_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	orderspg "github.com/mateusmacedo/dmpf/libs/backend/go/postgres/example/orders"
)

const repoOrderID = orders.OrderID("o-1001")

type repoResources struct {
	Orders ports.Repository[orders.OrderID, orders.Snapshot]
}

func bindRepo(tx *postgres.Tx) repoResources {
	return repoResources{Orders: orderspg.NewRepository(tx)}
}

func snapshot(quantity int) orders.Snapshot {
	return orders.Snapshot{
		ID:        repoOrderID,
		Status:    orders.Open,
		ItemLimit: 3,
		Items:     []orders.Item{{SKU: "sku-1", Quantity: quantity}},
	}
}

// withRepo runs one callback inside a transaction, which is the only way to
// reach the repository: it never opens a transaction of its own.
func withRepo(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context, repo ports.Repository[orders.OrderID, orders.Snapshot]) error) error {
	t.Helper()

	uow := postgres.NewUnitOfWork(pool, bindRepo)
	return uow.Within(context.Background(), func(ctx context.Context, res repoResources) error {
		return fn(ctx, res.Orders)
	})
}

func TestSaveCreatesAtVersionOne(t *testing.T) {
	pool := openPool(t)

	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[orders.OrderID, orders.Snapshot]) error {
		return repo.Save(ctx, repoOrderID, snapshot(1), 0)
	})
	if err != nil {
		t.Fatalf("Save(expected=0) = %v, want nil", err)
	}

	loaded, version := load(t, pool)
	if version != 1 {
		t.Fatalf("version = %d, want 1 — expected == 0 creates the aggregate", version)
	}
	if !loaded.Equal(snapshot(1)) {
		t.Fatalf("snapshot = %+v, want %+v — the jsonb round trip lost state", loaded, snapshot(1))
	}
}

func TestSaveAdvancesTheVersion(t *testing.T) {
	pool := openPool(t)
	seed(t, pool)

	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[orders.OrderID, orders.Snapshot]) error {
		return repo.Save(ctx, repoOrderID, snapshot(7), 1)
	})
	if err != nil {
		t.Fatalf("Save(expected=1) = %v, want nil", err)
	}

	loaded, version := load(t, pool)
	if version != 2 {
		t.Fatalf("version = %d, want 2", version)
	}
	if loaded.Items[0].Quantity != 7 {
		t.Fatalf("quantity = %d, want 7", loaded.Items[0].Quantity)
	}
}

func TestSaveRejectsADivergentExpectedVersion(t *testing.T) {
	pool := openPool(t)
	seed(t, pool)

	// Second write with the same expected version: the first already moved the
	// aggregate to 2, so this is the lost update optimistic locking exists for.
	if err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[orders.OrderID, orders.Snapshot]) error {
		return repo.Save(ctx, repoOrderID, snapshot(7), 1)
	}); err != nil {
		t.Fatalf("first Save(expected=1) = %v, want nil", err)
	}

	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[orders.OrderID, orders.Snapshot]) error {
		return repo.Save(ctx, repoOrderID, snapshot(9), 1)
	})

	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("second Save(expected=1) = %v, want ErrVersionConflict", err)
	}
	loaded, version := load(t, pool)
	if version != 2 || loaded.Items[0].Quantity != 7 {
		t.Fatalf("the store changed under a conflict: version %d, quantity %d", version, loaded.Items[0].Quantity)
	}
}

func TestSaveRejectsACreateOverAnExistingAggregate(t *testing.T) {
	pool := openPool(t)
	seed(t, pool)

	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[orders.OrderID, orders.Snapshot]) error {
		return repo.Save(ctx, repoOrderID, snapshot(9), 0)
	})

	if !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("Save(expected=0) over an existing aggregate = %v, want ErrVersionConflict", err)
	}
	if _, version := load(t, pool); version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}
}

func TestLoadReportsErrNotFound(t *testing.T) {
	pool := openPool(t)

	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[orders.OrderID, orders.Snapshot]) error {
		_, _, err := repo.Load(ctx, "o-404")
		return err
	})

	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Load() = %v, want ErrNotFound", err)
	}
}

func seed(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()

	if err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[orders.OrderID, orders.Snapshot]) error {
		return repo.Save(ctx, repoOrderID, snapshot(1), 0)
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func load(t *testing.T, pool *pgxpool.Pool) (orders.Snapshot, ports.Version) {
	t.Helper()

	var (
		loaded  orders.Snapshot
		version ports.Version
	)
	err := withRepo(t, pool, func(ctx context.Context, repo ports.Repository[orders.OrderID, orders.Snapshot]) error {
		var err error
		loaded, version, err = repo.Load(ctx, repoOrderID)
		return err
	})
	if err != nil {
		t.Fatalf("Load() = %v, want nil", err)
	}
	return loaded, version
}
