'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const html = fs.readFileSync(require('node:path').join(__dirname, '../assets/index.html'), 'utf8');

function dashboard(raw = null, status = 200, parentDenied = false) {
  const elements = new Map();
  function element(id) {
    if (!elements.has(id)) elements.set(id, { innerHTML: '', value: '', style: {}, classList: { add() {}, remove() {} }, addEventListener(type, fn) { this[type] = fn; }, querySelector(selector) { return element(id + selector); } });
    return elements.get(id);
  }
  const requests = [], writes = [];
  const storage = { getItem: () => raw, setItem: (...args) => writes.push(args) };
  const context = vm.createContext({
    window: { location: { host: 'panel.test:8317' }, parent: parentDenied ? { get localStorage() { throw Error('denied'); } } : { localStorage: storage } },
    navigator: { userAgent: 'test-browser' }, localStorage: storage,
    TextEncoder, TextDecoder, Uint8Array, atob, AbortController, setTimeout, clearTimeout,
    console: { warn() {}, error() {} }, document: { getElementById: element },
    grid: element('grid'), fetch: async (url, options) => { requests.push({ url, options }); return { ok: status === 200, status, json: async () => ({ files: [] }) }; },
    renderEgoAccountsTable() {}, lastEgoAccounts: [], renderThroughputPanel() {}, updateTabCounts() {}, render() {}, refreshAll() {}, escapeHtml: String,
  });
  const auth = html.slice(html.indexOf('  function deobfuscateCPAMC('), html.indexOf('  async function fetchFileDetails('));
  vm.runInContext(auth, context);
  const ego = html.slice(html.indexOf('  async function loadEgoOverview()'), html.indexOf('  function formatEgoTimelineLabel('));
  const egoFetch = html.slice(html.indexOf('  async function egoApiFetch('), html.indexOf('  function formatCompactTokens('));
  const egoSection = html.slice(html.indexOf('  async function fetchEgoSection('), html.indexOf('  async function loadEgoOverview('));
  vm.runInContext(`let egoLoadVersion = 0, egoLoadsPending = 0, egoRefreshWasDisabled = false, currentEgoRange = '24h';
    function showToast() {}\n` + egoFetch + egoSection + ego, context);
  return { context, element, requests, writes };
}
function encoded(version) {
  const plain = JSON.stringify({ state: { managementKey: 'synthetic-key' } });
  if (version === 'plain') return plain;
  const key = Buffer.from('cli-proxy-api-webui::secure-storage|panel.test:8317' + (version === 'v1' ? '|test-browser' : ''));
  return `enc::${version}::` + Buffer.from([...Buffer.from(plain)].map((byte, i) => byte ^ key[i % key.length])).toString('base64');
}
for (const version of ['plain', 'v1', 'v2']) {
  test(`${version} remembered key reaches existing authenticated HTTP loader`, async () => {
    const d = dashboard(encoded(version));
    await d.context.loadAuthFiles();
    assert.equal(d.requests[0]?.url, '/v0/management/auth-files');
    assert.equal(d.requests[0]?.options.headers.Authorization, 'Bearer synthetic-key');
    assert.doesNotMatch(d.element('grid').innerHTML, /type="password"/);
    assert.deepEqual(d.writes, []);
  });
}
test('missing key offers manual entry, clears input, reuses loader and never persists', async () => {
  const d = dashboard(null);
  await d.context.loadAuthFiles();
  assert.match(d.element('grid').innerHTML, /Authentication Required/);
  assert.match(d.element('grid').innerHTML, /type="password"/);
  assert.equal(d.requests.length, 0);
  const form = d.element('grid').querySelector('form');
  const input = form.querySelector('input');
  input.value = 'synthetic-manual';
  await form.submit({ preventDefault() {} });
  assert.equal(input.value, '');
  assert.equal(d.requests[0].options.headers.Authorization, 'Bearer synthetic-manual');
  assert.deepEqual(d.writes, []);
});
test('manual entry is shared across requests but lost in a new page', async () => {
  const d = dashboard();
  await d.context.loadAuthFiles();
  const form = d.element('grid').querySelector('form');
  form.querySelector('input').value = 'synthetic-page-key';
  await form.submit({ preventDefault() {} });
  await d.context.apiFetch('/v0/management/ego/settings');
  assert.equal(d.requests.at(-1).options.headers.Authorization, 'Bearer synthetic-page-key');
  assert.equal(dashboard().context.getManagementKey(), '');
  assert.doesNotMatch(d.element('grid').innerHTML, /synthetic-page-key/);
  assert.deepEqual(d.writes, []);
});

test('wrong remembered key keeps existing sign-in flow without manual override', async () => {
  const d = dashboard(encoded('v2'), 401);
  await d.context.loadAuthFiles();
  assert.match(d.element('grid').innerHTML, /401 Unauthorized/);
  assert.doesNotMatch(d.element('grid').innerHTML, /type="password"/);
  assert.equal(d.requests[0].options.headers.Authorization, 'Bearer synthetic-key');
});

test('manual wrong key is visible and allows retry without retaining input', async () => {
  const d = dashboard(null, 401);
  await d.context.loadAuthFiles();
  const form = d.element('grid').querySelector('form');
  const input = form.querySelector('input');
  input.value = 'synthetic-wrong';
  await form.submit({ preventDefault() {} });
  assert.match(d.element('grid').innerHTML, /401 Unauthorized/);
  assert.match(d.element('grid').innerHTML, /type="password"/);
  assert.equal(input.value, '');
  assert.equal(d.context.getManagementKey(), '');
  assert.deepEqual(d.writes, []);
});
test('Ego missing-key form reuses authenticated section loader and shows rejected-key retry', async () => {
  const d = dashboard(null, 401);
  await d.context.loadEgoOverview();
  assert.equal(d.requests.length, 0);
  assert.match(d.element('ego-auth-warning').innerHTML, /type="password"/);
  const form = d.element('ego-auth-warning').querySelector('form');
  const input = form.querySelector('input');
  input.value = 'synthetic-ego';
  await form.submit({ preventDefault() {} });
  assert.equal(d.requests.length, 6);
  for (const request of d.requests) {
    assert.match(request.url, /^\/v0\/management\/ego\//);
    assert.equal(request.options.headers.Authorization, 'Bearer synthetic-ego');
  }
  assert.match(d.element('ego-auth-warning').innerHTML, /401 Unauthorized/);
  assert.equal(input.value, '');
  assert.equal(d.context.getManagementKey(), '');
  assert.deepEqual(d.writes, []);
});

test('unreadable remembered data falls back to manual; parent denial retains local legacy read', async () => {
  const missing = dashboard('enc::v2::invalid!');
  await missing.context.loadAuthFiles();
  assert.match(missing.element('grid').innerHTML, /type="password"/);
  const legacy = dashboard(encoded('v1'), 200, true);
  await legacy.context.loadAuthFiles();
  assert.equal(legacy.requests[0].options.headers.Authorization, 'Bearer synthetic-key');
});
