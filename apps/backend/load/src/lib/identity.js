const PERMISSIONS = [
  'orders:read',
  'orders:write',
  'reservations:read',
  'reservations:write',
  'bookings:read',
  'bookings:write',
];

export function tenantName(index) {
  return `load-t${index}`;
}

export function tenantFor(vu, tenants) {
  return tenantName(((vu - 1) % tenants) + 1);
}

export function bearerFor(subject, tenant) {
  return `Bearer ${JSON.stringify({ sub: subject, tenant, permissions: PERMISSIONS })}`;
}
