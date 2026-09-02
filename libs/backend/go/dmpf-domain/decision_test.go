package dmpfdomain_test

import (
	"testing"

	dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"
)

type stubEvent struct{ id int }

func (e stubEvent) EventName() string { return "stub.happened" }

type stubResponse struct{ value int }

func sameEvents(t *testing.T, got []dmpfdomain.DomainEvent, want ...dmpfdomain.DomainEvent) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Events() has %d elements, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Events()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestAcceptExposesResponseAndOrderedEvents(t *testing.T) {
	e1, e2 := stubEvent{1}, stubEvent{2}
	acc := dmpfdomain.Accept(stubResponse{42}, e1, e2)

	if acc.Response() != (stubResponse{42}) {
		t.Fatalf("Response() = %v", acc.Response())
	}
	sameEvents(t, acc.Events(), e1, e2)
}

func TestAcceptInputSliceIsNotAliased(t *testing.T) {
	e1, e2 := stubEvent{1}, stubEvent{2}
	src := []dmpfdomain.DomainEvent{e1, e2}
	acc := dmpfdomain.Accept(stubResponse{1}, src...)

	src[0] = stubEvent{99}

	sameEvents(t, acc.Events(), e1, e2)
}

func TestEventsReturnedSliceIsNotAliased(t *testing.T) {
	e1, e2, e3 := stubEvent{1}, stubEvent{2}, stubEvent{3}
	acc := dmpfdomain.Accept(stubResponse{1}, e1, e2)

	first := acc.Events()
	first[0], first[1] = first[1], first[0]
	first = append(first, e3)
	_ = first

	sameEvents(t, acc.Events(), e1, e2)
}

func TestEventsWithoutEventsIsEmptyNotNil(t *testing.T) {
	acc := dmpfdomain.Accept(stubResponse{1})

	got := acc.Events()
	if got == nil {
		t.Fatal("Events() = nil, want empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("Events() has %d elements, want 0", len(got))
	}
}

func TestZeroAcceptedHasNoEvents(t *testing.T) {
	var acc dmpfdomain.Accepted[stubResponse]

	got := acc.Events()
	if got == nil || len(got) != 0 {
		t.Fatalf("zero Accepted: Events() = %v, want empty non-nil slice", got)
	}
	if acc.Response() != (stubResponse{}) {
		t.Fatalf("zero Accepted: Response() = %v", acc.Response())
	}
}

func TestAcceptEmptyResponse(t *testing.T) {
	acc := dmpfdomain.Accept(dmpfdomain.Empty{})

	if acc.Response() != (dmpfdomain.Empty{}) {
		t.Fatalf("Response() = %v, want Empty{}", acc.Response())
	}
}
