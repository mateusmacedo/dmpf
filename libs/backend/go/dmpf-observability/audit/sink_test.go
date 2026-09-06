package audit_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/audit"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

func event(action string) audit.Event {
	return audit.Event{
		Subject: "svc-a",
		Object:  "P-100",
		Action:  action,
		Outcome: "accepted",
		At:      dmpfports.Instant(1_755_432_000_000_000_000),
	}
}

func TestJSONSinkWritesOneLinePerEvent(t *testing.T) {
	var out strings.Builder
	sink := audit.NewJSONSink(&out)

	for _, action := range []string{"orders.AddItem", "orders.PlaceOrder"} {
		if err := sink.Emit(context.Background(), event(action)); err != nil {
			t.Fatalf("Emit() = %v, want nil", err)
		}
	}

	lines := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("wrote %d lines, want 2 — one JSON object per event", len(lines))
	}

	var decoded map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &decoded); err != nil {
		t.Fatalf("Unmarshal() = %v, want a JSON object per line", err)
	}
	want := map[string]any{
		"subject": "svc-a",
		"object":  "P-100",
		"action":  "orders.AddItem",
		"outcome": "accepted",
		"at":      float64(1_755_432_000_000_000_000),
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("decoded = %+v, want %+v — the record carries the five fields of LOG-14 and nothing else", decoded, want)
	}
}

func TestJSONSinkReportsTheWriteError(t *testing.T) {
	sink := audit.NewJSONSink(failingWriter{})

	if err := sink.Emit(context.Background(), event("orders.AddItem")); err == nil {
		t.Fatal("Emit() = nil, want the write error — a dropped audit record must be visible")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

var errWrite = errorString("disk full")

type errorString string

func (e errorString) Error() string { return string(e) }

func TestRecordingKeepsEveryEventInOrder(t *testing.T) {
	recording := &audit.Recording{}

	for _, action := range []string{"orders.AddItem", "orders.PlaceOrder"} {
		if err := recording.Emit(context.Background(), event(action)); err != nil {
			t.Fatalf("Emit() = %v, want nil", err)
		}
	}

	got := recording.Events()
	if len(got) != 2 {
		t.Fatalf("Events() has %d events, want 2", len(got))
	}
	if got[0].Action != "orders.AddItem" || got[1].Action != "orders.PlaceOrder" {
		t.Fatalf("Events() = %+v, want them in emission order", got)
	}
}

func TestRecordingEventsIsACopy(t *testing.T) {
	recording := &audit.Recording{}
	if err := recording.Emit(context.Background(), event("orders.AddItem")); err != nil {
		t.Fatalf("Emit() = %v, want nil", err)
	}

	recording.Events()[0].Action = "tampered"

	if got := recording.Events()[0].Action; got != "orders.AddItem" {
		t.Fatalf("Action = %q after mutating the returned slice, want the trail to be immutable from outside", got)
	}
}

func TestRecordingIsSafeUnderConcurrentEmit(t *testing.T) {
	recording := &audit.Recording{}

	var wg sync.WaitGroup
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = recording.Emit(context.Background(), event("orders.AddItem"))
		}()
	}
	wg.Wait()

	if got := len(recording.Events()); got != 32 {
		t.Fatalf("Events() has %d events, want 32", got)
	}
}

func TestNoAuditConstructorAcceptsASlogHandler(t *testing.T) {
	handler := reflect.TypeOf((*slog.Handler)(nil)).Elem()
	constructor := reflect.TypeOf(audit.NewJSONSink)

	for i := range constructor.NumIn() {
		param := constructor.In(i)
		if param == handler || param.Implements(handler) {
			t.Fatalf("NewJSONSink takes %v, which is a slog.Handler: the audit channel is separate from logging (LOG-13)", param)
		}
	}
}
