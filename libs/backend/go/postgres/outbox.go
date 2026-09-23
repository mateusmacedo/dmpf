package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/envelope"
	"github.com/mateusmacedo/dmpf/libs/backend/go/contracts/payloadhash"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// uniqueViolation is the SQLSTATE PostgreSQL raises for a unique constraint.
const uniqueViolation = "23505"

// status, attempt_count and the lease columns are left to the schema: their
// initial values are drain state, and the writer has no business authoring them
// (BLK-05). available_at is written because it anchors on occurred_at (OBX-05).
const insertOutbox = `
INSERT INTO dmpf_outbox (
	message_id, message_type, schema_version,
	aggregate_type, aggregate_id, aggregate_version,
	partition_key, destination,
	payload, payload_hash, metadata,
	occurred_at, available_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

// Outbox binds a mapper to this open transaction: the intent to publish is
// written on the same pgx.Tx as the business state, which is the whole point of
// the pattern — one commit makes both visible or neither (UOW-08).
func (t *Tx) Outbox(mapper EventMapper) ports.Outbox {
	return txOutbox{tx: t, mapper: mapper}
}

type txOutbox struct {
	tx     *Tx
	mapper EventMapper
}

func (o txOutbox) Enqueue(ctx context.Context, entry ports.OutboxEntry) error {
	if entry.MessageID == "" {
		return ErrEmptyMessageID
	}
	if err := checkDestination(entry.Intent.Destination); err != nil {
		return err
	}

	mapped, err := o.mapper.Map(entry.Event)
	if err != nil {
		return err
	}
	if err := checkMajor(mapped); err != nil {
		return err
	}

	// Serialização na escrita: os bytes do fato congelam aqui. Um mapeador que
	// mude depois descreve eventos novos e não reescreve os já gravados, que é o
	// que ADR-021 protege e o que a drenagem lê sem reserializar (ENV-18).
	payload, typeURL, err := envelope.Pack(mapped.Message)
	if err != nil {
		return err
	}

	metadata, err := encodeMetadata(entry.Context, enqueueTenant(ctx))
	if err != nil {
		return err
	}

	_, err = o.tx.conn.Exec(ctx, insertOutbox,
		string(entry.MessageID), mapped.Type, typeURL,
		entry.AggregateType, entry.AggregateID, int64(entry.AggregateVersion),
		entry.Intent.PartitionKey, entry.Intent.Destination,
		payload, payloadhash.Sum(payload), metadata,
		int64(entry.OccurredAt), int64(entry.OccurredAt))

	if isUniqueViolation(err) {
		// Dois %w: errors.Is alcança a sentinela e errors.As continua alcançando
		// *pgconn.PgError, porque a prova de OBX-01 é o SQLSTATE do schema.
		return fmt.Errorf("%w: %w", ErrDuplicateMessage, err)
	}
	return err
}

// metadataKeys are the ENV-08 attribute names the relay reads back
// (app/relay/record.go); the writer copies what the adapter authored and
// never fills a gap (OBX-02) — an absent attribute is absent, not "".
type metadataKeys struct {
	CorrelationID string `json:"correlationid,omitempty"`
	CausationID   string `json:"causationid,omitempty"`
	Traceparent   string `json:"traceparent,omitempty"`
	TenantID      string `json:"tenantid,omitempty"`
}

// encodeMetadata takes the tenant apart from the MessageContext because the two
// have different authors: the adapter writes the ENV-08 trio, authentication
// resolves the tenant (CTX-13). Absence stays absent, as OBX-02 requires.
func encodeMetadata(mc ports.MessageContext, tenant ports.TenantID) (string, error) {
	keys := metadataKeys{
		CorrelationID: mc.CorrelationID,
		CausationID:   mc.CausationID,
		Traceparent:   mc.Traceparent,
		TenantID:      string(tenant),
	}
	if keys == (metadataKeys{}) {
		return "{}", nil
	}
	encoded, err := json.Marshal(keys)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// enqueueTenant reads the scope the edge resolved, so no use case has to
// remember to copy it (IDN-14). Absence is legitimate here and is not refused:
// a platform chain has no tenant, and CTX-13 forbids inventing one.
func enqueueTenant(ctx context.Context) ports.TenantID {
	execution, ok := ports.ExecutionContextFrom(ctx)
	if !ok {
		return ""
	}
	tenant, scoped := execution.Tenant()
	if !scoped {
		return ""
	}
	return tenant
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
