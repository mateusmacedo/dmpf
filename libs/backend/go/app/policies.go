package app

import (
	"fmt"
	"time"
)

// Policies are the idempotency and purge values every context fixes at startup.
// InboxRetention only binds a context that consumes a channel.
type Policies struct {
	IdempotencyWait      time.Duration
	IdempotencyRetention time.Duration
	OutboxRetention      time.Duration
	InboxRetention       time.Duration
	PurgeInterval        time.Duration
	PurgeBatch           int
}

// Validate refuses the first non-positive value, naming it under invalid, so
// each context keeps its own sentinel and prefix.
func (p Policies) Validate(consumes bool, invalid error) error {
	type duration struct {
		name  string
		value time.Duration
	}
	durations := []duration{
		{"IdempotencyWait", p.IdempotencyWait},
		{"IdempotencyRetention", p.IdempotencyRetention},
		{"OutboxRetention", p.OutboxRetention},
	}
	if consumes {
		durations = append(durations, duration{"InboxRetention", p.InboxRetention})
	}
	durations = append(durations, duration{"PurgeInterval", p.PurgeInterval})
	for _, d := range durations {
		if d.value <= 0 {
			return fmt.Errorf("%w: %s %v", invalid, d.name, d.value)
		}
	}
	if p.PurgeBatch <= 0 {
		return fmt.Errorf("%w: PurgeBatch %d", invalid, p.PurgeBatch)
	}
	return nil
}
