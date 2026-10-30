# PRX package intelligence and enforcement research

Research date: **2026-10-26 (local date, as requested)**. Status: investigation and recommendations, not implementation or security certification.

Repository: Nebuless/PRX, inspected HEAD 060ff6dfb52a12ea5154b2d3739843b874ec1fde. Local citations refer to inspected working-tree files. Unrelated uncommitted website work was untouched. No application changes, installations, builds, tests, commits, deployments or delegation occurred during this research.

**Evidence convention:** Verified means inspected code or primary documentation supports the statement. Recommendation means proposed PRX behavior. Unverified denotes an operational, compatibility or legal gap. Sources are live documents, not historical snapshots frozen to the requested date. HTTP modification metadata does not establish release dates, installed-version compatibility or feed freshness. No universal coverage or freshness SLA was established.

## 1. Findings

| ID | Finding |
| --- | --- |
| F1 | Build **two distinct vulnerability intelligence registries**: Nebuless's global aggregation and a client-hosted, inventory-aware registry serving the client's PRX gateway. Neither inherently requires a package artifact mirror. |
| F2 | OSV provides a practical schema and aggregation source for npm, PyPI, crates.io and Go. Its software license does not license all aggregated data. Preserve source-specific and record-specific rights. |
| F3 | Inspected PRX code gates network requests, not every local command. HTTP responses reach the client before usage analysis. Credential transforms mutate requests, and SSH command matching explicitly has limitations. |
| F4 | Effective pre-execution enforcement must authorize the resolved graph and artifacts before code runs, including cached and build-time dependencies. A PATH wrapper or network proxy alone is bypassable. |
| F5 | Reply prevention requires protocol-aware buffering before delivery. SSE chunks, WebSocket frames and post-response samples are not sufficient security boundaries. Natural-language recommendation extraction remains ambiguous. |
| F6 | No advisory match means “no known matching advisory in this snapshot,” not “safe.” Existence, vulnerability, maliciousness, freshness, extraction completeness and policy authorization are separate states. |

**D1, recommendation:** build the two-registry foundation for all four ecosystems, then one locked npm execution path and one bounded non-streaming response adapter. Python/Cargo/Go execution controls and SSE/WebSocket response prevention should be separately accepted slices.

**D2, recommendation:** global service access belongs only to authenticated PRX installations. Agents and client applications call their client-hosted gateway or execution broker. Never embed Nebuless installation credentials in those applications.

**D3, recommendation:** claim “blocks policy-disallowed known package/version findings on supported enforced paths,” not universal insecure-package prevention.

## 2. Two registries, not two package mirrors

### Global registry: Nebuless-hosted

**Recommendation.** Maintain normalized advisory intelligence across npm, PyPI, crates.io and Go independently of client inventories. Keep original records, source IDs/revisions, licenses, attribution, range semantics, aliases, withdrawals and transformation provenance. OSV ecosystem exports can be the initial acquisition path, filtered to approved sources. Direct source reconciliation should add value rather than duplicate every OSV record.

Offer authenticated lookup and snapshot/delta synchronization. Use installation-specific credentials, rotation, revocation, tenant authorization, rate/size limits and audit records. mTLS or short-lived installation tokens are candidate mechanisms, not verified existing functionality. Authentication identifies credential possession, not an unforgeable proof that the caller is PRX.

Do not require full inventories, source code or private package names at Nebuless. Clients can request public-package subsets, or synchronize broader ecosystem snapshots if dependency-name privacy matters. Keep private origins and credentials client-local by default. Minimize query logging.

### Client registry: client-hosted

**Recommendation.** Use a distinct durable store beside the client-hosted PRX gateway, containing:

| ID | Client-local information |
| --- | --- |
| C1 | Environment-specific inventory: resolved identity, version, registry/source, artifact integrity, direct/transitive/build/dev classification, target platform/features/markers and lockfile/SBOM evidence digest. |
| C2 | Relevant advisory records and attribution, snapshot revision, last successful synchronization, completeness/freshness and withdrawal updates. |
| C3 | Private-package intelligence, client policy, narrowly scoped expiring exceptions, approvals and decision audit records. |
| C4 | A bounded candidate cache for proposed new packages and replacement versions, separate from installed inventory. |

Inventory is scoped to workspaces/environments and targets, not everything present in a machine-wide cache. Keep multiple installed versions. A lockfile can differ from actual artifacts and can omit tools or dynamically resolved build dependencies.

A package absent from client inventory must trigger candidate lookup through the authenticated gateway, followed by graph resolution and advisory subscription. Absence is not authorization. Merely mentioning a package must not add it to installed inventory. Add it only after verified installation or observation.

### Intelligence versus artifacts

| Store | Stores | Establishes | Does not establish alone |
| --- | --- | --- | --- |
| Vulnerability registry | Advisories, affected versions, provenance, inventory relationships | Known findings under a declared snapshot/matcher | Existence, all malware, reproducibility or runtime safety |
| Package metadata index | Names, versions, dependency metadata, yanks and hashes | Source-specific existence and resolution evidence | Security approval or truth of publisher claims |
| Artifact mirror | Tarballs, wheels, crates, module archives | Controlled delivery and caching | Control of cached files, alternative sources or execution |

**Recommendation.** Start with intelligence plus selective metadata lookup. Artifact mirroring adds storage, availability, compatibility, retention and package-license redistribution obligations. It is a separate product decision. Artifact licenses, advisory-data licenses and tool-code licenses are independent.

## 3. Sources, licensing and update semantics

### Verified sources and limits

| ID | Source | Verified role | License distinction |
| --- | --- | --- | --- |
| S1 | OSV.dev [U1–U3] | Covers the four requested ecosystem identifiers. Offers version/commit queries, batches, records, ecosystem ZIP exports and modified-ID CSVs. Exports include withdrawals, including records without ecosystem under [EMPTY]. Ecosystem coverage does not mean every package/vulnerability. | Service code is Apache-2.0 [L1]. Aggregated data has source-specific terms. OSV's licensing guidance describes enrichment and redistribution, not universal relicensing [U3]. |
| S2 | GitHub Advisory Database [U4, L2] | Cross-ecosystem upstream intelligence, also encountered through OSV. Preserve GHSA/CVE aliases rather than counting them as separate vulnerabilities. | Repository project/data is CC-BY-4.0. Retain attribution, license, notices and modification indication. |
| S3 | PyPA advisory database [U5, L3] | Community PyPI advisories. README describes CVE-to-package heuristics and human triage, not exhaustive coverage. | CC-BY-4.0. Python interpreter/tool licensing does not license this database. |
| S4 | RustSec advisory-db [L4] | Rust-specific advisory input. Preserve record metadata and distinguish informational findings from vulnerabilities. | Repository code/data is CC0-1.0 except specified GHSA-derived content, which is CC-BY-4.0 with advisory license and URL metadata. Scanner/tool repositories require their own review. |
| S5 | Go vulndb [U6, L5] | Go team describes reviewed OSV records and function-level information. Govulncheck adds call-graph evidence. Go deliberately avoids universal severity labels. | Code is BSD-3-Clause; /data/ is CC-BY-4.0. LICENSE explicitly separates them. |
| S6 | OpenSSF malicious-packages [U7, L6] | OSV-format reports, with scope also admitting certain nuisance/typosquatting and ambiguous obfuscation cases. Vulnerability reports are a separate category. | Root repository license is Apache-2.0. Report-specific and imported-source data rights remain unverified. The root license does not establish rights to every linked external report. Keep ingestion behind a data-rights review. |
| S7 | OSV-Scanner [U8, L7] | Official offline scanning uses separately downloaded local databases. Missing database is an error; offline scanning does not refresh it. Useful reference, not a complete PRX distribution/auth service. | Code is Apache-2.0. Downloaded advisory data does not acquire that license. No embedding decision was made. |

**Recommendation.** Adopt OSV representation and acquisition, not an immediate fork of the full OSV service. Its repository describes a GCP deployment [U9]. A smaller ingestion/index/query service may suffice. No sizing benchmark was performed.

Installation-only service access can coexist with open advisory data, but do not restrict recipients' CC-BY rights to licensed records. Separate service access terms from data reuse rights and distribute attribution with client snapshots/exports. Counsel should approve mixed-source redistribution. Authentication does not make public upstream data proprietary.

### Normalization and provenance

**Verified.** OSV supports IDs, publication/modification/withdrawal timestamps, aliases, related records, explicit affected versions and range events [U10]. Introduction is inclusive, fixed is exclusive, last_affected is inclusive, and limit is an exclusive bound rather than a declared fix. Explicit versions and ranges are alternatives. Multiple affected entries and disjoint ranges matter.

**Recommendation.** Preserve:

| ID | Evidence |
| --- | --- |
| P1 | Raw record, source URL/revision/export digest, license and attribution, transformation version. |
| P2 | Upstream modified/published/withdrawn versus Nebuless fetch/import time and client snapshot receipt. OSV enrichment itself can update modified [U3]. |
| P3 | Identity (ecosystem, source registry, canonical name/module path), exact version/revision, original spelling, purl where applicable, artifact hash where known. |
| P4 | Alias versus related relationships; conflicting ranges and severity sources. Never invent consensus. |
| P5 | Decision snapshot, matcher version, graph digest, policy revision, exception and completeness. Minimize raw command/model retention. |

Process withdrawals, deletions and licensing corrections. Use overlapping incremental reads plus deduplication and periodic full reconciliation. A strictly-newer timestamp cursor can miss equal-timestamp records or interrupted batches. Publish only validated atomic snapshots. Partial ingestion must not appear fully fresh.

Signed snapshots/deltas, sequence numbers, expiry, rollback protection and key rotation are recommendations. Signatures prove distribution integrity, not advisory truth. Keep last-known-good data with explicit staleness. Polling cadence and expiry need measured source behavior and client policy; no freshness promise was verified.

## 4. Identity, matching and resolution

| Ecosystem | Verified semantics and consequence |
| --- | --- |
| npm | node-semver defines comparator sets, OR, prereleases and caret/tilde behavior [U11]. Preserve scopes and resolve aliases to actual identity. Dist-tags such as latest are not versions. Resolve and pin exact versions/integrities rather than comparing strings. |
| PyPI | Lowercase names and replace runs of period, underscore and hyphen with a hyphen [U12]. Use PEP 440 semantics, including epochs, prerelease/post/dev/local versions [U13]. Import names differ from distribution names. Resolve extras, markers, Python version, target wheels and build dependencies. |
| crates.io | Cargo's bare version 1.2.3 is a compatible range, while =1.2.3 is exact. Cargo has its own prerelease and pre-1.0 rules [U14]. Registry metadata distinguishes aliases from original package identity and carries checksums/yanks [U15]. Do not apply PyPI normalization to crates. |
| Go | Module paths differ from contained package/import paths. Major /v2 suffixes matter. Pseudo-versions encode revision/time with validated ordering [U16]. Resolve actual selected modules, replacements, go.work, vendoring, private sources and toolchain/stdlib records. go.sum is not an active-graph lockfile. |

**Recommendation.** Separate requested-constraint parsing, native dependency resolution and exact-version advisory membership. npm installer prerelease-range rules are not automatically OSV range rules. Use ecosystem-aware implementations and conformance tests, not regex/LLM version ordering. Unsupported GIT ranges or ambiguous identities are indeterminate, not negative results.

Advisory databases are not complete package catalogs. Independently verify existence against configured sources: npm metadata [U17], PyPI Index/JSON [U18], Cargo index [U15], Go module/proxy metadata [U16]. Record exists, not_found_at_source, unknown or unsupported_source, with observation time/source. A private-package 403, timeout or outage is not a 404. Never fall back from private to public namespaces without policy.

Nonexistent/hallucinated names, typosquats, known malicious packages and vulnerable legitimate packages are different problems. A fabricated name can later be registered. An existing package can be malicious. Similarity is a review signal, not proof. For a name-only recommendation or range spanning affected/unaffected versions, resolve a target-specific candidate graph. Do not silently assume the currently installed version was intended.

### Decision model and upgrades

**Recommendation.** Return orthogonal dimensions, not a safe boolean:

| ID | Dimension | Example values |
| --- | --- | --- |
| V1 | Intelligence | known vulnerable, malicious report, no known match, conflict |
| V2 | Coverage | complete for declared sources/snapshot, partial, stale, unavailable, unsupported |
| V3 | Resolution | exact graph, unresolved range, not found at source, ambiguous/private |
| V4 | Policy | allow, deny, review required, indeterminate |

An explicit policy may allow no-known-match, but explanations must retain that limitation. Missing severity is not low severity. A severity-only block would mishandle Go's intentionally unscored advisories [U6]. Recommend denying known affected versions on the strict MVP path with narrow expiring exceptions.

Do not rewrite an insecure version to latest. A declared fixed release may be unavailable, yanked, incompatible, affected by another advisory or introduce vulnerable transitives. Re-resolve alternatives and inspect the full target graph. Suggested wording: “Version X is outside advisory A's affected range in snapshot S. The proposed graph has no matching findings from enabled sources. Compatibility and undisclosed vulnerabilities remain unverified.” If no verified candidate exists, say so.

## 5. Actual PRX extension points

| ID | Local evidence | Verified behavior and limitation |
| --- | --- | --- |
| E1 | cmd/nebprx/main.go:2756–2785, 2835–2838, 2974–2993 | Builds request input from method/URL/headers/bounded body, calls runtime.MatchRequest and returns 403 on deny before forwarding. This gates network requests, not arbitrary local execution or future model replies. |
| E2 | cmd/nebprx/main.go:3163–3193 | WebSocket upgrades enter a separate bridge. HTTP-only interception misses this transport. |
| E3 | cmd/nebprx/main.go:3229–3310 | RoundTrip obtains response; a TeeReader captures selected bodies while forwarding. resp.Write(tc) at 3294 precedes trackLLMUsage at 3307. The buffer is observational, not a pre-delivery gate. |
| E4 | internal/config/extplugin/transform.go:30–63, 115–137; pluginsdk/transform.go:13–19; pluginsdk/sdk.go:342–393 | TransformHTTP streams and mutates the outgoing request. HTTPTransformResponse means the plugin RPC result, not the upstream response. |
| E5 | cmd/nebprx/ws.go:270–280, 330–375 | Server-to-client pump receives nil rewriter. Text observers have no deny return. Raw frames are forwarded after observation. Credential rewriting is limited to uncompressed text frames. No complete-turn response gate exists here. |
| E6 | internal/config/plugins/facets/ssh/ssh.go:15–39 | Literal command, verb and selectively buffered bounded stdin are available. Comments explicitly call command matching advisory/audit rather than a hard boundary. Denying shell requests does not prevent an exec'd shell. This is SSH traffic, not the local Bash tool. |
| E7 | cmd/nebprx/run_linux.go:194–225, 443–460, 579–587 | Namespaced or whole-machine launch ends in exec. Whole-machine path explicitly says no sandbox. The inspected launcher is not a per-descendant package-manager authorization mechanism. |
| E8 | pluginsdk/sdk.go:105–119, 601–639; pluginsdk/example/demo_https.go:59–162, 229–278 | External endpoints own protocol handling and call Conn.Evaluate with structured actions. Demo owns/mutates response writing. SetResult and ResultFields report after-the-fact results, not rejection. The demo is not a production response guard. |
| E9 | internal/config/runtime/runtime.go:94–112 | HTTPSyntheticResponder short-circuits requests before credential injection. It does not inspect upstream replies; documented errors fall through. Not an implicit fail-closed response gate. |

**Recommendation.** Place a response gate after upstream acquisition but before any client-visible response write. A purpose-built endpoint plugin can prove one protocol while reusing structured facets, Conn.Evaluate, CEL, approvals and audit plumbing. A core interception change could avoid duplication later but expands regression scope. Neither route is implemented here. Credential transforms are the wrong extension point for response inspection.

For local package actions, a new trusted broker/adapter could submit structured facts to policy. No existing general pre-exec API was verified. Keep bounded intelligence lookup outside arbitrary CEL expressions. Do not mistake after-the-fact results for enforcement.

### Network policy versus caches

**Verified:** npm has content-addressed and npx caches [U19–U20]; pip caches HTTP responses and wheels [U21]; Cargo caches registry archives/source [U22]; Go has separate module and build caches [U16]. Cached code can install/build/import/execute without a network request. Direct URLs, alternate registries, Git/path dependencies and local artifacts complicate attribution.

Network policy remains defense in depth for traffic actually routed through PRX. It cannot prove which executable runs later, block every offline install or replace artifact verification and host mediation. Host sockets, remote execution tools and writable policy files require separate controls.

## 6. Agent integrations and pre-execution design

### Verified integration mechanisms

| ID | Integration | Capability and ceiling |
| --- | --- | --- |
| I1 | Claude Code PreToolUse [U23] | hookSpecificOutput.permissionDecision = deny prevents a tool call and can return a reason. MCP tools can be matched. Recommend a deterministic synchronous command adapter to the client-local broker, covering all execution tools, not only Bash. Optional best-effort command filters are not the security boundary. |
| I2 | Claude hook failure handling [U23] | HTTP non-2xx and connection failures are documented non-blocking. Invalid JSON schemas can also be non-blocking. A policy endpoint returning 403 does not itself deny execution. Adapter must convert denial/outage/malformed responses into a valid blocking result; pin and test actual client failure/timeout behavior. |
| I3 | Codex rules [U24] | Experimental prefix rules control commands outside the sandbox with allow/prompt/forbidden; restrictive matching wins. Useful to forbid raw-manager escalation, but not dependency resolution or proof of coverage inside the sandbox/child processes. No equivalent dynamic package pre-exec callback was verified. |
| I4 | PRX-owned structured execution tool | Recommended design, not an existing capability claim. Prefer “install this approved graph” to unrestricted shell parsing. The server owns authorization and execution. An optional package-check tool an agent may skip is advisory only. |

Claude's current docs describe MessageDisplay transformations, interactive line batching and some whole-message non-interactive delivery [U23]. Display-only scope and non-blocking failure semantics do not protect transcripts, tool dispatch or network replies. Compatibility was not tested. PostToolUse/Stop hooks cannot undo execution or retract delivered text.

### Recommended trusted broker flow

A protected hook/wrapper helps cooperative agents. For adversarial agents with arbitrary shell access, the execution boundary must be outside their control. A PATH shim can be bypassed by absolute paths, another interpreter, nested shell, alternate tool or modification of the shim. Exact OS isolation is a separate design task, not assumed solved by PRX network namespaces.

1. **A1: Capture.** Validate executable identity, argv, cwd, user, target/runtime, manager configuration/environment and workspace/lockfile digests without evaluating input. Reject unsupported shell constructs or require review rather than guessing through eval, substitution or downloaded scripts.
2. **A2: Resolve.** Obtain exact selected graph using approved metadata/resolvers in an isolated environment. Bound destinations, size, recursion and time. Resolution can itself execute code, notably Python source metadata hooks [U25].
3. **A3: Decide locally.** Check all direct/transitive/build/executable dependencies against the client registry. Fetch missing subsets via the installation-authenticated service. Evaluate source, existence, findings, freshness and exceptions. Incomplete input is not empty success.
4. **A4: Bind approval.** Use short-lived single-use authorization bound to graph, artifact hashes, manifests/lockfiles, target, manager version, policy and snapshot. Approval of a name does not authorize later versions.
5. **A5: Execute exact inputs.** Broker owns resolution through installation. Use immutable staging/read-only inputs, frozen modes, verified artifacts and constrained configuration. Rechecking hashes is insufficient if files remain agent-writable after checking. Prevent symlink swaps and concurrent mutation.
6. **A6: Verify.** Compare installed graph/artifacts to authorization and update inventory. Mismatch is failure. Do not automatically delete user state. Newly published advisories affect future approvals even for unchanged artifact bytes.

Policy, hooks and broker credentials must be protected from the agent. Reject policy-disabling flags, source overrides and alternate execution routes on the strict path. A strong claim requires a managed sandbox restricting package activation and escape routes, not merely command string recognition.

### Manager-specific execution limits

| ID | Manager | Narrow viable path and remaining controls |
| --- | --- | --- |
| M1 | npm | Existing reviewed lockfile plus npm ci --ignore-scripts, pinned manager/config and verified selected graph/source/integrity. ci errors on manifest disagreement and does not rewrite locks [U26]. ignore-scripts does not stop explicit npm run/test from executing intended scripts. Gate npx/exec, global installs, rebuild, alternate managers and Git/path inputs separately. Current v11 docs include script allow policies; pin/test before relying on them. |
| M2 | pip | Exact fully transitive hashed requirements, wheels only: --require-hashes and --only-binary :all: [U27]. Dry-run/report is useful but not a no-code-execution guarantee; source/local metadata can invoke a build backend [U25]. Reject or separately isolate sdists, editable/VCS/local installs. Include build dependencies and alternate interpreters/installers. |
| M3 | Cargo | Check Cargo.lock plus source, target/features and build dependencies before building. Locked prevents lock changes; frozen adds offline [U28]. Neither scans vulnerabilities. build.rs executes before building [U29]; procedural macros also require execution policy. No universal npm-ignore-scripts equivalent was established. Restrict Git/path/patch/alternate registries initially. |
| M4 | Go | Inspect actual selected modules, go.mod/go.work, replacements, checksums, vendor and toolchain, not every historical go.sum entry. go mod verify is integrity evidence, not vulnerability authorization [U16]. Gate run/install/test/generate separately. Do not characterize module download as npm-style lifecycle scripts. Builds/toolchains/cgo and produced code have separate risks. |

Maliciousness, known vulnerabilities, license policy, provenance and script approval are independent. A valid hash establishes identity, not benign behavior. Provenance is not proof of safety. Yanked/deprecated/unmaintained status is not automatically a vulnerability, though client policy can prohibit it.

## 7. Gating model replies before delivery

### Extraction and meaning

**Recommendation.** Parse provider envelopes before content. Extract assistant-visible text and fully assembled structured tool arguments, including commands writing manifests or launching package tools. Preserve source spans and context. Treat provider/model content as untrusted data, never instructions for the checker.

Use deterministic parsers for supported install syntax, manifests and explicit package/version references. Regex can locate candidates but cannot reliably establish shell semantics, aliases, negation, confusables or whether a quoted command is recommended. An LLM may suggest candidate spans or review reasons, but must not establish version ordering, authoritative existence, final authorization or a safe label.

Distinguish “install X@Y” from “do not install X@Y,” advisory explanations, quoted attack examples and remediation diffs removing vulnerable versions. Arbitrary prose cannot be classified perfectly. Recommended policy: block confidently identified actionable disallowed references, review ambiguous actionable output in strict mode, and allow contextual security discussion when its meaning is established. Prohibiting any appearance of a vulnerable package name would be a different policy that also obstructs remediation.

The command gate remains necessary. An allowed recommendation can produce different execution through shell expansion, changed files, tool defaults or dependency resolution. Conversely, execution gating alone does not stop a user reading and manually following an insecure recommendation.

### Transport requirements

| ID | Transport | Before-release behavior | Block/failure outcome |
| --- | --- | --- | --- |
| T1 | Non-streaming HTTP JSON | Read bounded complete response before client headers/body. Decode supported compression with separate compressed/decompressed limits. Validate schema and every relevant output/tool call. | Return a documented provider-adapter-specific error envelope, status, stable PRX policy code and decision ID. Never raw fragments or a refusal falsely attributed to the model. Recompute encoding/length if modifying content. |
| T2 | SSE | Parse SSE framing, UTF-8, multi-line data and provider events. Assemble text/tool arguments by response/item/block/call identity. For strict prevention, hold the whole supported response/turn and replay allowed events in valid order. | Prefer withholding headers until verdict where timeout constraints permit. After headers commit, HTTP 403 is impossible. Use a tested protocol-valid error/termination sequence or close. Never fabricate a successfully completed tool call. |
| T3 | WebSocket | Separate handshake from message/turn policy. Handle continuation frames, control frames, limits, negotiated compression and application multiplexing. A text frame is not a complete message/turn. | Before upgrade, HTTP denial is possible. After 101, use a tested application error and/or policy close, such as close code 1008 [U33], with bounded non-secret reason. Never send HTTP bytes into the upgraded stream. Reject unsupported modes on the strict path. |

**Verified upstream basis:** OpenAI documents typed SSE events and a separate WebSocket mode [U30–U31]. Anthropic tool input arrives in partial JSON deltas and is accumulated to content-block completion [U32]. These invalidate per-chunk regex enforcement. They do not prove all providers share a schema.

**Partial-output leakage:** delivering an install-command prefix and inspecting the rest later is irreversible. A rolling buffer cannot guarantee that later context will not complete an insecure recommendation. A released tool block might execute before later blocks alter the context. Full-turn buffering sacrifices latency, adds memory/storage pressure and can exceed SDK timeouts. Measure these costs before claiming strict streaming support.

Bound bytes, decoded size, item count, duration and concurrency. Incomplete JSON/SSE, unknown content-bearing events, malformed tool arguments, decode errors, timeout or overflow must be indeterminate/block on the strict path, not approval of the scanned prefix. Discard buffered content securely and avoid retaining raw replies by default. Buffering does not stop provider-hosted tools already executing upstream; those require provider-side permissions or exclusion.

Do not silently rewrite replies or upgrade versions in place. Return clear policy evidence and request a new proposal. If an adapter synthesizes a refusal later, identify it as PRX-generated and test client state, usage, IDs, retries and conversation continuation.

## 8. Offline and degraded behavior

| ID | Condition | Recommended strict behavior |
| --- | --- | --- |
| O1 | Fresh local snapshot and complete supported graph | Decide locally without Internet. |
| O2 | Known affected/malicious finding in last-known-good data | Keep denying unless a valid exception applies. An outage does not erase evidence. |
| O3 | Snapshot older than client policy permits | Deny or require explicit review. Report age and completeness. Stale negatives are not fresh results. |
| O4 | New package/version absent locally while offline | Indeterminate. No automatic allowlist insertion. Pre-approved exact graph/artifact bundles can support offline use. |
| O5 | Missing/corrupt data, invalid signature, rollback or partial synchronization | Retain valid last-known-good data but never claim completeness. Fail closed on required checks. |
| O6 | Private package unavailable globally | Use client-local authoritative metadata/policy. Do not leak names or declare nonexistence. |

Intelligence offline mode differs from package-manager offline mode. Advisories may exist without artifacts and cached artifacts may exist without current intelligence. OSV-Scanner explicitly requires separately downloaded databases [U8]. Expiry windows, review authority and availability exceptions are product decisions. Any stale-data exception needs bounded expiry and an audit event.

## 9. Scoped MVP and exclusions

| ID | Proposed slice | Acceptance boundary |
| --- | --- | --- |
| MVP1 | Global four-ecosystem advisory ingestion/index plus client-local inventory-aware registry | License allowlist, attribution, withdrawals, deterministic matching, atomic snapshots, installation auth and explicit unknown/stale states. No artifact mirror. |
| MVP2 | One managed npm execution path | Pinned manager, reviewed lockfile, approved sources, graph/integrity verification, scripts disabled, broker ownership and denied raw bypass paths within a documented sandbox. Unsupported commands are rejected or reviewed, not labeled scanned. |
| MVP3 | One bounded non-streaming provider adapter | Explicit actionable package/version references and structured install calls checked before release. Extraction uncertainty recorded. No perfect-language claim. Streaming/WS must not silently pass through an endpoint advertised as strict. |

Acceptance must demonstrate offline/cache-hit enforcement, candidate lookup for packages missing from inventory, zero denied-content delivery and visible uncertainty. If only MVP1 ships, describe it as intelligence/assessment, not command/reply prevention.

**Initial exclusions:** unrestricted shell interpretation; hostile-host security without OS-enforced mediation; every agent/tool; arbitrary private registries; Git/path/editable/source-build installation; automatic upgrades; runtime malware detection; universal reachability analysis; platform parity; artifact hosting; every model provider; SSE/WebSocket reply prevention; provider-hosted tool execution; image/binary/encoded recommendations; universal hallucination/malware prevention. Exclusions require explicit rejection or observation-only labeling, not quiet allow-through in strict mode.

**Q1:** Which client environment and agent versions are first, and can policy/broker remain outside agent-writable storage?

**Q2:** Is complete-response buffering acceptable? Should unsupported transports be denied or explicitly observation-only?

**Q3:** What freshness, offline exception and approval policy does each client accept?

**Q4:** May public candidate names leave the client, or must it download broader snapshots? Who approves redistribution and private-source contracts?

## 10. Adversarial acceptance matrix

These are proposed tests, not executed tests. Use synthetic fixtures, never real malware installations.

| ID | Case | Required result |
| --- | --- | --- |
| X1 | Affected exact version and fixed boundary | Inclusive introduction/exclusive fix matches OSV. |
| X2 | last_affected, limit, explicit versions, disjoint ranges, reintroduction | Correct unions; limit never described as a fix. |
| X3 | npm prerelease, OR/caret/tilde, scoped alias, moving dist-tag | Native resolution separate from advisory matching; exact authorized artifact cannot drift. |
| X4 | PyPI normalized aliases, epochs/post/dev/local versions, extras/markers | Correct identity and target graph; import name not assumed distribution. |
| X5 | Cargo compatible versus exact version, alias, prerelease, build dependency | Correct identity/range; build dependency included. |
| X6 | Go module/import distinction, major suffix, pseudo-version, replace/workspace/vendor | Correct effective source/version; local replacement not falsely cleared against upstream. |
| X7 | Advisory miss, registry 404, private 403, timeout | Separate states; none means safe. |
| X8 | New candidate absent from client inventory | Authenticated lookup or offline indeterminate. |
| X9 | Clean direct package with vulnerable transitive | Denied with path and exact versions. |
| X10 | Stale snapshot, withdrawal, alias duplicate, conflict | Staleness explicit; withdrawal propagated; aliases not double-counted; conflict retained. |
| X11 | Interrupted import, equal timestamps, replay/invalid signature | No false completeness, skipped update or accepted rollback. |
| X12 | Missing/revoked credential, wrong tenant, direct client access | Global service denies unauthorized requests. |
| X13 | Cached install/build with no network | Execution gate still checks; proxy-only design cannot claim this coverage. |
| X14 | Absolute path, alternate interpreter, nested shell/eval, downloaded script, alternate MCP executor | Bypass blocked or unsupported action explicitly rejected. |
| X15 | Lock/config/symlink/artifact swap after approval, concurrent installs | Only immutable approved inputs execute; no token replay. |
| X16 | npm script/explicit run, Python metadata backend, Cargo build/proc macro | No unreviewed code runs during checking; separate rejection/isolation policy. |
| X17 | Hashed/signed but malicious artifact | Integrity does not override denial. |
| X18 | HTTP hook 403, malformed JSON, timeout, crash, disabled/rewritten hook | Required path does not accidentally authorize. Test pinned client, not only mock service. |
| X19 | Disallowed reference after compressed prefix or near size limit | Zero forbidden bytes delivered; overflow never approves prefix. |
| X20 | SSE splits name/version/UTF-8/escape/tool JSON arbitrarily | Correct assembly independent of chunks, across all call IDs. |
| X21 | Multiple tool calls, early runnable call, missing completion | No executable call released before strict verdict; incomplete stream is not success. |
| X22 | WS continuation, compression/context takeover, control frames, multiplexing | No frame-only bypass; unsupported mode rejected; valid close. |
| X23 | Negation, quoted exploit, remediation diff, ambiguous prose | No blanket name blacklist; explicit review policy and measured error rates. |
| X24 | Prompt injection/fabricated advisory/malicious registry description | Untrusted content cannot change policy, invoke checker tools or forge provenance. |
| X25 | Proposed fix unavailable/yanked/incompatible or vulnerable transitives | No automatic secure-upgrade assertion; graph rechecked. |
| X26 | Retry/reconnect, IDs, prior leaked prefix, SDK error handling | No fabricated success or retry bypass; no claim to retract delivered content. |
| X27 | Private name/credential in query/log | Tenant isolation and redaction; private data local by default. |

## 11. Remaining evidence gaps

**R1: Documentation is not runtime verification.** No pinned agent/SDK version, sandbox, wrapper, response adapter or timeout behavior was tested. Client-specific fail-closed claims require integration evidence.

**R2: Licensing is source-scoped.** OSV software licensing does not cover its heterogeneous records. OpenSSF imported-report rights and artifact redistribution need further review. Do not ship unfiltered mixed data under a blanket proprietary or Apache label.

**R3: Coverage/freshness/performance are unmeasured.** No corpus counts, feed lag, coverage percentages, availability agreement or latency benchmarks were established. OSV currently documents no API rate limits [U2]; that is not a service agreement or reason to omit backoff/quotas.

**R4: Execution integrity needs environmental control.** A hook/network proxy cannot guarantee approved-only execution with unrestricted host access. A protected broker and tested isolation are prerequisites for the stronger claim.

**R5: Reply prevention is protocol- and meaning-limited.** Current PRX observation buffers do not provide it. Full buffering fixes delivery timing for supported protocols, not perfect prose interpretation or upstream tool execution.

## 12. Primary source register

Sources were directly consulted during this investigation, in full or relevant sections. They are live URLs. Failed guessed paths were not used as evidence. The unsuccessful OSV data/new/ path was replaced by the working data/new URL. OSV-Scanner offline guidance was read from its repository after documentation-path failures.

### Advisory systems

- **U1:** OSV sources/exports: <https://google.github.io/osv.dev/data/>
- **U2:** OSV API: <https://google.github.io/osv.dev/api/>
- **U3:** OSV source licensing/enrichment: <https://google.github.io/osv.dev/data/new>
- **U4:** GitHub Advisory Database: <https://raw.githubusercontent.com/github/advisory-database/main/README.md>
- **U5:** PyPA advisory database: <https://raw.githubusercontent.com/pypa/advisory-database/main/README.md>
- **U6:** Go vulnerability management: <https://go.dev/doc/security/vuln/>
- **U7:** OpenSSF scope: <https://raw.githubusercontent.com/ossf/malicious-packages/main/README.md>; contributions: <https://raw.githubusercontent.com/ossf/malicious-packages/main/CONTRIBUTING.md>
- **U8:** OSV-Scanner offline mode: <https://raw.githubusercontent.com/google/osv-scanner/main/docs/offline-mode.md>
- **U9:** OSV service architecture: <https://raw.githubusercontent.com/google/osv.dev/master/README.md>
- **U10:** OSV schema/range evaluation: <https://ossf.github.io/osv-schema/>

### Package identity, resolution and execution

- **U11:** npm semver: <https://raw.githubusercontent.com/npm/node-semver/main/README.md>
- **U12:** Python name normalization: <https://packaging.python.org/en/latest/specifications/name-normalization/>
- **U13:** Python version specifiers: <https://packaging.python.org/en/latest/specifications/version-specifiers/>
- **U14:** Cargo requirements: <https://doc.rust-lang.org/cargo/reference/specifying-dependencies.html>
- **U15:** Cargo index: <https://doc.rust-lang.org/cargo/reference/registry-index.html>
- **U16:** Go modules, paths, pseudo-versions, caches and verification: <https://go.dev/ref/mod>
- **U17:** npm metadata lookup: <https://docs.npmjs.com/cli/v11/commands/npm-view/>
- **U18:** PyPI JSON API: <https://docs.pypi.org/api/json/>
- **U19:** npm cache: <https://docs.npmjs.com/cli/v11/commands/npm-cache/>
- **U20:** npm exec/npx: <https://docs.npmjs.com/cli/v11/commands/npm-exec/>
- **U21:** pip cache: <https://pip.pypa.io/en/stable/topics/caching/>
- **U22:** Cargo home/cache: <https://doc.rust-lang.org/cargo/guide/cargo-home.html>
- **U25:** pip installation/resolution: <https://pip.pypa.io/en/stable/cli/pip_install/>
- **U26:** npm ci/scripts: <https://docs.npmjs.com/cli/v11/commands/npm-ci/>
- **U27:** pip secure installs: <https://pip.pypa.io/en/stable/topics/secure-installs/>
- **U28:** Cargo build flags: <https://doc.rust-lang.org/cargo/commands/cargo-build.html>
- **U29:** Cargo build scripts: <https://doc.rust-lang.org/cargo/reference/build-scripts.html>

### Agent and model protocols

- **U23:** Claude Code hooks, decisions and HTTP failure handling: <https://code.claude.com/docs/en/hooks>
- **U24:** Codex rules: <https://developers.openai.com/codex/rules> (redirected to <https://learn.chatgpt.com/docs/agent-configuration/rules>).
- **U30:** OpenAI HTTP streaming: <https://developers.openai.com/api/docs/guides/streaming-responses>
- **U31:** OpenAI WebSocket mode: <https://developers.openai.com/api/docs/guides/websocket-mode>
- **U32:** Anthropic tool-input streaming: <https://platform.claude.com/docs/en/build-with-claude/streaming>
- **U33:** WebSocket framing and close codes: <https://www.rfc-editor.org/rfc/rfc6455.html#section-7.4.1>

### License evidence

- **L1:** OSV.dev code, Apache-2.0: <https://raw.githubusercontent.com/google/osv.dev/master/LICENSE>
- **L2:** GitHub advisory database, CC-BY-4.0: <https://raw.githubusercontent.com/github/advisory-database/main/LICENSE.md>
- **L3:** PyPA advisory database, CC-BY-4.0: <https://raw.githubusercontent.com/pypa/advisory-database/main/LICENSE>
- **L4:** RustSec code/data CC0 with GHSA exception: <https://raw.githubusercontent.com/RustSec/advisory-db/main/LICENSE.txt>
- **L5:** Go vulndb code BSD-3-Clause, data CC-BY-4.0: <https://raw.githubusercontent.com/golang/vulndb/master/LICENSE>
- **L6:** OpenSSF malicious-packages root license: <https://raw.githubusercontent.com/ossf/malicious-packages/main/LICENSE>
- **L7:** OSV-Scanner code, Apache-2.0: <https://raw.githubusercontent.com/google/osv-scanner/main/LICENSE>
