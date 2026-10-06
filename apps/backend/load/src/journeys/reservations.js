import { check, sleep } from 'k6';
import { reservationConvergence, reservationConvergenceTimeouts } from '../lib/metrics.js';
import { placeOrder } from './orders.js';

const POLL_INTERVAL_S = 0.25;

export function awaitReservation(session, order, timeoutMs, { record = true } = {}) {
  const started = Date.now();
  while (Date.now() - started < timeoutMs) {
    const res = session.send({
      method: 'GET',
      name: '/reservations/{order_id}',
      path: `/reservations/${order}`,
      expected: [200, 404],
      kind: 'poll',
    });
    if (!res) {
      return null;
    }
    if (res.status === 200) {
      if (record) {
        reservationConvergence.add(Date.now() - started);
      }
      return res;
    }
    sleep(POLL_INTERVAL_S);
  }
  if (record) {
    reservationConvergenceTimeouts.add(1);
  }
  return null;
}

function reserveDirectly(session, random, order) {
  const reserved = session.send({
    method: 'POST',
    name: '/reservations/{order_id}/reserve',
    path: `/reservations/${order}/reserve`,
    body: { items: random.int(1, 3) },
    expected: [200],
    kind: 'write',
  });
  if (!reserved) {
    return false;
  }
  const res = session.send({
    method: 'GET',
    name: '/reservations/{order_id}',
    path: `/reservations/${order}`,
    expected: [200],
    kind: 'read',
  });
  return (
    Boolean(res) && check(res, { 'reserva confirmada': (r) => r.json('status') === 'confirmed' })
  );
}

function reserveByEvent(session, random, order, timeoutMs) {
  return (
    placeOrder(session, random, order, 1) && Boolean(awaitReservation(session, order, timeoutMs))
  );
}

function cancelBeforeDecision(session, random, order) {
  const cancelled = session.send({
    method: 'POST',
    name: '/reservations/{order_id}/cancel',
    path: `/reservations/${order}/cancel`,
    expected: [200],
    kind: 'write',
  });
  if (!cancelled || !placeOrder(session, random, order, 1)) {
    return;
  }
  const res = session.send({
    method: 'GET',
    name: '/reservations/{order_id}',
    path: `/reservations/${order}`,
    expected: [200],
    kind: 'read',
  });
  if (res) {
    check(res, { 'reserva segue cancelada após o pedido': (r) => r.json('status') === 'canceled' });
  }
}

export function reservationsJourney({ session, random, ids, params }) {
  const draw = random.fraction();
  if (draw < params.cancelRatio) {
    cancelBeforeDecision(session, random, ids.order);
  } else if (draw < params.cancelRatio + params.reserveRatio) {
    reserveDirectly(session, random, ids.order);
  } else {
    reserveByEvent(session, random, ids.order, params.convergenceTimeoutMs);
  }
}
