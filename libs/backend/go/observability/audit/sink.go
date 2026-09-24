package audit

import (
	"context"
	"encoding/json"
	"io"
	"sync"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Event is the audit record of LOG-14: who did what to which object, with which
// outcome and when. Outcome is a plain string because the category is named in
// the application block, which a provider must not import. Tenant and
// DataTenant are filled by a cross-tenant access alone, which IDN-12 records
// with both.
type Event struct {
	Subject    string        `json:"subject"`
	Object     string        `json:"object"`
	Action     string        `json:"action"`
	Outcome    string        `json:"outcome"`
	At         ports.Instant `json:"at"`
	Tenant     string        `json:"tenant,omitempty"`
	DataTenant string        `json:"data_tenant,omitempty"`
}

// Sink is the destination of the audit trail. It is its own interface, never a
// slog.Handler: the audit channel is separate from logging and is never sampled
// (LOG-13, DAT-25).
type Sink interface {
	Emit(ctx context.Context, event Event) error
}

// NewJSONSink writes one JSON object per line to w, serialising writers so a
// concurrent trail stays parseable.
func NewJSONSink(w io.Writer) Sink {
	return &jsonSink{encoder: json.NewEncoder(w)}
}

type jsonSink struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

func (s *jsonSink) Emit(_ context.Context, event Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.encoder.Encode(event)
}

// Recording keeps the trail in memory for tests to read back. Its zero value is
// ready to use.
type Recording struct {
	mu     sync.Mutex
	events []Event
}

func (r *Recording) Emit(_ context.Context, event Event) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}

// Events returns a copy, so a reader cannot rewrite the trail it is inspecting.
func (r *Recording) Events() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Event(nil), r.events...)
}
