//go:build integration

package provider_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/provider"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

const repoOrderID = domain.OrderID("order-1")

func snapshot(items int) domain.Snapshot {
	return domain.Snapshot{
		Order:  repoOrderID,
		Items:  items,
		Status: domain.Confirmed,
	}
}

func TestReservationRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.OrderID, domain.Snapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.RepositorySubject[domain.OrderID, domain.Snapshot]{
			Within: func(ctx context.Context, fn func(context.Context, ports.Repository[domain.OrderID, domain.Snapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewReservationRepository, fn)
			},
			Reader:           provider.NewReservationReader(postgres.NewReadPool(pool)),
			NewID:            func(n int) domain.OrderID { return domain.OrderID("kit-" + strconv.Itoa(n)) },
			NewState:         snapshot,
			Marker:           func(s domain.Snapshot) int { return s.Items },
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

func TestReservationRoundTripsTheWholeState(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	pg.Seed(t, ctx, pool, provider.NewReservationRepository, repoOrderID, snapshot(5))

	got, version := pg.Load(t, ctx, pool, provider.NewReservationRepository, repoOrderID)

	if version != 1 || !got.Equal(snapshot(5)) {
		t.Fatalf("Load() = %+v v%d, want %+v v1 — the snapshot round trip lost state", got, version, snapshot(5))
	}
}
