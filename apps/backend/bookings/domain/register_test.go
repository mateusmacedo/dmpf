package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

func TestRegisterAcceptsNewResource(t *testing.T) {
	r := domain.NewResource(resCode)

	acc, rej := r.Register(domain.RegisterResource{Code: resCode, Name: resName, At: at})

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.RegisteredResponse{Code: resCode}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	events := acc.Events()
	if len(events) != 1 {
		t.Fatalf("Events() len = %d, want 1", len(events))
	}
	ev := events[0].(domain.ResourceRegistered)
	if ev.Code != resCode || ev.Name != resName || ev.At != at {
		t.Fatalf("ResourceRegistered = %+v", ev)
	}
}

func TestRegisterIdempotentWhenAlreadyPresent(t *testing.T) {
	r := newRegisteredResource(t)
	before := r.Snapshot()

	acc, rej := r.Register(domain.RegisterResource{Code: resCode, Name: "Another name", At: at + 1000})

	tb.RequireAccepted(t, rej)
	if len(acc.Events()) != 0 {
		t.Fatalf("Events() len = %d, want 0 (idempotent)", len(acc.Events()))
	}
	if !r.Snapshot().Equal(before) {
		t.Fatalf("idempotent Register must not change state\nbefore: %+v\nafter:  %+v", before, r.Snapshot())
	}
}

func TestRegisterRejectsEmptyCode(t *testing.T) {
	r := domain.NewResource("")

	acc, rej := r.Register(domain.RegisterResource{Code: "", Name: resName, At: at})

	tb.RequireRejected(t, acc, rej, domain.CodeResourceCodeEmpty)
}

func TestRegisterKeepsTheNameItWasRegisteredWith(t *testing.T) {
	r := newRegisteredResource(t)

	_, rej := r.Register(domain.RegisterResource{Code: resCode, Name: "Another name", At: at + 1000})

	tb.RequireAccepted(t, rej)
	if got := r.Snapshot().Name; got != resName {
		t.Fatalf("Name = %q, want %q: a repeated Register accepts without changing the resource", got, resName)
	}
}

func TestRegisterRejectsEmptyName(t *testing.T) {
	r := domain.NewResource(resCode)

	acc, rej := r.Register(domain.RegisterResource{Code: resCode, At: at})

	tb.RequireRejected(t, acc, rej, domain.CodeResourceNameEmpty)
}

func TestTheDuplicateNameRejectionCarriesItsCodeWithoutTheValue(t *testing.T) {
	if got := domain.RejectResourceDuplicateName.Code(); got != domain.CodeResourceDuplicateName {
		t.Fatalf("Code() = %q, want %q", got, domain.CodeResourceDuplicateName)
	}
}
