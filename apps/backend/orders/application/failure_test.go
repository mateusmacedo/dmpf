package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/orders/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var errBoom = errors.New("boom")

func TestEachFailureOfACommandCarriesTheUseCasePrefix(t *testing.T) {
	addItem := func(t *testing.T, h *harness) error {
		_, err := h.service.AddItem(withExecution(t, context.Background()), application.AddItem{Order: orderID, SKU: "ABC", Quantity: 1})
		return err
	}
	placeOrder := func(t *testing.T, h *harness) error {
		_, err := h.service.PlaceOrder(withExecution(t, context.Background()), application.PlaceOrder{Order: orderID})
		return err
	}
	for _, tc := range []struct {
		name    string
		options []option
		seeded  bool
		run     func(*testing.T, *harness) error
		wantIs  error
		wantMsg string
	}{
		{"AddItem, load", []option{withLoadError(errBoom)}, false, addItem, errBoom, "application: add item to P-100: boom"},
		{"AddItem, save", []option{withSaveError(errBoom)}, false, addItem, errBoom, "application: add item to P-100: boom"},
		{"AddItem, enqueue", []option{withEnqueueError(errBoom)}, false, addItem, errBoom, "application: add item to P-100: enqueue: boom"},
		{"PlaceOrder, absent", nil, false, placeOrder, ports.ErrNotFound, "application: place order P-100: ports: aggregate not found"},
		{"PlaceOrder, load", []option{withLoadError(errBoom)}, true, placeOrder, errBoom, "application: place order P-100: boom"},
		{"PlaceOrder, save", []option{withSaveError(errBoom)}, true, placeOrder, errBoom, "application: place order P-100: boom"},
		{"PlaceOrder, enqueue", []option{withEnqueueError(errBoom)}, true, placeOrder, errBoom, "application: place order P-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t, tc.options...)
			if tc.seeded {
				h.seed(t, openSnapshot(1), 0)
			}

			err := tc.run(t, h)

			if !errors.Is(err, tc.wantIs) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q matching %v", err, tc.wantMsg, tc.wantIs)
			}
		})
	}
}
