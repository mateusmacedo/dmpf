package bookingsdomain_test

import (
	"testing"

	bookingsdomain "github.com/mateusmacedo/dmpf/libs/backend/go/bookings/domain"
)

func TestRegisterAcceptsNewResource(t *testing.T) {
	r := bookingsdomain.NewResource(resCode)

	acc, rej := r.Register(bookingsdomain.RegisterResource{Code: resCode, At: at})

	requireAccepted[bookingsdomain.RegisteredResponse](t, rej)
	if got, want := acc.Response(), (bookingsdomain.RegisteredResponse{Code: resCode}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(events))
	}
	ev := events[0].(bookingsdomain.ResourceRegistered)
	if ev.Code != resCode || ev.At != at {
		t.Fatalf("ResourceRegistered = %+v", ev)
	}
}

func TestRegisterIdempotentWhenAlreadyPresent(t *testing.T) {
	r := newRegisteredResource(t)
	before := r.Snapshot()

	acc, rej := r.Register(bookingsdomain.RegisterResource{Code: resCode, At: at + 1000})

	requireAccepted[bookingsdomain.RegisteredResponse](t, rej)
	if len(acc.Events()) != 0 {
		t.Fatalf("Events() len = %d, want 0 (idempotent)", len(acc.Events()))
	}
	if !r.Snapshot().Equal(before) {
		t.Fatalf("idempotent Register must not change state\nbefore: %+v\nafter:  %+v", before, r.Snapshot())
	}
}

func TestRegisterRejectsEmptyCode(t *testing.T) {
	r := bookingsdomain.NewResource("")

	acc, rej := r.Register(bookingsdomain.RegisterResource{Code: "", At: at})

	requireRejected(t, acc, rej, bookingsdomain.CodeCodeEmpty)
}
