package domainkit_test

import (
	"strconv"

	dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/orders"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/reservations"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/domainkit"
)

// The projections of the two example aggregates: how their state, responses
// and events read as abstract Fields (ORA-33). Shared by the pure tests and by
// the fixture-driven ones.

func orderState(o *orders.Order) domainkit.Fields {
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

func orderStatus(s orders.Status) string {
	if s == orders.Placed {
		return "placed"
	}
	return "open"
}

func orderFromState(f domainkit.Fields) *orders.Order {
	limit, _ := strconv.Atoi(f["item_limit"])
	count, _ := strconv.Atoi(f["items.count"])
	s := orders.Snapshot{ID: orders.OrderID(f["id"]), ItemLimit: limit, Items: []orders.Item{}}
	if f["status"] == "placed" {
		s.Status = orders.Placed
	}
	for i := range count {
		q, _ := strconv.Atoi(f["items."+strconv.Itoa(i)+".quantity"])
		s.Items = append(s.Items, orders.Item{SKU: orders.SKU(f["items."+strconv.Itoa(i)+".sku"]), Quantity: q})
	}
	return orders.FromSnapshot(s)
}

func orderEvent(e dmpfdomain.DomainEvent) (string, domainkit.Fields) {
	switch ev := e.(type) {
	case orders.ItemAdded:
		return ev.EventName(), domainkit.Fields{"order": string(ev.Order), "sku": string(ev.SKU), "quantity": strconv.Itoa(ev.Quantity), "at": strconv.FormatInt(int64(ev.At), 10)}
	case orders.OrderPlaced:
		return ev.EventName(), domainkit.Fields{"order": string(ev.Order), "items": strconv.Itoa(ev.Items), "at": strconv.FormatInt(int64(ev.At), 10)}
	default:
		return e.EventName(), domainkit.Fields{}
	}
}

func cloneOrder(o *orders.Order) *orders.Order { return orders.FromSnapshot(o.Snapshot()) }

func addItemSubject(cmd orders.AddItem) domainkit.Subject[*orders.Order, orders.ItemAccepted] {
	return domainkit.Subject[*orders.Order, orders.ItemAccepted]{
		Decide: func(o *orders.Order) (dmpfdomain.Accepted[orders.ItemAccepted], *dmpfdomain.Rejection) {
			return o.AddItem(cmd)
		},
		Response: func(r orders.ItemAccepted) domainkit.Fields {
			return domainkit.Fields{"order": string(r.Order), "items": strconv.Itoa(r.Items)}
		},
		Event:    orderEvent,
		Snapshot: orderState,
		Clone:    cloneOrder,
	}
}

func placeSubject(cmd orders.PlaceOrder) domainkit.Subject[*orders.Order, orders.PlacedResponse] {
	return domainkit.Subject[*orders.Order, orders.PlacedResponse]{
		Decide: func(o *orders.Order) (dmpfdomain.Accepted[orders.PlacedResponse], *dmpfdomain.Rejection) {
			return o.Place(cmd)
		},
		Response: func(r orders.PlacedResponse) domainkit.Fields { return domainkit.Fields{"order": string(r.Order)} },
		Event:    orderEvent,
		Snapshot: orderState,
		Clone:    cloneOrder,
	}
}

func reservationState(r *reservations.Reservation) domainkit.Fields {
	s := r.Snapshot()
	status := "pending"
	if s.Status == reservations.Confirmed {
		status = "confirmed"
	}
	return domainkit.Fields{"order": string(s.Order), "items": strconv.Itoa(s.Items), "status": status}
}

func reservationFromState(f domainkit.Fields) *reservations.Reservation {
	items, _ := strconv.Atoi(f["items"])
	s := reservations.Snapshot{Order: reservations.OrderID(f["order"]), Items: items}
	if f["status"] == "confirmed" {
		s.Status = reservations.Confirmed
	}
	return reservations.FromSnapshot(s)
}

func reserveSubject(cmd reservations.Reserve) domainkit.Subject[*reservations.Reservation, reservations.ReservedResponse] {
	return domainkit.Subject[*reservations.Reservation, reservations.ReservedResponse]{
		Decide: func(r *reservations.Reservation) (dmpfdomain.Accepted[reservations.ReservedResponse], *dmpfdomain.Rejection) {
			return r.Reserve(cmd)
		},
		Response: func(r reservations.ReservedResponse) domainkit.Fields {
			return domainkit.Fields{"order": string(r.Order), "items": strconv.Itoa(r.Items)}
		},
		Event: func(e dmpfdomain.DomainEvent) (string, domainkit.Fields) {
			switch ev := e.(type) {
			case reservations.ReservationConfirmed:
				return ev.EventName(), domainkit.Fields{"order": string(ev.Order), "items": strconv.Itoa(ev.Items), "at": strconv.FormatInt(int64(ev.At), 10)}
			default:
				return e.EventName(), domainkit.Fields{}
			}
		},
		Snapshot: reservationState,
		Clone: func(r *reservations.Reservation) *reservations.Reservation {
			return reservations.FromSnapshot(r.Snapshot())
		},
	}
}
