package reservationspg_test

import (
	"errors"
	"testing"

	dmpfdomain "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-domain/example/reservations"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
	reservationspg "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres/example/reservations"
)

func TestMapperMapsReservationConfirmed(t *testing.T) {
	m := reservationspg.Mapper{}
	mapped, err := m.Map(reservations.ReservationConfirmed{Order: "o-1", Items: 3, At: 100})
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

type unknownEvent struct{}

func (unknownEvent) EventName() string { return "unknown" }

var _ dmpfdomain.DomainEvent = unknownEvent{}

func TestMapperRejectsUnknownEvent(t *testing.T) {
	m := reservationspg.Mapper{}
	_, err := m.Map(unknownEvent{})
	if !errors.Is(err, dmpfpostgres.ErrUnmappedEvent) {
		t.Fatalf("Map(unknown) = %v, want ErrUnmappedEvent", err)
	}
}
