package domain_test

import (
	"strconv"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// The positive vectors of ORA-39, one per branch of each UPR, driven by the
// projection fixture in contracts/ (ORA-30): the same file a TypeScript
// kernel will read against its own aggregate.

func TestReservationsMatchTheProjectionFixture(t *testing.T) {
	f := tb.LoadProjection(t, "contracts/fixtures/reservations/projection/v1/reservation.golden")
	if f.Identity.Aggregate != "reservation" || len(f.Cases) != 7 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	var decided domainkit.Verdict
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			got, twice := runReservation(t, c)
			equal := domainkit.Equal(got, c.Expected.Projection())
			tb.Require(t, equal)
			tb.Require(t, twice)
			collect(&decided, equal, twice)
		})
	}
	evidence.RecordVerdict(t, "domain", "reservations", decided)
}

func collect(into *domainkit.Verdict, verdicts ...domainkit.Verdict) {
	for _, v := range verdicts {
		into.Diagnostics = append(into.Diagnostics, v.Diagnostics...)
	}
}

func runReservation(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	when := domain.Instant(instant(t, c.Command["at"]))
	switch c.Command["upr"] {
	case "reserve":
		items, _ := strconv.Atoi(c.Command["items"])
		s := reserveSubject(domain.Reserve{Items: items, At: when})
		return domainkit.Run(reservationFromState(c.StateBefore), s), domainkit.ReadTwice(reservationFromState(c.StateBefore), s)
	case "cancel":
		s := cancelSubject(domain.Cancel{At: when})
		return domainkit.Run(reservationFromState(c.StateBefore), s), domainkit.ReadTwice(reservationFromState(c.StateBefore), s)
	default:
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
		return domainkit.Projection{}, domainkit.Verdict{}
	}
}

func instant(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatalf("at %q: %v", s, err)
	}
	return n
}

// The projection of the aggregate: how its state, responses and events read
// as abstract Fields (ORA-33).

func reservationState(r *domain.Reservation) domainkit.Fields {
	s := r.Snapshot()
	status := "pending"
	switch s.Status {
	case domain.Confirmed:
		status = "confirmed"
	case domain.Canceled:
		status = "canceled"
	}
	return domainkit.Fields{"order": string(s.Order), "items": strconv.Itoa(s.Items), "status": status}
}

func reservationFromState(f domainkit.Fields) *domain.Reservation {
	items, _ := strconv.Atoi(f["items"])
	s := domain.Snapshot{Order: domain.OrderID(f["order"]), Items: items}
	switch f["status"] {
	case "confirmed":
		s.Status = domain.Confirmed
	case "canceled":
		s.Status = domain.Canceled
	}
	return domain.FromSnapshot(s)
}

func reservationEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	switch ev := e.(type) {
	case domain.ReservationConfirmed:
		return ev.EventName(), domainkit.Fields{"order": string(ev.Order), "items": strconv.Itoa(ev.Items), "at": strconv.FormatInt(int64(ev.At), 10)}
	case domain.ReservationCancelled:
		return ev.EventName(), domainkit.Fields{"order": string(ev.Order), "at": strconv.FormatInt(int64(ev.At), 10)}
	default:
		return e.EventName(), domainkit.Fields{}
	}
}

func cloneReservation(r *domain.Reservation) *domain.Reservation {
	return domain.FromSnapshot(r.Snapshot())
}

func reserveSubject(cmd domain.Reserve) domainkit.Subject[*domain.Reservation, domain.ReservedResponse] {
	return domainkit.Subject[*domain.Reservation, domain.ReservedResponse]{
		Decide: func(r *domain.Reservation) (kernel.Accepted[domain.ReservedResponse], *kernel.Rejection) {
			return r.Reserve(cmd)
		},
		Response: func(r domain.ReservedResponse) domainkit.Fields {
			return domainkit.Fields{"order": string(r.Order), "items": strconv.Itoa(r.Items)}
		},
		Event:    reservationEvent,
		Snapshot: reservationState,
		Clone:    cloneReservation,
	}
}

func cancelSubject(cmd domain.Cancel) domainkit.Subject[*domain.Reservation, domain.CancelledResponse] {
	return domainkit.Subject[*domain.Reservation, domain.CancelledResponse]{
		Decide: func(r *domain.Reservation) (kernel.Accepted[domain.CancelledResponse], *kernel.Rejection) {
			return r.Cancel(cmd)
		},
		Response: func(r domain.CancelledResponse) domainkit.Fields {
			return domainkit.Fields{"order": string(r.Order)}
		},
		Event:    reservationEvent,
		Snapshot: reservationState,
		Clone:    cloneReservation,
	}
}
