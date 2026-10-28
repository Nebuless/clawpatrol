# PRX Provider operations

Provider is a separate single-process service with its own SQLite database and state directory. It is not the client registry. Hosting metadata (`customer` or `nebuless`) describes an installation, not provisioning or an extra permission. Every gateway belongs to one organization. Customer dashboard access requires customer approval in both hosting modes.

## Prerequisites

- Dedicated OS identity and owner-only state directory. Do not share the gateway database or registry state. No distributed HA or multi-writer deployment.
- Exact HTTPS Provider origin, for example `https://provider.example`. Separate support domain `support.example` with wildcard DNS and a certificate covering `*.support.example`. Production support origins use default HTTPS port 443. Preserve Host and Origin, WebSocket upgrades and streaming at ingress. Route these hosts to Provider, not to arbitrary customer HTTP services.
- Explicit TLS certificate/key, TLS 1.3, no body/token/query logging on authentication, code or activation routes. Disable third-party analytics there.
- Confidential OIDC client with exact callback `https://provider.example/__provider/oidc/callback`. Read [example configuration](../examples/support/provider-oidc.json). Replace every endpoint and assurance value with issuer-approved values. The example assurance string is not proof of MFA.

OIDC uses maintained go-oidc/v3 plus oauth2. Login binds state, nonce and S256 PKCE to the browser. Signed `azp` must match client ID. Signed `acr` must exactly match an allowed value and `auth_time` must be recent. Redemption requires authentication within five minutes. Provider sessions last at most 30 minutes after authentication, bounded by token expiry. No refresh tokens. Every authorization uses authenticated introspection and verifies active, issuer, subject, client ID and expiry. Only `client_secret_basic` introspection is supported. An IdP that cannot meet this profile is incompatible, not grounds for bypass headers.

## Bootstrap

Commands below assume operator-approved service setup and credentials already provisioned. Run local management commands as the Provider state-directory owner. Each mutation records its OS UID actor, distinct from OIDC technicians.

```sh
install -d -m 700 /var/lib/prx-provider
nebprx provider --action org-add --state-dir /var/lib/prx-provider \
  --organization-id example-org --name 'Example organization'
nebprx provider --action technician-add --state-dir /var/lib/prx-provider \
  --technician-id tech-001 --issuer https://idp.example \
  --subject IMMUTABLE_SUBJECT --name 'Named technician' --administrator
nebprx provider --action assign --state-dir /var/lib/prx-provider \
  --technician-id tech-001 --organization-id example-org
```

Roster identity binds immutable issuer/subject, never email. Administrator still needs organization assignment. Non-administrator technicians see only assigned cases. Administrators can create/enable/disable assigned organizations, assign already-visible enabled roster colleagues, register/manage installations and assign cases without a phone code, but cannot View-as without customer consent. Roster creation remains OS-owner managed. Installation management shows bounded connector health (`not_enrolled`, `connected`, `disconnected`, `disabled`, `unavailable`) and client-registry sync health, never inventory or registry tokens. Disabled organizations/installations remain visible for recovery. Failed refresh replaces prior health with Unavailable rather than retaining a stale connected label. Provider audit never invents an effective customer for management operations.

Start service:

```sh
nebprx provider --state-dir /var/lib/prx-provider \
  --listen 127.0.0.1:8093 --public-url https://provider.example \
  --support-domain support.example \
  --tls-cert /etc/prx-provider/tls.crt --tls-key /etc/prx-provider/tls.key \
  --oidc-config-file /etc/prx-provider/oidc.json
```

Ingress must connect over TLS to this listener. Configure trusted certificate chains, not insecure verification. Startup creates owner-only `support-signing.key` (base64 Ed25519 private key, 64 bytes) and `storage-encryption.key` (base64 AES key, 32 bytes). Distribute only the support public key to enrolled gateways through an authenticated operator channel. Keep encryption key with protected database backups. Losing it makes stored access tokens unusable.

## Installation enrollment

Obtain durable gateway installation ID using [gateway support status](support.md). Do not substitute a profile name or change an existing identity. Register before connector enrollment:

```sh
nebprx provider --action installation-enroll --state-dir /var/lib/prx-provider \
  --organization-id example-org --installation-id GATEWAY_ID \
  --name 'Example gateway' --hosting customer \
  --output /var/lib/prx-provider/enrollment.once
```

Output must not exist. Enrollment secret expires after five minutes and is consumed once. Transfer privately to gateway operator, run gateway enrollment, then remove transient copies under approved retention policy. A failed output write can occur after registration, so inspect current installation before repeating. Re-enrollment is not a CLI idempotency guarantee. Existing-installation key/connector rotation needs coordinated owner-controlled store tooling and verification, not direct ad hoc SQL in production.

Open Provider, authenticate with MFA, choose organization/installation, assign a named technician to the case. Customer issues code after assignment. Assigned technician enters it and posts the one-use activation form. Activation returns a same-origin continuation page so Strict cookies work after cross-site navigation. No token appears in its URL. Support origin receives only its host-only support cookie, not a customer login cookie.

## Disable, incident response and recovery

Use Provider installation toggle to deny access. Revoke a technician locally:

```sh
nebprx provider --action revoke --state-dir /var/lib/prx-provider --technician-id tech-001
```

This disables roster identity and removes its sessions, pending handoffs and routes. Gateway independently denies stale requests. Also revoke at gateway for immediate local containment, including Provider outages. Connector disconnect revokes gateway grants/sessions and drops Provider routes. Restart does not restore customer consent.

Key rotation is coordinated downtime: disable gateways, revoke access, stop Provider, take consistent backups, replace compromised purpose-specific keys, distribute new pinned public keys, re-enroll connectors where required, restart and verify a fresh customer-approved session. Never reuse advisory/registry keys. OIDC credential rotation requires matching IdP and private-file changes followed by service restart. Rotating storage encryption key without clearing/re-encrypting sessions loses them. No online multi-key rotation protocol exists.

Back up Provider database consistently with SQLite-aware tooling, including WAL state, plus encryption/signing keys in separate protected backup custody. Provider audit is append-only through application mutations but not tamper-proof against OS owner. Provider `--audit-retention-days` defaults to 90 and accepts 30..365. Startup and hourly maintenance remove expired authentication transactions, handoffs, routes, sessions/access tokens, enrollment hashes and rate-limit buckets. Metadata audit and closed cases expire after retention. Cleanup failures emit a generic error without secret values. Gateway audit retention is separate. Backup retention must not exceed approved retention window.

Monitor HTTPS availability, IdP failures, connector availability, generic redemption failures, audit write failures and disk pressure. Do not log raw codes, cookies, access tokens, authorization bodies or customer request bodies. Support unavailability must not block ordinary local dashboard or client-registry use.

## Verification and deployment status

[Fixture instructions](../examples/support/README.md) use a synthetic TLS IdP and real service processes. They are not live assurance evidence. Live MFA/disabled-user proof, wildcard ingress behavior, dedicated per-customer hosted isolation, retention/backup restore and operator-approved key rotation remain deployment gates. See [release verification](support-release-verification.md) for current test and security evidence.
