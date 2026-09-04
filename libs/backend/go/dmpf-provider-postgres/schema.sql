CREATE TABLE IF NOT EXISTS dmpf_outbox (
  id                bigint  GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  message_id        text    NOT NULL,
  message_type      text    NOT NULL,
  schema_version    text    NOT NULL,
  aggregate_type    text    NOT NULL,
  aggregate_id      text    NOT NULL,
  aggregate_version bigint  NOT NULL,
  partition_key     text    NOT NULL DEFAULT '',
  destination       text    NOT NULL,
  payload           bytea   NOT NULL,
  payload_hash      text    NOT NULL,
  metadata          jsonb   NOT NULL DEFAULT '{}'::jsonb,
  occurred_at       bigint  NOT NULL,
  available_at      bigint  NOT NULL,
  attempt_count     integer NOT NULL DEFAULT 0,
  status            text    NOT NULL DEFAULT 'pending',
  locked_by         text,
  locked_until      bigint,
  published_at      bigint,
  last_error        text,
  CONSTRAINT dmpf_outbox_message_id_unique UNIQUE (message_id),
  CONSTRAINT dmpf_outbox_status_check
    CHECK (status IN ('pending', 'publishing', 'published', 'failed')),
  CONSTRAINT dmpf_outbox_available_at_check CHECK (available_at >= occurred_at),
  CONSTRAINT dmpf_outbox_payload_not_empty CHECK (octet_length(payload) > 0)
);

CREATE INDEX IF NOT EXISTS dmpf_outbox_published_at_idx
  ON dmpf_outbox (published_at) WHERE status = 'published';

CREATE TABLE IF NOT EXISTS dmpf_example_orders (
  order_id text   PRIMARY KEY,
  version  bigint NOT NULL,
  snapshot jsonb  NOT NULL
);
