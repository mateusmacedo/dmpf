CREATE TABLE IF NOT EXISTS outbox (
  id                bigint  GENERATED ALWAYS AS IDENTITY,
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
  CONSTRAINT outbox_pkey PRIMARY KEY (id),
  CONSTRAINT outbox_message_id_key UNIQUE (message_id),
  CONSTRAINT outbox_status_check
    CHECK (status IN ('pending', 'publishing', 'published', 'failed')),
  CONSTRAINT outbox_available_at_check CHECK (available_at >= occurred_at),
  CONSTRAINT outbox_payload_check CHECK (octet_length(payload) > 0)
);

CREATE INDEX IF NOT EXISTS outbox_published_at_idx
  ON outbox (published_at) WHERE status = 'published';

CREATE INDEX IF NOT EXISTS outbox_claim_idx
  ON outbox (available_at, id) WHERE status IN ('pending', 'publishing');
