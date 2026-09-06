package attempt_test

import (
	"context"
	"errors"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/attempt"
)

func TestHeader(t *testing.T) {
	if attempt.Header != "dmpf-attempt" {
		t.Fatalf("Header = %q, want dmpf-attempt", attempt.Header)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	for _, n := range []int{1, 2, 17, 1000} {
		got, err := attempt.Decode(attempt.Encode(n))
		if err != nil {
			t.Fatalf("Decode(Encode(%d)) = %v, want nil", n, err)
		}
		if got != n {
			t.Fatalf("Decode(Encode(%d)) = %d", n, got)
		}
	}
}

func TestDecodeRefusesNonPositiveAndNonNumeric(t *testing.T) {
	for _, value := range []string{"0", "-1", "", "abc", "1.5", " 2", "2 "} {
		if _, err := attempt.Decode(value); !errors.Is(err, attempt.ErrInvalidAttempt) {
			t.Fatalf("Decode(%q) = %v, want ErrInvalidAttempt", value, err)
		}
	}
}

func TestAttemptTravelsInTheContext(t *testing.T) {
	if _, ok := attempt.FromContext(context.Background()); ok {
		t.Fatal("a bare context carries an attempt")
	}
	ctx := attempt.WithContext(context.Background(), 3)
	if n, ok := attempt.FromContext(ctx); !ok || n != 3 {
		t.Fatalf("FromContext = %d, %v; want 3", n, ok)
	}
	if _, ok := attempt.FromContext(attempt.WithContext(context.Background(), 0)); ok {
		t.Fatal("a non-positive attempt was reported as present")
	}
}
