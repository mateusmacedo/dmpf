package ports_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const traceparent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

func TestMessageContextRoundTripsThroughTheContext(t *testing.T) {
	want := ports.MessageContext{CorrelationID: "corr-1", CausationID: "caus-1", Traceparent: traceparent}

	ctx := ports.WithMessageContext(context.Background(), want)

	got, ok := ports.MessageContextFrom(ctx)
	if !ok {
		t.Fatal("MessageContextFrom() ok = false, want true after WithMessageContext")
	}
	if got != want {
		t.Fatalf("MessageContextFrom() = %+v, want %+v", got, want)
	}
}

func TestMessageContextFromReportsAbsence(t *testing.T) {
	got, ok := ports.MessageContextFrom(context.Background())
	if ok {
		t.Fatal("MessageContextFrom() ok = true, want false on a context that carries nothing")
	}
	if !got.IsZero() {
		t.Fatalf("MessageContextFrom() = %+v, want the zero value when absent", got)
	}
}

func TestWithMessageContextReplacesThePreviousOne(t *testing.T) {
	first := ports.MessageContext{CorrelationID: "corr-1", CausationID: "caus-1", Traceparent: traceparent}
	second := ports.MessageContext{CorrelationID: "corr-2", CausationID: "caus-2", Traceparent: traceparent}

	ctx := ports.WithMessageContext(ports.WithMessageContext(context.Background(), first), second)

	if got, _ := ports.MessageContextFrom(ctx); got != second {
		t.Fatalf("MessageContextFrom() = %+v, want the innermost %+v", got, second)
	}
}

func TestMessageContextIsZero(t *testing.T) {
	tests := []struct {
		name string
		in   ports.MessageContext
		want bool
	}{
		{name: "zero value", in: ports.MessageContext{}, want: true},
		{name: "only correlation", in: ports.MessageContext{CorrelationID: "corr-1"}, want: false},
		{name: "only causation", in: ports.MessageContext{CausationID: "caus-1"}, want: false},
		{name: "only traceparent", in: ports.MessageContext{Traceparent: traceparent}, want: false},
		{name: "complete", in: ports.MessageContext{CorrelationID: "corr-1", CausationID: "caus-1", Traceparent: traceparent}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.in.IsZero(); got != tt.want {
				t.Fatalf("IsZero() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOutboxEntryCarriesTheMessageContext(t *testing.T) {
	mc := ports.MessageContext{CorrelationID: "corr-1", CausationID: "caus-1", Traceparent: traceparent}

	entry := ports.OutboxEntry{MessageID: "m-000001", Context: mc}

	if entry.Context != mc {
		t.Fatalf("OutboxEntry.Context = %+v, want %+v", entry.Context, mc)
	}
	if !(ports.OutboxEntry{}).Context.IsZero() {
		t.Fatal("a zero OutboxEntry must carry a zero MessageContext")
	}
}
