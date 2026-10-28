# Support lint verification

Coordinator follow-up: the setup error now begins with "client routing through
Tailscale", preserving the product name and meaning while satisfying ST1005.
Final selected-source checks are recorded in
[support-release-verification.md](support-release-verification.md).
The worker result below retains its original ownership boundary and snapshot.

Verified 2026-10-28 03:57 UTC. Implementation worker only. No commit, push,
staging, dependency, lint-configuration or browser/UI changes performed.

## Changes

- Corrected error handling, response/SQL cleanup, exported API comments,
  HTTP status constants and unused arguments in support/provider/registry code.
- Removed unused `supportIdentity`.
- Replaced registry-disabled `(nil, nil)` with `errRegistryDisabled`; updated
  gateway caller and lifecycle assertion to preserve disabled behavior.
- Tests check setup and cleanup errors. Restart-fixture cleanup accepts
  `sql.ErrConnDone` because those tests intentionally close the original
  connection before reopening it.
- Renamed fixture `IdP` field to `IDP`, retaining JSON key `IdP` explicitly.
- Did not edit either sibling-owned `release_acceptance_test.go` file.

## Checks and evidence

Heavy checks ran on ZO isolated snapshot `/tmp/prx-lint-worker`, not CAX.
Pinned binary: `/tmp/prx-release-tools/golangci-lint`, v2.12.2.
Latest owned Go files were copied into that snapshot without overwriting
sibling-owned release acceptance files. Root must rerun on its integrated tree.

- `golangci-lint run --timeout 5m --max-issues-per-linter 0 --max-same-issues 0 ./...`: exit 1, exactly one pre-existing out-of-scope issue. Log: `lint-final.log`.
- `go test -count=1 -timeout 10m ./internal/provider ./internal/supportwire ./internal/supporttest ./internal/registry`: pass; supporttest has no tests. Log: `packages-test.log`.
- `go test -count=1 -timeout 10m ./cmd/nebprx -run 'Test(Support|Registry|Dashboard|Provider)'`: pass, 17.015s. Log: `gateway-test-final.log`.
- `go test -race -count=1 -timeout 10m ./internal/provider ./internal/supportwire ./internal/registry`: pass. Log: `race.log`.
- `go test -race -count=1 -timeout 10m ./cmd/nebprx -run 'Test(Support|Registry|Dashboard|Provider)'`: pass, 173.794s. Log: `gateway-race.log`.
- Local `gofmt -l .` and `git diff --check`: no output.

Gateway race check preceded the final fixture-only JSON tag preservation.
Uncached gateway tests and uncapped lint were rerun after that tag change.
No browser rerun was performed by this worker.

Remaining CI blocker: `cmd/nebprx/setup.go:1548`, ST1005, capitalized error string
`Tailscale client routing is not supported on macOS; use a WireGuard gateway with --whole-machine`.
`git show HEAD:cmd/nebprx/setup.go` contains the identical line at 1548.
This source lies outside worker ownership and was left unchanged. Full CI lint
is therefore not green.

## LSP coverage and auxiliary warnings

Active `lens_diagnostics` checked 50 changed Go paths with `source=lsp`,
`scope=paths`, `severity=all`. Results: one confirmed clean, ten with auxiliary
findings, 39 inconclusive. Final two-test-file retry was also inconclusive.
No confirmed LSP cleanliness claim is made. Successful package compilation,
tests and pinned lint provide supported Go checker evidence above.

Auxiliary warnings remain outside these lint corrections:

- `cmd/nebprx/db.go:132,139,143`, `support.go:180`,
  `support_mutation.go:56,95,151,165,174,179`,
  `support_store.go:226,244,260`, `web_support.go:72,100,261`,
  `internal/provider/events.go:31`, `oidc.go:335`: heuristic GORM/N+1 warnings
  on existing `database/sql` logic. No query redesign authorized.
- `cmd/nebprx/provider.go:24`, `registry.go:108,126`, `support.go:189`:
  CLI usage/output triggers generic printf logging warnings. CLI output is
  intentional, not production request logging.
- `cmd/nebprx/registry.go:163` and `internal/provider/events.go:76`:
  generic WaitGroup heuristic matches cancellation/end callbacks,
  not `sync.WaitGroup.Done` calls.
- `cmd/nebprx/support.go:124`: existing explicit unlock-in-loop heuristic.
  Lock restructuring is outside this lint task.
- `cmd/nebprx/main.go:144,808,1279,1312,2488,2728,2862,2948,2990,3132,3247,3472,3473,3577,3637,3641,3714,3718,3748`:
  unrelated existing query, map, TLS, output and spelling heuristics.
  Worker changed only the registry-disabled branch near line 3723.

## Concurrency and snapshot caveat

Git index comparison showed concurrent root documentation staging, not worker
staging. No checkout/reset was used. During initial isolated `--fix`, shared
cached package paths caused internal fixes to land in root's isolated ZO
snapshot `/tmp/prx-depsec.cRSOah`. Root was informed. Subsequent verification
used dedicated `/tmp/prx-lint-worker-cache` and no further `--fix`. Treat the
original root snapshot as modified and rebuild it before independent review.
