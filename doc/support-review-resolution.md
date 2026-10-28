# Support review corrections

This records executor changes in response to the independent review in
[support-code-review.md](support-code-review.md). It is not a second independent
approval. The subsequent [final review](support-final-review.md) confirmed these
six fixes and found two lifecycle defects. Their corrections are recorded below;
the independent [lifecycle recheck](support-lifecycle-recheck.md) approves them.
Final execution and publication evidence is in
[release verification](support-release-verification.md).

| Finding | Correction | Verification |
| --- | --- | --- |
| F1: device-deletion lock inversion | Commit device/audit changes and update onboard memory under its lock; release that lock before taking the agent lock. Keep support admission serialized. | `TestSupportDeleteDoesNotInvertSnapshotLocks` reproduces concurrent snapshot/delete ordering, then checks revocation. |
| F2: unsafe credential overwrite | Reserve a new owner-only credential file exclusively in a private, non-symlink directory before consuming enrollment. Existing files and links are rejected without writing credentials. | `TestSupportEnrollmentCredentialDestination` covers new/existing/private/public/symlink destinations. |
| F3: excluded HITL poll on device page | Pass support state to DevicePage and omit its HITL component during impersonation. Gate initial page mounting on resolved identity/context. The backend exclusion remains enforced. | Combined real-browser acceptance passes reload and IPv4/IPv6 device navigation. Process acceptance checks permitted profile mutation. |
| F4: encoded queries rejected | Validate the relative path separately. Parse query escapes with error reporting and reject duplicate decoded keys while retaining the original signed target bytes. | `TestTargetEncodedQueryAndCanonicalPath` covers IPv6, spaces, literal percent values, malformed escapes, traversal, separators and duplicates. |
| F5: SSE batch exceeds delivery cap | Produce byte-bounded batches with an explicit event cursor. Provider follows continuations through fresh authorization/audit checks. Report ring-buffer eviction as a visible backlog gap. | `TestSupportEventsBoundedContinuation` delivers all 500 long summaries without duplicates, below 64 KiB per batch, and checks stale-cursor gap reporting. |
| F6: account context not integrated | Persist a bounded customer profile/theme/safe-route/filter schema with revision checks. Initialize support from that context without writing it. Apply actual theme and profile selection; reject late results after termination. | Context validation/concurrency/projection tests pass under race. Real browser saves preferences through controls and restores the analytics view in both customer and support sessions. |

Enrollment writes a fresh file rather than replacing one. For reenrollment,
choose a new credential path and obtain a new Provider enrollment token. If a
network or filesystem failure occurs after the Provider consumes its enrollment
token, obtain a new token rather than replaying the consumed one. Incomplete
local credential files are removed; successfully stored credentials remain
owner-only. Existing credential files are never silently overwritten.

Event history is limited to the existing live ring. Cursor eviction is reported
explicitly; it is not a claim of complete historical retention. Each response
is bounded to the protocol's 64 KiB event limit. An individual summary exceeding
that limit is rejected explicitly. Audit admission/response-release boundaries
still apply to every batch; network bytes already released cannot be recalled.

Executor race regressions for F1/F2/F4/F5 pass in
`/tmp/prx-depsec.cRSOah/root-review-regressions.log` on zo-computer.
Unfiltered provider, supportwire and registry race suites pass in
`root-final-package-race.log`; CLI support/account/context/registry selection
passes in `root-final-race.log` (319.158s). The initial selection's package
regex matched no provider/supportwire tests, so only the separate unfiltered
run establishes their coverage.

Combined process/browser acceptance passed again after the mobile banner fix:
`/tmp/prx-support-root-final-browser.log` (57.443s), fixture artifacts in
`/tmp/prx-support-root-final-browser-1793157447/`. This covers both hosting
modes, saved context/reload, IPv4/IPv6 device pages, audit paging/export,
customer revoke, keyboard technician end, IdP disable, cross-session cache
isolation and EventSource termination. At 390px the banner stacks and its End
button is at least 44px high. Executor visually inspected the desktop and
mobile captures. Private fixture directories contain synthetic credentials;
do not publish them wholesale.

## Final lifecycle findings

| Finding | Correction | Verification |
| --- | --- | --- |
| F1: stale enabled config overrides emergency disable after restart | Startup no longer resets the durable database admission flag. Disable changes only state, allowing immutable config. Status reports both controls; explicit audited `support --action enable` is required to clear the latch. Revoked consent stays revoked. | `TestSupportLifecycleCommandsAndDurableDisable` exercises actual CLI commands, unchanged read-only config, startup and explicit re-enable. |
| F2: bulk/system termination has no retained audit | CLI revoke/disable, restart and connector disconnect share transactional termination audit with affected account/case/grant/session bindings. CLI uses OS UID/name; automatic cleanup uses the gateway system identity. Audit failure blocks process admission, attempts durable revocation/disable and returns an error; connector cleanup also emits a generic error during shutdown. | `TestSupportLifecycleRestartAudit`, `TestSupportLifecycleUnredeemedGrantAudit`, CLI lifecycle checks, real TLS/WebSocket disconnect, and injected audit-failure cases. |

The independent restart probe in the final review established the missing-audit
failure before the correction. Exceptional audit-storage failure intentionally
prioritizes denying access over rollback: fallback revocation can leave an
evidence gap, which is reported rather than presented as an audited success.
Fully unwritable databases cannot guarantee durable changes. Repair storage and
explicitly enable/restart support; old customer approvals cannot be reused.
