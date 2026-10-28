// Injected before the dashboard SPA bundle. Rewrites every /api/...
// fetch to /fixtures/api/...json, fakes a successful /__login (the
// real one is gated by a password), and replaces EventSource with a
// looped replay of fixtures/events.jsonl.
//
// Updates: when the SPA evolves, no JS change here is needed unless
// the SPA hits a NEW endpoint shape. Fixtures live in site/demo/fixtures/.

(() => {
  // The demo is served from the site under /demo/ (see
  // site/scripts/build-demo.sh). The dashboard SPA fetches absolute
  // paths like "/api/foo"; those are rewritten to "/demo/fixtures/...".
  const BASE = '/demo';
  const FIXTURES = BASE + '/fixtures';

  // Live-event pacing: replay uses real timestamp deltas × EVENTS_SCALE.
  // Fixture events are ~400 ms apart → 400 × 3 = 1.2 s per event at scale 3.
  // Override via localStorage for debugging.
  const EVENTS_SCALE = parseFloat(localStorage.getItem('demo_events_scale') || '3');
  const EVENTS_MIN_MS = 700;   // never faster than this
  const EVENTS_MAX_MS = 4000;  // cap long gaps
  const EVENTS_FALLBACK_MS = 1200; // used when delta can't be computed
  const STATE_POLL_MS = parseInt(localStorage.getItem('demo_state_poll_ms') || '1500', 10);

  // --- Demo scenario ---
  // demoStartMs is set when the EventSource opens. All scripted timings
  // are absolute milliseconds from that moment, independent of how fast
  // the round-robin events fire.
  let demoStartMs = 0;

  const SCRIPTED_EVENTS = [
    // 3 s: force-push blocked by rule
    {
      _ms: 3000,
      id: 'act_aq_deny_push',
      mode: 'mitm',
      agent_ip: '100.99.99.1',
      host: 'github.com',
      method: 'POST',
      path: '/alderquay-cu/member-portal.git/git-receive-pack',
      status: 403,
      ms: 82,
      action: 'denied',
      rule: 'github:block-force-push',
      reason: 'force push to protected branch',
      family: 'http',
      facets: { method: 'POST', path: '/alderquay-cu/member-portal.git/git-receive-pack' },
      endpoint: 'github-api',
    },
    // 9 s: LLM judge denial with plain-English reason
    {
      _ms: 9000,
      id: 'act_aq_deny_llm',
      mode: 'mitm',
      agent_ip: '100.76.162.118',
      host: 'api.anthropic.com',
      method: 'POST',
      path: '/v1/messages',
      status: 200,
      ms: 4212,
      action: 'denied',
      rule: 'anthropic-api:content-guard',
      reason: "reply body contains the word 'moron'",
      approver: 'content-guard',
      approver_type: 'llm_approver',
      approver_by: 'llm:claude-sonnet-4-5',
      family: 'http',
      facets: { method: 'POST', path: '/v1/messages' },
      endpoint: 'anthropic-api',
    },
    // 16 s: postgres HITL
    {
      _ms: 16000,
      id: 'act_aq_approval_002',
      phase: 'start',
      mode: 'pg',
      agent_ip: '100.99.99.1',
      host: 'users-db',
      method: 'SELECT',
      path: 'env_vars',
      action: 'hitl_pending',
      rule: 'postgres:no-secret-reads',
      family: 'sql',
      facets: { verb: 'SELECT', tables: ['env_vars'] },
      endpoint: 'users-db',
    },
    // 21 s: kubectl HITL (staggered)
    {
      _ms: 21000,
      id: 'act_aq_approval_003',
      phase: 'start',
      mode: 'mitm',
      agent_ip: '100.76.162.118',
      host: 'k8s-dev-ord',
      method: 'DELETE',
      path: 'pods/deployd-hfk2v',
      action: 'hitl_pending',
      rule: 'k8s:destructive-ops',
      family: 'k8s',
      facets: { verb: 'DELETE', resource: 'pods', name: 'deployd-hfk2v' },
      endpoint: 'k8s-dev-ord',
    },
    // 28 s: both resolve
    {
      _ms: 28000,
      id: 'act_aq_approval_002',
      phase: 'end',
      mode: 'pg',
      agent_ip: '100.99.99.1',
      host: 'users-db',
      method: 'SELECT',
      path: 'env_vars',
      status: 200,
      ms: 1203,
      action: 'approved',
      rule: 'postgres:no-secret-reads',
      approver: 'team-approvals',
      approver_type: 'human_approver',
      approver_by: 'maya.chen@alderquay.internal',
      family: 'sql',
      facets: { verb: 'SELECT', tables: ['env_vars'] },
      endpoint: 'users-db',
    },
    {
      _ms: 28000,
      id: 'act_aq_approval_003',
      phase: 'end',
      mode: 'mitm',
      agent_ip: '100.76.162.118',
      host: 'k8s-dev-ord',
      method: 'DELETE',
      path: 'pods/deployd-hfk2v',
      status: 200,
      ms: 4412,
      action: 'approved',
      rule: 'k8s:destructive-ops',
      approver: 'team-approvals',
      approver_type: 'human_approver',
      approver_by: 'maya.chen@alderquay.internal',
      family: 'k8s',
      facets: { verb: 'DELETE', resource: 'pods', name: 'deployd-hfk2v' },
      endpoint: 'k8s-dev-ord',
    },
  ];

  // Index scripted events by id so the detail page can fetch them.
  // Later entries (same id) win — so act_aq_approval_002 resolves to the
  // approved event when the user clicks after it's resolved.
  const SCRIPTED_BY_ID = {};
  for (const { _ms, ...rest } of SCRIPTED_EVENTS) {
    SCRIPTED_BY_ID[rest.id] = rest;
  }

  // HITL pending bar: postgres from 16 s, kubectl joins at 21 s, both clear at 28 s.
  const HITL_START_MS = 16000;
  const HITL_KUBECTL_MS = 21000;
  const HITL_END_MS = 28000;

  function fakeHITLItems() {
    const elapsed = demoStartMs > 0 ? Date.now() - demoStartMs : 0;
    const now = new Date();
    const exp = new Date(now.getTime() + 60000).toISOString();
    const ts = now.toISOString();
    const items = [];
    if (elapsed >= HITL_START_MS) {
      items.push({
        id: 'act_aq_approval_002',
        agent_ip: '100.99.99.1',
        host: 'users-db',
        method: 'SELECT',
        path: 'env_vars',
        operation_state: 'sync_waiting',
        approval_effect: 'execute_upstream',
        body_sample: 'SELECT * FROM env_vars',
        reason: 'query reads from secrets table · postgres:no-secret-reads',
        ua: 'engineering-client',
        endpoint: 'users-db',
        family: 'sql',
        created_at: ts,
        expires_at: exp,
      });
    }
    if (elapsed >= HITL_KUBECTL_MS) {
      items.push({
        id: 'act_aq_approval_003',
        agent_ip: '100.76.162.118',
        host: 'k8s-dev-ord',
        method: 'DELETE',
        path: 'pods/deployd-hfk2v',
        operation_state: 'sync_waiting',
        approval_effect: 'execute_upstream',
        body_sample: 'kubectl delete pod deployd-hfk2v -n default',
        reason: 'destructive op requires approval · k8s:destructive-ops',
        ua: 'engineering-client',
        endpoint: 'k8s-dev-ord',
        family: 'k8s',
        created_at: ts,
        expires_at: exp,
      });
    }
    return items;
  }

  // --- setInterval shim ---
  // The SPA polls /api/state on a 5 s cadence. For a live-looking demo
  // that's too slow — the activity sparkline barely moves. Shrink the
  // interval here so the sparkline visibly walks; only the exact
  // 5000 ms timers get rewritten so other intervals (1 s HITL poll,
  // etc.) stay untouched.
  const origSetInterval = window.setInterval;
  window.setInterval = function (fn, ms, ...rest) {
    if (ms === 5000) return origSetInterval.call(this, fn, STATE_POLL_MS, ...rest);
    return origSetInterval.call(this, fn, ms, ...rest);
  };

  // --- fetch interception ---
  const origFetch = window.fetch.bind(window);
  window.fetch = (input, init) => {
    let url = typeof input === 'string' ? input : input.url;
    const method = (init && init.method) || (typeof input !== 'string' && input.method) || 'GET';

    // Fake login success.
    if (url.includes('/__login')) {
      return Promise.resolve(new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }));
    }

    // HITL pending: return scripted pending list during the demo window.
    if (url === '/api/hitl/pending') {
      const elapsed = demoStartMs > 0 ? Date.now() - demoStartMs : 0;
      const pending =
        elapsed >= HITL_START_MS && elapsed < HITL_END_MS
          ? fakeHITLItems()
          : [];
      return Promise.resolve(new Response(JSON.stringify(pending), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }));
    }

    // Refuse mutating verbs on /api/.
    if (url.startsWith('/api/') && method !== 'GET' && method !== 'HEAD') {
      return Promise.resolve(new Response(JSON.stringify({ error: 'Read-only access' }), {
        status: 403,
        headers: { 'Content-Type': 'application/json' },
      }));
    }

    // Serve scripted event detail pages without needing fixture files.
    const actionMatch = url.match(/^\/api\/actions\/([^?]+)/);
    if (actionMatch) {
      const scriptedEv = SCRIPTED_BY_ID[actionMatch[1]];
      if (scriptedEv) {
        return Promise.resolve(new Response(JSON.stringify({ ...scriptedEv, ts: new Date().toISOString() }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }));
      }
    }

    // Rewrite /api/foo[?...] → /fixtures/api/foo[__q__...].json. If
    // the param-encoded variant 404s (we don't crawl every query
    // permutation), fall back to the bare /fixtures/api/foo.json so
    // analytics etc. render with the canonical snapshot.
    if (url.startsWith('/api/') || url === '/info' || url === '/ca.crt') {
      const rel = url.replace(/^\//, '').replace(/^api\//, 'api/');
      const qi = rel.indexOf('?');
      const bare = qi < 0 ? rel : rel.slice(0, qi);
      const fname = rel
        .replace(/\?/g, '__q__')
        .replace(/&/g, '__a__')
        .replace(/=/g, '__e__');
      if (url === '/info' || url === '/ca.crt') {
        return origFetch(FIXTURES + url, { method: 'GET' });
      }
      // /api/analytics drives the per-range latency chart and the
      // device/host tables. Synthesize 5000 events spread across the
      // requested range from the events.jsonl templates so the 24 h
      // view doesn't render empty (the crawled fixture only has 15
      // events and no time spread).
      if (bare === 'api/analytics') {
        const u = new URL(url, location.origin);
        const range = u.searchParams.get('range') || '1h';
        const RANGE_MS = {
          '1m': 60e3, '5m': 300e3, '15m': 900e3, '30m': 1800e3,
          '1h': 3600e3, '6h': 21600e3, '24h': 86400e3,
        };
        const lookback = RANGE_MS[range] || RANGE_MS['1h'];
        return loadEvents().then((templates) => {
          if (!templates.length) {
            return new Response('{"events":[],"total":0,"total_count":0,"error_count":0,"by_device":[],"by_host":[]}', {
              status: 200, headers: { 'Content-Type': 'application/json' },
            });
          }
          const N = 5000;
          const now = Date.now();
          const events = new Array(N);
          const byDevice = new Map();
          const byHost = new Map();
          let errors = 0;
          for (let k = 0; k < N; k++) {
            const t = templates[Math.floor(Math.random() * templates.length)];
            const ts = new Date(now - Math.floor(Math.random() * lookback)).toISOString();
            // Sprinkle ~3 % errors so the err% column isn't always 0.
            const isErr = Math.random() < 0.03;
            const status = isErr ? (Math.random() < 0.5 ? 500 : 403) : (t.status || 200);
            if (isErr) errors++;
            const ev = { ...t, ts, status };
            events[k] = ev;
            byDevice.set(ev.agent_ip, (byDevice.get(ev.agent_ip) || 0) + 1);
            byHost.set(ev.host, (byHost.get(ev.host) || 0) + 1);
          }
          const body = {
            events,
            total: N,
            total_count: N,
            error_count: errors,
            by_device: [...byDevice].map(([key, count]) => ({ key, count })).sort((a, b) => b.count - a.count),
            by_host: [...byHost].map(([key, count]) => ({ key, count })).sort((a, b) => b.count - a.count),
          };
          return new Response(JSON.stringify(body), {
            status: 200, headers: { 'Content-Type': 'application/json' },
          });
        });
      }
      // /api/state drives the home-page agent table + activity
      // sparkline. The fixture is frozen; rewrite each poll so the
      // sparkline rotates and last_at stays "live" instead of decaying
      // into "2h ago" forever.
      if (bare === 'api/state') {
        return origFetch(FIXTURES + '/api/state.json').then((r) => r.json()).then((s) => {
          const now = new Date().toISOString();
          for (const a of (s.agents || [])) {
            a.last_at = now;
            if (Array.isArray(a.activity) && a.activity.length > 0) {
              const head = Math.max(1, Math.floor(Math.random() * 4));
              a.activity = [...a.activity.slice(1), head];
              a.reqs = (a.reqs || 0) + head;
            }
          }
          return new Response(JSON.stringify(s), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          });
        });
      }
      // The config fixture is a JSON string; the editor expects raw HCL.
      if (bare === 'api/config') {
        return origFetch(FIXTURES + '/api/config.json').then((r) => r.json()).then((config) =>
          new Response(config, { headers: { 'Content-Type': 'text/plain' } }));
      }
      const primary = FIXTURES + '/' + fname + '.json';
      const fallback = FIXTURES + '/' + bare + '.json';
      return origFetch(primary, { method: 'GET' }).then((r) => {
        if (r.ok || primary === fallback) return r;
        return origFetch(fallback, { method: 'GET' });
      });
    }

    return origFetch(input, init);
  };

  // --- EventSource replay ---
  let cachedEvents = null;
  async function loadEvents() {
    if (cachedEvents) return cachedEvents;
    try {
      const r = await origFetch(FIXTURES + '/events.jsonl');
      const text = await r.text();
      cachedEvents = text
        .split('\n')
        .filter((l) => l.trim().length > 0)
        .map((l) => {
          try { return JSON.parse(l); } catch { return null; }
        })
        .filter(Boolean);
    } catch (e) {
      console.warn('demo: events.jsonl load failed', e);
      cachedEvents = [];
    }
    return cachedEvents;
  }

  class FakeEventSource extends EventTarget {
    constructor(url) {
      super();
      this.url = url;
      this.readyState = 0; // CONNECTING
      this.withCredentials = false;
      this._closed = false;
      this._start();
    }

    async _start() {
      const agent = new URL(this.url, location.origin).searchParams.get('agent');
      const matchesAgent = (event) => !agent || event.agent_ip === agent;
      const events = (await loadEvents()).filter(matchesAgent);
      if (this._closed) return;
      this.readyState = 1; // OPEN

      // Record start time so scripted event timings and HITL window are
      // anchored to when the replay actually begins (after fixture load).
      demoStartMs = Date.now();

      // Emit backlog event up front so dashboard renders the recent
      // history immediately.
      const backlog = events.filter((e) => e && (e.host || e.id));
      this._dispatch('backlog', backlog);

      // Scripted events fire at absolute offsets from demoStartMs,
      // independent of the round-robin cadence.
      for (const { _ms, ...rest } of SCRIPTED_EVENTS) {
        if (!matchesAgent(rest)) continue;
        setTimeout(() => {
          if (this._closed) return;
          this._dispatch(null, { ...rest, ts: new Date().toISOString() });
        }, _ms);
      }

      if (events.length === 0) return;

      // Round-robin live events paced by real timestamp deltas × EVENTS_SCALE.
      // This makes playback feel like a real session: if two events in
      // the fixture are 2 s apart, they appear 2 × EVENTS_SCALE s apart.
      let i = 0;
      const tick = () => {
        if (this._closed) return;
        const ev = events[i % events.length];
        const nextEv = events[(i + 1) % events.length];

        this._dispatch(null, { ...ev, ts: new Date().toISOString() });
        i++;

        // Compute delay from real timestamp delta; fall back when
        // timestamps are missing or the delta is non-positive (loop wrap).
        let delay = EVENTS_FALLBACK_MS;
        if (ev.ts && nextEv && nextEv.ts) {
          const delta = new Date(nextEv.ts).getTime() - new Date(ev.ts).getTime();
          if (delta > 0) {
            delay = Math.max(EVENTS_MIN_MS, Math.min(EVENTS_MAX_MS, delta * EVENTS_SCALE));
          }
        }
        setTimeout(tick, delay);
      };
      setTimeout(tick, 500);
    }

    _dispatch(name, data) {
      const ev = new MessageEvent(name || 'message', {
        data: JSON.stringify(data),
      });
      this.dispatchEvent(ev);
      if (!name && typeof this.onmessage === 'function') this.onmessage(ev);
    }

    close() {
      this._closed = true;
      this.readyState = 2; // CLOSED
    }
  }
  FakeEventSource.CONNECTING = 0;
  FakeEventSource.OPEN = 1;
  FakeEventSource.CLOSED = 2;
  window.EventSource = FakeEventSource;

})();
