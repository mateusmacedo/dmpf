package golden

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventv1 "github.com/mateusmacedo/dmpf/apps/backend/bookings/contract/gen/go/company/bookings/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

var bookingCancelledSpec = tb.Spec[proto.Message]{
	Path:    "apps/backend/bookings/contract/fixtures/event/v1/booking-cancelled.golden",
	Source:  "urn:dmpf:reference-bookings",
	Subject: "b-1001",
	Identity: golden.Identity{
		Fixture:    "bookings/event/v1/booking-cancelled",
		Contract:   golden.Contract{Package: "company.bookings.event.v1", Message: "BookingCancelled"},
		Type:       "com.company.bookings.booking-cancelled.v1",
		DataSchema: "type.googleapis.com/company.bookings.event.v1.BookingCancelled",
	},
	Unknown:    7,
	New:        func() proto.Message { return &eventv1.BookingCancelled{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return bookingCancelled(fields) },
	Build: leanFixture(
		bookingCancelledFields("b-1001", "2026-09-02T12:00:00Z"),
		bookingCancelledFields("b-1002", "2026-09-02T12:00:01Z"),
		bookingCancelledFields("b-2001", "2026-09-02T12:00:02Z"),
		bookingCancelledFields("b-2002", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func bookingCancelledFields(bookingID, cancelledAt string) map[string]string {
	return map[string]string{
		"booking_id":   bookingID,
		"cancelled_at": cancelledAt,
	}
}

func bookingCancelled(fields map[string]string) (*eventv1.BookingCancelled, error) {
	cancelledAt, err := time.Parse(time.RFC3339Nano, fields["cancelled_at"])
	if err != nil {
		return nil, fmt.Errorf("cancelled_at %q: %w", fields["cancelled_at"], err)
	}
	return &eventv1.BookingCancelled{
		BookingId:   fields["booking_id"],
		CancelledAt: timestamppb.New(cancelledAt),
	}, nil
}
