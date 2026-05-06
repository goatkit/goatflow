import http from 'k6/http';
import { check, sleep } from 'k6';

const baseURL = (__ENV.BASE_URL || 'http://localhost:18081').replace(/\/+$/, '');
const endpoints = parseEndpoints(__ENV.LOAD_ENDPOINTS);
const maxErrorRate = __ENV.GF_LOAD_MAX_ERROR_RATE || '0.01';
const p95Limit = __ENV.GF_LOAD_P95_LIMIT || '750';
const p99Limit = __ENV.GF_LOAD_P99_LIMIT || '1500';

export const options = {
  scenarios: {
    smoke: {
      executor: 'constant-vus',
      vus: Number(__ENV.GF_LOAD_VUS || '10'),
      duration: __ENV.GF_LOAD_DURATION || '1m',
    },
  },
  thresholds: {
    http_req_failed: [`rate<${maxErrorRate}`],
    http_req_duration: [`p(95)<${p95Limit}`, `p(99)<${p99Limit}`],
  },
};

export default function () {
  const endpoint = chooseEndpoint();
  const response = http.request(
    endpoint.method,
    `${baseURL}${endpoint.path}`,
    endpoint.body || null,
    {
      tags: {
        endpoint: endpoint.name || endpoint.path,
      },
      redirects: Number(__ENV.GF_LOAD_REDIRECTS || '5'),
    },
  );

  check(response, {
    'status below 500': (r) => r.status > 0 && r.status < 500,
  });

  sleep(Number(__ENV.GF_LOAD_SLEEP_SECONDS || '1'));
}

function parseEndpoints(raw) {
  if (!raw) {
    return [
      { method: 'GET', path: '/health', weight: 5 },
      { method: 'GET', path: '/manifest.json', weight: 2 },
      { method: 'GET', path: '/sw.js', weight: 2 },
      { method: 'GET', path: '/login', weight: 3 },
      { method: 'GET', path: '/customer/login', weight: 3 },
    ];
  }

  const trimmed = raw.trim();
  if (trimmed.startsWith('[')) {
    return JSON.parse(trimmed).map(normalizeEndpoint);
  }

  return trimmed
    .split(',')
    .map((path) => normalizeEndpoint(path.trim()))
    .filter((endpoint) => endpoint.path);
}

function normalizeEndpoint(value) {
  if (typeof value === 'string') {
    return { method: 'GET', path: ensureAbsolutePath(value), weight: 1 };
  }

  return {
    method: (value.method || 'GET').toUpperCase(),
    path: ensureAbsolutePath(value.path || '/'),
    body: value.body || null,
    weight: Number(value.weight || 1),
    name: value.name || value.path,
  };
}

function ensureAbsolutePath(path) {
  return path.startsWith('/') ? path : `/${path}`;
}

function chooseEndpoint() {
  const totalWeight = endpoints.reduce((sum, endpoint) => sum + endpoint.weight, 0);
  let cursor = Math.random() * totalWeight;

  for (const endpoint of endpoints) {
    cursor -= endpoint.weight;
    if (cursor <= 0) {
      return endpoint;
    }
  }

  return endpoints[endpoints.length - 1];
}
