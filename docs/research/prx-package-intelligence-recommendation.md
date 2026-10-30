# PRX package intelligence: recommended architecture

Research date: **2026-10-26**. This report consolidates three Pi investigations and root review of primary documentation and PRX code. It proposes a design; no registry or enforcement adapter has been implemented or benchmarked.

## Recommendation

Build **two distinct vulnerability intelligence registries**. Nebuless hosts the global corpus; each customer hosts a durable registry containing the advisory subsets relevant to its environment. The customer's PRX gateway and a protected execution broker make decisions locally.

Use **OSV data and schema** as the foundation, preserving the underlying GitHub, PyPA, RustSec and Go records. Reuse inventory and matching components where practical. Build PRX-specific enrollment, selective signed delivery, local policy and enforcement adapters. Start with four-ecosystem intelligence, then prove one locked npm execution path and one non-streaming response adapter.

These are intelligence registries. They need not store package tarballs, wheels, crates or module archives. Existing customer artifact repositories can provide an additional control point later.

## 1. What to use upstream

| Source or component | Recommended role | Important limit |
| --- | --- | --- |
| [OSV](https://google.github.io/osv.dev/data/) | Primary aggregate, OSV interchange schema, ecosystem exports and incremental change indexes | Covers npm, PyPI, crates.io and Go; coverage is not exhaustive. Retain each source's data license |
| [GitHub Advisory Database](https://github.com/github/advisory-database) | Broad package advisory source and reconciliation input | Overlaps OSV and ecosystem databases; preserve source assertions and alias relationships |
| [PyPA advisory database](https://github.com/pypa/advisory-database) | Python distribution advisories | Distribution names differ from import names; target Python version, extras and markers affect resolution |
| [RustSec advisory-db](https://github.com/RustSec/advisory-db) | Rust advisories and ecosystem-specific metadata | Distinguish vulnerabilities from informational notices and yanked releases |
| [Go vulnerability database](https://go.dev/doc/security/vuln/database) | Canonical Go cross-check, module/package/symbol metadata | Use its public OSV API, not internal YAML. Call reachability needs source/build context |
| [OpenSSF Malicious Packages](https://github.com/ossf/malicious-packages) | Separate malicious-package intelligence | Includes some ambiguous/nuisance reports. Preserve classification and corrections; do not treat every record as proven malware |
| [OSV-SCALIBR](https://github.com/google/osv-scalibr), [OSV-Scanner](https://github.com/google/osv-scanner) | Candidate inventory/matching components and differential test tools | Pin supported interfaces; configure offline behavior and isolate any unsafe resolution plugins |
| [Dependency-Track](https://docs.dependencytrack.org/) | Optional customer SBOM portfolio, triage and reporting | Does not replace synchronous admission, signed subset delivery or response enforcement |

**Do not make CVE ID mandatory.** Useful advisories may have a GHSA, RUSTSEC, GO or MAL identifier without a CVE. Store source-qualified identifiers and relationships. A flat CVE list cannot represent package/version applicability adequately. Preserve OSV affected ranges and explicit versions, including withdrawals. [OSV schema](https://ossf.github.io/osv-schema/)

NVD, CISA KEV and EPSS are potential enrichment, not the core package matcher. EPSS publishes daily scores and recommends its bulk CSV for local synchronization. NVD access details and some enrichment redistribution terms remain unresolved in this investigation; they do not block the proposed core. [EPSS data guidance](https://www.first.org/epss/data)

The suggested alternative to building PRX-specific services is Dependency-Track plus adapters. It brings useful portfolio features, but its current OSV integration documentation labels the feature preview and describes daily/restart refresh. That makes it a poor sole foundation for the proposed low-latency admission service. [Dependency-Track OSV integration](https://docs.dependencytrack.org/datasources/osv/)

## 2. Two stores with different responsibilities

| Responsibility | Nebuless global registry | Customer registry and PRX service |
| --- | --- | --- |
| Stored intelligence | Public advisory corpus for the four ecosystems, raw revisions, provenance and source health | Relevant package-wide advisory subsets, including candidate packages |
| Environment knowledge | Public package subscriptions or broader shard selections | Exact versions, targets, dependency graphs, private packages and installation evidence |
| Authority | Publish verified distribution artifacts and source assertions | Apply customer policy, exceptions and offline rules |
| Access | Enrolled PRX installation identities only | Gateway/broker through authenticated local IPC or protected service interface |
| Suggested first storage | PostgreSQL; immutable bundles in object storage when useful | SQLite for one managed host; PostgreSQL only if shared-server concurrency warrants it |

The storage choices are engineering recommendations, not capacity results. Begin with one global application containing ingestion/publication jobs and one client daemon containing synchronization, storage and query modules. Multiple modules do not require multiple independently deployed microservices.

```text
OSV + selected upstream sources
          |
          v
Nebuless global registry -- signed scoped updates --> Customer registry
                                     ^                      |
                     outbound authenticated sync            v
                                              Local PRX policy decisions
                                                  /                \
                                      execution broker       response adapter
                                      before code runs       before delivery
```

### Select packages, retain all relevant advisory ranges

Subscribe by **ecosystem + canonical package identity + source**, across versions. If a customer has version 1.4 today, it needs both tomorrow's new advisory affecting 1.4 and enough data to evaluate a proposed 1.5. Storing only today's findings misses both cases.

Keep installed inventory separate from a bounded candidate set. For a package first mentioned by an agent:

1. Classify public/private identity locally and verify its configured source.
2. Obtain and verify the package's advisory scope, including an explicit empty scope when there are no records.
3. Resolve an exact, target-specific dependency graph using trusted metadata or isolated resolution.
4. Check direct, transitive and relevant build dependencies locally.
5. Authorize only the exact graph/artifacts and operation. Add to installed inventory after verified installation.

If offline and the required scope is absent, return **indeterminate** and withhold new admission under the proposed strict policy. Absence from inventory never means allowed. An advisory miss also does not prove a package exists; use approved registry metadata separately.

### Keep customer context local

By default, do not upload versions, full SBOMs, dependency graphs, private names, code, prompts or replies. Public-name subscriptions still disclose technology choices. Customers requiring stronger privacy can download broader ecosystem/shard bundles and filter locally, trading bandwidth for less disclosure. Hashing public names is not sufficient anonymization.

Use per-installation enrollment, mTLS, authorization, rotation and revocation. “Only accessible through PRX” should mean **authorized PRX installation credentials**, not a claim that an open-source binary is impossible to emulate. Never ship a universal service secret.

## 3. Deliver updates that remain trustworthy

Bootstrap from licensed OSV exports, consume incremental changes with overlap and deduplication, and periodically reconcile complete inventories. Record corrections, withdrawals, changed package identities and empty scopes. Keep raw revisions and source-specific conflicts rather than destructively merging everything sharing an alias.

Use [The Update Framework](https://theupdateframework.github.io/specification/latest/) for authenticated, versioned and expiring delivery. Begin with complete signed snapshots; add deltas only after snapshot correctness is demonstrated. Bind content to scope, schema/matcher compatibility, sequence, hashes and attribution. Deltas must name their exact base and resulting state. Verify outside the live store, then activate atomically. A failed update must leave the prior verified store intact.

Persist rollback protection across restarts. Include key rotation, trusted-time handling and recovery in the design. Signatures authenticate published bytes; they do not establish that upstream claims are correct or current.

Report separate health signals: last successful upstream check, source watermark, global ingestion/publication progress and client activation. OSV publishes a target of source data no more than 15 minutes stale 99.5% of the time; this is not an inherited PRX guarantee or a bound on all disclosure delays. [OSV FAQ](https://google.github.io/osv.dev/faq/)

An initial five-minute polling interval is a hypothesis to measure, not a contractual freshness promise. Measure observation-to-publication and publication-to-client latency before selecting service levels. Offline grace and source degradation must remain explicit.

## 4. Enforcement requires two separate interception points

### Commands: authorize before local execution

PRX currently gates network requests. Its inspected launch path does not authorize every descendant package-manager execution. Cached installs/builds may make no network request at all. SSH command matching is explicitly described as advisory/audit in [ssh.go](../../internal/config/plugins/facets/ssh/ssh.go), not a general local shell boundary.

Use a protected execution broker with a structured operation such as “install this approved graph.” The broker owns checking and execution; its policy and credentials must be outside the agent's writable scope. A PATH shim or optional package-check tool is insufficient when the agent can call a different executable, shell or tool.

Start with a pinned npm manager, reviewed lockfile, approved sources and `npm ci --ignore-scripts` inside a managed execution environment. Verify immutable inputs and artifact integrity before activation. Cover cache hits and reject unsupported bypass routes. Explicit `npm run`/`test` still executes the requested script despite `ignore-scripts`; it needs separate policy. [npm ci](https://docs.npmjs.com/cli/v11/commands/npm-ci/)

Bind each authorization to graph and artifact digests, manager configuration, target, policy revision and intelligence revision. Protect the inputs from mutation between check and execution. Python source metadata, Cargo build scripts and other build-time execution require their own acceptance work; a successful dry run is not necessarily execution-free.

### Replies: decide before releasing content

In [main.go](../../cmd/nebprx/main.go), `resp.Write(tc)` at line 3294 precedes usage analysis at line 3307. In [ws.go](../../cmd/nebprx/ws.go), the server-to-client pump at line 277 receives no payload rewriter. Existing observation and request credential transforms do not implement response blocking.

Add a bounded, provider-specific response gate before any content release. Start with one non-streaming JSON protocol, explicit package/version recommendations and structured install calls. Return a documented PRX policy error on denial; do not attribute fabricated refusals to the model.

Streaming requires a separate state machine for response IDs, content blocks, fragmented arguments, compression and completion. Strict prevention may require holding the full supported response before replay. A sliding text window cannot retract a command already delivered. Unsupported streaming/WS must be rejected on a strict endpoint or clearly separated into observation-only operation.

Buffering fixes delivery timing; it does not solve every ambiguity in prose. Distinguish “install this” from “do not install this” and quoted vulnerability examples. Do not blacklist an entire package name because one old version is affected. Provider-hosted tools that execute upstream are outside this local response gate.

## 5. Policy must preserve uncertainty

Keep independent fields for findings, package existence, graph completeness, intelligence freshness and policy decision. Avoid a `safe` boolean.

| Situation | Proposed strict-path behavior |
| --- | --- |
| Known affected version in resolved graph | Deny with advisory, dependency path and evidence revision; allow only a valid scoped exception |
| No known match, complete supported graph and fresh scope | Policy may allow; explanation retains the no-known-match limitation |
| Unknown name, unresolved graph or unsupported range semantics | Review/indeterminate; no automatic clearance |
| Stale intelligence or new package while disconnected | Withhold new admission; existing exact operations require an explicit offline continuity rule |
| Corrupt update, wrong scope or rollback | Reject update, retain verified history, expose degraded status |
| Corrected/withdrawn advisory | Recompute findings; preserve audit history and independent active source assertions |

Use native ecosystem semantics. Python is not SemVer; Go module paths, replacements and pseudo-versions matter; Cargo features/targets and npm aliases/transitives matter. Missing severity must not silently classify an advisory as low risk. Do not automatically substitute `latest`: a replacement needs a new graph check and compatibility remains separate.

## 6. Reuse rights and provenance

Code licenses and data licenses are separate. GitHub and PyPA advisory data are CC-BY-4.0. Go's repository distinguishes BSD-style source from CC-BY-4.0 data. RustSec defaults to CC0 with CC-BY-4.0 exceptions for imported GHSA records. OpenSSF's repository uses Apache-2.0; external linked material still needs source-aware treatment. The detailed reports contain the inspected license links.

Use a source/license allowlist, preserve attribution and transformation notices, and include a machine-readable attribution manifest with client bundles. Authenticated service access does not make upstream advisories proprietary. Exact redistribution packaging and transitive dependency notices remain a release gate, not a reason to assume the entire OSV aggregate has one license.

## 7. Prototype sequence and acceptance gates

| Stage | Deliverable | Evidence required to advance |
| --- | --- | --- |
| 1. Intelligence correctness | Four-ecosystem normalized store and local matcher | Boundary versions, aliases, withdrawals, empty scopes, source conflicts and native comparator tests against pinned reference tools |
| 2. Two-registry synchronization | Enrolled client, signed full snapshots, selective package scope, local decisions | New disclosure with unchanged inventory; first-seen package; wrong-tenant denial; replay/expiry/corrupt-update rejection; atomic recovery |
| 3. Controlled command prevention | One locked npm broker integration | Cache-hit denial, graph/artifact binding, mutation resistance and blocked raw execution bypasses in the declared environment |
| 4. Controlled reply prevention | One bounded non-streaming adapter | Zero denied content delivered; valid client error handling; malformed/oversized response fails closed; negation and quoted examples handled |
| 5. Expand measured coverage | Python, Cargo, Go execution; SSE/WS and additional agents | Separate protocol/manager acceptance suites, measured latency and documented unsupported cases |

Do not publish prevention claims after stage 1 or 2 alone. A defensible eventual claim is: **PRX blocks policy-disallowed known package/version findings on supported, enforced execution and response paths.**

Before implementation, specify the first managed client/agent versions, acceptable buffering latency, offline continuity policy and public-name disclosure preference. These are product choices; the research provides recommended defaults and the tests needed to evaluate them. Hardware sizing, coverage percentages and runtime compatibility remain unmeasured.

## Research record

- [Feeds, licensing and reusable tools](prx-package-intelligence-feeds.md)
- [Global/client architecture, API and synchronization design](prx-package-intelligence-architecture.md)
- [PRX code inspection, agent integrations and 27 adversarial acceptance cases](prx-package-intelligence-enforcement.md)

The root review confirmed critical HTTP/WS code paths and checked OSV exports/freshness, Dependency-Track's documented refresh behavior and EPSS bulk guidance. NVD documentation did not produce usable content and the root CISA page fetch was denied; those limitations remain explicit. Acceptance scenarios are proposed tests, not executed security or performance results.

The accompanying [visual brief](../../.lavish/prx-package-intelligence.html) uses PRX's existing Clear Glass palette and self-hosted fonts. Chromium checks on zo-computer passed at 1440px and 390px in light/dark modes: all four illustrative scenario selections, disclosure expansion, font loading and page overflow checks passed with zero page exceptions. Desktop/light and mobile/dark screenshots were visually inspected. This validates the report surface, not the proposed security services.
