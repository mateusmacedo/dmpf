package envelope_test

import (
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/shared/go/dmpf-contracts/envelope"
	eventv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/shared/go/dmpf-contracts/gen/go/company/orders/event/v1"
	cloudeventsv1 "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/shared/go/dmpf-contracts/gen/go/io/cloudevents/v1"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/shared/go/dmpf-contracts/payloadhash"
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
		Source:           "urn:lidercap:orders",
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

			ce, _ = envelope.Encode(validEnvelope(t))
			if name == "time" {
				return
			}
			ce.Attributes[name] = stringAttr("")
			_, err = envelope.Decode(ce)
			if !errors.Is(err, envelope.ErrMissingAttribute) && !errors.Is(err, envelope.ErrAttributeType) {
				t.Fatalf("empty %q: err = %v", name, err)
			}
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
