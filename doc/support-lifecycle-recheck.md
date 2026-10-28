# Final F1/F2 lifecycle recheck

Verdict: **APPROVE** for the final support F1/F2 corrections. No actionable finding remains within these boundaries. This is scoped code approval, not evidence that every release gate passed.

## Scope and source identity

Read root AGENTS.md and doc/support-final-review.md first. Independently inspected the focused delta from index tree 2f493f5d8a86bda7e210197ce7dfd998220005ad to 1a9f86be2d86cf3aa489b4038a477fdf989f52d2 in:

- cmd/nebprx/support.go
- cmd/nebprx/support_lifecycle.go
- cmd/nebprx/support_connector.go
- cmd/nebprx/support_store.go
- cmd/nebprx/support_lifecycle_review_test.go
- cmd/nebprx/support_acceptance_test.go (fixture split)
- cmd/nebprx/support_release_acceptance_test.go (SQL resource cleanup)
- doc/support.md

Also read the existing store admission/authorization, dashboard account bindings, support schema, dispatch locking and Link disconnect paths needed to assess those corrections. Did not reopen original support F1–F6, registry F1/F2, unrelated feature code, UI, or deferred production prerequisites.

Final focused worktree/index drift check against the stated current tree returned 0. The ZO release snapshot and private reviewer scratch copy matched these local SHA-256 values:

| Path | SHA-256 |
| --- | --- |
| cmd/nebprx/support.go | add5139d7249d03b432fc22a21596ff4565cb82f5e2d3a73b59d16800a773caf |
| cmd/nebprx/support_connector.go | 6bb54b6129cbf88dfd7598ec3a69999705c2c51a8a795ab642caee2dfa2b8430 |
| cmd/nebprx/support_store.go | 72840b070483dc035f811886f6517fc804b1568ca334960b804359eb3bb63fdc |
| cmd/nebprx/support_lifecycle.go | 99dc1921bb57972992b2aa518672146bebffb705e0e9a8dcfa2875261b7ca542 |
| cmd/nebprx/support_lifecycle_review_test.go | d2c60cf203ea81d078ecd779fa05f3f6c198db3ae18ee588160227e7af4768e6 |
| cmd/nebprx/support_acceptance_test.go | 1094d04ac419e8a9e84777ca11e725d51f201d7a22e6c51950ad0fee7fbd1f71 |

## Correction assessment

### F1: resolved

Startup reads the persisted support_control flag and never resets it from config. Both controls must permit startup. Disable commits its durable latch without rewriting deployed config. Status exposes configured, persisted and combined next-start state.

Explicit enable requires config opt-in, validates installation/organization against the local database, and inserts support_enable audit together with the flag update. An audit insert failure rolls that update back. Enable does not revive revoked grants or sessions. A process blocked by lifecycle persistence failure keeps its in-memory block even if another connection sets the durable flag back to 1. Documented restart requirement matches that behavior.

### F2: resolved

The lifecycle helper inserts per-session and per-issued-grant termination rows, plus a control row for disable, before revocation in the same transaction. A late control-audit failure rolls back earlier termination-audit inserts. Existing foreign keys preserve the account join needed for per-binding evidence. Rows carry effective account, case, grant, session where applicable, assignment revision, installation and organization. CLI uses local OS UID/username and CLI method. Automatic restart/disconnect uses system:gateway, local-system and SYSTEM, not a technician identity.

On lifecycle persistence failure, the helper sets the process block before attempting unaudited durable revocation plus disable, returns the primary failure joined with recovery failure, and never clears the process block. Admission, issue/redeem/activation and active-session authorization consult that block through enabled, caseValid or authorizeLocked. Restart construction returns no usable store when cleanup fails. The CLI factory does not falsely record gateway_restart just to perform a command.

Authenticated connector teardown clears its published link, runs the same audited cleanup and joins cleanup failure into the return error. The final generic cleanup error log is unconditional on context cancellation, so shutdown does not suppress persistence-failure reporting. It contains no credentials or raw database error. The outer retry loop also reports generic connector failure while running. Inspected these exact final log changes, not only the earlier raced snapshot.

## Independent focused verification

All Go execution stayed on ZO through ssh root@zo-computer. No local Go build/test, production-path probe, shared snapshot edit, index/branch change, agent, commit, push or deployment occurred. Only this report was written in the shared repository.

Copied the coordinator snapshot to private ZO scratch directory /tmp/prx-lifecycle-recheck.a9D7zp. Added only cmd/nebprx/reviewer_lifecycle_probe_test.go there. No application file or existing test was changed.

Ran:

```sh
go test -race ./cmd/nebprx -run '^Test(ReviewerLifecycle|SupportLifecycle)' -count=1 -timeout=3m
```

The independent probes cover:

- Config-disabled enable refusal, failed enable audit retaining the disabled latch, successful enable with exact local actor/installation/organization bindings, and old-consent denial after enable.
- Late control-audit failure rolling back already-inserted termination rows, successful durable recovery, and injected recovery-write failure retaining the process block.
- A subsequent durable enable not erasing the block, plus blocked authorization, issuance and activation after storage repair.
- Actual newSupportStore restart construction with injected audit failure returning no store, while durable recovery revokes the old session and disables admission.

The same selection ran all new author lifecycle tests: actual commands/read-only-config unchanged check, restart audit, unredeemed grant evidence, and real TLS/WebSocket connector disconnect with and without injected audit failure. The final generic disconnect-failure log appeared during the faulted transport case.

Results:

| Evidence | Result |
| --- | --- |
| Existing /tmp/prx-final-lifecycle-race.log | PASS, 27.976s, read independently; predates final log addition |
| Private reviewer-lifecycle-race.log, verbose | PASS, 42.855s; each independent probe and author lifecycle test passed |
| Private reviewer-lifecycle-race-confirm.log | PASS, 41.018s; independently captured shell exit 0 |

The first verbose invocation's test passed, but its shell wrapper subsequently failed because zsh reserves the variable name status. The confirmation used rc and returned exit 0. No wrapper error is presented as test success.

## Verification limits and release handoff

- No claim of LSP-cleanliness. This reviewer changed Markdown only and ran no local Go/LSP build activity. Focused ZO race compilation/execution supplies the independent code-check evidence.
- Trigger-based failures establish rollback, fallback and denial behavior, not power-loss durability or successful writes when storage rejects all writes. Such failure returns an error and cannot promise durable changes. The process block remains set. Failed-recovery data is revoked on a later successful startup before store exposure.
- Normal per-binding lifecycle audit is atomic. Exceptional unaudited recovery intentionally leaves an evidence gap, as documented. No claim of host-administrator-resistant audit.
- Independent scope was final F1/F2, not broad release revalidation. Shared final lint log reported 0 issues. Shared gateway selected race and process/browser logs later showed PASS in 291.177s and 71.699s respectively. Those are observed coordinator logs, not reviewer-owned reruns. Empty vet/build output was not treated as success.
- The concurrent /tmp/prx-final-tests.log ended FAIL: TestRunStackUDPForwarderActivityRefreshesIdleTimeout/upstream-to-child at daemon_session_linux_test.go:965, with client read activity UDP timeout; cmd/nebprx failed in 132.233s. This is outside the lifecycle delta and was not diagnosed or waived here. Coordinator must resolve that release gate separately. Scoped APPROVE does not authorize shipping with an unexplained failing full suite.

No new lifecycle finding. F1/F2 implementation approval granted for the inspected source identity above.
