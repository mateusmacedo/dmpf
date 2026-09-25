-- IDN-14: the tenant scope is a column of the key, not a condition each query
-- has to remember to include. Two tenants may legitimately use the same id.
CREATE TABLE IF NOT EXISTS bookings (
  tenant_id   text    NOT NULL,
  booking_id  text    NOT NULL,
  version     bigint  NOT NULL DEFAULT 0,
  resource_id text    NOT NULL DEFAULT '',
  quantity    integer NOT NULL DEFAULT 0,
  status      integer NOT NULL DEFAULT 0,
  reserved_at bigint  NOT NULL DEFAULT 0,
  CONSTRAINT bookings_pkey PRIMARY KEY (tenant_id, booking_id)
);

-- The index behind the relation query leads with the tenant for the same reason
-- the PK does: without it, a search by resource_id scans every tenant's rows.
CREATE INDEX IF NOT EXISTS bookings_tenant_id_resource_id_idx ON bookings (tenant_id, resource_id);

-- The cross-tenant probes of the kernel Table and Relation (IDN-12) look the
-- value up without the tenant, which an index led by tenant_id cannot serve.
CREATE INDEX IF NOT EXISTS bookings_booking_id_idx ON bookings (booking_id);
CREATE INDEX IF NOT EXISTS bookings_resource_id_idx ON bookings (resource_id);

CREATE TABLE IF NOT EXISTS resources (
  tenant_id     text   NOT NULL,
  resource_id   text   NOT NULL,
  version       bigint NOT NULL DEFAULT 0,
  registered_at bigint NOT NULL DEFAULT 0,
  CONSTRAINT resources_pkey PRIMARY KEY (tenant_id, resource_id)
);

CREATE INDEX IF NOT EXISTS resources_resource_id_idx ON resources (resource_id);
