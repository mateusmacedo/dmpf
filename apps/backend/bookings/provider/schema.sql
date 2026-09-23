CREATE TABLE IF NOT EXISTS bookings_booking (
  booking_id  text    NOT NULL PRIMARY KEY,
  version     bigint  NOT NULL DEFAULT 0,
  resource_id text    NOT NULL DEFAULT '',
  quantity    integer NOT NULL DEFAULT 0,
  status      integer NOT NULL DEFAULT 0,
  reserved_at bigint  NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS bookings_booking_resource_id_idx
  ON bookings_booking (resource_id);

CREATE TABLE IF NOT EXISTS bookings_resource (
  code          text   NOT NULL PRIMARY KEY,
  version       bigint NOT NULL DEFAULT 0,
  registered_at bigint NOT NULL DEFAULT 0
);

-- IDN-14: the tenant scope is a column of the key, not a condition each query
-- has to remember to include. Both tables were born with a global PK, and
-- CREATE TABLE IF NOT EXISTS does not alter an existing table, so the promotion
-- is explicit and idempotent. Same treatment as the kernel schema.
ALTER TABLE bookings_booking  ADD COLUMN IF NOT EXISTS tenant_id text;
ALTER TABLE bookings_resource ADD COLUMN IF NOT EXISTS tenant_id text;

-- IDN-20 forbids a synthetic tenant, so there is no backfill: the migration
-- stops rather than assigning a filler value to data whose owner is unknown.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM bookings_booking WHERE tenant_id IS NULL) THEN
    RAISE EXCEPTION 'bookings_booking has rows without tenant_id; IDN-20 forbids a synthetic backfill, so resolve each row''s tenant before migrating';
  END IF;
  IF EXISTS (SELECT 1 FROM bookings_resource WHERE tenant_id IS NULL) THEN
    RAISE EXCEPTION 'bookings_resource has rows without tenant_id; IDN-20 forbids a synthetic backfill, so resolve each row''s tenant before migrating';
  END IF;
END $$;

ALTER TABLE bookings_booking  ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE bookings_resource ALTER COLUMN tenant_id SET NOT NULL;

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint c JOIN pg_class t ON t.oid = c.conrelid
    WHERE t.relname = 'bookings_booking' AND c.contype = 'p' AND cardinality(c.conkey) = 2
  ) THEN
    ALTER TABLE bookings_booking DROP CONSTRAINT IF EXISTS bookings_booking_pkey;
    ALTER TABLE bookings_booking ADD PRIMARY KEY (tenant_id, booking_id);
  END IF;

  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint c JOIN pg_class t ON t.oid = c.conrelid
    WHERE t.relname = 'bookings_resource' AND c.contype = 'p' AND cardinality(c.conkey) = 2
  ) THEN
    ALTER TABLE bookings_resource DROP CONSTRAINT IF EXISTS bookings_resource_pkey;
    ALTER TABLE bookings_resource ADD PRIMARY KEY (tenant_id, code);
  END IF;
END $$;

-- The index behind the relation query becomes composite for the same reason the
-- PK does: with the tenant outside it, a search by resource_id scans every
-- tenant's rows before the scope predicate discards them.
DROP INDEX IF EXISTS bookings_booking_resource_id_idx;
CREATE INDEX IF NOT EXISTS bookings_booking_tenant_resource_idx
  ON bookings_booking (tenant_id, resource_id);
