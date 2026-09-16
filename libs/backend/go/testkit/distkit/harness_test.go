//go:build integration && distributed

package distkit_test

import (
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/distkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// TestDistkitRole is the body of every child process; it skips in the parent.
func TestDistkitRole(t *testing.T) { distkit.RunRole(t) }

const settle = 90 * time.Second

// V32 positive: the DMPF consumer sees evt-1 twice and evt-9 for the same
// order, and the effect is the one of a single delivery (RAS-13).
func TestV32RedeliveryDoesNotDuplicateTheEffect(t *testing.T) {
	h := distkit.New(t)
	consumer := h.Start(t, distkit.RoleConsumer)
	h.Start(t, distkit.RoleProducer).Wait(t, settle)

	distkit.WaitFor(t, "the consumer to dispose of every delivery", settle, func() bool {
		return h.Effects(t).Inbox >= 2
	})
	consumer.Stop(t, settle)

	effects := h.Effects(t)
	tb.Require(t, distkit.Decide(effects, h.Plan))
	if effects != (distkit.Effects{Inbox: 2, Reservations: 1, Outbox: 1, Items: 2, Version: 1}) {
		t.Fatalf("effects = %+v\nconsumer:\n%s", effects, consumer.Output())
	}
}

// V32 negative: a consumer that applies the effect on every delivery is
// reproved with DMPF-R004, naming the redelivered identity.
func TestV32NaiveConsumerIsReprovedWithR004(t *testing.T) {
	h := distkit.New(t)
	consumer := h.Start(t, distkit.RoleNaiveConsumer)
	h.Start(t, distkit.RoleProducer).Wait(t, settle)

	distkit.WaitFor(t, "the naive consumer to apply every delivery", settle, func() bool {
		return h.Effects(t).Version >= int64(len(h.Plan.Deliveries))
	})
	consumer.Stop(t, settle)

	v := distkit.Decide(h.Effects(t), h.Plan)
	if v.OK() {
		t.Fatalf("a consumer that repeats the effect passed; effects = %+v", h.Effects(t))
	}
	if v.Diagnostics[0].Code != distkit.CodeR004 {
		t.Fatalf("diagnostic = %+v, want DMPF-R004", v.Diagnostics[0])
	}
}
