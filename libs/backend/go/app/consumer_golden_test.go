package app_test

import (
	"context"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
)

const orderPlacedFixture = "../../../../contracts/fixtures/orders/event/v1/order-placed.golden"

// The canonical fixtures carry tenantid in both states (TestFixtureShape
// requires it), so running every case through the adapter proves the tenant
// reaches the rebuilt context when present and stays absent otherwise (CTX-24,
// CTX-26) on the committed contract, not on a fixture written for this test.
func TestEveryCanonicalCaseRebuildsTheTenantItCarries(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(orderPlacedFixture)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	fixture, err := golden.Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	var present, absent int
	for _, c := range fixture.Cases {
		t.Run(c.Name, func(t *testing.T) {
			handler := &fakeHandler{disposition: application.R1D1}
			consumer := newConsumer(handler, &fakeContainment{}, 3)
			consumer.Boundary.Sources = []string{c.Envelope["source"]}

			if _, err := consumer.Consume(context.Background(), app.Delivery{Raw: fixtureRaw(t, c), Attempt: 1}, &fakeAck{}); err != nil {
				t.Fatalf("Consume() = %v", err)
			}
			tenant, scoped := handler.execution.Tenant()
			want, carried := c.Envelope["tenantid"]
			switch {
			case carried && (!scoped || string(tenant) != want):
				t.Fatalf("tenant = %q, %v; want %q from the envelope", tenant, scoped, want)
			case !carried && scoped:
				t.Fatalf("tenant = %q; the envelope carried none, and no default takes its place (CTX-26)", tenant)
			}
		})
		if _, ok := c.Envelope["tenantid"]; ok {
			present++
		} else {
			absent++
		}
	}
	if present == 0 || absent == 0 {
		t.Fatalf("the fixture exercised tenantid present %d and absent %d times; both states are required", present, absent)
	}
}

func fixtureRaw(t *testing.T, c golden.Case) []byte {
	t.Helper()
	payload, err := hex.DecodeString(c.PayloadBytesHex)
	if err != nil {
		t.Fatalf("payload_bytes_hex: %v", err)
	}
	at, err := time.Parse(time.RFC3339Nano, c.Envelope["time"])
	if err != nil {
		t.Fatalf("time: %v", err)
	}
	env := envelope.Envelope{
		ID:              c.Envelope["id"],
		Source:          c.Envelope["source"],
		SpecVersion:     c.Envelope["specversion"],
		Type:            c.Envelope["type"],
		Subject:         c.Envelope["subject"],
		Time:            timestamppb.New(at),
		DataSchema:      c.Envelope["dataschema"],
		DataContentType: c.Envelope["datacontenttype"],
		CorrelationID:   c.Envelope["correlationid"],
		CausationID:     c.Envelope["causationid"],
		PartitionKey:    c.Envelope["partitionkey"],
		TraceParent:     c.Envelope["traceparent"],
		Payload:         payload,
	}
	if tenant, ok := c.Envelope["tenantid"]; ok {
		env.TenantID = &tenant
	}
	return encode(t, env)
}
