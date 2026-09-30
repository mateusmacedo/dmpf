-- IDN-14: the tenant scope is a column of the key, not a condition each query
-- has to remember to include. The reservation's natural key is its order.
CREATE TABLE IF NOT EXISTS reservations (
  tenant_id text   NOT NULL,
  order_id  text   NOT NULL,
  version   bigint NOT NULL,
  snapshot  jsonb  NOT NULL,
  CONSTRAINT reservations_pkey PRIMARY KEY (tenant_id, order_id)
);

-- The cross-tenant probe of the kernel Table (IDN-12) looks the id up without
-- the tenant, which the PK led by tenant_id cannot serve.
CREATE INDEX IF NOT EXISTS reservations_order_id_idx ON reservations (order_id);
