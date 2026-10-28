# Registry implementation verification

Verified **2026-10-27**. All builds, Go tests and process checks ran on
zo-computer. Operating instructions and schemas: [registry.md](registry.md).

## Delivered

- Standalone `nebprx registry --mode server|client`, with distinct persistent
  SQLite roles and init, enrollment, import/update, inventory, sync, subscription,
  local check and status actions.
- Gateway `--registry-mode server|client`, prefixed service options and integrated
  shutdown.
- Four-ecosystem OSV ingestion, attributed raw records, corrections, withdrawals,
  quarantine and source health.
- Installation authentication, verified TLS, an explicit loopback HTTP testing
  exception, pinned signed scopes and persistent replay protection. Exact
  versions and private inventory remain local.

## Final tests and build

The final registry suite and focused CLI/gateway selection passed under the race
detector. Exact command, run through `zo-run`:

```sh
go test -race ./internal/registry ./cmd/nebprx \
  -run 'TestRegistry|TestGatewayShutdown|TestImport|TestConcurrentSubscribe|TestMembership|TestConservative|TestLifecycle|TestEmpty|TestInterrupted|TestZIP|TestSnapshot|TestPrivacy|TestRole|TestTLS|TestSigned|TestMalformed|TestCheck|TestSync|TestOpen|TestPublished' \
  -count=1
```

Results: registry **3.325s**, CLI selection **2.171s**. All current registry tests
are selected. `go vet ./internal/registry`, formatting of every changed Go file
and `git diff --check` passed. The final binary was built with
`go build -o /tmp/nebprx-registry-root ./cmd/nebprx`.

Before the final two review fixes, the broader CLI suite also passed under the
race detector (**536.416s**), excluding `TestExampleSocksTunnel`,
`TestPluginTunnelViaCompiles` and `TestPluginTunnelViaE2E`. The worker's unfiltered
run failed those three because ZO lacks usable user namespace/Landlock facilities
for their plugin sandboxes. Their assertions were not changed. The complete
repository-wide Go suite was not run. The final reconciliation and subscription
changes are covered by the final registry/CLI selection above.

## Independent findings and resolution

The [Pi review](registry-review.md) reproduced two defects. The executor observed
both regressions fail before fixing production code:

| Finding | Resolution | Regression |
| --- | --- | --- |
| F1: mixed mapped/unmapped affected entries produced complete negatives | Validate required affected-package fields before ecosystem filtering; malformed entries mark coverage partial | `TestImportQuarantinesPartiallyUnmappedAffected`; separate coverage preserves valid other-ecosystem entries |
| F2: concurrent Store handles lost acknowledged subscriptions | Read inventory, candidates and limits, then write candidates in one SQLite transaction; contention returns an explicit error | `TestConcurrentSubscribeAcrossStoreHandles`, 30 concurrent rounds |

The executor reran the reviewer's original probes against final production files:

```sh
GOPROXY=off GOSUMDB=off go test -race ./internal/registry \
  -run '^TestReview(PartiallyUnmappedActiveAdvisory|ConcurrentSubscribe)$' \
  -count=1 -v
```

Both passed (**1.333s**) in `/tmp/prx-registry-review.hxNkeW` on ZO. The malformed
record now produces `indeterminate` with `partial` coverage. This is executor
validation of the original probes, not a second independent review. No actionable
finding from that review remains unresolved.

Earlier executor regressions cover request-supplied policy overrides, persisted
trust checks before sending credentials, wrong-ecosystem imports, corrupt local
credentials, coordinate quarantine and published package-name forms.

## Real processes

The final binary passed:

```sh
bash examples/registry/smoke.sh /tmp/nebprx-registry-root
```

This exercises separate server/client processes, signed sync, affected/fixed
findings, first-seen online/offline checks, restarts and invalid-token rejection.
ZO artifacts: `/tmp/prx-registry-smoke.WMaQQl/`.

A temporary executor harness started a real gateway with client mode and then
server mode, each with isolated state and ephemeral ports. Authenticated registry
status, gateway readiness, SIGTERM exit 0 and registry listener closure all
passed. ZO artifacts: `/tmp/prx-gateway-registry-root-texpvjpu/`.

## Live exports and signed local checks

The final binary reconciled all four observed exports. Go, crates.io and PyPI used
configured HTTPS downloads; npm used the complete OSV ZIP downloaded during this
verification with its explicit observation time.

| Ecosystem | Accepted source advisory records |
| --- | ---: |
| Go | 9,758 |
| PyPI | 14,653 |
| crates.io | 2,974 |
| npm | 7,970 |
| Total | 35,355 |

These are observed advisory-record counts, not unique vulnerabilities, exhaustive
coverage or a capacity benchmark. **All four source states were partial** because
the license allowlist excludes some sources and malformed coordinates are
quarantined. Their omission cannot create complete negative results. MAL records
are among the excluded sources in v1.

A fresh enrolled client synchronized the real `golang.org/x/crypto` scope and
checked `v0.10.0`: `known_affected` / `deny`, with 41 advisory-record matches and
explicit partial coverage. The local check retained that result after stopping
the global service.

ZO evidence:

- `/tmp/prx-registry-real-root-k0mz1hl3/`: global SQLite and export evidence.
- `/tmp/prx-registry-final-root-7avvzax_/result.json`: final imports, counts,
  health, signed sync and offline result.
- `/tmp/prx-registry-final-root-7avvzax_/global-status.json`: final source status.

The test state directories contain test credentials; publish only selected
non-secret evidence files, not entire directories.

## Boundaries

The release hosts and synchronizes intelligence. It does not enforce agent
execution or model responses. Python epoch/pre/post/dev/local versions and GIT
ranges remain indeterminate. Inventory is an explicit package list rather than a
dependency resolver or graph extractor. Signing is pinned Ed25519, not full TUF;
signing-key rotation requires authenticated rebootstrap. Resource and source
limits are documented in [registry.md](registry.md).

Existing website, branding, skills and research changes were preserved. No
production service or credential was deployed.
