package ports

import (
	"context"
	"sync"
)

type externalClassSlotKey struct{}

type externalClassSlot struct {
	mu    sync.Mutex
	class ExternalClass
}

// WithExternalClassSlot installs the mutable slot the instrumentation reads
// after the use case returns: the class is marked downstream, on a derived context.
func WithExternalClassSlot(ctx context.Context) context.Context {
	return context.WithValue(ctx, externalClassSlotKey{}, &externalClassSlot{})
}

func MarkExternalClass(ctx context.Context, class ExternalClass) {
	slot, ok := ctx.Value(externalClassSlotKey{}).(*externalClassSlot)
	if !ok {
		return
	}
	slot.mu.Lock()
	slot.class = class
	slot.mu.Unlock()
}

func ExternalClassFrom(ctx context.Context) (ExternalClass, bool) {
	slot, ok := ctx.Value(externalClassSlotKey{}).(*externalClassSlot)
	if !ok {
		return 0, false
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	return slot.class, slot.class != 0
}

type externalConflictSlotKey struct{}

type externalConflictSlot struct {
	mu       sync.Mutex
	conflict bool
}

// WithExternalConflictSlot installs the slot a use case marks when the ledger
// recorded a conflict, so the instrumentation reports it after the commit kept it.
func WithExternalConflictSlot(ctx context.Context) context.Context {
	return context.WithValue(ctx, externalConflictSlotKey{}, &externalConflictSlot{})
}

func MarkExternalConflict(ctx context.Context) {
	slot, ok := ctx.Value(externalConflictSlotKey{}).(*externalConflictSlot)
	if !ok {
		return
	}
	slot.mu.Lock()
	slot.conflict = true
	slot.mu.Unlock()
}

func ExternalConflictFrom(ctx context.Context) bool {
	slot, ok := ctx.Value(externalConflictSlotKey{}).(*externalConflictSlot)
	if !ok {
		return false
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	return slot.conflict
}

type externalSaturationSlotKey struct{}

type externalSaturationSlot struct {
	mu        sync.Mutex
	saturated bool
}

func WithExternalSaturationSlot(ctx context.Context) context.Context {
	return context.WithValue(ctx, externalSaturationSlotKey{}, &externalSaturationSlot{})
}

func MarkExternalSaturation(ctx context.Context) {
	slot, ok := ctx.Value(externalSaturationSlotKey{}).(*externalSaturationSlot)
	if !ok {
		return
	}
	slot.mu.Lock()
	slot.saturated = true
	slot.mu.Unlock()
}

func ExternalSaturationFrom(ctx context.Context) bool {
	slot, ok := ctx.Value(externalSaturationSlotKey{}).(*externalSaturationSlot)
	if !ok {
		return false
	}
	slot.mu.Lock()
	defer slot.mu.Unlock()
	return slot.saturated
}
