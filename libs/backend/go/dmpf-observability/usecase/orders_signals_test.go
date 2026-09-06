package usecase_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	ordersapp "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application/example/orders"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/audit"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/usecase"
)

func TestTheServiceSeriesCountByOperationAndOutcome(t *testing.T) {
	w := wire(t, allowAll())
	ctx := budgeted(t)

	if _, err := w.service.AddItem(ctx, ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1}); err != nil {
		t.Fatalf("AddItem() error = %v, want nil", err)
	}
	if _, err := w.service.FindOrder(ctx, orderID); err != nil {
		t.Fatalf("FindOrder() error = %v, want nil", err)
	}

	series := w.requestSeries(t)
	for key, want := range map[string]int64{
		ordersapp.OperationAddItem + "/accepted":   1,
		ordersapp.OperationFindOrder + "/accepted": 1,
	} {
		if series[key] != want {
			t.Errorf("%s{%s} = %d, want %d (MET-09)", metrics.RequestsTotal, key, series[key], want)
		}
	}

	// One histogram series per label set, so two operations give two series,
	// each with a single observation.
	durations := w.durationSeries(t)
	for _, operation := range []string{ordersapp.OperationAddItem, ordersapp.OperationFindOrder} {
		if durations[operation] != 1 {
			t.Errorf("%s{operation=%q} count = %d, want 1 (MET-08)",
				metrics.RequestDurationSeconds, operation, durations[operation])
		}
	}
}

// durationSeries counts the observations of dmpf_service_request_duration_seconds
// per operation.
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
	w.seed(t, openSnapshot(itemLimit))

	if _, err := w.service.AddItem(budgeted(t), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1}); err != nil {
		t.Fatalf("AddItem() error = %v, want nil", err)
	}

	if got := w.requestSeries(t)[ordersapp.OperationAddItem+"/rejected"]; got != 1 {
		t.Errorf("%s{rejected} = %d, want 1", metrics.RequestsTotal, got)
	}
	if points := collect(t, w.reader)[metrics.ErrorsTotal]; len(points) != 0 {
		t.Errorf("%s = %+v, want none: the refusing branch of the UPR is not a fault (DEC-04)",
			metrics.ErrorsTotal, points)
	}
}

func TestATechnicalFailureCountsUnderItsCategory(t *testing.T) {
	w := wire(t, func(context.Context, ordersapp.Command) error {
		return errors.New("timeout dialing the policy engine")
	})

	if _, err := w.service.AddItem(budgeted(t), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1}); err == nil {
		t.Fatal("AddItem() error = nil, want the authorizer failure")
	}

	points := collect(t, w.reader)[metrics.ErrorsTotal]
	if len(points) != 1 || points[0].Value != 1 {
		t.Fatalf("%s = %+v, want a single point of 1 (MET-10)", metrics.ErrorsTotal, points)
	}
	labels := labelsOf(points[0])
	if labels[metrics.KeyErrorCategory] != "storage" || labels[metrics.KeyOperation] != ordersapp.OperationAddItem {
		t.Errorf("labels = %v, want the injected category with the operation", labels)
	}
	if got := w.requestSeries(t)[ordersapp.OperationAddItem+"/failed"]; got != 1 {
		t.Errorf("%s{failed} = %d, want the failure counted as a request too", metrics.RequestsTotal, got)
	}
}

// LOG-14: the audit trail is a channel of its own. A record that also went to
// the log would put an auditable fact where log retention, sampling and levels
// could drop it.
func TestTheAuditTrailNeverPassesThroughTheLogHandler(t *testing.T) {
	w := wire(t, allowAll())

	if _, err := w.service.AddItem(budgeted(t), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1}); err != nil {
		t.Fatalf("AddItem() error = %v, want nil", err)
	}

	if events := w.trail.Events(); len(events) != 1 {
		t.Fatalf("audit trail = %+v, want exactly one record", events)
	}
	for _, record := range w.records(t) {
		for key, value := range record {
			if text, isText := value.(string); isText && strings.Contains(text, string(orderID)) {
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
	w := wire(t, func(context.Context, ordersapp.Command) error {
		return errors.New("the policy engine refused cpf=" + cpf + " with no reason")
	})

	if _, err := w.service.AddItem(budgeted(t), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1}); err == nil {
		t.Fatal("AddItem() error = nil, want the authorizer failure")
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

	for _, point := range collect(t, w.reader)[metrics.ErrorsTotal] {
		for key, value := range labelsOf(point) {
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
		ordersapp.OperationFindOrder,
	)

	if _, err := w.service.AddItem(budgeted(t), ordersapp.AddItem{Order: orderID, SKU: "XYZ", Quantity: 1}); err != nil {
		t.Fatalf("AddItem() error = %v, want nil: a trail that refuses does not fail the operation", err)
	}

	records := w.records(t)
	if len(records) != 1 {
		t.Fatalf("log records = %d, want exactly 1: losing an audit record is never silent", len(records))
	}

	record := records[0]
	if record["error_category"] != "audit_sink" {
		t.Errorf("error_category = %v, want %q", record["error_category"], "audit_sink")
	}
	for key, value := range record {
		if text, isText := value.(string); isText && strings.Contains(text, "123.456") {
			t.Errorf("log record field %s = %q carries the identifier (LOG-13)", key, text)
		}
	}
}
