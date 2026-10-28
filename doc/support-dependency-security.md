# Support dependency security

Updated 2026-10-28. Dependency remediation and the Linux Landlock API adaptation
are applied. Whole-program build, vet and vulnerability analysis pass on the
executor snapshot. Feature sources continue changing; final release validation
must still cover the exact shipped head.

## Versions

Before means repository HEAD before this work, not the intermediate recovery
snapshot. Go 1.26.10 stays in the user toolchain cache on zo-computer. System Go
1.26.6 was not replaced. No Go 1.27 migration or new major dependency was added.

| Dependency | Before | After |
| --- | --- | --- |
| Go directive/toolchain | 1.26.6 | 1.26.10 |
| golang.org/x/net | 0.58.0 (recovery began at 0.60.0) | 0.62.0 |
| github.com/klauspost/compress | 1.18.6 | 1.18.7 |
| golang.org/x/crypto | 0.57.0 | 0.58.0 |
| golang.org/x/sys | 0.48.0 | 0.49.0 |
| golang.org/x/term | 0.46.0 | 0.47.0 |
| golang.org/x/text | 0.42.0 | 0.43.0 |
| golang.org/x/sync | 0.23.0 | 0.24.0 |
| golang.org/x/tools | 0.49.0 | 0.51.0 |
| dashboard oxfmt | ^0.48.0, resolved 0.48.0 | ^0.67.0, resolved 0.67.0 |
| dashboard Vite | ^8.0.11, resolved 8.0.13 | ~8.1.6, resolved 8.1.6 |
| postcss | 8.5.14 | 8.5.31 |
| nanoid | 3.3.12 | 3.3.20 |
| source-map-js | 1.2.1 | 1.2.2 |
| tinypool | 2.1.0 | 2.1.2 |

Normal Go resolution selected the transitive upgrades. In particular,
[net 0.62.0](https://proxy.golang.org/golang.org/x/net/@v/v0.62.0.mod)
requires Go 1.26.0 and newer crypto/sys/term/text. The
[text 0.43.0 module](https://proxy.golang.org/golang.org/x/text/@v/v0.43.0.mod)
requires tools 0.51.0 and sync 0.24.0. Direct go-oidc/v3, websocket and x/mod
requirements from the active feature work were preserved.

Primary advisory sources:

- [GO-2026-5841](https://pkg.go.dev/vuln/GO-2026-5841): compress/s2 fixed in 1.18.7.
- [GO-2026-6866](https://pkg.go.dev/vuln/GO-2026-6866) and
  [GO-2026-6860](https://pkg.go.dev/vuln/GO-2026-6860): x/net/html fixed in 0.62.0.
- [GO-2026-5932](https://pkg.go.dev/vuln/GO-2026-5932): unmaintained
  x/crypto/openpgp, no fixed version.
- [tinypool GHSA-85c8-ppgw-ccpr](https://github.com/advisories/GHSA-85c8-ppgw-ccpr):
  fixed in 2.1.2. oxfmt 0.67.0 is the selected patched parent.
- [PostCSS GHSA-fxqj-rqcc-2cmp](https://github.com/advisories/GHSA-fxqj-rqcc-2cmp),
  [nanoid GHSA-2v37-7h3g-55p8](https://github.com/advisories/GHSA-2v37-7h3g-55p8),
  [source-map-js GHSA-68fv-2mgg-jv7q](https://github.com/advisories/GHSA-68fv-2mgg-jv7q).
- [Go release history](https://go.dev/doc/devel/release).

## Evidence and coverage

All builds, tests, downloads and audits ran on zo-computer. Private evidence is
in /tmp/prx-depsec.cRSOah, not committed. Recovery refreshed source files from
CAX at 2026-10-28 02:04:37 UTC, retaining previously built dashboard assets.
It did not copy feature source back to CAX. Source inventory is
recovery-source.sha256 (SHA-256

e34a5cd9f28711a3682e573fc6c0c60ecc3b0f6a988534393cdd136a0a7978b8).
This is a working-tree snapshot, not a final commit or an atomic snapshot of
concurrently edited sources.

Recovery commands used GOTOOLCHAIN=go1.26.10:

| Command | Result / evidence |
| --- | --- |
| go get golang.org/x/net@v0.62.0 github.com/klauspost/compress@v1.18.7 | PASS, recovery-upgrade.log |
| go mod verify | PASS, all modules verified, recovery-verify.log |
| go test ./internal/provider ./internal/supportwire | PASS, both packages actually ran tests, recovery-packages.log |
| go test ./cmd/nebprx -run 'TestSupport\|TestDashboardAccount\|TestDashboardNewMigration' -count=1 | BLOCKED at compilation, recovery-cli.log |
| go build ./... | BLOCKED at compilation, recovery-build.log |
| govulncheck -show verbose ./... | BLOCKED loading packages, recovery-vuln.log. Not a clean audit. |
| govulncheck -show verbose ./internal/provider ./internal/supportwire | PASS, no vulnerabilities found, recovery-vuln-small.log. Only these packages and their dependencies. |
| cd dashboard && deno install --frozen | PASS, recovery-deno-frozen.log |
| cd dashboard && deno audit | PASS, no known vulnerabilities, recovery-deno-audit.log |

Historical compilation errors from the required x/sys 0.49.0 upgrade:

- internal/sandbox/landlock_linux.go:75 uses removed LandlockRulesetAttr.Access_fs.
  Upstream field is now Handled_access_fs.
- internal/sandbox/landlock_linux.go:77 uses removed Access_net.
  Upstream field is now Handled_access_net.

The executor adapted both names without changing either access mask.
`go test ./internal/sandbox ./internal/provider ./internal/supportwire` passed.
This establishes compatibility, not operation of unavailable kernel sandbox
features on the test host. The dependency worker preserved its ownership boundary
and did not downgrade the required module.

Executor follow-up in the same private snapshot passed `go build ./...`,
`go vet ./...` and `govulncheck -show verbose ./...`. Logs are `root-build.log`,
`root-vet.log`, `root-vuln.log` and `root-compatibility-tests.log`.
The scan found zero reachable vulnerabilities, zero additional vulnerabilities
in imported packages and the required-module-only openpgp advisory below.
Normal `go mod tidy` removed superseded checksum entries; its generated go.sum
was promoted to the working tree.

Earlier evidence, before the latest feature changes and final Go upgrades:
Deno frozen install, audit, lint, format:check and build passed. npm audit also
reported zero findings. Logs include deno-audit.log, deno-task-lint.log,
deno-task-format-check.log, deno-task-build.log and npm-lock.log. Build reported
a non-blocking chunk-size warning. Dashboard manifest and lockfiles were not
changed by recovery, and their frozen compatibility was checked again above.
Earlier Go tests used a regex that skipped provider/supportwire tests. Only
the unfiltered recovery package test result above establishes their execution.

The earlier full govulncheck on Go 1.26.10/x/net 0.60.0 reported zero reachable
vulnerabilities, clearing the original 13 reachable stdlib/x/net findings.
It still reported compress/s2 at imported-package level, x/net/html at module
level, and openpgp at required-module level. Cached primary advisory JSON was
rechecked for the exact compress/net fixed versions. The executor follow-up above
supersedes this earlier scan for the dependency graph; final feature checks remain
necessary after source changes.

## Audit policy and residual risk

Deno lockfile is release-authoritative. Keep package.json and deno.lock coherent,
use frozen installs, retain conventional npm compatibility, and regenerate npm
lockfiles normally rather than editing resolutions by hand. Existing npm lock
had no tracked diff. Prefer patched parent releases over overrides or broad
upgrades. Preserve Go checksum verification and resolve dependencies normally.

Gate releases on package tests, selected CLI support/account/migration tests,
Go build, symbol-level govulncheck, dashboard frozen install/audit/lint/format/build.
Classify advisories separately as reachable symbols, imported packages, or
required modules. Do not describe zero reachable findings as an advisory-free
graph. Recheck primary advisories and the exact shipped head after source changes.

GO-2026-5932 remains a no-fix advisory covering required x/crypto, including
0.58.0. Earlier whole-program analysis did not import or call openpgp. Do not
introduce openpgp usage. Re-evaluate reachability during final validation rather
than suppressing the advisory or claiming it was fixed by the module upgrade.

Local LSP checks for go.mod/go.sum are unsupported (no configured server).
Remote module verification and compiler/test checks are the available evidence.
The historical compiler blocker is resolved. Final release evidence must still
identify the exact source state checked.
