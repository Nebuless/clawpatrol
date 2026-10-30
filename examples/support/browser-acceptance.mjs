// Run only against TestSupportProcessAcceptance's private synthetic fixture.
// No storage-state export, request logs, raw codes, tokens or credential screenshots.
import assert from "node:assert/strict";
import { readFile, writeFile, unlink } from "node:fs/promises";
import path from "node:path";
import { pathToFileURL } from "node:url";

try {
  const f = JSON.parse(await readFile(process.argv[2], "utf8"));
  const modulePath = process.env.PRX_PLAYWRIGHT_MODULE;
  assert.ok(modulePath, "PRX_PLAYWRIGHT_MODULE must name installed playwright index.mjs");
  const { chromium } = await import(pathToFileURL(modulePath));
  const providerPort = new URL(f.Provider).port;
  const browser = await chromium.launch({
    ...(process.env.PRX_CHROMIUM ? { executablePath: process.env.PRX_CHROMIUM } : {}),
    headless: true,
    args: [
      "--no-sandbox",
      "--no-proxy-server",
      "--ignore-certificate-errors",
      `--host-resolver-rules=MAP *.support.fixture.test 127.0.0.1:${providerPort}`,
    ],
  });
  const results = [];
  try {
    const technician = await browser.newContext({ ignoreHTTPSErrors: true });
    await technician.addInitScript(() => {
      window.fixtureStreams = [];
      const NativeEventSource = window.EventSource;
      window.EventSource = class extends NativeEventSource {
        constructor(...args) {
          super(...args);
          const stream = { source: this, deliveries: 0 };
          window.fixtureStreams.push(stream);
          for (const kind of ["message", "backlog"])
            this.addEventListener(kind, () => stream.deliveries++);
        }
      };
    });
    const customer = await browser.newContext({ ignoreHTTPSErrors: true });
    const provider = await technician.newPage();

    await provider.goto(f.Provider);
    await provider.getByRole("link", { name: "Sign in with technician MFA" }).click();
    await provider.getByRole("button", { name: "Complete synthetic MFA" }).click();
    await provider.getByRole("link", { name: "Continue to Provider", exact: true }).waitFor();
    assert.equal(new URL(provider.url()).pathname, "/__provider/oidc/callback");
    await provider.getByRole("link", { name: "Continue to Provider", exact: true }).click();
    await provider.getByRole("heading", { name: "PRX Provider", exact: true }).waitFor();
    // Management requires technician MFA, never customer phone approval.
    await provider.getByLabel("Organization ID", { exact: true }).fill("browser-managed");
    await provider
      .getByLabel("Organization name", { exact: true })
      .fill("Browser managed organization");
    await provider.getByRole("button", { name: "Create organization", exact: true }).click();
    await provider
      .getByRole("combobox", { name: "Organization", exact: true })
      .selectOption("browser-managed");
    await provider
      .getByRole("combobox", { name: "Organization technician", exact: true })
      .selectOption("other");
    const assigned = provider.waitForResponse(
      (r) => r.url().includes("/__provider/organization-assign") && r.request().method() === "POST",
    );
    await provider
      .getByRole("button", { name: "Assign organization technician", exact: true })
      .click();
    assert.equal((await assigned).status(), 200);
    await provider
      .getByLabel("Gateway installation ID", { exact: true })
      .fill("browser-managed-installation");
    await provider
      .getByLabel("Installation name", { exact: true })
      .fill("Browser managed installation");
    await provider.getByRole("button", { name: "Register installation", exact: true }).click();
    await provider
      .getByRole("combobox", { name: "Installation", exact: true })
      .selectOption("browser-managed-installation");
    const management = provider.getByRole("region", { name: "Installation management" });
    await management.getByText(/Connector: not_enrolled/).waitFor();
    await provider
      .getByRole("button", { name: "Toggle installation enabled", exact: true })
      .click();
    await management.getByText(/Connector: disabled/).waitFor();
    await provider
      .getByRole("button", { name: "Toggle installation enabled", exact: true })
      .click();
    await management.getByText(/Connector: not_enrolled/).waitFor();
    await provider
      .getByRole("button", { name: "Toggle organization enabled", exact: true })
      .click();
    await management.getByText(/Connector: disabled/).waitFor();
    await provider
      .getByRole("button", { name: "Toggle organization enabled", exact: true })
      .click();
    await management.getByText(/Connector: not_enrolled/).waitFor();
    await provider.route("**/__provider/installation?**", (route) => route.abort());
    await provider
      .getByRole("button", { name: "Refresh installation and cases", exact: true })
      .click();
    await management.getByText(/Connector: Unavailable/).waitFor();
    await provider.unroute("**/__provider/installation?**");
    await provider
      .getByRole("button", { name: "Refresh installation and cases", exact: true })
      .click();
    await management.getByText(/Connector: not_enrolled/).waitFor();
    for (const width of [1440, 768, 390]) {
      await provider.setViewportSize({ width, height: 900 });
      assert.equal(
        await provider.evaluate(() => document.documentElement.scrollWidth > innerWidth),
        false,
        "management overflow",
      );
      await provider.screenshot({ path: path.join(f.Dir, `managed-${width}.png`), fullPage: true });
    }
    await provider.setViewportSize({ width: 1440, height: 900 });
    async function checkAudit(page, code = "") {
      await page.getByRole("button", { name: "Load local audit", exact: true }).click();
      const auditList = page.getByRole("list", { name: "Local support audit" });
      await auditList.getByRole("listitem").nth(49).waitFor();
      await page.getByRole("button", { name: "Load more audit", exact: true }).click();
      await auditList.getByRole("listitem").nth(99).waitFor();
      const downloadPromise = page.waitForEvent("download");
      await page.getByRole("button", { name: "Export local audit JSON", exact: true }).click();
      const download = await downloadPromise;
      assert.equal(download.suggestedFilename(), "prx-support-audit.json");
      const audit = JSON.parse(await readFile(await download.path(), "utf8"));
      assert.ok(audit.events.length > 100);
      assert.equal(new Set(audit.events.map((e) => e.id)).size, audit.events.length);
      for (const event of audit.events) {
        assert.ok(event.actor_id && event.actor_name);
        assert.ok(event.effective_id && event.effective_name);
        for (const key of [
          "organization_id",
          "installation_id",
          "case_id",
          "session_id",
          "at",
          "operation",
          "method",
          "decision",
          "outcome",
        ])
          assert.ok(key in event);
        assert.ok(!code || !JSON.stringify(event).includes(code), "code leaked into export");
      }
    }
    if (process.env.PRX_SUPPORT_MANAGEMENT_ONLY) {
      for (const gateway of f.Gateways) {
        const page = await customer.newPage();
        await page.goto(`${gateway}/__login`);
        await page.locator('input[name="password"]').fill("synthetic-password");
        await page.locator('button[type="submit"]').click();
        await page.goto(`${gateway}/#/account`);
        await page
          .getByRole("heading", { name: "Customer-approved support", exact: true })
          .waitFor();
        await checkAudit(page);
        await page.close();
      }
    }
    const previousCases = [];
    const runtimeErrors = [];
    provider.on("pageerror", () => runtimeErrors.push("pageerror"));
    for (const [round, role] of (process.env.PRX_SUPPORT_MANAGEMENT_ONLY
      ? []
      : ["gateway-a", "gateway-b", "gateway-b"]
    ).entries()) {
      const index = round === 0 ? 0 : 1;
      const page = await customer.newPage();
      await page.goto(`${f.Gateways[index]}/__login`);
      await page.locator('input[name="password"]').fill("synthetic-password");
      await page.locator('button[type="submit"]').click();
      await page.goto(`${f.Gateways[index]}/#/request/fixture-support-action`);
      const fixtureDownload = page.waitForResponse((r) => r.url().includes("fmt=fixture"));
      await page.getByRole("button", { name: "Download action", exact: true }).click();
      assert.equal((await fixtureDownload).status(), 200, "customer fixture download unavailable");
      await page.goto(`${f.Gateways[index]}/#/account`);
      await page.getByRole("heading", { name: "Customer-approved support" }).waitFor();
      for (const [label, key, value] of [
        ["Dashboard profile", "profile", "default"],
        ["Dashboard theme", "theme", "light"],
        ["Dashboard profile", "profile", "limited"],
        ["Dashboard theme", "theme", "dark"],
      ]) {
        const saved = page.waitForResponse(
          (r) => r.url().endsWith("/api/account/context") &&
            r.request().method() === "PATCH" && r.request().postDataJSON().context[key] === value,
        );
        await page.getByRole("combobox", { name: label, exact: true }).selectOption(value);
        assert.equal((await saved).status(), 200, "customer preferences did not save");
        await page.getByText("Preferences saved", { exact: true }).waitFor();
      }
      const ownership = await (await page.request.get(`${f.Gateways[index]}/api/support`)).json();
      assert.equal(
        ownership.accounts.length,
        1,
        "fixture must preserve real single-principal ownership",
      );
      assert.equal(ownership.accounts[0].name, "root");
      const foreign = await page.request.post(`${f.Gateways[index]}/api/support/grants`, {
        headers: { Origin: f.Gateways[index] },
        data: { case_id: "foreign", target_account_id: "foreign-account", assignment_revision: 1 },
      });
      assert.equal(foreign.status(), 400, "foreign account accepted");
      await page.getByLabel("Case summary", { exact: true }).fill("Synthetic browser case");
      const submitted = page.waitForResponse(
        (r) => r.url().endsWith("/api/support/cases") && r.request().method() === "POST",
      );
      await page.getByRole("button", { name: "Submit support case", exact: true }).click();
      const response = await submitted;
      assert.equal(response.status(), 200, "case submission failed");
      const caseID = (await response.json()).id;
      assert.equal(
        (await provider.request.get(`${f.Provider}/__provider/session`)).status(),
        200,
        "Provider browser session missing",
      );
      await provider.getByRole("combobox", { name: "Organization", exact: true }).waitFor();
      await provider
        .getByRole("combobox", { name: "Organization", exact: true })
        .selectOption(role);
      await provider
        .getByRole("combobox", { name: "Installation", exact: true })
        .selectOption(role);
      await provider
        .getByRole("region", { name: "Installation management" })
        .getByText(/Connector: connected · Client registry: local_only/)
        .waitFor();
      if (round === 0) {
        // Failed health refresh must not keep last-good connected status.
        await provider.route("**/__provider/installation?**", (route) => route.abort());
        await provider
          .getByRole("button", { name: "Refresh installation and cases", exact: true })
          .click();
        await provider
          .getByRole("region", { name: "Installation management" })
          .getByText(/Connector: Unavailable/)
          .waitFor();
        await provider.unroute("**/__provider/installation?**");
        await provider
          .getByRole("button", { name: "Refresh installation and cases", exact: true })
          .click();
        await provider
          .getByRole("region", { name: "Installation management" })
          .getByText(/Connector: connected/)
          .waitFor();
      }
      await provider.getByRole("combobox", { name: /^Assigned technician/ }).selectOption("tech");
      const card = provider
        .locator("div")
        .filter({ has: provider.getByRole("heading", { name: `Case ${caseID}`, exact: true }) })
        .last();
      await card.getByRole("button", { name: "Assign technician", exact: true }).click();
      await page
        .getByRole("button", { name: "Issue one-time SUPPORT code for root", exact: true })
        .last()
        .waitFor();
      const customerCard = page
        .locator("div")
        .filter({ has: page.getByRole("heading", { name: `Case ${caseID}`, exact: true }) })
        .last();
      await customerCard
        .getByRole("button", { name: "Issue one-time SUPPORT code for root", exact: true })
        .click();
      const raw = await page.getByLabel("One-time SUPPORT code", { exact: true }).inputValue();
      assert.match(raw, /^SUPPORT-[0-9A-Z]{4}-[0-9A-Z]{4}-[0-9A-Z]{4}$/);
      await provider.getByLabel("Customer phone SUPPORT code", { exact: true }).fill(raw);
      await page.getByRole("button", { name: "Hide code", exact: true }).click();
      const savedView = "#/analytics?range=6h&color=host&scale=linear&host=public.example";
      const savedNavigation = page.waitForResponse(
        (r) => r.url().endsWith("/api/account/context") && r.request().method() === "PATCH" &&
          r.request().postDataJSON().context.route?.page === "analytics",
      );
      await page.evaluate((hash) => { location.hash = hash; }, savedView);
      assert.equal((await savedNavigation).status(), 200);
      await page.goto(f.Gateways[index]);
      await page.waitForURL((url) => url.hash === savedView);
      await page.getByRole("combobox", { name: "Dashboard profile", exact: true }).waitFor();
      assert.equal(await page.getByLabel("Dashboard profile", { exact: true }).inputValue(), "limited");
      assert.equal(await page.getByLabel("Dashboard theme", { exact: true }).inputValue(), "dark");
      assert.equal(await page.evaluate(() => document.documentElement.dataset.theme), "dark");
      const redemption = provider.waitForResponse(
        (r) => r.url().includes("/__provider/view-as") && r.request().method() === "POST",
      );
      await card.getByRole("button", { name: "View customer dashboard", exact: true }).click();
      const redeemed = await redemption;
      assert.equal(redeemed.status(), 200, "browser grant redemption failed");
      const activation = provider.waitForResponse(
        (r) => r.url().endsWith("/__support/activate") && r.request().method() === "POST",
      );
      await provider.getByRole("button", { name: "Open customer dashboard", exact: true }).click();
      const activationResponse = await activation;
      assert.equal(activationResponse.status(), 200, "activation failed");
      await provider
        .getByRole("link", { name: "Continue to customer dashboard", exact: true })
        .waitFor();
      await provider
        .getByRole("link", { name: "Continue to customer dashboard", exact: true })
        .click();
      await provider.getByRole("complementary", { name: "Support session", exact: true }).waitFor();
      assert.equal(new URL(provider.url()).hash, savedView, "support did not restore customer route");
      assert.equal(await provider.evaluate(() => document.documentElement.dataset.theme), "dark");
      assert.equal(await provider.getByLabel("Dashboard profile", { exact: true }).count(), 0);
      const origin = new URL(provider.url()).origin;
      assert.ok(origin.endsWith(".support.fixture.test"), "support origin missing isolation");
      await provider.reload();
      await provider.getByRole("complementary", { name: "Support session", exact: true }).waitFor();
      const state = await provider.evaluate(async () =>
        (await fetch("/api/state", { cache: "no-store" })).json(),
      );
      assert.equal(state.support.actor_id, "tech");
      assert.equal(state.support.effective_name, "root");
      assert.equal(state.support.installation_id, role);
      assert.equal(state.support.case_id, caseID);
      for (const previous of previousCases)
        assert.ok(
          !JSON.stringify(state).includes(previous),
          "previous customer cached in new support state",
        );
      previousCases.push(caseID);
      assert.equal(state.account_context.profile, "limited");
      assert.equal(state.account_context.theme, "dark");
      assert.equal(state.account_context.route.page, "analytics");
      assert.equal(state.account_context.filters.range, "6h");
      assert.equal(state.account_context.filters.scale, "linear");
      assert.equal(state.account_context.filters.host, "public.example");
      assert.equal(state.customer_dashboard_config_writes, true);
      assert.equal(state.dashboard_config_writes, false);
      assert.equal(
        await provider.evaluate(async () => {
          const r = await fetch("/api/state", { headers: { "If-None-Match": "*" } });
          return r.status;
        }),
        200,
      );
      assert.equal(
        await provider.evaluate(async () =>
          (await fetch("/api/state")).headers.get("cache-control"),
        ),
        "no-store",
      );
      const cookies = await technician.cookies(origin);
      assert.ok(
        cookies.some(
          (c) =>
            c.name === "__Host-prx_support" && !c.domain.startsWith(".") && c.secure && c.httpOnly,
        ),
      );
      assert.ok(
        !cookies.some((c) => c.name === "__Host-cp_session" || c.name === "__Host-prx_provider"),
      );
      await provider.goto(`${origin}/#/request/fixture-support-action`);
      await provider.getByRole("button", { name: "Block requests like this", exact: true }).waitFor();
      assert.equal(await provider.getByRole("button", { name: "Download action", exact: true }).count(), 0, "support exposes forbidden fixture download");
      await provider.getByRole("complementary", { name: "Support session", exact: true }).waitFor();
      await provider.getByRole("heading", { name: "Support session ended", exact: true }).waitFor({ state: "hidden" });
      const denied = await provider.evaluate(async () => {
        const out = [];
        for (const target of ["/api/config", "/api/future-unclassified", "/api/support/grants", "/api/actions/fixture-support-action?fmt=fixture"]) {
          out.push((await fetch(target, { cache: "no-store" })).status);
        }
        return out;
      });
      assert.deepEqual(denied, [403, 403, 403, 403]);
      for (const ip of ["10.55.0.2", "2001:db8::2"]) {
        await provider.goto(`${origin}/#/device/${encodeURIComponent(ip)}`);
        await provider.getByRole("complementary", { name: "Support session", exact: true }).waitFor();
        await provider.getByRole("heading", { name: "Support session ended", exact: true }).waitFor({ state: "hidden" });
        assert.ok(!(await provider.locator("body").textContent()).includes("no agent with ip"));
      }
      await provider.goto(`${origin}/#/settings`);
      await provider.getByRole("heading", { name: "Support settings view", exact: true }).waitFor();
      for (const width of [1440, 768, 390]) {
        await provider.setViewportSize({ width, height: 900 });
        assert.equal(
          await provider.evaluate(() => document.documentElement.scrollWidth > innerWidth),
          false,
          "horizontal overflow",
        );
        const endButton = await provider.getByRole("button", { name: "End support session", exact: true }).boundingBox();
        assert.ok(endButton && endButton.height >= 44, "support termination touch target too small");
        if (width === 390) {
          const banner = await provider.getByRole("complementary", { name: "Support session", exact: true }).boundingBox();
          assert.ok(banner && banner.height < 330, "mobile support banner squeezes text beside button");
        }
        await provider.screenshot({
          path: path.join(f.Dir, `${role}-support-${width}.png`),
          fullPage: true,
        });
      }
      await page.goto(`${f.Gateways[index]}/#/account`);
      await page.getByRole("heading", { name: "Customer-approved support", exact: true }).waitFor();
      await checkAudit(page, raw);
      await provider.goto(`${origin}/#/`);
      await provider.getByRole("complementary", { name: "Support session", exact: true }).waitFor();
      await provider.waitForFunction(() =>
        window.fixtureStreams.some((s) => s.source.readyState === 1 && s.deliveries > 0),
      );
      if (round === 0) {
        await page.reload();
        await page.getByRole("button", { name: "Revoke access", exact: true }).last().click();
        await provider
          .getByRole("heading", { name: "Support session ended", exact: true })
          .waitFor({ timeout: 15000 });
      } else if (round === 2) {
        await writeFile(path.join(f.Dir, "idp-inactive"), "disabled", { mode: 0o600 });
        await provider
          .getByRole("heading", { name: "Support session ended", exact: true })
          .waitFor({ timeout: 15000 });
        await unlink(path.join(f.Dir, "idp-inactive"));
      } else {
        await provider.getByRole("button", { name: "End support session", exact: true }).focus();
        await provider.keyboard.press("Enter");
        await provider
          .getByRole("heading", { name: "Support session ended", exact: true })
          .waitFor();
      }
      assert.equal(await provider.locator('[aria-label="Support session"]').count(), 0);
      assert.ok(
        !(await provider.locator("body").textContent()).includes(caseID),
        "terminal view retained case data",
      );
      assert.equal(
        await provider.evaluate(() =>
          window.fixtureStreams.every((s) => s.source.readyState === 2),
        ),
        true,
        "EventSource remained open after terminal state",
      );
      const delivered = await provider.evaluate(() =>
        window.fixtureStreams.reduce((n, s) => n + s.deliveries, 0),
      );
      await provider.waitForTimeout(1200);
      assert.equal(
        await provider.evaluate(() => window.fixtureStreams.reduce((n, s) => n + s.deliveries, 0)),
        delivered,
        "EventSource delivered after terminal state",
      );
      const endedResponse = await provider.goto(origin);
      assert.equal(endedResponse.status(), 401);
      results.push({
        installation: role,
        hosting: index ? "nebuless" : "customer",
        status: "passed",
        viewports: [1440, 768, 390],
        termination: ["customer", "technician", "idp-disable"][round],
      });
      await page.close();
      if (round !== 2) {
        await provider.goto(f.Provider);
        await provider.getByRole("heading", { name: "PRX Provider", exact: true }).waitFor();
      }
    }
    assert.deepEqual(runtimeErrors, [], "dashboard runtime errors");
    await writeFile(
      path.join(f.Dir, "browser-result.json"),
      JSON.stringify(
        {
          status: "passed",
          management: "passed",
          support: process.env.PRX_SUPPORT_MANAGEMENT_ONLY ? "not_run" : "passed",
          results,
        },
        null,
        2,
      ),
      { mode: 0o600 },
    );
    process.stdout.write(
      process.env.PRX_SUPPORT_MANAGEMENT_ONLY
        ? "Browser management and customer audit acceptance passed. Support session browser checks not run."
        : "Browser acceptance passed: management, two hosting modes, isolated contexts, dual identity, denied routes, desktop/mobile, local revoke, keyboard termination, technician disable and EventSource cleanup.",
    );
  } finally {
    await browser.close();
  }
} catch (error) {
  // Never print assertion values, page content or token-bearing URLs.
  console.error(
    "Browser acceptance failed:",
    error.name,
    "at",
    error.stack
      ?.split("\n")
      .find((line) => line.includes("browser-acceptance.mjs:"))
      ?.replace(/.*browser-acceptance.mjs:/, "line "),
  );
  process.exitCode = 1;
}
