# PRX package vulnerability intelligence research

**Research date:** 2026-10-26 (local date)
**Scope:** Distinct Nebuless global aggregation and client-side inventory registry for npm, PyPI/Python, crates.io and Go. This is vulnerability intelligence, not an artifact mirror.

## Recommendation

Use OSV.dev dumps and OSV schema as the global baseline. Ingest per-ecosystem ZIPs and modified_id.csv, retaining raw advisory, IDs/aliases, source, license, modified time and withdrawal state. Add Go's canonical vuln.go.dev feed as direct cross-check/fallback for Go-specific review status and import/symbol metadata. RustSec and PyPA publish OSV-compatible records; reconcile them as upstream sources rather than double-counting them by default. Use GitHub Advisory Database as secondary gap/quality cross-check, with alias deduplication and CC-BY attribution.

Nebuless global service should serve only authenticated PRX installations. Client-hosted PRX should maintain a separate durable, inventory-specific intelligence registry synchronized from signed/versioned global snapshots. Inventory is an optimization, not an allowlist: agent-named new packages absent from the current SBOM must still be checked. A valid complete check can return no-known-match; incomplete checks return indeterminate. Neither establishes safety. In offline mode return last-verified data with timestamp and explicit freshness/coverage state. Keep artifacts/downloads separate.

## Two registries, two roles

| | Nebuless global | Client-side |
|---|---|---|
| Corpus | Shared multi-ecosystem advisories and provenance | Durable package-wide advisory subsets, local inventory/graphs and a separate candidate admission set |
| Access | Authenticated PRX installations only; no public direct-client lookup | Client-hosted gateway; authenticated sync, local decisions |
| Authority | Source feeds normalized to OSV; retain source assertions | Global feed plus local package inventory, not separate hand-maintained truth |
| Offline | Publish consistent snapshots/deltas | Last-known-good snapshot; show freshness and degraded status |
| Artifact mirror | No | No |

Return distinct outcomes: vulnerable, no-known-match, unknown-package, invalid/unresolved-version, unsupported-ecosystem, stale-feed, unavailable. Include matched source, advisory IDs, affected range, fix boundary if supplied, withdrawn state and snapshot timestamp. Unpinned ranges, VCS/local path dependencies, aliases and unsupported version schemes can be indeterminate. A block is PRX policy, not a property of the feed. A package name/version absent from inventory must trigger lookup, not implicit allowance.

## Source comparison

### OSV.dev: primary aggregation

OSV schema describes package coordinates and version/commit ranges, aliases, references and withdrawn records. OSV.dev currently lists npm, PyPI, crates.io and Go among supported ecosystems. Its data page documents full and per-ecosystem ZIPs, individual records, top-level and per-ecosystem modified_id.csv, and inclusion of withdrawn entries (some in an empty-ecosystem directory). Incremental CSV entries list modification time and record ID. Periodic full reconciliation is still needed to catch missed updates and deletions.

Caveats: OSV describes coverage as ongoing, not exhaustive or SLA-backed. It converts external feeds, including NVD CVEs for open-source software. Thus duplicate aliases and overlapping ranges are expected. Preserve records and provenance; deduplicate for decisions, not by deleting source assertions. Ecosystem presence is not coverage proof. OSV.dev's own implementation is open source but its documented deployed architecture uses GCP; consuming dumps into a simpler PRX store is less operationally heavy than self-hosting its full service.

Data license is not the schema/code license. OSV's current [data-source catalog](https://google.github.io/osv.dev/data/) identifies GHSA, PyPI and Go data as CC-BY-4.0, RustSec as CC0-1.0, and OpenSSF Malicious Packages as Apache-2.0. Record-specific exceptions still matter: RustSec documents imported GHSA records under CC-BY-4.0. Preserve source license, attribution, URLs and modifications. Do not apply one license to the entire OSV aggregate; its wider catalog also contains differently restricted sources. Review the selected source terms before redistribution; this report does not provide a legal determination.

### GitHub Advisory Database

GitHub publishes OSV-format advisories in a repository described as free/open source and CC-BY 4.0, plus REST endpoints. It supports npm, pip/PyPI, Go and Rust among other ecosystems. It contains CVEs and GitHub-originated records and imports upstream data, causing overlap with OSV, RustSec and PyPA. Use for reconciliation and gaps, not unfiltered duplicate bulk ingestion. Preserve CC-BY attribution; API availability does not itself settle redistribution rights.

### Go Vulnerability Database

Canonical vuln.go.dev serves OSV records via static-file-friendly API endpoints, indexes, per-ID records and a bulk ZIP. It adds Go-specific import paths, vulnerable symbols, OS/architecture applicability, aliases and REVIEWED/UNREVIEWED status. Go docs warn that automated unreviewed reports may be limited and unusual version schemes may be conservatively classified, causing false positives. Some excluded/withdrawn records are deliberately not served. Do not consume golang/vulndb internal YAML as a stable feed: upstream explicitly warns its format may change. Use vuln.go.dev as Go cross-check/fallback. The [upstream README](https://github.com/golang/vulndb) explicitly distinguishes BSD-style Go code from CC-BY-4.0 database entries.

### RustSec and PyPA

RustSec advisory-db has TOML advisories with crate, ranges, withdrawal date, aliases and references, and exports OSV in real time. Its README says default records are CC0-1.0, with imported GHSA records marked CC-BY-4.0 and source URL required. GitHub Advisory Database imports RustSec. Reconcile withdrawals and preserve record-specific license.

PyPA's community Python Packaging Advisory Database uses YAML validated against OSV schema; its README says much data is derived from NVD with heuristic CVE-to-package/version matching plus human triage. The repo describes OSV exposure and pip-audit use. It is CC-BY-4.0; confirm at ingest. Python distribution and import names differ, so use ecosystem package identity, and only apply import-level conditions when explicitly supplied. PyPA and OSV copies overlap.

### OpenSSF Malicious Packages

Separate from vulnerability advisories. The repository publishes malicious-package reports in OSV format and includes account takeover, malicious install payloads and dependency/manifest confusion. Its [README](https://github.com/ossf/malicious-packages/blob/main/README.md) also permits certain borderline spam/typosquatting reports even when malicious behavior is not established; benign telemetry alone is not malware. Consequently, preserve report classification and evidence rather than treating every MAL identifier as identical certainty. Process whole-record withdrawals and per-version false-positive corrections. The [repository license](https://github.com/ossf/malicious-packages/blob/main/LICENSE) and OSV catalog identify Apache-2.0; preserve notices and source provenance.

### CVE/NVD, KEV and EPSS

CVE/NVD offer broad CVE/product records, but CPE/product identifiers commonly do not map directly to npm/PyPI/crate/module names and semver ranges. OSV already converts some NVD CVEs, so direct ingestion risks duplicates/conflicting mapping. Use NVD for aliases/references/enrichment, not primary package-version matching. Official NIST docs could not be fetched in this research, so current API rate limits, update cadence and terms remain unverified.

CISA KEV is a curated known-exploited CVE catalog with CSV/JSON/schema and license page. It enriches matching CVEs, not package matching. FIRST EPSS estimates probability of CVE exploitation in the next 30 days and publishes daily scores. EPSS is probability, not severity or proof of exploitation. Both require reliable CVE aliases; no match means no enrichment, not safety. Root verification found the current [EPSS data guide](https://www.first.org/epss/data): bulk synchronization should use the daily compressed CSV or historical repository, not repeated API queries. The [current API documentation](https://api.first.org/epss/) specifies https://api.first.org/data/v1/epss for small lookups. FIRST's [FAQ](https://www.first.org/epss/faq) describes free access and requests attribution in products; confirm redistribution terms for the intended packaging.

## Matching tools

| Tool | Fit and caveat |
|---|---|
| OSV-Scanner / OSV-Scalibr | Best initial reference for lockfile/SBOM extraction across many package managers. Supports offline scanning after DB download and some call analysis. Documented defaults contact OSV.dev, deps.dev and sometimes registries, sending package coordinates; use offline/local data or own endpoint for privacy. Verify exact library API/license before embedding. |
| Native OSV matching | Smallest common version-range matcher and local snapshot path. Test ecosystem semantics and withdrawn records; no result is not safe. |
| govulncheck | Strong Go-specific source/call analysis when source/build graph exists. Gateway inventory alone cannot claim vulnerable-symbol reachability. |
| pip-audit, cargo-audit | Useful ecosystem-specific validation and reconciliation; not four-ecosystem unified runtime services. |
| Trivy | Broad repository, image, SBOM and OS/language scanner. Consider only if PRX needs this wider scope; verify database sources, offline update and data redistribution separately. |
| Grype + Syft | Credible FOSS SBOM/container pipeline, supports JS/Python/Go/Rust and KEV/EPSS prioritization. Grype code is Apache-2.0 per README; transformed DB's upstream data rights remain separate. |
| VulnCheck | Commercial intelligence product; do not infer OSS code or free redistribution from free API/community access. Not selected as primary source. |

Scanner code license, data ingestion code, advisory payload license and hosted API terms are separate. No scanner's OSS license grants rights to redistribute upstream data.

## Suggested ingestion

1. Bootstrap from OSV full or per-ecosystem ZIPs. Store raw record/digest, IDs, aliases, ecosystem/package, source URL, license, modified and ingested timestamps, withdrawal state and snapshot ID. Validate against upstream schema.
2. Poll modified_id.csv, fetch and atomically upsert changed records including withdrawal tombstones. Persist cursor, retry safely, periodically reconcile full dumps. Publish immutable complete snapshots or versioned deltas.
3. Reconcile Go canonical DB, RustSec, PyPA and GHSA by IDs, aliases, package and ranges. Keep distinct source assertions; flag disagreement, unknown license, stale data, invalid schema and withdrawal conflicts.
4. Ingest KEV/EPSS as separate dated enrichment keyed by CVE alias. Keep malicious package records separate.
5. Authenticate global API/snapshot access; scope/rate-limit per installation. Sign snapshots, expose snapshot ID and health. Do not leak tenant inventory across clients.
6. Build client inventory from lockfiles/SBOM with dependency edges/depth and direct/transitive status. Query new recommendations even absent from inventory. Offline results carry age; beyond policy freshness threshold they are unknown/degraded.
7. Log normalized query, match method, source snapshot, range, decision and freshness. Minimize inventory telemetry. Blocks cite source/range; uncertainty is explicit.
8. Maintain a live source/license register. Review terms, caching, attribution, derivative and commercial redistribution rights before redistribution. If unclear, do not silently republish.

## Ranked choices

1. OSV dumps + modified_id.csv as primary global baseline for four ecosystems.
2. vuln.go.dev as direct Go fallback/cross-check for reviewed state and symbol applicability.
3. RustSec and PyPA as source-level reconciliation, selective gap coverage only; avoid duplicates.
4. GHSA as secondary coverage cross-check with CC-BY attribution.
5. OpenSSF Malicious Packages as separately licensed/policy-reviewed malicious-package feed.
6. KEV then EPSS as dated enrichment, never primary matching.
7. OSV-Scanner/Scalibr plus local OSV matcher for client inventory; compare Trivy/Grype only if SBOM/image breadth warrants complexity.

## Unresolved evidence

- Audit record-level license exceptions and attribution fields when implementing the selected feed adapters. Go data's CC-BY-4.0 and OpenSSF's Apache-2.0 are now verified from primary sources above.
- Trivy/Grype DB redistribution rights remain unverified; neither database is required by the recommended MVP.
- NVD current limits, update feeds and terms were not verified because official NIST pages failed to load.
- EPSS's current endpoint and bulk-download guidance are verified above; review its packaging/redistribution terms before enabling enrichment distribution.
- No coverage benchmark was run. Test representative PRX lockfiles for Go replacements/VCS dependencies, Python markers, npm aliases/workspaces and Cargo target/features.
- Authentication protocol, tenant privacy, retention and snapshot-signing design require product/security decisions.

## Primary sources

1. OSV sources, coverage, dumps and deltas: https://google.github.io/osv.dev/data/
2. OSV schema: https://github.com/ossf/osv-schema
3. OSV schema specification: https://ossf.github.io/osv-schema/
4. OSV implementation/GCP architecture: https://github.com/google/osv.dev
5. OSV data licensing issue (subset/history): https://github.com/google/osv.dev/issues/319
6. OSV-Scanner, offline mode and network behavior: https://github.com/google/osv-scanner
7. GitHub Advisory Database/license: https://github.com/github/advisory-database
8. GitHub Advisory Database overview: https://docs.github.com/en/code-security/concepts/vulnerability-reporting-and-management/github-advisory-database
9. GitHub security advisories API: https://docs.github.com/en/rest/security-advisories
10. Go database API/schema/ZIP/caveats: https://go.dev/security/vuln/database
11. Go vulnerability management: https://go.dev/security/vuln
12. Go internal feed warning: https://github.com/golang/vulndb
13. RustSec advisory DB and license exceptions: https://github.com/RustSec/advisory-db
14. RustSec advisories: https://rustsec.org/advisories/
15. PyPA advisory database: https://github.com/pypa/advisory-database
16. OpenSSF malicious packages scope: https://github.com/ossf/malicious-packages
17. NVD developer portal (not verified): https://nvd.nist.gov/developers
18. CISA KEV catalog/schema/license: https://www.cisa.gov/known-exploited-vulnerabilities-catalog
19. FIRST EPSS overview: https://www.first.org/epss/
20. FIRST EPSS API (current): https://api.first.org/epss/
21. Trivy: https://github.com/aquasecurity/trivy
22. Grype and Apache-2.0 claim: https://github.com/anchore/grype
23. pip-audit: https://github.com/pypa/pip-audit
24. govulncheck docs: https://go.dev/doc/security/vuln/
25. FIRST EPSS bulk data: https://www.first.org/epss/data
26. FIRST EPSS FAQ and attribution: https://www.first.org/epss/faq
27. OpenSSF repository license: https://github.com/ossf/malicious-packages/blob/main/LICENSE
