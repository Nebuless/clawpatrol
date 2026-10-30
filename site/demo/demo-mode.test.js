import assert from "node:assert/strict";
import fs from "node:fs";
import vm from "node:vm";
import path from "node:path";
import { test } from "vite-plus/test";
const fixtures = path.join(import.meta.dirname, "fixtures");
const events = fs
  .readFileSync(path.join(fixtures, "events.jsonl"), "utf8")
  .trim()
  .split("\n")
  .map(JSON.parse);
test("fixtures are consistent and scrubbed", () => {
  for (const event of events) {
    assert.deepEqual(
      JSON.parse(fs.readFileSync(path.join(fixtures, "api/actions", `${event.id}.json`))),
      event,
    );
  }
  for (const event of JSON.parse(fs.readFileSync(path.join(fixtures, "api/analytics.json")))
    .events) {
    assert.ok(fs.existsSync(path.join(fixtures, "api/actions", `${event.id}.json`)));
  }
  // Keep fixture text free of staging identifiers, including nested request bodies.
  for (const file of fs.readdirSync(fixtures, { recursive: true })) {
    const fullPath = path.join(fixtures, file);
    if (!fs.statSync(fullPath).isFile()) continue;
    const text = fs.readFileSync(fullPath, "utf8");
    if (file.endsWith(".json")) JSON.parse(text);
    if (file.endsWith(".jsonl")) text.trim().split("\n").forEach(JSON.parse);
    assert.doesNotMatch(
      text,
      /example|demo-|sample|fake|dummy|placeholder|mock|\.test\b|replay/i,
      file,
    );
  }
});
async function check(agent) {
  const timers = [];
  const messages = [];
  const window = {
    setInterval() {},
    fetch: async (input) =>
      new Response(fs.readFileSync(path.join(fixtures, input.replace("/demo/fixtures/", "")))),
  };
  const context = {
    window,
    EventTarget,
    MessageEvent,
    Response,
    URL,
    Date,
    console,
    location: { origin: "https://prx.alderquay.internal" },
    localStorage: { getItem: () => null },
    document: {
      addEventListener() {
        assert.fail("The shim must not inject visible chrome");
      },
    },
    setTimeout: (fn, ms) => timers.push({ fn, ms }),
  };
  vm.runInNewContext(
    fs.readFileSync(path.join(import.meta.dirname, "demo-mode.js"), "utf8"),
    context,
  );
  const source = new window.EventSource(
    `/api/events${agent ? `?agent=${encodeURIComponent(agent)}` : ""}`,
  );
  source.addEventListener("backlog", (e) => messages.push(...JSON.parse(e.data)));
  source.onmessage = (e) => messages.push(JSON.parse(e.data));
  await new Promise(setImmediate);
  const backlogSize = messages.length;
  assert.equal(backlogSize, events.filter((e) => !agent || e.agent_ip === agent).length);
  // Execute each initial replay/script timer once, without following the replay loop.
  const initialTimers = timers.slice();
  for (const timer of initialTimers) timer.fn();
  if (agent) assert.ok(messages.every((e) => e.agent_ip === agent));
  if (!agent) assert.ok(messages.some((e) => e.id === "act_aq_deny_push"));
  source.close();
  const analytics = await (await window.fetch("/api/analytics?range=24h")).json();
  assert.equal(analytics.events.length, 5000);
  for (const id of new Set(analytics.events.map((e) => e.id))) {
    assert.equal((await window.fetch(`/api/actions/${id}`)).status, 200);
  }
  assert.match(await (await window.fetch("/api/config")).text(), /^info_listen/);
}
test("filtered streams and scripted requests render from fixtures", async () => {
  for (const agent of [null, ...new Set(events.map((e) => e.agent_ip)), "100.99.99.254"])
    await check(agent);
});

test("fetch accepts URL and Request inputs while retaining read-only API policy", async () => {
  const origin = "https://prx.alderquay.internal";
  const passedThrough = [];
  const window = {
    setInterval() {},
    fetch: async (input) => {
      if (typeof input === "string" && input.startsWith("/demo/fixtures/")) {
        return new Response(
          fs.readFileSync(path.join(fixtures, input.replace("/demo/fixtures/", ""))),
        );
      }
      passedThrough.push(input);
      return new Response("external passthrough");
    },
  };
  vm.runInNewContext(fs.readFileSync(path.join(import.meta.dirname, "demo-mode.js"), "utf8"), {
    window,
    EventTarget,
    MessageEvent,
    Response,
    URL,
    Date,
    console,
    location: { origin },
    localStorage: { getItem: () => null },
    document: {
      addEventListener() {
        assert.fail("No injected demo chrome");
      },
    },
    setTimeout() {},
  });
  for (const input of [new URL("/api/state", origin), new Request(origin + "/api/state")]) {
    const response = await window.fetch(input);
    assert.equal(response.status, 200);
    assert(Array.isArray((await response.json()).agents));
  }
  const denied = await window.fetch(new Request(origin + "/api/config", { method: "POST" }));
  assert.equal(denied.status, 403);
  const external = new URL("https://external.example/api/state");
  assert.equal(await (await window.fetch(external)).text(), "external passthrough");
  assert.equal(passedThrough[0], external);
});
