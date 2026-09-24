package domain_test

import (
	"strconv"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/evidence"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// The positive vectors of ORA-39, one per branch of each UPR, driven by the
// projection fixture in contracts/ (ORA-30): the same file a TypeScript
// kernel will read against its own aggregate.

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

func collect(into *domainkit.Verdict, verdicts ...domainkit.Verdict) {
	for _, v := range verdicts {
		into.Diagnostics = append(into.Diagnostics, v.Diagnostics...)
	}
}

func runOrder(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	when := domain.Instant(instant(t, c.Command["at"]))
	switch c.Command["upr"] {
	case "add-item":
		q, _ := strconv.Atoi(c.Command["quantity"])
		s := addItemSubject(domain.AddItem{SKU: domain.SKU(c.Command["sku"]), Quantity: q, At: when})
		return domainkit.Run(orderFromState(c.StateBefore), s), domainkit.ReadTwice(orderFromState(c.StateBefore), s)
	case "place":
		s := placeSubject(domain.PlaceOrder{At: when})
		return domainkit.Run(orderFromState(c.StateBefore), s), domainkit.ReadTwice(orderFromState(c.StateBefore), s)
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

func orderState(o *domain.Order) domainkit.Fields {
	s := o.Snapshot()
	f := domainkit.Fields{
		"id":          string(s.ID),
		"status":      orderStatus(s.Status),
		"item_limit":  strconv.Itoa(s.ItemLimit),
		"items.count": strconv.Itoa(len(s.Items)),
	}
	for i, it := range s.Items {
		f["items."+strconv.Itoa(i)+".sku"] = string(it.SKU)
		f["items."+strconv.Itoa(i)+".quantity"] = strconv.Itoa(it.Quantity)
	}
	return f
}

func orderStatus(s domain.Status) string {
	if s == domain.Placed {
		return "placed"
	}
	return "open"
}

func orderFromState(f domainkit.Fields) *domain.Order {
	limit, _ := strconv.Atoi(f["item_limit"])
	count, _ := strconv.Atoi(f["items.count"])
	s := domain.Snapshot{ID: domain.OrderID(f["id"]), ItemLimit: limit, Items: []domain.Item{}}
	if f["status"] == "placed" {
		s.Status = domain.Placed
	}
	for i := range count {
		q, _ := strconv.Atoi(f["items."+strconv.Itoa(i)+".quantity"])
		s.Items = append(s.Items, domain.Item{SKU: domain.SKU(f["items."+strconv.Itoa(i)+".sku"]), Quantity: q})
	}
	return domain.FromSnapshot(s)
}

func orderEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	switch ev := e.(type) {
	case domain.ItemAdded:
		return ev.EventName(), domainkit.Fields{"order": string(ev.Order), "sku": string(ev.SKU), "quantity": strconv.Itoa(ev.Quantity), "at": strconv.FormatInt(int64(ev.At), 10)}
	case domain.OrderPlaced:
		return ev.EventName(), domainkit.Fields{"order": string(ev.Order), "items": strconv.Itoa(ev.Items), "at": strconv.FormatInt(int64(ev.At), 10)}
	default:
		return e.EventName(), domainkit.Fields{}
	}
}

func cloneOrder(o *domain.Order) *domain.Order { return domain.FromSnapshot(o.Snapshot()) }

func addItemSubject(cmd domain.AddItem) domainkit.Subject[*domain.Order, domain.ItemAccepted] {
	return domainkit.Subject[*domain.Order, domain.ItemAccepted]{
		Decide: func(o *domain.Order) (kernel.Accepted[domain.ItemAccepted], *kernel.Rejection) {
			return o.AddItem(cmd)
		},
		Response: func(r domain.ItemAccepted) domainkit.Fields {
			return domainkit.Fields{"order": string(r.Order), "items": strconv.Itoa(r.Items)}
		},
		Event:    orderEvent,
		Snapshot: orderState,
		Clone:    cloneOrder,
	}
}

func placeSubject(cmd domain.PlaceOrder) domainkit.Subject[*domain.Order, domain.PlacedResponse] {
	return domainkit.Subject[*domain.Order, domain.PlacedResponse]{
		Decide: func(o *domain.Order) (kernel.Accepted[domain.PlacedResponse], *kernel.Rejection) {
			return o.Place(cmd)
		},
		Response: func(r domain.PlacedResponse) domainkit.Fields { return domainkit.Fields{"order": string(r.Order)} },
		Event:    orderEvent,
		Snapshot: orderState,
		Clone:    cloneOrder,
	}
}
