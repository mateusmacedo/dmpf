package app_test

import (
	"errors"
	"testing"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
)

var errInvalid = errors.New("ctx: invalid idempotency or purge policy")

func validPolicies() kernelapp.Policies {
	return kernelapp.Policies{
		IdempotencyWait:      time.Second,
		IdempotencyRetention: 24 * time.Hour,
		OutboxRetention:      168 * time.Hour,
		InboxRetention:       192 * time.Hour,
		PurgeInterval:        15 * time.Minute,
		PurgeBatch:           1000,
	}
}

func TestPoliciesAcceptPositiveValues(t *testing.T) {
	for _, consumes := range []bool{false, true} {
		if err := validPolicies().Validate(consumes, errInvalid); err != nil {
			t.Fatalf("Validate(%v) = %v, want nil", consumes, err)
		}
	}
}

func TestPoliciesRefuseANonPositiveValue(t *testing.T) {
	for name, tc := range map[string]struct {
		spoil func(*kernelapp.Policies)
		want  string
	}{
		"wait":             {func(p *kernelapp.Policies) { p.IdempotencyWait = 0 }, "ctx: invalid idempotency or purge policy: IdempotencyWait 0s"},
		"retention":        {func(p *kernelapp.Policies) { p.IdempotencyRetention = -time.Hour }, "ctx: invalid idempotency or purge policy: IdempotencyRetention -1h0m0s"},
		"outbox retention": {func(p *kernelapp.Policies) { p.OutboxRetention = 0 }, "ctx: invalid idempotency or purge policy: OutboxRetention 0s"},
		"inbox retention":  {func(p *kernelapp.Policies) { p.InboxRetention = 0 }, "ctx: invalid idempotency or purge policy: InboxRetention 0s"},
		"purge interval":   {func(p *kernelapp.Policies) { p.PurgeInterval = 0 }, "ctx: invalid idempotency or purge policy: PurgeInterval 0s"},
		"purge batch":      {func(p *kernelapp.Policies) { p.PurgeBatch = 0 }, "ctx: invalid idempotency or purge policy: PurgeBatch 0"},
	} {
		t.Run(name, func(t *testing.T) {
			p := validPolicies()
			tc.spoil(&p)

			if err := p.Validate(true, errInvalid); !errors.Is(err, errInvalid) || err.Error() != tc.want {
				t.Fatalf("Validate(true) = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestPoliciesIgnoreTheInboxRetentionOfAContextThatConsumesNothing(t *testing.T) {
	p := validPolicies()
	p.InboxRetention = 0

	if err := p.Validate(false, errInvalid); err != nil {
		t.Fatalf("Validate(false) = %v, want nil", err)
	}
}

func TestPoliciesNameTheInboxRetentionBeforeThePurgeInterval(t *testing.T) {
	p := validPolicies()
	p.InboxRetention, p.PurgeInterval = 0, 0

	if err := p.Validate(true, errInvalid); err == nil || err.Error() != "ctx: invalid idempotency or purge policy: InboxRetention 0s" {
		t.Fatalf("Validate(true) = %v, want InboxRetention named first", err)
	}
}
