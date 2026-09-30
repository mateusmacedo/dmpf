import { SUMMARY_SUBMETRICS } from './lib/summary.js';
import { JOURNEYS, MAX_TENANTS } from './params.js';

export const SYSTEM_TAGS = [
  'status',
  'method',
  'name',
  'group',
  'check',
  'error_code',
  'scenario',
  'expected_response',
];

const ADMISSION_RATE = 80;
const MAX_PREALLOCATED_VUS = 1200;
const ITERATION_OVERHEAD_S = 2;
const PREALLOCATION_S = 4;

const abortOnFail = (threshold) => ({ threshold, abortOnFail: true, delayAbortEval: '30s' });

const steady = () => ({
  http_req_failed: ['rate<0.01'],
  'http_req_duration{kind:read}': ['p(95)<300', 'p(99)<800'],
  'http_req_duration{kind:write}': ['p(95)<500', 'p(99)<1500'],
  checks: ['rate>0.99'],
  admission_rejections: ['rate<0.001'],
  reservation_convergence: ['p(95)<5000'],
  reservation_convergence_timeouts: ['count==0'],
  dropped_iterations: ['count==0'],
});

const THRESHOLDS = {
  smoke: () => ({
    http_req_failed: ['rate==0'],
    checks: ['rate==1'],
    admission_rejections: ['rate==0'],
  }),
  average: steady,
  soak: steady,
  stress: () => ({
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(99)<2000'],
    checks: ['rate>0.95'],
  }),
  spike: () => ({ http_req_failed: ['rate<0.10'] }),
  breakpoint: () => ({
    http_req_failed: [abortOnFail('rate<0.05')],
    http_req_duration: [abortOnFail('p(99)<2000')],
    admission_rejections: [abortOnFail('rate<0.10')],
  }),
  admission: () => ({
    admission_rejections: ['rate>0.30', 'rate<0.45'],
    http_req_failed: ['rate==0'],
    checks: ['rate==1'],
  }),
};

const stage = (duration, multiplier) => ({ duration, multiplier });

export function shapeFor(profile, plateau) {
  switch (profile) {
    case 'average':
      return {
        startMultiplier: 0,
        stages: [stage('2m', 1), stage(plateau ?? '10m', 1), stage('1m', 0)],
      };
    case 'stress':
      return {
        startMultiplier: 0,
        stages: [
          stage('2m', 2),
          stage(plateau ?? '10m', 2),
          stage('2m', 3),
          stage(plateau ?? '5m', 3),
          stage('1m', 0),
        ],
      };
    case 'spike':
      return {
        startMultiplier: 1,
        stages: [stage('1m', 1), stage('30s', 8), stage('1m', 8), stage('30s', 1), stage('2m', 1)],
      };
    case 'breakpoint':
      return { startMultiplier: 0, stages: [stage('30m', 30)] };
    default:
      return null;
  }
}

export function effectiveTenants(params) {
  if (params.profile === 'breakpoint') {
    return MAX_TENANTS;
  }
  if (params.profile === 'admission') {
    return 1;
  }
  return params.tenants;
}

function vuAllocation(peakPerSecond, convergenceTimeoutMs) {
  const preAllocatedVUs = Math.max(1, Math.ceil(peakPerSecond * PREALLOCATION_S));
  const iterationCeilingS = convergenceTimeoutMs / 1000 + ITERATION_OVERHEAD_S;
  return {
    preAllocatedVUs,
    maxVUs: Math.max(preAllocatedVUs, Math.ceil(peakPerSecond * iterationCeilingS)),
  };
}

function journeyScenario(params, journey) {
  const share = (params.rate * params.weights[journey]) / 100;
  const perMinute = (multiplier) => Math.round(60 * share * multiplier);
  const base = { exec: journey, tags: { journey } };

  if (params.profile === 'smoke') {
    return { ...base, executor: 'constant-vus', vus: 1, duration: '1m' };
  }
  if (params.profile === 'soak') {
    return {
      ...base,
      executor: 'constant-arrival-rate',
      rate: perMinute(1),
      timeUnit: '1m',
      duration: params.duration ?? '60m',
      ...vuAllocation(share, params.convergenceTimeoutMs),
    };
  }
  const shape = shapeFor(params.profile, params.duration);
  const peak = Math.max(shape.startMultiplier, ...shape.stages.map((s) => s.multiplier));
  return {
    ...base,
    executor: 'ramping-arrival-rate',
    timeUnit: '1m',
    startRate: perMinute(shape.startMultiplier),
    stages: shape.stages.map((s) => ({ duration: s.duration, target: perMinute(s.multiplier) })),
    ...vuAllocation(share * peak, params.convergenceTimeoutMs),
  };
}

function scenarios(params) {
  if (params.profile === 'admission') {
    return {
      admission: {
        executor: 'constant-arrival-rate',
        exec: 'admission',
        tags: { journey: 'admission' },
        rate: ADMISSION_RATE,
        timeUnit: '1s',
        duration: params.duration ?? '2m',
        ...vuAllocation(ADMISSION_RATE, 0),
      },
    };
  }
  const built = {};
  for (const journey of JOURNEYS) {
    if (params.weights[journey] > 0) {
      built[journey] = journeyScenario(params, journey);
    }
  }
  return built;
}

export function rateLabel(params) {
  return params.profile === 'admission'
    ? `taxa fixa de ${ADMISSION_RATE} req/s`
    : `RATE=${params.rate}`;
}

function assertVuBudget(params, built) {
  const preallocated = Object.values(built).reduce(
    (total, scenario) => total + (scenario.preAllocatedVUs ?? scenario.vus ?? 0),
    0,
  );
  if (preallocated > MAX_PREALLOCATED_VUS) {
    throw new Error(
      `profiles: o perfil ${params.profile} com RATE=${params.rate} pré-aloca ${preallocated} VUs, acima do teto de ${MAX_PREALLOCATED_VUS} que cabe no 1 GiB do container k6; reduza RATE`,
    );
  }
}

export function buildOptions(params) {
  const built = scenarios(params);
  assertVuBudget(params, built);
  return {
    scenarios: built,
    thresholds: { ...SUMMARY_SUBMETRICS, ...THRESHOLDS[params.profile]() },
    systemTags: SYSTEM_TAGS,
    setupTimeout: '120s',
    summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(90)', 'p(95)', 'p(99)'],
  };
}
