import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Counter, Rate, Trend } from 'k6/metrics';

// Custom metrics aligned with Phase 5 SLOs
export const readLatency = new Trend('read_latency_ms', true);
export const writeLatency = new Trend('write_latency_ms', true);
export const failedRequests = new Rate('failed_requests_rate');
export const totalRequests = new Counter('total_requests');

export const options = {
  stages: [
    { duration: '1m', target: 20 },   // Warm up ramp
    { duration: '8m', target: 20 },   // Steady state
    { duration: '1m', target: 0 },    // Ramp down
  ],
  thresholds: {
    'read_latency_ms': ['p(95)<300'],     // SLO: p95 read < 300ms
    'write_latency_ms': ['p(95)<500'],    // SLO: p95 write < 500ms
    'failed_requests_rate': ['rate<0.005'],// SLO: availability >= 99.5% (5xx < 0.5%)
  },
};

function uuidv4() {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function(c) {
    const r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
    return v.toString(16);
  });
}

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

  // Extract session cookie or bearer token if returned
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

  group('Read Operations (Vehicles, Drivers, KPIs)', () => {
    // 1. Get vehicles list
    const t0 = Date.now();
    const vRes = http.get(`${BASE_URL}/api/v1/fleet/vehicles`, { headers });
    const vDur = Date.now() - t0;
    readLatency.add(vDur);
    totalRequests.add(1);
    failedRequests.add(vRes.status >= 500 ? 1 : 0);
    check(vRes, { 'vehicles 200': (r) => r.status === 200 });

    // 2. Get dashboard KPIs
    const t1 = Date.now();
    const kpiRes = http.get(`${BASE_URL}/api/v1/dashboard/kpis`, { headers });
    const kpiDur = Date.now() - t1;
    readLatency.add(kpiDur);
    totalRequests.add(1);
    failedRequests.add(kpiRes.status >= 500 ? 1 : 0);
    check(kpiRes, { 'kpis 200': (r) => r.status === 200 });

    // 3. Request realtime ticket
    const t2 = Date.now();
    const ticketRes = http.post(`${BASE_URL}/api/v1/realtime/ticket`, null, { headers });
    const ticketDur = Date.now() - t2;
    readLatency.add(ticketDur);
    totalRequests.add(1);
    failedRequests.add(ticketRes.status >= 500 ? 1 : 0);
    check(ticketRes, { 'ticket 200': (r) => r.status === 200 });
  });

  sleep(0.5);

  group('Write Operations (Draft Trip & Cancellation)', () => {
    // Unique UUID idempotency keys
    const idemKeyDraft = uuidv4();
    const draftPayload = JSON.stringify({
      vehicle_reg: 'VH-1001',
      driver_license: 'DL-1001',
      source_name: 'New York Central Hub',
      destination_name: 'Chicago Logistics Hub',
      cargo_weight: 1200,
    });

    const t3 = Date.now();
    const draftRes = http.post(`${BASE_URL}/api/v1/trips`, draftPayload, {
      headers: Object.assign({}, headers, { 'Idempotency-Key': idemKeyDraft }),
    });
    const draftDur = Date.now() - t3;
    writeLatency.add(draftDur);
    totalRequests.add(1);
    failedRequests.add(draftRes.status >= 500 ? 1 : 0);

    let tripId = null;
    if (draftRes.status === 201 || draftRes.status === 200) {
      try {
        const body = JSON.parse(draftRes.body);
        tripId = body.id || (body.trip && body.trip.id);
      } catch (_) {}
    }

    // Cancel trip if created to clean up
    if (tripId) {
      const idemKeyCancel = uuidv4();
      const t4 = Date.now();
      const cancelRes = http.post(`${BASE_URL}/api/v1/trips/${tripId}/cancel`, JSON.stringify({
        reason: 'Load test cleanup',
      }), {
        headers: Object.assign({}, headers, { 'Idempotency-Key': idemKeyCancel }),
      });
      const cancelDur = Date.now() - t4;
      writeLatency.add(cancelDur);
      totalRequests.add(1);
      failedRequests.add(cancelRes.status >= 500 ? 1 : 0);
    }
  });

  sleep(1);
}
