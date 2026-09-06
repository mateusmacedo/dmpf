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

CREATE INDEX IF NOT EXISTS dmpf_outbox_claim_idx
  ON dmpf_outbox (available_at, id) WHERE status IN ('pending', 'publishing');

CREATE TABLE IF NOT EXISTS dmpf_inbox (
  consumer_name text   NOT NULL,
  message_id    text   NOT NULL,
  message_type  text   NOT NULL,
  payload_hash  text   NOT NULL,
  received_at   bigint NOT NULL,
  processed_at  bigint NOT NULL,
  status        text   NOT NULL,
  last_error    text,
  CONSTRAINT dmpf_inbox_key UNIQUE (consumer_name, message_id),
  CONSTRAINT dmpf_inbox_status_check CHECK (status IN ('processed', 'rejected'))
);

CREATE INDEX IF NOT EXISTS dmpf_inbox_retention_idx ON dmpf_inbox (consumer_name, processed_at);

CREATE TABLE IF NOT EXISTS dmpf_quarantine (
  id            bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  consumer_name text   NOT NULL,
  message_id    text   NOT NULL,
  reason        text   NOT NULL,
  envelope      bytea  NOT NULL,
  last_error    text,
  contained_at  bigint NOT NULL,
  CONSTRAINT dmpf_quarantine_envelope_not_empty CHECK (octet_length(envelope) > 0)
);

CREATE INDEX IF NOT EXISTS dmpf_quarantine_reason_idx ON dmpf_quarantine (consumer_name, reason);

CREATE TABLE IF NOT EXISTS dmpf_example_orders (
  order_id text   PRIMARY KEY,
  version  bigint NOT NULL,
  snapshot jsonb  NOT NULL
);

CREATE TABLE IF NOT EXISTS dmpf_example_reservations (
  order_id text   PRIMARY KEY,
  version  bigint NOT NULL,
  snapshot jsonb  NOT NULL
);
