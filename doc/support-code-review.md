# Independent support/provider security and correctness review

**Verdict: Request changes.** 2026-10-28. Six required findings. This review does not approve shipping or deployment.

## Scope and revision

Reviewed actual tracked/untracked support/provider/account/connector/audit/projection/mutation sources, gateway integrations, migrations 0025–0028, dashboard integration, CLI, tests and runbooks against AGENTS.md, accepted hosting/support plan including #review-corrections, and doc/support-implementation.md. Registry internals and unrelated marketing/branding/skills excluded except service boundaries.

HEAD: 060ff6dfb52a12ea5154b2d3739843b874ec1fde plus uncommitted work. Reviewer wrote only this report and isolated temporary probes. No production edits, staging, commits, pushes, installs or delegates. F1–F6 below are code-review IDs, distinct from plan's earlier F1–F8 corrections.

Baseline captured at 2026-10-28T02:16:38Z in /tmp/support-review-baseline.json. Tests used isolated remote snapshot /tmp/prx-independent-review-20261028, copied around 02:22 UTC. Cached Go 1.26.10 and dependencies, GOPROXY=off, GOTOOLCHAIN=local. Dashboard artifacts copied, not rebuilt. Browser findings below use complete control-flow evidence, not fresh browser execution.

## Required findings

### F1 — High: support device deletion can deadlock ordinary activity and revocation

Evidence: cmd/nebprx/support_mutation.go:33–34,114–118; cmd/nebprx/agents.go:347–355,380–384; cmd/nebprx/onboard.go:189–193.

Deletion holds onboard mutex through agents.Delete, which needs AgentRegistry write lock. Concurrent AgentRegistry.snapshot holds AgentRegistry read lock while calling ProfileForIP, which needs onboard mutex. Ordinary dashboard snapshot and support deletion deadlock. Dispatcher retains support admission mutex, also blocking local revocation. Device DB deletion/audit can already be committed.

Independent TestIndependentReviewDeleteLockOrder held snapshot's read lock, started real support deletion, then attempted profile lookup. Both blocked until probe released simulated snapshot lock. Probe releases locks afterward to avoid hanging test process.

Minimal fix: finish committed onboard memory updates, release onboard mutex, then delete agent while retaining support admission serialization. Check other lock-order paths. Do not weaken audit atomicity.

Verification: deterministic concurrent snapshot/delete test with watchdog, successful subsequent revoke, DB/memory consistency, race test.

### F2 — Medium: credential overwrite preserves unsafe existing permissions

Evidence: cmd/nebprx/support.go:285; owner-only reader internal/provider/oidc.go:59–72.

Enrollment uses os.WriteFile(path, credential, 0600). Mode applies only to newly created files. Pre-existing 0644 target remains world-readable and receives bearer credential. Existing symlinks are followed. Startup later rejects permissive file, after exposure. Installation-key proof limits credential-only abuse, but does not excuse disclosure of distinct connector credential.

Minimal fix: secure exclusive creation or owner-only temporary file plus atomic replacement in trusted directory. Define overwrite behavior, reject symlink/non-regular targets, and never write secret bytes before permissions are safe.

Verification: absent target, existing 0644/0600 target, symlink, write failure and retry after consumed enrollment. Assert permissive file never receives credential.

### F3 — Medium: device page terminates valid support view

Evidence: dashboard/src/components/DevicePage.tsx:207; components/HITLBar.tsx:13–29; lib/api.ts:17–19,426–428; App.tsx:80–93,187–196; default-denied HITL route in cmd/nebprx/web.go and web_support_policy.go.

Home suppresses HITLBar during support, DevicePage does not. Its immediate poll calls /api/hitl/pending, intentionally denied with 403. Shared API wrapper interprets every support 403 as terminal and dispatches prx-support-ended. App clears customer data and renders session-ended screen. Normal navigation prevents permitted profile/delete workflows.

Complete control-flow chain verified. Independent dispatcher probe confirmed 403 for this route in otherwise active session. No fresh browser reproduction claimed.

Minimal fix: pass support state/capabilities into DevicePage and suppress excluded HITL component/poll, as home does. Keep backend denial. Check other always-mounted excluded fetches.

Verification: active support browser opens IPv4 device, stays active beyond poll interval, changes profile/deletes with normal confirmation, sends no HITL requests. Ordinary customer HITL unchanged.

### F4 — Medium: target validation rejects legitimate encoded query values

Evidence: internal/supportwire/protocol.go:85–98.

ValidateTarget rejects every percent sign before separating path/query. Dashboard encoding produces percent escapes for IPv6 addresses and profile names. Independent probe rejected:

- /api/status?profile=team%20one
- /api/events?agent=fd7a%3A115c%3Aa1e0%3A%3A1
- /api/analytics?range=1h&agent=fd7a%3A115c%3Aa1e0%3A%3A1

Support filtering and IPv6 device operations fail. Resulting 403 can terminate browser view.

Minimal fix: validate canonical relative path separately, retaining encoded-path traversal/separator rejection. Use error-reporting query parser, reject malformed escapes/duplicate keys, permit valid escaped values without altering signed target bytes.

Verification: legitimate queries pass and decode once. Encoded/double-encoded path traversal, separators, malformed query escapes, duplicate keys and absolute URLs still fail.

### F5 — Medium: normal backlog exceeds SSE cap and ends support

Evidence: cmd/nebprx/web_support_projection.go:80–121; internal/provider/events.go:47–50; ordinary 4 MiB response limit in cmd/nebprx/web_support_policy.go.

Gateway serializes entire recent backlog plus live updates without enforcing SSE's 64 KiB limit. Provider rejects batch above 65536 bytes and sends session-ended. Independent probe populated 500 normal synthetic event summaries. Gateway returned HTTP 200 with **125024 bytes**, which Provider rejects. Bounded event count is not bounded serialized size. Latest Provider terminal path also deletes support route.

Minimal fix: byte-bound producer batches with explicit continuation/cursor, preserving events and each delivery's authorization/audit boundary. Do not raise cap to conceal mismatch or silently discard events while claiming complete backlog.

Verification: full ring, long permitted fields and live events produce <=64 KiB deliveries, preserve continuity, remain usable, stop at revocation boundary.

### F6 — Medium: saved account context not wired into dashboard

Evidence: cmd/nebprx/web_support.go:191–239; web_support_projection.go:49–78; dashboard/src/lib/support.ts:11–12; components/SupportBanner.tsx:21; App.tsx and components/AccountPage.tsx.

Context API stores only profile/theme. No dashboard source calls /api/account/context. Saved fields are only typed/displayed in banner. Backend narrows integrations using manually persisted profile, but UI neither persists customer selections nor applies saved theme. Planned filters/last route absent and strict decoder rejects them. R3/KTD2/U2/U8 promised actual saved context, not display-only metadata.

Minimal fix: wire bounded validated context save/load into customer controls and apply read-only support snapshot, including agreed route/filter fields. Alternatively obtain explicit plan reduction. Support must not overwrite customer preferences.

Verification: customer saves non-default context, reloads, then phone-approved support opens same allowed context. Support navigation cannot mutate customer preferences. Unknown profiles/fields and stale revisions rejected.

## Independent verification

Probes and logs: /tmp/prx-review-probes/{review_probe_test.go,review_retention_test.go,results.log,targeted.log,existing-tests.log}. Test files added only to isolated remote snapshot.

First focused probe used direct SSH into isolated snapshot. Follow-up verification used ~/.local/bin/zo-run, command body changing to isolated snapshot. No local heavy builds/tests ran.

Existing-suite commands on snapshot:

    go test ./internal/provider ./internal/supportwire -skip TestIndependentReview -count=1 -timeout=2m

PASS: provider 0.619s, supportwire 0.013s. Unfiltered supportwire run matters: earlier targeted regex selected no supportwire tests.

    go test ./cmd/nebprx -run "TestSupport|TestDashboardAccount|TestDashboardNewMigration" -count=1 -timeout=4m

PASS: 13.619s, including process acceptance without browser hook. No selected existing tests failed.

Independent TestIndependentReview probes failed as intended for F1/F4/F5. Additional retention probe showed submitted case retained after 400 days. Not required defect: approved retention explicitly purges closed cases, while production has no close transition. Clarify lifecycle/policy rather than silently delete open cases.

Reviewer changed no production code. No independent LSP-clean claim: implementation report records unconfirmed coverage. No fresh race/browser/full-suite/platform-build/vulnerability scan performed. Root must run final checks on fixed tree.

## Plan coverage and remaining proof

| Area | Review result |
| --- | --- |
| Hosting/provider management/registry, R1/R2/U1/U3 | Separate management authorization and client-registry health paths present. Health detail and audit pagination/export UI now exist despite stale implementation report. No global-registry authority found in reviewed support dispatch. Final ancestry/staging root-owned. |
| Effective context/mutations, R3/U2/U8 | Actor/effective identities distinct. F1/F3/F4/F6 block parity. Blanket read-only workaround unacceptable. |
| Consent/identity, R4–R7/U4/U5 | Local HMAC grants, 60-bit generation, five-minute issuance, 30-second pending activation, 15-minute active session, persistent challenge buckets, account generation, current tailnet checks and maintained OIDC verifier present. Selected tests pass. Latest additional tests drifted after snapshot. |
| Projections/exclusions, R8/U6 | Explicit method allowlist and typed projections present. Analytics uses supportAnalytics and reprojects events, so raw analytics leakage suspicion is not finding. Exhaustive secret/outbound-side-effect matrix needs final evidence. |
| Connector/browser, R9/U7 | Enrolled WSS, purpose proofs, nonce/sequence/body binding, host-only cookies and POST handoff implemented. F2/F4/F5 remain. Exact-tree purpose-swap/browser-origin matrix execution needed. |
| Audit/revocation, R10/U5/U6 | Synchronous transactions and mutation intent/outcome paths present. F1 can block revoke. Latest buffered-SSE test waits for admission lock then checks next dispatch, not already-buffered Provider/browser bytes arriving after completed revoke. Keep proof scope explicit. |
| Local independence, R11 | Ordinary services separate. Process and full/read-only fault tests give partial evidence. Final protected-route IdP/Provider outage with valid root/tailnet cookies and no fallback still needs proof. |
| Migrations 0025–0028 | DDL/version markers transactional, backfill separate/idempotent. Generation and atomic-migration tests pass. Full backfill fault/retry and legacy coverage need final evidence. |
| Acceptance/operations/UX, U9/R12/R13 | Runbooks/process fixture present. Fixes and latest worker/browser proofs plus exact-tree QA outstanding. Live IdP/DNS/TLS/host isolation/backup/rotation are deployment gates, not code findings. |

## Dependency status and drift

Original 13 reachable Go findings and 14 dashboard findings are historical, not assumed current. doc/support-dependency-security.md records Go 1.26.10, x/net 0.62.0 and dashboard remediation, provider/supportwire clean scan and dashboard audit. Its Landlock compiler blocker was later fixed by root; review tests compiled. Do not re-report resolved field-name issue. Fresh whole-program scan still required. Required-module openpgp no-fix advisory differs from reachable-symbol vulnerability.

Baseline-to-02:29 UTC drift: cmd/nebprx/{support_http_test.go,support_integration_test.go,support_surface_test.go}, internal/provider/{events.go,health_test.go,http.go,oidc_flow_test.go}, internal/supportwire/protocol_test.go. Latest stream terminal-route deletion and relevant tests inspected, but not all rerun in isolated snapshot. No final-tree approval. Three known sandbox-host full-suite failures are pre-existing, not findings.

Finding-source SHA-256 at final evidence capture:

| File | SHA-256 |
| --- | --- |
| cmd/nebprx/support_mutation.go | 8b014255ab1dfda5350abe0150aa2a83bcd3161d0b2ea27f2f07e1607839c297 |
| cmd/nebprx/support.go | d69d19ece25403e8140fc838d8596ba44ea279353619535a10ef4b723a0b2a51 |
| cmd/nebprx/web_support_projection.go | 1672d01ce108073e649d96b86f1631d7d869c423f8c780942a6d3ae50dc73e9f |
| internal/supportwire/protocol.go | 0ff32e9db51fe5bbcb289cf94217b431b57aaf8e577c40d0fad6689e7ffce125 |
| internal/provider/events.go | e14cd539a3a5d6069a83c761d74cb83a5776e429658fcfa0e437b30b879d99dc |
| dashboard/src/lib/api.ts | a43e8438c97ba0fdcd3fa830a3c351ff494b26b509c29de79966ded961b705ee |
| dashboard/src/App.tsx | 00ad52e15b1a7f12cc0fadd7ed5f948fec9e3019a022c2d803846edc5d2fbeca |
| cmd/nebprx/web_support.go | 8d8f89b6ebf3bbc2ff4c91dc2264b8070a3dfa65923808750e023a303ab9d7f6 |

Full temporary manifest: /tmp/prx-review-probes/current-hashes.json. Recheck findings/hashes after fixes and rerun acceptance/security gates before shipping.
