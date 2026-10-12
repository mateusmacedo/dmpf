//go:build integration && distributed

package distkit_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/appkit"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/distkit"
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"
)

// orders is how many decisions the harness enqueues before the drains start,
// so both have something to compete for.
const orders = 6

// TestDistkitRole is the entry point of a re-executed child; it is inert in a
// normal run.
func TestDistkitRole(t *testing.T) { distkit.RunRole(t) }

func TestTwoDrainsPublishEveryRecordExactlyOnce(t *testing.T) {
	h := dist.New(t, distkit.Spec)
	enqueue(t, orders)

	tb.Require(t, dist.RelayVector(t, h, orders))
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
		if _, err := h.Service.AddItem(withExecution(t, ctx), application.AddItem{Order: order, SKU: "sku-1", Quantity: 1}); err != nil {
			t.Fatalf("AddItem(%s) = %v, want nil", order, err)
		}
	}
}
