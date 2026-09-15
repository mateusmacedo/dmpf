package dmpfreferencereservations_test

import (
	"context"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	dmpfreferencereservations "github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference-reservations-go"
	dmpfapp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-app"
	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/envelope"
	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/gen/go/company/orders/event/v1"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

const (
	orderPlacedV1    = "com.company.orders.order-placed.v1"
	envelopeTraceID  = "0af7651916cd43dd8448eb211c80319c"
	envelopeParentID = "b7ad6b7169203331"
)

type fakeHandler struct {
	calls int
	seen  trace.SpanContext
}

func (h *fakeHandler) handle(ctx context.Context, _ dmpfports.Receipt, _ envelope.Envelope) (dmpfapplication.Disposition, error) {
	h.calls++
	h.seen = trace.SpanContextFromContext(ctx)
	return dmpfapplication.R1D1, nil
}

type fakeAck struct{ acks, releases int }

func (a *fakeAck) Ack(context.Context) error     { a.acks++; return nil }
func (a *fakeAck) Release(context.Context) error { a.releases++; return nil }

type fakeContainment struct{ contained int }

func (c *fakeContainment) Quarantine(context.Context, dmpfports.Contained) error {
	c.contained++
	return nil
}

type fixedClock struct{}

func (fixedClock) Now() dmpfports.Instant { return dmpfports.Instant(1_756_000_000_000_000_000) }

func rawEnvelope(t *testing.T, eventType string, msg proto.Message) []byte {
	t.Helper()
	payload, typeURL, err := envelope.Pack(msg)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	ce, err := envelope.Encode(envelope.Envelope{
		ID:              "evt-1",
		Source:          "urn:dmpf:reference-orders",
		SpecVersion:     envelope.SpecVersion,
		Type:            eventType,
		Subject:         "o-1",
		Time:            timestamppb.New(time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)),
		DataSchema:      typeURL,
		DataContentType: envelope.ContentType,
		CorrelationID:   "corr-1",
		CausationID:     "evt-1",
		PartitionKey:    "o-1",
		TraceParent:     "00-" + envelopeTraceID + "-" + envelopeParentID + "-01",
		Payload:         payload,
	})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(ce)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	return raw
}

func newSink(handler *fakeHandler, containment *fakeContainment, tracer trace.Tracer) dmpfreferencereservations.Sink {
	return dmpfreferencereservations.Sink{
		Consumer: dmpfapp.Consumer{
			Name:        "reservations",
			MaxAttempts: 2,
			Handle:      handler.handle,
			Containment: containment,
			Clock:       fixedClock{},
		},
		EventType: orderPlacedV1,
		Tracer:    tracer,
	}
}

func TestSinkHandsTheSubscribedTypeToTheAdapter(t *testing.T) {
	handler, containment, ack := &fakeHandler{}, &fakeContainment{}, &fakeAck{}
	raw := rawEnvelope(t, orderPlacedV1, &eventv1.OrderPlaced{OrderId: "o-1", ItemCount: 2})

	if err := newSink(handler, containment, nil).Handle(context.Background(), raw, 1, ack); err != nil {
		t.Fatalf("Handle() = %v, want nil", err)
	}

	if handler.calls != 1 || ack.acks != 1 || containment.contained != 0 {
		t.Fatalf("handler=%d acks=%d contained=%d, want the adapter to run and ack once", handler.calls, ack.acks, containment.contained)
	}
}

func TestSinkAcknowledgesAnotherTypeWithoutTheAdapter(t *testing.T) {
	handler, containment, ack := &fakeHandler{}, &fakeContainment{}, &fakeAck{}
	raw := rawEnvelope(t, "com.company.orders.item-added.v1", &eventv1.ItemAdded{OrderId: "o-1", Sku: "A", Quantity: 1})

	if err := newSink(handler, containment, nil).Handle(context.Background(), raw, 1, ack); err != nil {
		t.Fatalf("Handle() = %v, want nil", err)
	}

	if handler.calls != 0 || containment.contained != 0 || ack.acks != 1 || ack.releases != 0 {
		t.Fatalf("handler=%d contained=%d acks=%d releases=%d, want only the ack", handler.calls, containment.contained, ack.acks, ack.releases)
	}
}

func TestSinkLeavesAnInvalidEnvelopeToTheAdapter(t *testing.T) {
	handler, containment, ack := &fakeHandler{}, &fakeContainment{}, &fakeAck{}

	if err := newSink(handler, containment, nil).Handle(context.Background(), []byte("not a cloudevent"), 1, ack); err != nil {
		t.Fatalf("Handle() = %v, want nil", err)
	}

	if handler.calls != 0 || containment.contained != 1 || ack.acks != 1 {
		t.Fatalf("handler=%d contained=%d acks=%d, want the adapter to quarantine it (INB-10)", handler.calls, containment.contained, ack.acks)
	}
}

func TestSinkContinuesTheTraceOfTheEnvelope(t *testing.T) {
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	handler, containment, ack := &fakeHandler{}, &fakeContainment{}, &fakeAck{}
	raw := rawEnvelope(t, orderPlacedV1, &eventv1.OrderPlaced{OrderId: "o-1", ItemCount: 2})

	if err := newSink(handler, containment, provider.Tracer("sink-test")).Handle(context.Background(), raw, 1, ack); err != nil {
		t.Fatalf("Handle() = %v, want nil", err)
	}

	want := "dmpf.kafka.consume " + orderPlacedV1
	for _, span := range spans.GetSpans() {
		if span.Name != want {
			continue
		}
		if span.SpanKind != trace.SpanKindConsumer {
			t.Fatalf("kind = %v, want consumer", span.SpanKind)
		}
		if span.Parent.TraceID().String() != envelopeTraceID || span.Parent.SpanID().String() != envelopeParentID {
			t.Fatalf("parent = %s/%s, want the envelope's traceparent (TRC-07)", span.Parent.TraceID(), span.Parent.SpanID())
		}
		if handler.seen.SpanID() != span.SpanContext.SpanID() {
			t.Fatalf("handler ran under span %s, want the consume span %s", handler.seen.SpanID(), span.SpanContext.SpanID())
		}
		return
	}
	t.Fatalf("no span named %q among %d", want, len(spans.GetSpans()))
}
