package golden

import (
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventv1 "github.com/mateusmacedo/dmpf/apps/backend/bookings/contract/gen/go/company/bookings/event/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

var resourceRegisteredSpec = tb.Spec[proto.Message]{
	Path:    "apps/backend/bookings/contract/fixtures/event/v1/resource-registered.golden",
	Source:  "urn:dmpf:reference-bookings",
	Subject: "room-1",
	Identity: golden.Identity{
		Fixture:    "bookings/event/v1/resource-registered",
		Contract:   golden.Contract{Package: "company.bookings.event.v1", Message: "ResourceRegistered"},
		Type:       "com.company.bookings.resource-registered.v1",
		DataSchema: "type.googleapis.com/company.bookings.event.v1.ResourceRegistered",
	},
	Unknown:    7,
	New:        func() proto.Message { return &eventv1.ResourceRegistered{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return resourceRegistered(fields) },
	Build: leanFixture(
		resourceRegisteredFields("room-1", "2026-09-02T12:00:00Z"),
		resourceRegisteredFields("room-2", "2026-09-02T12:00:01Z"),
		resourceRegisteredFields("room-3", "2026-09-02T12:00:02Z"),
		resourceRegisteredFields("room-4", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func resourceRegisteredFields(resourceID, registeredAt string) map[string]string {
	return map[string]string{
		"resource_id":   resourceID,
		"registered_at": registeredAt,
	}
}

func resourceRegistered(fields map[string]string) (*eventv1.ResourceRegistered, error) {
	registeredAt, err := time.Parse(time.RFC3339Nano, fields["registered_at"])
	if err != nil {
		return nil, fmt.Errorf("registered_at %q: %w", fields["registered_at"], err)
	}
	return &eventv1.ResourceRegistered{
		ResourceId:   fields["resource_id"],
		RegisteredAt: timestamppb.New(registeredAt),
	}, nil
}
