package domainkit_test

import (
	"strconv"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/orders"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/reservations"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
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

func orderEvent(e domain.DomainEvent) (string, domainkit.Fields) {
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
		Decide: func(o *orders.Order) (domain.Accepted[orders.ItemAccepted], *domain.Rejection) {
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
		Decide: func(o *orders.Order) (domain.Accepted[orders.PlacedResponse], *domain.Rejection) {
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
	switch s.Status {
	case reservations.Confirmed:
		status = "confirmed"
	case reservations.Canceled:
		status = "canceled"
	}
	return domainkit.Fields{"order": string(s.Order), "items": strconv.Itoa(s.Items), "status": status}
}

func reservationFromState(f domainkit.Fields) *reservations.Reservation {
	items, _ := strconv.Atoi(f["items"])
	s := reservations.Snapshot{Order: reservations.OrderID(f["order"]), Items: items}
	switch f["status"] {
	case "confirmed":
		s.Status = reservations.Confirmed
	case "canceled":
		s.Status = reservations.Canceled
	}
	return reservations.FromSnapshot(s)
}

func reserveSubject(cmd reservations.Reserve) domainkit.Subject[*reservations.Reservation, reservations.ReservedResponse] {
	return domainkit.Subject[*reservations.Reservation, reservations.ReservedResponse]{
		Decide: func(r *reservations.Reservation) (domain.Accepted[reservations.ReservedResponse], *domain.Rejection) {
			return r.Reserve(cmd)
		},
		Response: func(r reservations.ReservedResponse) domainkit.Fields {
			return domainkit.Fields{"order": string(r.Order), "items": strconv.Itoa(r.Items)}
		},
		Event:    reservationEvent,
		Snapshot: reservationState,
		Clone:    cloneReservation,
	}
}

func cancelSubject(cmd reservations.Cancel) domainkit.Subject[*reservations.Reservation, reservations.CancelledResponse] {
	return domainkit.Subject[*reservations.Reservation, reservations.CancelledResponse]{
		Decide: func(r *reservations.Reservation) (domain.Accepted[reservations.CancelledResponse], *domain.Rejection) {
			return r.Cancel(cmd)
		},
		Response: func(r reservations.CancelledResponse) domainkit.Fields {
			return domainkit.Fields{"order": string(r.Order)}
		},
		Event:    reservationEvent,
		Snapshot: reservationState,
		Clone:    cloneReservation,
	}
}

func reservationEvent(e domain.DomainEvent) (string, domainkit.Fields) {
	switch ev := e.(type) {
	case reservations.ReservationConfirmed:
		return ev.EventName(), domainkit.Fields{"order": string(ev.Order), "items": strconv.Itoa(ev.Items), "at": strconv.FormatInt(int64(ev.At), 10)}
	case reservations.ReservationCancelled:
		return ev.EventName(), domainkit.Fields{"order": string(ev.Order), "at": strconv.FormatInt(int64(ev.At), 10)}
	default:
		return e.EventName(), domainkit.Fields{}
	}
}

func cloneReservation(r *reservations.Reservation) *reservations.Reservation {
	return reservations.FromSnapshot(r.Snapshot())
}
