package app

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestSubjectIsReadFromTheCarrier(t *testing.T) {
	who := ports.SubjectID("s-1")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID:     "r-1",
		CorrelationID: "c-1",
		TraceContext:  "t-1",
		Subject:       &who,
		Permissions:   []ports.Permission{},
		Deadline:      ports.Instant(1_755_432_000_000_000_000),
		Locale:        "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}

	if got := subject(ports.WithExecutionContext(context.Background(), execution)); got != "s-1" {
		t.Fatalf("subject() = %q, want %q: the audit trail names who acted (LOG-14)", got, "s-1")
	}
}

func TestSubjectStaysAbsentWithoutOne(t *testing.T) {
	if got := subject(context.Background()); got != "" {
		t.Fatalf("subject() = %q, want absent: no identity is invented for it (IDN-20)", got)
	}
}
