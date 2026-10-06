package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/usecase"
)

func TestTheServiceSeriesCountByOperationAndOutcome(t *testing.T) {
	w := wire(t, allowAll())
	ctx := budgeted(t)

	if _, err := w.service.Bump(withExecution(t, ctx), bumpCounter{Counter: subjectID, By: 1}); err != nil {
		t.Fatalf("Bump() error = %v, want nil", err)
	}
	if _, err := w.service.Find(withExecution(t, ctx), subjectID); err != nil {
		t.Fatalf("Find() error = %v, want nil", err)
	}

	series := w.requestSeries(t)
	for key, want := range map[string]int64{
		operationBump + "/accepted": 1,
		operationFind + "/accepted": 1,
	} {
		if series[key] != want {
			t.Errorf("%s{%s} count = %d, want %d (MET-09)", metrics.RequestDurationSeconds, key, series[key], want)
		}
	}

	// One histogram series per label set, so two operations give two series,
	// each with a single observation.
	durations := w.durationSeries(t)
	for _, operation := range []string{operationBump, operationFind} {
		if durations[operation] != 1 {
			t.Errorf("%s{operation=%q} count = %d, want 1 (MET-08)",
				metrics.RequestDurationSeconds, operation, durations[operation])
		}
	}
}

func (w *wiring) durationSeries(t *testing.T) map[string]uint64 {
	t.Helper()

	var collected metricdata.ResourceMetrics
	if err := w.reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}

	counts := make(map[string]uint64)
	for _, scope := range collected.ScopeMetrics {
		for _, series := range scope.Metrics {
			if series.Name != metrics.RequestDurationSeconds {
				continue
			}
			histogram, ok := series.Data.(metricdata.Histogram[float64])
			if !ok {
				t.Fatalf("%s is a %T, want a Histogram[float64]", series.Name, series.Data)
			}
			for _, point := range histogram.DataPoints {
				for _, kv := range point.Attributes.ToSlice() {
					if string(kv.Key) == metrics.KeyOperation {
						counts[kv.Value.AsString()] = point.Count
					}
				}
			}
		}
	}
	return counts
}

func TestARejectionCountsAsARequestAndNeverAsAnError(t *testing.T) {
	w := wire(t, allowAll())
	w.seed(t, counterAt(limit))

	if _, err := w.service.Bump(withExecution(t, budgeted(t)), bumpCounter{Counter: subjectID, By: 1}); err != nil {
		t.Fatalf("Bump() error = %v, want nil", err)
	}

	if got := w.requestSeries(t)[operationBump+"/rejected"]; got != 1 {
		t.Errorf("%s{rejected} count = %d, want 1", metrics.RequestDurationSeconds, got)
	}
	for _, point := range durationPoints(t, w.reader) {
		if errorType, labelled := labelsOf(point.Attributes)[metrics.KeyErrorType]; labelled {
			t.Errorf("error.type = %q, want none: the refusing branch of the UPR is not a fault (DEC-04)", errorType)
		}
	}
	assertTheServiceSeriesAreGone(t, w.reader)
}

func TestATechnicalFailureCountsUnderItsCategory(t *testing.T) {
	w := wire(t, func(context.Context, bumpCounter) error {
		return errors.New("timeout dialing the policy engine")
	})

	if _, err := w.service.Bump(withExecution(t, budgeted(t)), bumpCounter{Counter: subjectID, By: 1}); err == nil {
		t.Fatal("Bump() error = nil, want the authorizer failure")
	}

	points := durationPoints(t, w.reader)
	if len(points) != 1 || points[0].Count != 1 {
		t.Fatalf("%s = %+v, want a single point of 1 (MET-10)", metrics.RequestDurationSeconds, points)
	}
	labels := labelsOf(points[0].Attributes)
	if labels[metrics.KeyErrorType] != "storage" || labels[metrics.KeyOperation] != operationBump {
		t.Errorf("labels = %v, want the injected category with the operation", labels)
	}
	if got := w.requestSeries(t)[operationBump+"/failed"]; got != 1 {
		t.Errorf("%s{failed} count = %d, want the failure counted as a request too", metrics.RequestDurationSeconds, got)
	}
	assertTheServiceSeriesAreGone(t, w.reader)
}

// LOG-14: the audit trail is a channel of its own. A record that also went to
// the log would put an auditable fact where log retention, sampling and levels
// could drop it.
func TestTheAuditTrailNeverPassesThroughTheLogHandler(t *testing.T) {
	w := wire(t, allowAll())

	if _, err := w.service.Bump(withExecution(t, budgeted(t)), bumpCounter{Counter: subjectID, By: 1}); err != nil {
		t.Fatalf("Bump() error = %v, want nil", err)
	}

	if events := w.trail.Events(); len(events) != 1 {
		t.Fatalf("audit trail = %+v, want exactly one record", events)
	}
	for _, record := range w.records(t) {
		for key, value := range record {
			if text, isText := value.(string); isText && strings.Contains(text, string(subjectID)) {
				t.Errorf("the log record carries %s=%q; the audit object belongs to the trail, not the log", key, text)
			}
		}
	}
}

// The end-to-end shape of TRC-15, MET-07 and LOG-13: a personal identifier in
// the message of a technical failure must not leave the process through any of
// the three channels.
func TestAPersonalIdentifierInAFailureLeavesThroughNoChannel(t *testing.T) {
	const cpf = "123.456.789-00"
	w := wire(t, func(context.Context, bumpCounter) error {
		return errors.New("the policy engine refused cpf=" + cpf + " with no reason")
	})

	if _, err := w.service.Bump(withExecution(t, budgeted(t)), bumpCounter{Counter: subjectID, By: 1}); err == nil {
		t.Fatal("Bump() error = nil, want the authorizer failure")
	}

	span := w.endedSpan(t)
	if strings.Contains(span.Status.Description, "123.456") {
		t.Errorf("the span status carries %q (TRC-12)", span.Status.Description)
	}
	for _, kv := range span.Attributes {
		if strings.Contains(kv.Value.AsString(), "123.456") {
			t.Errorf("span attribute %s carries the identifier (TRC-15)", kv.Key)
		}
	}

	for _, point := range durationPoints(t, w.reader) {
		for key, value := range labelsOf(point.Attributes) {
			if strings.Contains(value, "123.456") {
				t.Errorf("label %s = %q carries the identifier (MET-07)", key, value)
			}
		}
	}

	// This flow writes no log record at all, and that is the assertion: an
	// unlogged failure cannot leak through the log. What the service does write
	// when something goes wrong with the trail is covered separately, below.
	if records := w.records(t); len(records) != 0 {
		for _, record := range records {
			for key, value := range record {
				if text, isText := value.(string); isText && strings.Contains(text, "123.456") {
					t.Errorf("log record field %s = %q carries the identifier (LOG-13)", key, text)
				}
			}
		}
	}
}

// refusingSink stands for a trail that is unavailable. Its error carries a
// personal identifier, which is what the log must not repeat.
type refusingSink struct{ err error }

func (s refusingSink) Emit(context.Context, audit.Event) error { return s.err }

func TestAFailingAuditSinkIsReportedByCategoryAndNeverByMessage(t *testing.T) {
	const cpf = "123.456.789-00"
	w := wire(t, allowAll())
	w.service.Instrumentation = usecase.New(
		w.runtime,
		refusingSink{err: errors.New("the trail refused the record for cpf=" + cpf)},
		func(context.Context) string { return "svc-a" },
		func(error) string { return "storage" },
		operationFind,
	)

	if _, err := w.service.Bump(withExecution(t, budgeted(t)), bumpCounter{Counter: subjectID, By: 1}); err != nil {
		t.Fatalf("Bump() error = %v, want nil: a trail that refuses does not fail the operation", err)
	}

	records := w.records(t)
	if len(records) != 1 {
		t.Fatalf("log records = %d, want exactly 1: losing an audit record is never silent", len(records))
	}

	record := records[0]
	if record[redact.KeyErrorType] != redact.CategoryUnclassified {
		t.Errorf("%s = %v, want %q: the failure of the sink by redact.Error (RF-A3)", redact.KeyErrorType, record[redact.KeyErrorType], redact.CategoryUnclassified)
	}
	if record["dmpf.audit.action"] != operationBump {
		t.Errorf("dmpf.audit.action = %v, want %q past the processor (RF-A3)", record["dmpf.audit.action"], operationBump)
	}
	for key, value := range record {
		if text, isText := value.(string); isText && strings.Contains(text, "123.456") {
			t.Errorf("log record field %s = %q carries the identifier (LOG-13)", key, text)
		}
	}
}
