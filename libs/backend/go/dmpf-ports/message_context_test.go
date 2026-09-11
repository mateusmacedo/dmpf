package dmpfports_test

import (
	"context"
	"testing"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

const traceparent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

func TestMessageContextRoundTripsThroughTheContext(t *testing.T) {
	want := dmpfports.MessageContext{CorrelationID: "corr-1", CausationID: "caus-1", Traceparent: traceparent}

	ctx := dmpfports.WithMessageContext(context.Background(), want)

	got, ok := dmpfports.MessageContextFrom(ctx)
	if !ok {
		t.Fatal("MessageContextFrom() ok = false, want true after WithMessageContext")
	}
	if got != want {
		t.Fatalf("MessageContextFrom() = %+v, want %+v", got, want)
	}
}

func TestMessageContextFromReportsAbsence(t *testing.T) {
	got, ok := dmpfports.MessageContextFrom(context.Background())
	if ok {
		t.Fatal("MessageContextFrom() ok = true, want false on a context that carries nothing")
	}
	if !got.IsZero() {
		t.Fatalf("MessageContextFrom() = %+v, want the zero value when absent", got)
	}
}

func TestWithMessageContextReplacesThePreviousOne(t *testing.T) {
	first := dmpfports.MessageContext{CorrelationID: "corr-1", CausationID: "caus-1", Traceparent: traceparent}
	second := dmpfports.MessageContext{CorrelationID: "corr-2", CausationID: "caus-2", Traceparent: traceparent}

	ctx := dmpfports.WithMessageContext(dmpfports.WithMessageContext(context.Background(), first), second)

	if got, _ := dmpfports.MessageContextFrom(ctx); got != second {
		t.Fatalf("MessageContextFrom() = %+v, want the innermost %+v", got, second)
	}
}

func TestMessageContextIsZero(t *testing.T) {
	tests := []struct {
		name string
		in   dmpfports.MessageContext
		want bool
	}{
		{name: "zero value", in: dmpfports.MessageContext{}, want: true},
		{name: "only correlation", in: dmpfports.MessageContext{CorrelationID: "corr-1"}, want: false},
		{name: "only causation", in: dmpfports.MessageContext{CausationID: "caus-1"}, want: false},
		{name: "only traceparent", in: dmpfports.MessageContext{Traceparent: traceparent}, want: false},
		{name: "complete", in: dmpfports.MessageContext{CorrelationID: "corr-1", CausationID: "caus-1", Traceparent: traceparent}, want: false},
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
	mc := dmpfports.MessageContext{CorrelationID: "corr-1", CausationID: "caus-1", Traceparent: traceparent}

	entry := dmpfports.OutboxEntry{MessageID: "m-000001", Context: mc}

	if entry.Context != mc {
		t.Fatalf("OutboxEntry.Context = %+v, want %+v", entry.Context, mc)
	}
	if !(dmpfports.OutboxEntry{}).Context.IsZero() {
		t.Fatal("a zero OutboxEntry must carry a zero MessageContext")
	}
}
