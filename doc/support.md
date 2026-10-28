# Customer-approved PRX support

Support lets a named, assigned Provider technician use the existing gateway dashboard as a specific customer account while preserving both identities. It does not grant shell access, expose arbitrary customer network services or reuse the customer's login cookie. Existing password/root and Tailnet permission ceilings remain. No customer RBAC is added.

## Customer flow

1. Sign into the normal gateway dashboard. Open Account, then Customer-approved support. Submit a short case summary without credentials, tokens, request bodies or codes.
2. Provider assigns a named technician. Verify that name and case through your established support channel. A code is not a password, login MFA challenge or evidence of your identity.
3. Choose Issue one-time SUPPORT code for your account. Read it only to the verified assigned technician over the phone. Never paste it into a case, chat, screenshot or log. Hide it after use.
4. Technician enters code in Provider after recent MFA, then opens approved dashboard. Support banner identifies technician, effective customer, case and expiry. Read-only safe settings are available. Supported device profile changes/deletions and rule preview keep normal validation and synchronous audit.
5. Click Revoke access in your own dashboard to end access. Technician can use End support session. A replacement code revokes prior pending/active access for that account. A lost activation response requires a new code, not replay of old one.

Codes use `SUPPORT-XXXX-XXXX-XXXX` with 12 uniformly sampled Crockford Base32 characters (60 bits), expire after five minutes and are consumed once. Only installation/grant-bound HMAC-SHA-256 is stored. Pending activation lasts at most 30 seconds. Active support lasts at most 15 minutes, without silent renewal. Failed guesses use durable technician/case-revision/installation limits. Five failures lock that challenge and invalidate its outstanding grants, with broader technician/installation limits too.

## Gateway setup

Support is opt-in. Omitting `--support-config-file` starts no support connector. Each gateway has exactly one organization and durable installation ID in its existing `nebprx.db`. Provider is [operated separately](provider.md).

Copy [gateway-support.json](../examples/support/gateway-support.json) into an owner-only file. Keep `enabled: false` until enrollment is complete. Replace organization, hosting and paths. JSON loader rejects unknown fields. Key files are base64: installation Ed25519 private key is 64 bytes, pinned Provider public key is 32 bytes, code pepper is 32 random bytes. Generate keys with an approved cryptographic tool and transfer only the public part where appropriate. Do not use example bytes or registry/advisory signing keys.

`broker_url` and `provider_origin` must be the same exact HTTPS origin. `support_domain` must match Provider's isolated wildcard support domain. Gateway validates HTTPS and a single session hostname beneath that configured domain before reservation, then pins the exact origin to the session. Provider chooses session hostname, not browser input. Ports on support origins are rejected.

Read local installation identity:

```sh
nebprx support --action status --state-dir /var/lib/nebprx \
  --config-file /etc/nebprx/support.json
```

The status command requires an initialized gateway database and valid private config. Its `enabled_on_next_start` field reports configuration, not current connector health. Do not rewrite installation ID in SQLite to match a registration. Register that actual ID at Provider and obtain one-time enrollment secret. Enrollment verifies local ID and organization before consuming it:

```sh
nebprx support --action enroll --state-dir /var/lib/nebprx \
  --config-file /etc/nebprx/support.json \
  --enrollment-secret-file /var/lib/nebprx/enrollment.once
```

Without secret-file flag, command reads stdin. It writes owner-only connector credential to configured path. Remove temporary enrollment copies under approved policy. Set config `enabled: true` and start gateway with its existing HCL config:

```sh
nebprx gateway --support-config-file /etc/nebprx/support.json /etc/nebprx/gateway.hcl
```

HCL still controls gateway `state_dir` and ordinary `dashboard_config_writes`. Support config is separate private JSON. A successful dashboard connection indicator, Provider installation listing and fresh approved support session are runtime evidence. Configuration alone is not proof of enrollment or reachability.

## Authorization and restricted surface

Every request and bounded event delivery checks local enabled state, account generation, current account/root existence, case assignment/revision, technician authority, session expiry and exact origin. Tailnet targets require stable upstream user ID and control-server authority plus current server-trusted membership proof. Unavailable proof, renamed/reassigned login or authority changes fail closed. Password rotation, account deletion/recreation, reissue and connector loss invalidate support.

Default-denied support routing permits classified safe dashboard reads, typed `GET /api/support/config-view`, summary-only action/event projections, authorized `POST /api/agents/profile` and `POST /api/agents/delete`, and `POST /api/actions/rule-preview`. Unknown/future routes, raw `/api/config`, credential/OAuth/onboarding/approval APIs, HITL decisions and arbitrary HCL writes are denied. Support restrictions do not disable ordinary dashboard configuration edits when `dashboard_config_writes` is enabled.

Provider uses host-only Secure/HttpOnly cookies on isolated HTTPS origins and one-use POST activation. Customer cookie never authenticates support. No credential/config/raw action payload is intentionally projected. Account context saves selected profile and theme without changing permissions.

## Revoke, disable and rollback

Customer revoke works locally even when Provider is unreachable. For operator containment:

```sh
nebprx support --action revoke-all --state-dir /var/lib/nebprx \
  --config-file /etc/nebprx/support.json
nebprx support --action disable --state-dir /var/lib/nebprx \
  --config-file /etc/nebprx/support.json
```

`revoke-all` revokes stored grants/sessions. `disable` also clears the durable SQLite admission flag. Neither command rewrites deployed configuration. Startup requires both `enabled: true` in config and the retained admission flag; an old enabled config cannot undo an emergency disable, including on a read-only config mount. `status` reports both controls and their combined next-start result.

To re-enable, repair the incident first, keep `enabled: true` in the private config, then run `nebprx support --action enable` with the same state/config arguments. Restart the gateway if support did not start or audit failure blocked the connector. Explicit enable is audited and never revives revoked consent; obtain a new customer approval.

Rollback removes support startup flag or keeps config disabled, revokes sessions and stops Provider if appropriate. Preserve database migrations and audit records. Do not downgrade a gateway binary onto a database schema it cannot read without a tested restore. Ordinary local dashboard/client registry remain separate.

Device mutations commit database changes and outcome audit together, then update memory. WireGuard deletion persists external-revocation intent in same commit. Failed peer revocation is audited and retried before connector restart. A failure after DB commit may mean deletion committed but external completion is pending. Inspect durable intent/audit instead of blindly repeating a destructive action.

## Audit, retention and failure handling

Bulk revoke/disable, connector disconnect and gateway restart record termination reasons for affected sessions and outstanding grants in the revocation transaction. Local CLI attribution uses the operating-system UID and username; automatic cleanup identifies the PRX gateway system, not a technician. If audit storage fails, termination reports an error, blocks process admission and attempts durable revocation plus disable without audit. This exceptional fallback may leave an evidence gap; repair storage, preserve the error and explicitly re-enable/restart before new approvals. A database that cannot accept any writes cannot promise durable state changes.

Gateway audit includes actor/effective account, organization, installation, case, session, grant ID, operation and outcome, not raw code or customer bodies. Local audit reads are account-scoped with `cursor` and `limit` (1..100) on `GET /api/support/audit`. Cursor is the last returned audit-row ID, scoped to the authenticated account. Account page loads 50 metadata rows at a time with Load more audit. Export local audit JSON follows 100-row pages and downloads `prx-support-audit.json` with schema version 1, stable event IDs, actor/effective names and IDs, available organization/installation/case/session IDs, timestamps, actual HTTP method when available, operation, decision, outcome and reason. No CSV export. Download uses same-origin requests and a temporary object URL that is revoked after use. Provider management audit records actual Provider or local OS actor, never fabricated customer identity.

Gateway `audit_retention_days` defaults to 90, allowed range 30..365. Hourly maintenance revokes expired sessions/grants, removes terminal code hashes and expired rate-limit rows after 24 hours, then removes terminal grant/session metadata, closed cases and audit rows after retention. Sessions needed by pending WireGuard revocation intents remain until recovery completes. Maintenance failures emit generic errors but have no dedicated metric, so monitor disk pressure and verify cleanup as an operator gate. Provider retention is described separately in its runbook. Application audit is not protection against an OS/database owner.

If audit commit, IdP introspection, case authority or connector verification fails, support fails closed. Local dashboard and registry do not depend on Provider. During incidents disable support locally first, preserve private evidence, repair dependency with approved procedure, then issue a new customer code. Never relax TLS, MFA, audit, origin checks or plugin sandboxing to recover support.

[Repeatable fixture](../examples/support/README.md) and [verification record](support-implementation.md) distinguish synthetic testing from production readiness.
