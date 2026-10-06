package relay

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

var semconvDurationBoundaries = []float64{0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10}

func meteredRelay(store Store, publisher Publisher) (Relay, *sdkmetric.ManualReader) {
	reader := sdkmetric.NewManualReader()
	relay, _ := sendRelay(store, publisher)
	relay.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	return relay, reader
}

func collected(t *testing.T, reader *sdkmetric.ManualReader) map[string]metricdata.Metrics {
	t.Helper()

	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	byName := map[string]metricdata.Metrics{}
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			byName[m.Name] = m
		}
	}
	return byName
}

func instrument(t *testing.T, metrics map[string]metricdata.Metrics, name, unit string) metricdata.Metrics {
	t.Helper()

	m, ok := metrics[name]
	if !ok {
		t.Fatalf("no %s among %d instruments", name, len(metrics))
	}
	if m.Unit != unit {
		t.Fatalf("%s unit = %q, want %q", name, m.Unit, unit)
	}
	return m
}

func sentPoints(t *testing.T, metrics map[string]metricdata.Metrics) []metricdata.DataPoint[int64] {
	t.Helper()

	m := instrument(t, metrics, "messaging.client.sent.messages", "{message}")
	sum, ok := m.Data.(metricdata.Sum[int64])
	if !ok || !sum.IsMonotonic {
		t.Fatalf("%s = %T, want a monotonic int64 counter", m.Name, m.Data)
	}
	return sum.DataPoints
}

func durationPoints(t *testing.T, metrics map[string]metricdata.Metrics) []metricdata.HistogramDataPoint[float64] {
	t.Helper()

	m := instrument(t, metrics, "messaging.client.operation.duration", "s")
	histogram, ok := m.Data.(metricdata.Histogram[float64])
	if !ok {
		t.Fatalf("%s = %T, want a float64 histogram", m.Name, m.Data)
	}
	return histogram.DataPoints
}

func requireSet(t *testing.T, what string, got attribute.Set, want map[string]string) {
	t.Helper()

	if got.Len() != len(want) {
		t.Errorf("%s attributes = %v, want exactly %v", what, got.ToSlice(), want)
	}
	for key, value := range want {
		v, ok := got.Value(attribute.Key(key))
		if !ok || v.AsString() != value {
			t.Errorf("%s %s = %q (present %v), want %q", what, key, v.AsString(), ok, value)
		}
	}
}

func TestADeliveredSendCountsTheMessageAndTimesTheOperation(t *testing.T) {
	relay, reader := meteredRelay(newFakeStore([]postgres.Claimed{recordOfTenant(t, 1, 1)}), &fakePublisher{})

	runBriefly(t, relay)
	metrics := collected(t, reader)

	base := map[string]string{
		"messaging.operation.name":   "send",
		"messaging.system":           "kafka",
		"messaging.destination.name": testAddress,
	}
	sent := sentPoints(t, metrics)
	if len(sent) != 1 || sent[0].Value != 1 {
		t.Fatalf("sent = %+v, want one message counted once", sent)
	}
	requireSet(t, "sent", sent[0].Attributes, base)

	durations := durationPoints(t, metrics)
	if len(durations) != 1 || durations[0].Count != 1 {
		t.Fatalf("durations = %+v, want one send timed", durations)
	}
	requireSet(t, "duration", durations[0].Attributes, map[string]string{
		"messaging.operation.name":   "send",
		"messaging.operation.type":   "send",
		"messaging.system":           "kafka",
		"messaging.destination.name": testAddress,
	})
	if !slices.Equal(durations[0].Bounds, semconvDurationBoundaries) {
		t.Fatalf("bounds = %v, want the semconv advisory %v (RF-D4)", durations[0].Bounds, semconvDurationBoundaries)
	}
}

func TestAFailedSendCarriesItsErrorTypeOnBothInstruments(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"categorized failure", categorizedError{category: "network"}, "network"},
		{"unclassified failure", errTransport, semconv.ErrorTypeOther.Value.AsString()},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			relay, reader := meteredRelay(newFakeStore([]postgres.Claimed{recordOfTenant(t, 1, 1)}), &fakePublisher{err: c.err})

			runBriefly(t, relay)
			metrics := collected(t, reader)

			sent := sentPoints(t, metrics)
			if len(sent) != 1 || sent[0].Value != 1 {
				t.Fatalf("sent = %+v, want the attempt counted once", sent)
			}
			requireSet(t, "sent", sent[0].Attributes, map[string]string{
				"messaging.operation.name":   "send",
				"messaging.system":           "kafka",
				"messaging.destination.name": testAddress,
				"error.type":                 c.want,
			})
			durations := durationPoints(t, metrics)
			if len(durations) != 1 {
				t.Fatalf("durations = %+v, want one series", durations)
			}
			if got, _ := durations[0].Attributes.Value("error.type"); got.AsString() != c.want {
				t.Fatalf("duration error.type = %q, want %q", got.AsString(), c.want)
			}
		})
	}
}

func TestTheTransportCategoryOfAFailedSendIsTheOneEverySignalCarries(t *testing.T) {
	const transportCategory = "Forbidden"
	publisher := publishFunc(func(ctx context.Context, _ string, _ []byte) error {
		if span, owned := tracing.OwnsSpan(ctx); owned {
			span.SetAttributes(tracing.Attributes{}.OutcomeCategory(transportCategory).KeyValues()...)
			tracing.RecordError(span, transportCategory)
		}
		return errTransport
	})
	relay, recorder := sendRelay(newFakeStore([]postgres.Claimed{recordOfTenant(t, 1, 1)}), publisher)
	reader := sdkmetric.NewManualReader()
	relay.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	relay, collect := withLogger(t, relay)

	runBriefly(t, relay)
	metrics := collected(t, reader)

	if got := stringOf(t, onlySend(t, recorder), "error.type"); got != transportCategory {
		t.Fatalf("send error.type = %q, want the transport's %q", got, transportCategory)
	}
	sent := sentPoints(t, metrics)
	if len(sent) != 1 {
		t.Fatalf("sent = %+v, want the attempt counted once", sent)
	}
	if got, _ := sent[0].Attributes.Value("error.type"); got.AsString() != transportCategory {
		t.Fatalf("sent error.type = %q, want the send's %q", got.AsString(), transportCategory)
	}
	durations := durationPoints(t, metrics)
	if len(durations) != 1 {
		t.Fatalf("durations = %+v, want one series", durations)
	}
	if got, _ := durations[0].Attributes.Value("error.type"); got.AsString() != transportCategory {
		t.Fatalf("duration error.type = %q, want the send's %q", got.AsString(), transportCategory)
	}
	logged := recordsNamed(collect(), publishFailed)
	if len(logged) != 1 {
		t.Fatalf("%d %q records, want one", len(logged), publishFailed)
	}
	requireLogged(t, logged[0], map[string]attribute.Value{redact.KeyErrorType: attribute.StringValue(transportCategory)})
}

func TestARecordThatNeverReachedTheBrokerIsNotCountedAsSent(t *testing.T) {
	record := recordOfTenant(t, 1, 1)
	record.PayloadHash = "sha256:tampered"
	relay, reader := meteredRelay(newFakeStore([]postgres.Claimed{record}), &fakePublisher{})

	runBriefly(t, relay)
	metrics := collected(t, reader)

	if m, ok := metrics["messaging.client.sent.messages"]; ok {
		if sum, _ := m.Data.(metricdata.Sum[int64]); len(sum.DataPoints) != 0 {
			t.Fatalf("sent = %+v, want nothing counted for a record never published", sum.DataPoints)
		}
	}
	durations := durationPoints(t, metrics)
	if len(durations) != 1 {
		t.Fatalf("durations = %+v, want the failed send timed", durations)
	}
	if got, _ := durations[0].Attributes.Value("error.type"); got != semconv.ErrorTypeOther.Value {
		t.Fatalf("duration error.type = %q, want %s", got.AsString(), semconv.ErrorTypeOther.Value.AsString())
	}
}

func TestNewCarriesTheMeterProviderOntoTheRelay(t *testing.T) {
	config := validConfig()
	config.MeterProvider = sdkmetric.NewMeterProvider()

	relay, err := New(newFakeStore(), &fakePublisher{}, &countingIDs{}, fixedClock(0), config)
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}
	if relay.MeterProvider != config.MeterProvider {
		t.Fatalf("MeterProvider = %v, want the declared %v", relay.MeterProvider, config.MeterProvider)
	}
}

func TestARelayWithoutAMeterProviderReportsToTheGlobalOne(t *testing.T) {
	previous := otel.GetMeterProvider()
	t.Cleanup(func() { otel.SetMeterProvider(previous) })
	reader := sdkmetric.NewManualReader()
	otel.SetMeterProvider(sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)))
	relay, _ := sendRelay(newFakeStore([]postgres.Claimed{recordOfTenant(t, 1, 1)}), &fakePublisher{})

	runBriefly(t, relay)

	if sent := sentPoints(t, collected(t, reader)); len(sent) != 1 {
		t.Fatalf("sent = %+v, want the send counted on the global provider", sent)
	}
}

func TestTheSendDurationCoversThePublish(t *testing.T) {
	const hold = 20 * time.Millisecond
	relay, reader := meteredRelay(newFakeStore([]postgres.Claimed{recordOfTenant(t, 1, 1)}), &fakePublisher{hold: hold})

	runBriefly(t, relay)

	durations := durationPoints(t, collected(t, reader))
	if len(durations) != 1 || durations[0].Sum < hold.Seconds() {
		t.Fatalf("durations = %+v, want one send timed for at least %v", durations, hold)
	}
}

type countingMeterProvider struct {
	metric.MeterProvider
	meters atomic.Int64
}

func (p *countingMeterProvider) Meter(name string, options ...metric.MeterOption) metric.Meter {
	p.meters.Add(1)
	return p.MeterProvider.Meter(name, options...)
}

func TestTheSendInstrumentsAreBuiltOncePerRunNotPerSend(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := &countingMeterProvider{MeterProvider: sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))}
	relay, recorder := sendRelay(newFakeStore(batchOf(t, 5)), &fakePublisher{})
	relay.MeterProvider = provider

	runBriefly(t, relay)

	if found := sends(recorder); len(found) != 5 {
		t.Fatalf("%d sends, want the batch of 5", len(found))
	}
	if got := provider.meters.Load(); got != 1 {
		t.Fatalf("Meter() called %d times for 5 sends, want once for the run", got)
	}
	if sent := sentPoints(t, collected(t, reader)); len(sent) != 1 || sent[0].Value != 5 {
		t.Fatalf("sent = %+v, want the 5 sends counted on the instruments built once", sent)
	}
}
