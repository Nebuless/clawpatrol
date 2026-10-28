# PR #1 feedback verification

Coordinator record, 2026-10-28. Baseline head `17ccb3a` passed all six GitHub
checks, including Linux namespace/Landlock and macOS sandbox jobs. The two
subsequent Codex P2 findings were valid and corrected together.

| Finding | Correction | Regression evidence |
| --- | --- | --- |
| [OIDC callback redirect with a Strict cookie](https://github.com/Nebuless/clawpatrol/pull/1#discussion_r4396944356) | Successful callback returns a static same-origin completion document with a Continue link. Cookie protections, verified MFA and callback validation remain enforced. | New callback HTTP test failed with 303 before the fix. Browser removes its old manual navigation workaround and reaches authenticated management through the continuation link after cross-site IdP login. |
| [Unsupported fixture download in support view](https://github.com/Nebuless/clawpatrol/pull/1#discussion_r4396944369) | App propagates support mode to request-detail controls, which omit Download action. Backend `?fmt=fixture` rejection remains enforced. | Real browser failed the absent-button assertion before the UI fix. A persisted synthetic HTTP event verifies ordinary customer download 200, hidden support control, active support view and direct fixture-request 403. |

The event is produced through the real Sink with a complete, empty request body
and a public synthetic endpoint. The first seed omitted completion state and
correctly failed ordinary fixture export; it was corrected before observing the
support-control regression. No recorded self-test JSON was edited.

Heavy verification ran only on zo-computer in
`/tmp/prx-release-final-2f493f5d`. Private browser fixtures contain synthetic
credentials and must not be published wholesale.

| Check | Result |
| --- | --- |
| Callback red test | Expected FAIL, 303 instead of a continuation document; `/tmp/prx-pr1-oidc-red.log`. |
| Download-control red browser | Expected FAIL at the support button assertion after customer download succeeded; `/tmp/prx-pr1-download-red-confirm.log`. |
| Unfiltered Provider race suite | PASS, 3.009s; `/tmp/prx-pr1-provider-race.log`. |
| Dashboard format/lint/TypeScript/Vite | PASS; `/tmp/prx-pr1-dashboard-{format,lint,build}.log`. Existing bundle-size warning remains. |
| Uncapped golangci-lint v2.12.2 | `0 issues.`; `/tmp/prx-pr1-feedback-lint.log`. |
| Full process/browser fixture | PASS, 56.071s; `/tmp/prx-pr1-feedback-browser.log`, private fixture `/tmp/prx-pr1-feedback-green-browser/`. |
| Serialized full Go suite | PASS, shell exit 0, CLI 87.224s; `/tmp/prx-pr1-feedback-tests.log`. Same three host-dependent sandbox tests excluded; baseline CI ran their supported backends successfully. |
| Fresh independent Pi review | [APPROVE for both corrections](support-pr1-feedback-review.md), with matching local/ZO source hashes. Reviewer inspected existing test logs and performed syntax/diff checks; no independent heavy rerun is attributed to it. Full Go result completed after that review. |

No dependency, advisory protocol, cookie policy or backend download permission
was relaxed. Live IdP/ingress and customer-isolation rollout checks remain
separate deployment prerequisites in the original runbooks.
