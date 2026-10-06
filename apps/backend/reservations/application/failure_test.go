package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/application"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var errBoom = errors.New("boom")

func TestEachFailureOfACommandCarriesTheUseCasePrefix(t *testing.T) {
	reserve := func(t *testing.T, h *syncHarness) error {
		_, err := h.service.Reserve(withExecution(t, context.Background()), application.Reserve{Order: syncOrder, Items: 1})
		return err
	}
	cancel := func(t *testing.T, h *syncHarness) error {
		_, err := h.service.Cancel(withExecution(t, context.Background()), application.Cancel{Order: syncOrder})
		return err
	}
	consume := func(t *testing.T, h *syncHarness) error {
		_, err := h.service.ConsumeOrderPlaced(withExecution(t, context.Background()), consumeOrderPlaced("m-placed", "h-1", syncOrder, 1))
		return err
	}
	for _, tc := range []struct {
		name    string
		option  syncOption
		run     func(*testing.T, *syncHarness) error
		wantIs  error
		wantMsg string
	}{
		{"Reserve, load", withSyncLoadError(errBoom), reserve, errBoom, "application: reserve P-100: boom"},
		{"Reserve, save", withSyncSaveError(errBoom), reserve, errBoom, "application: reserve P-100: boom"},
		{"Reserve, enqueue", withSyncEnqueueError(errBoom), reserve, errBoom, "application: reserve P-100: enqueue: boom"},
		{"Cancel, load", withSyncLoadError(errBoom), cancel, errBoom, "application: cancel P-100: boom"},
		{"Cancel, save", withSyncSaveError(errBoom), cancel, errBoom, "application: cancel P-100: boom"},
		{"Cancel, enqueue", withSyncEnqueueError(errBoom), cancel, errBoom, "application: cancel P-100: enqueue: boom"},
		{"Consume, load", withSyncLoadError(errBoom), consume, errBoom, "application: consume P-100: boom"},
		{"Consume, save", withSyncSaveError(errBoom), consume, errBoom, "application: consume P-100: boom"},
		{"Consume, enqueue", withSyncEnqueueError(errBoom), consume, errBoom, "application: consume P-100: enqueue: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newSyncHarness(t, tc.option)

			err := tc.run(t, h)

			if !errors.Is(err, tc.wantIs) || err.Error() != tc.wantMsg {
				t.Fatalf("err = %v, want %q matching %v", err, tc.wantMsg, tc.wantIs)
			}
		})
	}
}

func TestAConsumeConflictIsARetryableFailureInsideThePrefix(t *testing.T) {
	h := newSyncHarness(t, withSyncSaveError(ports.ErrVersionConflict))

	disposition, err := h.service.ConsumeOrderPlaced(withExecution(t, context.Background()), consumeOrderPlaced("m-placed", "h-1", syncOrder, 1))

	if err == nil || err.Error() != "application: consume P-100: Conflict: ports: version conflict" {
		t.Fatalf("err = %v, want the conflict inside the consume prefix", err)
	}
	var failure *usecase.Failure
	if !errors.As(err, &failure) || failure.Category() != usecase.Conflict || !errors.Is(err, ports.ErrVersionConflict) {
		t.Fatalf("err = %v, want a Conflict failure over ErrVersionConflict", err)
	}
	if disposition != usecase.R1D3 || usecase.Classify(err) != usecase.R1D3 {
		t.Fatalf("disposition = %v, Classify = %v; want R1D3", disposition, usecase.Classify(err))
	}
}
