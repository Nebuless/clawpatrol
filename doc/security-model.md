# Support security boundary

This page covers the new Provider/support surface, not a full gateway security audit.

- Provider management, customer dashboard, isolated support-session hosts and client registry are separate trust domains. Registry credentials/advisory keys do not grant Provider authority. Each gateway belongs to one organization. Hosting metadata does not provision infrastructure or increase permissions.
- Technician identity comes from maintained OIDC verification plus authenticated fail-closed introspection, exact signed assurance mapping and immutable issuer/subject roster. No email enrollment or technician-controlled MFA headers.
- Customer approval is a five-minute, single-use, installation/grant-bound 60-bit SUPPORT code. It authorizes only named assigned technician/case/account. Activation reserves at most 30 seconds, then grants at most 15 minutes. No universal credential, refresh or customer-cookie impersonation.
- Connector is enrolled outbound WSS with pinned Ed25519 keys, fresh challenge, signed org/installation/session/case/origin/body bindings and monotonically sequenced connection-specific frames. It is not CONNECT, arbitrary TCP or generic reverse proxy. Bounded frames, deadlines and concurrency limit resource use.
- Gateway support dispatcher defaults to deny, using typed secret-safe response projections. Safe reads and narrowly audited device mutations/rule preview are allowed. Raw configuration, credentials, OAuth, approval and unknown routes are denied. Local account generation, Tailnet proof, expiry, assignment and revocation are rechecked before delivery.
- Audit failure denies support results. DB mutation and outcome audit share transaction, memory follows commit. External WireGuard effects use durable intents and truthful outcomes. OS owner can modify SQLite and keys, so application audit is not tamper-proof against host compromise.
- Browser support uses HTTPS isolated origins, host-only Secure/HttpOnly cookies, strict Origin checks and one-use POST activation. Referrer policy is strict-origin to preserve browser POST Origin without leaking paths/tokens. Client clears support cache and EventSource on termination.

Deployment requires actual IdP MFA/disabled-user evidence, wildcard DNS/TLS ingress validation, isolated hosted OS/state credentials, approved backup/retention and key-rotation procedures. Read [Provider](provider.md), [gateway support](support.md) and [test evidence/blockers](support-implementation.md). Synthetic tests do not clear dependency vulnerabilities or establish live production readiness.
