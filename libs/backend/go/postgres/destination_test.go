package postgres

import (
	"errors"
	"testing"
)

// White box on purpose: the check runs inside Enqueue and exporting it only to
// test it would widen the package surface for no caller.
func TestCheckDestination(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		destination string
		wantErr     bool
	}{
		{"logical flow", "orders.events", false},
		{"three segments", "billing.invoice.issued", false},
		{"single segment", "orders", false},
		{"digits after the first letter", "orders2.events", false},
		{"ARN names a physical target", "arn:aws:sns:us-east-1:123456789012:orders", true},
		{"path separator", "orders/events", true},
		{"broker URL", "kafka://orders", true},
		{"uppercase", "Orders.Events", true},
		{"empty", "", true},
		{"leading dot", ".orders", true},
		{"trailing dot", "orders.", true},
		{"empty segment", "orders..events", true},
		{"segment starting with a digit", "orders.2events", true},
		{"underscore", "orders_events", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := checkDestination(tc.destination)

			if tc.wantErr {
				if !errors.Is(err, ErrInvalidDestination) {
					t.Fatalf("checkDestination(%q) = %v, want ErrInvalidDestination (BLK-04)", tc.destination, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("checkDestination(%q) = %v, want nil", tc.destination, err)
			}
		})
	}
}
