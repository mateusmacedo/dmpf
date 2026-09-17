package domain_test

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
)

func TestFromSnapshotRoundTripsTheObservableState(t *testing.T) {
	before := newOpenOrder(t, 2, 3).Snapshot()

	got := domain.FromSnapshot(before).Snapshot()

	if !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestFromSnapshotSharesNoItemsWithTheSnapshot(t *testing.T) {
	source := newOpenOrder(t, 1, 3).Snapshot()

	reconstituted := domain.FromSnapshot(source)

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
	if _, rej := o.Place(domain.PlaceOrder{At: at}); rej != nil {
		t.Fatalf("setup: Place rejected: %v", rej)
	}
	placed := o.Snapshot()

	reconstituted := domain.FromSnapshot(placed)

	if got := reconstituted.Snapshot().Status; got != domain.Placed {
		t.Fatalf("Status = %v, want Placed", got)
	}
	acc, rej := reconstituted.AddItem(domain.AddItem{SKU: "ZZZ", Quantity: 1, At: at})
	requireRejected(t, acc, rej, domain.CodeOrderNotOpen)
}

func TestFromSnapshotRestoresTheItemLimitAsBehaviour(t *testing.T) {
	full := newOpenOrder(t, 3, 3).Snapshot()

	reconstituted := domain.FromSnapshot(full)

	acc, rej := reconstituted.AddItem(domain.AddItem{SKU: "ZZZ", Quantity: 1, At: at})
	requireRejected(t, acc, rej, domain.CodeItemLimitExceeded)
}
