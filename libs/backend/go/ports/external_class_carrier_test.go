package ports_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestExternalClassCarrier(t *testing.T) {
	tests := []struct {
		name      string
		slot      bool
		marks     []ports.ExternalClass
		wantClass ports.ExternalClass
		wantOK    bool
	}{
		{"no slot ignores the mark", false, []ports.ExternalClass{ports.ClassAccepted}, 0, false},
		{"slot without a mark", true, nil, 0, false},
		{"slot keeps the mark", true, []ports.ExternalClass{ports.ClassUnknownSent}, ports.ClassUnknownSent, true},
		{"the last mark wins", true, []ports.ExternalClass{ports.ClassRetryableTransport, ports.ClassAccepted}, ports.ClassAccepted, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.slot {
				ctx = ports.WithExternalClassSlot(ctx)
			}
			downstream, cancel := context.WithCancel(ctx)
			defer cancel()
			for _, class := range tt.marks {
				ports.MarkExternalClass(downstream, class)
			}

			class, ok := ports.ExternalClassFrom(ctx)
			if class != tt.wantClass || ok != tt.wantOK {
				t.Fatalf("ExternalClassFrom = %v, %v; want %v, %v", class, ok, tt.wantClass, tt.wantOK)
			}
		})
	}
}

func TestExternalConflictCarrier(t *testing.T) {
	tests := []struct {
		name  string
		slot  bool
		marks int
		want  bool
	}{
		{"no slot ignores the mark", false, 1, false},
		{"slot without a mark", true, 0, false},
		{"slot keeps the mark", true, 1, true},
		{"marking twice keeps it", true, 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			if tt.slot {
				ctx = ports.WithExternalConflictSlot(ctx)
			}
			downstream, cancel := context.WithCancel(ctx)
			defer cancel()
			for range tt.marks {
				ports.MarkExternalConflict(downstream)
			}

			if got := ports.ExternalConflictFrom(ctx); got != tt.want {
				t.Fatalf("ExternalConflictFrom = %v, want %v", got, tt.want)
			}
		})
	}
}
