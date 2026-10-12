package app_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/app"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

type absentOrder struct{}

func (absentOrder) Load(context.Context, domain.OrderID) (domain.Snapshot, ports.Version, error) {
	return domain.Snapshot{}, 0, ports.ErrNotFound
}

func TestAFindOfAnAbsentOrderIsNotFoundOnTheUseCaseSpanAndItsRED(t *testing.T) {
	obstest.NotFoundOnTheUseCase(t, "orders", application.OperationFindOrder, func(ctx context.Context, rt *otelboot.Runtime, pool *pgxpool.Pool) error {
		service, err := app.NewOrdersService(pool, rt, app.Defaults(app.RoleAPI))
		if err != nil {
			return err
		}
		service.Reader = absentOrder{}
		_, err = service.FindOrder(ctx, "order-404")
		return err
	})
}
