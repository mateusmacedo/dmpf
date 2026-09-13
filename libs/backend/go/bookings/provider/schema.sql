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
