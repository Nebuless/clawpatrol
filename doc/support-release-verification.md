# Support release verification

Coordinator evidence, 2026-10-28. Current status supersedes historical worker
handoffs in [support-implementation.md](support-implementation.md).
The repository implementation, dependency/lint corrections and final lifecycle
fixes are complete. Final Go/race/browser/lint checks pass with the host limits
below; the independent lifecycle recheck approves the corrected source. Publication is tracked
by the feature pull request. This record does not establish production deployment.

## Scope and ancestry

The writable branch is `rebrand/prx-glass`, based on `060ff6d`. Its 14 existing
commits above `origin/main` carry the previously authorized PRX brand/module/CLI
and platform changes. Current staged work contains the registry prerequisite,
Provider/support feature, dependency fixes, tests, examples, runbooks and plan.
Unrelated uncommitted assessment-site, marketing/skills and research artifacts
are excluded. Main gateway integration requires the registry and support changes
together; no artificial multi-PR stack is planned.

The three service roles are explicit CLI entry points: `nebprx gateway`,
`nebprx registry --mode client|server`, and `nebprx provider`.
Provider management requires a separate state directory, enrolled technician
identity and MFA. Possessing the public binary is not a secret entitlement.
Customer View-as additionally requires a local one-time phone approval.

## Executor checks

Heavy checks ran on zo-computer. The isolated source snapshot is
`/tmp/prx-release-final-2f493f5d`. Logs are private execution artifacts, not committed
credentials or fixtures.

| Check | Observed result |
| --- | --- |
| Unfiltered full Go suite, earlier snapshot | Only three sandbox-capability tests fail: `TestExampleSocksTunnel`, `TestPluginTunnelViaCompiles`, `TestPluginTunnelViaE2E`. Host has neither usable user namespaces nor Landlock. Assertions were not weakened. `/tmp/prx-depsec.cRSOah/root-full-suite.log`. |
| Final `go test -p 1 ./... -skip '^Test(ExampleSocksTunnel\|PluginTunnelViaCompiles\|PluginTunnelViaE2E)$' -count=1 -timeout=15m` | PASS on final lifecycle source, all packages; CLI 89.608s, shell exit 0. `/tmp/prx-final-tests.log`. Only the same three host-dependent tests are excluded; CI covers those backends. |
| Unfiltered package race | Provider 5.071s, supportwire 1.045s, registry 18.840s; `/tmp/prx-final-packages-race.log`. These packages are unchanged by the final lifecycle correction. |
| CLI support/account/context/registry race selection | PASS with final lifecycle source, 291.177s; `/tmp/prx-final-gateway-race.log`. Its regex matches no Provider/supportwire tests, which are established by the separate unfiltered command above. |
| Review regressions | F1/F2/F4/F5 race probes pass; `root-review-regressions.log`. F3/F6 have context/race/browser evidence in linked reports. |
| Recovered acceptance tests | Focused `TestRelease` race rerun passes: CLI 32.017s, Provider 1.376s; `/tmp/prx-final-release-cleanup-race.log`. See [acceptance trace](support-acceptance-completion.md). |
| Lifecycle regressions | CLI disable/re-enable, stale config/restart, termination bindings, issued grants, real TLS/WebSocket disconnect and audit-failure recovery pass under race in 27.976s; `/tmp/prx-final-lifecycle-race.log`. Final generic disconnect-error log was added afterward; the broader rerun includes that addition. |
| Go lint/vet | Pinned golangci-lint v2.12.2, uncapped: `0 issues.` Vet passes. `/tmp/prx-final-lint.log`, `/tmp/prx-final-vet.log`. No lint policy was weakened. |
| Module integrity/tidy | `go mod verify` passes; `go mod tidy -diff` empty. `/tmp/prx-final-modverify.log`, `/tmp/prx-final-tidy.log`. |
| Vulnerability analysis | Whole-program symbol and imported-package results have zero findings. Required-module-only `GO-2026-5932` remains: unmaintained openpgp is not imported or called. `/tmp/prx-final-vuln.log`. This is not an advisory-free dependency graph. |
| Release-platform builds | CGO-disabled Linux amd64/arm64 and Darwin amd64/arm64 builds pass after the lifecycle fixes; final combined command exited 0. No native macOS execution claim. |
| Dashboard | Frozen install, format check, lint, TypeScript and Vite build pass. Existing 566.62 kB bundle-size warning remains. |
| Full process/browser | PASS, 71.699s with final lifecycle fixes; `/tmp/prx-final-browser.log`. Private fixture `/tmp/prx-support-lifecycle-final-browser/`. |
| Registry processes | Selected-source Linux binary passes separate server/client signed sync, affected/fixed results, online/offline admission, restart and invalid-token smoke. `/tmp/prx-final-registry-smoke.log`; private fixture `/tmp/prx-registry-smoke.Ptjpsp/`. Registry source is unchanged by the lifecycle fixes. |
| Website/demo compatibility | Selected-index snapshot builds the landing page, 13 documentation pages and Demo with the updated dashboard; all five website worker tests pass. Existing bundle-size warning remains. The newer uncommitted assessment pages were excluded. |

Gitleaks 8.24.2 scanned an exported selected-index tree, not unrelated local WIP,
with redacted output. Its 12 findings are existing sanitization/AWS example
fixtures, generated discovery examples with placeholder authorization headers,
and one prose-pattern match in the plan. These were inspected; no real
credential was identified. No scanner rule or fixture was suppressed. Private
redacted report: `/tmp/prx-stage-secrets.json` on zo-computer. A scanner result
with manually rejected fixture/prose findings is not a zero-finding result.

The first final-suite attempt ran alongside race, browser and build jobs and
failed the untouched `TestRunStackUDPForwarderActivityRefreshesIdleTimeout`.
A concurrent five-repeat probe also failed once. After competing jobs finished,
five isolated repeats passed (2.081s) and the final serialized full-suite command
above passed. No UDP source, timing threshold or assertion changed. Failure logs
remain `/tmp/prx-final-tests-parallel-failure.log` and
`/tmp/prx-final-udp-parallel-failure.log`; passing probe is
`/tmp/prx-final-udp-recheck.log`. The short idle timer remains sensitive to load.

The browser exercises actual MFA, management without impersonation, both hosting
modes, customer preference controls, saved profile/theme/analytics restoration,
support reload, IPv4/IPv6 device views, audit pagination/export, local customer
revocation, keyboard technician end, IdP disable, cache isolation and EventSource
closure. It checks 1440/768/390px overflow and a minimum 44px End control. The
executor inspected desktop/light management and desktop/mobile dark support
captures. The mobile banner now stacks; no clipping was observed.

Earlier diagnostic coverage and its limitations are retained in
[context implementation](support-context-implementation.md) and the historical
worker handoff. The local TypeScript language server's unresolved new-module
finding remains a discovery/cache discrepancy despite successful project
compiler/build; no diagnostic suppression was added.

## Remaining release and deployment gates

The baseline selected-source snapshot was exported from index tree
`2f493f5d8a86bda7e210197ce7dfd998220005ad` to
`/tmp/prx-release-final-2f493f5d`. It contains the lint corrections, including the
coordinator's final setup error wording fix. Subsequent acceptance-test cleanup,
CLI help handling and lifecycle fixes were copied into that isolated snapshot.
Index tree `1a9f86be2d86cf3aa489b4038a477fdf989f52d2` contains the final executable
sources. Local/remote SHA-256 values match for all six lifecycle source/test
files. A separate lint cache avoids the worker's earlier cached-path interference.
No lint policy or test assertion was relaxed. Report-only changes after this tree
do not alter tested executable sources.

The [final review](support-final-review.md) confirmed the original support and
registry fixes, then found the durable-disable and bulk termination-audit defects.
Their [corrections](support-review-resolution.md#final-lifecycle-findings) now
respect the retained disabled latch and audit local/system termination. Fresh
independent [lifecycle recheck](support-lifecycle-recheck.md) approves this exact
source after private fault/recovery probes and author lifecycle tests pass under
race (41.018s, exit 0). Its full-suite warning records the earlier concurrent
failure, before the serialized full-suite and isolated UDP reruns completed.
The successful later commands above resolve that verification gate without
changing code or assertions. No broader approval is attributed to the scoped
reviewer. The original broad review confirmed the earlier support/registry fixes.

Actual production deployment also requires live IdP assurance and disabled-user
evidence, wildcard DNS/TLS/ingress checks, dedicated customer OS/network/state/key
isolation, approved phone/roster operations, backup restore and key rotation.
Those operational requirements are separate from a code PR. Neither synthetic
fixtures nor a successful build establish live production readiness.
