package golden

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/envelope"
	cloudeventsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/gen/go/io/cloudevents/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-contracts/payloadhash"
)

// Subject is what the contract under test contributes: a fresh message and
// the single reader of the string-typed payload, shared with the generator so
// the two never disagree on parsing (FIX-02).
type Subject struct {
	NewMessage        func() proto.Message
	MessageFromFields func(fields map[string]string) (proto.Message, error)
}

// Consumer runs the reading direction: bytes → hash (oracle 2) → decoded
// message against the declared fields, then the envelope through
// Encode/Marshal/Unmarshal/Decode attribute by attribute (oracle 1), and the
// identity of Any.value across that trip (oracle 3).
type Consumer struct{}

func (Consumer) Run(f Fixture, c Case, s Subject) []Outcome {
	r := newRecorder(f, c, DirectionConsumer)
	transported, err := hex.DecodeString(c.PayloadBytesHex)
	if err != nil {
		for _, o := range []Oracle{OracleSemantic, OracleHash, OracleBytes} {
			r.fail(o, "payload_bytes_hex", "hexadecimal", err.Error())
		}
		return r.outcomes()
	}

	if got := payloadhash.Sum(transported); got != c.PayloadHash {
		r.fail(OracleHash, "payload_hash", c.PayloadHash, got)
	}

	decoded := s.NewMessage()
	if err := proto.Unmarshal(transported, decoded); err != nil {
		r.fail(OracleSemantic, "payload", "decodes as "+f.Identity.DataSchema, err.Error())
	} else {
		// The unknown-field discriminator carries bytes the contract does not
		// declare, while the payload map describes only the known fields.
		known := proto.Clone(decoded)
		known.ProtoReflect().SetUnknown(nil)
		want, err := s.MessageFromFields(c.Payload)
		if err != nil {
			r.fail(OracleSemantic, "payload", "declared fields build a message", err.Error())
		} else if !proto.Equal(known, want) {
			field, expected, got := firstDivergence(want, known)
			r.fail(OracleSemantic, field, expected, got)
		}
	}

	in, attr, err := envelopeFromFields(c.Envelope, transported)
	if err != nil {
		r.fail(OracleSemantic, "envelope."+attr, "well-formed attribute", err.Error())
		r.skipped(OracleBytes, "payload", "Any.value identity across the envelope trip", "envelope."+attr+" is not well-formed")
		return r.outcomes()
	}
	out, err := envelopeRoundTrip(in)
	if err != nil {
		r.fail(OracleSemantic, "envelope", "survives Encode/Marshal/Unmarshal/Decode", err.Error())
		r.skipped(OracleBytes, "payload", "Any.value identity across the envelope trip", "the envelope did not survive the trip")
		return r.outcomes()
	}
	if attr, expected, got, same := sameEnvelope(in, out); !same {
		r.fail(OracleSemantic, "envelope."+attr, expected, got)
	}
	if !bytes.Equal(out.Payload, transported) {
		r.fail(OracleBytes, "payload", hex.EncodeToString(transported), hex.EncodeToString(out.Payload))
	}
	if got := payloadhash.Sum(out.Payload); got != c.PayloadHash {
		r.fail(OracleHash, "payload_hash", c.PayloadHash, got)
	}
	return r.outcomes()
}

// Producer runs the writing direction: the declared fields serialized
// deterministically must be the transported bytes (oracle 3, ENV-24) and hash
// to the declared payload_hash (oracle 2); the declared envelope must encode
// (oracle 1). A discriminator is non-canonical by design and only the consumer
// runs over it — see Evaluate.
type Producer struct{}

func (Producer) Run(f Fixture, c Case, s Subject) []Outcome {
	r := newRecorder(f, c, DirectionProducer)
	want, err := s.MessageFromFields(c.Payload)
	if err != nil {
		for _, o := range []Oracle{OracleSemantic, OracleHash, OracleBytes} {
			r.fail(o, "payload", "declared fields build a message", err.Error())
		}
		return r.outcomes()
	}
	produced, err := proto.MarshalOptions{Deterministic: true}.Marshal(want)
	if err != nil {
		for _, o := range []Oracle{OracleSemantic, OracleHash, OracleBytes} {
			r.fail(o, "payload", "message serializes", err.Error())
		}
		return r.outcomes()
	}

	transported, err := hex.DecodeString(c.PayloadBytesHex)
	if err != nil {
		r.fail(OracleBytes, "payload_bytes_hex", "hexadecimal", err.Error())
	} else if !bytes.Equal(produced, transported) {
		r.fail(OracleBytes, "payload_bytes_hex", hex.EncodeToString(transported), hex.EncodeToString(produced))
	}
	if got := payloadhash.Sum(produced); got != c.PayloadHash {
		r.fail(OracleHash, "payload_hash", c.PayloadHash, got)
	}

	in, attr, err := envelopeFromFields(c.Envelope, produced)
	if err != nil {
		r.fail(OracleSemantic, "envelope."+attr, "well-formed attribute", err.Error())
		return r.outcomes()
	}
	if _, err := envelope.Encode(in); err != nil {
		r.fail(OracleSemantic, "envelope", "encodes", err.Error())
	}
	return r.outcomes()
}

// Evaluate runs both directions over a fixture: every case and discriminator
// as a consumer, the canonical cases as a producer.
func Evaluate(f Fixture, s Subject) Report {
	var rep Report
	for _, c := range f.AllCases() {
		rep.Add(Consumer{}.Run(f, c, s)...)
	}
	for _, c := range f.Cases {
		rep.Add(Producer{}.Run(f, c, s)...)
	}
	return rep
}

func envelopeRoundTrip(in envelope.Envelope) (envelope.Envelope, error) {
	ce, err := envelope.Encode(in)
	if err != nil {
		return envelope.Envelope{}, fmt.Errorf("encode: %w", err)
	}
	wire, err := proto.Marshal(ce)
	if err != nil {
		return envelope.Envelope{}, fmt.Errorf("marshal: %w", err)
	}
	var back cloudeventsv1.CloudEvent
	if err := proto.Unmarshal(wire, &back); err != nil {
		return envelope.Envelope{}, fmt.Errorf("unmarshal: %w", err)
	}
	out, err := envelope.Decode(&back)
	if err != nil {
		return envelope.Envelope{}, fmt.Errorf("decode: %w", err)
	}
	return out, nil
}

// envelopeFromFields builds the envelope the fixture declares. The timestamp
// goes through protojson because a contract-block unit may not import time
// (io.clock) and the Timestamp JSON mapping is RFC 3339 already.
func envelopeFromFields(fields map[string]string, payload []byte) (envelope.Envelope, string, error) {
	ts := &timestamppb.Timestamp{}
	if err := protojson.Unmarshal([]byte(strconv.Quote(fields["time"])), ts); err != nil {
		return envelope.Envelope{}, "time", err
	}
	e := envelope.Envelope{
		ID:              fields["id"],
		Source:          fields["source"],
		SpecVersion:     fields["specversion"],
		Type:            fields["type"],
		Subject:         fields["subject"],
		Time:            ts,
		DataSchema:      fields["dataschema"],
		DataContentType: fields["datacontenttype"],
		CorrelationID:   fields["correlationid"],
		CausationID:     fields["causationid"],
		PartitionKey:    fields["partitionkey"],
		TraceParent:     fields["traceparent"],
		Payload:         payload,
	}
	if v, ok := fields["aggregateversion"]; ok {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return envelope.Envelope{}, "aggregateversion", err
		}
		av := int32(n)
		e.AggregateVersion = &av
	}
	if v, ok := fields["tenantid"]; ok {
		tenant := v
		e.TenantID = &tenant
	}
	if v, ok := fields["tracestate"]; ok {
		state := v
		e.TraceState = &state
	}
	return e, "", nil
}

// sameEnvelope compares the eleven required attributes, the timestamp and the
// three conditionals, naming the first that differs.
func sameEnvelope(in, out envelope.Envelope) (attr, expected, got string, same bool) {
	pairs := []struct{ name, in, out string }{
		{"id", in.ID, out.ID}, {"source", in.Source, out.Source}, {"specversion", in.SpecVersion, out.SpecVersion},
		{"type", in.Type, out.Type}, {"subject", in.Subject, out.Subject}, {"dataschema", in.DataSchema, out.DataSchema},
		{"datacontenttype", in.DataContentType, out.DataContentType}, {"correlationid", in.CorrelationID, out.CorrelationID},
		{"causationid", in.CausationID, out.CausationID}, {"partitionkey", in.PartitionKey, out.PartitionKey},
		{"traceparent", in.TraceParent, out.TraceParent},
		{"time", in.Time.AsTime().UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), out.Time.AsTime().UTC().Format("2006-01-02T15:04:05.999999999Z07:00")},
		{"aggregateversion", optInt(in.AggregateVersion), optInt(out.AggregateVersion)},
		{"tenantid", optString(in.TenantID), optString(out.TenantID)},
		{"tracestate", optString(in.TraceState), optString(out.TraceState)},
	}
	for _, p := range pairs {
		if p.in != p.out {
			return p.name, p.in, p.out, false
		}
	}
	return "", "", "", true
}

func optString(p *string) string {
	if p == nil {
		return "<absent>"
	}
	return *p
}

func optInt(p *int32) string {
	if p == nil {
		return "<absent>"
	}
	return strconv.FormatInt(int64(*p), 10)
}

// firstDivergence names the first declared field whose value differs, in
// field-number order; when only unknown or nested content differs it reports
// the message as a whole.
func firstDivergence(want, got proto.Message) (field, expected, actual string) {
	wr, gr := want.ProtoReflect(), got.ProtoReflect()
	fields := wr.Descriptor().Fields()
	for i := 0; i < fields.Len(); i++ {
		fd := fields.Get(i)
		w, g := valueString(fd, wr), valueString(fd, gr)
		if w != g {
			return string(fd.Name()), w, g
		}
	}
	return "(message)", string(protojson.MarshalOptions{}.Format(want)), string(protojson.MarshalOptions{}.Format(got))
}

func valueString(fd protoreflect.FieldDescriptor, m protoreflect.Message) string {
	if !m.Has(fd) && fd.HasPresence() {
		return "<absent>"
	}
	return describeValue(fd, m.Get(fd))
}

// describeValue renders a field value deterministically: lists element by
// element, maps by sorted key, messages through protojson, scalars as is.
func describeValue(fd protoreflect.FieldDescriptor, v protoreflect.Value) string {
	switch {
	case fd.IsList():
		list := v.List()
		parts := make([]string, 0, list.Len())
		for i := 0; i < list.Len(); i++ {
			parts = append(parts, describeScalar(fd, list.Get(i)))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case fd.IsMap():
		mp := v.Map()
		keys := make([]string, 0, mp.Len())
		entries := map[string]string{}
		mp.Range(func(k protoreflect.MapKey, val protoreflect.Value) bool {
			ks := k.String()
			keys = append(keys, ks)
			entries[ks] = describeScalar(fd.MapValue(), val)
			return true
		})
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			parts = append(parts, k+"="+entries[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return describeScalar(fd, v)
	}
}

func describeScalar(fd protoreflect.FieldDescriptor, v protoreflect.Value) string {
	if fd.Kind() == protoreflect.MessageKind || fd.Kind() == protoreflect.GroupKind {
		return string(protojson.MarshalOptions{}.Format(v.Message().Interface()))
	}
	return v.String()
}
