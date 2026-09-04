package dmpfpostgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/envelope"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-contracts/payloadhash"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
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
func (t *Tx) Outbox(mapper EventMapper) dmpfports.Outbox {
	return txOutbox{tx: t, mapper: mapper}
}

type txOutbox struct {
	tx     *Tx
	mapper EventMapper
}

func (o txOutbox) Enqueue(ctx context.Context, entry dmpfports.OutboxEntry) error {
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

	// metadata vazio, nunca nulo: o conteúdo do contexto de mensagem é de FND-07,
	// e inventá-lo aqui seria o provider autorando o que não lhe pertence (OBX-02).
	_, err = o.tx.conn.Exec(ctx, insertOutbox,
		string(entry.MessageID), mapped.Type, typeURL,
		entry.AggregateType, entry.AggregateID, int64(entry.AggregateVersion),
		entry.Intent.PartitionKey, entry.Intent.Destination,
		payload, payloadhash.Sum(payload), "{}",
		int64(entry.OccurredAt), int64(entry.OccurredAt))

	if isUniqueViolation(err) {
		// Dois %w: errors.Is alcança a sentinela e errors.As continua alcançando
		// *pgconn.PgError, porque a prova de OBX-01 é o SQLSTATE do schema.
		return fmt.Errorf("%w: %w", ErrDuplicateMessage, err)
	}
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == uniqueViolation
}
