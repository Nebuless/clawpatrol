// After make dashboard: pnpm exec vpx scripts/check-device-credentials.mjs
// Uses only a local static bundle and synthetic API responses.
import assert from "node:assert/strict";
import { once } from "node:events";
import { readFile } from "node:fs/promises";
import http from "node:http";
import path from "node:path";
import { chromium } from "playwright";

const dist = path.resolve(import.meta.dirname, "../dist");
const types = {
  ".html": "text/html",
  ".js": "text/javascript",
  ".css": "text/css",
  ".svg": "image/svg+xml",
  ".woff2": "font/woff2",
};
const server = http.createServer(async (req, res) => {
  try {
    const pathname = new URL(req.url, "http://localhost").pathname;
    const file = path.resolve(dist, `.${pathname === "/" ? "/index.html" : pathname}`);
    assert(file.startsWith(dist + path.sep));
    const body = await readFile(file);
    res.writeHead(200, { "Content-Type": types[path.extname(file)] || "application/octet-stream" });
    res.end(body);
  } catch {
    res.writeHead(404).end();
  }
});
server.listen(0, "127.0.0.1");
await once(server, "listening");
const origin = `http://127.0.0.1:${server.address().port}`;
const credential = (id) => ({
  id,
  name: id,
  type: "test_credential",
  has_oauth: false,
  connected: true,
});
const agent = (ip, profile) => ({
  ip,
  profile,
  hostname: `test-${profile || "legacy"}`,
  user: "test",
  os: "linux",
  first_at: "2026-10-30T00:00:00Z",
  last_at: "2026-10-30T00:00:00Z",
  reqs: 0,
  bytes_in: 0,
  bytes_out: 0,
  last_host: "",
  activity: [],
  sessions: [],
});
const agents = [agent("10.55.0.2", "writer"), agent("10.55.0.3", "reader"), agent("10.55.0.4", "")];
const writerGate = Promise.withResolvers();
const readerGate = Promise.withResolvers();
let writerMode = "pending";
let holdReader = true;
let browser;
try {
  browser = await chromium.launch({
    ...(process.env.PRX_CHROMIUM ? { executablePath: process.env.PRX_CHROMIUM } : {}),
    headless: true,
    args: ["--no-sandbox"],
  });
  const page = await browser.newPage();
  page.setDefaultTimeout(7000);
  const errors = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.route("**/api/**", async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === "/api/state") {
      return route.fulfill({
        json: {
          agents,
          integrations: [credential("global-only-credential")],
          whoami: { user: "test", device: "", host: "", auth_method: "password" },
        },
      });
    }
    if (url.pathname === "/api/profiles") return route.fulfill({ json: ["writer", "reader"] });
    if (url.pathname === "/api/status") {
      if (url.searchParams.get("profile") === "writer") {
        if (writerMode === "pending") await writerGate.promise;
        return writerMode === "ready"
          ? route.fulfill({ json: [credential("writer-only-credential")] })
          : route.fulfill({ status: 503, body: "Synthetic status failure" });
      }
      if (holdReader) {
        await readerGate.promise;
      }
      return route.fulfill({ json: holdReader ? [credential("reader-only-credential")] : [] });
    }
    if (url.pathname === "/api/events")
      return route.fulfill({ contentType: "text/event-stream", body: ": connected\n\n" });
    return route.fulfill({ json: [] });
  });
  await page.goto(`${origin}/#/device/10.55.0.2`);
  await page.getByText("test-writer", { exact: true }).first().waitFor();
  const initiallyLoading = await page
    .getByText("Loading credentials…", { exact: true })
    .isVisible();
  assert.equal(await page.getByText("global-only-credential", { exact: true }).count(), 0);
  writerMode = "error";
  writerGate.resolve();
  await page
    .getByRole("alert")
    .getByText("Credentials could not be loaded for this profile.", { exact: true })
    .waitFor();
  assert.equal(initiallyLoading, true);
  assert.equal(await page.getByText("global-only-credential", { exact: true }).count(), 0);
  writerMode = "ready";
  await page.getByRole("button", { name: "Retry credentials", exact: true }).click();
  await page.getByText("writer-only-credential", { exact: true }).waitFor();
  assert.equal(await page.getByRole("alert").count(), 0);

  const readerRequested = page.waitForRequest(
    (r) => new URL(r.url()).searchParams.get("profile") === "reader",
  );
  await page.goto(`${origin}/#/device/10.55.0.3`);
  await readerRequested;
  await page.getByText("Loading credentials…", { exact: true }).waitFor();
  assert.equal(await page.getByText("writer-only-credential", { exact: true }).count(), 0);
  await page.goto(`${origin}/#/device/10.55.0.2`);
  await page.getByText("writer-only-credential", { exact: true }).waitFor();
  const lateReader = page.waitForResponse(
    (r) => new URL(r.url()).searchParams.get("profile") === "reader",
  );
  readerGate.resolve();
  await lateReader;
  await page.evaluate(
    () => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))),
  );
  assert.equal(await page.getByText("reader-only-credential", { exact: true }).count(), 0);

  await page.goto(`${origin}/#/device/10.55.0.4`);
  await page.getByText("global-only-credential", { exact: true }).waitFor();
  holdReader = false;
  await page.goto(`${origin}/#/device/10.55.0.3`);
  await page.getByText("No credentials declared for this profile.", { exact: true }).waitFor();
  assert.deepEqual(errors, []);
  console.log(
    JSON.stringify({
      loading: "passed",
      failureRetry: "passed",
      staleProfile: "passed",
      legacyAndEmpty: "passed",
      pageErrors: errors,
    }),
  );
} finally {
  writerGate.resolve();
  readerGate.resolve();
  if (browser) await browser.close();
  server.closeAllConnections();
  await new Promise((resolve) => server.close(resolve));
}
