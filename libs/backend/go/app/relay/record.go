package relay

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

// The three ENV-08 attributes with no column of their own. FND-05 leaves the
// form of persisting them to FND-04, which hands the content back to FND-07;
// metadata is where they land, and reading them is all this block may do.
const (
	metaCorrelationID = "correlationid"
	metaCausationID   = "causationid"
	metaTraceParent   = "traceparent"
	metaTenantID      = "tenantid"
)

// Assemble turns one claimed record into the envelope of ENV-14, without ever
// decoding the payload: the bytes were frozen on write (ENV-18) and travel as
// they are. Every error it reports is terminal — none of them is repaired by
// publishing the same record again.
func Assemble(record postgres.Claimed, source string) (envelope.Envelope, error) {
	if source == "" {
		return envelope.Envelope{}, ErrSourceRequired
	}
	if got := payloadhash.Sum(record.Payload); got != record.PayloadHash {
		return envelope.Envelope{}, fmt.Errorf("%w: stored %s, computed %s", ErrPayloadHashMismatch, record.PayloadHash, got)
	}
	if record.AggregateVersion < 0 || record.AggregateVersion > math.MaxInt32 {
		return envelope.Envelope{}, fmt.Errorf("%w: %d", ErrAggregateVersionOutOfRange, record.AggregateVersion)
	}

	attributes, err := contextAttributes(record.Metadata)
	if err != nil {
		return envelope.Envelope{}, err
	}

	aggregateVersion := int32(record.AggregateVersion)

	// CTX-13: the tenant crosses preserved, and its absence means a platform
	// chain — so the pointer stays nil rather than carrying "".
	var tenantID *string
	if tenant := attributes[metaTenantID]; tenant != "" {
		tenantID = &tenant
	}

	return envelope.Envelope{
		ID:               record.MessageID,
		Source:           source,
		SpecVersion:      envelope.SpecVersion,
		Type:             record.MessageType,
		Subject:          record.AggregateID,
		Time:             timestamppb.New(time.Unix(0, int64(record.OccurredAt)).UTC()),
		DataSchema:       record.SchemaVersion,
		DataContentType:  envelope.ContentType,
		CorrelationID:    attributes[metaCorrelationID],
		CausationID:      attributes[metaCausationID],
		PartitionKey:     record.PartitionKey,
		TraceParent:      attributes[metaTraceParent],
		AggregateVersion: &aggregateVersion,
		TenantID:         tenantID,
		Payload:          record.Payload,
	}, nil
}

// contextAttributes reads the three keys as strings. RawMessage rather than
// map[string]string because metadata is open: a key this block does not read,
// carried as an object or a number, must not fail the ones it does.
func contextAttributes(metadata []byte) (map[string]string, error) {
	raw := map[string]json.RawMessage{}
	if len(metadata) > 0 {
		if err := json.Unmarshal(metadata, &raw); err != nil {
			return nil, fmt.Errorf("%w: metadata is not a JSON object", ErrMissingContextAttributes)
		}
	}

	attributes := make(map[string]string, 4)
	for _, key := range [...]string{metaCorrelationID, metaCausationID, metaTraceParent} {
		value, ok := raw[key]
		if !ok {
			return nil, fmt.Errorf("%w: %s is absent", ErrMissingContextAttributes, key)
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil || text == "" {
			return nil, fmt.Errorf("%w: %s is not a non-empty string", ErrMissingContextAttributes, key)
		}
		attributes[key] = text
	}

	// The tenant is absent on a platform chain (CTX-26), so its absence is not a
	// defect. Present and unreadable is: treating that as absence would turn one
	// tenant's fact into a chain that belongs to nobody.
	if value, ok := raw[metaTenantID]; ok {
		var text string
		if err := json.Unmarshal(value, &text); err != nil || text == "" {
			return nil, fmt.Errorf("%w: %s is present but not a non-empty string", ErrMissingContextAttributes, metaTenantID)
		}
		attributes[metaTenantID] = text
	}
	return attributes, nil
}
