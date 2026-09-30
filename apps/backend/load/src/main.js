import { sleep } from 'k6';
import exec from 'k6/execution';
import http from 'k6/http';
import { bookingsJourney } from './journeys/bookings.js';
import { ordersJourney, placeOrder } from './journeys/orders.js';
import { readsJourney } from './journeys/reads.js';
import { awaitReservation, reservationsJourney } from './journeys/reservations.js';
import { assertLocalTarget } from './lib/guard.js';
import { createSession } from './lib/http.js';
import { bearerFor, tenantFor, tenantName } from './lib/identity.js';
import { idsFor, setupOrderId } from './lib/ids.js';
import { createRandom, hashSeed } from './lib/random.js';
import { renderSummary } from './lib/summary.js';
import { parseParams } from './params.js';
import { buildOptions, effectiveTenants, rateLabel, shapeFor } from './profiles.js';

const params = parseParams(__ENV);
if (!params.testid) {
  throw new Error('main: TESTID ausente; rode pelo apps/backend/load/scripts/run.sh');
}
const tenants = effectiveTenants(params);
const READY_DEADLINE_S = 60;
const SETUP_ORDERS_PER_TENANT = 10;

export const options = buildOptions(params);

function waitReady() {
  const deadline = Date.now() + READY_DEADLINE_S * 1000;
  while (Date.now() < deadline) {
    const res = http.get(`${params.baseUrl}/readyz`, {
      tags: { name: '/readyz' },
      timeout: '3s',
      responseCallback: http.expectedStatuses(204, 503),
    });
    if (res.status === 204) {
      return;
    }
    sleep(1);
  }
  exec.test.abort(`main: /readyz não respondeu 204 em ${READY_DEADLINE_S} s`);
}

function seedReadable() {
  const readable = {};
  if (params.weights.reads === 0 || params.profile === 'admission') {
    return readable;
  }
  const random = createRandom(hashSeed(params.seed, 'setup'));
  const pending = [];
  for (let index = 1; index <= tenants; index++) {
    const tenant = tenantName(index);
    const session = createSession({
      baseUrl: params.baseUrl,
      bearer: bearerFor('load-setup', tenant),
      correlation: `load-${params.testid}-setup-${index}`,
    });
    readable[tenant] = [];
    for (let sequence = 1; sequence <= SETUP_ORDERS_PER_TENANT; sequence++) {
      const order = setupOrderId(params.testid, index, sequence);
      if (!placeOrder(session, random, order, 1)) {
        throw new Error(`main: o setup não criou o pedido ${order}`);
      }
      readable[tenant].push(order);
      pending.push({ session, order });
    }
  }
  for (const { session, order } of pending) {
    if (!awaitReservation(session, order, params.convergenceTimeoutMs, { record: false })) {
      throw new Error(`main: a reserva de ${order} não convergiu no setup`);
    }
  }
  return readable;
}

export function setup() {
  assertLocalTarget(params.baseUrl);
  waitReady();
  return { readable: seedReadable() };
}

function context(data) {
  const vu = exec.vu.idInTest;
  const iteration = exec.vu.iterationInScenario;
  const tenant = tenantFor(vu, tenants);
  const ids = idsFor(params.testid, vu, iteration);
  return {
    params,
    ids,
    readable: data?.readable?.[tenant] ?? [],
    random: createRandom(hashSeed(params.seed, exec.scenario.name, vu, iteration)),
    session: createSession({
      baseUrl: params.baseUrl,
      bearer: bearerFor(`load-vu-${vu}`, tenant),
      correlation: ids.correlation,
    }),
  };
}

export function orders(data) {
  ordersJourney(context(data));
}

export function reservations(data) {
  reservationsJourney(context(data));
}

export function bookings(data) {
  bookingsJourney(context(data));
}

export function reads(data) {
  readsJourney(context(data));
}

export function admission(data) {
  context(data).session.send({
    method: 'GET',
    name: '/bookings/booking',
    path: '/bookings/booking?resourceId=load-admission',
    expected: [200],
    kind: 'read',
  });
}

export function handleSummary(data) {
  const markdown = renderSummary(data, {
    params,
    tenants,
    shape: shapeFor(params.profile, params.duration),
    rate: rateLabel(params),
  });
  const directory = `/results/${params.testid}`;
  return {
    stdout: markdown,
    [`${directory}/summary.json`]: JSON.stringify(data, null, 2),
    [`${directory}/summary.md`]: markdown,
  };
}
