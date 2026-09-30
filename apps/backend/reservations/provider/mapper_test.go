package provider_test

import (
	"errors"
	"testing"

	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/provider"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

func TestMapperMapsReservationConfirmed(t *testing.T) {
	m := provider.Mapper{}
	mapped, err := m.Map(domain.ReservationConfirmed{Order: "o-1", Items: 3, At: 100})
	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	if mapped.Type != "com.company.reservations.reservation-confirmed.v1" {
		t.Errorf("Type = %q, want %q", mapped.Type, "com.company.reservations.reservation-confirmed.v1")
	}
	if mapped.Message == nil {
		t.Fatal("Message is nil")
	}
}

func TestMapperMapsReservationCancelled(t *testing.T) {
	m := provider.Mapper{}
	mapped, err := m.Map(domain.ReservationCancelled{Order: "o-1", At: 100})
	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	if mapped.Type != "com.company.reservations.reservation-cancelled.v1" {
		t.Errorf("Type = %q, want %q", mapped.Type, "com.company.reservations.reservation-cancelled.v1")
	}
	if mapped.Message == nil {
		t.Fatal("Message is nil")
	}
}

type unknownEvent struct{}

func (unknownEvent) EventName() string { return "unknown" }

var _ kernel.DomainEvent = unknownEvent{}

func TestMapperRejectsUnknownEvent(t *testing.T) {
	m := provider.Mapper{}
	_, err := m.Map(unknownEvent{})
	if !errors.Is(err, postgres.ErrUnmappedEvent) {
		t.Fatalf("Map(unknown) = %v, want ErrUnmappedEvent", err)
	}
}
