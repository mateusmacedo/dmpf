//go:build integration

package provider_test

import (
	"context"
	"fmt"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/application"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/provider"
)

const (
	e2eBookingID  = domain.BookingID("e2e-b-001")
	e2eResourceID = domain.ResourceID("e2e-r-001")
	e2eOccurred   = ports.Instant(1_755_432_000)
)

type fixedClock struct{}

func (fixedClock) Now() ports.Instant { return e2eOccurred }

type sequenceIDs struct {
	mu     sync.Mutex
	issued int
}

func (g *sequenceIDs) NewMessageID() ports.MessageID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.issued++
	return ports.MessageID(fmt.Sprintf("m-%06d", g.issued))
}

type outboxRow struct {
	MessageType      string
	SchemaVersion    string
	AggregateVersion int64
	Destination      string
	Status           string
}

func newService(pool *pgxpool.Pool) application.Service {
	bind := func(tx *postgres.Tx) application.Resources {
		return application.Resources{
			Bookings:  provider.NewBookingRepository(tx),
			Resources: provider.NewResourceRepository(tx),
			Outbox:    tx.Outbox(provider.Mapper{}),
		}
	}
	return application.Service{
		UoW:       postgres.NewUnitOfWork(pool, bind),
		Reader:    provider.NewBookingReader(pool),
		Clock:     fixedClock{},
		IDs:       &sequenceIDs{},
		Authorize: usecase.AllowAll[application.Command](),
	}
}

func TestReserveBookingEndToEnd(t *testing.T) {
	pool := pg.OpenPool(t, "bookings_booking", "bookings_resource")
	service := newService(pool)
	ctx := context.Background()

	outcome, err := service.ReserveBooking(ctx, application.Reserve{
		BookingID:  e2eBookingID,
		ResourceID: e2eResourceID,
		Quantity:   3,
	})
	if err != nil {
		t.Fatalf("ReserveBooking() = %v, want nil", err)
	}
	if _, refused := outcome.Rejection(); refused {
		t.Fatal("ReserveBooking() was rejected, want accepted")
	}

	t.Run("the booking is persisted", func(t *testing.T) {
		snap, version, err := provider.NewBookingReader(pool).Load(ctx, e2eBookingID)
		if err != nil {
			t.Fatalf("Load() = %v", err)
		}
		if version != 1 {
			t.Fatalf("version = %d, want 1", version)
		}
		if snap.Quantity != 3 || snap.ResourceID != e2eResourceID || snap.Status != domain.BookingReservedStatus {
			t.Fatalf("snapshot = %+v", snap)
		}
	})

	t.Run("the outbox row lands at version 1", func(t *testing.T) {
		row := outboxRowOf(t, pool, "m-000001")
		want := outboxRow{
			MessageType:      "com.company.bookings.booking-reserved.v1",
			SchemaVersion:    "type.googleapis.com/company.bookings.event.v1.BookingReserved",
			AggregateVersion: 1,
			Destination:      "bookings.events",
			Status:           "pending",
		}
		if row != want {
			t.Fatalf("outbox row = %+v, want %+v", row, want)
		}
	})

	t.Run("cancel commits without outbox row", func(t *testing.T) {
		_, outboxBefore := counts(t, pool)

		cancelOutcome, err := service.CancelBooking(ctx, application.Cancel{
			BookingID: e2eBookingID,
		})
		if err != nil {
			t.Fatalf("CancelBooking() = %v, want nil", err)
		}
		if _, refused := cancelOutcome.Rejection(); refused {
			t.Fatal("CancelBooking() was rejected, want accepted")
		}

		snap, version, err := provider.NewBookingReader(pool).Load(ctx, e2eBookingID)
		if err != nil {
			t.Fatalf("Load() = %v", err)
		}
		if version != 2 {
			t.Fatalf("version = %d, want 2", version)
		}
		if snap.Status != domain.BookingCancelled {
			t.Fatalf("Status = %v, want BookingCancelled", snap.Status)
		}

		_, outboxAfter := counts(t, pool)
		if outboxAfter != outboxBefore {
			t.Fatalf("outbox count moved on cancel: %d→%d (internal event must not enqueue)", outboxBefore, outboxAfter)
		}
	})
}

func outboxRowOf(t *testing.T, pool *pgxpool.Pool, messageID string) outboxRow {
	t.Helper()
	var row outboxRow
	err := pool.QueryRow(context.Background(), `
		SELECT message_type, schema_version, aggregate_version, destination, status
		FROM dmpf_outbox WHERE message_id = $1`, messageID).Scan(
		&row.MessageType, &row.SchemaVersion, &row.AggregateVersion, &row.Destination, &row.Status)
	if err != nil {
		t.Fatalf("SELECT outbox row %s = %v, want nil", messageID, err)
	}
	return row
}

func counts(t *testing.T, pool *pgxpool.Pool) (int, int) {
	t.Helper()
	var bookingsCount, outboxCount int
	err := pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM bookings_booking), (SELECT count(*) FROM dmpf_outbox)`).
		Scan(&bookingsCount, &outboxCount)
	if err != nil {
		t.Fatalf("counts = %v, want nil", err)
	}
	return bookingsCount, outboxCount
}
