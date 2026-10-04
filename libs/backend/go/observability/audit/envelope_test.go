package audit_test

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type recordingExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (e *recordingExporter) Shutdown(context.Context) error   { return nil }
func (e *recordingExporter) ForceFlush(context.Context) error { return nil }

func (e *recordingExporter) Records() []sdklog.Record {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]sdklog.Record(nil), e.records...)
}

func platformProvider(t *testing.T, exporter sdklog.Exporter) *sdklog.LoggerProvider {
	t.Helper()
	provider := otelboot.NewLoggerProvider(otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource:   otelboot.Resource{ServiceName: "orders", ServiceVersion: "1.2.3", ServiceInstanceID: "pod-1"},
	}, exporter)
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	return provider
}

func emitOne(t *testing.T, ctx context.Context, event audit.Event) sdklog.Record {
	t.Helper()
	exporter := &recordingExporter{}
	provider := platformProvider(t, exporter)
	if err := audit.NewLogSink(provider).Emit(ctx, event); err != nil {
		t.Fatalf("Emit() = %v, want nil", err)
	}
	if err := provider.ForceFlush(ctx); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
	records := exporter.Records()
	if len(records) != 1 {
		t.Fatalf("exported %d records, want 1", len(records))
	}
	return records[0]
}

func attributesOf(record sdklog.Record) map[string]string {
	attributes := map[string]string{}
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		attributes[string(kv.Key)] = kv.Value.String()
		return true
	})
	return attributes
}

func scopedExecution(t *testing.T) ports.ExecutionContext {
	t.Helper()
	tenant := ports.TenantID("acme")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-1", CorrelationID: "c-1", TraceContext: "t-1", Tenant: &tenant,
		Deadline: ports.Instant(1_755_432_000_000_000_000), Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	return execution
}

func TestTheRecordIsTheAuditEventOfTheLogsAPI(t *testing.T) {
	record := emitOne(t, context.Background(), audit.Event{
		Subject: "user-1", Object: "order-1", Action: "orders.PlaceOrder", Outcome: "accepted", At: ports.Instant(1_700_000_000_000_000_000),
	})

	if got := record.EventName(); got != "dmpf.audit" {
		t.Fatalf("EventName = %q, want %q: the trail is told apart from the logs by it (RF-A7)", got, "dmpf.audit")
	}
	if got, want := record.InstrumentationScope().Name, reflect.TypeFor[audit.Event]().PkgPath(); got != want || audit.Scope != want {
		t.Fatalf("scope = %q (audit.Scope = %q), want the import path %q: panels exclude the trail by scope_name", got, audit.Scope, want)
	}
	if got := record.Timestamp(); !got.Equal(time.Unix(0, 1_700_000_000_000_000_000)) {
		t.Fatalf("Timestamp = %v, want the instant of the event", got)
	}
	if got := record.Severity(); got != log.SeverityInfo {
		t.Fatalf("Severity = %v, want %v", got, log.SeverityInfo)
	}
	want := map[string]string{
		"dmpf.audit.subject": "user-1", "dmpf.audit.object": "order-1",
		"dmpf.audit.action": "orders.PlaceOrder", "dmpf.audit.outcome": "accepted",
	}
	attributes := attributesOf(record)
	for key, value := range want {
		if attributes[key] != value {
			t.Fatalf("%s = %q, want %q in %v", key, attributes[key], value, attributes)
		}
	}
}

func TestTheRecordCarriesNoEnvelopeOfItsOwn(t *testing.T) {
	exporter := &recordingExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	event := audit.Event{Subject: "user-1", Object: "order-1", Action: "orders.PlaceOrder", Outcome: "accepted", Tenant: "acme", DataTenant: "globex"}
	if err := audit.NewLogSink(provider).Emit(context.Background(), event); err != nil {
		t.Fatalf("Emit() = %v, want nil", err)
	}
	records := exporter.Records()
	if len(records) != 1 {
		t.Fatalf("exported %d records, want 1", len(records))
	}

	for key := range attributesOf(records[0]) {
		if !strings.HasPrefix(key, "dmpf.") {
			t.Fatalf("attribute %q in the record, want only dmpf.* keys: kind, service, trace_id and time belong to the data model", key)
		}
	}
}

func TestTheRecordCarriesTheCorrelationOfTheBaggage(t *testing.T) {
	ctx := tracing.WithExecutionBaggage(context.Background(), scopedExecution(t))

	attributes := attributesOf(emitOne(t, ctx, audit.Event{Action: "orders.PlaceOrder", Outcome: "accepted"}))

	want := map[string]string{tracing.KeyCorrelationID: "c-1", tracing.KeyRequestID: "r-1", tracing.KeyTenantID: "acme"}
	for key, value := range want {
		if attributes[key] != value {
			t.Fatalf("%s = %q, want %q: the trail joins the logs beside it through the baggage", key, attributes[key], value)
		}
	}
}

func TestTheRecordOfASecurityEventCarriesBothTenants(t *testing.T) {
	attributes := attributesOf(emitOne(t, context.Background(), audit.Event{
		Subject: "user-1", Object: "orders/o-1", Action: "security.cross_tenant_access", Outcome: "denied",
		Tenant: "globex", DataTenant: "acme",
	}))

	if got := attributes[tracing.KeyTenantID]; got != "globex" {
		t.Fatalf("%s = %q, want the tenant of the context, %q", tracing.KeyTenantID, got, "globex")
	}
	if got := attributes["dmpf.audit.data_tenant_id"]; got != "acme" {
		t.Fatalf("dmpf.audit.data_tenant_id = %q, want the tenant of the data reached, %q (IDN-12)", got, "acme")
	}
}

func TestTheRecordOfAnOrdinaryEventOmitsWhatIsAbsent(t *testing.T) {
	attributes := attributesOf(emitOne(t, context.Background(), audit.Event{Action: "orders.PlaceOrder", Outcome: "accepted"}))

	for _, key := range []string{"dmpf.audit.subject", "dmpf.audit.object", "dmpf.audit.data_tenant_id", tracing.KeyTenantID} {
		if value, present := attributes[key]; present {
			t.Fatalf("%s = %q, want it absent: no value is invented for it (IDN-20)", key, value)
		}
	}
}

func TestTheRecordFollowsTheSpanOfTheCallWithoutWritingToIt(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)).Tracer("test")
	ctx, span := tracer.Start(context.Background(), "orders.PlaceOrder")

	record := emitOne(t, ctx, audit.Event{Subject: "user-1", Object: "order-1", Action: "orders.PlaceOrder", Outcome: "accepted"})
	span.End()

	if got, want := record.TraceID(), span.SpanContext().TraceID(); got != want {
		t.Fatalf("TraceId = %v, want the trace of the call %v", got, want)
	}
	ended := spans.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended %d spans, want 1", len(ended))
	}
	for _, kv := range ended[0].Attributes() {
		if strings.HasPrefix(string(kv.Key), "dmpf.audit.") {
			t.Fatalf("span carries %s, want no audit attribute on a span (DAT-25)", kv.Key)
		}
	}
	if events := ended[0].Events(); len(events) != 0 {
		t.Fatalf("span carries %d events, want none from the audit trail (DAT-25)", len(events))
	}
}

func TestConcurrentEmitsKeepEveryRecord(t *testing.T) {
	exporter := &recordingExporter{}
	provider := platformProvider(t, exporter)
	sink := audit.NewLogSink(provider)
	var wg sync.WaitGroup

	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sink.Emit(context.Background(), audit.Event{Action: "orders.PlaceOrder", Outcome: "accepted"})
		}()
	}
	wg.Wait()
	if err := provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}

	if got := len(exporter.Records()); got != 50 {
		t.Fatalf("exported %d records, want 50: an audit record is never sampled", got)
	}
}
