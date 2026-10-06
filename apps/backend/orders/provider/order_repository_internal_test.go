package provider

import (
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
)

func TestTheOrderSnapshotKeepsTheBytesItIsStoredWith(t *testing.T) {
	cases := map[string]struct {
		snapshot domain.Snapshot
		want     string
	}{
		"no items": {domain.Snapshot{ID: "o-1", ItemLimit: 3}, `{"status":0,"itemLimit":3,"items":[]}`},
		"one item": {
			domain.Snapshot{ID: "o-1", Status: domain.Status(1), ItemLimit: 3, Items: []domain.Item{{SKU: "sku-1", Quantity: 2}}},
			`{"status":1,"itemLimit":3,"items":[{"sku":"sku-1","quantity":2}]}`,
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			values, err := orderTable.Encode(c.snapshot)
			if err != nil {
				t.Fatalf("Encode() = %v, want nil", err)
			}
			if got := string(values[0].([]byte)); got != c.want {
				t.Fatalf("snapshot column = %s, want %s", got, c.want)
			}
		})
	}
}

func TestAnOrderStoredWithoutItemsDecodesWithNoItems(t *testing.T) {
	raw := []byte(`{"status":0,"itemLimit":3,"items":[]}`)

	snapshot, err := orderTable.Decode(func(dest ...any) error {
		*dest[0].(*[]byte) = raw
		return nil
	})

	if err != nil {
		t.Fatalf("Decode() = %v, want nil", err)
	}
	if snapshot.Items != nil || snapshot.ItemLimit != 3 {
		t.Fatalf("snapshot = %+v, want ItemLimit 3 and nil Items", snapshot)
	}
}
