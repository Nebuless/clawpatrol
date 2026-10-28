# Synthetic support fixture

These examples contain no production credentials. JSON files are configuration templates, not a working issuer or trust policy. Use [Provider runbook](../../doc/provider.md) and [gateway support runbook](../../doc/support.md). Synthetic IdP in `internal/supporttest` is test-only, with exact callback allowlist, generated RSA signing key and fixed synthetic identities.

## Repeat process and browser acceptance

Run heavy commands on zo-computer only. Existing cached dependencies and browser must be present. No installs are implied by these commands.

```sh
/home/egsox/.local/bin/zo-run bash -lc 'cd dashboard && deno task build'
/home/egsox/.local/bin/zo-run bash -lc '
  PRX_SUPPORT_FIXTURE_DIR=/tmp/prx-support-browser-$(date +%s) \
  PRX_SUPPORT_BROWSER_SCRIPT=$PWD/examples/support/browser-acceptance.mjs \
  PRX_PLAYWRIGHT_MODULE=/root/.npm/_npx/e41f203b7505f1fb/node_modules/playwright/index.mjs \
  go test ./cmd/nebprx -run ^TestSupportProcessAcceptance$ -count=1 -timeout=4m
'
```

If cached Playwright path changes, set `PRX_PLAYWRIGHT_MODULE` to installed `playwright/index.mjs`. `PRX_CHROMIUM` defaults to `/root/.local/bin/chromium`. Browser script takes fixture JSON path as its sole argument. It suppresses request bodies and storage-state exports. Chromium's no-sandbox/certificate-ignore flags are confined to isolated synthetic fixture, never deployment settings.

Fixture launches seven child processes: TLS IdP, production Provider command, two production gateway commands (each with client registry), two HTTPS gateway proxies. Random loopback ports and directories are written into private `fixture.json`. Gateways represent customer-hosted and Nebuless-hosted metadata in distinct organizations. Generated TLS CA/key, connector/registry keys, passwords and tokens stay in private fixture directory. `result.json` records Go-client acceptance. `browser-result.json` records browser assertions. Screenshots show management and support settings at 1440px/768px/390px, never one-time codes. `PRX_SUPPORT_MANAGEMENT_ONLY=1` runs management and ordinary customer audit pagination/export browser checks without support-session browser flows and records `support: "not_run"` in browser results. This focused mode does not establish full browser acceptance.

Go test automatically kills child processes during cleanup. Without retained-directory variable, temporary data is deleted. With it, target directory must not already exist and remains private after process exit. Treat retained database/keys/logs as sensitive even though synthetic. Do not commit it or attach whole directory to a review.

For coordinator inspection, set `PRX_SUPPORT_FIXTURE_HOLD=1` and a bounded test timeout. Fixture logs readiness after Go-client flow (and browser hook if configured), then waits until interrupted/timed out. Use addresses in private JSON, not guessed ports. Browsers need loopback mapping for `*.support.fixture.test` to Provider's allocated port, configured by supplied script. Held timeout is intentional, not a passing test result. Do not leave it unattended.

## Focused checks

```sh
/home/egsox/.local/bin/zo-run bash -lc '
  go test ./internal/provider ./internal/supportwire ./internal/supporttest -count=1 &&
  go test ./cmd/nebprx -run "TestSupport|TestDashboardAccount|TestDashboardNewMigration" -count=1 -timeout=4m
'
/home/egsox/.local/bin/zo-run bash -lc '
  go test -race ./internal/provider ./internal/supportwire ./internal/supporttest -count=1 &&
  go test -race ./cmd/nebprx -run "TestSupport|TestDashboardAccount|TestDashboardNewMigration" -count=1 -timeout=5m
'
```

Browser assertions cover no-code organization create/assignment/installation registration, disabled health and failed-refresh recovery, two customer contexts, Provider MFA, account-owned case/assignment, code redemption/POST activation, actor/effective identity, host-only cookies, denied config/future routes, support settings layouts, audit pagination/JSON export, local customer revoke, keyboard technician end, technician disable, reload/cache isolation and EventSource closure. Assertions are not evidence until the full run passes. Current fixture deliberately verifies only the authenticated root account and rejects foreign targets. Multi-account ownership has no server-side relation and must not be simulated. This is implementation evidence, not an independent security review or exhaustive AE matrix. See [evidence and gaps](../../doc/support-implementation.md). Live IdP assurance, public DNS/TLS ingress, hosted OS isolation, backup restore and key rotation remain untested deployment prerequisites.
