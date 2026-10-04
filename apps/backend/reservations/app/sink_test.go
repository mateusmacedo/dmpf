package app_test

import (
	"bytes"
	"context"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventv1 "github.com/mateusmacedo/dmpf/apps/backend/orders/contract/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/app"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	orderPlacedV1    = "com.company.orders.order-placed.v1"
	envelopeTraceID  = "0af7651916cd43dd8448eb211c80319c"
	envelopeParentID = "b7ad6b7169203331"
	ordersTopic      = "orders.events"
)

type fakeHandler struct {
	calls int
	seen  trace.SpanContext
}

func (h *fakeHandler) handle(ctx context.Context, _ ports.Receipt, _ envelope.Envelope) (application.Disposition, error) {
	h.calls++
	h.seen = trace.SpanContextFromContext(ctx)
	return application.R1D1, nil
}

type fakeAck struct{ acks, releases int }

func (a *fakeAck) Ack(context.Context) error     { a.acks++; return nil }
func (a *fakeAck) Release(context.Context) error { a.releases++; return nil }

type fakeContainment struct{ contained int }

func (c *fakeContainment) Quarantine(context.Context, ports.Contained) error {
	c.contained++
	return nil
}

type fixedClock struct{}

func (fixedClock) Now() ports.Instant { return ports.Instant(1_756_000_000_000_000_000) }

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

// sinkTimeout is the consumer's own time policy, which CTX-28 makes mandatory.
// It lives here because e2e_test.go is behind the integration build tag.
const sinkTimeout = 10 * time.Second

func newSink(handler *fakeHandler, containment *fakeContainment, tracer trace.Tracer) app.Sink {
	return app.Sink{
		Consumer: kernelapp.Consumer{
			Name:        "reservations",
			MaxAttempts: 2,
			Handle:      handler.handle,
			Containment: containment,
			Clock:       fixedClock{},
			Timeout:     sinkTimeout,
			Boundary:    kernelapp.Boundary{Transport: kernelapp.TransportDevelopmentOnly, Sources: []string{"urn:dmpf:reference-orders"}},
			Locale:      "en",
			Tracer:      tracer,
			Channel:     kernelapp.Channel{Address: ordersTopic, Group: "reservations"},
		},
		EventType: orderPlacedV1,
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

func TestSinkLogsAnotherTypeAtDebug(t *testing.T) {
	var out bytes.Buffer
	sink := newSink(&fakeHandler{}, &fakeContainment{}, nil)
	sink.Logger = slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelDebug}))
	raw := rawEnvelope(t, "com.company.orders.item-added.v1", &eventv1.ItemAdded{OrderId: "o-1", Sku: "A", Quantity: 1})

	if err := sink.Handle(context.Background(), raw, 1, &fakeAck{}); err != nil {
		t.Fatalf("Handle() = %v, want nil", err)
	}

	if got := out.String(); strings.Count(got, "\n") != 1 || !strings.Contains(got, "level=DEBUG") || !strings.Contains(got, "delivery of another event type acknowledged") {
		t.Fatalf("log = %q, want the one discard record at debug (RF-A5)", got)
	}
}

func TestSinkLogsAnotherTypeUnderTheMessagingKeys(t *testing.T) {
	logs := &recordingExporter{}
	provider := otelboot.NewLoggerProvider(otelboot.Config{Resource: otelboot.Resource{
		ServiceName: "reservations", ServiceVersion: "test", ServiceInstanceID: "reservations-consumer-1", Role: "consumer",
	}}, logs)
	sink := newSink(&fakeHandler{}, &fakeContainment{}, nil)
	sink.Logger = logging.NewLogger(provider, reflect.TypeFor[app.Sink]().PkgPath())
	raw := rawEnvelope(t, "com.company.orders.item-added.v1", &eventv1.ItemAdded{OrderId: "o-1", Sku: "A", Quantity: 1})

	if err := sink.Handle(context.Background(), raw, 1, &fakeAck{}); err != nil {
		t.Fatalf("Handle() = %v, want nil", err)
	}
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown() = %v", err)
	}

	attributes, found := recordAttributes(logs, "delivery of another event type acknowledged")
	if !found {
		t.Fatal("no discard record exported")
	}
	requireTheVocabulary(t, "delivery of another event type acknowledged", attributes, map[string]string{
		string(semconv.CloudEventsEventTypeKey): "com.company.orders.item-added.v1",
		string(semconv.MessagingMessageIDKey):   "evt-1",
	})
}

func TestSinkLeavesTheProcessSpanToTheKernel(t *testing.T) {
	spans := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spans))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	handler, containment, ack := &fakeHandler{}, &fakeContainment{}, &fakeAck{}
	raw := rawEnvelope(t, orderPlacedV1, &eventv1.OrderPlaced{OrderId: "o-1", ItemCount: 2})

	if err := newSink(handler, containment, provider.Tracer("sink-test")).Handle(context.Background(), raw, 1, ack); err != nil {
		t.Fatalf("Handle() = %v, want nil", err)
	}

	ended := spans.GetSpans()
	if len(ended) != 1 || ended[0].Name != "process "+ordersTopic || ended[0].SpanKind != trace.SpanKindConsumer {
		names := make([]string, 0, len(ended))
		for _, span := range ended {
			names = append(names, span.Name)
		}
		t.Fatalf("spans = %v, want only the kernel's CONSUMER process %q (RF-B9)", names, "process "+ordersTopic)
	}
	if handler.seen.SpanID() != ended[0].SpanContext.SpanID() {
		t.Fatalf("handler ran under span %s, want the process span %s", handler.seen.SpanID(), ended[0].SpanContext.SpanID())
	}
}
