# Hosting/support plan review

Verdict: **ITERATE**. Architecture is viable, but identity integration, activation recovery, mutation auditing and hosted-management scope need corrections before affected units proceed. No live deployment credentials are needed to make those corrections or run later synthetic acceptance.

Document review complete (non-interactive mode). Applied 0 plan fixes. This reviewer owns only this report. Proposed corrections return to the caller under the existing approval, not a new confirmation menu.

## Reviewed baseline and coverage

- Plan: `2026-10-27-1621-feat-hosting-support-impersonation-plan.html` in this directory, all sections, including U1–U9, R1–R13, V1–V6 and AE1–AE12. Classification: unified-plan. Provenance: product_contract_source: ce-brainstorm, supplemented by the caller's explicit product decisions.
- Checkout: branch `rebrand/prx-glass`, HEAD `060ff6d`. Cached origin/main is `2604e15`. Fourteen commits separate them. No fetch performed.
- Plan SHA-256: `f8c51ef5aee83c52ca24ea4c5263df1acd6fdb5130291fa286d7d34aa7757d05`.
- Read-only source inspection covered authentication, CSRF, sessions, migrations, device mutations, action rule generation, SSE, dashboard lifecycle, registry configuration/status, dependency sources and CI/release declarations.
- All seven lenses applied inline: coherence and feasibility throughout, security for authorization/transport/audit, product for hosted-management scope, design for approval/termination flows, scope for replacement work, adversarial for failure/concurrency paths. One reviewer, not seven independent reviewers. No subagents or cross-model calls, as explicitly prohibited. Compound-pack resolver returned no packs, warnings or errors.
- No tests, builds, installs, browser sessions, remote commands, commits or pushes. No code edits, so no code/LSP cleanliness claim. Runtime acceptance remains unverified.

## Proposed fixes, severity ordered

### F1 — P1 blocker: use an OIDC verifier and finish the issuer integration contract

**Recommendation:** Correct before technician authentication work.

**Consequence:** The plan assigns PRX a custom OIDC verifier while leaving required issuer behavior ambiguous. A valid deployment can reject every login, or a permissive implementation can accept insufficiently bound identity or MFA evidence.

**Plan location:** KTD3 (OIDC authentication), CLI/configuration “OIDC file”, U4 (verified technician authentication), V6 (external identity gate). KTD3 says to reuse oauth2 and jose, then implement ID-token validation and JWKS refresh. It requires “matching subject/client/issuer policy” during introspection without an explicit response contract. The configuration has an introspection secret but no separate client ID or authentication-method contract.

**Source evidence:** `go.mod:40,73,85` confirms oauth2, websocket and jose, but no OIDC relying-party library. OAuth2 is a flow/token client, and jose's `jwt/claims.go:27-36` defines generic JWT claims, not OIDC nonce, authorized-party or authentication-assurance validation. A concrete smaller alternative is already available in the local module cache: coreos/go-oidc v3.17.0, recorded in `go.sum:106-107`. Its `oidc/verify.go:54-71` provides NewVerifier with a configured issuer and key set, without mandatory discovery. Being cached does not mean it is already a direct project dependency.

**Minimal remedy:** Select that maintained verifier, or another reviewed OIDC verifier, as the narrow direct dependency instead of implementing OIDC over jose. Keep explicit application checks for nonce, azp where required, nonempty subject, issuer-specific acr and bounded auth_time. Specify the authorization request's openid scope, max_age/step-up request and browser-bound one-use login transaction. Define supported introspection authentication and required claims, including active, subject equality with the ID token, expected client, expiry and issuer provenance. RFC 7662 does not guarantee every optional claim, nor immediate disabled-user invalidation. Unsupported profiles must fail closed with an actionable configuration error. Missing claims must not silently fall back to email or client assertions. Synthetic IdP fixtures must exercise omitted claims, swapped subjects, stale MFA and disabled-user behavior. This is an integration contract correction, not a demand for live credentials now.

### F2 — P1 blocker: preserve hosted organization management outside impersonation

**Recommendation:** Clarify and cover the already-requested management surface.

**Consequence:** The implementation can pass its current acceptance table while offering only a status list and case assignment, rather than usable hosted organization/instance management.

**Plan location:** R2 (Provider management), HTTP contract “Provider browser”, UI direction, U3/U8, AE2. R2 limits management to hosting/connection/registry health. AE2 says “Provider org selection alone grants no customer view.” Browser endpoints expose installation/case lists, case assignment and redemption. Organization creation and enrollment are local CLI actions.

**Source evidence:** `cmd/nebprx/main.go:3525-3528` dispatches gateway and WIP registry, not an existing Provider application. `dashboard/src/App.tsx:18-54` has no Provider route. There is no existing hosted-management UI that fills the missing outcome automatically. This finding is against the caller's explicit requirement that authorized hosted organization management work without customer impersonation, not against the settled phone-approval requirement.

**Minimal remedy:** Define two separate authenticated actions: “Manage organization/instance” opens the authorized hosted instance's management detail and approved management controls, while “View as customer” opens no account data until phone approval. Put organization/installation registration and assignment management in that Provider surface using the existing planned records and role checks. Specify its exact non-customer fields/actions and audit actor/target organization, without inventing an effective customer. Rewrite AE2 to prove both paths: management works without a code, account dashboard/data does not. Keep cloud provisioning, shell and arbitrary proxying excluded.

### F3 — P1 blocker: required mutation parity needs explicit lower-level audit work

**Recommendation:** Expand the affected-file scope, not the permission exclusions.

**Consequence:** A wrapper around current handlers cannot guarantee atomic database effects and audit outcomes. It can report success after persistence or device-revocation errors, or leave required mutations permanently denied.

**Plan location:** KTD8 (permitted device mutations), KTD9 (atomic security audit), U6 (support-aware dashboard authorization), U8 mutation acceptance. The plan promises that allowed SQLite mutations commit effects and outcome audit together, and that device deletion/profile changes remain usable. U6's listed edits focus on route/response projection, not the lower-level mutation stores.

**Source evidence:** `cmd/nebprx/agents.go:1096-1155` calls void mutation methods and returns success. `cmd/nebprx/onboard.go:221-235,330-369` changes in-memory profile mappings before an unchecked database write. Its device deletion at `691-714` discards database errors. `cmd/nebprx/wireguard.go:673-696` discards tunnel and deletion failures. These operations cannot join an audit transaction through the current interfaces.

**Minimal remedy:** Name these concrete files/functions in the mutation unit. Use error-returning, transaction-aware database operations for device/profile effects and the corresponding audit row, then update memory after commit. Treat WireGuard changes as explicit non-transactional operations with durable intent and truthful success/failure/unknown outcomes. Do not claim rollback of an external effect. Add failure-injection acceptance for both profile assignment and device deletion, including database commit failure and tunnel failure. Keep the same shared validators for ordinary and support access. A generic transaction framework is unnecessary.

### F4 — P1 blocker: failed activation can strand the only allowed session

**Recommendation:** Define the pending-to-active transition and cleanup.

**Consequence:** A burned code can leave an unusable session occupying the account's single-session slot. The mandated customer reissue then cannot restore access until expiry or a separate manual revocation.

**Plan location:** KTD4 (atomic redemption), KTD6 (one active session per effective account), KTD7 (30-second handoff), AE4 (lost reply recovery). Redemption inserts a session before the Provider returns the activation form. Lost success requires reissue, but reissue revokes only the previous active code. The plan does not say how an unactivated session releases the account slot.

**Source evidence:** The existing session primitive in `cmd/nebprx/dashboard_sessions.go:29-58` creates an immediately usable database session and returns its credential. It supplies no two-process pending/activation lifecycle to reuse. The Provider and supportwire packages do not yet exist.

**Minimal remedy:** Add a short-lived pending session reservation tied to the one-use handoff. Activation must recheck current local account/grant/case and Provider actor authorization before atomically making that reservation active. Failed/lost/expired handoffs and reissue must retire orphaned pending reservations without reviving the consumed grant. No data access in pending state, no distributed transaction requirement and no retry that returns an old bearer credential. Test crashes/lost responses after gateway consume, before Provider handoff persistence, and after handoff consume but before Set-Cookie reaches the browser. Prove reissue works immediately after cleanup and never yields two active sessions.

### F5 — P1 blocker: tailnet subjects need a current identity source, not a saved login name

**Recommendation:** Complete the stable-account and deletion lifecycle before using saved accounts as support subjects.

**Consequence:** Support can authorize a stale or wrong account after authority changes, user removal or login-name reassignment. Password reset/rebootstrap can also accidentally reactivate historical grants if account lifecycle checks follow only the listed password-upsert hook.

**Plan location:** KTD2 (stable accounts), U2, U5, failure behavior and AE6. KTD2 promises binding to UserProfile.ID plus tailnet authority, but declares uniqueness only on (auth_kind, auth_subject). It refreshes records on ordinary login while requiring current-policy eligibility on every support request. The implementation file list names setDashboardUser, not the separate deletion path.

**Source evidence:** `cmd/nebprx/agents.go:261-293` discards the upstream numeric user ID and exposes only LoginName. `third_party/tailscale/tailcfg/tailcfg.go:280-281` confirms the upstream ID exists. `cmd/nebprx/web.go:596-665` authorizes a fresh peer lookup by login-name allowlist. There is no API in the plan for refreshing an arbitrary saved subject when that customer is offline. `cmd/nebprx/dashboard_users.go:80-90` deletes a root user and revokes only ordinary sessions. The ordinary gate also denies all dashboard accounts while root is absent at `web.go:359-377`.

**Minimal remedy:** Retain upstream user ID in the verified identity adapter. Include authority in uniqueness, or explicitly forbid authority changes without retiring old bindings. Specify a server-trusted current subject/membership lookup or update feed for offline impersonation eligibility, with fail-closed behavior when unavailable. Do not substitute last-known LoginName. Wire password deletion/reset and authority changes into support invalidation, preserve historical audit identities, and ensure re-created accounts do not revive grants. Cover root absence for tailnet targets too. If the available tailnet API cannot prove current eligibility, identify that limit before promising immediate removal semantics.

### F6 — P2 blocker for migration work: existing migration runner is not transactional

**Recommendation:** Make the promised rollback mechanism explicit.

**Consequence:** Reusing the current migration pattern unchanged can leave a partially created support schema or account backfill after failure, despite the required rollback test.

**Plan location:** “Data migrations and ownership”, U2/U5. The plan says to use the existing OpenDB/migrate pattern and verify rollback on failure, but neither migration unit lists the runner as an affected file.

**Source evidence:** `cmd/nebprx/db.go:82-139` executes each SQL file outside a transaction and records the schema version in another statement. Existing `cmd/nebprx/migrations/sqlite/0024_device_node_id.sql` contains an unwrapped ALTER TABLE. SQLite migration paths are under cmd/nebprx, not repository-root migrations/.

**Minimal remedy:** State that each new migration's DDL and version marker commit together, either through a narrowly updated runner or self-contained transactional migration files. Define the account-ID backfill's transaction/restart boundary. Add db.go to the unit if changing the runner. Fault-inject midway through both new migrations and verify successful retry. This is a local correction, not a redesign of historical migrations.

### F7 — P2 improvement: correct the claim that ordinary config writes are unavailable

**Recommendation:** Keep the security exclusion, but give its real reason.

**Consequence:** Implementers can mistakenly disable an existing customer capability or certify permission parity only against the default configuration.

**Plan location:** KTD8 (capability ceiling), sentences “legacy config apply (already unavailable to ordinary customers)” and “Config remains read-only for ordinary customers too.”

**Source evidence:** `internal/config/config.go:179-188,483-486` defines the optional dashboard_config_writes setting. `cmd/nebprx/web.go:1497-1584` validates and applies HCL when enabled. Disabled by default is not unavailable.

**Minimal remedy:** State that ordinary users retain configured write behavior. Deny arbitrary HCL apply in support because it can cross secret, routing and privilege exclusions, not because the route does not exist. Show “Unavailable during support” for that exclusion and test both enabled and disabled ordinary configurations. No new config editor or blanket read-only support role is needed.

### F8 — P2 blocker for limiter acceptance: bad guesses cannot identify the promised grant counter

**Recommendation:** Bind failed-attempt accounting to a resolvable non-secret challenge scope.

**Consequence:** The “five failures per grant” test cannot be implemented consistently when one case has grants for multiple target accounts. Arbitrarily picking a grant either misses the limit or lets failed guesses revoke another grant.

**Plan location:** KTD4 (persistent attempt counters), HTTP contract redemption body, support migration uniqueness and U5/AE4. Redemption receives case_id, installation_id and code. Grant uniqueness is per case/subject, so multiple subjects may have different codes for one case. A wrong code identifies none of them, and the plan also says wrong bindings cannot target another grant's counter.

**Source evidence:** These are new routes and tables, not behavior inherited from `cmd/nebprx/web.go:233-291` or the existing ordinary session store. The conflict is between the plan's concrete payload, uniqueness rule and limiter guarantee.

**Minimal remedy:** Use a persistent failed-attempt bucket keyed by verified technician, case assignment revision and installation, with the five-attempt lockout invalidating only that authorized challenge scope. Retain the existing broader technician/installation and installation limits. Alternatively, an explicit challenge identifier requires a revised browser contract, but is not necessary for this release. Test two target accounts on one case, wrong bindings, concurrency and restart. Never use the raw code as a limiter key or log field.

## Startability and reference verification

| Area | Verified state | Startability |
| --- | --- | --- |
| U1, integration baseline | Registry command/store/sync files and main.go hunks exist as WIP. Fourteen commits beyond cached origin/main include unrelated naming/site/platform work. | Read-only inventory is startable. Publishing still requires the caller's explicit prerequisite/base choice. The plan already states this correctly. |
| U2, gateway identity | Principal, root sessions, CSRF, whoami and profile selectors exist. Latest gateway migration is 0024. | Start after F5/F6 corrections. New account helpers and 0025 are intentional new work. |
| U3–U4, Provider and identity | No internal/provider package or Provider command exists. Planned store/HTTP APIs are new, not fictional existing APIs. oauth2/jose dependencies exist. | Registration is startable after F2 scope correction. Authentication needs F1. External credentials do not block synthetic implementation. |
| U5–U7, consent and transport | No supportwire package, consent schema, support handler or connector exists. Existing websocket dependency is present. | Start after F3–F6/F8 corrections in dependency order. Keeping broker transport in a later unit is feasible using internal service calls/tests first. |
| U8, UI | Referenced React pages, Button/Main/PageTitle, format helper and shared assets exist. App swallows refresh errors and api.ts caches state across calls, as the plan reports. | Start after backend contracts. Required session-ended clearing and responsive/keyboard behavior are adequately specified. No visual/browser validation performed. |
| U9, verification/shipping | CI files, dashboard package scripts and Linux/macOS amd64/arm64 CGO-disabled release matrix exist. Both dashboard lockfiles exist. | Test/build commands are plausible declarations, not executed evidence. Remote wrapper availability was not exercised. No reason to block local synthetic acceptance solely on V6 credentials. |

## Critical-target disposition

| Target | Review result |
| --- | --- |
| Issuer/subject and MFA | Correct intended trust boundary. F1 closes library/profile contract and browser binding. No email enrollment or asserted MFA headers should be added. |
| Disablement/reassignment | Local roster/assignment checks per request and case revision invalidation are required already. Test simultaneous assignment changes against pending redemption and activation, not only later requests. F1 covers real IdP disablement semantics. |
| Customer identity/permission parity | Root and tailnet are currently instance-wide operators, not existing fine-grained customer RBAC. Preserve that ceiling. F3/F5/F7 correct the actual source integration. |
| Atomic consent/activation | Single gateway consume transaction and unique grant/session constraint are sound. F4 closes the distributed handoff failure window without two-phase commit. |
| Audit and streaming | Separate synchronous security audit is warranted. Existing Sink.Emit has no commit acknowledgment. F3 is necessary for mutations. Per-delivery authorization must occur before both gateway emission and Provider browser release, including queued SSE chunks. A support-only streaming writer must enforce bounds and backpressure, not buffer an infinite SSE response. |
| Replay and resource limits | Connection nonce, sequence, request/body binding, frame limits and per-installation concurrency are specified. F8 corrects failure accounting. Keep bounded stream queues and admission before buffering/introspection work. |
| Connector trust | Enrolled connection lookup, pinned Provider key, gateway proof and in-process dispatcher avoid arbitrary upstream proxying. No inbound management port or root credential copy is required. |
| Origins/cookies/CSRF | Per-session host-only cookies and body-only handoff are viable. Activation cannot expect the host-only Provider cookie on the sibling support host. Resolve its linked Provider session from the one-use handoff and recheck it server-side. This interpretation needs no shared Domain cookie. |
| Projection/secret leakage | Typed projections are required, not cosmetic UI hiding. In particular, rule preview currently falls back to raw SQL statement/request body in cmd/nebprx/rulegen.go:146-173. A support-safe preview must exclude that fallback and explain unavailable exact previews. Test generated HCL and error strings as well as ordinary response fields. |
| Disconnect/restart/revocation | Terminal disconnect/restart behavior, no automatic replay and local revoke during Provider outage are explicitly required. Retained audit must survive terminal-session cleanup. F4/F5 cover remaining account/activation lifecycle risks. |
| Delivery complexity | Separate control plane, local consent authority and outbound constrained transport are justified by the accepted boundaries. Reuse a real OIDC verifier. Do not add cloud provisioning, a generic RBAC system, distributed transactions or a second dashboard. |

## Handoff

Caller owns plan corrections and implementation. Fix blockers before affected units, retain F7 as a small factual correction, then independently verify code and process/browser acceptance. No production files or existing WIP were changed by this review.

Review complete
