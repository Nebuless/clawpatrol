// Run: node site/demo/demo-mode.test.cjs
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const path = require('node:path');
const fixtures = path.join(__dirname, 'fixtures');
const events = fs.readFileSync(path.join(fixtures, 'events.jsonl'), 'utf8').trim().split('\n').map(JSON.parse);
for (const event of events) {
  assert.deepEqual(JSON.parse(fs.readFileSync(path.join(fixtures, 'api/actions', `${event.id}.json`))), event);
}
for (const event of JSON.parse(fs.readFileSync(path.join(fixtures, 'api/analytics.json'))).events) {
  assert.ok(fs.existsSync(path.join(fixtures, 'api/actions', `${event.id}.json`)));
}
async function check(agent) {
  const timers = [];
  const messages = [];
  const window = {
    setInterval() {},
    fetch: async (input) => new Response(fs.readFileSync(path.join(fixtures, input.replace('/demo/fixtures/', '')))),
  };
  const context = { window, EventTarget, MessageEvent, Response, URL, Date, console,
    location: { origin: 'https://demo.example.com' },
    localStorage: { getItem: () => null },
    document: { addEventListener() {} },
    setTimeout: (fn, ms) => timers.push({ fn, ms }),
  };
  vm.runInNewContext(fs.readFileSync(path.join(__dirname, 'demo-mode.js'), 'utf8'), context);
  const source = new window.EventSource(`/api/events${agent ? `?agent=${encodeURIComponent(agent)}` : ''}`);
  source.addEventListener('backlog', (e) => messages.push(...JSON.parse(e.data)));
  source.onmessage = (e) => messages.push(JSON.parse(e.data));
  await new Promise(setImmediate);
  const backlogSize = messages.length;
  assert.equal(backlogSize, events.filter((e) => !agent || e.agent_ip === agent).length);
  // Execute each initial replay/script timer once, without following the replay loop.
  for (const timer of [...timers]) timer.fn();
  if (agent) assert.ok(messages.every((e) => e.agent_ip === agent));
  if (!agent) assert.ok(messages.some((e) => e.id === 'demo-deny-push'));
  source.close();
  const analytics = await (await window.fetch('/api/analytics?range=24h')).json();
  assert.equal(analytics.events.length, 5000);
  for (const id of new Set(analytics.events.map((e) => e.id))) {
    assert.equal((await window.fetch(`/api/actions/${id}`)).status, 200);
  }
  assert.match(await (await window.fetch('/api/config')).text(), /^info_listen/);
}
(async () => {
  for (const agent of [null, ...new Set(events.map((e) => e.agent_ip)), '100.99.99.254']) await check(agent);
  console.log('PASS: fixture consistency, analytics details, config text, filtered backlog/replay/scripts, unknown agent');
})().catch((error) => { console.error(error); process.exitCode = 1; });
