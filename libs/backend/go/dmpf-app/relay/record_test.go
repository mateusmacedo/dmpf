package relay

import (
	"errors"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	eventv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/gen/go/company/orders/event/v1"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/payloadhash"
)

const (
	testSource      = "urn:lidercap:orders"
	testCorrelation = "corr-1"
	testCausation   = "caus-1"
	testTraceParent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
)

func TestAssembleMapsTheOutboxColumnsOntoTheEnvelope(t *testing.T) {
	t.Parallel()

	record := publishableRecord(t)

	env, err := Assemble(record, testSource)
	if err != nil {
		t.Fatalf("Assemble() = %v, want nil", err)
	}

	cases := []struct{ attribute, got, want string }{
		{"id", env.ID, record.MessageID},
		{"type", env.Type, record.MessageType},
		{"dataschema", env.DataSchema, record.SchemaVersion},
		{"subject", env.Subject, record.AggregateID},
		{"partitionkey", env.PartitionKey, record.PartitionKey},
		{"source", env.Source, testSource},
		{"specversion", env.SpecVersion, envelope.SpecVersion},
		{"datacontenttype", env.DataContentType, envelope.ContentType},
		{"correlationid", env.CorrelationID, testCorrelation},
		{"causationid", env.CausationID, testCausation},
		{"traceparent", env.TraceParent, testTraceParent},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.attribute, c.got, c.want)
		}
	}

	if env.Time == nil || env.Time.AsTime().UnixNano() != int64(record.OccurredAt) {
		t.Errorf("time = %v, want %d", env.Time, int64(record.OccurredAt))
	}
	if env.AggregateVersion == nil || *env.AggregateVersion != int32(record.AggregateVersion) {
		t.Errorf("aggregateversion = %v, want %d", env.AggregateVersion, record.AggregateVersion)
	}
	if !proto.Equal(decoded(t, env), &eventv1.OrderPlaced{OrderId: "o-1", ItemCount: 3}) {
		t.Errorf("payload does not decode back to the event that was frozen on write")
	}

	if err := env.Validate(); err != nil {
		t.Fatalf("assembled envelope does not satisfy the profile: %v", err)
	}
}

// ENV-11: an envelope attribute naming a topic, queue or integration flow
// couples the contract to the transport. destination routes, it never travels.
func TestAssembleLeavesTheDestinationOutOfTheEnvelope(t *testing.T) {
	t.Parallel()

	record := publishableRecord(t)
	record.Destination = "a-very-recognizable-destination"

	env, err := Assemble(record, testSource)
	if err != nil {
		t.Fatalf("Assemble() = %v, want nil", err)
	}

	ce, err := envelope.Encode(env)
	if err != nil {
		t.Fatalf("Encode() = %v, want nil", err)
	}
	for name, value := range ce.GetAttributes() {
		if strings.Contains(value.String(), record.Destination) {
			t.Fatalf("attribute %q carries the destination: %v", name, value)
		}
	}
	if ce.GetSource() == record.Destination {
		t.Fatal("source carries the destination")
	}
}

// aggregateversion is ce_integer, which is int32. A bigint beyond that range
// would wrap into a negative version and publish a lie; it is terminal instead.
func TestAssembleRejectsAnAggregateVersionOutsideInt32(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		version int64
		wantErr bool
	}{
		{"the largest representable version", 2_147_483_647, false},
		{"one past the largest", 2_147_483_648, true},
		{"far past the largest", 9_223_372_036_854_775_807, true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			record := publishableRecord(t)
			record.AggregateVersion = c.version

			env, err := Assemble(record, testSource)
			if !c.wantErr {
				if err != nil {
					t.Fatalf("Assemble() = %v, want nil", err)
				}
				if env.AggregateVersion == nil || *env.AggregateVersion != int32(c.version) {
					t.Fatalf("aggregateversion = %v, want %d", env.AggregateVersion, c.version)
				}
				return
			}
			if !errors.Is(err, ErrAggregateVersionOutOfRange) {
				t.Fatalf("Assemble() = %v, want ErrAggregateVersionOutOfRange", err)
			}
			if env.AggregateVersion != nil {
				t.Fatalf("aggregateversion = %d, want nothing assembled", *env.AggregateVersion)
			}
		})
	}
}

func TestAssembleRequiresTheThreeContextAttributes(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		metadata string
	}{
		{"no metadata at all", `{}`},
		{"only correlationid", `{"correlationid":"corr-1"}`},
		{"traceparent missing", `{"correlationid":"corr-1","causationid":"caus-1"}`},
		{"correlationid present but empty", `{"correlationid":"","causationid":"caus-1","traceparent":"tp"}`},
		{"correlationid carried as a number", `{"correlationid":1,"causationid":"caus-1","traceparent":"tp"}`},
		{"metadata is not an object", `[]`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			record := publishableRecord(t)
			record.Metadata = []byte(c.metadata)

			if _, err := Assemble(record, testSource); !errors.Is(err, ErrMissingContextAttributes) {
				t.Fatalf("Assemble() = %v, want ErrMissingContextAttributes", err)
			}
		})
	}
}

// ENV-18: the bytes were frozen on write. A digest that no longer matches means
// corruption at rest, which retrying cannot fix.
func TestAssembleRejectsAMismatchedPayloadHash(t *testing.T) {
	t.Parallel()

	record := publishableRecord(t)
	record.PayloadHash = payloadhash.Sum([]byte("something else entirely"))

	_, err := Assemble(record, testSource)
	if !errors.Is(err, ErrPayloadHashMismatch) {
		t.Fatalf("Assemble() = %v, want ErrPayloadHashMismatch", err)
	}
	if strings.Contains(err.Error(), string(record.Payload)) {
		t.Fatalf("the error reproduces the payload bytes: %q", err)
	}
}

func TestAssembleRequiresASource(t *testing.T) {
	t.Parallel()

	if _, err := Assemble(publishableRecord(t), ""); !errors.Is(err, ErrSourceRequired) {
		t.Fatalf("Assemble() with no source = %v, want ErrSourceRequired", err)
	}
}

func decoded(t *testing.T, env envelope.Envelope) *eventv1.OrderPlaced {
	t.Helper()

	var got eventv1.OrderPlaced
	if err := envelope.Unpack(env, &got); err != nil {
		t.Fatalf("Unpack() = %v, want nil", err)
	}
	return &got
}
