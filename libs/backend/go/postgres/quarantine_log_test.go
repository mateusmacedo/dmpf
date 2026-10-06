//go:build integration

package postgres_test

import (
	"context"
	"sync"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const postgresScope = "github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

type recordingProcessor struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (p *recordingProcessor) provider() log.LoggerProvider {
	return sdklog.NewLoggerProvider(sdklog.WithProcessor(p))
}

func (p *recordingProcessor) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }
func (p *recordingProcessor) Shutdown(context.Context) error                         { return nil }
func (p *recordingProcessor) ForceFlush(context.Context) error                       { return nil }

func (p *recordingProcessor) OnEmit(_ context.Context, record *sdklog.Record) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.records = append(p.records, record.Clone())
	return nil
}

func (p *recordingProcessor) named(message string) []map[string]string {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []map[string]string
	for _, record := range p.records {
		if record.Body().AsString() != message {
			continue
		}
		attributes := map[string]string{"level": record.SeverityText(), "scope": record.InstrumentationScope().Name}
		record.WalkAttributes(func(kv attribute.KeyValue) bool {
			attributes[string(kv.Key)] = kv.Value.Emit()
			return true
		})
		out = append(out, attributes)
	}
	return out
}

func (p *recordingProcessor) contained() []map[string]string { return p.named("message contained") }

func TestQuarantineLogsMessageContainedInWarnWithTheMessageIDAndTheReason(t *testing.T) {
	pool := openPool(t)
	handler := &recordingProcessor{}
	q := postgres.NewQuarantine(pool, postgres.WithQuarantineLoggerProvider(handler.provider()))

	if err := q.Quarantine(context.Background(), ports.Contained{
		Consumer: "orders", MessageID: "m-1", Reason: ports.ReasonTerminalFailure, Envelope: []byte{0x0a, 0x01}, Error: "storage: timeout", At: 100,
	}); err != nil {
		t.Fatalf("Quarantine() = %v, want nil", err)
	}

	records := handler.contained()
	if len(records) != 1 {
		t.Fatalf("message contained records = %v, want one", records)
	}
	want := map[string]string{"level": "WARN", "scope": postgresScope, "messaging.message.id": "m-1", "dmpf.containment.reason": "terminal-failure"}
	if len(records[0]) != len(want) {
		t.Errorf("message contained = %v, want exactly %v", records[0], want)
	}
	for key, value := range want {
		if records[0][key] != value {
			t.Errorf("%s = %q, want %q (RF-A6)", key, records[0][key], value)
		}
	}
}

func TestQuarantineWithoutMessageIDLogsNoMessageIDKey(t *testing.T) {
	pool := openPool(t)
	handler := &recordingProcessor{}
	q := postgres.NewQuarantine(pool, postgres.WithQuarantineLoggerProvider(handler.provider()))

	if err := q.Quarantine(context.Background(), ports.Contained{
		Consumer: "orders", Reason: ports.ReasonInvalidEnvelope, Envelope: []byte{0x01}, At: 100,
	}); err != nil {
		t.Fatalf("Quarantine() = %v, want nil", err)
	}

	records := handler.contained()
	if len(records) != 1 || records[0]["dmpf.containment.reason"] != "invalid-envelope" {
		t.Fatalf("message contained records = %v, want one with the reason", records)
	}
	if _, has := records[0]["messaging.message.id"]; has {
		t.Fatal("messaging.message.id is present without a message id; a missing key stays absent (CTX-26)")
	}
}

func TestQuarantineThatIsNotPersistedLogsNothing(t *testing.T) {
	pool := openPool(t)
	handler := &recordingProcessor{}
	q := postgres.NewQuarantine(pool, postgres.WithQuarantineLoggerProvider(handler.provider()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := q.Quarantine(ctx, ports.Contained{
		Consumer: "orders", MessageID: "m-1", Reason: ports.ReasonCollision, Envelope: []byte{0x01}, At: 100,
	}); err == nil {
		t.Fatal("Quarantine() = nil with a cancelled context")
	}
	if err := q.Quarantine(context.Background(), ports.Contained{Reason: ports.ReasonCollision, Envelope: []byte{0x01}}); err == nil {
		t.Fatal("Quarantine() = nil without consumer")
	}
	if records := handler.contained(); len(records) != 0 {
		t.Fatalf("message contained records = %v, want none: a message that was not kept is not contained", records)
	}
}
