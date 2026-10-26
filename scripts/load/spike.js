import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

export const spikeLatency = new Trend('spike_latency_ms', true);
export const serverErrors = new Rate('server_errors_rate');
export const timeoutErrors = new Rate('timeout_errors_rate');
export const totalRequests = new Counter('total_requests');

export const options = {
  stages: [
    { duration: '30s', target: 15 },  // baseline normal traffic
    { duration: '15s', target: 60 },  // 4x sudden spike
    { duration: '1m',  target: 60 },  // hold spike
    { duration: '30s', target: 15 },  // ramp down to baseline
    { duration: '15s', target: 0 },   // cool off
  ],
  thresholds: {
    // Under 4x spike, 5xx server crash errors must be bounded < 1%
    'server_errors_rate': ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8000';

export function setup() {
  const email = __ENV.ADMIN_EMAIL || 'admin@transitops.com';
  const password = __ENV.ADMIN_PASSWORD || '';

  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    email: email,
    password: password,
  }), {
    headers: { 'Content-Type': 'application/json' },
  });

  check(loginRes, {
    'setup login successful': (r) => r.status === 200,
  });

  let cookie = '';
  if (loginRes.headers['Set-Cookie']) {
    cookie = loginRes.headers['Set-Cookie'];
  }

  return { cookie };
}

export default function (data) {
  const headers = {
    'Content-Type': 'application/json',
    'Cookie': data.cookie,
  };

  group('Spike Traffic Mix', () => {
    // Readiness probe check (verifies service health under load)
    const t0 = Date.now();
    const readyRes = http.get(`${BASE_URL}/readyz`);
    const readyDur = Date.now() - t0;
    spikeLatency.add(readyDur);
    totalRequests.add(1);

    if (readyRes.status >= 500 && readyRes.status !== 503) {
      serverErrors.add(1);
    } else {
      serverErrors.add(0);
    }

    // Dashboard KPI query
    const t1 = Date.now();
    const kpiRes = http.get(`${BASE_URL}/api/v1/dashboard/kpis`, { headers });
    const kpiDur = Date.now() - t1;
    spikeLatency.add(kpiDur);
    totalRequests.add(1);

    if (kpiRes.status === 504) {
      timeoutErrors.add(1);
    } else if (kpiRes.status >= 500) {
      serverErrors.add(1);
    } else {
      serverErrors.add(0);
    }

    // Fleet list
    const t2 = Date.now();
    const vRes = http.get(`${BASE_URL}/api/v1/fleet/vehicles`, { headers });
    const vDur = Date.now() - t2;
    spikeLatency.add(vDur);
    totalRequests.add(1);

    if (vRes.status >= 500 && vRes.status !== 504) {
      serverErrors.add(1);
    } else {
      serverErrors.add(0);
    }
  });

  sleep(0.2);
}
