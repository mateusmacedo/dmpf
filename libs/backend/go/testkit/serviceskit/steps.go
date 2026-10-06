package serviceskit

import (
	"context"
	"slices"
	"sync"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Steps is the ordered log of the ports a use case touched, by the names its
// sequence test asserts; the instrumentation double of a context writes into
// the same log, so begin, end and audit fall in their place.
type Steps struct {
	mu       sync.Mutex
	observed []string
}

func (s *Steps) Record(step string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observed = append(s.observed, step)
}

func (s *Steps) Observed() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.observed)
}

func (s *Steps) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observed = nil
}

func (f *Fakes) Clock(inner ports.Clock) ports.Clock {
	return recordingClock{inner: inner, steps: f.Steps}
}

func (f *Fakes) IDs(inner ports.IDGenerator) ports.IDGenerator {
	return recordingIDs{inner: inner, steps: f.Steps}
}

func Authorize[C any](f *Fakes, inner func(context.Context, C) error) func(context.Context, C) error {
	return func(ctx context.Context, cmd C) error {
		f.Steps.Record("authorize")
		return inner(ctx, cmd)
	}
}

// FoldDigest stands in for the SHA-256 of the idempotency policy: it keeps a
// test free of crypto while every byte of the canonical form still counts.
func FoldDigest(canonical []byte) ports.Fingerprint {
	var fingerprint ports.Fingerprint
	for i, b := range canonical {
		fingerprint[i%len(fingerprint)] = fingerprint[i%len(fingerprint)]*31 + b
	}
	return fingerprint
}

type recordingClock struct {
	inner ports.Clock
	steps *Steps
}

func (c recordingClock) Now() ports.Instant {
	c.steps.Record("clock.Now")
	return c.inner.Now()
}

type recordingIDs struct {
	inner ports.IDGenerator
	steps *Steps
}

func (g recordingIDs) NewMessageID() ports.MessageID {
	g.steps.Record("ids.NewMessageID")
	return g.inner.NewMessageID()
}
