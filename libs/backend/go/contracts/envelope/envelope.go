// Package envelope encodes and decodes the DMPF profile of the official
// CloudEvents Protobuf envelope (docs/dmpf/cloudevents-protobuf-buf.md §3-§4;
// docs/adr/022): proto_data only, fifteen attributes, and the ENV-16 checks.
package envelope

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	cloudeventsv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/io/cloudevents/v1"
)

const (
	SpecVersion = "1.0"
	ContentType = "application/protobuf"
)

// typeURLPrefix mirrors what anypb.New emits, so dataschema and Any.type_url compare literally (ENV-16 a).
const typeURLPrefix = "type.googleapis.com/"

const (
	attrSubject          = "subject"
	attrTime             = "time"
	attrDataSchema       = "dataschema"
	attrDataContentType  = "datacontenttype"
	attrCorrelationID    = "correlationid"
	attrCausationID      = "causationid"
	attrPartitionKey     = "partitionkey"
	attrAggregateVersion = "aggregateversion"
	attrTenantID         = "tenantid"
	attrTraceParent      = "traceparent"
	attrTraceState       = "tracestate"
)

// Envelope is the profile view of a CloudEvent: the fifteen ENV-08 attributes
// plus the payload bytes exactly as carried in Any.value. Conditional
// attributes are pointers; nil means absent, never a fill value (ENV-12).
type Envelope struct {
	ID              string
	Source          string
	SpecVersion     string
	Type            string
	Subject         string
	Time            *timestamppb.Timestamp
	DataSchema      string
	DataContentType string
	CorrelationID   string
	CausationID     string
	PartitionKey    string
	TraceParent     string

	AggregateVersion *int32
	TenantID         *string
	TraceState       *string

	Payload []byte
}

// Pack is the only bridge from a message to (Payload, DataSchema); Deterministic
// makes the bytes reproducible within Go, which the golden fixture relies on.
func Pack(msg proto.Message) (payload []byte, typeURL string, err error) {
	payload, err = proto.MarshalOptions{Deterministic: true}.Marshal(msg)
	if err != nil {
		return nil, "", err
	}
	return payload, typeURLPrefix + string(msg.ProtoReflect().Descriptor().FullName()), nil
}

// Validate applies the profile in the order the norm lists it: presence
// (ENV-08, ENV-12) before the value rules (specversion, content type, ENV-16 b).
func (e Envelope) Validate() error {
	if err := e.validatePresence(); err != nil {
		return err
	}
	return e.validateProfile()
}

func (e Envelope) validatePresence() error {
	required := []struct{ name, value string }{
		{"id", e.ID}, {"source", e.Source}, {"specversion", e.SpecVersion}, {"type", e.Type},
		{attrSubject, e.Subject}, {attrDataSchema, e.DataSchema}, {attrDataContentType, e.DataContentType},
		{attrCorrelationID, e.CorrelationID}, {attrCausationID, e.CausationID},
		{attrPartitionKey, e.PartitionKey}, {attrTraceParent, e.TraceParent},
	}
	for _, r := range required {
		if r.value == "" {
			return attributeError(ErrMissingAttribute, r.name)
		}
	}
	if e.Time == nil {
		return attributeError(ErrMissingAttribute, attrTime)
	}
	if e.TenantID != nil && *e.TenantID == "" {
		return attributeError(ErrEmptyConditional, attrTenantID)
	}
	if e.TraceState != nil && *e.TraceState == "" {
		return attributeError(ErrEmptyConditional, attrTraceState)
	}
	return nil
}

func (e Envelope) validateProfile() error {
	if e.SpecVersion != SpecVersion {
		return ErrSpecVersion
	}
	if e.DataContentType != ContentType {
		return ErrContentType
	}
	if !strings.HasPrefix(e.DataSchema, typeURLPrefix) || len(e.DataSchema) == len(typeURLPrefix) {
		return ErrDataSchemaForm
	}
	if majorOfSchema(e.DataSchema) == "" || majorOfSchema(e.DataSchema) != majorOfType(e.Type) {
		return ErrMajorMismatch
	}
	return nil
}

func Encode(e Envelope) (*cloudeventsv1.CloudEvent, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	attrs := map[string]*cloudeventsv1.CloudEvent_CloudEventAttributeValue{
		attrSubject:         ceString(e.Subject),
		attrTime:            {Attr: &cloudeventsv1.CloudEvent_CloudEventAttributeValue_CeTimestamp{CeTimestamp: e.Time}},
		attrDataSchema:      {Attr: &cloudeventsv1.CloudEvent_CloudEventAttributeValue_CeUri{CeUri: e.DataSchema}},
		attrDataContentType: ceString(e.DataContentType),
		attrCorrelationID:   ceString(e.CorrelationID),
		attrCausationID:     ceString(e.CausationID),
		attrPartitionKey:    ceString(e.PartitionKey),
		attrTraceParent:     ceString(e.TraceParent),
	}
	if e.AggregateVersion != nil {
		attrs[attrAggregateVersion] = &cloudeventsv1.CloudEvent_CloudEventAttributeValue{
			Attr: &cloudeventsv1.CloudEvent_CloudEventAttributeValue_CeInteger{CeInteger: *e.AggregateVersion},
		}
	}
	if e.TenantID != nil {
		attrs[attrTenantID] = ceString(*e.TenantID)
	}
	if e.TraceState != nil {
		attrs[attrTraceState] = ceString(*e.TraceState)
	}
	return &cloudeventsv1.CloudEvent{
		Id:          e.ID,
		Source:      e.Source,
		SpecVersion: e.SpecVersion,
		Type:        e.Type,
		Attributes:  attrs,
		Data: &cloudeventsv1.CloudEvent_ProtoData{
			ProtoData: &anypb.Any{TypeUrl: e.DataSchema, Value: e.Payload},
		},
	}, nil
}

// Decode applies the profile to a CloudEvent and hands back Any.value untouched,
// so payloadhash.Sum sees the transported bytes (ENV-17).
func Decode(ce *cloudeventsv1.CloudEvent) (Envelope, error) {
	var e Envelope
	if ce == nil {
		return e, ErrModality
	}
	protoData, ok := ce.GetData().(*cloudeventsv1.CloudEvent_ProtoData)
	if !ok || protoData.ProtoData == nil {
		return e, ErrModality
	}

	e.ID, e.Source, e.SpecVersion, e.Type = ce.GetId(), ce.GetSource(), ce.GetSpecVersion(), ce.GetType()
	attrs := ce.GetAttributes()

	var err error
	if e.Subject, err = requiredString(attrs, attrSubject); err != nil {
		return e, err
	}
	if e.Time, err = requiredTimestamp(attrs, attrTime); err != nil {
		return e, err
	}
	if e.DataSchema, err = requiredURI(attrs, attrDataSchema); err != nil {
		return e, err
	}
	if e.DataContentType, err = requiredString(attrs, attrDataContentType); err != nil {
		return e, err
	}
	if e.CorrelationID, err = requiredString(attrs, attrCorrelationID); err != nil {
		return e, err
	}
	if e.CausationID, err = requiredString(attrs, attrCausationID); err != nil {
		return e, err
	}
	if e.PartitionKey, err = requiredString(attrs, attrPartitionKey); err != nil {
		return e, err
	}
	if e.TraceParent, err = requiredString(attrs, attrTraceParent); err != nil {
		return e, err
	}
	if e.AggregateVersion, err = optionalInteger(attrs, attrAggregateVersion); err != nil {
		return e, err
	}
	if e.TenantID, err = optionalString(attrs, attrTenantID); err != nil {
		return e, err
	}
	if e.TraceState, err = optionalString(attrs, attrTraceState); err != nil {
		return e, err
	}
	if err := e.validatePresence(); err != nil {
		return e, err
	}
	// ENV-16 (a) before (b): the norm lists the literal comparison first, so a
	// doubly wrong event reports the schema, not the major.
	if protoData.ProtoData.GetTypeUrl() != e.DataSchema {
		return e, ErrSchemaMismatch
	}
	if err := e.validateProfile(); err != nil {
		return e, err
	}
	e.Payload = protoData.ProtoData.GetValue()
	return e, nil
}

// Marshal is the inverse of Unmarshal: it applies the profile (Encode) and
// serializes, so a publisher that hands raw transport bytes to a broker never
// imports the CloudEvent generated type directly.
func Marshal(e Envelope) ([]byte, error) {
	ce, err := Encode(e)
	if err != nil {
		return nil, err
	}
	return proto.Marshal(ce)
}

// Unmarshal decodes the wire bytes of a CloudEvent and applies the profile
// (Decode), so adapters that receive raw transport bytes never import the
// CloudEvent generated type directly.
func Unmarshal(raw []byte) (Envelope, error) {
	var ce cloudeventsv1.CloudEvent
	if err := proto.Unmarshal(raw, &ce); err != nil {
		return Envelope{}, fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	return Decode(&ce)
}

// Unpack is the inverse of Pack: it decodes Payload into msg only when
// dataschema names msg's own type (ENV-16 a), so a consumer never has to guess
// which contract the transported bytes carry.
func Unpack(e Envelope, msg proto.Message) error {
	want := typeURLPrefix + string(msg.ProtoReflect().Descriptor().FullName())
	if e.DataSchema != want {
		return fmt.Errorf("%w: %s is not %s", ErrSchemaMismatch, e.DataSchema, want)
	}
	if err := proto.Unmarshal(e.Payload, msg); err != nil {
		return fmt.Errorf("%w: %w", ErrMalformed, err)
	}
	return nil
}

func ceString(v string) *cloudeventsv1.CloudEvent_CloudEventAttributeValue {
	return &cloudeventsv1.CloudEvent_CloudEventAttributeValue{
		Attr: &cloudeventsv1.CloudEvent_CloudEventAttributeValue_CeString{CeString: v},
	}
}

func requiredString(attrs map[string]*cloudeventsv1.CloudEvent_CloudEventAttributeValue, name string) (string, error) {
	v, ok := attrs[name]
	if !ok {
		return "", attributeError(ErrMissingAttribute, name)
	}
	s, ok := v.GetAttr().(*cloudeventsv1.CloudEvent_CloudEventAttributeValue_CeString)
	if !ok {
		return "", attributeError(ErrAttributeType, name)
	}
	return s.CeString, nil
}

func requiredURI(attrs map[string]*cloudeventsv1.CloudEvent_CloudEventAttributeValue, name string) (string, error) {
	v, ok := attrs[name]
	if !ok {
		return "", attributeError(ErrMissingAttribute, name)
	}
	u, ok := v.GetAttr().(*cloudeventsv1.CloudEvent_CloudEventAttributeValue_CeUri)
	if !ok {
		return "", attributeError(ErrAttributeType, name)
	}
	return u.CeUri, nil
}

func requiredTimestamp(attrs map[string]*cloudeventsv1.CloudEvent_CloudEventAttributeValue, name string) (*timestamppb.Timestamp, error) {
	v, ok := attrs[name]
	if !ok {
		return nil, attributeError(ErrMissingAttribute, name)
	}
	ts, ok := v.GetAttr().(*cloudeventsv1.CloudEvent_CloudEventAttributeValue_CeTimestamp)
	if !ok {
		return nil, attributeError(ErrAttributeType, name)
	}
	return ts.CeTimestamp, nil
}

func optionalString(attrs map[string]*cloudeventsv1.CloudEvent_CloudEventAttributeValue, name string) (*string, error) {
	v, ok := attrs[name]
	if !ok {
		return nil, nil
	}
	s, ok := v.GetAttr().(*cloudeventsv1.CloudEvent_CloudEventAttributeValue_CeString)
	if !ok {
		return nil, attributeError(ErrAttributeType, name)
	}
	value := s.CeString
	return &value, nil
}

func optionalInteger(attrs map[string]*cloudeventsv1.CloudEvent_CloudEventAttributeValue, name string) (*int32, error) {
	v, ok := attrs[name]
	if !ok {
		return nil, nil
	}
	i, ok := v.GetAttr().(*cloudeventsv1.CloudEvent_CloudEventAttributeValue_CeInteger)
	if !ok {
		return nil, attributeError(ErrAttributeType, name)
	}
	value := i.CeInteger
	return &value, nil
}

// majorOfSchema reads the package major of the fully qualified proto name that
// dataschema carries (PTB-01 form): "…/company.orders.event.v1.OrderPlaced" → "v1".
func majorOfSchema(dataSchema string) string {
	fqn := dataSchema[strings.LastIndex(dataSchema, "/")+1:]
	segments := strings.Split(fqn, ".")
	if len(segments) < 2 {
		return ""
	}
	return segments[len(segments)-2]
}

// majorOfType reads the trailing major of the envelope type (PTB-03 form):
// "com.company.orders.order-placed.v1" → "v1".
func majorOfType(eventType string) string {
	dot := strings.LastIndex(eventType, ".")
	if dot < 0 {
		return ""
	}
	return eventType[dot+1:]
}
