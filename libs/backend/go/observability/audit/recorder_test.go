package audit_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type fixedClock ports.Instant

func (c fixedClock) Now() ports.Instant { return ports.Instant(c) }

const (
	recordedAt = ports.Instant(1_760_000_000_000_000_000)
	sinkSecret = "disk full at /srv/trail/acme-secret"
)

type refusingSink struct{ emitted int }

func (s *refusingSink) Emit(context.Context, audit.Event) error {
	s.emitted++
	return errors.New(sinkSecret)
}

func platformEvent() audit.Event {
	return audit.Event{
		Subject:    "operator-1",
		Object:     "orders",
		Action:     audit.ActionPlatformRead,
		Outcome:    "accepted",
		Tenant:     "acme",
		DataTenant: "globex",
	}
}

func logInto(out *strings.Builder) *slog.Logger {
	return slog.New(slog.NewJSONHandler(out, nil))
}

func logLines(t *testing.T, out string) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for line := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("log line %q is not JSON: %v", line, err)
		}
		lines = append(lines, record)
	}
	return lines
}

func TestNewRecorderPanicsWithoutAnyOfItsParts(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	cases := map[string]func(){
		"sink":   func() { audit.NewRecorder(nil, logger, fixedClock(recordedAt)) },
		"logger": func() { audit.NewRecorder(&audit.Recording{}, nil, fixedClock(recordedAt)) },
		"clock":  func() { audit.NewRecorder(&audit.Recording{}, logger, nil) },
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("NewRecorder without %s did not panic; an access across tenants without a trail must not be built", name)
				}
			}()
			build()
		})
	}
}

func TestRecorderEmitsToTheSinkAndCopiesTheEventToTheLog(t *testing.T) {
	trail := &audit.Recording{}
	var out strings.Builder
	recorder := audit.NewRecorder(trail, logInto(&out), fixedClock(recordedAt))

	if err := recorder.Record(context.Background(), platformEvent()); err != nil {
		t.Fatalf("Record() = %v, want nil", err)
	}

	want := platformEvent()
	want.At = recordedAt
	if got := trail.Events(); len(got) != 1 || got[0] != want {
		t.Fatalf("trail = %+v, want [%+v]: the instant comes from the clock when the event has none", got, want)
	}
	lines := logLines(t, out.String())
	if len(lines) != 1 {
		t.Fatalf("log has %d lines, want 1: the log channel receives every event", len(lines))
	}
	for key, value := range map[string]string{
		audit.KeyAction: want.Action, audit.KeyObject: want.Object, audit.KeySubject: want.Subject,
		audit.KeyOutcome: want.Outcome, audit.KeyDataTenantID: want.DataTenant, "dmpf.tenant_id": want.Tenant,
	} {
		if lines[0][key] != value {
			t.Errorf("log %s = %v, want %q", key, lines[0][key], value)
		}
	}
}

func TestRecorderKeepsTheInstantTheEventCarries(t *testing.T) {
	trail := &audit.Recording{}
	recorder := audit.NewRecorder(trail, slog.New(slog.DiscardHandler), fixedClock(recordedAt))
	event := platformEvent()
	event.At = ports.Instant(42)

	if err := recorder.Record(context.Background(), event); err != nil {
		t.Fatalf("Record() = %v, want nil", err)
	}
	if got := trail.Events()[0].At; got != 42 {
		t.Fatalf("At = %d, want 42", got)
	}
}

func TestRecorderFailsClosedWhenTheSinkRefuses(t *testing.T) {
	sink := &refusingSink{}
	var out strings.Builder
	recorder := audit.NewRecorder(sink, logInto(&out), fixedClock(recordedAt))

	err := recorder.Record(context.Background(), platformEvent())
	if !errors.Is(err, ports.ErrAuditUnavailable) {
		t.Fatalf("Record() = %v, want ErrAuditUnavailable", err)
	}
	if strings.Contains(err.Error(), sinkSecret) {
		t.Fatalf("Record() = %q embeds the sink's message, which may carry what redaction exists to keep out", err)
	}
	if sink.emitted != 1 {
		t.Fatalf("sink saw %d emits, want 1", sink.emitted)
	}
	lines := logLines(t, out.String())
	if len(lines) != 1 {
		t.Fatalf("log has %d lines, want 1", len(lines))
	}
	if lines[0][audit.KeyAction] != audit.ActionPlatformRead || lines[0][audit.KeyDataTenantID] != "globex" {
		t.Fatalf("log line = %v, want the refused event whole", lines[0])
	}
	if strings.Contains(out.String(), sinkSecret) {
		t.Fatal("the log carries the sink's message; only its category may leave (LOG-07)")
	}
}

func TestNewRecorderAcceptsNoSlogHandler(t *testing.T) {
	handler := reflect.TypeFor[slog.Handler]()
	signature := reflect.TypeOf(audit.NewRecorder)
	for i := range signature.NumIn() {
		if param := signature.In(i); param == handler || param.Implements(handler) {
			t.Fatalf("NewRecorder takes %v, a slog.Handler: the trail is separate from logging (LOG-13)", param)
		}
	}
}

func TestTrailRecordsThePortRecordAsAnEventStampedByTheClock(t *testing.T) {
	trail := &audit.Recording{}
	port := audit.NewRecorder(trail, slog.New(slog.DiscardHandler), fixedClock(recordedAt)).Trail()

	record := ports.AuditRecord{
		Action: ports.AuditPlatformRead, Subject: "operator-1", Object: "orders",
		Outcome: "accepted", Tenant: "acme", DataTenant: "globex",
	}
	if err := port.Record(context.Background(), record); err != nil {
		t.Fatalf("Record() = %v, want nil", err)
	}

	want := platformEvent()
	want.At = recordedAt
	if got := trail.Events(); len(got) != 1 || got[0] != want {
		t.Fatalf("trail = %+v, want [%+v]", got, want)
	}
}

func TestTrailFailsClosedWhenTheSinkRefuses(t *testing.T) {
	port := audit.NewRecorder(&refusingSink{}, slog.New(slog.DiscardHandler), fixedClock(recordedAt)).Trail()

	err := port.Record(context.Background(), ports.AuditRecord{Action: ports.AuditProjectionWrite, Object: "orders"})
	if !errors.Is(err, ports.ErrAuditUnavailable) {
		t.Fatalf("Record() = %v, want ErrAuditUnavailable", err)
	}
}

func TestTrailOfANilRecorderIsNil(t *testing.T) {
	var recorder *audit.Recorder
	if trail := recorder.Trail(); trail != nil {
		t.Fatalf("Trail() of a nil Recorder = %v, want nil, so the provider refuses the construction", trail)
	}
}
