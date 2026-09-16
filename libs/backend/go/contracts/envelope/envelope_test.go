package envelope_test

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/company/orders/event/v1"
	cloudeventsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/io/cloudevents/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
)

const (
	wantTypeURL = "type.googleapis.com/company.orders.event.v1.OrderPlaced"
	eventType   = "com.company.orders.order-placed.v1"
)

func orderPlaced() *eventv1.OrderPlaced {
	return &eventv1.OrderPlaced{
		OrderId:    "o-1",
		CustomerId: "c-1",
		TotalCents: 1999,
		Channel:    eventv1.OrderChannel_ORDER_CHANNEL_WEB,
	}
}

func validEnvelope(t *testing.T) envelope.Envelope {
	t.Helper()
	payload, typeURL, err := envelope.Pack(orderPlaced())
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	aggregateVersion := int32(7)
	tenant := "tenant-a"
	traceState := "vendor=1"
	return envelope.Envelope{
		ID:               "evt-1",
		Source:           "urn:dmpf:orders",
		SpecVersion:      envelope.SpecVersion,
		Type:             eventType,
		Subject:          "order/o-1",
		Time:             timestamppb.New(time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)),
		DataSchema:       typeURL,
		DataContentType:  envelope.ContentType,
		CorrelationID:    "corr-1",
		CausationID:      "evt-1",
		PartitionKey:     "c-1",
		TraceParent:      "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
		AggregateVersion: &aggregateVersion,
		TenantID:         &tenant,
		TraceState:       &traceState,
		Payload:          payload,
	}
}

func roundTrip(t *testing.T, ce *cloudeventsv1.CloudEvent) *cloudeventsv1.CloudEvent {
	t.Helper()
	wire, err := proto.Marshal(ce)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back cloudeventsv1.CloudEvent
	if err := proto.Unmarshal(wire, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return &back
}

func TestPackMatchesAnyTypeURL(t *testing.T) {
	t.Parallel()

	payload, typeURL, err := envelope.Pack(orderPlaced())
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if typeURL != wantTypeURL {
		t.Fatalf("Pack type URL = %q, want %q", typeURL, wantTypeURL)
	}
	viaAny, err := anypb.New(orderPlaced())
	if err != nil {
		t.Fatalf("anypb.New: %v", err)
	}
	if viaAny.GetTypeUrl() != typeURL {
		t.Fatalf("anypb.New type URL = %q, Pack = %q", viaAny.GetTypeUrl(), typeURL)
	}
	if string(viaAny.GetValue()) != string(payload) {
		t.Fatal("Pack bytes differ from anypb.New bytes for the same message")
	}
}

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	in := validEnvelope(t)
	ce, err := envelope.Encode(in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	out, err := envelope.Decode(roundTrip(t, ce))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if string(out.Payload) != string(in.Payload) {
		t.Fatal("payload bytes changed across the round trip")
	}
	if payloadhash.Sum(out.Payload) != payloadhash.Sum(in.Payload) {
		t.Fatal("payload_hash changed across the round trip")
	}
	if out.ID != in.ID || out.Source != in.Source || out.Type != in.Type || out.Subject != in.Subject ||
		out.DataSchema != in.DataSchema || out.DataContentType != in.DataContentType ||
		out.CorrelationID != in.CorrelationID || out.CausationID != in.CausationID ||
		out.PartitionKey != in.PartitionKey || out.TraceParent != in.TraceParent ||
		!out.Time.AsTime().Equal(in.Time.AsTime()) {
		t.Fatalf("attributes changed across the round trip:\n in=%+v\nout=%+v", in, out)
	}
	if out.AggregateVersion == nil || *out.AggregateVersion != *in.AggregateVersion ||
		out.TenantID == nil || *out.TenantID != *in.TenantID ||
		out.TraceState == nil || *out.TraceState != *in.TraceState {
		t.Fatalf("conditional attributes changed across the round trip: %+v", out)
	}
	var decoded eventv1.OrderPlaced
	if err := proto.Unmarshal(out.Payload, &decoded); err != nil {
		t.Fatalf("payload does not decode: %v", err)
	}
	if !proto.Equal(&decoded, orderPlaced()) {
		t.Fatalf("payload decoded to %v", &decoded)
	}
}

func TestConditionalsAbsent(t *testing.T) {
	t.Parallel()

	in := validEnvelope(t)
	in.AggregateVersion, in.TenantID, in.TraceState = nil, nil, nil
	ce, err := envelope.Encode(in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for _, name := range []string{"aggregateversion", "tenantid", "tracestate"} {
		if _, present := ce.GetAttributes()[name]; present {
			t.Fatalf("absent conditional %q must not enter the attributes map", name)
		}
	}
	out, err := envelope.Decode(roundTrip(t, ce))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if out.AggregateVersion != nil || out.TenantID != nil || out.TraceState != nil {
		t.Fatalf("absent conditionals decoded as present: %+v", out)
	}
}

func TestConditionalPresentButEmpty(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"tenantid", "tracestate"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in := validEnvelope(t)
			empty := ""
			if name == "tenantid" {
				in.TenantID = &empty
			} else {
				in.TraceState = &empty
			}
			_, err := envelope.Encode(in)
			assertAttributeError(t, err, envelope.ErrEmptyConditional, name)

			ce, _ := envelope.Encode(validEnvelope(t))
			ce.Attributes[name] = stringAttr("")
			_, err = envelope.Decode(ce)
			assertAttributeError(t, err, envelope.ErrEmptyConditional, name)
		})
	}
}

func TestRequiredAttributeMissing(t *testing.T) {
	t.Parallel()

	own := map[string]func(*cloudeventsv1.CloudEvent){
		"id":          func(ce *cloudeventsv1.CloudEvent) { ce.Id = "" },
		"source":      func(ce *cloudeventsv1.CloudEvent) { ce.Source = "" },
		"specversion": func(ce *cloudeventsv1.CloudEvent) { ce.SpecVersion = "" },
		"type":        func(ce *cloudeventsv1.CloudEvent) { ce.Type = "" },
	}
	for name, clear := range own {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ce, _ := envelope.Encode(validEnvelope(t))
			clear(ce)
			_, err := envelope.Decode(ce)
			assertAttributeError(t, err, envelope.ErrMissingAttribute, name)
		})
	}
	mapped := []string{
		"subject", "time", "dataschema", "datacontenttype",
		"correlationid", "causationid", "partitionkey", "traceparent",
	}
	for _, name := range mapped {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ce, _ := envelope.Encode(validEnvelope(t))
			delete(ce.Attributes, name)
			_, err := envelope.Decode(ce)
			assertAttributeError(t, err, envelope.ErrMissingAttribute, name)

			// A timestamp has no empty form; the string-typed attributes and the
			// URI-typed dataschema do, and "present but empty" counts as missing.
			if name == "time" {
				return
			}
			ce, _ = envelope.Encode(validEnvelope(t))
			if name == "dataschema" {
				ce.Attributes[name] = &cloudeventsv1.CloudEvent_CloudEventAttributeValue{
					Attr: &cloudeventsv1.CloudEvent_CloudEventAttributeValue_CeUri{CeUri: ""},
				}
			} else {
				ce.Attributes[name] = stringAttr("")
			}
			_, err = envelope.Decode(ce)
			assertAttributeError(t, err, envelope.ErrMissingAttribute, name)
		})
	}
}

func TestModalityRejected(t *testing.T) {
	t.Parallel()

	cases := map[string]func(*cloudeventsv1.CloudEvent){
		"binary_data": func(ce *cloudeventsv1.CloudEvent) {
			ce.Data = &cloudeventsv1.CloudEvent_BinaryData{BinaryData: []byte{1, 2, 3}}
		},
		"text_data": func(ce *cloudeventsv1.CloudEvent) {
			ce.Data = &cloudeventsv1.CloudEvent_TextData{TextData: "{}"}
		},
		"no data": func(ce *cloudeventsv1.CloudEvent) { ce.Data = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ce, _ := envelope.Encode(validEnvelope(t))
			mutate(ce)
			if _, err := envelope.Decode(roundTrip(t, ce)); !errors.Is(err, envelope.ErrModality) {
				t.Fatalf("err = %v, want ErrModality", err)
			}
		})
	}
	if _, err := envelope.Decode(nil); !errors.Is(err, envelope.ErrModality) {
		t.Fatalf("Decode(nil) err = %v, want ErrModality", err)
	}
}

func TestSchemaAndTypeURLMustMatchLiterally(t *testing.T) {
	t.Parallel()

	ce, _ := envelope.Encode(validEnvelope(t))
	ce.GetProtoData().TypeUrl = wantTypeURL + "X"
	if _, err := envelope.Decode(ce); !errors.Is(err, envelope.ErrSchemaMismatch) {
		t.Fatalf("err = %v, want ErrSchemaMismatch", err)
	}

	// Both ENV-16 checks violated: (a) is reported first.
	ce, _ = envelope.Encode(validEnvelope(t))
	ce.GetProtoData().TypeUrl = wantTypeURL + "X"
	ce.Type = "com.company.orders.order-placed.v2"
	if _, err := envelope.Decode(ce); !errors.Is(err, envelope.ErrSchemaMismatch) {
		t.Fatalf("err = %v, want ErrSchemaMismatch before ErrMajorMismatch", err)
	}
}

func TestMalformedMajorsAreRejected(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		mutate func(*envelope.Envelope)
		want   error
	}{
		"type without dots":          {func(e *envelope.Envelope) { e.Type = "v1" }, envelope.ErrMajorMismatch},
		"dataschema without package": {func(e *envelope.Envelope) { e.DataSchema = "type.googleapis.com/OrderPlaced" }, envelope.ErrMajorMismatch},
		"dataschema without type URL prefix": {func(e *envelope.Envelope) {
			e.DataSchema = "https://schemas.local/company.orders.event.v1.OrderPlaced"
		}, envelope.ErrDataSchemaForm},
		"dataschema is only the prefix": {func(e *envelope.Envelope) { e.DataSchema = "type.googleapis.com/" }, envelope.ErrDataSchemaForm},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			in := validEnvelope(t)
			tc.mutate(&in)
			if _, err := envelope.Encode(in); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}
}

// aggregateversion is an integer: zero is a legitimate value, so it has no
// "empty" form and the ENV-12 rule only applies to the string conditionals.
func TestAggregateVersionZeroIsCarried(t *testing.T) {
	t.Parallel()

	in := validEnvelope(t)
	zero := int32(0)
	in.AggregateVersion = &zero
	ce, err := envelope.Encode(in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	out, err := envelope.Decode(roundTrip(t, ce))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if out.AggregateVersion == nil || *out.AggregateVersion != 0 {
		t.Fatalf("aggregateversion = %v, want present with 0", out.AggregateVersion)
	}
}

func TestMajorConcordance(t *testing.T) {
	t.Parallel()

	in := validEnvelope(t)
	in.Type = "com.company.orders.order-placed.v2"
	if _, err := envelope.Encode(in); !errors.Is(err, envelope.ErrMajorMismatch) {
		t.Fatalf("Encode err = %v, want ErrMajorMismatch", err)
	}

	ce, _ := envelope.Encode(validEnvelope(t))
	ce.Type = "com.company.orders.order-placed.v2"
	if _, err := envelope.Decode(ce); !errors.Is(err, envelope.ErrMajorMismatch) {
		t.Fatalf("Decode err = %v, want ErrMajorMismatch", err)
	}

	same := validEnvelope(t)
	same.Type = "com.other.orders.order-was-placed.v1"
	if _, err := envelope.Encode(same); err != nil {
		t.Fatalf("same major with different strings must pass: %v", err)
	}
}

func TestSpecVersionAndContentType(t *testing.T) {
	t.Parallel()

	in := validEnvelope(t)
	in.SpecVersion = "0.3"
	if _, err := envelope.Encode(in); !errors.Is(err, envelope.ErrSpecVersion) {
		t.Fatalf("err = %v, want ErrSpecVersion", err)
	}
	in = validEnvelope(t)
	in.DataContentType = "application/json"
	if _, err := envelope.Encode(in); !errors.Is(err, envelope.ErrContentType) {
		t.Fatalf("err = %v, want ErrContentType", err)
	}
}

func TestAttributeCarriedWithWrongType(t *testing.T) {
	t.Parallel()

	cases := map[string]*cloudeventsv1.CloudEvent_CloudEventAttributeValue{
		"time":             stringAttr("2026-09-02T12:00:00Z"),
		"aggregateversion": stringAttr("7"),
		"dataschema":       stringAttr(wantTypeURL),
		"subject":          {Attr: &cloudeventsv1.CloudEvent_CloudEventAttributeValue_CeInteger{CeInteger: 1}},
	}
	for name, wrong := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ce, _ := envelope.Encode(validEnvelope(t))
			ce.Attributes[name] = wrong
			_, err := envelope.Decode(ce)
			assertAttributeError(t, err, envelope.ErrAttributeType, name)
		})
	}
}

func TestPayloadHashIgnoresEnvelope(t *testing.T) {
	t.Parallel()

	base := validEnvelope(t)
	want := payloadhash.Sum(base.Payload)
	variants := map[string]func(*envelope.Envelope){
		"id":               func(e *envelope.Envelope) { e.ID = "other" },
		"source":           func(e *envelope.Envelope) { e.Source = "urn:other" },
		"type":             func(e *envelope.Envelope) { e.Type = "com.other.orders.order-was-placed.v1" },
		"subject":          func(e *envelope.Envelope) { e.Subject = "order/o-2" },
		"time":             func(e *envelope.Envelope) { e.Time = timestamppb.Now() },
		"correlationid":    func(e *envelope.Envelope) { e.CorrelationID = "corr-2" },
		"causationid":      func(e *envelope.Envelope) { e.CausationID = "evt-0" },
		"partitionkey":     func(e *envelope.Envelope) { e.PartitionKey = "c-2" },
		"traceparent":      func(e *envelope.Envelope) { e.TraceParent = "00-1-2-00" },
		"aggregateversion": func(e *envelope.Envelope) { v := int32(8); e.AggregateVersion = &v },
		"tenantid":         func(e *envelope.Envelope) { e.TenantID = nil },
		"tracestate":       func(e *envelope.Envelope) { e.TraceState = nil },
		// dataschema points at another message of the same major; Encode does not
		// decode the payload, so the bytes stay the same and only the URL varies.
		"dataschema": func(e *envelope.Envelope) {
			e.DataSchema = "type.googleapis.com/company.orders.event.v1.OrderCancelled"
		},
		// specversion and datacontenttype are fixed by the profile: the only value
		// Validate accepts is the one already in place, so they cannot vary alone.
	}
	for name, mutate := range variants {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := validEnvelope(t)
			mutate(&e)
			ce, err := envelope.Encode(e)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			out, err := envelope.Decode(roundTrip(t, ce))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if got := payloadhash.Sum(out.Payload); got != want {
				t.Fatalf("payload_hash = %s, want %s (attribute %q must not affect it)", got, want, name)
			}
		})
	}
}

func stringAttr(v string) *cloudeventsv1.CloudEvent_CloudEventAttributeValue {
	return &cloudeventsv1.CloudEvent_CloudEventAttributeValue{
		Attr: &cloudeventsv1.CloudEvent_CloudEventAttributeValue_CeString{CeString: v},
	}
}

func assertAttributeError(t *testing.T, err, sentinel error, attribute string) {
	t.Helper()
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want %v", err, sentinel)
	}
	var attrErr *envelope.AttributeError
	if !errors.As(err, &attrErr) || attrErr.Attribute != attribute {
		t.Fatalf("err = %v, want attribute %q", err, attribute)
	}
}

func TestUnmarshalRejectsInvalidBytes(t *testing.T) {
	t.Parallel()

	_, err := envelope.Unmarshal([]byte{0xff, 0xff, 0xff})
	if !errors.Is(err, envelope.ErrMalformed) {
		t.Fatalf("err = %v, want %v", err, envelope.ErrMalformed)
	}
}

func TestUnmarshalRoundTrip(t *testing.T) {
	t.Parallel()

	in := validEnvelope(t)
	ce, err := envelope.Encode(in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	wire, err := proto.Marshal(ce)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out, err := envelope.Unmarshal(wire)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.ID != in.ID || out.Source != in.Source || out.Type != in.Type {
		t.Fatalf("attributes changed across the round trip:\n in=%+v\nout=%+v", in, out)
	}
	if !bytes.Equal(out.Payload, in.Payload) {
		t.Fatalf("payload bytes changed across the round trip: in=%x out=%x", in.Payload, out.Payload)
	}
}

func TestUnpackDecodesThePayloadNamedByDataschema(t *testing.T) {
	env := validEnvelope(t)

	var got eventv1.OrderPlaced
	if err := envelope.Unpack(env, &got); err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	if !proto.Equal(&got, orderPlaced()) {
		t.Fatalf("Unpack = %v, want %v", &got, orderPlaced())
	}
}

func TestUnpackRefusesAMessageOfAnotherContract(t *testing.T) {
	env := validEnvelope(t)

	var wrong eventv1.ItemAdded
	err := envelope.Unpack(env, &wrong)
	if !errors.Is(err, envelope.ErrSchemaMismatch) {
		t.Fatalf("Unpack into another contract = %v, want ErrSchemaMismatch", err)
	}
}

func TestUnpackRefusesBytesThatDoNotDecode(t *testing.T) {
	env := validEnvelope(t)
	env.Payload = []byte{0xff, 0xff, 0xff}

	var got eventv1.OrderPlaced
	if err := envelope.Unpack(env, &got); !errors.Is(err, envelope.ErrMalformed) {
		t.Fatalf("Unpack of garbage = %v, want ErrMalformed", err)
	}
}

func TestMarshalRoundTrip(t *testing.T) {
	t.Parallel()

	in := validEnvelope(t)
	wire, err := envelope.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out, err := envelope.Unmarshal(wire)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if out.ID != in.ID || out.Source != in.Source || out.Type != in.Type || out.Subject != in.Subject {
		t.Fatalf("attributes changed across the round trip:\n in=%+v\nout=%+v", in, out)
	}
	if out.AggregateVersion == nil || *out.AggregateVersion != *in.AggregateVersion {
		t.Fatalf("aggregateversion = %v, want %d", out.AggregateVersion, *in.AggregateVersion)
	}
	if out.TenantID == nil || *out.TenantID != *in.TenantID {
		t.Fatalf("tenantid = %v, want %q", out.TenantID, *in.TenantID)
	}
	if !bytes.Equal(out.Payload, in.Payload) {
		t.Fatalf("payload bytes changed across the round trip: in=%x out=%x", in.Payload, out.Payload)
	}
}

func TestMarshalMatchesEncodePlusProtoMarshal(t *testing.T) {
	t.Parallel()

	in := validEnvelope(t)
	ce, err := envelope.Encode(in)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	want, err := proto.Marshal(ce)
	if err != nil {
		t.Fatalf("proto.Marshal: %v", err)
	}
	got, err := envelope.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var backGot, backWant cloudeventsv1.CloudEvent
	if err := proto.Unmarshal(got, &backGot); err != nil {
		t.Fatalf("Unmarshal(got): %v", err)
	}
	if err := proto.Unmarshal(want, &backWant); err != nil {
		t.Fatalf("Unmarshal(want): %v", err)
	}
	if !proto.Equal(&backGot, &backWant) {
		t.Fatalf("Marshal produced a different CloudEvent:\n got=%v\nwant=%v", &backGot, &backWant)
	}
}

func TestMarshalRejectsAnInvalidEnvelopeBeforeSerializing(t *testing.T) {
	t.Parallel()

	in := validEnvelope(t)
	in.Subject = ""

	wire, err := envelope.Marshal(in)
	assertAttributeError(t, err, envelope.ErrMissingAttribute, "subject")
	if wire != nil {
		t.Fatalf("Marshal returned %d bytes for an invalid envelope, want nil", len(wire))
	}
}
