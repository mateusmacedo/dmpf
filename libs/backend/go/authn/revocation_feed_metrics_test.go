package authn_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
)

type feedReading struct {
	events map[string]int64
	delays []float64
	unit   string
}

func readFeedMetrics(t *testing.T, reader *sdkmetric.ManualReader) feedReading {
	t.Helper()

	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	got := feedReading{events: map[string]int64{}}
	for _, scope := range data.ScopeMetrics {
		for _, m := range scope.Metrics {
			if !strings.HasPrefix(m.Name, "dmpf.auth.revocation.") {
				continue
			}
			if scope.Scope.Name != authnScope {
				t.Errorf("%s under scope %q, want %q", m.Name, scope.Scope.Name, authnScope)
			}
			switch m.Name {
			case "dmpf.auth.revocation.events":
				sum, ok := m.Data.(metricdata.Sum[int64])
				if !ok || !sum.IsMonotonic {
					t.Fatalf("%s = %T, want a monotonic int64 counter", m.Name, m.Data)
				}
				for _, point := range sum.DataPoints {
					var labels []string
					for _, kv := range point.Attributes.ToSlice() {
						labels = append(labels, string(kv.Key)+"="+kv.Value.String())
					}
					got.events[strings.Join(labels, ",")] += point.Value
				}
			case "dmpf.auth.revocation.apply_delay":
				histogram, ok := m.Data.(metricdata.Histogram[float64])
				if !ok {
					t.Fatalf("%s = %T, want a float64 histogram", m.Name, m.Data)
				}
				got.unit = m.Unit
				for _, point := range histogram.DataPoints {
					if point.Attributes.Len() != 0 {
						t.Errorf("%s carries %v, want no attribute", m.Name, point.Attributes.ToSlice())
					}
					if point.Count != 1 {
						t.Fatalf("%s holds %d samples in one point, want each test to record at most one", m.Name, point.Count)
					}
					got.delays = append(got.delays, point.Sum)
				}
			default:
				t.Errorf("unexpected metric %s from the feed", m.Name)
			}
		}
	}
	return got
}

func withFeedTelemetry(reader *sdkmetric.ManualReader, sink, raw *records) func(*authn.FeedConfig) {
	return func(cfg *authn.FeedConfig) {
		cfg.MeterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
		cfg.LoggerProvider = sink.apiProvider(raw)
	}
}

func foreignEvent(occurred time.Time) authn.SessionEvent {
	e := sessionEnded("jti-1", aliceSession, occurred)
	e.Issuer = "https://evil.example.com/realms/dmpf"
	return e
}

func invalidEvent(occurred time.Time) authn.SessionEvent {
	e := subjectRevoked("evt-1", occurred)
	e.Kind = authn.SessionEventKind("subject_revoked:" + aliceSubject)
	return e
}

func TestRecordCountsEachOutcomeAndTheDelayOfWhatItApplied(t *testing.T) {
	occurred := feedEpoch.Add(-12 * time.Second)

	tests := []struct {
		name       string
		before     []authn.SessionEvent
		event      authn.SessionEvent
		want       string
		wantDelays []float64
	}{
		{"applied", nil, sessionEnded("jti-1", aliceSession, occurred), "outcome=applied", []float64{12}},
		{"duplicate", []authn.SessionEvent{sessionEnded("jti-1", aliceSession, occurred)},
			sessionEnded("jti-1", aliceSession, occurred), "outcome=duplicate", nil},
		{"expired", nil, subjectRevoked("evt-1", feedEpoch.Add(-time.Hour)), "outcome=expired", nil},
		{"invalid", nil, invalidEvent(occurred), "outcome=invalid", nil},
		{"foreign issuer", nil, foreignEvent(occurred), "outcome=foreign_issuer", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := sdkmetric.NewManualReader()
			f := newFeedFixture(t, withFeedTelemetry(reader, &records{}, &records{}))
			for _, event := range tt.before {
				if _, err := f.feed.Apply(t.Context(), event); err != nil {
					t.Fatal(err)
				}
			}

			outcome, err := f.feed.Apply(t.Context(), tt.event)
			if err != nil {
				t.Fatal(err)
			}
			f.feed.Record(t.Context(), tt.event, outcome)

			got := readFeedMetrics(t, reader)
			if len(got.events) != 1 || got.events[tt.want] != 1 {
				t.Fatalf("dmpf.auth.revocation.events = %v, want one event with %s and nothing else", got.events, tt.want)
			}
			if len(got.delays) != len(tt.wantDelays) || (len(tt.wantDelays) == 1 && got.delays[0] != tt.wantDelays[0]) {
				t.Fatalf("dmpf.auth.revocation.apply_delay = %v, want %v", got.delays, tt.wantDelays)
			}
			if len(tt.wantDelays) > 0 && got.unit != "s" {
				t.Errorf("apply_delay unit = %q, want s", got.unit)
			}
		})
	}
}

func TestApplyAloneCountsNothing(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	sink := &records{}
	f := newFeedFixture(t, withFeedTelemetry(reader, sink, &records{}))

	for range 3 {
		if _, err := f.feed.Apply(t.Context(), foreignEvent(feedEpoch)); err != nil {
			t.Fatal(err)
		}
		if _, err := f.feed.Apply(t.Context(), sessionEnded("jti-1", aliceSession, feedEpoch)); err != nil {
			t.Fatal(err)
		}
	}

	got := readFeedMetrics(t, reader)
	if len(got.events) != 0 || len(got.delays) != 0 {
		t.Fatalf("after Apply alone: events = %v, delays = %v; want nothing: a reapplied window counts once, by Record", got.events, got.delays)
	}
	if logged := sink.all(); len(logged) != 0 {
		t.Fatalf("%d records after Apply alone, want none", len(logged))
	}
}

func TestADelayFromAClockAheadOfTheIdPIsNotNegative(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	f := newFeedFixture(t, withFeedTelemetry(reader, &records{}, &records{}))
	event := sessionEnded("jti-1", aliceSession, feedEpoch.Add(2*time.Second))

	outcome, err := f.feed.Apply(t.Context(), event)
	if err != nil {
		t.Fatal(err)
	}
	f.feed.Record(t.Context(), event, outcome)

	if got := readFeedMetrics(t, reader).delays; len(got) != 1 || got[0] != 0 {
		t.Fatalf("apply_delay = %v, want a single 0", got)
	}
}

func TestAnOutcomeOutsideTheContractCountsAsInvalid(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	f := newFeedFixture(t, withFeedTelemetry(reader, &records{}, &records{}))

	f.feed.Record(t.Context(), sessionEnded("jti-1", aliceSession, feedEpoch), authn.EventOutcome("applied:"+aliceSession))

	if got := readFeedMetrics(t, reader); len(got.events) != 1 || got.events["outcome=invalid"] != 1 || len(got.delays) != 0 {
		t.Fatalf("events = %v, delays = %v; want one invalid and no delay", got.events, got.delays)
	}
}

func TestAnEmptyOutcomeFromAFailedApplyIsNeitherCountedNorLogged(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	sink, raw := &records{}, &records{}
	f := newFeedFixture(t, withFeedTelemetry(reader, sink, raw))

	f.feed.Record(t.Context(), sessionEnded("jti-1", aliceSession, feedEpoch), "")

	if got := readFeedMetrics(t, reader); len(got.events) != 0 || len(got.delays) != 0 {
		t.Fatalf("events = %v, delays = %v; want nothing for the outcome of a store failure", got.events, got.delays)
	}
	if logged := raw.all(); len(logged) != 0 {
		t.Fatalf("%d records, want none", len(logged))
	}
}

func TestOnlyARefusedEventIsLoggedAndWithoutWhoItConcerns(t *testing.T) {
	occurred := feedEpoch.Add(-time.Second)

	tests := []struct {
		name    string
		event   authn.SessionEvent
		outcome authn.EventOutcome
		logged  bool
	}{
		{"applied", sessionEnded("jti-1", aliceSession, occurred), authn.EventApplied, false},
		{"duplicate", sessionEnded("jti-1", aliceSession, occurred), authn.EventDuplicate, false},
		{"expired", subjectRevoked("evt-1", occurred), authn.EventExpired, false},
		{"invalid", invalidEvent(occurred), authn.EventInvalid, true},
		{"foreign issuer", foreignEvent(occurred), authn.EventForeignIssuer, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sink, raw := &records{}, &records{}
			f := newFeedFixture(t, withFeedTelemetry(sdkmetric.NewManualReader(), sink, raw))

			f.feed.Record(t.Context(), tt.event, tt.outcome)

			for _, record := range raw.all() {
				texts := []string{record.Body().AsString()}
				for _, value := range attributesOf(record) {
					texts = append(texts, value)
				}
				for _, text := range texts {
					for _, personal := range []string{aliceSubject, aliceSession, "jti-1", "evt-1", "evil.example.com"} {
						if strings.Contains(text, personal) {
							t.Errorf("raw record %q carries %q", record.Body().AsString(), personal)
						}
					}
				}
			}

			logged := sink.all()
			if !tt.logged {
				if len(logged) != 0 {
					t.Fatalf("%d records, want none", len(logged))
				}
				return
			}
			if len(logged) != 1 {
				t.Fatalf("%d records, want one", len(logged))
			}
			record := logged[0]
			if record.Body().AsString() != "auth: session event refused" {
				t.Errorf("body = %q", record.Body().AsString())
			}
			attrs := attributesOf(record)
			if attrs["dmpf.auth.revocation.outcome"] != string(tt.outcome) || attrs["error.type"] != "Validation" {
				t.Errorf("attributes = %v, want the outcome and error.type=Validation", attrs)
			}
			for key, value := range attrs {
				for _, personal := range []string{aliceSubject, aliceSession, "jti-1", "evt-1", "evil.example.com"} {
					if strings.Contains(value, personal) {
						t.Errorf("attribute %s = %q carries %q", key, value, personal)
					}
				}
			}
		})
	}
}

func TestWithoutAMeterProviderTheFeedMetricsGoToTheGlobalOne(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(previous) })

	f := newFeedFixture(t)
	event := sessionEnded("jti-1", aliceSession, feedEpoch.Add(-time.Second))
	outcome, err := f.feed.Apply(t.Context(), event)
	if err != nil {
		t.Fatal(err)
	}
	f.feed.Record(t.Context(), event, outcome)

	if got := readFeedMetrics(t, reader); got.events["outcome=applied"] != 1 || len(got.delays) != 1 {
		t.Fatalf("global events = %v, delays = %v; want the applied event counted", got.events, got.delays)
	}
}
