package domainkit_test

import (
	"strconv"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// The positive vectors of ORA-39, one per branch of each UPR, driven by the
// projection fixtures in contracts/ (ORA-30): the same file a TypeScript
// kernel will read against its own aggregates.

func TestOrdersMatchTheProjectionFixture(t *testing.T) {
	f := tb.LoadProjection(t, "contracts/fixtures/orders/projection/v1/order.golden")
	if f.Identity.Aggregate != "order" || len(f.Cases) != 5 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	var decided domainkit.Verdict
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			got, twice := runOrder(t, c)
			equal := domainkit.Equal(got, c.Expected.Projection())
			tb.Require(t, equal)
			tb.Require(t, twice)
			collect(&decided, equal, twice)
		})
	}
	evidence.RecordVerdict(t, "domain", "orders", decided)
}

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

func runOrder(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	at := orders.Instant(instant(t, c.Command["at"]))
	switch c.Command["upr"] {
	case "add-item":
		q, _ := strconv.Atoi(c.Command["quantity"])
		s := addItemSubject(orders.AddItem{SKU: orders.SKU(c.Command["sku"]), Quantity: q, At: at})
		return domainkit.Run(orderFromState(c.StateBefore), s), domainkit.ReadTwice(orderFromState(c.StateBefore), s)
	case "place":
		s := placeSubject(orders.PlaceOrder{At: at})
		return domainkit.Run(orderFromState(c.StateBefore), s), domainkit.ReadTwice(orderFromState(c.StateBefore), s)
	default:
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
		return domainkit.Projection{}, domainkit.Verdict{}
	}
}

func runReservation(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	at := reservations.Instant(instant(t, c.Command["at"]))
	switch c.Command["upr"] {
	case "reserve":
		items, _ := strconv.Atoi(c.Command["items"])
		s := reserveSubject(reservations.Reserve{Items: items, At: at})
		return domainkit.Run(reservationFromState(c.StateBefore), s), domainkit.ReadTwice(reservationFromState(c.StateBefore), s)
	case "cancel":
		s := cancelSubject(reservations.Cancel{At: at})
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
