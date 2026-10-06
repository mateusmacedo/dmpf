import { check } from 'k6';

export function bookingsJourney({ session, random, ids, params }) {
  const registered = session.send({
    method: 'POST',
    name: '/bookings/resource',
    path: '/bookings/resource',
    body: { code: ids.resource },
    expected: [201],
    kind: 'write',
  });
  if (!registered) {
    return;
  }
  const reserved = session.send({
    method: 'POST',
    name: '/bookings/booking',
    path: '/bookings/booking',
    body: { bookingId: ids.booking, resourceId: ids.resource, quantity: random.int(1, 5) },
    expected: [201],
    kind: 'write',
  });
  if (!reserved) {
    return;
  }
  const found = session.send({
    method: 'GET',
    name: '/bookings/booking/{id}',
    path: `/bookings/booking/${ids.booking}`,
    expected: [200],
    kind: 'read',
  });
  if (!found || !check(found, { 'booking reservado': (r) => r.json('status') === 'reserved' })) {
    return;
  }
  const listed = session.send({
    method: 'GET',
    name: '/bookings/booking',
    path: `/bookings/booking?resourceId=${ids.resource}`,
    expected: [200],
    kind: 'read',
  });
  if (!listed || !check(listed, { 'listagem com um booking': (r) => r.json().length === 1 })) {
    return;
  }
  if (random.chance(params.cancelRatio)) {
    session.send({
      method: 'POST',
      name: '/bookings/booking/{id}/cancel',
      path: `/bookings/booking/${ids.booking}/cancel`,
      expected: [200],
      kind: 'write',
    });
  }
}
