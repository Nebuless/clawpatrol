# PRX package intelligence: global and client-side registries

Research date: **2026-10-26**, local date on CAX. Status: **research and proposed architecture, not an implementation or approved design**.

Scope: npm, Python packages distributed through PyPI, Rust crates from crates.io, and Go modules, with a client-hosted PRX gateway. This document distinguishes a vulnerability intelligence registry from a package artifact registry. Existing website changes are unrelated. Only this document was authored for this investigation.

Evidence labels: **Verified** means a current primary source was read. **Proposed** means a PRX recommendation, not an upstream feature or service guarantee. **Unverified** identifies a question requiring implementation experiments, legal review, or product approval. Sources are linked directly in the source register. Live documentation is mutable, not a historical archive.

## 1. Findings and recommendation

- **F1 — Keep two distinct registries.** Nebuless operates a global advisory aggregation store. Each client operates a separate, durable inventory-specific intelligence store beside its own PRX gateway. The client store contains package-wide advisory histories, local dependency graphs, findings, policy, and exceptions. It is not merely a cache of cloud verdicts or a tenant view in Nebuless's database.
- **F2 — Reuse OSV data and extraction tools, build the PRX delivery and admission layer.** OSV covers the four requested ecosystem identifiers and exports records and change indexes. It does not supply PRX tenant enrollment, private inventory isolation, signed client subsets, or command/reply enforcement. Use an OSV mirror plus a small authenticated PRX API rather than deploying the entire upstream OSV service initially. [S01–S04]
- **F3 — Subscribe by package identity across all versions.** Version-specific findings alone cannot check an upgrade, a newly proposed version, or a newly disclosed advisory without another query. Retain the full relevant range records for each subscribed package and recompute local findings when intelligence changes. An advisory feed is not a catalogue of all existing packages. No match never means safe.
- **F4 — Dependency-Track is useful portfolio software, not the complete enforcement architecture.** It already consumes CycloneDX, tracks usage, supports triage/VEX, and reanalyzes components. Its documented OSV integration is marked preview and refreshed daily or on restart. A lower-latency signed subscription channel still needs separate work. Treat version-specific behavior as something to test, not an assumed property of every release. [S05–S10]
- **F5 — Keep execution and content decisions local.** Only a client sync service authenticates to Nebuless. Agents and applications call the local gateway. New packages require an admission path before being installed or recommended, including transitive graph checking. A network gateway alone cannot observe every shell command or prevent execution from a local cache.
- **F6 — Use authenticated, signed, expiring distribution with explicit uncertainty.** TUF is suitable for delivering advisory snapshots and deltas as authenticated target files. It does not make advisories correct, hide subscription identities, supply tenant authorization, or decide offline policy. Code reproducibility does not grant service access, but neither can OSS code prove an authorized caller is an unmodified PRX binary. [S19–S21, S25]

## 2. Boundaries and textual flow

### 2.1 The two registries

| Responsibility | Nebuless global intelligence registry | Client inventory-specific registry |
| --- | --- | --- |
| Ownership and location | Nebuless-hosted service and storage | Separate client-controlled process/storage in the client's environment |
| Scope | Licensed public advisories for the four ecosystems, including corrections and withdrawals | Complete advisory subsets for locally relevant public package identities, private advisories, inventory, candidate packages, graph edges, decisions |
| Knows exact installed versions and graph? | Not required. Do not upload by default | Yes, with environment and evidence provenance |
| Authority | Publishes attributed intelligence and source health | Client policy determines allow, deny, review, and offline exceptions |
| Consumers | Authenticated PRX installation sync identities only | Client-hosted PRX gateway through local IPC, optional authorized local administration |
| Package artifacts | None required | None required. Existing client artifact repositories remain separate |
| Failure isolation | Ingest or cloud outage does not erase the client's last verified state | Local store failure must not become an empty successful answer |

An inventory-specific registry still needs package-level data for versions not yet installed. Keep **active inventory** separate from a bounded **candidate/admission set**. A newly suggested public package can temporarily enter the latter, receive an all-version advisory snapshot, and become active only after a verified install/inventory event. Do not replicate every package artifact or all global advisories into every client by default.

### 2.2 Proposed flow

```text
Public advisory sources / OSV exports
  Nebuless initiates outbound ingest over authenticated/validated HTTPS as applicable
  -> raw immutable evidence + license/provenance ledger
  -> validated, indexed global advisory revisions + durable change log
  -> signed package snapshots / subset deltas / source-health manifests

Client inventory collector reads approved lockfiles, SBOMs, installed metadata
  -> local identities, exact versions, dependency graph, completeness evidence
Client sync service initiates outbound mTLS HTTPS to Nebuless
  -> requests public package identities or privacy-preserving broad feed shards
  -> verifies trusted metadata, scope, sequence, hashes, expiry and base state
  -> atomically updates its separate inventory-specific registry
  -> recomputes local version findings and invalidates old decisions

Agent command / model reply -> client PRX gateway
  -> local parser / admission checks -> local registry over authenticated IPC
  -> client policy -> allow / deny / review / indeterminate
  -> local audit containing evidence revisions, not uploaded prompts

Admitted package download -> client's normal artifact source or existing proxy
  -> enforce exact resolved artifact/version/graph before execution
  -> reconcile actual installation back into local inventory
```

These are logical modules, not a request for eight services. Start with one Nebuless application with ingest/publish jobs and one client daemon containing sync, storage, and query modules. The gateway can use an in-process read interface if these live in one trusted process. Use IPC only when deployment or privilege separation warrants it.

### 2.3 What is and is not established about PRX

Local evidence: `README.md` describes PRX as a wire-level policy gateway using HCL/CEL and describes `nebprx run` routing a process tree's network traffic. `doc/external-plugin-control-plane.md` describes host-owned evaluation and brokered plugin calls. Those documents support keeping policy authority in the gateway and secrets out of plugins. They do **not** establish that arbitrary package commands or streaming model replies are already parsed or blocked. This investigation did not audit the implementation of those paths.

The proposed registry must therefore be usable by future command/tool and reply adapters without claiming existing universal interception. The exact integration point needs a separate code investigation and approved implementation task.

## 3. Upstream facts and make-versus-reuse comparison

### 3.1 OSV mirror plus custom PRX API

**Verified.** OSV lists `npm`, `PyPI`, `crates.io`, and `Go` as covered ecosystems. Its exports include an all-database zip, ecosystem zips, individual JSON records, and global/per-ecosystem `modified_id.csv` indexes. The global export includes withdrawn entries. Records that lose ecosystem information can appear under `[EMPTY]`. The change index is sorted by modified timestamp. OSV schema defines aliases, related records, affected ranges and explicit versions, severity, source-specific fields, and withdrawal. [S01, S03]

The query API is documented principally as package/version or commit matching, with pagination that can return a page token without findings. All pages must be exhausted. The read documentation is **not sufficient evidence of a durable all-version subscription/changefeed API**. Do not build subscription correctness around an untested assumption that omitting `version` returns every advisory. PRX can provide that contract from its own package index over mirrored records. [S02]

**Proposed.** Bootstrap the four ecosystem subsets, supplement with global withdrawal/orphan reconciliation, then poll the global change index with overlap and deduplicate by source identity and content digest. Periodically reconcile a full export. Persist ingest cursors only after validation, indexing, change-log publication, and durable commit. A timestamp is an upstream hint, not PRX's authoritative event sequence. Use overlapping windows to handle equal timestamps, delayed visibility, retries and changed records with unexpected timestamps.

Do not copy the full OSV hosting infrastructure solely to expose lookups. Its source is Apache-2.0, but running it also introduces upstream deployment and operational assumptions. Reuse the interchange format and licensed data first. A custom API must still maintain range-matching correctness and distribution security. [S28]

### 3.2 Dependency-Track

**Verified.** Dependency-Track is an SBOM/component-analysis platform, not an artifact repository. Its documentation describes CycloneDX import/export, portfolio usage tracking, VEX, policies, notifications, and a private vulnerability database. BOM ingestion or component changes trigger analysis. The cited documentation says all components are reanalyzed daily. [S05, S06]

Its OSV datasource documentation states that it mirrors selectable ecosystems from the OSV bucket, refreshes daily or on instance restart, marks this a preview feature, and warns synchronization may be incomplete. It also warns that OSV aliases can sometimes describe related rather than truly identical vulnerabilities and disables alias synchronization by default in the documented behavior. GitHub advisory mirroring is separately documented as daily or on restart. These are documented schedules/caveats, not measurements of any deployed version. [S07, S08]

Dependency-Track's OSS Index integration consumes information as identified, not an entire OSS Index mirror. Its documentation says the integration is enabled by default and now requires authentication. A private/offline client deployment must explicitly inventory and disable or reroute external analyzers, repository metadata, notifications, and integrations that can send package identities elsewhere. An external advisory provider's API access does not confer redistribution rights. [S09]

Triage states distinguish false positive from not affected and preserve audit history. Reuse that distinction. Its production database guidance recommends PostgreSQL, rejects embedded H2 for production, and discusses migration/version differences. Do not equate an evaluation deployment with a supported production setup. [S10, S11]

**Proposed use:** optional client-side portfolio system for customers who need SBOM UI, VEX workflow, and organization-wide reporting. Import/export through documented APIs and retain PRX's synchronous admission and signed subset delivery as a separate responsibility. Avoid modifying Dependency-Track's internal tables. A Nebuless-hosted Dependency-Track instance containing client BOMs would contradict the default privacy boundary here.

**Unverified:** exact all-version package enumeration, change notification semantics, offline completeness, tenant isolation, range behavior and query latency in the version chosen for a proof of concept. No cited source establishes that Dependency-Track alone implements the PRX subscription protocol or blocks agent replies before delivery.

### 3.3 Scanners and libraries

**Verified.** OSV-SCALIBR explicitly supports library use for software inventory extraction and vulnerability detection, with configurable plugins and network capabilities. OSV-Scanner is its CLI frontend. OSV-Scanner's strict `--offline` mode does not send project/dependency information and fails when the local database is missing. Its separate `--offline-vulnerabilities` mode can still allow other network-dependent features. Documented offline commit-level scanning is unsupported. [S12–S14]

`govulncheck` adds Go symbol/call-based analysis. The Go database API supports local file-backed serving and distinguishes modules, packages, symbols, `stdlib`, and `toolchain`. Use the documented OSV endpoint/API, not Go's unstable internal YAML format. Current API documentation includes both reviewed and unreviewed report status. [S15, S16]

RustSec publishes an advisory client library and Cargo.lock parser as well as cargo-audit. `pip-audit` audits environments/requirements against known vulnerabilities, but warns dependency resolution is not guaranteed static and can behave like an install. It is not a defense against malicious packages, and cannot infer all vulnerable native libraries exposed through Python distributions. [S17, S18]

**Proposed use:** reuse narrowly selected inventory/range components where their APIs are supported. Initially use a pinned CLI as a differential test oracle if library compatibility is uncertain. Never run an untrusted resolver in the gateway's privileged process merely to discover dependencies. Prefer lockfile/installed-metadata extraction, disable unneeded network plugins, and sandbox metadata resolution without lifecycle scripts or credentials. Source and interface stability need a pinned-version review before embedding.

### 3.4 Artifact proxies and mirrors

**Verified.** Verdaccio is an npm-compatible private proxy registry. PyPI's index API serves distribution download URLs. Cargo registries contain an index and download support. Go module proxies serve module metadata and archives. These responsibilities differ from storing advisory records. [S22–S24, S26]

**Proposed use:** integrate a client's existing artifact proxy as an additional enforcement boundary if available, particularly for exact artifact hashes and transitive downloads. Do not build four package mirrors for this intelligence feature. An artifact proxy cannot prevent a harmful package suggestion before download, cannot see a cached package execution, and is bypassable through direct URLs unless egress/execution policy closes those routes. A vulnerability registry alone cannot guarantee artifact integrity, reproducibility, or availability.

### 3.5 Make-versus-reuse matrix

| Option | Reuse | Missing for the requested architecture | Recommendation |
| --- | --- | --- | --- |
| Dependency-Track | SBOM portfolio, findings, policies, VEX, triage, UI | PRX signed scoped delivery, first-seen synchronous admission, command/reply parsing, proven low-latency freshness | Optional customer integration. Not the sole global or local enforcement engine |
| Full upstream OSV deployment | OSV indexing/query implementation | PRX authentication, tenant policy, client scope protocol, local inventory, enforcement | Evaluate only if upstream runtime compatibility materially lowers effort |
| OSV data mirror + custom API | Existing licensed advisories and OSV schema | Small ingest/index/publish service, secure subscription protocol | Preferred global foundation |
| OSV-SCALIBR / OSV-Scanner | Inventory extraction, matching, test comparison | Delivery security, tenant enrollment, policy/admission state | Reuse selected components. Pin and measure |
| govulncheck / RustSec / pip-audit | Ecosystem-specific validation, reachability and inventory evidence | One shared four-ecosystem service, authenticated delivery, complete admission path | Offline/local enrichment and differential test oracles |
| Artifact proxy | Artifact caching, registry compatibility, possible download admission hooks | Advisory aggregation and pre-delivery reply checks | Complement, never substitute |
| PRX-specific code | Existing gateway policy boundary where applicable | Subscription state, snapshot verifier integration, inventory-specific materialization and adapters | Build only these gaps, after proof of concept |

## 4. Licensing: software is not advisory data

**Verified license texts, read separately:**

| Material | Code license evidence | Advisory data license evidence and consequence |
| --- | --- | --- |
| OSV infrastructure | Apache-2.0 [S28] | Aggregated sources retain their own terms. OSV's source list labels different licenses. Do not relabel its entire export Apache-2.0 [S01] |
| GitHub advisory database | No application library selected from this database | CC-BY-4.0 in repository license. Preserve attribution, license links and modification notices [S29] |
| PyPA advisory database | No separate library selected from this repository | CC-BY-4.0 in repository license [S30] |
| RustSec advisory database | Repository dedication includes code and data, with exceptions | CC0-1.0 by default, **GHSA-derived content CC-BY-4.0** with per-advisory license/URL metadata. Not uniformly CC0 [S31] |
| Go vulnerability database infrastructure | BSD-style Go source license [S32] | Database entries explicitly CC-BY-4.0 in README and `data/LICENSE`. Root code license is not the data license [S33] |
| Dependency-Track | Apache-2.0 [S34] | Its included/fetched intelligence is not relicensed by the application. Commercial datasource rights need separate review |
| OSV-SCALIBR / OSV-Scanner | Apache-2.0 license files [S35] | Same upstream data provenance obligations. Scanner code permission is not dataset permission |
| pip-audit | Apache-2.0, with README noting ISC-derived examples [S18, S36] | Advisory source terms apply independently |
| RustSec client crates | Apache-2.0 OR MIT stated in project README [S17] | Not the same license as advisory-db |
| govulncheck code | BSD-style license in `golang/vuln` [S37] | Go advisory data remains CC-BY-4.0 |
| go-tuf/v2 | Apache-2.0 [S21] | Does not confer rights in signed targets |

**Proposed licensing gate:** maintain source URL, upstream revision/digest, license expression, attribution text and transformation history for every imported record and any enrichment. Include a machine-readable attribution manifest plus accessible license notices in client snapshots. Retain upstream record links when normalizing or merging. Verify exact pinned dependencies and NOTICE files before distribution. Code license checks in this research are not a full transitive dependency legal audit.

Restrict initial ingestion by approved source/license as well as ecosystem. OSV's wider source list includes share-alike and noncommercial/no-derivatives material outside the intended initial set. Filtering by ecosystem alone does not prove redistributability. Reject or quarantine records with unresolved provenance, including enrichment whose applicable terms are unknown. No conclusion is made here about commercial feed redistribution. [S01]

Authenticated service access is compatible in principle with operating a non-public API. However, do not claim exclusive ownership of upstream advisories or impose additional restrictions inconsistent with recipients' CC-BY rights. Service credentials, quotas, Nebuless software, and upstream data rights are different matters. Have counsel review the chosen distribution terms and attribution design before launch.

## 5. Package-wide subscriptions and newly disclosed vulnerabilities

### 5.1 Package identity, advisory coverage, and findings are separate

Use an ecosystem-aware unversioned package key for subscriptions, and exact package/version/source coordinates for findings. A PURL is useful interchange, not evidence that an artifact is genuine. Keep registry/source identity alongside it to distinguish a private package or fork with the same name. Preserve raw coordinates for diagnosis. [S02, S27]

| Dimension | Example states | Why it must not be conflated |
| --- | --- | --- |
| Package existence | verified in approved registry, missing, private, unknown | OSV is not a catalogue of every package |
| Intelligence coverage | complete-for-scope-at-sequence, pending, stale, partial, unsupported | An empty result may be a failed or never-loaded subset |
| Version finding | affected, no-known-match, indeterminate | No match is limited to available data and a valid matcher |
| Graph coverage | complete-for-recorded-target, partial, unresolved, absent | A clean direct dependency can bring a vulnerable transitive dependency |
| Policy decision | allow, deny, review, indeterminate | Client decisions can be stricter than advisory matching |

Avoid a single boolean `safe`. An empty successful response must carry scope, sequence, source health, freshness and completeness. `no-known-match` does not rule out undisclosed vulnerabilities, malware, compromised releases, typosquatting, missing records or an incorrect graph. [S04, S18]

### 5.2 Why version-only storage is insufficient

Suppose a client uses `example-lib@1.4.2`. A version query returns no findings today. Tomorrow an advisory introduces a range affecting 1.4.2. Nothing changed in the lockfile, so a system scanning only on installation never notices. A second problem occurs when the agent proposes 1.5.0, which was not part of the original query.

**Proposed subscription contract:** `(ecosystem, canonical public package name, source namespace)` subscribes to every current and future advisory revision affecting that identity, regardless of installed version. Deliver original ranges, explicit version lists, affected platform/function metadata, and correction/withdrawal events. Keep exact installed-version findings as **derived** rows. Store an empty package scope as a real subscribed scope so the first future advisory is delivered.

All-version subscription means all applicable **advisory records and ranges**, not every released artifact or an assertion about unreleased versions. Open-ended ranges must be retained. Exact-version matching remains ecosystem-specific. A future version may be assessable against open ranges while package existence, integrity or graph resolution remains unknown.

### 5.3 Correct update handling

1. **U1 — Ingest revisions durably.** Store raw evidence and digest even when `modified` is unchanged. Track source ID, transport revision/ETag when available, fetched time, source effective times, and ingest outcome separately. Never assign a new publication time merely to make old intelligence appear fresh.
2. **U2 — Compute affected scope changes.** Compare old and new package memberships. Send removals to old subscribed identities and additions to new ones. A record that loses its ecosystem or corrects its package name must not remain active forever in the old client subset.
3. **U3 — Recompute on data changes.** Find locally installed versions and candidates under changed identities, recalculate matches, then invalidate allow decisions tied to older evidence. Notify locally for newly affected existing inventory. Newly disclosed findings do not automatically mean kill running applications. That is a separate operator policy.
4. **U4 — Reconcile periodically.** Incremental indexes are not an authenticated complete deletion ledger. Reconcile complete source inventories, cross-check suspicious gaps, and record source degradation. Source disappearance alone is not a trusted withdrawal.
5. **U5 — Retain history.** Corrections, withdrawals, local suppressions and reinstatements remain auditable. Keep immutable raw revisions according to a defined retention policy and current materialized state for fast queries.

**Verified caveat:** OSV's FAQ says withdrawn records are excluded from POST query results but retained in exports and direct record lookup. Deletion behavior differs by source: GCS deletions are marked withdrawn with a bulk-change safety threshold, while REST/Git deletions can leave orphaned valid records. This makes successful query-only scanning inadequate for a faithful correction history. [S04]

### 5.4 False positives, aliases and withdrawals

Preserve `(source, source_record_id)` records and alias edges. A CVE/GHSA/RUSTSEC alias helps presentation and deduplication but should not automatically erase source disagreements or merge merely related reports. Keep withdrawn status source-specific. One source's withdrawal must not suppress an independently active advisory from another source without explicit reconciliation. [S03, S07]

Store local `false_positive`, `not_affected`, and risk-accepted exceptions separately from upstream intelligence. Require actor, reason/evidence, environment/package/version scope, expiry and source revision. Invalidate or request renewed approval when affected ranges, identity, graph, or evidence materially change. A customer exception must never mutate the global advisory for other clients. [S10]

Unexplained deletion should produce `source_missing`/review state, not silently change a denial to allow. If an advisory is authoritatively withdrawn, recompute its contribution to the verdict while keeping history. No finding from that record is not evidence that the package as a whole is safe.

## 6. First-seen install and suggestion admission

### 6.1 Proposed admission state machine

```text
observed command / proposed recommendation
  -> parse ecosystem, identity, source, requested version constraint
  -> classify public vs private locally
  -> load existing package-wide scope or create pending candidate
  -> obtain verified scope before any new permission
  -> resolve exact selected version + target-specific dependency graph safely
  -> assess direct + transitive findings, coverage and freshness
  -> bind decision to exact plan, graph hash, snapshot and policy revisions
  -> allow exact operation, deny, or request bounded human review
  -> verify actual artifact/installation and reconcile inventory
```

Never allow the first use merely because the package is absent from the client's current inventory. Subscribe-and-fetch is a real admission step, not a background best effort after installation. If lookup or graph resolution exceeds a bounded local deadline, return `review/indeterminate`; do not hold a tool call indefinitely or convert timeouts to no findings.

**Name-only recommendations:** a package can have both vulnerable and nonmatching versions. Do not ban the entire name because one old release is affected, or approve it because a fixed release exists. Require a specific checked version and qualification, or withhold an actionable install recommendation pending local resolution. A requested range or `latest` tag is not an immutable resolution.

**Previously unseen packages:** verify existence using approved registry metadata separately from advisory matching. Public metadata can be obtained by the client's existing resolver/proxy or an explicitly scoped Nebuless metadata facility. Such a facility has separate coverage, licensing, rate and privacy boundaries. Do not claim the initial advisory service proves name existence. Unknown names can be hallucinated, private or newly published.

### 6.2 Dependency graphs and ecosystem-specific limits

Inventory must retain direct and transitive nodes, edges and paths to affected components. Include source repository/registry, exact version, integrity hash where available, platform, interpreter/toolchain, build/runtime/dev scope, optional dependencies and resolution evidence. Reuse CycloneDX for import/export, but do not infer complete graphs from a component list. CycloneDX explicitly says omitted graph nodes may have unknown dependencies, not zero dependencies. [S38]

| Ecosystem | Required distinctions and proposed tests |
| --- | --- |
| npm | Scoped names, aliases, workspaces, nested versions, optional/peer/dev dependencies, overrides, tags and git/file/URL sources. Check the resolved lockfile graph, not only package.json. Lifecycle scripts and `npx`/equivalent temporary execution remain admission boundaries |
| PyPI | Distribution name is not import name. Normalize names correctly. Use Python version semantics, including epochs, pre/post/dev/local versions, not a universal SemVer comparator. Extras, environment markers, wheels versus sdists, direct URLs and native libraries can change the graph or risk [S02, S18, S39] |
| crates.io | Preserve Cargo source identity, renamed dependencies, features, target-specific/build dependencies, patches and git revisions. A crate with the same name in a private registry is not automatically the public crate. Do not conflate withdrawn advisories, yanked releases and unmaintained notices [S17, S24] |
| Go | Match OSV module names, not arbitrary import paths. Retain `/v2` major paths, pseudo-versions, `replace`, workspace/vendor evidence, standard library and toolchain versions. Symbol reachability is an additional signal, not a universal safe result [S15, S16, S26] |

OSV supports `SEMVER`, `ECOSYSTEM`, and `GIT` range types. Preserve `introduced`, exclusive `fixed`, inclusive `last_affected`, and range limits. Explicit version lists and ranges are both relevant. An unsupported comparator or unavailable Git ancestry yields indeterminate, never no-known-match. Offline OSV-Scanner's documented lack of commit-level support is a concrete reason to retain that state. [S03, S14]

**Verified enrichment limit:** deps.dev's dependency endpoint currently documents npm, Cargo, Maven and PyPI, and models an isolated generic 64-bit Linux install. It reports graph/node errors that may make the whole graph incorrect. Its Go data also has collection restrictions. This is useful public enrichment, not an authoritative reconstruction of a client's private, platform-specific graph. [S40]

Do not execute arbitrary build hooks to resolve an unknown Python sdist. Resolve in a constrained, disposable environment with scripts disabled where supported, or require trusted lock/metadata and return partial. `pip-audit` explicitly warns it cannot guarantee static resolution. [S18]

### 6.3 Blocking agent commands and model replies

**Commands:** prefer structured package-manager/tool arguments over regex over arbitrary shell. Cover exact install operations, temporary package execution, package-file edits and subsequent resolution. Shell expansion, nested scripts, encoded payloads and direct downloads can evade a text parser. Reject or review unsupported forms in a strict policy. A local runner/tool boundary plus controlled network/artifact access is necessary for claims about preventing execution, including cached/offline use.

**Model replies:** enforce before content is released to the client, not after logging the completed response. Parse provider-specific tool calls and structured package recommendations first. For free text, distinguish discussion of a vulnerability from recommending installation. Use bounded buffering across streaming chunks and formats. When a strict blocking policy cannot classify an actionable recommendation, withhold or return a local explanation rather than leak an unsafe prefix and attempt to retract it.

Buffering an entire response can harm latency and memory use. A small sliding window alone cannot guarantee blocking when names, versions or instructions span distant chunks. Start with supported structured outputs and explicit package-manager snippets, document unhandled formats, and test actual provider protocols. Do not promise universal semantic detection or rely on an LLM classifier as the sole security boundary.

Generated replacements must be checked too. Prefer a local error identifying the advisory and requested version over silently rewriting the user's plan or claiming a particular upgrade is compatible. Keep prompt bodies, code and model replies local.

## 7. Secure distribution: snapshots, deltas and TUF

### 7.1 Proposed signed content contract

Deliver deterministic serialized snapshot/delta files covered by target hashes and lengths. The payload contains:

| Field | Required meaning |
| --- | --- |
| `schema_version`, `matcher_compatibility` | Versioned parsing/matching contract. Unsupported required semantics reject activation |
| `feed_epoch`, `global_sequence` | Durable monotonic publication position. Restore must not reuse an old sequence for different content |
| `subscription_id`, `scope_revision`, `scope_digest` | Binds content to the exact expected public identities/shards. Opaque subscription identifier is not authorization |
| `base_sequence`, `base_digest`, `target_sequence`, `target_digest` | Delta applies only to its stated base and produces a verifiable resulting materialization |
| `issued_at`, `expires_at` | Publisher lifetime, not a claim that upstream data was recently changed |
| `source_status[]` | Last successful source check, source watermark, coverage/completeness, license status, errors and ingest lag |
| `records[]`, `removals[]` | Source-qualified upserts and tombstones with reasons/revisions. Empty package scopes are explicit |
| `attribution_manifest` | Source/license/notice mapping for included data |

**Complete scope is essential.** Signing individual advisory records proves neither that all package records were delivered nor that a missing package is clean. Sign a package-scope index or full subset manifest, including explicit empty scopes. On subscription expansion, bootstrap the added scopes at a committed cut and replay subsequent changes so no advisory is lost between snapshot and subscription activation.

Subset delivery may skip global events unrelated to the client. Therefore, contiguous local coverage does **not** mean global sequence numbers must differ by one. A signed delta certifies a complete transition for the same scope from its declared base to target. Any gap in that chain requires a replacement snapshot. Scope changes create a new scope revision and require a full scope bootstrap or explicitly authenticated equivalent.

Stage downloads outside the live database. Validate size/decompression bounds, schema, signatures, expected scope, TUF metadata, sequence/base and source status before a single atomic database transaction activates content and updates its high-water mark. Queries see old or new state, never half an update. Replayed identical content is idempotent. A replay with conflicting bytes at the same revision is an integrity incident.

### 7.2 Why TUF fits

**Verified.** TUF separates root, targets, snapshot and timestamp roles, supports threshold keys and rotation, checks metadata versions and expiration, and authenticates target hashes/lengths. Its threat model includes rollback, freeze, mix-and-match, malicious mirrors and key compromise. It explicitly does not provide update privacy. go-tuf/v2 provides client updater and metadata libraries. [S19–S21]

**Proposed:** use an established TUF client implementation rather than inventing signature canonicalization, metadata rotation and freeze protection. Treat PRX intelligence files as target files. Bootstrap a reviewed trusted root through the PRX release/enrollment route. Keep root keys offline with threshold control. Use separate target/publisher and short-lived timestamp roles with an explicit key ceremony and recovery plan. Evaluate delegated roles or hash-bin targets for public package/shard content, and per-subscription manifests only where necessary.

TUF metadata authenticates the files, not the correctness of normalized advisories, SQL application logic, tenant policies or source freshness. PRX still needs application sequence/scope validation, delta replay, license handling, policy semantics and atomic storage. Authentication still protects endpoints and private subscription metadata. A signature on an old feed cannot make the feed fresh.

**Costs to test:** metadata count and signing load for many dynamic subscriptions, root rotation for intermittently connected clients, deletion/retention of target files, and library compatibility with authenticated fetching. Prefer shared content-addressed targets behind authorization over signing a unique copy of every advisory for every client. A custom fetcher can attach installation credentials without making targets public. [S20]

### 7.3 Expiry, rollback and recovery

Persist the highest accepted TUF metadata versions and PRX scope sequence outside disposable caches. Reject older snapshots even if their signatures are valid and their expiry has not passed. Restore server backups without resetting the publication counter. A new feed epoch is not self-authorizing: it requires a root-authorized transition/rebootstrap procedure, not acceptance of any signed lower sequence.

Expiration requires a trustworthy time basis. Detect clock rollback relative to persisted observations, monitor time synchronization, and return degraded/indeterminate when clock trust is inadequate. Monotonic process time helps during a process lifetime but does not solve clock tampering across restarts or cloned VM snapshots. A fresh installation with no prior state cannot infer which valid unexpired snapshot is newest without trusted update metadata.

Keep the last **verified** snapshot as an availability fallback, not the last artifact that happened to download. Bad updates never replace it. If a trusted publisher ships erroneous intelligence, publish a corrective **higher-sequence** revision. Do not disable rollback protection to restore old content.

TUF must reject expired update metadata. Retaining an already verified old local dataset for restricted offline operation is a separate, explicit client risk policy, not a declaration that expired metadata is valid. Air-gapped clients need authenticated import bundles and a deliberate offline validity/renewal policy. Longer expiry expands the freeze window and cannot promise live disclosure coverage.

## 8. Authentication, tenant isolation and privacy

### 8.1 Service access

**Proposed:** only an enrolled client PRX installation's sync identity can reach Nebuless's protected intelligence endpoints. Issue per-installation keys/certificates after authenticated administrator enrollment. No shared secret in an OSS binary, download image, browser bundle, or example configuration. Rotate and revoke credentials individually. Separate tenants, installations, roles and entitlements.

Start with mTLS installation authentication and server-side authorization. If an existing identity service issues OAuth tokens, certificate-bound tokens are standardized in RFC 8705 and can limit stolen-token reuse. Do not add an OAuth service merely to satisfy this architecture. If TLS terminates at a proxy, certificate identity must be verified there and securely conveyed only over an authenticated trusted hop. Never trust a caller-supplied client-certificate header. [S25]

Authorize every subscription, cursor and object fetch against the authenticated installation. An opaque ID, signed target path, valid URL or possession of an advisory digest is not sufficient authorization. Check revocation on refresh and bound token/certificate lifetime to the supported revocation model. Avoid public object-bucket ACLs or anonymous CDN access. If using expiring URLs, treat them as bearer credentials with scoped lifetime and prevent log/referrer leakage.

The global origin can be internet-reachable for outbound installation sync while still rejecting anonymous requests. Private network deployment is an optional additional control, not a replacement for installation authentication. Restrict ingest workers' outbound destinations and disallow arbitrary fetch URLs supplied by tenants or advisory reference fields. Do not let sync credentials authorize source ingest or publication.

### 8.2 Limits of protecting a reproducible OSS client

Publishing PRX source and public signing roots does not expose Nebuless private keys or create a right to use the service. Server-held enrollment/entitlement state controls access. However, an authorized customer controls its own machine and can extract credentials, alter OSS code, or call an allowed endpoint directly. mTLS proves key possession, not binary identity or benign intent.

Consequently, “installations only” is an identity/authorization contract, not an unforgeable assertion of application type. Use per-install quotas, scope caps, rate limits, audit and revocation to limit abuse. Never embed a universal secret, rely on an obscured endpoint, or promise to prevent all copying of advisories already delivered to an authorized client. Hardware-backed attestation would be a separate product/trust decision and still has deployment limits.

### 8.3 Outbound-only sync and local IPC

Client sync initiates outbound HTTPS. Nebuless does not need an inbound connection into the client's gateway. Use polling with jitter and bounded exponential retry initially. Server-initiated push adds connectivity/operational complexity without removing the need for replay and full resync.

Local checks use an in-process interface or a Unix-domain socket with restrictive ownership/permissions and caller validation, or an ACL-protected Windows named pipe. A separate local-network registry needs mutually authenticated transport and network policy. Loopback alone is not authorization on a multi-user host. Keep mutation/admin endpoints separate from the gateway's read/check capability. Agents and sandboxed plugins receive no cloud synchronization credentials.

### 8.4 Data minimization and private packages

Default outbound payload: public ecosystem/name subscriptions or broader public feed shard identifiers, installation identity, protocol cursor and necessary transport metadata. Do not send exact installed versions, graph edges, private paths, code, lockfiles, SBOMs, prompts, replies, environment variables, secrets or registry credentials. Even public package subscriptions reveal technology choices, so make that disclosure visible to client administrators.

Classify private registries/namespaces locally **before** querying Nebuless, OSV, deps.dev, or an upstream package registry. Keep private advisories and fork mappings in the client store. A private fork needs an explicit local mapping and patch/provenance evidence, not automatic equivalence to the public package. An internal import name must not be sent out merely because it is syntactically valid.

Hashing names is not adequate anonymization: public names form an enumerable dictionary, hashes are linkable, and a server that performs ordinary name matching usually needs the identity or an equivalent lookup handle. A shared salt remains enumerable; a secret client salt prevents normal server lookup. Do not describe hash-prefix queries as private without a defined privacy analysis.

For privacy-sensitive clients, authenticated download of full ecosystem data or coarse shared shards allows local filtering and keeps exact inventory private, at higher bandwidth/storage cost. The **operational** client registry can remain inventory-specific while holding a separate broad-feed staging/cache area. Truly inventory-specific cloud responses and zero disclosure of selected identities are competing requirements. PIR/OPRF designs could be researched later but are not a minimal first version.

Keep subscription retention and logs minimal. Package identities can leak through paths, tracing, analytics and error messages even when request bodies are redacted. Avoid request-body logging, rotate opaque subscription IDs, define tenant deletion/backup retention, and never mix client private advisories into the global public corpus.

## 9. Proposed data model and storage

### 9.1 Global model

| Entity | Minimal fields |
| --- | --- |
| `source` | Stable ID, canonical feed URL, license/attribution policy, last successful check, source watermark, error/completeness status |
| `advisory_revision` | Source + record ID + immutable digest/revision, raw OSV, upstream published/modified/withdrawn, fetched/accepted time, provenance, normalization version |
| `affected_package` | Advisory revision, ecosystem, canonical name, source identity, original range/version data, evaluator compatibility |
| `advisory_relation` | Source-qualified alias/related/upstream edges and confidence/provenance, not destructive merging |
| `change_event` | Monotonic sequence, previous/current revision, old/new package scopes, operation/reason |
| `installation` / `subscription` | Tenant and installation authorization, scope digest/revision, active identities/shards, cursor, retention, quotas |
| `publication` | Feed epoch/sequence, scope, hashes/lengths, source health, attribution, issue/expiry, TUF target references |

### 9.2 Client model

| Entity | Minimal fields |
| --- | --- |
| `environment` / `inventory_revision` | Local-only identity, collector/evidence version, lock/SBOM hashes, target platform, collection time, completeness |
| `package_instance` | Ecosystem/name/source, raw and canonical version, artifact digest if known, scope, evidence locator, public/private classification |
| `dependency_edge` | Inventory revision, parent/child instance IDs, declared constraint, resolved version, optional/build/dev/target metadata and path provenance |
| `package_scope` | Active/candidate state, public subscription or local private source, coverage, snapshot scope revision, last verified update, no-known-records marker |
| `local_advisory` | Verified source revisions, ranges, provenance/license data plus separately owned private advisories |
| `finding` | Package instance + source-qualified advisory revision, matcher version, affectedness, uncertainty/reachability evidence |
| `exception` / `decision` | Client policy revision, actor/reason/expiry, plan/graph digest, evidence sequence, outcome and audit linkage |
| `sync_state` | Trusted TUF state, monotonic high-water marks, epoch, applied scope/digest, expiry/source health, last verified dataset |

Keep intelligence, inventory and policy revisions separate so an advisory correction invalidates a decision without pretending the installed package changed. Store decisions with the evidence used to reproduce them. Bound candidate retention to avoid an attacker growing local or cloud subscriptions indefinitely.

### 9.3 Storage choice without premature services

**Proposed:** one global application backed by PostgreSQL for relational indexing, transactions, tenant state and change jobs. Keep raw OSV JSON and normalized indexes, not only flattened summaries. An object store can hold immutable compressed targets and raw source evidence when it lowers delivery/retention cost. A database-backed job table/outbox provides crash-safe publication initially. No Kafka, Elasticsearch, graph database or per-ecosystem service is justified by the current evidence.

Use SQLite for a single-host client daemon with one controlled writer, transactional activation and concurrent reads. Consider PostgreSQL only for a multi-gateway shared client service or demonstrated write/concurrency needs. Do not share a SQLite database over a network filesystem as a clustering strategy. Matchers and package/advisory indexes, not full-text search, dominate the hot path.

Index global `(ecosystem, canonical_name, source_namespace)`, record identity/revision and sequence. Index client package identity/version and reverse dependency edges. Cache decisions keyed by **all** of plan, graph, policy, inventory, matcher and intelligence revisions. Eviction is not evidence deletion. Define raw evidence, historical finding and tombstone retention before launch.

## 10. Minimal API contract, not implementation

All endpoints below are **proposed PRX APIs**, not OSV or Dependency-Track endpoints. Version the schema and publish typed/OpenAPI contracts during implementation. Use UTC RFC3339 for machine exchange while preserving PRX's required `yyyy-MM-dd HH:mm:ss.SSS` presentation in any eventual UI.

### 10.1 Nebuless service: installation identities only

| Endpoint | Input / output | Semantics |
| --- | --- | --- |
| `PUT /v1/subscriptions/{id}` | Desired public package identities or approved shard IDs, expected scope revision, idempotency key | Installation-owned resource. Atomic replacement/expansion returns a new scope revision and bootstrap target at a fixed cut. Same key/body repeats result, conflicting body is an error |
| `GET /v1/subscriptions/{id}/manifest` | Expected scope revision; returns current authorized publication and trusted metadata/target references | Binds empty scopes and completeness. Not a boolean package verdict |
| `GET /v1/subscriptions/{id}/changes?cursor=...` | Opaque scope-bound cursor; ordered page of signed transition target references and continuation | A cursor is bound to installation/scope and a committed replay cut. Final-page marker establishes completion. Not all gaps imply missing relevant events |
| `GET /v1/targets/{digest}` | Authorized immutable content address | Verify authorization independently of digest. Return exact bytes/hash/length covered by trusted target metadata |
| `GET /v1/status` | Minimal authenticated source/ingest/publisher health | Does not reveal other tenants or private subscriptions; cannot override signed per-publication health |

Enrollment/renewal is a separate administrator/installation identity flow, not a public self-asserted request saying “I am PRX.” Signed/TUF metadata paths are likewise protected, with only the bootstrap trust root shipped through the approved distribution channel.

A subscription response must distinguish `pending` from `complete`. First-seen requests cannot use a `202` subscription creation as admission permission. Return a complete bootstrap or a bounded pending state and let the local policy withhold permission. Public package version need not be sent to Nebuless when package-wide advisory data suffices.

### 10.2 Client-only interface

Conceptual `CheckPlan` request fields:

```text
operation: install | execute | recommend
ecosystem, canonical_name, source_namespace
requested_constraint, resolved_version?
plan_digest, inventory_revision?, dependency_graph_digest?
target_platform, local_policy_revision, deadline
```

Response fields:

```text
decision: allow | deny | review | indeterminate
finding_state: affected | no_known_match | indeterminate
coverage: complete_for_scope | partial | pending | unsupported
freshness: fresh | stale | expired | unknown
graph_state: complete_for_target | partial | unresolved | absent
package_existence: verified | missing | private | unknown
reasons[], source_qualified_advisory_ids[], local_dependency_paths[]
evidence: feed_epoch, sequence, scope_digest, matcher_version, policy_revision
plan_digest, decision_expires_at, retryable
```

These fields remain local. The operation is check-only. It must not invoke a shell or install package artifacts as a side effect. Inventory imports and sync/apply actions require separate mutation authority. Existence and graph information can be unknown even when the advisory slice is complete.

### 10.3 Errors and failure semantics

| Condition | Transport / local result | Required behavior |
| --- | --- | --- |
| Invalid shape, too many identities, unsupported comparator | `400/413/422`, typed reason | No successful empty result; bound size, nesting and work |
| Missing/invalid identity; unauthorized scope | `401/403` | No data, renewal/re-enrollment path; do not silently fall back to anonymous upstream queries |
| Scope revision mismatch or wrong delta base | `409` or typed local integrity error | Fetch full authorized snapshot; retain current verified dataset |
| Cursor outside replay retention | `410` with snapshot-required reason | Full resync, no assumption that missed changes were irrelevant |
| Rate limit | `429` with bounded retry guidance | Jitter/backoff, local policy handles missing freshness |
| Source outage / ingest partial | Health state in signed manifest; possibly `503` for unavailable service | Never issue apparently fresh complete coverage from old data without marking source degradation |
| Invalid signature/hash, unexpected scope, sequence rollback | Local verification failure | Do not activate, alert, preserve last verified state; no automatic trust reset |
| Missing snapshot, unhandled source/version or malformed graph | `indeterminate` | New execution/admission withheld or human-approved under explicit policy |
| `200` with zero findings | `no_known_match` only with evidence dimensions | Not safe, not proof of existence, and not proof of transitive coverage |

Paginate deterministically against a snapshot/cut; never infer completeness from an empty intermediate page. A timeout is an unknown result, not a clean check. Make mutation retries idempotent and atomically bind publication sequence to content.

## 11. Offline and degraded-operation policy

The registry must work without a synchronous cloud call for known subscribed packages. Local policy must choose availability versus prevention explicitly. The following is a **proposed strict default for package-changing/execute operations**, not a decision to stop all gateway traffic:

| Local state | Existing pinned operation already admitted | New package/version or actionable recommendation |
| --- | --- | --- |
| Verified, fresh, sufficient scope/graph | Recheck current policy and allow/deny normally | Resolve/check then decide |
| Verified but stale within approved grace | Continue only if local policy permits the unchanged pinned plan and no known blocking advisory | Review/withhold by default; explicit bounded exception possible |
| Expired intelligence or source coverage untrusted | Retain for diagnostics and known denials. Unchanged operations require explicit offline continuity policy | Deny or human review. Never treat absence as clearance |
| No trusted dataset or corrupted store | No automatic allow | Indeterminate, withhold |
| Offline and package outside subscribed set | Existing decision cannot cover a different package | Pending/unknown. Require authenticated offline import or review |

Offline continuation exceptions must bind package/source/version, graph and artifact digests, operation, approving actor, reason and expiry. They cannot override a known critical finding silently. A “known-good snapshot” means integrity-verified and previously accepted, not a guarantee of vulnerability-free contents.

Credential expiry/revocation stops future cloud access but cannot retract already delivered bytes or instantly revoke a disconnected local exception. Signed bundle expiry and local policy bound this exposure. Explicitly explain the tradeoff to operators. Network outages and source feed outages are distinct, and a freshly signed manifest must not disguise the latter.

## 12. Freshness, observability and benchmark-based sizing

### 12.1 Measured clocks and proposed SLOs

**Verified upstream statement:** OSV's FAQ publishes availability/latency objectives and a data-freshness SLO of source data no more than 15 minutes stale 99.5% of the time. This is an upstream objective, not a guarantee that every disclosure is imported within 15 minutes or that PRX has inherited that property. Missing/import-failed records and upstream source publication delays remain possible. Go's API documentation also cautions against comparing its database modified timestamp to wall clock for cache invalidation because visibility can lag. [S04, S16]

Measure separate intervals: source publication to OSV visibility (external), first successful PRX observation to global indexed revision, global committed revision to signed availability, signed availability to client activation, and activation to recomputed findings. Expose last successful source **check** separately from latest advisory **modified** time. A quiet source can have old modified times without a failed poll.

**Proposed PRX proof-of-concept targets, to validate and renegotiate after measurement:**

| Metric | Initial target, not an upstream guarantee |
| --- | --- |
| Poll cadence | Attempt relevant change-index checks every 5 minutes with jitter |
| PRX-observed upstream change to published global update | p95 within 10 minutes and p99 within 30 minutes under a declared healthy-source workload |
| Publication to activation at connected, healthy clients | p95 within 10 minutes and p99 within 30 minutes |
| Client activation to findings invalidation/recalculation | p95 within 1 minute for the explicitly benchmarked inventory profile |
| Warm local exact-plan lookup | p95 below 20 ms at the agreed gateway concurrency/profile, excluding graph resolution and human approval |
| Source/sync degradation alert | Emit when no successful relevant check or no client progress for 30 minutes; classify cause before interpreting as stale data |
| Initial strict-policy age limits | Warn after 30 minutes of failed sync/checks; withhold new admission after 2 hours without sufficient verified coverage; 24-hour signed content expiry as an initial test value |

The time limits above are policy hypotheses. TUF timestamp/target expiry, source freshness and offline grace are separate values. Tune them together using outage drills, expected disconnected-client duration and key rotation procedures. Do not offer a contractual SLO or availability number before load and recovery tests.

Record ingest/reject counts by source, withdrawal spikes, oldest pending event, publisher/signer errors, cursor age, full-resync frequency, local decision reasons, graph completeness, source status and high-water marks. Keep telemetry aggregate and secret-free. Test new disclosed advisories with controlled fixture timestamps rather than claiming a publication-to-client bound for all real-world sources.

### 12.2 Sizing plan instead of invented capacity

No capacity benchmark was run. No claim is made that a particular RAM size, database or instance handles an arbitrary number of clients/packages. No corpus download, heavy build or test was performed locally for this research. Later heavy benchmarks run on **EGGIE** under the current development boundary, using authorized workloads and synthetic/public evidence. Historical references to other hosts record the earlier research environment.

Measure on pinned code/data revisions:

1. **B1 — Corpus:** record date, selected sources/licenses, advisory and affected-package counts, compressed/uncompressed bytes, revision churn, index/JSON overhead, withdrawals and alias fanout. Include `[EMPTY]` and reassigned-package cases.
2. **B2 — Client profiles:** sample authorized real profiles locally without exporting inventory, or synthetic small/medium/large graphs based on observed distributions. Include many versions of the same name, mostly empty package scopes, private packages and candidate floods. State node/edge/package/advisory counts rather than vague project counts.
3. **B3 — Global workloads:** bootstrap, incremental ingest, high-fanout advisory correction, scope expansion, subscriber reconnect storms, expired cursor resync and overlapping ingest retries. Measure signing cost separately from indexing.
4. **B4 — Client workloads:** cold bootstrap, steady deltas, warm/cold checks, changed-range recomputation, graph invalidation, atomic activation under concurrent reads, restart and disk-full recovery. Disable unintended network calls and verify by capture/firewall.
5. **B5 — Metrics:** CPU/RSS, bytes read/written, database/index sizes, transaction contention, queue depth/age, p50/p95/p99 latency, bytes per client-day and per full resync, signature verification CPU and error rates. Repeat with declared concurrency and hardware limits.
6. **B6 — Capacity decision:** size from measured peak rates, backlog-drain time and outage-recovery needs plus stated headroom. Add replicas, partitioning, job queues or new services only when a measured constraint identifies the benefit.

Useful accounting: global storage is raw retained revisions + current indexes + replay log + signed target retention. Client storage is subscribed advisory subsets + local inventory/graphs + decisions/history + staging headroom. Bandwidth is subscription bootstraps + relevant delta fanout + periodic reconciliation + reconnects. Shared package target files can reduce duplicated storage, but per-install authentication and scope manifests still cost work.

## 13. Threat cases and controls

| Case | Failure | Proposed control and residual limit |
| --- | --- | --- |
| T1: Anonymous caller or copied OSS binary | Direct access to global intelligence | Enrollment, per-install mTLS authorization, quotas and revocation. Authorized key holders can still emulate the client |
| T2: Compromised distribution/cache | Tampered or mixed records | TUF hashes/versions/expiry plus complete scope manifest, base checks, atomic activation. Does not correct a compromised authoritative publisher's false claims |
| T3: Replay/freeze or clock rollback | Old valid data appears current | Persist high-water marks, trusted time checks, TUF expiry and source-health age. Local admin/VM-state rollback remains a stronger threat than ordinary network replay |
| T4: Tenant boundary bypass | Other clients' subscription metadata leaks | Authorization on every ID/cursor/target path, tenant-bound queries and negative tests. Shared public advisories are not private inventory |
| T5: First-seen unknown package | Empty local store permits install | Pending candidate state, existence and all-version scope bootstrap, exact graph admission. Unknown is not safe |
| T6: New disclosure with unchanged inventory | Previously allowed installed version stays allowed | Package-wide subscriptions, recomputation and revision-bound decision invalidation |
| T7: Unsafe transitive dependency or changed resolution | Clean direct package bypasses block | Target-specific graph plus artifact/lock binding and post-resolution check. Unknown graph edges cause partial state |
| T8: Private package/name leakage | Sensitive stack details sent to Nebuless or public APIs | Local classification, outbound allowlist, broader shard option, no raw SBOM/prompt upload. Name hashing alone is inadequate |
| T9: Malicious advisory metadata | Parser/SQL/SSRF/resource exhaustion | Schema/size/depth limits, parameterized storage, no arbitrary reference fetches, archive extraction bounds and least-privileged ingest |
| T10: False withdrawal or bulk deletion | Existing denies silently disappear | Source-qualified history, signed tombstones, disappearance quarantine, mass-change alert and independent-source reconciliation |
| T11: Prompt injection or chunk-split recommendation | Unsafe content reaches agent/user before check | Treat output as untrusted, provider-aware framing, pre-release buffering, explicit unsupported-format policy. Semantic completeness is not guaranteed |
| T12: Credential theft or local bypass | Agent queries cloud or edits registry | Keep cloud keys outside agent environment, IPC ACLs, read-only checks, file ownership and audit. Client administrator/root compromise is outside this control's prevention claim |
| T13: Resolver/install script runs during checking | Admission itself executes attacker code | Static extraction first, isolated/script-disabled resolution, no credentials, no privileged in-process installer [S18] |
| T14: Upstream lacks a vulnerability record | “No advisories” interpreted as secure | Coverage provenance, explicit no-known-match result, separate malware/provenance controls and no universal coverage claim |

## 14. Phased proof of concept and acceptance gates

No phase is implementation authorization. All later work needs approved scope. Do not deploy a partial prototype as an enforcement guarantee.

### P1 — Data correctness and license/provenance feasibility

Use a bounded public fixture set for the four ecosystems, plus synthetic advisories for deterministic edge cases. Include unaffected/affected versions, multiple ranges, prereleases, withdrawn/alias conflicts, name normalization, missing sources and unsupported Git cases. Record pinned upstream tool/data revisions and notices.

Acceptance: **A1** every imported fixture has source/digest/license evidence; **A2** boundary-version matches agree with the intended ecosystem semantics and documented reference tool results or have reviewed differences; **A3** zero findings is distinguishable from unknown/partial/unsupported; **A4** database revision changes trigger reevaluation of unchanged inventory.

### P2 — Two-registry synchronization

Prototype a small global mirror/index and an independently stored client subset. Subscribe by unversioned identity and include empty scopes. Start with immutable full snapshots, then add deltas only after snapshot verification and recovery are proven.

Acceptance: **A5** a newly disclosed advisory reaches a previously empty subscribed scope without a lockfile change; **A6** an advisory moved between package identities is removed from the old scope and added to the new one; **A7** withdrawal/deletion/alias conflicts preserve history and do not globally erase independent evidence; **A8** a subscription added during publication has no snapshot/replay race.

### P3 — Distribution security and privacy

Integrate a pinned TUF verifier and installation credentials, test multiple synthetic tenants and source outages, and inspect actual outbound data. No real private inventory or prompts are needed.

Acceptance: **A9** anonymous/cross-tenant requests fail, including object/cursor guessing; **A10** bad signatures, wrong scope, tampered lengths, expired metadata, lower sequences, replay and missing bases fail without replacing verified data; **A11** root/online-key rotation and interrupted sync recover without trust reset; **A12** packet/log inspection shows no secrets, prompts, replies, private names, full graph or exact installed versions in default sync.

### P4 — Local admission, graph and offline behavior

Support one explicit operation family per ecosystem before arbitrary shell parsing. Use local exact package/graph inputs and structured model recommendation fixtures. Defer unstructured universal detection claims.

Acceptance: **A13** new package, upgrade, range/tag resolution and vulnerable transitive cases cannot use absent inventory as clearance; **A14** decision replay after graph, artifact, policy or intelligence changes is rejected/rechecked; **A15** offline fresh/stale/expired/missing/corrupt states follow the approved policy with visible reasons; **A16** cached execution and direct URL paths are either controlled by an actual hook or explicitly declared outside coverage.

### P5 — Model-reply and operational readiness tests

Test supported provider streaming/tool protocols and the selected workload on the approved benchmark host.

Acceptance: **A17** split package/version strings, tool-call fragments, code blocks and harmful-prefix cases are withheld before release; benign vulnerability discussion is not automatically treated as a recommendation; **A18** generated alternative versions are rechecked; **A19** disk full, crash mid-apply, source silence, signer failure, reconnect storm and cursor expiry preserve integrity and surface uncertainty; **A20** measured freshness/latency/recovery results support or revise the proposed SLOs, with no invented capacity claims.

After these gates, decide whether Dependency-Track adds enough portfolio value to ship as an optional integration, whether additional private metadata resolution is necessary, and which command/reply formats can be honestly supported.

## 15. Decisions still required

- **Q1 — Admission posture:** which operations must fail closed, and who can approve time-bounded offline exceptions? Severity-only blocking, reachability-aware policy and risk acceptance need explicit client ownership. Missing severity must not silently mean low severity.
- **Q2 — Privacy mode:** may Nebuless see public package identities, or must clients download broader shards? Private names/graphs/prompts remain local in both designs.
- **Q3 — Coverage claim:** which package managers, lockfile versions, platforms, shell forms and model protocols will the first release support? npm/PyPI/crates.io/Go labels do not imply every install form or vulnerable native dependency is covered.
- **Q4 — Dataset policy:** which source licenses and enrichments are approved for commercial redistribution, and how will attribution and disputed/deleted records be handled?
- **Q5 — Operations:** who owns signing roots, credential enrollment/revocation, source-quality incidents, availability recovery and air-gapped renewal?
- **Q6 — Reuse decision:** does the client need Dependency-Track's portfolio UI/VEX workflows enough to justify its runtime and database, or is a small local registry sufficient? Test pinned versions before commitment.

## 16. Primary source register

Sources below were read during this investigation. License claims use license files or explicit maintainer license statements, not a tool's marketing page. Documentation schedules and objectives are attributed upstream facts, not production measurements. Unresolved pages and search snippets were not used as proof of capabilities.

- **S01 — OSV data sources, coverage and exports:** https://google.github.io/osv.dev/data/
- **S02 — OSV query contract and pagination:** https://google.github.io/osv.dev/post-v1-query/
- **S03 — OSV schema and range/withdrawal semantics:** https://ossf.github.io/osv-schema/
- **S04 — OSV FAQ, data quality, withdrawal/deletion behavior and published SLOs:** https://google.github.io/osv.dev/faq/
- **S05 — Dependency-Track capabilities:** https://docs.dependencytrack.org/
- **S06 — Dependency-Track BOM ingestion and continuous reanalysis:** https://docs.dependencytrack.org/usage/cicd/
- **S07 — Dependency-Track OSV integration caveats, cadence and aliases:** https://docs.dependencytrack.org/datasources/osv/
- **S08 — Dependency-Track GitHub advisory integration:** https://docs.dependencytrack.org/datasources/github-advisories/
- **S09 — Dependency-Track OSS Index behavior/authentication:** https://docs.dependencytrack.org/datasources/ossindex/
- **S10 — Dependency-Track triage states and audit:** https://docs.dependencytrack.org/triage/analysis-states/
- **S11 — Dependency-Track database support:** https://docs.dependencytrack.org/getting-started/database-support/
- **S12 — OSV-SCALIBR library interface, capabilities and platform caveats:** https://raw.githubusercontent.com/google/osv-scalibr/main/README.md
- **S13 — OSV-Scanner scope, architecture and outbound service disclosure:** https://raw.githubusercontent.com/google/osv-scanner/main/README.md
- **S14 — OSV-Scanner strict offline versus offline-vulnerabilities and limitations:** https://google.github.io/osv-scanner/usage/offline-mode/
- **S15 — Go vulnerability tooling and call analysis:** https://go.dev/doc/security/vuln/
- **S16 — Go database API, review status, version caveats and local serving:** https://go.dev/doc/security/vuln/database
- **S17 — RustSec library/CLI workspace and code licensing:** https://raw.githubusercontent.com/RustSec/rustsec/main/README.md
- **S18 — pip-audit scope, security model, resolver warning and licensing details:** https://raw.githubusercontent.com/pypa/pip-audit/main/README.md
- **S19 — TUF specification, roles, attacks, expiry and client workflow:** https://theupdateframework.github.io/specification/latest/
- **S20 — go-tuf/v2 client, updater and fetcher support:** https://raw.githubusercontent.com/theupdateframework/go-tuf/master/README.md
- **S21 — go-tuf license:** https://raw.githubusercontent.com/theupdateframework/go-tuf/master/LICENSE
- **S22 — Verdaccio's artifact proxy role:** https://verdaccio.org/docs/what-is-verdaccio/
- **S23 — PyPI index and distribution downloads:** https://docs.pypi.org/api/index-api/
- **S24 — Cargo registry/index role:** https://doc.rust-lang.org/cargo/reference/registries.html
- **S25 — mTLS and certificate-bound OAuth access tokens:** https://www.rfc-editor.org/rfc/rfc8705.html
- **S26 — Go module identity, resolution and proxy protocol:** https://go.dev/ref/mod
- **S27 — Package URL identification overview:** https://www.packageurl.org/docs/purl/introduction
- **S28 — OSV infrastructure code license:** https://raw.githubusercontent.com/google/osv.dev/master/LICENSE
- **S29 — GitHub advisory data license:** https://raw.githubusercontent.com/github/advisory-database/main/LICENSE.md
- **S30 — PyPA advisory data license:** https://raw.githubusercontent.com/pypa/advisory-database/main/LICENSE
- **S31 — RustSec data/code dedication and GHSA exception:** https://raw.githubusercontent.com/RustSec/advisory-db/main/LICENSE.txt
- **S32 — Go vulnerability database code license:** https://raw.githubusercontent.com/golang/vulndb/master/LICENSE
- **S33 — Go data/code licensing split:** https://raw.githubusercontent.com/golang/vulndb/master/README.md and https://raw.githubusercontent.com/golang/vulndb/master/data/LICENSE
- **S34 — Dependency-Track code license:** https://raw.githubusercontent.com/DependencyTrack/dependency-track/master/LICENSE.txt
- **S35 — Scanner/library code licenses:** https://raw.githubusercontent.com/google/osv-scanner/main/LICENSE and https://raw.githubusercontent.com/google/osv-scalibr/main/LICENSE
- **S36 — pip-audit code license:** https://raw.githubusercontent.com/pypa/pip-audit/main/LICENSE
- **S37 — govulncheck code license:** https://raw.githubusercontent.com/golang/vuln/master/LICENSE
- **S38 — CycloneDX dependency completeness semantics:** https://cyclonedx.org/docs/1.6/json/ (the `dependencies` section)
- **S39 — Python version semantics:** https://packaging.python.org/en/latest/specifications/version-specifiers/
- **S40 — deps.dev coverage and graph limits:** https://docs.deps.dev/api/v3/ (Coverage, GetVersion and GetDependencies)

### Research limitations

No code was implemented, no software installed, no services deployed, and no heavy computation or full advisory corpus processing was performed. Tools, ranges, APIs and license texts were read, not exercised end-to-end. No benchmark, legal opinion, universal coverage claim or production freshness guarantee is supplied. Versionless OSV package queries, specific pinned-library ABI/API compatibility, Dependency-Track package-wide subscription suitability, and PRX command/reply interception remain unverified integration questions.
