import { sleep } from 'k6';

export function readsJourney({ session, random, readable, pause }) {
  if (pause > 0) {
    sleep(pause);
  }
  const order = readable[random.int(0, readable.length - 1)];
  const found = session.send({
    method: 'GET',
    name: '/orders/{id}',
    path: `/orders/${order}`,
    expected: [200],
    kind: 'read',
  });
  if (!found) {
    return;
  }
  session.send({
    method: 'GET',
    name: '/reservations/{order_id}',
    path: `/reservations/${order}`,
    expected: [200],
    kind: 'read',
  });
}
