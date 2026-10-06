package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// Loader is step 4 of FND-04 §3.2: it turns what the reader answers into the
// aggregate to decide over and the version Save will expect.
type Loader[ID comparable, S, A any] func(ctx context.Context, reader ports.Reader[ID, S], id ID) (A, ports.Version, error)

// OrNew starts a fresh aggregate at version zero when the reader finds none,
// including when the identifier belongs to another tenant (IDN-13).
func OrNew[ID comparable, S, A any](fresh func(ID) A, from func(S) A) Loader[ID, S, A] {
	return func(ctx context.Context, reader ports.Reader[ID, S], id ID) (A, ports.Version, error) {
		snapshot, stored, err := reader.Load(ctx, id)
		switch {
		case errors.Is(err, ports.ErrNotFound):
			return fresh(id), 0, nil
		case err != nil:
			var zero A
			return zero, 0, err
		default:
			return from(snapshot), stored, nil
		}
	}
}

func Existing[ID comparable, S, A any](from func(S) A) Loader[ID, S, A] {
	return func(ctx context.Context, reader ports.Reader[ID, S], id ID) (A, ports.Version, error) {
		var zero A
		snapshot, stored, err := reader.Load(ctx, id)
		if err != nil {
			return zero, 0, err
		}
		return from(snapshot), stored, nil
	}
}

// Absent creates the aggregate and refuses with ErrAlreadyExists when it exists.
// S comes first so a caller names it alone and lets ID and A be inferred.
func Absent[S any, ID comparable, A any](fresh func(ID) A) Loader[ID, S, A] {
	return func(ctx context.Context, reader ports.Reader[ID, S], id ID) (A, ports.Version, error) {
		var zero A
		switch _, _, err := reader.Load(ctx, id); {
		case err == nil:
			return zero, 0, ports.ErrAlreadyExists
		case !errors.Is(err, ports.ErrNotFound):
			return zero, 0, err
		}
		return fresh(id), 0, nil
	}
}

// Decide is steps 4 to 7 of FND-04 §3.2. A refusal writes nothing and is no
// error (DEC-04); load and Save failures come back unchanged and an Enqueue
// failure as "enqueue: <cause>", so the use case owns the prefix of each.
func Decide[ID comparable, S, T any, A interface {
	*T
	Snapshot() S
}, R any](
	ctx context.Context,
	repo ports.Repository[ID, S],
	outbox ports.Outbox,
	origin Origin,
	id ID,
	identity Identity,
	load Loader[ID, S, A],
	decide func(A) (domain.Accepted[R], *domain.Rejection),
) (Outcome[R], error) {
	var zero Outcome[R]

	aggregate, stored, err := load(ctx, repo, id)
	if err != nil {
		return zero, err
	}

	accepted, rejection := decide(aggregate)
	if rejection != nil {
		return Rejected[R](rejection), nil
	}

	if err := repo.Save(ctx, id, aggregate.Snapshot(), stored); err != nil {
		return zero, err
	}
	if err := Enqueue(ctx, outbox, identity, origin, stored+1, accepted.Events()); err != nil {
		return zero, fmt.Errorf("enqueue: %w", err)
	}
	return Accepted(accepted.Response()), nil
}
