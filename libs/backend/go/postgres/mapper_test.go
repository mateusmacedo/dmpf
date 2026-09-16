package postgres

import (
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
)

// strayEvent satisfies DomainEvent without any registered contract, which is
// the only way to reach ErrUnmappedEvent from outside a real bounded context.
type strayEvent struct{}

func (strayEvent) EventName() string { return "test.stray" }

// fakeMapper answers with whatever the test arms, so the major check can be
// exercised with a Type the real mapper would never produce.
type fakeMapper struct {
	mapped Mapped
	err    error
}

func (m fakeMapper) Map(domain.DomainEvent) (Mapped, error) {
	if m.err != nil {
		return Mapped{}, m.err
	}
	return m.mapped, nil
}

func TestMapperReportsAnUnmappedEvent(t *testing.T) {
	t.Parallel()

	mapper := fakeMapper{err: ErrUnmappedEvent}

	_, err := mapper.Map(strayEvent{})

	if !errors.Is(err, ErrUnmappedEvent) {
		t.Fatalf("Map() = %v, want ErrUnmappedEvent", err)
	}
}

func TestCheckMajor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		mapped  Mapped
		wantErr bool
	}{
		{
			name:   "v1 contract under a v1 envelope type",
			mapped: Mapped{Message: &eventv1.OrderPlaced{}, Type: "com.company.orders.order-placed.v1"},
		},
		{
			name:    "v1 contract under a v2 envelope type",
			mapped:  Mapped{Message: &eventv1.OrderPlaced{}, Type: "com.company.orders.order-placed.v2"},
			wantErr: true,
		},
		{
			name:    "envelope type without a major",
			mapped:  Mapped{Message: &eventv1.ItemAdded{}, Type: "orders-item-added"},
			wantErr: true,
		},
	}

	t.Run("a mapper that returns no message is reported, not dereferenced", func(t *testing.T) {
		t.Parallel()

		err := checkMajor(Mapped{Type: "com.company.orders.order-placed.v1"})

		if !errors.Is(err, ErrEmptyMapping) {
			t.Fatalf("checkMajor() = %v, want ErrEmptyMapping", err)
		}
	})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := checkMajor(tc.mapped)

			if tc.wantErr {
				if !errors.Is(err, envelope.ErrMajorMismatch) {
					t.Fatalf("checkMajor() = %v, want envelope.ErrMajorMismatch (ENV-16)", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("checkMajor() = %v, want nil", err)
			}
		})
	}
}
