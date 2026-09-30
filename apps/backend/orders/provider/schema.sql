-- IDN-14: the tenant scope is a column of the key, not a condition each query
-- has to remember to include. Two tenants may legitimately use the same id.
CREATE TABLE IF NOT EXISTS orders (
  tenant_id text   NOT NULL,
  order_id  text   NOT NULL,
  version   bigint NOT NULL,
  snapshot  jsonb  NOT NULL,
  CONSTRAINT orders_pkey PRIMARY KEY (tenant_id, order_id)
);

-- The cross-tenant probe of the kernel Table (IDN-12) looks the id up without
-- the tenant, which the PK led by tenant_id cannot serve.
CREATE INDEX IF NOT EXISTS orders_order_id_idx ON orders (order_id);
