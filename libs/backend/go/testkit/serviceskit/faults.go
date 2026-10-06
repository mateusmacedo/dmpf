package serviceskit

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Faults is what a failure-path test injects into a use case's ports: a nil
// field lets that call through. The memory realization never fails on its own,
// so this is how a load, a Save or an Enqueue failure reaches the use case.
type Faults struct {
	Load    error
	Save    error
	Enqueue error
}

func FaultyRepository[ID comparable, S any](inner ports.Repository[ID, S], faults Faults) ports.Repository[ID, S] {
	return faultyRepository[ID, S]{inner: inner, faults: faults}
}

func FaultyOutbox(inner ports.Outbox, faults Faults) ports.Outbox {
	return faultyOutbox{inner: inner, faults: faults}
}

type faultyRepository[ID comparable, S any] struct {
	inner  ports.Repository[ID, S]
	faults Faults
}

func (r faultyRepository[ID, S]) Load(ctx context.Context, id ID) (S, ports.Version, error) {
	if r.faults.Load != nil {
		var zero S
		return zero, 0, r.faults.Load
	}
	return r.inner.Load(ctx, id)
}

func (r faultyRepository[ID, S]) Save(ctx context.Context, id ID, state S, expected ports.Version) error {
	if r.faults.Save != nil {
		return r.faults.Save
	}
	return r.inner.Save(ctx, id, state, expected)
}

type faultyOutbox struct {
	inner  ports.Outbox
	faults Faults
}

func (o faultyOutbox) Enqueue(ctx context.Context, entry ports.OutboxEntry) error {
	if o.faults.Enqueue != nil {
		return o.faults.Enqueue
	}
	return o.inner.Enqueue(ctx, entry)
}
