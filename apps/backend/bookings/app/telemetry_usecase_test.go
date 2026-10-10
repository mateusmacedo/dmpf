package app_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/app"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

type absentBooking struct{}

func (absentBooking) Load(context.Context, domain.BookingID) (domain.BookingSnapshot, ports.Version, error) {
	return domain.BookingSnapshot{}, 0, ports.ErrNotFound
}

func TestAFindOfAnAbsentBookingIsNotFoundOnTheUseCaseSpanAndItsRED(t *testing.T) {
	obstest.NotFoundOnTheUseCase(t, "bookings", application.OperationFindBooking, func(ctx context.Context, rt *otelboot.Runtime, pool *pgxpool.Pool) error {
		service, err := app.NewBookingsService(pool, rt, app.Defaults(app.RoleAPI))
		if err != nil {
			return err
		}
		service.Reader = absentBooking{}
		_, err = service.FindBooking(ctx, "booking-404")
		return err
	})
}
