// Run after make site on the build host: pnpm exec vpx scripts/check-pages.mjs
import assert from "node:assert/strict";
import { spawn, execFileSync } from "node:child_process";
import { readdirSync, readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { createRequire } from "node:module";

const require = createRequire(new URL("../../dashboard/package.json", import.meta.url));
const { chromium } = require("playwright");

const origin = "http://127.0.0.1:8473";
const paths = ["/", "/assessment/", "/how-it-works/", "/security/", "/deployment/", "/pilot/"];
const output = process.env.PRX_PAGES_OUTPUT || "/tmp/prx-pages";
mkdirSync(output, { recursive: true });
const server = spawn(
  "python3",
  ["-m", "http.server", "8473", "--bind", "127.0.0.1", "--directory", "dist"],
  { stdio: "ignore" },
);
const chrome = spawn(
  process.env.PRX_CHROMIUM || chromium.executablePath(),
  [
    "--headless",
    "--no-sandbox",
    "--disable-dev-shm-usage",
    "--hide-scrollbars",
    "--remote-debugging-port=9473",
    "--user-data-dir=/tmp/prx-chrome-check",
    "about:blank",
  ],
  { stdio: "ignore" },
);
const delay = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
async function ready(url) {
  for (let i = 0; i < 100; i++) {
    try {
      const r = await fetch(url);
      if (r.ok) return r;
    } catch {}
    await delay(100);
  }
  throw new Error(`Server did not start: ${url}`);
}
let socket;
try {
  await ready(origin);
  const targets = await (await ready("http://127.0.0.1:9473/json")).json();
  socket = new WebSocket(targets.find((t) => t.type === "page").webSocketDebuggerUrl);
  await new Promise((resolve, reject) => {
    socket.onopen = resolve;
    socket.onerror = reject;
  });
  let sequence = 0;
  const pending = new Map();
  const errors = [];
  socket.onmessage = ({ data }) => {
    const m = JSON.parse(data);
    if (m.id) {
      const p = pending.get(m.id);
      pending.delete(m.id);
      if (m.error) p.reject(m.error);
      else p.resolve(m.result);
    } else if (m.method === "Runtime.exceptionThrown") errors.push(m.params);
  };
  function cdp(method, params = {}) {
    return new Promise((resolve, reject) => {
      const id = ++sequence;
      pending.set(id, { resolve, reject });
      socket.send(JSON.stringify({ id, method, params }));
    });
  }
  async function evaluate(expression) {
    const r = await cdp("Runtime.evaluate", {
      expression,
      awaitPromise: true,
      returnByValue: true,
    });
    assert(!r.exceptionDetails, JSON.stringify(r.exceptionDetails));
    return r.result.value;
  }
  await cdp("Page.enable");
  await cdp("Runtime.enable");
  const report = [];
  for (const path of paths) {
    const html = readFileSync(join("dist", path, "index.html"), "utf8");
    assert.equal((html.match(/<h1[ >]/g) ?? []).length, 1);
    assert(html.includes(`rel="canonical" href="https://prx.nebuless.cloud${path}"`));
    assert(html.includes(`property="og:url" content="https://prx.nebuless.cloud${path}"`));
    assert(html.includes('name="description"'));
    for (const width of [1440, 390]) {
      await cdp("Emulation.setDeviceMetricsOverride", {
        width,
        height: 1000,
        deviceScaleFactor: 1,
        mobile: false,
      });
      await cdp("Page.navigate", { url: origin + path });
      await delay(800);
      await evaluate("document.fonts.ready");
      assert.equal(
        await evaluate("document.documentElement.scrollWidth <= innerWidth"),
        true,
        `${path} overflow at ${width}: ${JSON.stringify(await evaluate('[...document.querySelectorAll("body *")].filter(e => e.getBoundingClientRect().right > innerWidth + 1).map(e => ({tag:e.tagName,cls:e.className,right:e.getBoundingClientRect().right})).slice(0,15)'))}`,
      );
      assert.equal(await evaluate(`document.fonts.check('600 24px "Hanken Grotesk"')`), true);
      const metrics = await cdp("Page.getLayoutMetrics");
      const name = path === "/" ? "home" : path.split("/")[1];
      const clip = {
        x: 0,
        y: 0,
        width,
        height: width === 1440 ? Math.ceil(metrics.cssContentSize.height) : 1000,
        scale: 1,
      };
      const shot = await cdp("Page.captureScreenshot", {
        format: "png",
        captureBeyondViewport: true,
        clip,
      });
      writeFileSync(join(output, `${name}-${width}.png`), Buffer.from(shot.data, "base64"));
      if (width === 390) {
        assert.equal(
          await evaluate('getComputedStyle(document.querySelector(".site-nav-mobile")).display'),
          "block",
        );
        await evaluate('document.querySelector(".site-nav-mobile").open = true');
        assert.equal(
          await evaluate(
            'document.querySelector(".site-nav-disclosure").getBoundingClientRect().right <= innerWidth',
          ),
          true,
        );
        const menu = await cdp("Page.captureScreenshot", { format: "png" });
        writeFileSync(join(output, `${name}-menu-390.png`), Buffer.from(menu.data, "base64"));
      }
      report.push({ path, width, height: metrics.cssContentSize.height });
    }
  }
  assert.equal(errors.length, 0, JSON.stringify(errors));
  const internal = new Set();
  const legacy = [];
  const markers = [];
  function scan(dir) {
    for (const entry of readdirSync(dir, { withFileTypes: true })) {
      const file = join(dir, entry.name);
      if (entry.isDirectory()) scan(file);
      else if (file.endsWith(".html")) {
        const html = readFileSync(file, "utf8");
        for (const m of html.matchAll(/href="([^"#]+)(?:#[^"]*)?"/g)) {
          if (m[1].startsWith("/") && !m[1].startsWith("//"))
            internal.add(m[1].replaceAll("&amp;", "&"));
        }
        for (const m of html.matchAll(/.{0,100}clawpatrol\.dev.{0,100}/g))
          legacy.push({ file, match: m[0] });
        // Report words in visible text, not script names or route identifiers.
        const text = html.replace(/<script[\s\S]*?<\/script>/g, "").replace(/<[^>]*>/g, " ");
        for (const m of text.matchAll(
          /.{0,70}\b(?:sample|placeholder|example|coming soon|draft|illustrative|demo)\b.{0,70}/gi,
        ))
          markers.push({ file, match: m[0] });
      }
    }
  }
  scan("dist");
  const links = [];
  for (const href of [...internal].sort((a, b) => a.localeCompare(b))) {
    const status = execFileSync(
      "curl",
      ["-s", "-L", "-o", "/dev/null", "-w", "%{http_code}", origin + href],
      { encoding: "utf8" },
    );
    links.push({ href, status });
  }
  const failures = links.filter((l) => l.status !== "200");
  writeFileSync(
    join(output, "verification.json"),
    JSON.stringify({ report, links, legacy, markers, errors }, null, 2),
  );
  console.log(
    JSON.stringify({
      screenshots: report.length,
      internalLinks: links.length,
      failures,
      legacyOccurrences: legacy.length,
      markerOccurrences: markers.length,
    }),
  );
  assert.equal(failures.length, 0, JSON.stringify(failures));
} finally {
  socket?.close();
  chrome.kill();
  server.kill();
}
