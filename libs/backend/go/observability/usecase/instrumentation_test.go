package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const readOperation = operationFind

// booted is everything a test needs to look at what the instrumentation
// produced: the spans that left the process, the metrics that were collected
// and the audit records that were emitted.
type booted struct {
	instrumentation *usecase.Instrumentation
	runtime         *otelboot.Runtime
	reader          *sdkmetric.ManualReader
	exporter        *tracetest.InMemoryExporter
	recording       *audit.Recording
	log             *recordingLogs
	loggerProvider  *sdklog.LoggerProvider
}

type options struct {
	subject    usecase.SubjectFunc
	classifier usecase.Classifier
	sampling   tracing.Rates
	logs       sdklog.Exporter
}

// boot starts a runtime in memory. Sampling is 1.0 for every class unless a
// test says otherwise, because the single processor only exports a span that is
// sampled or failed, and most tests here are about what the span carries rather
// than about whether it was drawn.
func boot(t *testing.T, opts options) booted {
	t.Helper()

	sampling := opts.sampling
	if sampling == nil {
		sampling = tracing.Rates{
			tracing.ClassWrite: 1.0, tracing.ClassRead: 1.0,
			tracing.ClassError: 1.0, tracing.ClassMaintenance: 1.0,
			tracing.ClassUnclassified: 1.0,
		}
	}

	reader := sdkmetric.NewManualReader()
	exporter := tracetest.NewInMemoryExporter()

	config := otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{
			ServiceName:       "orders",
			ServiceVersion:    "1.4.2",
			ServiceInstanceID: "orders-7c9f",
		},
		Sampling:      sampling,
		TraceExporter: exporter,
		MetricReader:  reader,
	}
	written := &recordingLogs{}
	var logs sdklog.Exporter = written
	if opts.logs != nil {
		logs = opts.logs
	}
	config.LoggerProvider = otelboot.NewLoggerProvider(config, logs)

	runtime, err := otelboot.Start(context.Background(), config)
	if err != nil {
		t.Fatalf("Start() = %v", err)
	}
	t.Cleanup(func() { _ = runtime.Shutdown(context.Background()) })

	recording := &audit.Recording{}
	return booted{
		instrumentation: usecase.New(runtime, recording, opts.subject, opts.classifier, readOperation),
		runtime:         runtime,
		reader:          reader,
		exporter:        exporter,
		recording:       recording,
		log:             written,
		loggerProvider:  config.LoggerProvider,
	}
}

func (b booted) spans(t *testing.T) tracetest.SpanStubs {
	t.Helper()

	if err := b.runtime.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
	return b.exporter.GetSpans()
}

func (b booted) onlySpan(t *testing.T) tracetest.SpanStub {
	t.Helper()

	exported := b.spans(t)
	if len(exported) != 1 {
		t.Fatalf("exported spans = %d, want exactly 1", len(exported))
	}
	return exported[0]
}

func attributeOf(span tracetest.SpanStub, key string) (string, bool) {
	for _, kv := range span.Attributes {
		if string(kv.Key) == key {
			return kv.Value.AsString(), true
		}
	}
	return "", false
}

func TestBeginOperationOpensTheUseCaseSpanAsWriteTraffic(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
	end(ports.Result{Outcome: ports.OutcomeAccepted})

	span := fixture.onlySpan(t)
	if span.Name != "dmpf.usecase.orders.AddItem" {
		t.Fatalf("span name = %q, want %q", span.Name, "dmpf.usecase.orders.AddItem")
	}
	class, ok := attributeOf(span, "dmpf.traffic_class")
	if !ok || class != "write" {
		t.Fatalf("dmpf.traffic_class = %q (present=%v), want \"write\"", class, ok)
	}
}

func TestADeclaredReadOperationCarriesTheReadTrafficClass(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), readOperation)
	end(ports.Result{Outcome: ports.OutcomeAccepted})

	class, ok := attributeOf(fixture.onlySpan(t), "dmpf.traffic_class")
	if !ok || class != "read" {
		t.Fatalf("dmpf.traffic_class = %q (present=%v), want \"read\" for a declared read", class, ok)
	}
}

// The sampler reads dmpf.traffic_class from the attributes given at span
// creation and never sees one set afterwards. Rating reads at 1.0 and writes at
// 0 turns that into something observable: only the read comes out sampled.
func TestTheTrafficClassIsSetAtStartSoTheSamplerSeesIt(t *testing.T) {
	fixture := boot(t, options{sampling: tracing.Rates{tracing.ClassRead: 1.0, tracing.ClassWrite: 0}})

	for _, operation := range []string{"orders.AddItem", readOperation} {
		_, end := fixture.instrumentation.BeginOperation(context.Background(), operation)
		end(ports.Result{Outcome: ports.OutcomeAccepted})
	}

	exported := fixture.spans(t)
	if len(exported) != 1 {
		t.Fatalf("exported spans = %d, want only the read: the write was rated at 0", len(exported))
	}
	if exported[0].Name != "dmpf.usecase."+readOperation {
		t.Errorf("exported %q, want the read operation", exported[0].Name)
	}
}

func TestTheUseCaseSpanInheritsTheTenantOfTheExecutionByTheBaggage(t *testing.T) {
	fixture := boot(t, options{})
	execution := testExecution(t)
	ctx := tracing.WithExecutionBaggage(ports.WithExecutionContext(context.Background(), execution), execution)

	_, end := fixture.instrumentation.BeginOperation(ctx, "orders.AddItem")
	end(ports.Result{Outcome: ports.OutcomeAccepted})

	span := fixture.onlySpan(t)
	if got, ok := attributeOf(span, tracing.KeyTenantID); !ok || got != "acme" {
		t.Errorf("span %s = %q (present=%v), want %q inherited from the baggage (RF-B8)", tracing.KeyTenantID, got, ok, "acme")
	}
	if author, present := attributeOf(span, "tenant_id"); present {
		t.Errorf("span tenant_id = %q, want absent: the tenant arrives only by the baggage (RF-B8)", author)
	}
}

func TestEndOperationRecordsTheIdempotencyOutcomeOfACommand(t *testing.T) {
	fixture := boot(t, options{})

	ctx, end := fixture.instrumentation.BeginOperation(ports.WithIdempotencySlot(context.Background()), "orders.AddItem")
	ports.MarkIdempotency(ctx, ports.IdempotencyReplayed)
	end(ports.Result{Outcome: ports.OutcomeAccepted})

	got, ok := attributeOf(fixture.onlySpan(t), "dmpf.idempotency_outcome")
	if !ok || got != "replayed" {
		t.Fatalf("dmpf.idempotency_outcome = %q (present=%v), want %q", got, ok, "replayed")
	}
}

func TestEndOperationOmitsTheIdempotencyOutcomeWithoutAClaim(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(ports.WithIdempotencySlot(context.Background()), "orders.FindOrder")
	end(ports.Result{Outcome: ports.OutcomeAccepted})

	if got, ok := attributeOf(fixture.onlySpan(t), "dmpf.idempotency_outcome"); ok {
		t.Fatalf("dmpf.idempotency_outcome = %q on an operation that made no claim", got)
	}
}

func TestEndOperationRecordsTheOutcomeCategory(t *testing.T) {
	for _, outcome := range []ports.OutcomeCategory{
		ports.OutcomeAccepted, ports.OutcomeRejected, ports.OutcomeDenied,
	} {
		t.Run(string(outcome), func(t *testing.T) {
			fixture := boot(t, options{})

			_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
			end(ports.Result{Outcome: outcome})

			span := fixture.onlySpan(t)
			got, ok := attributeOf(span, "dmpf.outcome_category")
			if !ok || got != string(outcome) {
				t.Fatalf("dmpf.outcome_category = %q (present=%v), want %q", got, ok, outcome)
			}
			if span.Status.Code == codes.Error {
				t.Fatalf("status = Error for outcome %q, want unset — only a technical failure is an error", outcome)
			}
		})
	}
}

func TestEndOperationOnFailedSetsTheErrorStatusWithoutAMessage(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
	end(ports.Result{Outcome: ports.OutcomeFailed, Err: errors.New("storage unavailable")})

	span := fixture.onlySpan(t)
	if span.Status.Code != codes.Error {
		t.Fatalf("status = %v, want Error", span.Status.Code)
	}
	if span.Status.Description != "" {
		t.Fatalf("status description = %q, want empty: the error message never reaches the span (TRC-12)", span.Status.Description)
	}
}

func TestAuditForwardsToTheSinkWithTheResolvedSubject(t *testing.T) {
	fixture := boot(t, options{subject: func(context.Context) string { return "svc-a" }})

	fixture.instrumentation.Audit(context.Background(), ports.AuditEvent{
		Object:  "P-100",
		Action:  "orders.AddItem",
		Outcome: ports.OutcomeAccepted,
		At:      ports.Instant(1_755_432_000_000_000_000),
	})

	events := fixture.recording.Events()
	if len(events) != 1 {
		t.Fatalf("Events() has %d events, want 1", len(events))
	}
	want := audit.Event{
		Subject: "svc-a",
		Object:  "P-100",
		Action:  "orders.AddItem",
		Outcome: "accepted",
		At:      ports.Instant(1_755_432_000_000_000_000),
	}
	if events[0] != want {
		t.Fatalf("Event = %+v, want %+v", events[0], want)
	}
}

func TestAuditWithoutASubjectFuncRecordsTheSubjectAsAbsent(t *testing.T) {
	fixture := boot(t, options{})

	fixture.instrumentation.Audit(context.Background(), ports.AuditEvent{Object: "P-100", Action: "orders.AddItem"})

	if got := fixture.recording.Events()[0].Subject; got != "" {
		t.Fatalf("Subject = %q, want the empty string: an absent identity is recorded as absent, never invented", got)
	}
}

func TestInstrumentationSatisfiesThePort(t *testing.T) {
	var _ ports.Instrumentation = boot(t, options{}).instrumentation
}

func collect(t *testing.T, reader *sdkmetric.ManualReader) map[string][]metricdata.DataPoint[int64] {
	t.Helper()

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}

	counters := make(map[string][]metricdata.DataPoint[int64])
	for _, scope := range collected.ScopeMetrics {
		for _, series := range scope.Metrics {
			if sum, ok := series.Data.(metricdata.Sum[int64]); ok {
				counters[series.Name] = sum.DataPoints
			}
		}
	}
	return counters
}

func histogramOf(t *testing.T, reader *sdkmetric.ManualReader, name string) metricdata.HistogramDataPoint[float64] {
	t.Helper()

	points := durationPoints(t, reader)
	if len(points) != 1 {
		t.Fatalf("%s has %d data points, want exactly 1", name, len(points))
	}
	return points[0]
}

func durationPoints(t *testing.T, reader *sdkmetric.ManualReader) []metricdata.HistogramDataPoint[float64] {
	t.Helper()

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, series := range scope.Metrics {
			if series.Name != metrics.RequestDurationSeconds {
				continue
			}
			histogram, ok := series.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("%s is a %T, want a Histogram[float64]", series.Name, series.Data)
			}
			return histogram.DataPoints
		}
	}
	return nil
}

func labelsOf(set attribute.Set) map[string]string {
	labels := make(map[string]string)
	for _, kv := range set.ToSlice() {
		labels[string(kv.Key)] = kv.Value.AsString()
	}
	return labels
}

func assertTheServiceSeriesAreGone(t *testing.T, reader *sdkmetric.ManualReader) {
	t.Helper()

	counters := collect(t, reader)
	for _, retired := range []string{"dmpf_service_requests_total", "dmpf_service_errors_total"} {
		if points := counters[retired]; len(points) != 0 {
			t.Errorf("%s = %+v, want no series: the RED of the use case is %s (RF-D2)",
				retired, points, metrics.RequestDurationSeconds)
		}
	}
}

func TestAnAcceptedOperationRecordsItsDurationByOperationAndOutcome(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
	end(ports.Result{Outcome: ports.OutcomeAccepted})

	duration := histogramOf(t, fixture.reader, metrics.RequestDurationSeconds)
	if duration.Count != 1 {
		t.Errorf("%s count = %d, want 1 (MET-08)", metrics.RequestDurationSeconds, duration.Count)
	}
	if duration.Sum < 0 {
		t.Errorf("%s sum = %v, want a non-negative duration", metrics.RequestDurationSeconds, duration.Sum)
	}
	got := labelsOf(duration.Attributes)
	want := map[string]string{
		tracing.KeyOperation:       "orders.AddItem",
		tracing.KeyOutcomeCategory: string(ports.OutcomeAccepted),
	}
	if len(got) != len(want) || got[tracing.KeyOperation] != want[tracing.KeyOperation] ||
		got[tracing.KeyOutcomeCategory] != want[tracing.KeyOutcomeCategory] {
		t.Errorf("labels = %v, want exactly %v: no service, no error.type without a failure (RF-D2, RF-D3)", got, want)
	}
	assertTheServiceSeriesAreGone(t, fixture.reader)
}

func TestEveryOutcomeCategoryCountsAsItsOwnSeries(t *testing.T) {
	fixture := boot(t, options{})

	for _, outcome := range []ports.OutcomeCategory{
		ports.OutcomeAccepted, ports.OutcomeRejected, ports.OutcomeDenied,
	} {
		_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
		end(ports.Result{Outcome: outcome})
	}

	byOutcome := map[string]uint64{}
	for _, point := range durationPoints(t, fixture.reader) {
		byOutcome[labelsOf(point.Attributes)[tracing.KeyOutcomeCategory]] = point.Count
	}

	for _, want := range []string{"accepted", "rejected", "denied"} {
		if byOutcome[want] != 1 {
			t.Errorf("%s{dmpf.outcome_category=%q} count = %d, want 1", metrics.RequestDurationSeconds, want, byOutcome[want])
		}
	}
	if _, counted := byOutcome["failed"]; counted {
		t.Error("a failed series appeared without any failure")
	}
}

func TestOnlyAFailedOutcomeCountsAnError(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
	end(ports.Result{Outcome: ports.OutcomeRejected})

	if errorType, labelled := labelsOf(histogramOf(t, fixture.reader, metrics.RequestDurationSeconds).Attributes)[metrics.KeyErrorType]; labelled {
		t.Errorf("error.type = %q, want none: a rejection is a business outcome, not a failure (DEC-04)", errorType)
	}
	if errorType, present := attributeOf(fixture.onlySpan(t), metrics.KeyErrorType); present {
		t.Errorf("span error.type = %q, want none on a rejection (DEC-04)", errorType)
	}
}

func TestAFailureIsCountedUnderTheCategoryTheClassifierGives(t *testing.T) {
	fixture := boot(t, options{classifier: func(error) string { return "timeout" }})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
	end(ports.Result{Outcome: ports.OutcomeFailed, Err: errors.New("the payment gateway timed out")})

	duration := histogramOf(t, fixture.reader, metrics.RequestDurationSeconds)
	if duration.Count != 1 {
		t.Fatalf("%s count = %d, want 1 (MET-10)", metrics.RequestDurationSeconds, duration.Count)
	}
	if got := labelsOf(duration.Attributes); got[metrics.KeyErrorType] != "timeout" ||
		got[tracing.KeyOperation] != "orders.AddItem" ||
		got[tracing.KeyOutcomeCategory] != string(ports.OutcomeFailed) {
		t.Errorf("labels = %v, want the classified error.type with operation and outcome", got)
	} else if _, labelled := got["service"]; labelled {
		t.Errorf("labels = %v, want no service: it is a resource attribute (RF-D3)", got)
	}
	if got, ok := attributeOf(fixture.onlySpan(t), metrics.KeyErrorType); !ok || got != "timeout" {
		t.Errorf("span error.type = %q (present=%v), want the category of the Classifier (RF-B1)", got, ok)
	}
	assertTheServiceSeriesAreGone(t, fixture.reader)
}

func TestAFailureWithoutAClassifierIsUnclassifiedAndNeverCarriesTheMessage(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		classifier usecase.Classifier
	}{
		{"no classifier", nil},
		{"classifier says nothing", func(error) string { return "" }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := boot(t, options{classifier: testCase.classifier})

			_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
			end(ports.Result{
				Outcome: ports.OutcomeFailed,
				Err:     errors.New("cpf=123.456.789-00 was refused"),
			})

			other := semconv.ErrorTypeOther.Value.AsString()
			labels := labelsOf(histogramOf(t, fixture.reader, metrics.RequestDurationSeconds).Attributes)
			if labels[metrics.KeyErrorType] != other {
				t.Errorf("error.type = %q, want %q (RF-B1)", labels[metrics.KeyErrorType], other)
			}
			for key, value := range labels {
				if strings.Contains(value, "123.456") {
					t.Errorf("label %s = %q carries the error message (MET-07)", key, value)
				}
			}

			span := fixture.onlySpan(t)
			if got, ok := attributeOf(span, metrics.KeyErrorType); !ok || got != other {
				t.Errorf("span error.type = %q (present=%v), want %q (RF-B1)", got, ok, other)
			}
			for _, kv := range span.Attributes {
				if strings.Contains(kv.Value.String(), "123.456") {
					t.Errorf("span attribute %s carries the error message (TRC-15)", kv.Key)
				}
			}
		})
	}
}

func TestAFailureAlsoCountsAsARequest(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
	end(ports.Result{Outcome: ports.OutcomeFailed, Err: errors.New("boom")})

	duration := histogramOf(t, fixture.reader, metrics.RequestDurationSeconds)
	if duration.Count != 1 {
		t.Errorf("%s count = %d, want the failure counted as a request too", metrics.RequestDurationSeconds, duration.Count)
	}
	if _, labelled := labelsOf(duration.Attributes)[metrics.KeyErrorType]; !labelled {
		t.Errorf("labels = %v, want error.type: the errors come from the count by error.type (RF-D2)",
			labelsOf(duration.Attributes))
	}
}

func TestAReadOperationIsCountedLikeAnyOther(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), readOperation)
	end(ports.Result{Outcome: ports.OutcomeAccepted})

	duration := histogramOf(t, fixture.reader, metrics.RequestDurationSeconds)
	if duration.Count != 1 || labelsOf(duration.Attributes)[tracing.KeyOperation] != readOperation {
		t.Errorf("%s = %+v, want the read operation counted like any other", metrics.RequestDurationSeconds, duration)
	}
}

// The injected SubjectFunc says someone else on purpose: the subject of the
// security record comes from the carrier, the one source CTX-03 allows.
func TestACrossTenantAccessIsRecordedAsASecurityEvent(t *testing.T) {
	fixture := boot(t, options{subject: func(context.Context) string { return "not-the-carrier" }})
	access := ports.CrossTenantAccess{Object: "probes/o-1", ContextTenant: "globex", DataTenant: "acme"}

	_, end := fixture.instrumentation.BeginOperation(withExecution(t, context.Background()), operationFind)
	end(ports.Result{Outcome: ports.OutcomeFailed, Err: fmt.Errorf("application: find order o-1: %w", access)})

	events := fixture.recording.Events()
	if len(events) != 1 {
		t.Fatalf("Events() has %d events, want 1: IDN-12 requires the attempt to be recorded", len(events))
	}
	got := events[0]
	if got.At == 0 {
		t.Fatal("At = 0, want the instant of the attempt")
	}
	got.At = 0
	want := audit.Event{
		Subject:    "s-test",
		Object:     "probes/o-1",
		Action:     usecase.ActionCrossTenantAccess,
		Outcome:    string(ports.OutcomeDenied),
		Tenant:     "globex",
		DataTenant: "acme",
	}
	if got != want {
		t.Fatalf("Event = %+v, want %+v", got, want)
	}
}

func TestAPlainNotFoundRecordsNoSecurityEvent(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(withExecution(t, context.Background()), operationFind)
	end(ports.Result{Outcome: ports.OutcomeFailed, Err: fmt.Errorf("application: find order o-1: %w", ports.ErrNotFound)})

	if events := fixture.recording.Events(); len(events) != 0 {
		t.Fatalf("Events() = %+v, want none: an absent identifier is not an access (IDN-13)", events)
	}
}

// IDN-12 requires the attempt to be recorded. When the audit sink refuses it,
// the event goes to the log channel whole instead of vanishing behind a
// category, because the log leaves the process by another path.
func TestASecurityEventTheSinkRefusesReachesTheLogWhole(t *testing.T) {
	logs := &recordingLogs{}
	fixture := boot(t, options{logs: logs})
	instrumentation := usecase.New(fixture.runtime, refusingSink{err: errors.New("sink closed")}, nil, nil, readOperation)
	access := ports.CrossTenantAccess{Object: "probes/o-1", ContextTenant: "acme", DataTenant: "globex"}
	execution := testExecution(t)
	ctx := tracing.WithExecutionBaggage(ports.WithExecutionContext(context.Background(), execution), execution)

	_, end := instrumentation.BeginOperation(ctx, operationFind)
	end(ports.Result{Outcome: ports.OutcomeFailed, Err: access})

	record := logs.securityRecord(t, fixture.loggerProvider)
	for key, want := range map[string]string{
		"dmpf.audit.action":         usecase.ActionCrossTenantAccess,
		"dmpf.audit.object":         "probes/o-1",
		"dmpf.audit.subject":        "s-test",
		"dmpf.audit.data_tenant_id": "globex",
		tracing.KeyTenantID:         "acme",
	} {
		if record[key] != want {
			t.Errorf("record %s = %q, want %q: the security record must survive the sink (IDN-12)", key, record[key], want)
		}
	}
	if author, present := record["tenant_id"]; present {
		t.Errorf("record tenant_id = %q, want absent: the tenant of the context arrives by the baggage (RF-B8)", author)
	}
}

type recordingLogs struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (r *recordingLogs) Export(_ context.Context, records []sdklog.Record) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, record := range records {
		r.records = append(r.records, record.Clone())
	}
	return nil
}

func (r *recordingLogs) Shutdown(context.Context) error   { return nil }
func (r *recordingLogs) ForceFlush(context.Context) error { return nil }

func (r *recordingLogs) securityRecord(t *testing.T, provider *sdklog.LoggerProvider) map[string]string {
	t.Helper()

	if err := provider.ForceFlush(context.Background()); err != nil {
		t.Fatalf("ForceFlush() = %v", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	var found []map[string]string
	for _, record := range r.records {
		attributes := map[string]string{}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			attributes[string(kv.Key)] = kv.Value.String()
			return true
		})
		if _, carriesTheObject := attributes["dmpf.audit.object"]; carriesTheObject {
			found = append(found, attributes)
		}
	}
	if len(found) != 1 {
		t.Fatalf("exported %d log records carrying the object of the event, want 1", len(found))
	}
	return found[0]
}
