# Support acceptance completion

Executor recovery, 2026-10-28. The Pi acceptance worker saved two new test files
before its tool request timed out. Its remote test process finished successfully:
`/tmp/prx-release-gateway-race.log` (137.576s) and
`/tmp/prx-release-packages-race.log` on zo-computer. The coordinator recovered
the files and checked those logs. No production code was changed by this worker.

## Added observations

- `TestReleaseDeniedRoutesNoOutboundOrStateChange` enumerates the production
  routes classified as denied. Each declared method must deny, append an audit
  row with the actual actor/effective identity, leave all other application
  tables and the configuration file unchanged, and make no default-transport
  HTTP call. Injected headers/body/query sentinels must not appear in response
  or any audit column. This trap does not observe arbitrary OS network syscalls.
- `TestReleaseOutcomeAuditFailureWithholdsCustomerReads` injects a SQLite
  completion-audit failure for state and event reads. No customer body is
  released, the support session becomes unusable and ordinary customer session
  authentication still works.
- `TestReleaseRevocationBeforeFinalReadRelease` injects a durable revocation
  after initial admission but before final buffered state/event release. No
  customer body or successful completion audit is released. This tests the
  gateway admission boundary, not recall of network bytes already sent.
- `TestReleaseCredentialPurposeBoundaries` sends registry, connector,
  support-cookie, Provider-cookie, signing-key and spent-enrollment values to
  actual cookie/enrollment/connector/registry handlers. Only each intended
  credential advances its own boundary. The valid connector reaches WebSocket
  upgrade validation; this test does not itself perform that handshake.
- `TestReleaseForgedActorIgnoredByManagementAudit` sends forged actor, forwarded
  user and Tailscale headers through an authenticated management request. Audit
  retains the verified technician and no fabricated effective customer.

The snapshot was `/root/zo-run/clawpatrol-release-acceptance`. Reproduction:

```sh
go test -race ./internal/provider ./internal/supportwire ./internal/registry \
  -count=1 -timeout=180s
go test -race ./cmd/nebprx \
  -run 'TestRelease|TestSupport(AllDenied|RealTimer|AuditReadOnly|AuditUses|TypedProjection|DeviceMutation|OriginAnd|Failure|Concurrent|EventsBounded|Runtime|Pending|Unknown)' \
  -count=1 -timeout=180s
```

These commands ran only on zo-computer. Package race results were Provider
9.817s, supportwire 1.771s and registry 10.702s. The caller's 240-second shell
timeout included compilation and expired before the gateway result was
delivered; it did not establish a failing test. The resulting gateway log
contains a passing exit summary. Final lint may make further source changes;
that gate and the final independent review remain coordinator-owned.

## Acceptance trace

| Scenario | Observed evidence |
| --- | --- |
| AE1: approval and real customer view | Production-command TLS fixture, actual MFA/case/code/activation, actor/effective identity, profile mutation; combined browser verifies real saved profile/theme/analytics view and feature flags. |
| AE2: hosting modes and management | Fixture starts separate customer-hosted and Nebuless-hosted gateways/client registries. Browser creates/assigns/registers/manages without a phone code; View-as requires approval. Isolation/management-audit tests enforce org boundaries. |
| AE3: actor/case/assignment isolation | Verified OIDC and local ownership/assignment tests, production foreign-account/org denials, and new forged-header audit assertions. |
| AE4: one-time admission | Production fixture sends 100 concurrent redemption requests, requires one winner and rejects replay. SQLite lifecycle tests cover persistent failure counters, restart and pending-activation recovery. These fault variants are not all browser-driven. |
| AE5: local revocation and stream end | Combined browser revokes locally, clears visible data and closes EventSource. New deterministic final-release tests withhold buffered state/events after revocation. |
| AE6: expiry and identity changes | Injected-clock lifecycle/password/assignment/Tailnet tests plus real-timer tests. Browser disables technician introspection during activity and observes terminal state. |
| AE7: secret projection and denied mutations | Typed projection sentinels plus new production deny-inventory DB/config/audit/default-HTTP side-effect checks. This is bounded sentinel coverage, not a proof about every future field or OS side effect. |
| AE8: audit and dependency failures | Read-only/full-store tests, new final-audit failure tests, fail-closed introspection, actual connector/IdP/Provider process outages with ordinary local login/state and client-registry controls. |
| AE9: purpose and transport separation | New credential-handler swap matrix plus production registry/connector credential rejection, signed-envelope nonce/sequence/connection replay tests and real WebSocket tests. |
| AE10: browser/host/session boundaries | Production HTTP checks activation method/replay, Origin/Host, sibling host, anonymous copied URL and cookie attributes. Browser checks isolated host-only cookies and denied routes. |
| AE11: reload and customer switch | Combined browser opens separate approved A/B sessions, reloads real assets, checks prior-case absence, no-store behavior, runtime errors and terminal EventSource cleanup. |
| AE12: responsive/accessibility/audit retention | Browser checks 1440/768/390px, keyboard end, audit pagination/export and 44px termination target. Executor inspected light management and dark support captures; clock-controlled maintenance tests check retention. This is not a full screen-reader audit. |

The evidence combines real service/browser scenarios with focused SQLite and
handler fault tests. It is not an assertion that every fault variant ran through
a browser or that live production readiness is established. Actual IdP assurance,
wildcard DNS/TLS/ingress, hosted OS isolation, operator phone/roster procedure,
backup restore and key rotation remain deployment validation requirements.
