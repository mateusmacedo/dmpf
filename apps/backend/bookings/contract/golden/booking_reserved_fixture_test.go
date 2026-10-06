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

var bookingReservedSpec = tb.Spec[proto.Message]{
	Path:    "apps/backend/bookings/contract/fixtures/event/v1/booking-reserved.golden",
	Source:  "urn:dmpf:reference-bookings",
	Subject: "b-1001",
	Identity: golden.Identity{
		Fixture:    "bookings/event/v1/booking-reserved",
		Contract:   golden.Contract{Package: "company.bookings.event.v1", Message: "BookingReserved"},
		Type:       "com.company.bookings.booking-reserved.v1",
		DataSchema: "type.googleapis.com/company.bookings.event.v1.BookingReserved",
	},
	Unknown:    7,
	New:        func() proto.Message { return &eventv1.BookingReserved{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return bookingReserved(fields) },
	Build: leanFixture(
		bookingReservedFields("b-1001", "room-1", "2", "2026-09-02T12:00:00Z"),
		bookingReservedFields("b-1002", "room-2", "1", "2026-09-02T12:00:01Z"),
		bookingReservedFields("b-2001", "room-1", "2", "2026-09-02T12:00:02Z"),
		bookingReservedFields("b-2002", "room-2", "5", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func bookingReservedFields(bookingID, resourceID, quantity, reservedAt string) map[string]string {
	return map[string]string{
		"booking_id":  bookingID,
		"resource_id": resourceID,
		"quantity":    quantity,
		"reserved_at": reservedAt,
	}
}

func bookingReserved(fields map[string]string) (*eventv1.BookingReserved, error) {
	quantity, err := tb.ParseInt(fields, "quantity", 32)
	if err != nil {
		return nil, err
	}
	reservedAt, err := time.Parse(time.RFC3339Nano, fields["reserved_at"])
	if err != nil {
		return nil, fmt.Errorf("reserved_at %q: %w", fields["reserved_at"], err)
	}
	return &eventv1.BookingReserved{
		BookingId:  fields["booking_id"],
		ResourceId: fields["resource_id"],
		Quantity:   int32(quantity),
		ReservedAt: timestamppb.New(reservedAt),
	}, nil
}
