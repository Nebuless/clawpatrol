# PR #1 correction review

**Verdict: APPROVE for these two corrections only.** No actionable correctness or security regression found in focused delta. Not whole-feature, merge, or release approval.

## Scope

Reviewed uncommitted changes against `17ccb3a2471338c071e89d57aa65b798b8cb8563` on `rebrand/prx-glass` in:

- `internal/provider/http.go`
- `internal/provider/oidc_flow_test.go`
- `dashboard/src/App.tsx`
- `dashboard/src/components/RequestDetailPage.tsx`
- `cmd/nebprx/support_integration_test.go`
- `examples/support/browser-acceptance.mjs`

Read root `AGENTS.md` target `.claude/CLAUDE.md`. Application files, index, branch, unrelated WIP, snapshot, and root logs left unchanged. Only this report written. No agents, commits, pushes, installs, or PR actions.

## Findings

**F1 — Cross-site OIDC continuation: corrected.** `internal/provider/http.go:48-50` replaces callback 303 with static 200 HTML and fixed same-origin `/` link. Committed Provider document gives subsequent user click Provider-origin navigation rather than retaining cross-site redirect context. No reflected callback values, script, external destination, or automatic redirect added. Existing callback verification still precedes cookie issuance. Single-use browser-bound state, PKCE, signed-token/nonce/authorized-party checks, recent MFA assurance, technician lookup, and introspection remain unchanged. Session cookie remains Secure, HttpOnly, host-only, Path `/`, SameSite Strict. Existing no-store, referrer policy, CSP, and host validation still cover callback.

Regression test `TestOIDCCallbackCommitsOriginBeforeStrictSessionNavigation` checks 200/no Location, continuation link, cookie protections, and authenticated follow-up. HTTP recorder cannot prove browser SameSite behavior, but browser flow now clicks continuation after synthetic MFA without repairing login through manual navigation. Fixture uses separate sites: IdP `127.0.0.4`, Provider `127.0.0.1`.

**F2 — Support fixture download: corrected.** `dashboard/src/App.tsx:149` passes resolved support state through `RequestDetailPage` to `ActionButtons`, which omits `DownloadActionButton` only during support. Customer behavior and permitted rule-preview control remain intact. Context loading gates dashboard rendering, so initial unresolved support state does not briefly expose download. Backend remains authoritative: unchanged `cmd/nebprx/web_support_policy.go` action projection rejects any query string, including `?fmt=fixture`, with 403.

Seeded action uses real `Sink` persistence, existing public endpoint, and explicit complete empty request-body capture. This satisfies existing fixture-export validation without relaxing it. Browser asserts customer download HTTP 200, support download button absent after action detail loads, active support banner/no ended view, and direct fixture request 403. Subsequent device/settings navigation continues under active support. Earlier unknown body-capture fixture refusal was valid validation, not evidence of F2; confirmed red run instead reaches absent-button assertion.

## Verification

**V1 — Independently performed:** focused `git diff --check` passed. `node --check examples/support/browser-acceptance.mjs` passed on ZO. SHA-256 values for all six reviewed files match ZO snapshot `/tmp/prx-release-final-2f493f5d`.

**V2 — Independently inspected existing ZO evidence:**

- `/tmp/prx-pr1-oidc-red.log`: named regression fails on original 303 continuation.
- `/tmp/prx-pr1-download-red-confirm.log`: browser fails at absent-button assertion line 355.
- `/tmp/prx-pr1-provider-race.log`: provider race suite passes, 3.009s.
- `/tmp/prx-pr1-feedback-browser.log`: combined browser/process acceptance passes, 56.071s.
- `/tmp/prx-pr1-feedback-lint.log`: uncapped Go lint reports `0 issues.`
- Dashboard format log passes; lint log reports zero warnings/errors; build log finishes successfully with large-chunk advisory. No bundle changes requested in this correction review.

**V3 — Limits:** no independent heavy Go/browser rerun. Existing logs are root-run evidence, not reviewer-run commands. Full serialized Go log `/tmp/prx-pr1-feedback-tests.log` was empty when inspected, so full-suite result remains pending. No fresh LSP diagnostic claim, no cross-browser proof beyond supplied Chromium acceptance, and no audit of unrelated PR behavior. Root owns final checks, publishing, and thread resolution.
