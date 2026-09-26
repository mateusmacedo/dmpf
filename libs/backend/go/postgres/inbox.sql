CREATE TABLE IF NOT EXISTS inbox (
  consumer_name text   NOT NULL,
  message_id    text   NOT NULL,
  message_type  text   NOT NULL,
  payload_hash  text   NOT NULL,
  received_at   bigint NOT NULL,
  processed_at  bigint NOT NULL,
  status        text   NOT NULL,
  last_error    text,
  CONSTRAINT inbox_consumer_name_message_id_key UNIQUE (consumer_name, message_id),
  CONSTRAINT inbox_status_check CHECK (status IN ('processed', 'rejected'))
);

CREATE INDEX IF NOT EXISTS inbox_retention_idx ON inbox (consumer_name, processed_at);

CREATE TABLE IF NOT EXISTS quarantine (
  id            bigint GENERATED ALWAYS AS IDENTITY,
  consumer_name text   NOT NULL,
  message_id    text   NOT NULL,
  reason        text   NOT NULL,
  envelope      bytea  NOT NULL,
  last_error    text,
  contained_at  bigint NOT NULL,
  CONSTRAINT quarantine_pkey PRIMARY KEY (id),
  CONSTRAINT quarantine_envelope_check CHECK (octet_length(envelope) > 0)
);

CREATE INDEX IF NOT EXISTS quarantine_consumer_name_reason_idx ON quarantine (consumer_name, reason);
