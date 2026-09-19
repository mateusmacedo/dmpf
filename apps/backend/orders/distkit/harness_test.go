//go:build integration && distributed

package distkit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/distkit"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
)

// orders is how many decisions the harness enqueues before the drains start,
// so both have something to compete for.
const orders = 6

// TestDistkitRole is the entry point of a re-executed child; it is inert in a
// normal run.
func TestDistkitRole(t *testing.T) { distkit.RunRole(t) }

func TestTwoDrainsPublishEveryRecordExactlyOnce(t *testing.T) {
	h := distkit.New(t)
	enqueue(t, orders)

	var pending int
	if err := h.Pool.QueryRow(context.Background(),
		"SELECT count(*) FROM dmpf_outbox WHERE status = 'pending'").Scan(&pending); err != nil {
		t.Fatalf("counting the outbox: %v", err)
	}
	if pending != orders {
		t.Fatalf("outbox holds %d pending records before the drains start, want %d: the vector needs something to compete for", pending, orders)
	}

	relays := make([]*distkit.Process, distkit.Relays)
	for i := range relays {
		relays[i] = h.Start(t, distkit.RoleRelay)
	}
	t.Cleanup(func() {
		if t.Failed() {
			rows, err := h.Pool.Query(context.Background(),
				"SELECT message_id, status, attempt_count, coalesce(last_error, '') FROM dmpf_outbox ORDER BY id")
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var id, status, lastErr string
					var attempts int
					_ = rows.Scan(&id, &status, &attempts, &lastErr)
					t.Logf("outbox %s: status=%s attempts=%d last_error=%q", id, status, attempts, lastErr)
				}
			}
			for i, r := range relays {
				t.Logf("relay %d wrote: %s", i, r.Output())
			}
		}
	})
	settled := h.Settled(t, orders, 30*time.Second)
	published := h.Collect(t, len(settled), 60*time.Second)
	for _, r := range relays {
		r.Stop(t, 30*time.Second)
	}

	if verdict := distkit.Decide(settled, published); !verdict.OK() {
		t.Fatalf("two drains over one outbox: %v", verdict.Failures())
	}
}

// enqueue drives the real use case, so what the drains carry is what the
// application authored and not a row a test wrote by hand.
//
// The context carries a message context because the claim of OBX-08 only sees
// records whose metadata names correlation, causation and traceparent: a
// record authored outside a message is not drainable by design.
func enqueue(t *testing.T, n int) {
	t.Helper()
	h := appkit.NewOrders(t, clock.New(ports.Instant(1)), &ids.Sequence{Prefix: "m-"})
	ctx := ports.WithMessageContext(context.Background(), ports.MessageContext{
		CorrelationID: "corr-distkit",
		CausationID:   "caus-distkit",
		Traceparent:   "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	})
	for i := range n {
		order := domain.OrderID(fmt.Sprintf("o-distkit-%d", i))
		if _, err := h.Service.AddItem(ctx, application.AddItem{Order: order, SKU: "sku-1", Quantity: 1}); err != nil {
			t.Fatalf("AddItem(%s) = %v, want nil", order, err)
		}
	}
}
