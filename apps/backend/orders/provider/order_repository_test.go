//go:build integration

package provider_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/provider"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

const repoOrderID = domain.OrderID("o-1001")

func snapshot(quantity int) domain.Snapshot {
	return domain.Snapshot{
		ID:        repoOrderID,
		Status:    domain.Open,
		ItemLimit: 3,
		Items:     []domain.Item{{SKU: "sku-1", Quantity: quantity}},
	}
}

func TestOrderRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.OrderID, domain.Snapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.RepositorySubject[domain.OrderID, domain.Snapshot]{
			Within: func(ctx context.Context, fn func(context.Context, ports.Repository[domain.OrderID, domain.Snapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewOrderRepository, fn)
			},
			Reader:           provider.NewOrderReader(postgres.NewReadPool(pool)),
			NewID:            func(n int) domain.OrderID { return domain.OrderID("kit-" + strconv.Itoa(n)) },
			NewState:         snapshot,
			Marker:           func(s domain.Snapshot) int { return s.Items[0].Quantity },
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}
