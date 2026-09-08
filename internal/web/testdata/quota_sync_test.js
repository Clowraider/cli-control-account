'use strict';

const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

function createJwt(payload) {
  const header = Buffer.from(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).toString('base64url');
  const body = Buffer.from(JSON.stringify(payload)).toString('base64url');
  return `${header}.${body}.signature`;
}

function loadDashboard(fetchImpl = async () => ({ ok: false, status: 500 }), pluginVersion = '1.0.0') {
  let html = fs.readFileSync(path.join(__dirname, '../assets/index.html'), 'utf8');
  html = html.replaceAll('__PLUGIN_VERSION__', pluginVersion);
  const scriptMatch = html.match(/<script>\s*(\(function \(\) \{[\s\S]*?\}\)\(\);)\s*<\/script>/);
  assert.ok(scriptMatch, 'embedded dashboard script must be present');

  const bootstrap = `if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }`;
  const exposure = `globalThis.__dashboardTest = {
    fetchClaudeQuota,
    fetchCodexQuota,
    parseIdTokenPayload,
    extractCodexChatgptAccountId,
    parseCodexResetCredits,
    parseClaudePlan,
    syncServerTimeOffset,
    getServerTimeOffset: () => serverTimeOffsetMs,
    setServerTimeOffset: (v) => { serverTimeOffsetMs = v; },
    formatResetInfo,
    formatDuration,
    renderCard,
    quotaStore,
    compareSemver,
    checkForPluginUpdates,
  };`;
  const source = scriptMatch[1].replace(bootstrap, exposure);
  assert.notEqual(source, scriptMatch[1], 'dashboard bootstrap must be replaced for testing');

  const storage = {
    getItem: key => key === 'cli-proxy-auth'
      ? JSON.stringify({ state: { managementKey: 'test-key' } })
      : null,
  };
  const elements = new Map();
  function getOrCreateElement(id) {
    if (!elements.has(id)) {
      const classSet = new Set();
      const listeners = {};
      elements.set(id, {
        id,
        tagName: 'DIV',
        textContent: '',
        title: '',
        disabled: false,
        dataset: {},
        classList: {
          add: (...cls) => cls.forEach(c => classSet.add(c)),
          remove: (...cls) => cls.forEach(c => classSet.delete(c)),
          contains: (c) => classSet.has(c),
          toggle: (c) => (classSet.has(c) ? (classSet.delete(c), false) : (classSet.add(c), true)),
        },
        addEventListener(event, fn) {
          listeners[event] = listeners[event] || [];
          listeners[event].push(fn);
        },
        click() {
          if (listeners['click']) {
            listeners['click'].forEach(fn => fn({ target: this }));
          }
        },
      });
    }
    return elements.get(id);
  }

  let lastOpenedWindow = null;
  const timers = [];
  const wrappedSetTimeout = (fn, delay) => {
    const timer = setTimeout(fn, delay);
    if (timer && typeof timer.unref === 'function') {
      timer.unref();
    }
    timers.push({ fn, delay, timer });
    return timer;
  };
  const wrappedClearTimeout = (id) => {
    clearTimeout(id);
    const idx = timers.findIndex(t => t.timer === id);
    if (idx >= 0) timers.splice(idx, 1);
  };
  const context = {
    console,
    fetch: fetchImpl,
    localStorage: storage,
    navigator: { userAgent: 'node-test' },
    TextDecoder,
    TextEncoder,
    Buffer,
    setTimeout: wrappedSetTimeout,
    clearTimeout: wrappedClearTimeout,
    atob: str => Buffer.from(str, 'base64').toString('utf8'),
    window: {
      location: { host: 'test.local' },
      parent: { localStorage: storage },
      setTimeout: wrappedSetTimeout,
      clearTimeout: wrappedClearTimeout,
      atob: str => Buffer.from(str, 'base64').toString('utf8'),
      open: (url, target, features) => {
        lastOpenedWindow = { url, target, features };
        return null;
      },
    },
    document: {
      readyState: 'loading',
      addEventListener() {},
      getElementById: getOrCreateElement,
      querySelectorAll() { return []; },
    },
  };
  context.globalThis = context;
  vm.runInNewContext(source, context, { filename: 'index.html' });
  context.__dashboardTest.elements = elements;
  context.__dashboardTest.getElement = getOrCreateElement;
  context.__dashboardTest.getLastOpenedWindow = () => lastOpenedWindow;
  context.__dashboardTest.getPendingTimers = () => timers;
  context.__dashboardTest.triggerLastTimer = () => {
    const last = timers.pop();
    if (last) {
      clearTimeout(last.timer);
      last.fn();
    }
  };
  return context.__dashboardTest;
}

test('Scope 1: parseIdTokenPayload decodes JWT and objects', () => {
  const dashboard = loadDashboard();
  const jwt = createJwt({
    'https://api.openai.com/auth': { chatgpt_account_id: 'acc-openai-123' },
    sub: 'user-1',
  });

  const parsed = dashboard.parseIdTokenPayload(jwt);
  assert.ok(parsed, 'should parse valid JWT');
  assert.equal(parsed['https://api.openai.com/auth']?.chatgpt_account_id, 'acc-openai-123');

  const objParsed = dashboard.parseIdTokenPayload({ chatgpt_account_id: 'acc-direct' });
  assert.equal(objParsed.chatgpt_account_id, 'acc-direct');

  assert.equal(dashboard.parseIdTokenPayload('invalid-jwt'), null);
  assert.equal(dashboard.parseIdTokenPayload(null), null);
});

test('Scope 1: extractCodexChatgptAccountId handles candidates and fallback', () => {
  const dashboard = loadDashboard();
  const jwt = createJwt({
    'https://api.openai.com/auth': { chatgpt_account_id: 'acc-jwt' },
  });

  // Candidate: file.id_token as JWT
  assert.equal(dashboard.extractCodexChatgptAccountId({ id_token: jwt }), 'acc-jwt');

  // Candidate: file.metadata.id_token
  assert.equal(dashboard.extractCodexChatgptAccountId({ metadata: { id_token: jwt } }), 'acc-jwt');

  // Candidate: file.attributes.id_token
  assert.equal(dashboard.extractCodexChatgptAccountId({ attributes: { id_token: jwt } }), 'acc-jwt');

  // Direct JWT string passed
  assert.equal(dashboard.extractCodexChatgptAccountId(jwt), 'acc-jwt');

  // Fallback: project_id
  assert.equal(dashboard.extractCodexChatgptAccountId({ project_id: 'proj-fallback' }), 'proj-fallback');

  // Fallback: _raw.account_id
  assert.equal(dashboard.extractCodexChatgptAccountId({ _raw: { account_id: 'raw-account' } }), 'raw-account');

  // Fallback: _raw.chatgpt_account_id
  assert.equal(dashboard.extractCodexChatgptAccountId({ _raw: { chatgpt_account_id: 'raw-chatgpt' } }), 'raw-chatgpt');

  // None found
  assert.equal(dashboard.extractCodexChatgptAccountId({}), null);
});

test('Scope 1 & 2: fetchCodexQuota sets headers and extracts resetCredits', async () => {
  const jwt = createJwt({
    'https://api.openai.com/auth': { chatgpt_account_id: 'acc-codex-999' },
  });

  const calls = [];
  const dashboard = loadDashboard(async (endpoint, options) => {
    const payload = JSON.parse(options.body);
    calls.push({ endpoint, payload });

    if (payload.url.includes('/usage')) {
      return {
        ok: true,
        status: 200,
        json: async () => ({
          statusCode: 200,
          header: { date: ['Tue, 08 Sep 2026 12:00:00 GMT'] },
          body: {
            plan_type: 'plus',
            rate_limit: {
              primary_window: { used_percent: 15, limit_window_seconds: 18000 },
              secondary_window: { used_percent: 40, limit_window_seconds: 604800 },
            },
          },
        }),
      };
    }

    if (payload.url.includes('/rate-limit-reset-credits')) {
      return {
        ok: true,
        status: 200,
        json: async () => ({
          statusCode: 200,
          header: { date: ['Tue, 08 Sep 2026 12:00:00 GMT'] },
          body: {
            available_count: 2,
            credits: [
              { reset_type: 'codex_rate_limits', status: 'available' },
              { reset_type: 'codex_rate_limits', status: 'available' },
            ],
          },
        }),
      };
    }

    return { ok: false, status: 404 };
  });

  const file = {
    name: 'codex-account-1.json',
    id_token: jwt,
  };

  await dashboard.fetchCodexQuota(file, 'auth-1');

  assert.equal(calls.length, 2, 'should fetch usage and reset credits in parallel');
  for (const call of calls) {
    assert.equal(call.payload.header['Authorization'], 'Bearer $TOKEN$');
    assert.equal(call.payload.header['OpenAI-Beta'], 'codex-1');
    assert.equal(call.payload.header['Originator'], 'Codex Desktop');
    assert.equal(call.payload.header['User-Agent'], 'codex-tui/0.149.1 (Mac OS 26.5.2; arm64) iTerm.app/3.6.11 (codex-tui; 0.149.1)');
    assert.equal(call.payload.header['Chatgpt-Account-Id'], 'acc-codex-999');
  }

  const stored = dashboard.quotaStore['codex-account-1.json'];
  assert.ok(stored, 'quota should be stored');
  assert.equal(stored.plan, 'Plus');
  assert.equal(stored.resetCredits?.availableCount, 2);

  // Render card should contain badge
  const cardHtml = dashboard.renderCard(file);
  assert.ok(cardHtml.includes('reset-credits-badge'), 'card should render reset-credits-badge');
  assert.ok(cardHtml.includes('⚡ 2 reset credits'), 'card should display credit count');
});

test('Scope 2: parseCodexResetCredits parses different payload shapes', () => {
  const dashboard = loadDashboard();

  // Shape 1: available_count
  const s1 = dashboard.parseCodexResetCredits({ available_count: 3 });
  assert.equal(s1.availableCount, 3);

  // Shape 2: applicable_available_count
  const s2 = dashboard.parseCodexResetCredits({ applicable_available_count: 1 });
  assert.equal(s2.availableCount, 1);

  // Shape 3: credits array filtering
  const s3 = dashboard.parseCodexResetCredits({
    credits: [
      { reset_type: 'codex_rate_limits', status: 'available' },
      { reset_type: 'other_limit', status: 'available' },
      { reset_type: 'codex_rate_limits', status: 'used' },
    ],
  });
  assert.equal(s3.availableCount, 1);
  assert.equal(s3.credits.length, 1);
});

test('Scope 3: parseClaudePlan resolves plan according to CPAMC specification', () => {
  const dashboard = loadDashboard();

  // has_claude_max -> Max
  assert.equal(dashboard.parseClaudePlan({ account: { has_claude_max: true } }), 'Max');

  // has_claude_pro -> Pro
  assert.equal(dashboard.parseClaudePlan({ account: { has_claude_pro: true } }), 'Pro');

  // claude_team active -> Team
  assert.equal(dashboard.parseClaudePlan({
    organization: { organization_type: 'claude_team', subscription_status: 'active' },
  }), 'Team');

  // has_claude_max === false && has_claude_pro === false -> Free
  assert.equal(dashboard.parseClaudePlan({
    account: { has_claude_max: false, has_claude_pro: false },
  }), 'Free');

  // Default -> Pro
  assert.equal(dashboard.parseClaudePlan({}), 'Pro');
  assert.equal(dashboard.parseClaudePlan(null), 'Pro');
});

test('Scope 3: fetchClaudeQuota parses profile in parallel and model-specific windows', async () => {
  const calls = [];
  const dashboard = loadDashboard(async (endpoint, options) => {
    const payload = JSON.parse(options.body);
    calls.push({ endpoint, payload });

    if (payload.url.includes('/usage')) {
      return {
        ok: true,
        status: 200,
        json: async () => ({
          statusCode: 200,
          header: { date: ['Tue, 08 Sep 2026 12:00:00 GMT'] },
          body: {
            five_hour: { utilization: 0.1, resets_at: '2026-09-08T15:00:00Z' },
            seven_day: { utilization: 0.2, resets_at: '2026-09-10T12:00:00Z' },
            seven_day_opus: { utilization: 0.3, resets_at: '2026-09-11T12:00:00Z' },
            seven_day_sonnet: { utilization: 0.4, resets_at: '2026-09-12T12:00:00Z' },
            seven_day_cowork: { utilization: 0.5, resets_at: '2026-09-13T12:00:00Z' },
            seven_day_oauth_apps: { utilization: 0.6, resets_at: '2026-09-14T12:00:00Z' },
            iguana_necktie: { utilization: 0.7, resets_at: '2026-09-15T12:00:00Z' },
          },
        }),
      };
    }

    if (payload.url.includes('/profile')) {
      return {
        ok: true,
        status: 200,
        json: async () => ({
          statusCode: 200,
          body: {
            account: { has_claude_max: true },
          },
        }),
      };
    }

    return { ok: false, status: 404 };
  });

  const file = { name: 'claude-account-1.json' };
  await dashboard.fetchClaudeQuota(file, 'auth-claude');

  assert.equal(calls.length, 2, 'should call usage and profile in parallel');
  const stored = dashboard.quotaStore['claude-account-1.json'];
  assert.ok(stored);
  assert.equal(stored.plan, 'Max', 'should resolve Max plan');

  const rows = stored.groups[0].rows;
  const labels = rows.map(r => r.label);
  assert.ok(labels.includes('Five Hour Limit'));
  assert.ok(labels.includes('Weekly Limit'));
  assert.ok(labels.includes('Opus Limit'));
  assert.ok(labels.includes('Sonnet Limit'));
  assert.ok(labels.includes('Cowork Limit'));
  assert.ok(labels.includes('OAuth Apps Limit'));
  assert.ok(labels.includes('Fable Limit'));

  // Check percent calculations
  const opusRow = rows.find(r => r.label === 'Opus Limit');
  assert.equal(opusRow.percent, 70);
  assert.equal(opusRow.percentLabel, '70% remaining');
  assert.ok(opusRow.resetMs > 0);
});

test('Scope 4: Server Time Clock Skew Synchronization', () => {
  const dashboard = loadDashboard();

  // Test syncServerTimeOffset with Date header
  const fakeServerDate = '2026-09-08T16:00:00.000Z';
  const serverTimeMs = new Date(fakeServerDate).getTime();

  dashboard.syncServerTimeOffset({
    header: { date: [fakeServerDate] },
  });

  const offset = dashboard.getServerTimeOffset();
  assert.notEqual(offset, 0);

  // formatResetInfo should use (Date.now() + serverTimeOffsetMs)
  // When resetAfterSeconds is 3600, resetMs = now + 3600*1000, delta = 3600000ms -> 1h 0m
  const resetAfterInfo = dashboard.formatResetInfo(null, 3600);
  assert.equal(resetAfterInfo.label, 'Refreshes in 1h 0m');
  assert.ok(Math.abs(resetAfterInfo.ms - (Date.now() + offset + 3600000)) < 50);

  // Target timestamp in the future using server time basis
  const targetMs = serverTimeMs + 3600 * 1000 + 10000; // 1h and 10s ahead
  const resetInfo = dashboard.formatResetInfo(new Date(targetMs).toISOString(), null);
  assert.equal(resetInfo.label, 'Refreshes in 1h 0m');
  assert.equal(resetInfo.ms, targetMs);
});

test('compareSemver behaves correctly for older, newer, equal, and v-prefixed versions', () => {
  const dashboard = loadDashboard();
  const { compareSemver } = dashboard;

  // Older
  assert.equal(compareSemver('1.0.0', '1.0.1'), -1, 'older patch should be -1');
  assert.equal(compareSemver('1.1.0', '1.2.0'), -1, 'older minor should be -1');
  assert.equal(compareSemver('1.0.0', '2.0.0'), -1, 'older major should be -1');

  // Newer
  assert.equal(compareSemver('1.0.1', '1.0.0'), 1, 'newer patch should be 1');
  assert.equal(compareSemver('1.2.0', '1.1.9'), 1, 'newer minor should be 1');
  assert.equal(compareSemver('2.0.0', '1.9.9'), 1, 'newer major should be 1');

  // Equal
  assert.equal(compareSemver('1.0.0', '1.0.0'), 0, 'identical versions should be 0');
  assert.equal(compareSemver('2.15.3', '2.15.3'), 0, 'identical multi-digit versions should be 0');

  // v-prefixed versions
  assert.equal(compareSemver('v1.2.3', '1.2.3'), 0, 'v-prefix equal should be 0');
  assert.equal(compareSemver('v1.2.3', 'v1.2.3'), 0, 'both v-prefix equal should be 0');
  assert.equal(compareSemver('v2.0.0', 'v1.9.9'), 1, 'v-prefix newer should be 1');
  assert.equal(compareSemver('v1.0.0', 'v1.0.1'), -1, 'v-prefix older should be -1');
  assert.equal(compareSemver('1.2.3', 'v1.2.4'), -1, 'non-v vs v older should be -1');
  assert.equal(compareSemver('v1.2.4', '1.2.3'), 1, 'v vs non-v newer should be 1');

  // Different segment lengths
  assert.equal(compareSemver('v1.0', '1.0.0'), 0, '2 segments vs 3 segments equal should be 0');
  assert.equal(compareSemver('v1.0.1', '1.0'), 1, '3 segments vs 2 segments newer should be 1');
});

test('checkForPluginUpdates triggers API call, handles "Update available" (sets class and releaseUrl), "Up to date", and error handling', async () => {
  // 1. Update available scenario
  const calls = [];
  const updateFetch = async (endpoint, options = {}) => {
    const payload = JSON.parse(options.body || '{}');
    calls.push({ endpoint, payload });
    return {
      ok: true,
      status: 200,
      json: async () => ({
        statusCode: 200,
        body: JSON.stringify({
          tag_name: 'v1.2.0',
          html_url: 'https://github.com/Clowraider/cli-control-account/releases/tag/v1.2.0',
        }),
      }),
    };
  };

  const dashboardUpdate = loadDashboard(updateFetch, '1.0.0');
  const btn = dashboardUpdate.getElement('btn-check-update');
  const icon = dashboardUpdate.getElement('update-icon');
  const text = dashboardUpdate.getElement('update-text');

  await dashboardUpdate.checkForPluginUpdates();

  // Verify API call
  assert.equal(calls.length, 1);
  assert.equal(calls[0].endpoint, '/v0/management/api-call');
  assert.equal(calls[0].payload.method, 'GET');
  assert.equal(calls[0].payload.url, 'https://api.github.com/repos/Clowraider/cli-control-account/releases/latest');
  assert.equal(calls[0].payload.header['Accept'], 'application/vnd.github+json');
  assert.equal(calls[0].payload.header['X-GitHub-Api-Version'], '2022-11-28');
  assert.equal(calls[0].payload.header['User-Agent'], 'control-account-plugin');

  // Verify button state
  assert.ok(btn.classList.contains('update-available'), 'should add update-available class');
  assert.equal(btn.classList.contains('checking'), false, 'should remove checking class');
  assert.equal(btn.disabled, false, 'should not be disabled');
  assert.equal(btn.dataset.releaseUrl, 'https://github.com/Clowraider/cli-control-account/releases/tag/v1.2.0');
  assert.equal(icon.textContent, '★');
  assert.ok(text.textContent.includes('v1.2.0'), 'should display update tag');

  // Verify clicking when update-available opens release URL
  await dashboardUpdate.checkForPluginUpdates();
  assert.equal(calls.length, 1, 'should not trigger another API call when update is available');
  const opened = dashboardUpdate.getLastOpenedWindow();
  assert.ok(opened, 'window.open should have been called');
  assert.equal(opened.url, 'https://github.com/Clowraider/cli-control-account/releases/tag/v1.2.0');
  assert.equal(opened.target, '_blank');

  // 2. Up to date scenario
  const upToDateCalls = [];
  const upToDateFetch = async (endpoint, options = {}) => {
    const payload = JSON.parse(options.body || '{}');
    upToDateCalls.push({ endpoint, payload });
    return {
      ok: true,
      status: 200,
      json: async () => ({
        statusCode: 200,
        body: JSON.stringify({
          tag_name: 'v1.0.0',
          html_url: 'https://github.com/Clowraider/cli-control-account/releases/tag/v1.0.0',
        }),
      }),
    };
  };

  const dashboardUpToDate = loadDashboard(upToDateFetch, '1.0.0');
  const btn2 = dashboardUpToDate.getElement('btn-check-update');
  const icon2 = dashboardUpToDate.getElement('update-icon');
  const text2 = dashboardUpToDate.getElement('update-text');

  await dashboardUpToDate.checkForPluginUpdates();
  assert.equal(upToDateCalls.length, 1);
  assert.ok(btn2.classList.contains('up-to-date'), 'should add up-to-date class');
  assert.equal(btn2.classList.contains('update-available'), false);
  assert.equal(btn2.disabled, false);
  assert.equal(icon2.textContent, '✓');
  assert.equal(text2.textContent, 'Up to date');

  // Verify up-to-date timer reverts to default after 4s
  const upToDateTimers = dashboardUpToDate.getPendingTimers();
  assert.equal(upToDateTimers.length, 1);
  assert.equal(upToDateTimers[0].delay, 4000);
  dashboardUpToDate.triggerLastTimer();
  assert.equal(btn2.classList.contains('up-to-date'), false, 'should revert up-to-date class');
  assert.equal(icon2.textContent, '↻');
  assert.equal(text2.textContent, 'Check update');

  // 3. Error handling scenario
  const errorFetch = async () => ({
    ok: false,
    status: 500,
    json: async () => ({ error: 'Internal Server Error' }),
  });

  const dashboardError = loadDashboard(errorFetch, '1.0.0');
  const btn3 = dashboardError.getElement('btn-check-update');
  const icon3 = dashboardError.getElement('update-icon');
  const text3 = dashboardError.getElement('update-text');

  await dashboardError.checkForPluginUpdates();
  assert.ok(btn3.classList.contains('update-error'), 'should add update-error class');
  assert.equal(btn3.classList.contains('checking'), false);
  assert.equal(btn3.disabled, false);
  assert.equal(icon3.textContent, '⚠');
  assert.equal(text3.textContent, 'Check failed');

  // Verify error timer reverts to default after 3.5s
  const errorTimers = dashboardError.getPendingTimers();
  assert.equal(errorTimers.length, 1);
  assert.equal(errorTimers[0].delay, 3500);
  dashboardError.triggerLastTimer();
  assert.equal(btn3.classList.contains('update-error'), false, 'should revert update-error class');
  assert.equal(icon3.textContent, '↻');
  assert.equal(text3.textContent, 'Check update');
});
