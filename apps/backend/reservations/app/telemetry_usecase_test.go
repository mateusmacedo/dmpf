package app_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

type absentReservation struct{}

func (absentReservation) Load(context.Context, domain.OrderID) (domain.Snapshot, ports.Version, error) {
	return domain.Snapshot{}, 0, ports.ErrNotFound
}

func TestAFindOfAnAbsentReservationIsNotFoundOnTheUseCaseSpanAndItsRED(t *testing.T) {
	obstest.NotFoundOnTheUseCase(t, "reservations", application.OperationFindReservation, func(ctx context.Context, rt *otelboot.Runtime, pool *pgxpool.Pool) error {
		service, err := app.NewReservationsService(pool, rt, app.Defaults(app.RoleAPI))
		if err != nil {
			return err
		}
		service.Reader = absentReservation{}
		_, err = service.FindReservation(ctx, "order-404")
		return err
	})
}
