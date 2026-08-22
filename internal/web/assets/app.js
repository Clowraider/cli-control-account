/**
 * Control Account Quota Dashboard SPA
 * Handles provider tab filtering, search, countdown timers, and quota progress gauges.
 */

(function () {
  'use strict';

  // State
  let currentProvider = 'all';
  let searchQuery = '';
  let accountsData = [];
  let countdownTimerId = null;

  // DOM Elements
  const tabs = document.querySelectorAll('.tab-btn');
  const searchInput = document.getElementById('search-input');
  const refreshBtn = document.getElementById('refresh-btn');
  const cardsGrid = document.getElementById('cards-grid');
  const loadingState = document.getElementById('loading-state');
  const emptyState = document.getElementById('empty-state');
  const totalAccountsCount = document.getElementById('total-accounts-count');
  const activeWindowsCount = document.getElementById('active-windows-count');

  // Initialization
  function init() {
    setupEventListeners();
    fetchQuotaData();
    startCountdownLoop();
  }

  // Setup Event Listeners
  function setupEventListeners() {
    tabs.forEach((tab) => {
      tab.addEventListener('click', () => {
        tabs.forEach((t) => {
          t.classList.remove('active');
          t.setAttribute('aria-selected', 'false');
        });
        tab.classList.add('active');
        tab.setAttribute('aria-selected', 'true');
        currentProvider = tab.dataset.provider.toLowerCase();
        render();
      });
    });

    if (searchInput) {
      searchInput.addEventListener('input', (e) => {
        searchQuery = e.target.value.trim().toLowerCase();
        render();
      });
    }

    if (refreshBtn) {
      refreshBtn.addEventListener('click', () => {
        fetchQuotaData();
      });
    }
  }

  // Fetch Quota / Auth Files Data
  async function fetchQuotaData() {
    showLoading(true);
    try {
      // Try management API endpoint first
      const res = await fetch('/v0/management/auth-files', {
        headers: { Accept: 'application/json' },
      });

      if (res.ok) {
        const json = await res.json();
        accountsData = parseAccountsFromResponse(json);
      } else {
        // Fallback to sample accounts if running standalone/dev mode
        accountsData = getSampleAccounts();
      }
    } catch (err) {
      console.warn('Could not fetch from /v0/management/auth-files, using local sample state', err);
      accountsData = getSampleAccounts();
    } finally {
      showLoading(false);
      render();
    }
  }

  // Parse Management API Response into Account Quota objects
  function parseAccountsFromResponse(payload) {
    if (!payload) return [];
    const rawList = Array.isArray(payload) ? payload : payload.files || payload.accounts || [];
    return rawList.map((item, idx) => {
      const id = item.id || item.filename || item.name || `account-${idx + 1}`;
      const provider = (item.provider || item.type || 'claude').toLowerCase();
      const prefix = item.prefix || item.account_prefix || '';
      
      const windows = (item.quota_windows || item.windows || [
        {
          name: 'Token Quota',
          used: item.tokens_used || 0,
          limit: item.tokens_limit || 100000,
          resets_at: item.resets_at || new Date(Date.now() + 3600000).toISOString(),
        }
      ]).map((w) => ({
        name: w.name || 'Requests',
        used: Number(w.used || 0),
        limit: Number(w.limit || 1000),
        resetsAt: new Date(w.resets_at || w.reset_at || w.resetsAt || Date.now() + 3600000),
      }));

      return { id, provider, prefix, windows };
    });
  }

  // Sample data fallback for local standalone testing
  function getSampleAccounts() {
    const now = Date.now();
    return [
      {
        id: 'acc_claude_pro_01',
        provider: 'claude',
        prefix: 'team-alpha',
        windows: [
          {
            name: 'Hourly Rate Limit',
            used: 42000,
            limit: 50000,
            resetsAt: new Date(now + 25 * 60 * 1000 + 30 * 1000),
          },
          {
            name: 'Daily Token Window',
            used: 350000,
            limit: 1000000,
            resetsAt: new Date(now + 8 * 3600 * 1000),
          },
        ],
      },
      {
        id: 'acc_codex_shared_prod',
        provider: 'codex',
        prefix: '', // No prefix -> tests '-' fallback
        windows: [
          {
            name: 'Requests per Minute',
            used: 980,
            limit: 1000,
            resetsAt: new Date(now + 45 * 1000),
          },
        ],
      },
      {
        id: 'acc_antigravity_core',
        provider: 'antigravity',
        prefix: 'cluster-west',
        windows: [
          {
            name: 'Compute Credits',
            used: 120,
            limit: 1000,
            resetsAt: new Date(now + 18 * 3600 * 1000),
          },
        ],
      },
      {
        id: 'acc_kimi_moonshot_01',
        provider: 'kimi',
        prefix: 'research-nlp',
        windows: [
          {
            name: 'Context Window Quota',
            used: 850000,
            limit: 1000000,
            resetsAt: new Date(now + 3 * 3600 * 1000 + 12 * 60 * 1000),
          },
        ],
      },
      {
        id: 'acc_xai_grok_fast',
        provider: 'xai',
        prefix: '', // Empty prefix -> tests '-' fallback
        windows: [
          {
            name: 'API Calls Window',
            used: 250,
            limit: 500,
            resetsAt: new Date(now + 42 * 60 * 1000),
          },
        ],
      },
    ];
  }

  // Main Render Routine
  function render() {
    const filtered = filterAccounts();
    updateSummaryStats(filtered);

    if (filtered.length === 0) {
      cardsGrid.innerHTML = '';
      emptyState.classList.remove('hidden');
      return;
    }

    emptyState.classList.add('hidden');
    cardsGrid.innerHTML = filtered.map(renderAccountCard).join('');
  }

  // Filter Accounts by Provider Tab and Search Query
  function filterAccounts() {
    return accountsData.filter((account) => {
      const matchProvider = currentProvider === 'all' || account.provider.toLowerCase() === currentProvider;
      if (!matchProvider) return false;

      if (!searchQuery) return true;
      const idMatch = account.id.toLowerCase().includes(searchQuery);
      const prefixMatch = (account.prefix || '').toLowerCase().includes(searchQuery);
      const providerMatch = account.provider.toLowerCase().includes(searchQuery);
      return idMatch || prefixMatch || providerMatch;
    });
  }

  // Update Summary Counts
  function updateSummaryStats(filtered) {
    if (totalAccountsCount) {
      totalAccountsCount.textContent = filtered.length;
    }
    if (activeWindowsCount) {
      const totalWindows = filtered.reduce((acc, a) => acc + (a.windows ? a.windows.length : 0), 0);
      activeWindowsCount.textContent = totalWindows;
    }
  }

  // Render a Single Account Quota Card
  function renderAccountCard(account) {
    const providerClass = escapeHtml(account.provider.toLowerCase());
    const prefixDisplay = account.prefix && account.prefix.trim() !== ''
      ? `<span class="account-prefix-text">${escapeHtml(account.prefix)}</span>`
      : `<span class="prefix-fallback">-</span>`;

    const windowsHtml = (account.windows || []).map((w, idx) => renderWindowItem(w, account.id, idx)).join('');

    return `
      <article class="quota-card" data-account-id="${escapeHtml(account.id)}">
        <div class="card-header">
          <div class="card-title-row">
            <h2 class="account-id">${escapeHtml(account.id)}</h2>
            <span class="provider-tag ${providerClass}">${escapeHtml(account.provider)}</span>
          </div>
          <div class="account-prefix-container" title="Account Prefix: ${escapeHtml(account.prefix || 'None')}">
            ${prefixDisplay}
          </div>
        </div>
        <div class="card-body">
          ${windowsHtml}
        </div>
      </article>
    `;
  }

  // Render a Quota Window Progress Row
  function renderWindowItem(win, accountId, winIndex) {
    const used = Number(win.used) || 0;
    const limit = Number(win.limit) || 1;
    const pct = Math.min(100, Math.max(0, Math.round((used / limit) * 100)));

    let statusClass = 'normal';
    if (pct >= 95) {
      statusClass = 'critical';
    } else if (pct >= 80) {
      statusClass = 'warning';
    }

    const remaining = Math.max(0, limit - used);
    const countdownStr = formatCountdown(win.resetsAt);

    return `
      <div class="window-item">
        <div class="window-header">
          <span class="window-name">${escapeHtml(win.name)}</span>
          <span class="window-metrics">${formatNumber(used)} / ${formatNumber(limit)} (${pct}%)</span>
        </div>
        <div class="meter-container" role="progressbar" aria-valuenow="${pct}" aria-valuemin="0" aria-valuemax="100">
          <div class="meter-fill ${statusClass}" style="width: ${pct}%;"></div>
        </div>
        <div class="window-footer">
          <span>Remaining: ${formatNumber(remaining)}</span>
          <span class="countdown-badge ${pct >= 90 ? 'urgent' : ''}" data-resets-at="${win.resetsAt.getTime()}">
            ⏱ ${countdownStr}
          </span>
        </div>
      </div>
    `;
  }

  // Calculate and Format Countdown String
  function formatCountdown(targetDate) {
    if (!targetDate || isNaN(targetDate.getTime())) return '00:00:00';
    const now = Date.now();
    const diff = targetDate.getTime() - now;

    if (diff <= 0) return '00:00:00 (due)';

    const totalSeconds = Math.floor(diff / 1000);
    const hours = Math.floor(totalSeconds / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    const seconds = totalSeconds % 60;

    const pad = (n) => String(n).padStart(2, '0');
    if (hours > 0) {
      return `${pad(hours)}:${pad(minutes)}:${pad(seconds)}`;
    }
    return `${pad(minutes)}:${pad(seconds)}`;
  }

  // Global Countdown Loop (ticks every second)
  function startCountdownLoop() {
    if (countdownTimerId) clearInterval(countdownTimerId);

    countdownTimerId = setInterval(() => {
      const badgeElements = document.querySelectorAll('.countdown-badge[data-resets-at]');
      let needsRefresh = false;

      badgeElements.forEach((badge) => {
        const timestamp = Number(badge.dataset.resetsAt);
        if (!isNaN(timestamp)) {
          const target = new Date(timestamp);
          const diff = timestamp - Date.now();
          if (diff <= 0 && diff > -2000) {
            needsRefresh = true;
          }
          badge.textContent = `⏱ ${formatCountdown(target)}`;
        }
      });

      if (needsRefresh) {
        // Trigger background refresh when timer reaches 0
        fetchQuotaData();
      }
    }, 1000);
  }

  // Helpers
  function showLoading(show) {
    if (loadingState) {
      if (show) loadingState.classList.remove('hidden');
      else loadingState.classList.add('hidden');
    }
  }

  function formatNumber(num) {
    return new Intl.NumberFormat('en-US').format(num);
  }

  function escapeHtml(str) {
    if (typeof str !== 'string') return '';
    return str
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#039;');
  }

  // Bootstrap when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
