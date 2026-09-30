import { check } from 'k6';

export function placeOrder(session, random, order, items) {
  for (let item = 1; item <= items; item++) {
    const added = session.send({
      method: 'POST',
      name: '/orders/{id}/items',
      path: `/orders/${order}/items`,
      body: { sku: `sku-${random.int(1, 50)}`, quantity: random.int(1, 5) },
      expected: [201],
      kind: 'write',
    });
    if (!added) {
      return false;
    }
  }
  return Boolean(
    session.send({
      method: 'POST',
      name: '/orders/{id}/place',
      path: `/orders/${order}/place`,
      expected: [200],
      kind: 'write',
    }),
  );
}

export function ordersJourney({ session, random, ids }) {
  if (!placeOrder(session, random, ids.order, random.int(1, 3))) {
    return;
  }
  const res = session.send({
    method: 'GET',
    name: '/orders/{id}',
    path: `/orders/${ids.order}`,
    expected: [200],
    kind: 'read',
  });
  if (res) {
    check(res, { 'pedido colocado': (r) => r.json('status') === 'placed' });
  }
}
