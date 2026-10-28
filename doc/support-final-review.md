# Independent final support/registry review

Verdict: **REQUEST CHANGES**. Two implementation findings remain. Final lint is now clean. Live deployment prerequisites are separate gates, not additional code findings.

## Scope and source identity

Reviewed staged feature/prerequisite changes against HEAD `060ff6dfb52a12ea5154b2d3739843b874ec1fde`. The 14 previously approved PRX/platform commits are prerequisites, not fresh rename-review targets. Unstaged assessment-site/skills/research work is excluded.

Initial frozen index tree: `2f493f5d8a86bda7e210197ce7dfd998220005ad`. During review, the coordinator changed two staged test files relative to that tree: `cmd/nebprx/support_release_acceptance_test.go` and `internal/provider/release_acceptance_test.go`. Inspected those cleanup changes, including scoped deferred SQL closure helpers. A final drift check also found flag.ErrHelp success handling added to runSupport/runProvider and new cmd/nebprx/management_cli_test.go. Inspected that help-only change. It does not alter the findings. Completed core checks below preceded this last delta, so its checks remain coordinator-owned. Hashes below identify inspected files. The support.go hash includes the final help-only delta. Its prior tested hash was 4a3c86c502a6b6e02503213867524d12ed87231454b17b60238e0f4942409673. Both cleanup-test hashes matched the coordinator's ZO snapshot:

| Path | SHA-256 |
| --- | --- |
| cmd/nebprx/support.go | e3df0b9fe0ddf53a12992efa2a9176a8c46060ef7255982004af41a55ca823bb |
| cmd/nebprx/support_connector.go | abc45dd1e518de2df535a96f4f10222cc24419ab3048d95fa8402075bf02067d |
| cmd/nebprx/support_store.go | 72fe3479bda8b4cabd609e1c6446bee6f4f75fab042fb7e0c63c2f13a63dee38 |
| cmd/nebprx/support_release_acceptance_test.go | fa6166a4117517a8591871bc2c75a670c31b62b296adf687ca0032f0f18eaff9 |
| internal/provider/release_acceptance_test.go | 2825bcf17925f0c023acc8091650e5a04864e4e7cba082104ef808543cfca5ab |

Read the plan, including `#review-corrections`, and the requested implementation, acceptance, review-resolution, lint and registry handoffs. This review writes only this report. No source/index/branch/commit/push/deployment changes or child agents were made.

## Findings

### F1. Medium: failed config persistence lets restart undo local support disable

**Locations:** `cmd/nebprx/support.go:237-249`, startup at `cmd/nebprx/support.go:106-111`.

The disable command commits session/grant revocation and `support_control.enabled=0`, then rewrites the enabled configuration with `os.WriteFile`. If the config cannot be written, for example a read-only config mount or disk failure, the command returns an error but leaves the current process disabled and the old `enabled=true` configuration intact. On restart, `startSupport` unconditionally sets the database control back to 1 from that stale configuration. Previously revoked grants stay revoked, but new connector admission and approvals become possible without a successful explicit re-enable.

This is not a bypass of phone approval or a claim that the command returned success. It is a fail-open restart transition after an emergency disable partially completed. The retained disabled database state is discarded, contrary to the local-disable authority and restart expectations.

**Smallest remedy:** preserve a durable disabled latch that startup cannot override accidentally, or persist the disabled configuration safely before a restart can re-enable support. Keep immediate runtime denial on persistence failure and return a clear failure. Test config-write failure followed by startup with the still-enabled file. No new architecture is required.

### F2. Medium: bulk local revocation and connector/startup termination omit security audit

**Locations:** `cmd/nebprx/support.go:225-252`, `cmd/nebprx/support_connector.go:102-114`, `cmd/nebprx/support_store.go:59-65`.

CLI `revoke-all`/`disable`, connector-disconnect cleanup and startup each revoke support grants/sessions without inserting any `support_audit` row. The ordinary customer revoke and technician end paths do audit in the same transaction, so these alternate termination paths leave materially different evidence for the same consent lifecycle. An operator can terminate every session successfully and the customer's retained audit has no termination event or reason. Restart loses the explanation in exactly the lifecycle the plan requires to remain reviewable.

**Independent reproduction:** a private ZO scratch copy at `/tmp/prx-final-review.JVrhga`, copied from the coordinator snapshot, runs `go test ./cmd/nebprx -run '^TestFinalReviewRestartRevocationAuditProbe$' -count=1 -timeout=2m -v`. It creates and activates a real fixture session, records the audit count, closes the store and constructs a new store over the same database. Result: `session state=revoked audit before=3 after=3`, followed by the expected failed assertion `restart revocation persisted without audit event`. Log: `review-probe.log` in that private directory. Production source was not changed.

**Smallest remedy:** insert bounded lifecycle audit rows in the same revocation transaction, with truthful local-operator or system identity, termination reason and affected session/grant bindings. Do not invent a technician actor for a local command. Preserve fail-closed termination if audit persistence fails, and surface the failure. Cover CLI bulk revoke/disable, disconnect and restart, including audit failure. This follows R7/KTD9 and the plan's retained lifecycle evidence, not a new product requirement.

## Goal and prior-finding coverage

- **G1. Registry/service roles:** explicit gateway, client/server registry and Provider CLI entry points remain separate. Client inventory/public subscriptions do not turn a hosted installation into the global server. Server ingest/signing and client authenticated verified sync remain purpose-separated. Registry query decisions use persisted administrative policy rather than a caller's claimed allow decision.
- **G2. Identity/consent:** Provider uses OIDC verification, configured assurance/authentication age and live introspection against a named technician roster. Stable local account generations, current root availability and tailnet membership checks constrain effective identity. Local one-time code redemption, pending activation, fixed session expiry and revision checks remain gateway authority. No silent management impersonation was found.
- **G3. Transport/dashboard:** signed envelopes bind connection, sequence, purpose, installation, actor, case and session. Canonical relative targets, bounded headers/bodies and method-aware default-deny dispatch prevent this connector becoming a generic tunnel or shell. Actual dashboard assets and typed data projections are served. Allowed mutations have explicit transaction/audit boundaries. Raw HCL, credential/OAuth material, action bodies/fixtures and HITL work release remain denied.
- **G4. Context/lifecycle:** customer context is bounded and validated, revision checked and restored by the UI. Support consumes a read-only snapshot and does not PATCH customer preferences. Banner/terminal transitions, session-aware state cache handling and polling/EventSource cleanup are wired. F1/F2 above qualify lifecycle completion.
- **G5. Original support F1–F6:** current code releases onboard lock before agent deletion, reserves fresh private credential files exclusively, suppresses DevicePage HITL in support and gates initial mount, parses escaped queries separately from canonical paths, uses byte-bounded SSE cursor batches with explicit eviction gaps, and restores saved profile/theme/route/filter context without support writes. The prior defects were not retained as open findings.
- **G6. Original registry F1–F2:** ingest validates required affected-package fields before ecosystem filtering and preserves quarantine uncertainty. Subscription candidate reads, limit check and write now share a transaction. The earlier false-negative/lost-update implementations are no longer present. Relevant regression tests remain.

## Verification evidence and limits

Heavy execution stayed on ZO. Independently read final log contents from `/tmp/prx-final-*.log`, not just historical executor claims:

| Check | Evidence observed |
| --- | --- |
| Pinned final lint | `prx-final-lint.log`: `0 issues.` after the two test-cleanup corrections. Earlier four errcheck and two sqlclosecheck results are superseded. |
| Full Go tests | `prx-final-tests.log`: all listed packages pass, including CLI 215.972s. This final log does not contain the three sandbox failures reported for the older snapshot. |
| Unfiltered core races | Provider 5.071s, supportwire 1.045s, registry 18.840s. |
| Gateway selected race | CLI 345.553s. |
| Cleanup acceptance race rerun | `prx-final-release-cleanup-race.log`: CLI 32.017s and Provider 1.376s. |
| Process/browser | `prx-final-browser.log`: CLI 90.233s, pass. Scenario detail comes from inspected test code and the coordinator's report, not a newly driven browser session by this reviewer. |
| Registry process smoke | Pass for affected/fixed, online/offline admission, client/server restart and invalid token. Fixture `/tmp/prx-registry-smoke.Ptjpsp`. |
| Dependency checks | Module verification says all modules verified. Dashboard audit says no known vulnerabilities. Go vulnerability result: zero called/imported vulnerabilities, with required-module-only GO-2026-5932 for unused openpgp. |

The final vet/tidy/build logs read were empty. Empty files alone do not prove exit status. Their successful exits, four-platform builds, dashboard format/lint/TypeScript/Vite checks and selected website compatibility remain coordinator evidence, not independently reconstructed executions here. Historical LSP results were partly inconclusive; this report does not claim all affected files are LSP-clean. No code change was made by this reviewer.

The independent restart probe intentionally fails and is separate from the passing author suite. The author suite therefore does not establish complete termination-audit coverage.

Denied-route transport trapping does not observe every possible OS network syscall. Final-release revocation blocks later gateway releases, not bytes already sent. The credential-purpose test does not itself complete a WebSocket handshake. Responsive/keyboard fixtures are not a full screen-reader audit. No claim of tamper-proof host-administrator-resistant audit is made.

## Remaining deployment risks

- **R1. Live assurance:** validate real IdP MFA/auth_time and disabled-user introspection, wildcard DNS/TLS/ingress and exact host/origin/WebSocket behavior.
- **R2. Hosted isolation/operations:** validate per-customer OS/network/state/key boundaries, approved phone/technician-roster process, backup restore and incident revocation. Synthetic tests do not establish these.
- **R3. Provider operations:** Provider is a single-process SQLite service, not an HA/multi-writer design. Storage-key rotation requires coordinated downtime and clearing or re-encrypting sessions. Enrollment output failure can require owner-controlled repair before retry. These documented operational ceilings are not redesign demands.

Resolve F1/F2 with focused regressions and obtain a fresh review task for the resulting source changes. Implementation approval is withheld. Live deployment remains pending independently of code approval.
