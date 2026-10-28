# Registry correctness/security review

**Executor resolution:** Both findings below were fixed after this independent
review. Repository regressions and the reviewer's original two probes pass
against final production files. See [verification](registry-verification.md) for
commands and scope. The original review and verdict are preserved below; this
note records executor validation, not a second independent approval.

## Actionable Findings

### F1 — High: partially unmapped affected entries can produce a fresh complete negative

- **Location:** `internal/registry/ingest.go:196-214`. Downstream negative: `internal/registry/query.go:108-109`.
- **Trigger:** Import an active advisory with one valid npm affected entry and another entry whose package has a name but no ecosystem. The valid entry sets `relevant=true`. The missing-ecosystem entry bypasses both canonical validation and quarantine. Import succeeds with complete health and replaces previous bindings.
- **Reproduction:** First import `GHSA-review` affecting npm/example 1.0.0. Replace it with this single-record export, then publish, activate and check npm/example 1.0.0:

```json
[{"id":"GHSA-review","modified":"2026-01-01T00:00:00Z","affected":[{"package":{"ecosystem":"npm","name":"other"},"versions":["1.0.0"]},{"package":{"name":"example"},"versions":["1.0.0"]}]}]
```

- **Observed:** `finding=no_known_match coverage=complete_for_declared_sources freshness=fresh`. Reproduced with and without the race detector in `TestReviewPartiallyUnmappedActiveAdvisory`.
- **Impact:** A malformed partial record erases a known finding and asserts complete negative coverage. No forged signature or untrusted check decision is needed. Trigger requires malformed source/import data, not an unauthenticated client write. Decision remains `review`, so this is a false intelligence negative, not an automatic allow.
- **Minimal remedy:** Validate every affected package's required structural fields before ecosystem filtering. Missing ecosystem/name should fail reconciliation atomically or mark the ecosystem partial. Preserve legitimate complete entries for other ecosystems. Add this mixed-valid/unmapped regression; existing entirely-unmapped and invalid-name tests do not cover it.

### F2 — Medium: concurrent subscriptions silently lose successful writes

- **Location:** `internal/registry/store.go:351-373`, especially the read at 354 before the transaction at 365.
- **Trigger:** Two CLI processes or independently opened Store handles share one client directory and subscribe different public candidates concurrently. Each reads the old JSON candidate list outside its write transaction. Per-Store mutexes do not serialize separate handles/processes.
- **Reproduction:** Open two client Stores on the same directory. Start concurrent `Subscribe(npm/first@1.0.0)` and `Subscribe(npm/second@1.0.0)`. Both return nil, but `Subscriptions()` contains only one identity. Reproduced on the first iteration in three runs, including two race-enabled runs, in `TestReviewConcurrentSubscribe`.
- **Impact:** A successful explicit subscription disappears without error. Its scope stops being refreshed and eventually expires. This contradicts the supported concurrent administrative CLI use documented at `doc/registry.md:305-307`. This is transactional lost-update behavior, not a Go memory race.
- **Minimal remedy:** Move candidate read, limit check and write into one SQLite transaction, with locking/conflict handling that prevents both stale writes from succeeding. Keep private-inventory checks consistent with that transaction. Add a two-handle concurrency regression requiring both candidates after two successful returns, or an explicit error for the losing operation.

## Coverage

Read `AGENTS.md` and reviewed these implementation paths, including untracked files:

- All ten Go files under `internal/registry/`, including existing tests and root's boundary regressions.
- `cmd/nebprx/registry.go`, `cmd/nebprx/registry_test.go` and only registry-related changes/context in `cmd/nebprx/main.go`.
- `doc/registry.md` and all five files under `examples/registry/`.

Reviewed matching boundaries and unsupported forms, withdrawal handling, source/license filtering, reconciliation transactions, snapshot signatures and scope/installation binding, replay floors, persisted pins before outbound requests, redirect refusal, private subscription filtering, local/installation authentication, file permissions, service lifecycle and CLI defaults. Confirmed root's listed fixes exist and their existing regression tests pass in the isolated package run. Did not report their superseded behavior.

All compilation/tests ran on ZO via `ssh -o BatchMode=yes root@zo-computer`. Created independent scratch copy `/tmp/prx-registry-review.hxNkeW` containing only `go.mod`, `go.sum` and `internal/registry/`. Reviewer probes live only in its `internal/registry/review_test.go`. SHA-256 comparison confirmed all ten copied repository Go files match CAX sources. No installs, production-source changes, branch/index changes or live-secret reads.

### Exact verification commands and outcomes

Remote commands ran from `/tmp/prx-registry-review.hxNkeW`:

```sh
GOPROXY=off GOSUMDB=off go test -race ./internal/registry -skip "^TestReview" -count=1
# PASS: existing registry suite, 2.249s.

GOPROXY=off GOSUMDB=off go test ./internal/registry -run '^TestReview' -count=1 -v
# Exploratory probes: concurrent subscription failed; quarantine probe passed.
# Zero-time withdrawal probe also failed its exploratory assertion; not adopted
# as a finding because withdrawal presence currently suppresses records and
# no required chronology contract establishes that timestamp as invalid.

GOPROXY=off GOSUMDB=off go test ./internal/registry -run "^TestReviewPartiallyUnmappedActiveAdvisory$" -count=1 -v
# FAIL: F1 reproduced.

GOPROXY=off GOSUMDB=off go test -race ./internal/registry -run "^TestReviewConcurrentSubscribe$" -count=1 -v
# FAIL: F2 reproduced.

GOPROXY=off GOSUMDB=off go test -race ./internal/registry -run "^TestReview(PartiallyUnmappedActiveAdvisory|ConcurrentSubscribe)$" -count=1 -v
# FAIL: both F1 and F2 reproduced.
```

One preliminary attempt to append F1's scratch test had a shell/Python quoting error. Its following test command reported `[no tests to run]`. That result is not counted as evidence. Corrected scratch append and actual failing tests above followed.

Local read-only checks:

```sh
git diff --check -- cmd/nebprx/main.go
bash -n examples/registry/smoke.sh
# Both exit 0.
sha256sum internal/registry/*.go
# Compared with same command on ZO scratch copy; repository sources match.
```

No CLI compilation, full CLI race suite, process smoke or real-corpus ingestion executed by this reviewer. Root's supplied runtime evidence remains root-reported. Root's active corpus run and existing state were untouched. No LSP diagnostic claim: production code was not changed; only this Markdown report was written.

## Verdict

**Request changes.** F1 permits a false complete negative. F2 loses successfully acknowledged subscriptions under supported concurrent CLI use. Root owns fixes and final verification.

## Residual non-blocking limitations

Accepted v1 scope: full reconciliation, explicit signing-key rebootstrap rather than TUF rotation, restricted Python release tuples, unsupported GIT ranges, unknown package existence/graph, resource ceilings and no command/reply enforcement. No redesign requested. Real-corpus completeness and broader CLI/runtime behavior were not independently revalidated here. Partial-coordinate quarantine returns indeterminate/partial rather than a complete negative in the separate scratch probe.
