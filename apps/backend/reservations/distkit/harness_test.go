//go:build integration && distributed

package distkit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/distkit"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
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

	dist.WaitFor(t, "the consumer to dispose of every delivery", settle, func() bool {
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

	dist.WaitFor(t, "the naive consumer to apply every delivery", settle, func() bool {
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

func TestTheConsumerChildReachesTheDatabaseAsTheRuntimeRole(t *testing.T) {
	h := distkit.New(t)
	consumer := h.Start(t, distkit.RoleConsumer)
	h.Start(t, distkit.RoleProducer).Wait(t, settle)

	dist.WaitFor(t, "the consumer to dispose of every delivery", settle, func() bool {
		return h.Effects(t).Inbox >= 2
	})
	var sessions int
	if err := h.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND usename = $1", pg.RuntimeRole).Scan(&sessions); err != nil {
		t.Fatalf("pg_stat_activity = %v", err)
	}
	consumer.Stop(t, settle)

	if sessions == 0 {
		t.Fatalf("sessions of %s in the database the consumer wrote = 0, want the child connected as the runtime role\nconsumer:\n%s", pg.RuntimeRole, consumer.Output())
	}
}

const reservations = 6

func TestTwoRelaysPublishEveryRecordExactlyOnce(t *testing.T) {
	h := dist.New(t, distkit.RelaySpec)
	enqueue(t, reservations)

	tb.Require(t, dist.RelayVector(t, h, reservations))
}

// enqueue drives the real use case under a message context: the claim of OBX-08
// only sees records whose metadata names correlation, causation and traceparent.
func enqueue(t *testing.T, n int) {
	t.Helper()
	h := appkit.NewReservations(t, clock.New(ports.Instant(1)), &ids.Sequence{Prefix: "m-"})
	ctx := ports.WithMessageContext(context.Background(), ports.MessageContext{
		CorrelationID: "corr-distkit",
		CausationID:   "caus-distkit",
		Traceparent:   "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	})
	for i := range n {
		order := domain.OrderID(fmt.Sprintf("o-distkit-%d", i))
		key := fmt.Sprintf("k-distkit-%d", i)
		if _, err := h.Service.Reserve(execctx.WithKey(t, ctx, key), application.Reserve{Order: order, Items: 1}); err != nil {
			t.Fatalf("Reserve(%s) = %v, want nil", order, err)
		}
	}
}
