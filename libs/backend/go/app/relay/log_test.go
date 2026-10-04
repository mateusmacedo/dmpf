package relay

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const (
	publishFailed = "outbox publish failed"
	claimFailed   = "outbox claim failed"
	relayScope    = "github.com/mateusmacedo/dmpf/libs/backend/go/app/relay"
	brokerDown    = "unavailable"
	brokerCode    = "broker-unavailable"
)

type recordingExporter struct{ records []sdklog.Record }

func (e *recordingExporter) Export(_ context.Context, records []sdklog.Record) error {
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}
func (*recordingExporter) Shutdown(context.Context) error   { return nil }
func (*recordingExporter) ForceFlush(context.Context) error { return nil }

func withLogger(t *testing.T, relay Relay) (Relay, func() []sdklog.Record) {
	t.Helper()
	exporter := &recordingExporter{}
	provider := otelboot.NewLoggerProvider(otelboot.Config{
		Propagator: propagation.TraceContext{},
		Resource:   otelboot.Resource{ServiceName: "orders", ServiceVersion: "1.0.0", ServiceInstanceID: "orders-relay-1", Role: "relay"},
	}, exporter)
	relay.LoggerProvider = provider
	return relay, func() []sdklog.Record {
		if err := provider.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() = %v", err)
		}
		return exporter.records
	}
}

func recordsNamed(records []sdklog.Record, body string) []sdklog.Record {
	var found []sdklog.Record
	for _, record := range records {
		if record.Body().AsString() == body {
			found = append(found, record)
		}
	}
	return found
}

func logAttributes(record sdklog.Record) map[string]attribute.Value {
	attributes := map[string]attribute.Value{}
	record.WalkAttributes(func(kv attribute.KeyValue) bool {
		attributes[string(kv.Key)] = kv.Value
		return true
	})
	return attributes
}

func requireLogged(t *testing.T, record sdklog.Record, want map[string]attribute.Value) {
	t.Helper()
	got := logAttributes(record)
	for key, value := range want {
		if actual, ok := got[key]; !ok {
			t.Errorf("%q carries no %s, want %v", record.Body().AsString(), key, value.String())
		} else if actual != value {
			t.Errorf("%q %s = %v, want %v", record.Body().AsString(), key, actual.String(), value.String())
		}
	}
}

func requireNoErrorText(t *testing.T, record sdklog.Record, text string) {
	t.Helper()
	for key, value := range logAttributes(record) {
		if strings.Contains(value.String(), text) {
			t.Errorf("%q %s = %q carries the error text, want only its category", record.Body().AsString(), key, value.String())
		}
	}
}

func TestEachFailedPublishAttemptLogsOneWarningUnderItsSend(t *testing.T) {
	failing := publishFunc(func(context.Context, string, []byte) error {
		return categorizedError{category: brokerDown, code: brokerCode}
	})
	first, second := recordOfTenant(t, 1, 1), recordOfTenant(t, 1, 2)
	relay, recorder := sendRelay(newFakeStore([]postgres.Claimed{first}, []postgres.Claimed{second}), failing)
	relay, collect := withLogger(t, relay)

	runBriefly(t, relay)
	logged := recordsNamed(collect(), publishFailed)
	attempts := sends(recorder)

	if len(logged) != 2 || len(attempts) != 2 {
		t.Fatalf("%d %q records for %d sends, want one per failed attempt (2)", len(logged), publishFailed, len(attempts))
	}
	for i, record := range logged {
		if record.Severity() != log.SeverityWarn {
			t.Errorf("%q severity = %v, want %v", publishFailed, record.Severity(), log.SeverityWarn)
		}
		if scope := record.InstrumentationScope().Name; scope != relayScope {
			t.Errorf("%q scope = %q, want the import path of the emitting package %q (RF-A1)", publishFailed, scope, relayScope)
		}
		if record.TraceID() != attempts[i].SpanContext().TraceID() || record.SpanID() != attempts[i].SpanContext().SpanID() {
			t.Errorf("%q logged under %s/%s, want the send %s/%s", publishFailed,
				record.TraceID(), record.SpanID(), attempts[i].SpanContext().TraceID(), attempts[i].SpanContext().SpanID())
		}
		requireLogged(t, record, map[string]attribute.Value{
			string(semconv.MessagingMessageIDKey): attribute.StringValue(first.MessageID),
			tracing.KeyOutboxAttempt:              attribute.Int64Value(int64(i + 1)),
			redact.KeyErrorType:                   attribute.StringValue(brokerDown),
			redact.KeyErrorCode:                   attribute.StringValue(brokerCode),
			tracing.KeyCorrelationID:              attribute.StringValue(testCorrelation),
			tracing.KeyRequestID:                  attribute.StringValue(stringOf(t, attempts[i], tracing.KeyRequestID)),
			tracing.KeyTenantID:                   attribute.StringValue(testTenant),
		})
		requireNoErrorText(t, record, "10.0.0.9")
	}
}

func TestASuccessfulPublishLeavesNoRecord(t *testing.T) {
	relay, _ := sendRelay(newFakeStore([]postgres.Claimed{recordOfTenant(t, 1, 1)}), &fakePublisher{})
	relay, collect := withLogger(t, relay)

	runBriefly(t, relay)

	if records := collect(); len(records) != 0 {
		t.Fatalf("%d records after a successful publish (first: %q), want none: the send covers it", len(records), records[0].Body().AsString())
	}
}

func TestARecordThatCannotBeAssembledLogsItsFailedAttempt(t *testing.T) {
	record := publishableRecord(t)
	record.Metadata = []byte(`{"causationid":"caus-1","traceparent":"00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"}`)
	store := newFakeStore([]postgres.Claimed{record})
	relay, collect := withLogger(t, loopRelay(store, &fakePublisher{}))

	runBriefly(t, relay)
	logged := recordsNamed(collect(), publishFailed)

	if len(logged) != 1 {
		t.Fatalf("%d %q records, want one for the attempt that never reached the broker", len(logged), publishFailed)
	}
	requireLogged(t, logged[0], map[string]attribute.Value{
		string(semconv.MessagingMessageIDKey): attribute.StringValue(record.MessageID),
		tracing.KeyOutboxAttempt:              attribute.Int64Value(1),
		redact.KeyErrorType:                   attribute.StringValue(redact.CategoryUnclassified),
	})
}

func TestAPublishCancelledByTheShutdownIsNotLogged(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelling := publishFunc(func(ctx context.Context, _ string, _ []byte) error {
		cancel()
		<-ctx.Done()
		return ctx.Err()
	})
	relay, collect := withLogger(t, loopRelay(newFakeStore([]postgres.Claimed{recordOfTenant(t, 1, 1)}), cancelling))

	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil on shutdown", err)
	}
	if logged := recordsNamed(collect(), publishFailed); len(logged) != 0 {
		t.Fatalf("%d %q records for a publish the shutdown cancelled, want none: it did not fail on its own merits", len(logged), publishFailed)
	}
}

type claimFailingStore struct {
	*fakeStore
	err error
}

func (s claimFailingStore) Claim(context.Context, string, int, time.Duration) ([]postgres.Claimed, error) {
	return nil, s.err
}

func TestAFailedClaimLogsAnErrorWithItsCategory(t *testing.T) {
	failure := categorizedError{category: brokerDown, code: brokerCode}
	relay, collect := withLogger(t, loopRelay(claimFailingStore{fakeStore: newFakeStore(), err: failure}, &fakePublisher{}))

	if err := relay.Run(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("Run() = %v, want the claim failure", err)
	}
	logged := recordsNamed(collect(), claimFailed)

	if len(logged) != 1 {
		t.Fatalf("%d %q records, want exactly one", len(logged), claimFailed)
	}
	if logged[0].Severity() != log.SeverityError {
		t.Errorf("%q severity = %v, want %v", claimFailed, logged[0].Severity(), log.SeverityError)
	}
	requireLogged(t, logged[0], map[string]attribute.Value{
		redact.KeyErrorType: attribute.StringValue(brokerDown),
		redact.KeyErrorCode: attribute.StringValue(brokerCode),
	})
	requireNoErrorText(t, logged[0], "10.0.0.9")
}

type claimCancellingStore struct {
	*fakeStore
	cancel context.CancelFunc
}

func (s claimCancellingStore) Claim(ctx context.Context, _ string, _ int, _ time.Duration) ([]postgres.Claimed, error) {
	s.cancel()
	return nil, ctx.Err()
}

func TestAClaimCancelledByTheShutdownIsNotLogged(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := claimCancellingStore{fakeStore: newFakeStore(), cancel: cancel}
	relay, collect := withLogger(t, loopRelay(store, &fakePublisher{}))

	if err := relay.Run(ctx); err != nil {
		t.Fatalf("Run() = %v, want nil on shutdown", err)
	}
	if logged := recordsNamed(collect(), claimFailed); len(logged) != 0 {
		t.Fatalf("%d %q records on shutdown, want none: stopping is not a failure", len(logged), claimFailed)
	}
}

func TestNewCarriesTheLoggerOntoTheRelay(t *testing.T) {
	config := validConfig()
	config.LoggerProvider = sdklog.NewLoggerProvider()

	relay, err := New(newFakeStore(), &fakePublisher{}, &countingIDs{}, fixedClock(0), config)
	if err != nil {
		t.Fatalf("New() = %v, want nil", err)
	}
	if relay.LoggerProvider != config.LoggerProvider {
		t.Fatalf("LoggerProvider = %v, want the declared %v", relay.LoggerProvider, config.LoggerProvider)
	}
}

func TestARelayWithoutALoggerStillRuns(t *testing.T) {
	failing := publishFunc(func(context.Context, string, []byte) error { return errTransport })
	relay := loopRelay(newFakeStore([]postgres.Claimed{recordOfTenant(t, 1, 1)}), failing)

	runBriefly(t, relay)
}

type countingProvider struct {
	log.LoggerProvider
	loggers atomic.Int64
}

func (p *countingProvider) Logger(name string, options ...log.LoggerOption) log.Logger {
	p.loggers.Add(1)
	return p.LoggerProvider.Logger(name, options...)
}

func TestTheRelayBuildsItsLoggerOncePerRun(t *testing.T) {
	failing := publishFunc(func(context.Context, string, []byte) error { return errTransport })
	relay := loopRelay(newFakeStore([]postgres.Claimed{recordOfTenant(t, 1, 1)}, []postgres.Claimed{recordOfTenant(t, 1, 2)}), failing)
	provider := &countingProvider{LoggerProvider: sdklog.NewLoggerProvider()}
	relay.LoggerProvider = provider

	runBriefly(t, relay)

	if built := provider.loggers.Load(); built != 1 {
		t.Fatalf("%d loggers built for two failed attempts, want one per run", built)
	}
}

type claimFailingAfterADrainStore struct {
	*fakeStore
	err error
}

func (s claimFailingAfterADrainStore) Claim(ctx context.Context, claimID string, batch int, lease time.Duration) ([]postgres.Claimed, error) {
	if s.claims.Load() > 0 {
		return nil, s.err
	}
	return s.fakeStore.Claim(ctx, claimID, batch, lease)
}

func TestTheLoopRecordAfterADrainCarriesNoExecution(t *testing.T) {
	store := claimFailingAfterADrainStore{fakeStore: newFakeStore([]postgres.Claimed{recordOfTenant(t, 1, 1)}), err: errTransport}
	relay, _ := sendRelay(store, &fakePublisher{})
	relay, collect := withLogger(t, relay)

	if err := relay.Run(context.Background()); !errors.Is(err, errTransport) {
		t.Fatalf("Run() = %v, want the claim failure", err)
	}
	logged := recordsNamed(collect(), claimFailed)

	if len(logged) != 1 {
		t.Fatalf("%d %q records, want exactly one", len(logged), claimFailed)
	}
	if logged[0].TraceID().IsValid() {
		t.Errorf("%q logged under trace %s, want none: the drain that preceded it had ended", claimFailed, logged[0].TraceID())
	}
	got := logAttributes(logged[0])
	for _, key := range []string{tracing.KeyCorrelationID, tracing.KeyRequestID, tracing.KeyTenantID} {
		if value, ok := got[key]; ok {
			t.Errorf("%q carries %s = %v, want it absent outside an execution (CTX-26)", claimFailed, key, value.String())
		}
	}
}
