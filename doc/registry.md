# PRX vulnerability intelligence registries

`nebprx registry --mode server|client` runs two distinct services. Neither is an
artifact mirror. The server aggregates public OSV advisories. The client stores
inventory, exact versions, local decisions, candidate subscriptions and verified
package-wide advisory subsets. This release does not block commands, package
execution, model replies or downloads. An advisory miss is never proof of safety.

## Quick start: synthetic offline data

Build using the repository's supported Go toolchain. All commands below run from
the repository root. Use a new state directory. State files and inventories must
be readable/writable only by their owner. Shell examples use Python 3 only to
extract the public key, not to call registry APIs.

```sh
go build -o /tmp/nebprx ./cmd/nebprx
umask 077
mkdir -p /tmp/prx-demo
chmod 700 /tmp/prx-demo
/tmp/nebprx registry --mode server --state-dir /tmp/prx-demo/server \
  --action init > /tmp/prx-demo/server-init.json
python3 -c 'import json; print(json.load(open("/tmp/prx-demo/server-init.json"))["public_key"])' \
  > /tmp/prx-demo/server.pub
/tmp/nebprx registry --mode server --state-dir /tmp/prx-demo/server \
  --action enroll --installation demo --output /tmp/prx-demo/demo.token

for ecosystem in npm PyPI crates.io Go; do
  input=examples/registry/empty.json
  [ "$ecosystem" != npm ] || input=examples/registry/npm.json
  /tmp/nebprx registry --mode server --state-dir /tmp/prx-demo/server \
    --action import --ecosystem "$ecosystem" --input "$input" \
    --observed-at "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
done
/tmp/nebprx registry --mode server --state-dir /tmp/prx-demo/server \
  --listen 127.0.0.1:8091 --allow-loopback-http
```

The example advisory and alias are synthetic, not actual GitHub/CVE records.
Importing `empty.json` asserts a deliberately empty synthetic source. Never use
it to represent a production ecosystem export. Stop with SIGINT/SIGTERM.

In another terminal:

```sh
umask 077
install -m 600 examples/registry/inventory.json /tmp/prx-demo/inventory.json
/tmp/nebprx registry --mode client --state-dir /tmp/prx-demo/client --action init
/tmp/nebprx registry --mode client --state-dir /tmp/prx-demo/client \
  --action inventory --input /tmp/prx-demo/inventory.json
/tmp/nebprx registry --mode client --state-dir /tmp/prx-demo/client \
  --action sync --upstream http://127.0.0.1:8091 --installation demo \
  --token-file /tmp/prx-demo/demo.token --trust-key /tmp/prx-demo/server.pub \
  --allow-loopback-http
/tmp/nebprx registry --mode client --state-dir /tmp/prx-demo/client \
  --action check --ecosystem npm --name prx-synthetic-demo \
  --package-version 1.0.0 --public
/tmp/nebprx registry --mode client --state-dir /tmp/prx-demo/client --action status
/tmp/nebprx registry --mode client --state-dir /tmp/prx-demo/client \
  --listen 127.0.0.1:8092 --allow-loopback-http \
  --inventory /tmp/prx-demo/inventory.json \
  --upstream http://127.0.0.1:8091 --installation demo \
  --token-file /tmp/prx-demo/demo.token --trust-key /tmp/prx-demo/server.pub \
  --interval 1h
```

The client immediately syncs and then polls. Its inventory file is reloaded each
cycle, so new disclosures and inventory changes require no lockfile changes.
Invalid inventory reload pauses that sync cycle and retains old inventory/data.
No upstream option means an offline local query service, not a failed network
configuration. Stop and restart services against the same directories to retain
state. Use `bash examples/registry/smoke.sh /tmp/nebprx` for an isolated automated
process-level smoke with logs, restart checks and invalid-token verification.

## Production exports, authentication and TLS

`examples/registry/sources.json` configures the four OSV ecosystem ZIP exports.
These are real corpus downloads, not required for synthetic tests:

```sh
nebprx registry --mode server --state-dir /srv/prx-global --action update \
  --sources examples/registry/sources.json
nebprx registry --mode server --state-dir /srv/prx-global \
  --listen 0.0.0.0:8091 --tls-cert /secure/fullchain.pem --tls-key /secure/key.pem \
  --sources examples/registry/sources.json --interval 6h
```

Create `/srv/prx-global` with mode 0700 or let PRX create it. TLS key must be a
regular owner-only file. TLS 1.3 is required for inbound service connections.
Clients verify server certificates using Go's system certificate roots, including
`SSL_CERT_FILE`/`SSL_CERT_DIR` where supported by Go. There is no insecure-skip
verification flag. HTTP is allowed only for a literal loopback IP with explicit
`--allow-loopback-http` on both client and server. No hostname-resolution
exception. Do not expose development HTTP through a tunnel or port forward.

Each installation receives a random 256-bit bearer token, stored hashed on the
server. `--action enroll --installation ID --output NEW_FILE` creates or rotates
that installation's token. Existing output files are never overwritten. Deliver
the file by an authenticated private channel, keep it mode 0600, and restart
clients after rotation. Old tokens fail immediately once rotation commits.
Enrollment is a local administrative command, not a public registration API.
Use a new token to invalidate a compromised token. No token expiry or standalone
revocation command in v1. No embedded universal secret, mTLS or binary attestation.
Possession of a token identifies an installation, not an unmodified PRX binary.

The server authenticates the requested installation ID against that installation's
credential. A valid token cannot request another installation's signed envelope.
Publishing is stateless with respect to tenants: requests contain public,
unversioned identities only, and no private inventory is stored server-side.
Every enrolled installation may query any supported public package. This is not
an inventory ACL. The service access restriction does not restrict recipients'
upstream advisory-data license rights.

Upstream URLs are administrator-configured HTTPS exports only. Requests cannot
select URLs. Redirects are disabled on upstream fetches and authenticated syncs.
Tokens never go to upstream sources or redirected hosts. Do not configure URLs
with credentials/query secrets. Advisory reference URLs are preserved as raw
data but never fetched. Request bodies and package names are not logged.

## CLI contract

All flags precede positional arguments. Registry accepts no positional arguments.
`--mode` is mandatory. `--action serve` is the default. Failures exit nonzero.
`--help` exits successfully and prints every flag.

| Action | Role | Inputs and result |
| --- | --- | --- |
| `init` | either | Creates durable role/schema, local API token; server also creates signing key and prints public key |
| `enroll` | server | `--installation ID --output NEW_TOKEN_FILE`; rotates existing installation credential |
| `import` | server | `--ecosystem E --input ARRAY_OR_ZIP --observed-at RFC3339`; full ecosystem replacement |
| `update` | server | `--sources CONFIG_JSON`; fetches and reconciles configured exports |
| `inventory` | client | `--input OWNER_ONLY_JSON`; atomic inventory and local-decision replacement |
| `sync` | client | `--upstream URL --installation ID --token-file FILE --trust-key FILE`; optional `--inventory FILE` reload |
| `check` | client | `--ecosystem E --name NAME --package-version EXACT [--public]`; offline result by default |
| `check --online` / `subscribe` | client | Explicit `--public` required; persists bounded candidate, syncs complete scopes and checks version |
| `status` | either | Local DB status JSON, source health or inventory findings |

Common service flags: `--listen`, `--state-dir`, `--tls-cert`, `--tls-key`,
`--allow-loopback-http`, `--interval`. Default directories are
`$HOME/.nebprx/registry-server` and `$HOME/.nebprx/registry-client`. Listen defaults are
`127.0.0.1:8091` and `127.0.0.1:8092`. Default interval is 1h, allowed 1m to 24h.
No new implicit `NEBPRX_*` environment configuration is introduced.

Online first-seen example:

```sh
nebprx registry --mode client --state-dir /tmp/prx-demo/client \
  --action check --online --public --ecosystem npm --name new-public-demo \
  --package-version 1.0.0 --upstream http://127.0.0.1:8091 \
  --installation demo --token-file /tmp/prx-demo/demo.token \
  --trust-key /tmp/prx-demo/server.pub --allow-loopback-http
```

A missing offline scope returns `indeterminate`. A failed online sync exits
nonzero and retains verified scopes. Candidate subscriptions remain for retry
and periodic refresh. At most 64 distinct candidates, and 256 total subscriptions.
Candidate reset currently requires a new client state directory and explicit
trust bootstrap. Never mark a private namespace public merely to obtain a result.
Inventory entries marked private override candidates and check request visibility.

## Gateway integration

Every service option has a gateway `--registry-` prefix. Empty mode disables it:

```sh
nebprx gateway --registry-mode client \
  --registry-state-dir /tmp/prx-demo/client \
  --registry-listen 127.0.0.1:8092 --registry-allow-loopback-http \
  --registry-inventory /tmp/prx-demo/inventory.json \
  --registry-upstream http://127.0.0.1:8091 --registry-installation demo \
  --registry-token-file /tmp/prx-demo/demo.token \
  --registry-trust-key /tmp/prx-demo/server.pub examples/gateway.example.hcl

nebprx gateway --registry-mode server --registry-state-dir /tmp/prx-demo/server \
  --registry-listen 127.0.0.1:8091 --registry-allow-loopback-http \
  examples/gateway.example.hcl
```

Do not simultaneously run standalone service on the same port. Registry config,
key, DB and bind errors terminate gateway startup before gateway listeners start.
Unexpected registry listener failure terminates the process visibly. Upstream
refresh/sync failure degrades health but does not terminate the service. Gateway
signal shutdown closes registry HTTP, cancels background work and closes SQLite
through its bounded shutdown hook before continuing existing shutdown. This
integration starts services only. Gateway policies do not yet consume findings.

## Inventory and API schemas

Inventory JSON is a package list, not a resolver or SBOM parser:

```json
{
  "graph_complete": false,
  "packages": [
    {"ecosystem":"PyPI","name":"Example_Pkg","version":"1.0","public":true,"decision":"review"},
    {"ecosystem":"npm","name":"@private/tool","version":"3.0.0","public":false}
  ]
}
```

Supported ecosystems: exactly `npm`, `PyPI`, `crates.io`, `Go`. Versions must be
exact, not requirements, npm tags or Cargo constraints. `public` defaults false.
`decision` is optional `allow|deny|review`, persisted locally and independent of
matching evidence. An explicit administrative allow may coexist with a finding
or unknown data. It does not mean data is complete or safe. Only persisted
inventory decisions apply to the same identity/version. Check requests cannot
supply their own decision or override administrator policy.
Graph completeness is caller-supplied evidence, defaults false, and is reported
without inferring completeness from the list. No dependency graphs, versions,
decisions or private names are sent upstream.

All HTTP endpoints require bearer authentication. No CORS/browser Origin access.
Local endpoints use the independently generated `STATE/local.token`, not an
installation token. CLI one-shot commands access owner-protected DB directly.

| Endpoint | Role | Body / result |
| --- | --- | --- |
| `GET /v1/status` | either | Local-token auth; status with source health or inventory findings |
| `POST /v1/check` | client | Local-token auth; ecosystem, name, version and public fields only; returns local result |
| `POST /v1/snapshot` | server | Installation-token auth; body below; signed complete scope envelope |

```json
{"installation":"demo","packages":[{"ecosystem":"npm","name":"example"}]}
```

Strict request fields, canonical identities, duplicate scope rejection. Response
envelope: `{"payload":"BASE64","signature":"BASE64","sha256":"HEX"}`.
Decoded payload includes `protocol`, `installation`, durable `sequence`,
`revision`, `issued`, `expires` and sorted `scopes`. Each scope contains
`identity`, `records` (including zero records) and signed `health`. Each record
contains exact raw OSV bytes encoded as base64, SHA-256, source URL, data license,
attribution and local observation time. Raw bytes preserve upstream timestamps,
aliases, withdrawal, ranges, explicit versions and all unknown OSV metadata.

Query results keep independent `finding`, `decision`, `coverage`, `freshness`,
`existence` and `evidence` fields. Findings are `known_affected`,
`no_known_match` or `indeterminate`. Default decision is review, or deny for known
affected. Fresh complete empty scopes can yield `no_known_match`, never `safe`.
Existence remains `unknown` because vulnerability feeds are not package catalogs.
Known affected versions remain known when stale. Incomplete, expired, private,
unknown and unsupported data cannot create a complete negative.

Status HTTP 200 means the local service/DB can answer, not that intelligence is
fresh. Inspect source `state`, `checked`, `rejected`, query `freshness` and
`sync_attempt` (`failed|verified`). Failed attempts do not replace `last_sync`,
the last verified snapshot metadata. Errors use non-2xx statuses with short text
bodies, without package names, credentials or DB details.

## Ingestion, licensing and reconciliation

Primary contracts: [OSV export documentation](https://google.github.io/osv.dev/data/),
[OSV schema](https://ossf.github.io/osv-schema/),
[Go module version semantics](https://pkg.go.dev/golang.org/x/mod/semver),
[Python normalization](https://packaging.python.org/en/latest/specifications/name-normalization/).

Configured sources are full OSV ecosystem ZIPs, not arbitrary record endpoints
or incremental lists. Local imports accept ZIPs or JSON arrays. The operator
asserts they represent the entire desired ecosystem source set and supplies the
actual observation time. Do not stamp an old offline export with the current
time. Complete means complete for that accepted export/source set only, not all
real vulnerabilities. Export digest and source observation are signed. Upstream
`modified/published/withdrawn` remain distinct from import observation.

Every import replaces one ecosystem's records and package index atomically and
increments durable revision. Removed records disappear, withdrawals suppress
findings, corrected records replace old assertions, and previously empty scopes
include later disclosures. Full reconciliation avoids incremental timestamp
cursor races. This v1 does not consume `modified_id.csv` or `[EMPTY]` separately.
Ecosystem removal is reflected by next full export replacement. Cross-ecosystem
consistency is not claimed: exports commit independently. Failed/cancelled
imports retain prior records, never advance revision, mark source failed and
retain the old source check time. Malformed JSON and wrong-ecosystem exports
never activate. Unknown source/license records are excluded. Invalid package
coordinates in otherwise accepted records are quarantined: raw evidence remains
stored, but invalid coordinates are not indexed. Both exclusions and quarantine
increment `rejected` and make the ecosystem partial, so their omission cannot
produce complete negatives.

Accepted source mapping, not a blanket license for all OSV data:

| IDs | Attribution and data license |
| --- | --- |
| `GHSA-*` | [GitHub Advisory Database](https://github.com/github/advisory-database), CC-BY-4.0 |
| `PYSEC-*` | [PyPA advisory database](https://github.com/pypa/advisory-database), CC-BY-4.0 |
| `GO-*` | [Go vulnerability database](https://vuln.go.dev), CC-BY-4.0 data, distinct from BSD code |
| `RUSTSEC-*` | [RustSec](https://github.com/RustSec/advisory-db), CC0-1.0 unless explicit CC-BY-4.0 or GHSA-derived content; then CC-BY-4.0 |
| `MAL-*` and all others | Excluded pending source/report-specific rights review; source becomes partial |

RustSec's unknown explicit `database_specific.license` is rejected. OSV exports
are trusted as the ID-to-origin mapping input; prefixes alone cannot authenticate
a locally imported record. Preserve RustSec/GHSA attribution embedded in raw data.
Records are not transformed beyond indexing and wrapper metadata. All original
notices remain in raw records. CC-BY attribution/license links accompany every
delivered record. Source rights and production redistribution still require
operator review, especially mixed/derived records. Tool-code licenses do not
relicense data. MAL's repository Apache license is not treated as authorization
for all external/imported reports.

## Persistence, trust and failure limits

SQLite schema version 1 has strict `server|client` metadata. Wrong-role reopen
fails. State/key/token files are owner-only; directories must be 0700 and not
symlinks. SQLite uses FULL synchronous transactions and DELETE journals, no
long-lived WAL. Back up whole state directories consistently while stopped.
One running service per directory is recommended. Local administrative CLI
operations may use the same DB while it runs. SQLite lock contention returns an
error instead of publishing partial state. Plaintext installation tokens remain
only in administrator-selected client credential files; signing key and local
API token are separate files.

Distribution uses pinned Ed25519, **not full TUF**. Signatures cover the exact
payload bytes including fixed protocol identifier, installation, scopes, revision,
sequence and times. SHA-256 covers those bytes too. Verification checks signature,
hashes, exact complete scope set, attribution, record relevance, time bounds and
persisted strictly increasing sequence/nondecreasing revision. Scope activation,
trust tuple and replay floors commit in one transaction. Bad signature, expired
envelope, wrong installation/scope/key, rollback or disk failure retains prior
verified data. The six-hour snapshot expiry and 48-hour maximum source check age
are independent. Newly signing old/failed source health does not make it fresh.

Bootstrap `server.pub` using a separately authenticated operator channel. The
public key is not secretly fetched or accepted from the sync response. The first
successful activation persists server URL + installation ID + key. Subsequent
pin changes are rejected before any credentials or package subscriptions are sent.
Server signing-key rotation has no cross-signing,
threshold, delegated-role, trusted-root expiry or compromise recovery protocol
in v1. Rotate via separately authenticated new client state/bootstrap and retain
old state for investigation. Never delete replay state merely to accept an older
snapshot. Restoring an old DB backup can lose replay floors or server sequence.
Server and clients must be recovered as a coordinated administrative operation.
Host clock and local filesystem integrity remain trust assumptions.

Matching uses OSV affected membership, not installer requirement resolution:

| Ecosystem | v1 semantics |
| --- | --- |
| npm | Exact scoped/unscoped spelling including published mixed-case names; exact SemVer incl. prereleases/build metadata; OSV range intervals, not npm installer prerelease exclusion |
| crates.io | Exact crate spelling including mixed-case advisory names, hyphen/underscore not merged; exact SemVer, not Cargo caret requirements |
| Go | Case-sensitive module paths, exact SemVer/pseudo-version ordering via x/mod, major-path validation; stdlib/toolchain special identities |
| PyPI | PEP503 name normalization and PEP440 release tuples incl. leading/trailing zero normalization; epoch/pre/post/dev/local forms explicitly indeterminate |

Explicit versions and ranges are alternatives. Introduced inclusive, fixed and
limit exclusive, last_affected inclusive. Unsupported GIT ranges, unsupported
versions and malformed relevant range semantics cannot clear a package. Aliases
and independent source records are retained as evidence, not merged into an
invented consensus or a deduplicated vulnerability count. Withdrawals are retained
raw but no longer generate findings. No call-graph reachability, package imports,
stdlib symbol reachability, target markers, dependency resolution, artifact hashes,
malware detection, package-existence verification or graph extraction is provided.

Resource ceilings: 512 MiB downloaded/compressed export, 1 GiB expanded export,
250,000 entries, 2 MiB individual OSV/request/inventory/offline JSON array, 4,096 advisory records per
scope, 32 MiB decoded snapshot, 64 MiB envelope. Structured per-record limits bound
affected entries, explicit versions, ranges and events. Archives are streamed per
entry and never extracted or recursively unpacked. HTTP has eight concurrent
requests, shared 8 requests/sec with burst 32, bounded headers/timeouts. Shared
limits mean abusive callers can reduce availability, not bypass authorization.
Full exports may exceed v1 ceilings as feeds grow. Such updates fail visibly and
retain old data rather than silently truncating. No corpus sizing/SLA claim.
