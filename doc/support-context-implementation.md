# Saved dashboard context (independent review F6)

2026-10-28. Scoped implementation, not deployment approval. Shared uncommitted checkout. No commits, pushes, branches, index changes, dependency edits or deployments by this worker.

## Implemented contract

- Authenticated customer context remains available only when the existing support capability is configured. Dashboard first loads state, then account context when advertised. Disabled gateways do not call or poll the context endpoint. Normal config-write permissions unchanged.
- Explicit bounded schema: profile (known current policy name, <=128 bytes), theme (light/dark/system), route (main/devices/device/analytics/request/settings/plugins/account, optional validated device_ip/request_id), filters (range 1m/5m/15m/30m/1h/6h/24h, color device/host/status, scale log/linear, valid IP, hostname <=253 bytes). Request IDs use 1–128 ASCII alphanumeric/underscore/hyphen characters. Payload limit 4096 bytes. No arbitrary URL, query, onboarding/login/support route or unknown JSON field storage. Revisions must be nonnegative safe JS integers with room for increment.
- Legacy absent rows, {} and profile/theme-only rows remain readable. Read-time validation rejects removed profiles and malformed saved context rather than broadening to all integrations.
- Dashboard profile and theme selectors have accessible labels, save/error feedback and explicit reload recovery after a conflict. Selected profile reaches backend /api/state?profile=... and changes actual integration projection. This is existing integration/profile selection, not new account RBAC or global request authorization.
- Dark/system theme uses dashboard-scoped Clear Glass variable aliases and semantic contrast overrides, existing fonts and bundled light-on-dark PRX icon. No shared token changes.
- Safe navigation auto-saves. Analytics range/color/scale and device/host chart filters are URL-backed, react to Back/Forward, and persist. Bar filter controls are keyboard buttons with pressed state. Unsupported/unknown query values are omitted from saved context.
- Bare origin restores last safe route and filters. Explicit deep links win. Ordinary onboarding links still work but never become saved context. Initial render waits for identity/context/profile validation before mounting page queries or HITL. DevicePage support prop preserved.
- Support initializes effective customer's profile/theme/route/filter snapshot once. Five-second polls refresh data, not technician navigation or theme. Support never calls account-context PATCH. Existing server support exclusion also denies PATCH.
- Saves serialize through one writer, coalescing pending edits against returned revision. Cross-tab conflict stops further writes and displays reload guidance. Profile switches discard old visible state until scoped response arrives. Session epoch rejects late state/context responses after end, clears caches and unmounts page/EventSource consumers. No changes to root's supportEvents cursor/batching or LiveRequests.

## Owned files

- cmd/nebprx/dashboard_context.go (new schema and shared read validation)
- cmd/nebprx/dashboard_context_test.go (new tests)
- cmd/nebprx/web_support.go (accountContext handler only)
- cmd/nebprx/web_support_projection.go (supportState context only)
- dashboard/src/App.tsx (lifecycle/gate, preferences mount)
- dashboard/src/lib/api.ts (typed context, profile-scoped cache, session epoch)
- dashboard/src/lib/dashboardContext.ts (new bounded view/theme helpers)
- dashboard/src/lib/dashboardContext.test.ts (new runnable assertions)
- dashboard/src/lib/useDashboardContext.ts (new lifecycle/write queue)
- dashboard/src/components/DashboardPreferences.tsx (new controls)
- dashboard/src/components/AnalyticsPage.tsx (URL-backed chart filters)
- dashboard/src/index.css (scoped dark theme)
- dashboard/src/components/SupportBanner.tsx (root-requested responsive stack and 44px minimum end-session touch target, existing text/case/time preserved)

Other workers own process/browser fixture and final whole-tree checks. SupportPanel, ProviderPage, AccountPage, DevicePage, LiveRequests, protocol/provider events, support mutation/policy and root regression tests were not edited here.

## Executed verification

Heavy checks exclusively through ~/.local/bin/zo-run, serialized remote checkout. Go 1.26.10 confirmed.

1. go test ./cmd/nebprx -run TestDashboardContext -count=1 -timeout=2m: PASS, latest 0.607s. Covers unknown/extra/oversize fields, profile/theme/route/filter validation, stale and concurrent revisions, own-account separation, old minimal context, removed profile read rejection, support snapshot, denied support PATCH, actual selected integration projection for ordinary and support state.
2. go test -race ./cmd/nebprx -run TestDashboardContext -count=1 -timeout=2m: PASS, final scoped rerun 7.827s including extra projection assertions and safe-integer bound. Helper assertions, TypeScript, lint and eight-file formatting checks also passed again.
3. dashboard: deno run --unstable-sloppy-imports src/lib/dashboardContext.test.ts: PASS. Tests encoded IPv6/filter roundtrip and exclusion of tokens, unknown fields, duplicate query selectors, unsafe routes and invalid IPs.
4. dashboard: node_modules/.bin/tsc -b --noEmit: PASS. node_modules/.bin/oxlint src: PASS, zero errors/warnings, 50 files.
5. dashboard: node_modules/.bin/oxfmt --check on eight owned TS/TSX/CSS paths: PASS. Only owned formatted paths pulled back. Local gofmt -l on four owned Go files and scoped git diff --check: empty.
6. dashboard: node_modules/.bin/vite build: PASS, latest Vite 8.1.6. Existing bundle-size warning remains (566.56 kB JS, 182.29 kB gzip). No bundle-splitting scope expansion.

## Diagnostics disposition

Active full targeted LSP scan on 12 changed files plus main.tsx, DevicePage.tsx and SettingsPage.tsx completed. Local TypeScript language server repeatedly reports App.tsx:22 missing new DashboardPreferences module despite file presence and successful remote project tsc/build. This is unresolved local LSP discovery/cache coverage, not a claimed clean LSP result. No suppression added. Supported project checker independently validates module/type resolution.

Remaining warnings: index.css:9/14/19/24/29 unknown @utility are existing Tailwind extensions understood by successful Vite build. web_support.go:71/99/259 N+1 heuristic findings concern existing support/audit loops outside owned handler. web_support_projection.go:143 Fprintf/XSS/logging heuristics concern root-owned JSON SSE serialization, not context edits. Extensionless import warnings match existing bundler tsconfig (allowImportingTsExtensions=false). Existing analytics inline styles encode measured chart values, not new styling abstractions. Auxiliary style/type hints remain nonblocking. No runtime/config/check weakening.

## Root browser evidence and handoff

Root reports combined full browser PASS in 53.482s, log /tmp/prx-support-root-browser.log. Reported coverage: actual customer profile/theme UI save, customer/support analytics route and filter restoration, IPv4/IPv6 device pages without HITL termination, both modes, audit paging/export, revoke/technician end/IdP disable, cache/reload/EventSource cleanup. Browser log was not independently re-read by this worker. Root owns fixture and screenshots.

Root fixed integration structured decoder and added synthetic 2001:db8::2 device because gateway boot intentionally purges fd7a rows. EventSource assertions explicitly navigate support to #/ because analytics correctly restores. Full browser not rerun here.

Root visual review of /tmp/prx-support-evidence/gateway-a-support-390.png found banner text collapsing beside end button. Requested fix applied: flex-col sm:flex-row, items-start sm:items-center, button min-h-11. Banner formatting, project TypeScript and targeted lint passed. Responsive delta screenshot verification remains root-owned.

Original acceptance checklist retained below as handoff detail, not a claim that every subcheck was individually verified by this worker.

1. On authenticated customer page choose Dashboard profile=limited and Dashboard theme=dark using labels. Change from all/default and light first so test proves a real save rather than fixture-seeded values. Wait for PATCH 200 and Preferences saved.
2. Navigate to #/analytics?range=6h&color=host&scale=linear&host=public.example (optional device=10.55.0.2). Assert URL-backed selected buttons and saved context. Check real dark canvas computed background and data-theme=dark, not banner text only. Navigate bare origin, verify saved route/filter/profile/theme restoration. Explicit #/devices should win.
3. Customer must visit account page to create case/issue phone code, which saves account as last route. After hiding code, return customer to analytics and await its save before technician opens activated support origin.
4. Assert support starts on same analytics route/filter state, uses /api/state?profile=limited and actual narrowed integration array, applies dark computed canvas, retains dual identity. Record requests from first navigation: no /api/hitl/pending, no PATCH /api/account/context. Navigate support locally, wait beyond five-second poll, assert route stays local and customer's stored context/revision unchanged.
5. Assert unknown fields/profile, unsafe route and oversize PATCH reject 400, stale revision rejects 409. For browser stale-revision UI, advance revision through customer API, then change theme and assert reload guidance rather than retry overwrite. Restore fixture after probe.
6. Delay a support state response, end session, release response. Ended screen must retain no customer data or reopened EventSource. Repeat support reload and session B isolation.
7. Capture theme/preferences at desktop and 390px. Test light/system selection and system color-scheme changes. Disabled ordinary gateway must remain usable without context 404 requests.

External live IdP/DNS/TLS not needed for synthetic local fixture. Live production readiness remains outside this evidence.
