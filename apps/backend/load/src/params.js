import { assertLocalTarget } from './lib/guard.js';

export const PROFILES = ['smoke', 'average', 'stress', 'spike', 'soak', 'breakpoint', 'admission'];
export const JOURNEYS = ['orders', 'reservations', 'bookings', 'reads'];
export const MAX_TENANTS = 5;

const DEFAULTS = {
  RATE: '10',
  TENANTS: '4',
  WEIGHTS: 'orders=30,reservations=25,bookings=35,reads=10',
  CANCEL_RATIO: '0.2',
  RESERVE_RATIO: '0.2',
  CONVERGENCE_TIMEOUT: '10s',
  SEED: '1',
  BASE_URL: 'http://dmpf-bff:8080',
};

function fail(name, rule, value) {
  throw new Error(`params: ${name} ${rule} (recebido "${value}")`);
}

function read(env, name) {
  const value = env[name];
  return value === undefined || value === '' ? DEFAULTS[name] : value;
}

function integer(env, name, min, max) {
  const raw = read(env, name);
  const value = Number(raw);
  if (!/^\d+$/.test(raw) || value < min || value > max) {
    fail(name, `deve ser inteiro de ${min} a ${max}`, raw);
  }
  return value;
}

function ratio(env, name) {
  const raw = read(env, name);
  const value = Number(raw);
  if (!/^\d+(\.\d+)?$/.test(raw) || value > 1) {
    fail(name, 'deve ser número de 0 a 1', raw);
  }
  return value;
}

function weights(env) {
  const raw = read(env, 'WEIGHTS');
  const parsed = {};
  for (const pair of raw.split(',')) {
    const match = /^([a-z]+)=(\d{1,3})$/.exec(pair.trim());
    if (!match || !JOURNEYS.includes(match[1]) || match[1] in parsed) {
      fail('WEIGHTS', `deve listar ${JOURNEYS.join(', ')} uma vez cada, como nome=peso`, raw);
    }
    parsed[match[1]] = Number(match[2]);
  }
  const sum = JOURNEYS.reduce((total, journey) => total + (parsed[journey] ?? Number.NaN), 0);
  if (sum !== 100) {
    fail('WEIGHTS', 'deve somar 100 com as quatro jornadas', raw);
  }
  return parsed;
}

function profile(env) {
  const raw = env.PROFILE ?? '';
  if (!PROFILES.includes(raw)) {
    fail('PROFILE', `deve ser um de ${PROFILES.join(', ')}`, raw);
  }
  return raw;
}

function optional(env, name, format, rule) {
  const raw = env[name];
  if (raw === undefined || raw === '') {
    return null;
  }
  if (!format.test(raw)) {
    fail(name, rule, raw);
  }
  return raw;
}

function convergenceTimeoutMs(env) {
  const raw = read(env, 'CONVERGENCE_TIMEOUT');
  if (!/^[1-9]\d*s$/.test(raw)) {
    fail('CONVERGENCE_TIMEOUT', 'deve ser segundos inteiros, como 10s', raw);
  }
  return Number(raw.slice(0, -1)) * 1000;
}

function baseUrl(env) {
  try {
    return assertLocalTarget(read(env, 'BASE_URL'));
  } catch (error) {
    throw new Error(`params: ${error.message}`);
  }
}

function ratios(env) {
  const cancelRatio = ratio(env, 'CANCEL_RATIO');
  const reserveRatio = ratio(env, 'RESERVE_RATIO');
  if (cancelRatio + reserveRatio > 1) {
    fail(
      'CANCEL_RATIO',
      'somado a RESERVE_RATIO não pode passar de 1',
      `${cancelRatio} + ${reserveRatio}`,
    );
  }
  return { cancelRatio, reserveRatio };
}

export function parseParams(env) {
  return Object.freeze({
    profile: profile(env),
    testid: optional(env, 'TESTID', /^[A-Za-z0-9._-]{1,64}$/, 'deve casar ^[A-Za-z0-9._-]{1,64}$'),
    rate: integer(env, 'RATE', 1, 1000),
    duration: optional(env, 'DURATION', /^[1-9]\d*[smh]$/, 'deve ser duração como 30s, 10m ou 1h'),
    tenants: integer(env, 'TENANTS', 1, MAX_TENANTS),
    weights: Object.freeze(weights(env)),
    ...ratios(env),
    convergenceTimeoutMs: convergenceTimeoutMs(env),
    seed: integer(env, 'SEED', 0, 2147483647),
    baseUrl: baseUrl(env),
  });
}
