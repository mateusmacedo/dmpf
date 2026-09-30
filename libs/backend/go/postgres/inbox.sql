CREATE TABLE IF NOT EXISTS inbox (
  consumer_name text   NOT NULL,
  message_id    text   NOT NULL,
  message_type  text   NOT NULL,
  payload_hash  text   NOT NULL,
  received_at   bigint NOT NULL,
  processed_at  bigint NOT NULL,
  status        text   NOT NULL,
  last_error    text,
  outcome       bytea,
  expires_at    bigint,
  CONSTRAINT inbox_consumer_name_message_id_key UNIQUE (consumer_name, message_id),
  CONSTRAINT inbox_status_check CHECK (status IN ('processed', 'rejected'))
);

CREATE INDEX IF NOT EXISTS inbox_retention_idx ON inbox (consumer_name, processed_at);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid = 'inbox'::regclass AND attname = 'outcome' AND NOT attisdropped) THEN
    ALTER TABLE inbox ADD COLUMN outcome bytea;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid = 'inbox'::regclass AND attname = 'expires_at' AND NOT attisdropped) THEN
    ALTER TABLE inbox ADD COLUMN expires_at bigint;
  END IF;
  IF to_regclass('inbox_consumer_name_expires_at_idx') IS NULL THEN
    CREATE INDEX inbox_consumer_name_expires_at_idx
      ON inbox (consumer_name, expires_at) WHERE expires_at IS NOT NULL;
  END IF;
END
$$;

CREATE TABLE IF NOT EXISTS quarantine (
  id            bigint GENERATED ALWAYS AS IDENTITY,
  consumer_name text   NOT NULL,
  message_id    text   NOT NULL,
  reason        text   NOT NULL,
  envelope      bytea  NOT NULL,
  envelope_digest bytea,
  last_error    text,
  contained_at  bigint NOT NULL,
  CONSTRAINT quarantine_pkey PRIMARY KEY (id),
  CONSTRAINT quarantine_envelope_check CHECK (octet_length(envelope) > 0)
);

CREATE INDEX IF NOT EXISTS quarantine_consumer_name_reason_idx ON quarantine (consumer_name, reason);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid = 'quarantine'::regclass AND attname = 'envelope_digest' AND NOT attisdropped) THEN
    ALTER TABLE quarantine ADD COLUMN envelope_digest bytea;
    UPDATE quarantine SET envelope_digest = sha256(envelope);
    DELETE FROM quarantine newer USING quarantine older
     WHERE newer.consumer_name = older.consumer_name
       AND newer.envelope_digest = older.envelope_digest
       AND newer.id > older.id;
  END IF;
  IF to_regclass('quarantine_consumer_name_envelope_digest_idx') IS NULL THEN
    CREATE UNIQUE INDEX quarantine_consumer_name_envelope_digest_idx
      ON quarantine (consumer_name, envelope_digest);
  END IF;
END
$$;
