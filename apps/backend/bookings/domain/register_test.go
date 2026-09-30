package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
)

func TestRegisterAcceptsNewResource(t *testing.T) {
	r := domain.NewResource(resCode)

	acc, rej := r.Register(domain.RegisterResource{Code: resCode, At: at})

	requireAccepted[domain.RegisteredResponse](t, rej)
	if got, want := acc.Response(), (domain.RegisteredResponse{Code: resCode}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(events))
	}
	ev := events[0].(domain.ResourceRegistered)
	if ev.Code != resCode || ev.At != at {
		t.Fatalf("ResourceRegistered = %+v", ev)
	}
}

func TestRegisterIdempotentWhenAlreadyPresent(t *testing.T) {
	r := newRegisteredResource(t)
	before := r.Snapshot()

	acc, rej := r.Register(domain.RegisterResource{Code: resCode, At: at + 1000})

	requireAccepted[domain.RegisteredResponse](t, rej)
	if len(acc.Events()) != 0 {
		t.Fatalf("Events() len = %d, want 0 (idempotent)", len(acc.Events()))
	}
	if !r.Snapshot().Equal(before) {
		t.Fatalf("idempotent Register must not change state\nbefore: %+v\nafter:  %+v", before, r.Snapshot())
	}
}

func TestRegisterRejectsEmptyCode(t *testing.T) {
	r := domain.NewResource("")

	acc, rej := r.Register(domain.RegisterResource{Code: "", At: at})

	requireRejected(t, acc, rej, domain.CodeResourceCodeEmpty)
}
