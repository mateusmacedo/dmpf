package orders_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain/example/orders"
)

func TestFromSnapshotRoundTripsTheObservableState(t *testing.T) {
	before := newOpenOrder(t, 2, 3).Snapshot()

	got := orders.FromSnapshot(before).Snapshot()

	if !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestFromSnapshotSharesNoItemsWithTheSnapshot(t *testing.T) {
	source := newOpenOrder(t, 1, 3).Snapshot()

	reconstituted := orders.FromSnapshot(source)

	source.Items[0].Quantity = 999
	if got := reconstituted.Snapshot().Items[0].Quantity; got != 1 {
		t.Fatalf("mutating the source snapshot reached the aggregate: Quantity = %d, want 1", got)
	}

	mutated := reconstituted.Snapshot()
	mutated.Items[0].Quantity = 7
	if got := reconstituted.Snapshot().Items[0].Quantity; got != 1 {
		t.Fatalf("mutating the returned snapshot reached the aggregate: Quantity = %d, want 1", got)
	}
}

func TestFromSnapshotRestoresTheStatusAsBehaviour(t *testing.T) {
	o := newOpenOrder(t, 1, 3)
	if _, rej := o.Place(orders.PlaceOrder{At: at}); rej != nil {
		t.Fatalf("setup: Place rejected: %v", rej)
	}
	placed := o.Snapshot()

	reconstituted := orders.FromSnapshot(placed)

	if got := reconstituted.Snapshot().Status; got != orders.Placed {
		t.Fatalf("Status = %v, want Placed", got)
	}
	acc, rej := reconstituted.AddItem(orders.AddItem{SKU: "ZZZ", Quantity: 1, At: at})
	requireRejected(t, acc, rej, orders.CodeOrderNotOpen)
}

func TestFromSnapshotRestoresTheItemLimitAsBehaviour(t *testing.T) {
	full := newOpenOrder(t, 3, 3).Snapshot()

	reconstituted := orders.FromSnapshot(full)

	acc, rej := reconstituted.AddItem(orders.AddItem{SKU: "ZZZ", Quantity: 1, At: at})
	requireRejected(t, acc, rej, orders.CodeItemLimitExceeded)
}
