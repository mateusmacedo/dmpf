package app_test

import (
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestTheIdempotencyPolicyDigestsTheCanonicalFingerprintWithSHA256(t *testing.T) {
	policy, err := app.IdempotencyPolicy(2*time.Second, 24*time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v, want nil", err)
	}

	canonical := application.NewFingerprint("orders.AddItem").String("o-1").Canonical()
	if got, want := policy.Digest(canonical), ports.Fingerprint(sha256.Sum256(canonical)); got != want {
		t.Fatalf("Digest() = %x, want SHA-256 %x", got, want)
	}
	if policy.Wait != int64(2*time.Second) || policy.Retention != int64(24*time.Hour) {
		t.Fatalf("policy = {Wait: %d, Retention: %d}, want nanoseconds of 2s and 24h", policy.Wait, policy.Retention)
	}
}

func TestTheIdempotencyPolicyRefusesANonPositiveWaitOrRetention(t *testing.T) {
	for name, c := range map[string]struct{ wait, retention time.Duration }{
		"zero wait":          {0, time.Hour},
		"negative wait":      {-time.Second, time.Hour},
		"zero retention":     {time.Second, 0},
		"negative retention": {time.Second, -time.Hour},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := app.IdempotencyPolicy(c.wait, c.retention); !errors.Is(err, app.ErrInvalidIdempotencyPolicy) {
				t.Fatalf("IdempotencyPolicy(%v, %v) = %v, want ErrInvalidIdempotencyPolicy", c.wait, c.retention, err)
			}
		})
	}
}
