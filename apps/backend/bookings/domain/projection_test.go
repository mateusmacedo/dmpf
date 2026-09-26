package domain_test

import (
	"strconv"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

func TestBookingsMatchTheProjectionFixture(t *testing.T) {
	f := tb.LoadProjection(t, "contracts/fixtures/bookings/projection/v1/booking.golden")
	if f.Identity.Aggregate != "booking" || len(f.Cases) != 4 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	var decided domainkit.Verdict
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			got, twice := runBooking(t, c)
			equal := domainkit.Equal(got, c.Expected.Projection())
			tb.Require(t, equal)
			tb.Require(t, twice)
			collect(&decided, equal, twice)
		})
	}
	evidence.RecordVerdict(t, "domain", "bookings", decided)
}

func collect(into *domainkit.Verdict, verdicts ...domainkit.Verdict) {
	for _, v := range verdicts {
		into.Diagnostics = append(into.Diagnostics, v.Diagnostics...)
	}
}

func runBooking(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	when := domain.Instant(integer(t, c.Command["at"]))
	switch c.Command["upr"] {
	case "reserve":
		s := reserveSubject(domain.ReserveBooking{
			ResourceID: domain.ResourceID(c.Command["resource"]),
			Quantity:   int(integer(t, c.Command["quantity"])),
			At:         when,
		})
		return domainkit.Run(bookingFromState(t, c.StateBefore), s), domainkit.ReadTwice(bookingFromState(t, c.StateBefore), s)
	case "cancel":
		s := cancelSubject(domain.CancelBooking{At: when})
		return domainkit.Run(bookingFromState(t, c.StateBefore), s), domainkit.ReadTwice(bookingFromState(t, c.StateBefore), s)
	default:
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
		return domainkit.Projection{}, domainkit.Verdict{}
	}
}

func integer(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatalf("integer %q: %v", s, err)
	}
	return n
}

var statusNames = map[domain.BookingStatus]string{
	domain.New:       "new",
	domain.Reserved:  "reserved",
	domain.Cancelled: "cancelled",
}

func bookingState(b *domain.Booking) domainkit.Fields {
	s := b.Snapshot()
	return domainkit.Fields{
		"booking":     string(s.ID),
		"resource":    string(s.ResourceID),
		"quantity":    strconv.Itoa(s.Quantity),
		"status":      statusNames[s.Status],
		"reserved_at": strconv.FormatInt(int64(s.ReservedAt), 10),
	}
}

func bookingFromState(t *testing.T, f domainkit.Fields) *domain.Booking {
	t.Helper()
	s := domain.BookingSnapshot{
		ID:         domain.BookingID(f["booking"]),
		ResourceID: domain.ResourceID(f["resource"]),
		Quantity:   int(integer(t, f["quantity"])),
		ReservedAt: domain.Instant(integer(t, f["reserved_at"])),
	}
	for status, name := range statusNames {
		if name == f["status"] {
			s.Status = status
		}
	}
	return domain.FromBookingSnapshot(s)
}

func bookingEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	switch ev := e.(type) {
	case domain.BookingReserved:
		return ev.EventName(), domainkit.Fields{
			"booking":  string(ev.BookingID),
			"resource": string(ev.ResourceID),
			"quantity": strconv.Itoa(ev.Quantity),
			"at":       strconv.FormatInt(int64(ev.At), 10),
		}
	case domain.BookingCancelled:
		return ev.EventName(), domainkit.Fields{"booking": string(ev.BookingID), "at": strconv.FormatInt(int64(ev.At), 10)}
	default:
		return e.EventName(), domainkit.Fields{}
	}
}

func cloneBooking(b *domain.Booking) *domain.Booking {
	return domain.FromBookingSnapshot(b.Snapshot())
}

func reserveSubject(cmd domain.ReserveBooking) domainkit.Subject[*domain.Booking, domain.ReservedResponse] {
	return domainkit.Subject[*domain.Booking, domain.ReservedResponse]{
		Decide: func(b *domain.Booking) (kernel.Accepted[domain.ReservedResponse], *kernel.Rejection) {
			return b.Reserve(cmd)
		},
		Response: func(r domain.ReservedResponse) domainkit.Fields {
			return domainkit.Fields{"booking": string(r.BookingID)}
		},
		Event:    bookingEvent,
		Snapshot: bookingState,
		Clone:    cloneBooking,
	}
}

func cancelSubject(cmd domain.CancelBooking) domainkit.Subject[*domain.Booking, domain.CancelledResponse] {
	return domainkit.Subject[*domain.Booking, domain.CancelledResponse]{
		Decide: func(b *domain.Booking) (kernel.Accepted[domain.CancelledResponse], *kernel.Rejection) {
			return b.Cancel(cmd)
		},
		Response: func(r domain.CancelledResponse) domainkit.Fields {
			return domainkit.Fields{"booking": string(r.BookingID)}
		},
		Event:    bookingEvent,
		Snapshot: bookingState,
		Clone:    cloneBooking,
	}
}
