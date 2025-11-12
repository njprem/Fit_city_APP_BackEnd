/**
 * Exercises the public destination catalogue without requiring authentication.
 * Each iteration either targets a supplied destination ID/slug directly or
 * fetches the list endpoint before loading a random detail view.
 *
 * Run with:
 * BASE_URL=http://localhost:8080 \
 * k6 run scripts/k6/view_published_destinations.js
 */

import http from 'k6/http';
import { Trend } from 'k6/metrics';
import { check, fail, group, sleep } from 'k6';

const BASE_URL = (__ENV.BASE_URL || 'http://localhost:8080').replace(/\/+$/, '');
const API_BASE = `${BASE_URL}/api/v1`;

const STEP_DELAY = parseNumber(__ENV.STEP_DELAY, 0);
const ALLOW_EMPTY_RESULTS = __ENV.ALLOW_EMPTY_RESULTS === 'true';
const DESTINATION_IDS = parseCsv(__ENV.DESTINATION_IDS);

const LIST_QUERY = (__ENV.LIST_QUERY || '').trim();
const LIST_CATEGORIES = parseCsv(__ENV.LIST_CATEGORIES);
const MIN_RATING = parseFloatValue(__ENV.MIN_RATING);
const MAX_RATING = parseFloatValue(__ENV.MAX_RATING);
const LIST_SORT = (__ENV.LIST_SORT || '').trim();
const LIST_LIMIT = parseIntValue(__ENV.LIST_LIMIT, 20);
const LIST_OFFSET = parseIntValue(__ENV.LIST_OFFSET, 0);

const vus = Number(__ENV.VUS) || 10;
const iterations = Number(__ENV.ITERATIONS) || vus * 10;

const listTrend = new Trend('destination_list_duration', true);
const detailTrend = new Trend('destination_detail_duration', true);

export const options = {
  vus,
  iterations,
  insecureSkipTLSVerify: __ENV.SKIP_TLS_VERIFY === 'true',
  thresholds: {
    http_req_failed: ['rate<0.05'],
    checks: ['rate>0.95'],
    destination_list_duration: ['p(95)<1500'],
    destination_detail_duration: ['p(95)<1500'],
  },
};

export default function viewDestinationsScenario() {
  group('public destinations', () => {
    const identifier = resolveDestinationIdentifier();
    if (!identifier) {
      return;
    }

    delayIfNeeded();
    requestDestinationDetail(identifier);
  });
}

function resolveDestinationIdentifier() {
  if (DESTINATION_IDS.length > 0) {
    // Keep the list trend alive for threshold evaluation even when we skip the list fetch.
    listTrend.add(0);
    return pickRandom(DESTINATION_IDS);
  }
  return fetchIdentifierFromList();
}

function fetchIdentifierFromList() {
  const listUrl = `${API_BASE}/destinations${buildListQuery()}`;
  const listRes = http.get(listUrl, jsonHeaders());
  listTrend.add(listRes.timings.duration);

  const listData = extractJson(listRes);
  const listOk = check(listRes, {
    'list returned 200': (r) => r.status === 200,
    'destinations array present': () => listData && Array.isArray(listData.destinations),
  });
  if (!listOk) {
    fail(`unable to list destinations (${listRes.status}): ${listRes.body}`);
  }

  const destinations = listData.destinations || [];
  if (destinations.length === 0) {
    if (!ALLOW_EMPTY_RESULTS) {
      fail('no published destinations matched the filter');
    }
    return null;
  }

  const selected = pickRandom(destinations);
  const identifier = selected.slug || selected.id;
  if (!identifier) {
    fail('destination payload missing slug/id');
  }
  return identifier;
}

function requestDestinationDetail(identifier) {
  const detailRes = http.get(`${API_BASE}/destinations/${identifier}`, jsonHeaders());
  detailTrend.add(detailRes.timings.duration);

  const detailOk = check(detailRes, {
    'detail returned 200': (r) => r.status === 200,
    'destination payload present': () => {
      const data = extractJson(detailRes);
      return data && data.destination && data.destination.id;
    },
  });
  if (!detailOk) {
    fail(`failed to load destination ${identifier}: ${detailRes.status} ${detailRes.body}`);
  }
}

function jsonHeaders() {
  return { headers: { Accept: 'application/json' } };
}

function buildListQuery() {
  const params = [];
  if (LIST_QUERY) {
    params.push(`query=${encodeURIComponent(LIST_QUERY)}`);
  }
  if (LIST_CATEGORIES.length > 0) {
    params.push(`categories=${LIST_CATEGORIES.map(encodeURIComponent).join(',')}`);
  }
  if (typeof MIN_RATING === 'number') {
    params.push(`min_rating=${MIN_RATING}`);
  }
  if (typeof MAX_RATING === 'number') {
    params.push(`max_rating=${MAX_RATING}`);
  }
  if (LIST_SORT) {
    params.push(`sort=${encodeURIComponent(LIST_SORT)}`);
  }
  if (typeof LIST_LIMIT === 'number' && LIST_LIMIT > 0) {
    params.push(`limit=${LIST_LIMIT}`);
  }
  if (typeof LIST_OFFSET === 'number' && LIST_OFFSET > 0) {
    params.push(`offset=${LIST_OFFSET}`);
  }
  return params.length ? `?${params.join('&')}` : '';
}

function pickRandom(items) {
  const index = Math.floor(Math.random() * items.length);
  return items[index];
}

function extractJson(res) {
  try {
    return res.json();
  } catch (_) {
    return null;
  }
}

function delayIfNeeded() {
  if (STEP_DELAY > 0) {
    sleep(STEP_DELAY);
  }
}

function parseCsv(raw) {
  if (!raw) {
    return [];
  }
  return raw
    .split(',')
    .map((part) => part.trim())
    .filter((part) => part.length > 0);
}

function parseNumber(rawValue, fallback) {
  if (rawValue === undefined || rawValue === null || rawValue === '') {
    return fallback;
  }
  const parsed = Number(rawValue);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function parseFloatValue(raw) {
  if (!raw && raw !== '0') {
    return undefined;
  }
  const parsed = parseFloat(raw);
  return Number.isFinite(parsed) ? parsed : undefined;
}

function parseIntValue(raw, fallback) {
  if (raw === undefined || raw === null || raw === '') {
    return fallback;
  }
  const parsed = parseInt(raw, 10);
  return Number.isFinite(parsed) ? parsed : fallback;
}
