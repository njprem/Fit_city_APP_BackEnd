/**
 * Simulates the admin flow for creating a destination change request,
 * uploading a hero image, and submitting it for review.
 *
 * Run with:
 * BASE_URL=http://localhost:8080 \
 * ADMIN_EMAIL=admin@example.com \
 * ADMIN_PASSWORD=secret \
 * k6 run scripts/k6/create_destination_with_image.js
 *
 * Optional overrides: DEST_* fields, HERO_IMAGE_* metadata, STEP_DELAY, VUS, ITERATIONS,
 * LOGIN_PER_ITERATION (defaults to false), etc.
 */

import http from 'k6/http';
import { Trend } from 'k6/metrics';
import { check, fail, group, sleep } from 'k6';
import encoding from 'k6/encoding';
import { uuidv4 } from 'https://jslib.k6.io/k6-utils/1.4.0/index.js';

const BASE_URL = (__ENV.BASE_URL || 'http://localhost:8080').replace(/\/+$/, '');
const API_BASE = `${BASE_URL}/api/v1`;

const ADMIN_EMAIL = __ENV.ADMIN_EMAIL;
const ADMIN_PASSWORD = __ENV.ADMIN_PASSWORD;
if (!ADMIN_EMAIL || !ADMIN_PASSWORD) {
  throw new Error('ADMIN_EMAIL and ADMIN_PASSWORD environment variables are required');
}

const DEST_CATEGORY = __ENV.DEST_CATEGORY || 'Nature';
const DEST_CITY = __ENV.DEST_CITY || 'Jakarta';
const DEST_COUNTRY = __ENV.DEST_COUNTRY || 'Indonesia';
const DEST_DESCRIPTION =
  __ENV.DEST_DESCRIPTION || 'Automated destination seeded by the k6 load test.';
const DEST_CONTACT = __ENV.DEST_CONTACT || '+62-21-555-1234';
const DEST_OPENING = __ENV.DEST_OPENING || '08:00';
const DEST_CLOSING = __ENV.DEST_CLOSING || '18:00';
const DEST_STATUS = __ENV.DEST_STATUS || 'draft';
const DEST_LAT = parseNumber(__ENV.DEST_LAT, -6.2088);
const DEST_LON = parseNumber(__ENV.DEST_LON, 106.8456);
const STEP_DELAY = parseNumber(__ENV.STEP_DELAY, 0);
const LOGIN_PER_ITERATION = __ENV.LOGIN_PER_ITERATION === 'true';

const heroImageBytes = loadHeroImage();
const heroImageName = __ENV.HERO_IMAGE_NAME || inferFileName(__ENV.HERO_IMAGE_PATH) || 'k6-hero.png';
const heroImageMime = __ENV.HERO_IMAGE_MIME || 'image/png';

const vus = Number(__ENV.VUS) || 1;
const iterations = Number(__ENV.ITERATIONS) || vus;

const loginTrend = new Trend('destination_login_duration', true);
const draftTrend = new Trend('destination_draft_create_duration', true);
const heroTrend = new Trend('destination_hero_upload_duration', true);
const submitTrend = new Trend('destination_submit_duration', true);

export const options = {
  vus,
  iterations,
  insecureSkipTLSVerify: __ENV.SKIP_TLS_VERIFY === 'true',
  thresholds: {
    http_req_failed: ['rate<0.05'],
    checks: ['rate>0.95'],
    destination_draft_create_duration: ['p(95)<2000'],
    destination_hero_upload_duration: ['p(95)<2000'],
    destination_submit_duration: ['p(95)<2000'],
  },
};

export function setup() {
  if (LOGIN_PER_ITERATION) {
    return {};
  }
  return { token: login() };
}

export default function createDestinationScenario(setupData) {
  group('create destination with hero image', () => {
    const token = resolveToken(setupData);
    delayIfNeeded();

    const draft = createDraft(token);
    console.log(`Draft ${draft.id} created for ${draft.name} (${draft.slug})`);
    delayIfNeeded();

    uploadHeroImage(token, draft.id);
    delayIfNeeded();

    submitDraft(token, draft.id);
  });
}

function login() {
  const payload = JSON.stringify({
    email: ADMIN_EMAIL,
    password: ADMIN_PASSWORD,
  });
  const res = http.post(`${API_BASE}/auth/login`, payload, {
    headers: { 'Content-Type': 'application/json' },
  });
  loginTrend.add(res.timings.duration);

  const ok = check(res, {
    'login succeeded': (r) => r.status === 200,
    'token returned': (r) => {
      const data = extractJson(r);
      return data && data.token;
    },
  });
  if (!ok) {
    fail(`login failed (${res.status}): ${res.body}`);
  }

  const data = extractJson(res);
  if (!data || !data.token) {
    fail('login response missing token');
  }
  return data.token;
}

function createDraft(token) {
  const payload = buildDestinationPayload();
  const res = http.post(`${API_BASE}/admin/destination-changes`, JSON.stringify(payload), authJsonParams(token));
  draftTrend.add(res.timings.duration);

  const ok = check(res, {
    'draft created': (r) => r.status === 201,
    'change id returned': (r) => {
      const data = extractJson(r);
      return data && data.change_request && data.change_request.id;
    },
  });
  if (!ok) {
    fail(`failed to create draft (${res.status}): ${res.body}`);
  }

  const data = extractJson(res);
  const changeRequest = data && data.change_request;
  if (!changeRequest || !changeRequest.id) {
    fail('draft response is missing change_request.id');
  }

  return {
    id: changeRequest.id,
    name: payload.fields.name,
    slug: payload.fields.slug,
  };
}

function uploadHeroImage(token, changeId) {
  const res = http.post(
    `${API_BASE}/admin/destination-changes/${changeId}/hero-image`,
    { file: http.file(heroImageBytes, heroImageName, heroImageMime) },
    authMultipartParams(token),
  );
  heroTrend.add(res.timings.duration);

  const ok = check(res, {
    'hero image uploaded': (r) => r.status === 200,
  });
  if (!ok) {
    fail(`failed to upload hero image (${res.status}): ${res.body}`);
  }
}

function submitDraft(token, changeId) {
  const res = http.post(`${API_BASE}/admin/destination-changes/${changeId}/submit`, '{}', authJsonParams(token));
  submitTrend.add(res.timings.duration);

  const ok = check(res, {
    'submission accepted': (r) => r.status === 202,
  });
  if (!ok) {
    fail(`failed to submit draft (${res.status}): ${res.body}`);
  }
}

function buildDestinationPayload() {
  const suffix = uuidv4().split('-')[0];
  const name = `K6 Destination ${suffix}`;
  const slug = `k6-destination-${suffix}`.toLowerCase();

  return {
    action: 'create',
    fields: {
      name,
      slug,
      city: DEST_CITY,
      country: DEST_COUNTRY,
      category: DEST_CATEGORY,
      description: DEST_DESCRIPTION,
      latitude: DEST_LAT,
      longitude: DEST_LON,
      contact: DEST_CONTACT,
      opening_time: DEST_OPENING,
      closing_time: DEST_CLOSING,
      status: DEST_STATUS,
    },
  };
}

function authJsonParams(token) {
  return {
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
  };
}

function authMultipartParams(token) {
  return {
    headers: {
      Authorization: `Bearer ${token}`,
    },
  };
}

function resolveToken(setupData) {
  if (LOGIN_PER_ITERATION) {
    return login();
  }
  if (setupData && setupData.token) {
    return setupData.token;
  }
  return login();
}

function extractJson(res) {
  try {
    return res.json();
  } catch (err) {
    return null;
  }
}

function delayIfNeeded() {
  if (STEP_DELAY > 0) {
    sleep(STEP_DELAY);
  }
}

function parseNumber(rawValue, fallback) {
  if (rawValue === undefined || rawValue === null || rawValue === '') {
    return fallback;
  }
  const parsed = Number(rawValue);
  return isFinite(parsed) ? parsed : fallback;
}

function inferFileName(path) {
  if (!path) {
    return null;
  }
  const segments = path.split(/[\\/]/);
  return segments[segments.length - 1] || null;
}

function loadHeroImage() {
  if (__ENV.HERO_IMAGE_PATH) {
    return open(__ENV.HERO_IMAGE_PATH, 'b');
  }
  return encoding.b64decode(DEFAULT_HERO_IMAGE_BASE64, 'std');
}

const DEFAULT_HERO_IMAGE_BASE64 =
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8Xw8AAn0B9kTnWYEAAAAASUVORK5CYII=';
