package usecase_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/audit"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/logging"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/otelboot"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/tracing"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/usecase"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

const readOperation = "orders.FindOrder"

// booted is everything a test needs to look at what the instrumentation
// produced: the spans that left the process, the metrics that were collected
// and the audit records that were emitted.
type booted struct {
	instrumentation *usecase.Instrumentation
	runtime         *otelboot.Runtime
	reader          *sdkmetric.ManualReader
	exporter        *tracetest.InMemoryExporter
	recording       *audit.Recording
	log             *bytes.Buffer
}

type options struct {
	subject    usecase.SubjectFunc
	classifier usecase.Classifier
	sampling   tracing.Rates
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

	// The platform handler, not a bare JSON one: the tests that check what does
	// not leak have to read the records the service would really write.
	var written bytes.Buffer
	logger := slog.New(logging.NewHandler(&written, logging.Config{
		Service:  "orders",
		Version:  "1.4.2",
		Instance: "orders-7c9f",
		Class:    tracing.ClassWrite,
		Sampling: tracing.Rates{tracing.ClassWrite: 1.0},
	}))

	runtime, err := otelboot.Start(context.Background(), otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource: otelboot.Resource{
			ServiceName:       "orders",
			ServiceVersion:    "1.4.2",
			ServiceInstanceID: "orders-7c9f",
		},
		Sampling:      sampling,
		TraceExporter: exporter,
		MetricReader:  reader,
		Logger:        logger,
	})
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
		log:             &written,
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
	end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})

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
	end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})

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
		end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})
	}

	exported := fixture.spans(t)
	if len(exported) != 1 {
		t.Fatalf("exported spans = %d, want only the read: the write was rated at 0", len(exported))
	}
	if exported[0].Name != "dmpf.usecase."+readOperation {
		t.Errorf("exported %q, want the read operation", exported[0].Name)
	}
}

func TestEndOperationRecordsTheOutcomeCategory(t *testing.T) {
	for _, outcome := range []dmpfports.OutcomeCategory{
		dmpfports.OutcomeAccepted, dmpfports.OutcomeRejected, dmpfports.OutcomeDenied,
	} {
		t.Run(string(outcome), func(t *testing.T) {
			fixture := boot(t, options{})

			_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
			end(dmpfports.Result{Outcome: outcome})

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
	end(dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: errors.New("storage unavailable")})

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

	fixture.instrumentation.Audit(context.Background(), dmpfports.AuditEvent{
		Object:  "P-100",
		Action:  "orders.AddItem",
		Outcome: dmpfports.OutcomeAccepted,
		At:      dmpfports.Instant(1_755_432_000_000_000_000),
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
		At:      dmpfports.Instant(1_755_432_000_000_000_000),
	}
	if events[0] != want {
		t.Fatalf("Event = %+v, want %+v", events[0], want)
	}
}

func TestAuditWithoutASubjectFuncRecordsTheSubjectAsAbsent(t *testing.T) {
	fixture := boot(t, options{})

	fixture.instrumentation.Audit(context.Background(), dmpfports.AuditEvent{Object: "P-100", Action: "orders.AddItem"})

	if got := fixture.recording.Events()[0].Subject; got != "" {
		t.Fatalf("Subject = %q, want the empty string: an absent identity is recorded as absent, never invented", got)
	}
}

func TestInstrumentationSatisfiesThePort(t *testing.T) {
	var _ dmpfports.Instrumentation = boot(t, options{}).instrumentation
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

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	for _, scope := range collected.ScopeMetrics {
		for _, series := range scope.Metrics {
			if series.Name != name {
				continue
			}
			histogram, ok := series.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("%s is a %T, want a Histogram[float64]", name, series.Data)
			}
			if len(histogram.DataPoints) != 1 {
				t.Fatalf("%s has %d data points, want exactly 1", name, len(histogram.DataPoints))
			}
			return histogram.DataPoints[0]
		}
	}
	t.Fatalf("%s was never recorded", name)
	return metricdata.HistogramDataPoint[float64]{}
}

func labelsOf(point metricdata.DataPoint[int64]) map[string]string {
	labels := make(map[string]string)
	for _, kv := range point.Attributes.ToSlice() {
		labels[string(kv.Key)] = kv.Value.AsString()
	}
	return labels
}

func TestAnAcceptedOperationCountsARequestAndItsDuration(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
	end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})

	requests := collect(t, fixture.reader)[metrics.RequestsTotal]
	if len(requests) != 1 || requests[0].Value != 1 {
		t.Fatalf("%s = %+v, want a single point of 1 (MET-09)", metrics.RequestsTotal, requests)
	}
	if got := labelsOf(requests[0]); got[metrics.KeyService] != "orders" ||
		got[metrics.KeyOperation] != "orders.AddItem" ||
		got[metrics.KeyOutcomeCategory] != string(dmpfports.OutcomeAccepted) {
		t.Errorf("labels = %v, want service, operation and outcome_category (MET-04)", got)
	}

	duration := histogramOf(t, fixture.reader, metrics.RequestDurationSeconds)
	if duration.Count != 1 {
		t.Errorf("%s count = %d, want 1 (MET-08)", metrics.RequestDurationSeconds, duration.Count)
	}
	if duration.Sum < 0 {
		t.Errorf("%s sum = %v, want a non-negative duration", metrics.RequestDurationSeconds, duration.Sum)
	}
}

func TestEveryOutcomeCategoryCountsAsItsOwnSeries(t *testing.T) {
	fixture := boot(t, options{})

	for _, outcome := range []dmpfports.OutcomeCategory{
		dmpfports.OutcomeAccepted, dmpfports.OutcomeRejected, dmpfports.OutcomeDenied,
	} {
		_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
		end(dmpfports.Result{Outcome: outcome})
	}

	byOutcome := map[string]int64{}
	for _, point := range collect(t, fixture.reader)[metrics.RequestsTotal] {
		byOutcome[labelsOf(point)[metrics.KeyOutcomeCategory]] = point.Value
	}

	for _, want := range []string{"accepted", "rejected", "denied"} {
		if byOutcome[want] != 1 {
			t.Errorf("%s{outcome_category=%q} = %d, want 1", metrics.RequestsTotal, want, byOutcome[want])
		}
	}
	if _, counted := byOutcome["failed"]; counted {
		t.Error("a failed series appeared without any failure")
	}
}

func TestOnlyAFailedOutcomeCountsAnError(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
	end(dmpfports.Result{Outcome: dmpfports.OutcomeRejected})

	if points := collect(t, fixture.reader)[metrics.ErrorsTotal]; len(points) != 0 {
		t.Errorf("%s = %+v, want none: a rejection is a business outcome, not a failure (DEC-04)",
			metrics.ErrorsTotal, points)
	}
}

func TestAFailureIsCountedUnderTheCategoryTheClassifierGives(t *testing.T) {
	fixture := boot(t, options{classifier: func(error) string { return "timeout" }})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
	end(dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: errors.New("the payment gateway timed out")})

	points := collect(t, fixture.reader)[metrics.ErrorsTotal]
	if len(points) != 1 || points[0].Value != 1 {
		t.Fatalf("%s = %+v, want a single point of 1 (MET-10)", metrics.ErrorsTotal, points)
	}
	if got := labelsOf(points[0]); got[metrics.KeyErrorCategory] != "timeout" ||
		got[metrics.KeyOperation] != "orders.AddItem" || got[metrics.KeyService] != "orders" {
		t.Errorf("labels = %v, want the classified category with service and operation", got)
	}
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
			end(dmpfports.Result{
				Outcome: dmpfports.OutcomeFailed,
				Err:     errors.New("cpf=123.456.789-00 was refused"),
			})

			points := collect(t, fixture.reader)[metrics.ErrorsTotal]
			if len(points) != 1 {
				t.Fatalf("%s = %+v, want a single point", metrics.ErrorsTotal, points)
			}

			labels := labelsOf(points[0])
			if labels[metrics.KeyErrorCategory] != usecase.CategoryUnclassified {
				t.Errorf("error_category = %q, want %q", labels[metrics.KeyErrorCategory], usecase.CategoryUnclassified)
			}
			for key, value := range labels {
				if strings.Contains(value, "123.456") {
					t.Errorf("label %s = %q carries the error message (MET-07)", key, value)
				}
			}
		})
	}
}

func TestAFailureAlsoCountsAsARequest(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), "orders.AddItem")
	end(dmpfports.Result{Outcome: dmpfports.OutcomeFailed, Err: errors.New("boom")})

	counters := collect(t, fixture.reader)
	if points := counters[metrics.RequestsTotal]; len(points) != 1 || points[0].Value != 1 {
		t.Errorf("%s = %+v, want the failure counted as a request too", metrics.RequestsTotal, points)
	}
	if points := counters[metrics.ErrorsTotal]; len(points) != 1 {
		t.Errorf("%s = %+v, want the failure counted as an error", metrics.ErrorsTotal, points)
	}
}

func TestAReadOperationIsCountedLikeAnyOther(t *testing.T) {
	fixture := boot(t, options{})

	_, end := fixture.instrumentation.BeginOperation(context.Background(), readOperation)
	end(dmpfports.Result{Outcome: dmpfports.OutcomeAccepted})

	points := collect(t, fixture.reader)[metrics.RequestsTotal]
	if len(points) != 1 || labelsOf(points[0])[metrics.KeyOperation] != readOperation {
		t.Errorf("%s = %+v, want the read operation counted like any other", metrics.RequestsTotal, points)
	}
}
