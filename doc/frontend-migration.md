# pnpm and Vite+ migration

The dashboard and website use Node.js 22.23.4, pnpm 12.16.1 and Vite+ 1.4.0.
Each application has its own frozen pnpm lockfile and Vite+ catalog. Deno is
no longer needed to build, test or format either application. The Go CLI still
ships its dashboard as embedded static assets and requires no JavaScript runtime.

Use `make frontend-install`, `make frontend-check`, `make frontend-test`,
`make build` and `make site`. See [development setup](dev-setup.md). The complete
site task generates examples, checks TypeScript, builds the site, prerenders
marketing/docs pages and assembles the demo. Running only `vp build` skips those
other steps. CI and source installation use the same pinned toolchain and locks.

## Verification

Verification ran on EGGIE in an isolated checkout of the offered source, with
telemetry disabled. Evidence is retained at `/root/prx-pnpm-vite-logs` and
`/root/prx-pnpm-vite-runtime`; those paths are test artifacts, not prerequisites.

- Both frontend formatting, lint and type checks passed with zero findings.
- Dashboard: 20 Vitest tests. Site, worker and demo: 11 Vitest tests. All passed.
- Frozen installs passed, including clean dashboard source installation and a
  clean site dependency tree. Both locks rejected intentional manifest mismatches.
- Both `pnpm audit --json` runs reported zero known advisories on 2026-10-30.
  This is an audit snapshot, not a promise that packages are vulnerability-free.
- Full site, documentation, sitemap and demo builds passed. The existing large
  dashboard chunk warning remains; no limit was raised to hide it.
- Six marketing routes passed at 1440px and 390px, including fonts, overflow and
  46 internal links. Assessment screenshots were visually inspected.
- Site development served the assessment, server-rendered docs and raw markdown;
  a docs-file change triggered browser reload without page exceptions.
- The built CLI and source-installed CLI ran away from the source checkout,
  completed first-run login and served all 12 dashboard assets with expected
  embedded bytes. Nine static routes, including the demo, mounted without page
  exceptions. The production bundle contains no demo shim.
- The credential-page browser regression reproduced the old failure and passed
  after the review fix. It covers loading, failed fetch with retry, delayed
  responses after switching profiles, the legacy no-profile case and a known
  empty profile. Run on the build host after installing Playwright's browser:
  `cd dashboard && pnpm exec vpx scripts/check-device-credentials.mjs`.
- An injected frontend build failure also stopped source installation before Go
  compilation and left no installed binary.
- Registry smoke covered affected/fixed versions, first-seen online/offline
  admission, client/server restart and invalid installation credentials.
- Support browser acceptance passed with the original script: both hosting
  modes, MFA, approval, isolated customer context, three viewports, revocation,
  keyboard termination, technician disable and EventSource teardown. An earlier
  run timed out waiting for SSE; a diagnostic rerun and the final original-script
  run passed. The initial timeout remains in the logs; its cause was not proved.
- Go vet, golangci-lint 2.12.2, repository-wide gofmt and all HCL examples passed.
  Linux and macOS release builds passed for amd64 and arm64. Native macOS runtime
  and native optional dependency installation still depend on macOS CI coverage.
- The complete Go suite was run. All packages passed except three CLI tests:
  `TestExampleSocksTunnel`, `TestPluginTunnelViaCompiles`, and
  `TestPluginTunnelViaE2E`. EGGIE has neither usable user namespaces nor Landlock.
  Sandbox policy and test assertions were preserved. GitHub's sandbox jobs remain
  the authoritative checks for those kernel-dependent paths.

The installer test substituted Git clone transport with the exact trial tree;
normal frozen dependency installation, frontend compilation and Go compilation
then ran in a fresh temporary directory. This does not establish that a not-yet-
pushed GitHub ref was downloadable.

The included dashboard screenshot and fixture identities are fictional examples:
commit `fedcc49` changed the fixtures and screenshot together. Installed marketing
and Taste skill payloads retain their pinned upstream sources and licenses.

## Code review

The completed Pi review (`20261030-045022-43c41d0b`) retained one P2 finding:
credential-fetch failure looked like empty configuration. The fix adds explicit
loading, error and retry states, with a component keyed by profile so late
responses cannot publish another profile's credentials. The browser regression
failed against the old bundle and passed against the corrected bundle. The
review also covered build/CI fidelity, security, reliability and frontend races;
no other actionable finding remained. A different serving-model family could
not be verified, so the configured cross-model pass used the skill's local
adversarial fallback. It is not claimed as a verified cross-family review.

Focused hook-level permutations of queued preference PATCH failures, termination
during refresh, disconnect errors and stream-source changes remain future test
coverage. Full support browser acceptance exercises termination and cache clearing
but does not exhaustively permute those races. Native macOS installation/runtime
and the three kernel-dependent Go tests remain CI coverage requirements.

## Release validation

The release operator should inspect the new main CI run before cutting a release,
including the Linux namespace, Landlock and native macOS jobs. After deployment,
repeat dashboard login, asset loading, assessment/demo navigation and an approved
support session with revocation. No deployment was part of this migration.

Unexpected blank dashboards, missing embedded assets, failed support termination,
or new CI failures should block a release. Rebuild the previous known-good Git
revision and restore its matching frontend artifacts if rollback is needed;
keep the prior database and configuration intact. No database migration was added.
