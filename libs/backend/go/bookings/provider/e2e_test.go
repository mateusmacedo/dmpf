//go:build integration

package bookingspostgres_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
	dmpfpostgres "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-postgres"

	bookingsapplication "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/application"
	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
	bookingspostgres "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/provider"
)

const (
	e2eBookingID  = bookingsdomain.BookingID("e2e-b-001")
	e2eResourceID = bookingsdomain.ResourceID("e2e-r-001")
	e2eOccurred   = dmpfports.Instant(1_755_432_000)
)

type fixedClock struct{}

func (fixedClock) Now() dmpfports.Instant { return e2eOccurred }

type sequenceIDs struct {
	mu     sync.Mutex
	issued int
}

func (g *sequenceIDs) NewMessageID() dmpfports.MessageID {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.issued++
	return dmpfports.MessageID(fmt.Sprintf("m-%06d", g.issued))
}

type outboxRow struct {
	MessageType      string
	SchemaVersion    string
	AggregateVersion int64
	Destination      string
	Status           string
}

func newService(pool *pgxpool.Pool) bookingsapplication.Service {
	bind := func(tx *dmpfpostgres.Tx) bookingsapplication.Resources {
		return bookingsapplication.Resources{
			Bookings:  bookingspostgres.NewBookingRepository(tx),
			Resources: bookingspostgres.NewResourceRepository(tx),
			Outbox:    tx.Outbox(bookingspostgres.Mapper{}),
		}
	}
	return bookingsapplication.Service{
		UoW:       dmpfpostgres.NewUnitOfWork(pool, bind),
		Reader:    bookingspostgres.NewBookingReader(pool),
		Clock:     fixedClock{},
		IDs:       &sequenceIDs{},
		Authorize: dmpfapplication.AllowAll[bookingsapplication.Command](),
	}
}

func TestReserveBookingEndToEnd(t *testing.T) {
	pool := openPool(t)
	service := newService(pool)
	ctx := context.Background()

	outcome, err := service.ReserveBooking(ctx, bookingsapplication.Reserve{
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
		snap, version, err := bookingspostgres.NewBookingReader(pool).Load(ctx, e2eBookingID)
		if err != nil {
			t.Fatalf("Load() = %v", err)
		}
		if version != 1 {
			t.Fatalf("version = %d, want 1", version)
		}
		if snap.Quantity != 3 || snap.ResourceID != e2eResourceID || snap.Status != bookingsdomain.BookingReservedStatus {
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

		cancelOutcome, err := service.CancelBooking(ctx, bookingsapplication.Cancel{
			BookingID: e2eBookingID,
		})
		if err != nil {
			t.Fatalf("CancelBooking() = %v, want nil", err)
		}
		if _, refused := cancelOutcome.Rejection(); refused {
			t.Fatal("CancelBooking() was rejected, want accepted")
		}

		snap, version, err := bookingspostgres.NewBookingReader(pool).Load(ctx, e2eBookingID)
		if err != nil {
			t.Fatalf("Load() = %v", err)
		}
		if version != 2 {
			t.Fatalf("version = %d, want 2", version)
		}
		if snap.Status != bookingsdomain.BookingCancelled {
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
