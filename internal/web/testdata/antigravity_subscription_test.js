'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function loadDashboard(fetchImpl = async () => ({ ok: false, status: 500 })) {
  const html = fs.readFileSync(path.join(__dirname, '../assets/index.html'), 'utf8');
  const scriptMatch = html.match(/<script>\s*(\(function \(\) \{[\s\S]*?\}\)\(\);)\s*<\/script>/);
  assert.ok(scriptMatch, 'embedded dashboard script must be present');

  const bootstrap = `if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }`;
  const exposure = `globalThis.__dashboardTest = {
    fetchAntigravityQuota,
    fetchAntigravityTierSummary,
    fetchAntigravityTierSummarySafe,
    getAntigravityPlanLabel,
    parseAntigravityTierSummary,
    quotaStore,
  };`;
  const source = scriptMatch[1].replace(bootstrap, exposure);
  assert.notEqual(source, scriptMatch[1], 'dashboard bootstrap must be replaced for testing');

  const storage = {
    getItem: key => key === 'cli-proxy-auth'
      ? JSON.stringify({ state: { managementKey: 'test-key' } })
      : null,
  };
  const context = {
    console,
    fetch: fetchImpl,
    localStorage: storage,
    navigator: { userAgent: 'node-test' },
    TextDecoder,
    TextEncoder,
    window: { location: { host: 'test.local' }, parent: { localStorage: storage } },
    document: {
      readyState: 'loading',
      addEventListener() {},
      getElementById() { return { addEventListener() {}, classList: { add() {}, remove() {} } }; },
      querySelectorAll() { return []; },
    },
  };
  context.globalThis = context;
  vm.runInNewContext(source, context, { filename: 'index.html' });
  return context.__dashboardTest;
}

test('parseAntigravityTierSummary resolves known tiers from object and nested string payloads', () => {
  const dashboard = loadDashboard();
  const cases = [
    ['free current tier', { currentTier: { id: 'free-tier', name: 'Free Tier' } }, 'free'],
    ['pro snake case tier', { current_tier: { id: 'g1-pro-tier', name: 'Pro Tier' } }, 'pro'],
    ['ultra nested object body', { body: { currentTier: { id: 'g1-ultra-tier' } } }, 'ultra'],
    ['ultra lite nested string body', JSON.stringify({ body: JSON.stringify({ currentTier: { id: 'g1-ultra-lite-tier' } }) }), 'ultra-lite'],
  ];

  for (const [name, payload, expectedPlan] of cases) {
    assert.equal(dashboard.parseAntigravityTierSummary(payload).plan, expectedPlan, name);
  }
});

test('parseAntigravityTierSummary uses paid tier only when it has an id', () => {
  const dashboard = loadDashboard();
  const paid = dashboard.parseAntigravityTierSummary({
    currentTier: { id: 'free-tier', name: 'Free Tier' },
    paidTier: { id: 'g1-ultra-tier', name: 'Ultra Tier' },
  });
  assert.deepEqual({ ...paid }, { plan: 'ultra', tierId: 'g1-ultra-tier', tierName: 'Ultra Tier' });

  const paidWithoutID = dashboard.parseAntigravityTierSummary({
    current_tier: { id: 'free-tier', name: 'Free Tier' },
    paid_tier: { name: 'Incomplete Paid Tier' },
  });
  assert.deepEqual({ ...paidWithoutID }, { plan: 'free', tierId: 'free-tier', tierName: 'Free Tier' });
});

test('subscription labels map known plans and preserve unknown tier evidence', () => {
  const dashboard = loadDashboard();
  const labels = [
    [{ plan: 'free' }, 'Free'],
    [{ plan: 'pro' }, 'Pro'],
    [{ plan: 'ultra' }, 'Ultra'],
    [{ plan: 'ultra-lite' }, 'Ultra Lite'],
    [{ plan: 'unknown', tierName: 'Custom Plan', tierId: 'custom-tier' }, 'Custom Plan'],
    [{ plan: 'unknown', tierName: null, tierId: 'custom-tier' }, 'custom-tier'],
    [null, null],
  ];
  for (const [subscription, expected] of labels) {
    assert.equal(dashboard.getAntigravityPlanLabel(subscription), expected);
  }
});

test('fetchAntigravityTierSummary proxies loadCodeAssist through management api-call', async () => {
  let capturedEndpoint;
  let capturedOptions;
  const dashboard = loadDashboard(async (endpoint, options) => {
    capturedEndpoint = endpoint;
    capturedOptions = options;
    return {
      ok: true,
      json: async () => ({ status_code: 200, body: JSON.stringify({ currentTier: { id: 'free-tier' } }) }),
    };
  });

  const result = await dashboard.fetchAntigravityTierSummary('credential-7');
  assert.equal(capturedEndpoint, '/v0/management/api-call');
  assert.equal(capturedOptions.method, 'POST');
  assert.equal(capturedOptions.headers.Authorization, 'Bearer test-key');
  const proxyRequest = JSON.parse(capturedOptions.body);
  assert.equal(proxyRequest.authIndex, 'credential-7');
  assert.equal(proxyRequest.url, 'https://daily-cloudcode-pa.googleapis.com/v1internal:loadCodeAssist');
  assert.equal(proxyRequest.header.Authorization, 'Bearer $TOKEN$');
  assert.deepEqual(JSON.parse(proxyRequest.data), { metadata: { ideType: 'ANTIGRAVITY' } });
  assert.deepEqual({ ...result }, { plan: 'free', tierId: 'free-tier', tierName: null });
});

test('Antigravity quota path completes when subscription lookup fails', async () => {
  let callCount = 0;
  const dashboard = loadDashboard(async () => {
    callCount++;
    if (callCount === 1) return { ok: false, status: 503 };
    return {
      ok: true,
      json: async () => ({
        status_code: 200,
        body: JSON.stringify({
          groups: [{ displayName: 'Models', buckets: [{ displayName: 'Weekly', remainingFraction: 0.5 }] }],
        }),
      }),
    };
  });

  await dashboard.fetchAntigravityQuota({ name: 'credential.json', project_id: 'project-1' }, 'credential-7');
  const quota = dashboard.quotaStore['credential.json'];
  assert.equal(quota.loading, false);
  assert.equal(quota.subscription, null);
  assert.equal(quota.error, null);
  assert.equal(quota.groups.length, 1);
  assert.equal(quota.groups[0].rows[0].percent, 50);
});
