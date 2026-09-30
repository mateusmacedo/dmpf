import { check } from 'k6';
import http from 'k6/http';
import { admissionRejections, contextAdmissionRejections } from './metrics.js';

const EDGE_REFUSAL_PREFIX = 'admission refused:';
const REQUEST_TIMEOUT = '10s';

http.setResponseCallback(http.expectedStatuses({ min: 200, max: 499 }));

function recordRefusal(res, name) {
  admissionRejections.add(true, { name });
  let message = '';
  try {
    message = String(res.json('message') ?? '');
  } catch {
    message = '';
  }
  if (message.startsWith(EDGE_REFUSAL_PREFIX)) {
    check(res, {
      'recusa de admissão da borda com Retry-After: 1': (r) => r.headers['Retry-After'] === '1',
    });
  } else {
    contextAdmissionRejections.add(1, { name });
  }
}

export function createSession({ baseUrl, bearer, correlation }) {
  return {
    send({ method, name, path, body, expected, kind }) {
      const headers = { Authorization: bearer, 'X-Correlation-ID': correlation };
      if (method === 'POST') {
        headers['Idempotency-Key'] = crypto.randomUUID();
      }
      if (body !== undefined) {
        headers['Content-Type'] = 'application/json';
      }
      const payload = body === undefined ? null : JSON.stringify(body);
      const res = http.request(method, `${baseUrl}${path}`, payload, {
        headers,
        tags: { name, kind },
        timeout: REQUEST_TIMEOUT,
      });
      if (res.status === 429) {
        recordRefusal(res, name);
        return null;
      }
      admissionRejections.add(false, { name });
      const ok = check(res, {
        [`${method} ${name} responde ${expected.join(' ou ')}`]: (r) => expected.includes(r.status),
      });
      return ok ? res : null;
    },
  };
}
