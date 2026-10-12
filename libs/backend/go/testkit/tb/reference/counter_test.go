package reference_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/reference"
)

func service(store *memory.Store) reference.CounterService {
	return reference.CounterService{
		UoW: memory.NewUnitOfWork(store, func(tx *memory.Tx) reference.CounterResources {
			return reference.CounterResources{Counters: reference.CountersTable.Repository(tx), Outbox: tx.Outbox()}
		}),
		Reader:    reference.CountersTable.Reader(store),
		Clock:     clock.New(ports.Instant(1_755_432_000_000_000_000)),
		IDs:       &ids.Sequence{Prefix: "m-"},
		Authorize: application.AllowAll[reference.BumpCounter](),
		Limit:     2,
	}
}

func TestTheServiceRunsWithoutInstrumentation(t *testing.T) {
	ctx := execctx.WithExecution(t, context.Background())
	svc := service(memory.New())

	accepted, err := svc.Bump(ctx, reference.BumpCounter{Counter: "c-1", By: 2})
	if err != nil {
		t.Fatalf("Bump() = %v, want nil", err)
	}
	if accepted.Response().Total != 2 {
		t.Fatalf("total = %d, want 2", accepted.Response().Total)
	}
	refused, err := svc.Bump(ctx, reference.BumpCounter{Counter: "c-1", By: 1})
	if rejection, declined := refused.Rejection(); err != nil || !declined || rejection.Code() != reference.CodeLimitExceeded {
		t.Fatalf("Bump() over the limit = %v, %v; want the limit rejection", rejection, err)
	}
	if state, err := svc.Find(ctx, "c-1"); err != nil || state.Total != 2 {
		t.Fatalf("Find() = %+v, %v; want the committed total", state, err)
	}
}
