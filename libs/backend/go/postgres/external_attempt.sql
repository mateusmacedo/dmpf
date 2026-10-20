CREATE TABLE IF NOT EXISTS external_attempt (
  tenant_id        text     NOT NULL,
  outbound_key     text     NOT NULL,
  attempt          integer  NOT NULL,
  message_id       text     NOT NULL,
  state            text     NOT NULL,
  started_at       bigint   NOT NULL,
  finished_at      bigint,
  class            smallint,
  finding          smallint,
  observation_id   text,
  provider         text,
  external_ref     text,
  source_at        bigint,
  terminal_finding smallint,
  observation_ids  text[]   NOT NULL DEFAULT '{}',
  queries          integer  NOT NULL DEFAULT 0,
  conflicts        jsonb    NOT NULL DEFAULT '[]',
  CONSTRAINT external_attempt_pkey PRIMARY KEY (tenant_id, outbound_key, attempt),
  CONSTRAINT external_attempt_state_check CHECK (state IN ('begun', 'finished'))
);

CREATE INDEX IF NOT EXISTS external_attempt_started_at_idx
  ON external_attempt (started_at) WHERE state = 'begun';

CREATE INDEX IF NOT EXISTS external_attempt_finished_at_idx
  ON external_attempt (finished_at) WHERE state = 'finished';
