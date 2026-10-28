# Hosting and support implementation evidence

Current coordinator status is in [support-release-verification.md](support-release-verification.md).
The handoff and continuation notes below record earlier snapshots and their
limits; they do not supersede that release record.

Updated 2026-10-28. Implementation handoff, not completion of accepted Definition of Done. Canonical checkout `/home/egsox/repo/clawpatrol`, branch `rebrand/prx-glass`. Implementation-only worker. No staging, commits, pushes, releases, deployment, independent review or additional agents. Pre-existing marketing/skills/registry WIP preserved, including registry integration in main.go. Plan and review bodies unchanged.

## Current continuation status (2026-10-28)

Supersedes historical verification and coverage below. Coordinator owns review fixes F1–F6 and reserved source paths. Current concurrent changes are not validated by earlier passing snapshots.

- Management health and customer audit pagination/export are implemented. Focused browser management checks pass organization creation, visible-roster assignment, installation registration, organization/installation disable and recovery, failed-refresh recovery and 1440/768/390px no-overflow. Failed refresh clears stale connected status.
- Audit API uses account-scoped audit-row UUID cursors. A 205-event test verifies metadata, unique IDs, limits and foreign-cursor denial. UI loads 50-row pages and exports all available metadata through 100-row pages. Earlier full browser run verified pagination/export. New management-only audit hook awaits execution after F6 fixture adaptation.
- Account API exposes exactly authenticated password root or verified Tailnet principal. No relation defines ownership of multiple accounts. Selector handles returned lists and disables approval for empty lists. Multiple-account/empty-list surface evidence remains unestablished. Do not fabricate ownership.
- Latest full browser rerun fails waiting for support banner after reload. Initial home render mounts HITL while support identity is unset. Denied polling can terminate support. Coordinator owns lifecycle correction. New EventSource-close, post-terminal-delivery and cross-case cache assertions await passing full run.
- Latest process attempt fails before browser at support_integration_test.go:531. Its map[string]string decoder rejects structured account_context fields added by F6. Adapt fixture to typed contract when coordinator releases context changes. This is verification blocker, not evidence preferences vanished.

Scoped evidence, all heavy checks through zo-run:

| Evidence | Result |
| --- | --- |
| /tmp/prx-support-management-verified.log | PASS dashboard build, provider/supportwire/registry tests, pagination and process acceptance with management-only browser hook. |
| /tmp/prx-support-management-race.log | PASS provider/supportwire/registry race and support/account/migration CLI race, including process and focused management browser. Earlier snapshot before F6 contract change. |
| /tmp/prx-support-management-format.log | PASS scoped formatting, dashboard lint and tsc/Vite build. Existing chunk warning, 558.72 kB. |
| /tmp/prx-support-management-green.log | FAIL full browser after management success: support reload banner timeout. |
| /tmp/prx-support-management-audit.log | FAIL before browser: context decoder mismatch. New focused customer audit hook not yet executed. |
| Active LSP, eight owned/affected paths | No errors/blockers. Seven browser hints require retaining parentheses around awaited expressions. Correctness reminders, not defects. No suppressions. |

Final 17-file LSP probe has no compiler errors/blockers. Remaining auxiliary findings: browser awaited-member parentheses correctness hints, existing LiveRequests inline style, Vite-style extensionless imports and known-value/unknown-shape hints in api.ts, bounded fixed rate-limit/admission SQL loops in support_store.go:228/245/261, and existing bounded per-case refresh in provider/http.go:92. These are not new compiler defects; no suppressions or unrelated refactors applied.

Repository-wide dashboard format check fails SupportBanner.tsx. No broad formatting applied during concurrent shared UI work. Owned Go formatting and git diff --check pass.

Dependency worker applied Go 1.26.10, x/net 0.62, compress 1.18.7 and required transitive modules. Coordinator reports whole-program build/vet/govulncheck pass with zero reachable/imported vulnerabilities after Landlock rename. Required-module OpenPGP no-fix advisory remains. Tidy diff remains coordinator investigation. Dependency document records earlier snapshot, needs owner reconciliation. Historical vulnerability counts below are not current scan evidence. No blanket security-clean claim.

## Changed paths owned by this feature

- `internal/provider/`: separate SQLite schema/store, management, OIDC, connector broker, HTTP, SSE, assets, maintenance and tests.
- `internal/supportwire/`: signed connection-bound protocol, bounded WebSocket link and real-transport tests.
- `internal/supporttest/idp.go`: synthetic TLS issuer with PKCE, signed tokens, introspection faults and exact callback allowlist. Test-only.
- `cmd/nebprx/dashboard_accounts{,_test}.go`, `dashboard_users.go`, `db.go`, migrations `0025_dashboard_accounts.sql` through `0028_support_tunnel_revocations.sql`.
- `cmd/nebprx/support_surface_test.go`: route/method no-effect matrix, real expiry/revoke, audit faults, UUID pagination ownership and HTTP-method metadata.
- Bounded management health: `internal/supportwire/health.go`, `internal/provider/health{,_test}.go`, `internal/provider/organization.go`, `internal/registry/support_health{,_test}.go`.
- `cmd/nebprx/provider.go`, `support.go`, `support_connector.go`, `support_store{,_test}.go`, `support_customer_audit.go`, `support_customer_revoke.go`, `support_tailnet.go`, `support_mutation.go`, `support_acceptance_test.go`, `support_http_test.go`, `support_integration_test.go`, `support_maintenance_test.go`, `web_support.go`, `web_support_policy.go`, `web_support_projection.go`.
- Existing gateway integration: `cmd/nebprx/main.go`, `web.go`, `agents.go`, `onboard.go`, `wireguard.go`. main.go also contains pre-existing registry changes, so its whole diff is not feature-exclusive.
- Dashboard: `src/App.tsx`, components `AccountPage.tsx`, `LiveRequests.tsx`, `SettingsPage.tsx`, `ProviderPage.tsx`, `SupportPanel.tsx`, `SupportBanner.tsx`, and `lib/api.ts`, `lib/support.ts`.
- `go.mod`: maintained existing go-oidc/v3 v3.17.0, oauth2 and WebSocket dependencies made direct as required. No vulnerability-remediation upgrades.
- Runbooks `doc/provider.md`, `doc/support.md`, scoped `doc/security-model.md`, this evidence file and `examples/support/`. No generated dashboard/dist intended for shipping.

## U/R implementation coverage

| Unit | Evidence and remaining limit |
| --- | --- |
| U1 | Existing registry WIP preserved and process fixture starts client registries. Branch ancestry, selective staging and shipping isolation remain coordinator-owned. |
| U2 | Durable installation/account identities, permission-generation invalidation, atomic new migration DDL/version markers, separate idempotent account backfill. Dashboard account/migration tests pass. |
| U3 | Independent single-process Provider SQLite, organization/installation registration/assignment/enable controls, immutable named technicians, account-owned cases. Organization isolation and assignment visibility tests pass. Bounded management connector/registry-health UI implemented; focused browser checks pass. |
| U4 | Maintained OIDC verifier, browser state/nonce/S256 PKCE, signed azp/acr/auth_time, encrypted access tokens, authenticated fail-closed introspection, no refresh. Real signed TLS callback tests pass. Actual deployment IdP assurance remains external. |
| U5 | Five-minute 60-bit one-time SUPPORT grants, HMAC-only storage, atomic pending activation, 30-second reservation, replay/reissue/revoke, scoped persistent failed guesses, account/Tailnet lifecycle checks and synchronous audit. Concurrency, crash-window, rotation, ownership and runtime-disable tests pass. |
| U6 | Default-denied classified dashboard surface, typed projections, secret sentinel test, authorized device mutations, DB/audit atomicity and memory-after-commit. WireGuard deletion uses recoverable durable intent. Exhaustive side-effect/secret-path AE proof remains incomplete. |
| U7 | Enrolled outbound WSS, pinned purpose-specific Ed25519 keys, nonce/connection/sequence/org/installation binding, bounded calls and disconnect invalidation. Real WebSocket concurrency and envelope replay tests pass. Full credential-swap matrix remains untested. |
| U8 | Existing dashboard with support banner, dual identities, customer case/code/revoke, Provider selectors/assignment/View-as, cache and EventSource termination. Normal dashboard_config_writes preserved while support HCL apply denied. Audit UI paginates and exports JSON. Reload/EventSource acceptance awaits coordinator lifecycle fixes. |
| U9 | Real production-command process fixture, synthetic TLS IdP, two gateways/registries, two hosting modes, browser script, runbooks/examples, retention cleanup, race/vet/build checks implemented. Full AE contract and security gates remain open. |

R1–R13 have implementation paths across U2–U9. This is not a claim that every R-ID is fully verified. R2 management health and R10 audit UI/export have implementation and targeted evidence. R13 exhaustive acceptance remains incomplete. R11 process tests exercise local login/state and registry through connector, IdP and Provider outages. Deployment gates below remain mandatory.

## Historical verification (before continuation and review fixes)

All heavy commands ran on zo-computer via `/home/egsox/.local/bin/zo-run`, working checkout `/root/zo-run/clawpatrol`. Remote dependencies were already cached. No new tools, dependencies or services installed. Commands below are remote command bodies unless stated otherwise.

| Check | Result / evidence |
| --- | --- |
| `go test ./internal/provider ./internal/supportwire ./internal/supporttest -count=1` | PASS. TLS OIDC, case/org isolation, maintenance, signed protocol and real WebSocket coverage. supporttest has no standalone tests. |
| `go test ./cmd/nebprx -run "TestSupport\|TestDashboardAccount\|TestDashboardNewMigration" -count=1 -timeout=4m` | PASS. Includes actual production-command process acceptance without browser hook. |
| `go test -race ./internal/provider ./internal/supportwire ./internal/supporttest -count=1` | PASS. |
| `go test -race ./cmd/nebprx -run "TestSupport\|TestDashboardAccount\|TestDashboardNewMigration" -count=1 -timeout=5m` | PASS, 76.387 s. |
| Browser-hook `TestSupportProcessAcceptance` | PASS, 29.088 s. Both customer and nebuless modes, isolated browser contexts, dual identity, cookie isolation, denied routes, 1440px/390px, customer revoke and technician keyboard end. |
| `go vet ./...` | PASS. |
| `go mod tidy -diff` | PASS, empty. Contradictory local LSP module warnings persist. |
| `gofmt -l .` and `git diff --check` | PASS, empty. Source formatting only, no repository-wide rewrite. |
| `cd dashboard && deno task format:check && deno task lint && deno task build` | PASS, zero lint findings. tsc runs as build prerequisite. Vite non-blocking chunk warning: JS 553.25 kB. |
| `CGO_ENABLED=0 GOOS=... GOARCH=... go build ./cmd/nebprx` | PASS: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, output files remote /tmp. |
| `go test ./... -count=1 -timeout=5m` | BLOCKED by three existing sandbox-host failures below. All other reported packages pass. |
| Targeted LSP active probes on feature paths and affected existing dependents | No compiler errors on final probes, but 12 unconfirmed checks and advisories remain. Not diagnostic-clean evidence. Supported checker fallback: remote Go tests/vet and dashboard tsc. |

Local command-output logs: `/tmp/prx-support-final-go.log`, `/tmp/prx-support-final-race-browser.log`, `/tmp/prx-support-final-dashboard.log`, `/tmp/prx-support-final-builds.log`, `/tmp/prx-support-final-suite.log`. These are operational evidence, not committed artifacts.

Latest private remote fixture: `/tmp/prx-support-final-browser-1793150947`. `browser-result.json` reports both installations passed, widths 1440/390, customer termination for gateway-a and technician termination for gateway-b. Four screenshots exist. Directory also holds generated private synthetic keys/databases. Do not attach whole directory or commit it. Process cleanup is automatic. Repeat instructions: [examples/support/README.md](../examples/support/README.md).

### Sandbox environment blocker

Unmodified `TestExampleSocksTunnel`, `TestPluginTunnelViaCompiles` and `TestPluginTunnelViaE2E` fail with:

> cannot establish a sandbox on this system: no sandbox backend works on this host: user namespaces unavailable (signal: killed); Landlock unavailable (kernel lacks Landlock (need >= 5.13 with CONFIG_SECURITY_LANDLOCK)).

Sandbox checks were not disabled, skipped or weakened. Need compatible test host to clear full suite.

### Historical security blocker (superseded by dependency work above)

`govulncheck ./internal/provider ./internal/supportwire ./cmd/nebprx` reports 13 reachable Go standard-library vulnerabilities on Go 1.26.6. GO-2026-6809 (crypto/x509) requires Go 1.26.10. Remaining reported stdlib fixes require at least Go 1.26.9. Scan also reports golang.org/x/net v0.58.0 findings fixed in v0.60.0. Remote log: `/tmp/prx-support-vuln.log`.

Dashboard lockfile audit reports 14 findings: 2 critical, 5 high, 6 moderate, 1 low. Remote logs `/tmp/prx-support-dashboard-audit.log` and `/tmp/prx-support-dashboard-audit.json`. These findings were not remediated because toolchain/dependency updates require separate approval. No security-clean or production-ready claim.

### Diagnostics remaining

Fresh targeted probes cover new packages, support files and modified gateway/dashboard dependents, but include unconfirmed silent-on-clean results. Local module warnings at `internal/provider/oidc.go:73,175` contradict existing direct requirements and empty remote tidy diff. No suppressions or weaker checks added.

- `internal/provider/maintenance.go:23,33,42` and `cmd/nebprx/support.go:181`: GORM N+1 heuristic on fixed SQL cleanup sequences. These are database/sql transactions over a fixed statement list, not per-record GORM queries. Kept required FK order.
- `internal/provider/management.go:16,33,49,125`, `http.go:92`, `oidc.go:319`, `cmd/nebprx/web_support.go:70,98,265`: heuristic covers row scans or fixed transactional cleanup. CurrentCase refresh is bounded to 100 cases but performs one connector round trip per case. No bulk protocol added.
- `internal/provider/events.go:52` and `cmd/nebprx/web_support_projection.go:73,88`: direct-write warnings concern typed JSON/SSE, not HTML. JSON marshaling escapes line breaks, and response type is text/event-stream. No HTML-template conversion appropriate.
- `internal/supportwire/link.go:75` and `internal/provider/events.go:62`: stale WaitGroup-scope findings do not match current channel-slot/poll code. Race checks pass. `support.go:125` lock/unlock pairs cover each drain operation before network work, not an unpaired unlock.
- `internal/supporttest/idp.go:99`: test redirect warning, exact callback allowlist checked before redirect. Not arbitrary browser target.
- `cmd/nebprx/provider.go:24` and `support.go:190`: CLI error output, intentionally stderr. New maintenance errors use generic structured logs without secret error values.
- Existing gateway `main.go:2487,2727,4231` has TLS/HTTP/pprof advisories outside changed support lines. Existing `web.go:383,419,496,512,535,563` has login redirect/conditional-cookie advisories outside support changes. Not broadened into unrelated hardening. Lower-priority existing typos/style/test modernization hints remain.

## Historical AE baseline (see current continuation status above)

| AE | Status and gap |
| --- | --- |
| AE1 | PARTIAL: real case/MFA/grant/activation/dashboard and dual identity pass. Device rollback test passes. Saved context/feature flag equivalence and complete ordinary mutation browser flow not all asserted. |
| AE2 | PARTIAL: both hosting modes and real organization/installation selection pass. Organization/case isolation tests pass. Full management audit/registry-health browser matrix missing. |
| AE3 | PARTIAL: account/case ownership, organization isolation, assignment revision and Tailnet authority/login/subject rejection pass. Full forged-header/other-technician browser matrix missing. |
| AE4 | SUBSTANTIAL TARGETED: 100 concurrent redeems, one winner, replay, persistent scoped five-failure lockout and crash/lost-handoff recovery pass. Not all scenarios driven concurrently through browser/production processes. |
| AE5 | PARTIAL: customer revoke and terminal browser state pass. EventSource cleanup implemented. Dedicated buffered-SSE race/no-post-revoke-byte proof not complete. |
| AE6 | PARTIAL: injected-clock expiry, password rotation, reassignment, account generation/Tailnet lifecycle tests pass. Complete real-timer and technician-disable during active browser matrix missing. |
| AE7 | PARTIAL: whole typed payload sentinel scan and denied raw-config/unknown routes pass. Exhaustive API/UI/log/audit sentinel and every denied mutation/no-outbound-effect matrix missing. |
| AE8 | PARTIAL: audit fault rollback, fail-closed introspection, real connector disconnect and runtime disable checks pass. Full read-only/full disk plus Provider outage with ordinary customer/registry control process missing. |
| AE9 | PARTIAL: signature, connection replay, unsafe target and WebSocket concurrency checks pass. All credential-purpose swap cases missing. |
| AE10 | PARTIAL: host-only cookie isolation, missing-cookie denial, one-use activation and strict Origin enforcement covered by code/HTTP/browser flow. Full malicious cross-site/sibling/forged-host browser matrix missing. |
| AE11 | PARTIAL: approved A then B sessions on distinct origins and real assets pass. Explicit cache/console/EventSource/reload sentinel assertions missing. |
| AE12 | PARTIAL: 390px/1440px no-overflow, keyboard technician end and retention clock advance pass. Full keyboard flow, light/dark artifact review, audit pagination UI/export missing. |

## Remaining implementation and deployment gates

Coordinator independent review and correction loop remain required. Accepted AE suite is not exhaustive yet. Management health and audit pagination/export UI are implemented. Full browser acceptance, context-fixture adaptation, exhaustive faults, coordinator review corrections and independent artifact review remain incomplete. Do not mark U1/U9, all R-IDs or Definition of Done complete.

Live rollout also needs real IdP MFA/disabled-user introspection evidence, wildcard DNS/TLS/ingress streaming behavior, dedicated per-customer OS/network/state/credential isolation, approved roster/phone process, consistent backup/restore with retention and verified key rotation. These are deployment prerequisites, not bypass opportunities. Dependency remediation applied. Exact-snapshot verification and residual advisory disposition remain coordinator-owned. Full-suite sandbox host support remains required.
